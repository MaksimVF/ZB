package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"

	gen "github.com/MaksimVF/ZB/services/head-go/gen"
	model "github.com/MaksimVF/ZB/services/head-go/gen_model"
	"github.com/MaksimVF/ZB/services/head-go/internal/auth"
	"github.com/MaksimVF/ZB/services/head-go/internal/config"
	"github.com/MaksimVF/ZB/services/head-go/internal/docs"
	"github.com/MaksimVF/ZB/services/head-go/internal/embedding"
	"github.com/MaksimVF/ZB/services/head-go/internal/models"
	modelclient "github.com/MaksimVF/ZB/services/head-go/internal/providers"
	"github.com/MaksimVF/ZB/services/head-go/internal/metrics"
	"github.com/MaksimVF/ZB/services/head-go/internal/webhook"

	"github.com/afex/hystrix-go/hystrix"
	"github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/health/grpc_health_v1"
)

var (
	requestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{Name: "head_requests_total", Help: "Total requests"},
		[]string{"model", "status"},
	)
	requestLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{Name: "head_request_latency_seconds", Help: "Request latency"},
		[]string{"model"},
	)
	requestErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{Name: "head_request_errors_total", Help: "Total request errors"},
		[]string{"model", "error_type"},
	)
	activeConnections = promauto.NewGauge(
		prometheus.GaugeOpts{Name: "head_active_connections", Help: "Currently active connections"},
	)
	circuitBreakerState = promauto.NewGaugeVec(
		prometheus.GaugeOpts{Name: "head_circuit_breaker_state", Help: "Circuit breaker state"},
		[]string{"circuit", "state"},
	)

	// Tracing setup
	var tracer = setupTracer()

	// Connection pool
	var connectionPool = &sync.Pool{
		New: func() interface{} {
			return &grpc.ClientConn{}
		},
	}

	// Structured logger
	logger = zerolog.New(os.Stdout).With().
		Str("service", "head-go").
		Str("component", "server").
		Logger()
)

type HeadServer struct {
	gen.UnimplementedChatServiceServer // встраиваем, чтобы не писать заглушки
	gen.UnimplementedEmbeddingServiceServer // встраиваем, чтобы не писать заглушки
	cfg                    *config.Config
	model                  *modelclient.ModelClient
	auth                   *auth.Authenticator
	webhook                *webhook.WebhookClient
	registry               *models.ModelRegistry
	embedding              *embedding.EmbeddingService
	networkConfigManager   *config.NetworkConfigManager
	shutdown               bool
	shutdownMutex          sync.RWMutex
	activeRequests         int32
	maxRequests            int
	healthStatus           string
	healthMutex            sync.RWMutex
	logger                 zerolog.Logger
	startTime              time.Time
	metrics                *ServerMetrics
}

// ServerMetrics holds server-specific metrics
type ServerMetrics struct {
	mu            sync.RWMutex
	startTime     time.Time
	totalRequests int64
	totalErrors   int64
	avgLatency    float64
	lastError     error
	lastErrorTime time.Time
}

// NewServerMetrics creates new server metrics
func NewServerMetrics() *ServerMetrics {
	return &ServerMetrics{
		startTime: time.Now(),
	}
}

// IncrementRequests increments the request counter
func (m *ServerMetrics) IncrementRequests() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalRequests++
}

// IncrementErrors increments the error counter
func (m *ServerMetrics) IncrementErrors(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalErrors++
	m.lastError = err
	m.lastErrorTime = time.Now()
}

// UpdateLatency updates average latency
func (m *ServerMetrics) UpdateLatency(latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.totalRequests == 0 {
		m.avgLatency = latency.Seconds()
	} else {
		// Exponential moving average
		alpha := 0.1
		m.avgLatency = alpha*latency.Seconds() + (1-alpha)*m.avgLatency
	}
}

// GetMetrics returns current metrics
func (m *ServerMetrics) GetMetrics() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return map[string]interface{}{
		"uptime_seconds":      time.Since(m.startTime).Seconds(),
		"total_requests":      m.totalRequests,
		"total_errors":        m.totalErrors,
		"error_rate":          float64(m.totalErrors) / float64(m.totalRequests+1),
		"avg_latency_seconds": m.avgLatency,
		"last_error":          m.lastError.Error(),
		"last_error_time":     m.lastErrorTime,
	}
}

func New(cfg *config.Config, networkConfigManager *config.NetworkConfigManager) *HeadServer {
	// Get network config for model proxy address
	networkConfig := networkConfigManager.GetConfig()
	modelProxyAddr := networkConfig.HeadEndpoint
	if modelProxyAddr == "" {
		modelProxyAddr = cfg.ModelProxyAddr
	}

	// Validate configuration before initialization
	if err := validateConfig(cfg); err != nil {
		logger.Fatal().Err(err).Msg("Invalid configuration")
	}

	modelClient := modelclient.NewModelClient(modelProxyAddr, networkConfigManager)
	
	server := &HeadServer{
		cfg:                     cfg,
		model:                   modelClient,
		auth:                    auth.NewAuthenticator(cfg.AuthConfig),
		webhook:                 webhook.NewWebhookClient(cfg.WebhookConfig),
		registry:                cfg.ModelRegistry,
		embedding:               embedding.NewEmbeddingService(cfg, modelClient),
		networkConfigManager:    networkConfigManager,
		shutdown:                false,
		activeRequests:          0,
		maxRequests:             1000, // Default max concurrent requests
		healthStatus:            "healthy",
		logger:                  logger,
		startTime:               time.Now(),
		metrics:                 NewServerMetrics(),
	}

	logger.Info().
		Str("grpc_addr", cfg.GRPCAddr).
		Int("metrics_port", cfg.MetricsPort).
		Str("model_proxy_addr", modelProxyAddr).
		Msg("HeadServer initialized successfully")

	return server
}

// validateConfig validates the server configuration
func validateConfig(cfg *config.Config) error {
	var errs []error

	if cfg.GRPCAddr == "" {
		errs = append(errs, errors.New("GRPC address is required"))
	}

	if cfg.MetricsPort <= 0 || cfg.MetricsPort > 65535 {
		errs = append(errs, errors.New("metrics port must be between 1 and 65535"))
	}

	if cfg.AuthConfig.JWTSecret == "" {
		errs = append(errs, errors.New("JWT secret is required"))
	}

	if len(errs) > 0 {
		return fmt.Errorf("configuration validation failed: %v", errs)
	}

	return nil
}

func setupTracer() trace.Tracer {
	ctx := context.Background()

	// Get OTLP endpoint from environment or use default
	otlpEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if otlpEndpoint == "" {
		otlpEndpoint = "otel-collector:4317"
	}

	// Create OTLP exporter with better error handling
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithEndpoint(otlpEndpoint),
	)
	if err != nil {
		logger.Warn().
			Err(err).
			Str("endpoint", otlpEndpoint).
			Msg("Failed to create OTLP exporter, using noop tracer")
		return noop.NewTracerProvider().Tracer("head")
	}

	// Create tracer provider with proper resource configuration
	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(resource.NewSchemaless(
			attribute.String("service.name", "head-go"),
			attribute.String("service.version", getVersion()),
			attribute.String("environment", getEnvironment()),
		)),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tp)

	logger.Info().
		Str("endpoint", otlpEndpoint).
		Msg("OpenTelemetry tracer initialized successfully")

	return tp.Tracer("head-go")
}

// getVersion returns the service version
func getVersion() string {
	// In production, this would be set via ldflags during build
	return os.Getenv("SERVICE_VERSION")
}

// getEnvironment returns the deployment environment
func getEnvironment() string {
	return os.Getenv("DEPLOYMENT_ENVIRONMENT")
}

func (s *HeadServer) Run() error {
	ctx := context.Background()
	
	// Initialize tracing with better error handling
	if err := s.initializeTracing(ctx); err != nil {
		s.logger.Warn().
			Err(err).
			Msg("Failed to initialize tracing, continuing without tracing")
	}

	// Start metrics server
	if err := s.startMetricsServer(); err != nil {
		s.logger.Fatal().
			Err(err).
			Int("metrics_port", s.cfg.MetricsPort).
			Msg("Failed to start metrics server")
	}

	// Initialize circuit breakers with configuration
	if err := s.initializeCircuitBreakers(); err != nil {
		s.logger.Fatal().
			Err(err).
			Msg("Failed to initialize circuit breakers")
	}

	// Initialize model client with timeout and retry logic
	if err := s.initializeModelClient(ctx); err != nil {
		s.logger.Fatal().
			Err(err).
			Msg("Failed to initialize model client")
	}

	// Load TLS credentials for mTLS
	creds, err := s.loadTLSCredentials()
	if err != nil {
		s.logger.Fatal().
			Err(err).
			Msg("Failed to load TLS credentials")
	}

	// Create gRPC server with comprehensive configuration
	srv, err := s.createGRPCServer(creds)
	if err != nil {
		s.logger.Fatal().
			Err(err).
			Msg("Failed to create gRPC server")
	}

	// Register services
	s.registerServices(srv)

	// Start health check goroutine
	go s.runHealthChecks()

	// Start server with proper error handling
	if err := s.startServer(srv); err != nil {
		s.logger.Fatal().
			Err(err).
			Str("grpc_addr", s.cfg.GRPCAddr).
			Msg("Server startup failed")
	}

	return nil
}

// initializeTracing initializes OpenTelemetry tracing
func (s *HeadServer) initializeTracing(ctx context.Context) error {
	return metrics.InitializeTracing(ctx)
}

// startMetricsServer starts the metrics, health, and documentation HTTP server
func (s *HeadServer) startMetricsServer() error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/health", s.healthCheckHandler)
	mux.Handle("/docs/", http.StripPrefix("/docs", docs.DocumentationHandler()))

	addr := fmt.Sprintf(":%d", s.cfg.MetricsPort)
	s.logger.Info().
		Int("metrics_port", s.cfg.MetricsPort).
		Msg("Starting metrics, health, and documentation server")

	go func() {
		if err := http.ListenAndServe(addr, mux); err != nil {
			s.logger.Error().
				Err(err).
				Int("metrics_port", s.cfg.MetricsPort).
				Msg("Metrics server failed")
		}
	}()

	return nil
}

// initializeCircuitBreakers initializes circuit breaker configurations
func (s *HeadServer) initializeCircuitBreakers() error {
	// Configure circuit breaker with environment-based settings
	timeout := s.getCircuitBreakerTimeout()
	maxConcurrent := s.getCircuitBreakerMaxConcurrent()
	errorThreshold := s.getCircuitBreakerErrorThreshold()

	hystrix.ConfigureCommand("model_proxy", hystrix.CommandConfig{
		Timeout:                timeout,
		MaxConcurrentRequests:  maxConcurrent,
		ErrorPercentThreshold:  errorThreshold,
		SleepWindow:            10000, // 10 seconds recovery window
		RequestVolumeThreshold: 10,   // Minimum requests before tripping
	})

	s.logger.Info().
		Int("timeout_ms", timeout/1000000).
		Int("max_concurrent", maxConcurrent).
		Int("error_threshold", errorThreshold).
		Msg("Circuit breakers initialized")

	return nil
}

// initializeModelClient initializes the model client with retry logic
func (s *HeadServer) initializeModelClient(ctx context.Context) error {
	initCtx, cancel := context.WithTimeout(ctx, s.getModelClientTimeout())
	defer cancel()

	maxRetries := s.getMaxRetries()
	for attempt := 1; attempt <= maxRetries; attempt++ {
		select {
		case <-initCtx.Done():
			return fmt.Errorf("model client initialization timeout after %d attempts", attempt-1)
		default:
		}

		err := s.model.Init(ctx)
		if err == nil {
			s.logger.Info().
				Int("attempt", attempt).
				Msg("Model client initialized successfully")
			return nil
		}

		s.logger.Warn().
			Err(err).
			Int("attempt", attempt).
			Int("max_retries", maxRetries).
			Msg("Model client initialization failed, retrying...")

		if attempt < maxRetries {
			delay := time.Duration(attempt) * time.Second
			time.Sleep(delay)
			continue
		}

		// Handle specific error types
		var errType string
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			errType = "timeout"
		case isCertificateError(err):
			errType = "certificate"
		case isConnectionError(err):
			errType = "connection"
		default:
			errType = "unknown"
		}

		s.logger.Error().
			Err(err).
			Str("error_type", errType).
			Int("attempt", attempt).
			Msg("Model client initialization failed after all retries")

		return fmt.Errorf("model client initialization failed: %w", err)
	}

	return fmt.Errorf("model client initialization failed after %d attempts", maxRetries)
}

// loadTLSCredentials loads TLS credentials for mTLS
func (s *HeadServer) loadTLSCredentials() (credentials.TransportCredentials, error) {
	return loadServerTLSCredentials()
}

// createGRPCServer creates the gRPC server with comprehensive configuration
func (s *HeadServer) createGRPCServer(creds credentials.TransportCredentials) (*grpc.Server, error) {
	// Configure keepalive for better connection management
	keepaliveParams := grpc.KeepaliveParams{
		Time:    10 * time.Second, // ping every 10 seconds
		Timeout: 2 * time.Second,  // wait 2 seconds for pong
	}
	keepalivePolicy := grpc.KeepaliveEnforcementPolicy{
		MinTime:             5 * time.Second, // minimum ping interval
		PermitWithoutStream: true,
	}

	// Create authentication interceptors
	var unaryInterceptors []grpc.UnaryServerInterceptor
	var streamInterceptors []grpc.StreamServerInterceptor

	// Add monitoring and tracing middleware
	unaryInterceptors = append(unaryInterceptors,
		grpc_prometheus.UnaryServerInterceptor,
		otelgrpc.UnaryServerInterceptor(),
	)

	streamInterceptors = append(streamInterceptors,
		grpc_prometheus.StreamServerInterceptor,
		otelgrpc.StreamServerInterceptor(),
	)

	// Add authentication if enabled
	if s.cfg.FeaturesConfig.IsEnabled("authentication") {
		s.logger.Info().Msg("Authentication enabled")
		unaryInterceptors = append(unaryInterceptors, s.auth.UnaryServerInterceptor())
		streamInterceptors = append(streamInterceptors, s.auth.StreamServerInterceptor())
	} else {
		s.logger.Info().Msg("Authentication disabled")
	}

	// Create gRPC server with middleware
	srv := grpc.NewServer(
		grpc.Creds(creds),
		grpc.KeepaliveParams(keepaliveParams),
		grpc.KeepaliveEnforcementPolicy(keepalivePolicy),
		grpc.MaxConcurrentStreams(1000), // Limit concurrent streams
		grpc.ConnectionTimeout(5*time.Second), // Connection timeout
		grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(streamInterceptors...)),
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(unaryInterceptors...)),
	)

	return srv, nil
}

// registerServices registers all gRPC services
func (s *HeadServer) registerServices(srv *grpc.Server) {
	gen.RegisterChatServiceServer(srv, s)
	gen.RegisterEmbeddingServiceServer(srv, s.embedding)
	grpc_prometheus.Register(srv)
	grpc_health_v1.RegisterHealthServer(srv, s)

	s.logger.Info().Msg("All services registered successfully")
}

// startServer starts the gRPC server
func (s *HeadServer) startServer(srv *grpc.Server) error {
	lis, err := net.Listen("tcp", s.cfg.GRPCAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.cfg.GRPCAddr, err)
	}

	s.logger.Info().
		Str("grpc_addr", s.cfg.GRPCAddr).
		Msg("Starting gRPC+mTLS server")

	if err := srv.Serve(lis); err != nil {
		return fmt.Errorf("server failed: %w", err)
	}

	return nil
}

// Helper methods for configuration
func (s *HeadServer) getCircuitBreakerTimeout() time.Duration {
	timeout := os.Getenv("CIRCUIT_BREAKER_TIMEOUT")
	if timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil {
			return d
		}
	}
	return 5 * time.Second
}

func (s *HeadServer) getCircuitBreakerMaxConcurrent() int {
	if max := os.Getenv("CIRCUIT_BREAKER_MAX_CONCURRENT"); max != "" {
		if val, err := parseInt(max); err == nil && val > 0 {
			return val
		}
	}
	return 100
}

func (s *HeadServer) getCircuitBreakerErrorThreshold() int {
	if threshold := os.Getenv("CIRCUIT_BREAKER_ERROR_THRESHOLD"); threshold != "" {
		if val, err := parseInt(threshold); err == nil && val > 0 && val <= 100 {
			return val
		}
	}
	return 25
}

func (s *HeadServer) getModelClientTimeout() time.Duration {
	if timeout := os.Getenv("MODEL_CLIENT_TIMEOUT"); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil {
			return d
		}
	}
	return 15 * time.Second
}

func (s *HeadServer) getMaxRetries() int {
	if retries := os.Getenv("MAX_RETRIES"); retries != "" {
		if val, err := parseInt(retries); err == nil && val > 0 {
			return val
		}
	}
	return 3
}

// Helper functions for error type detection
func isCertificateError(err error) bool {
	var x509Err x509.UnknownAuthorityError
	return errors.As(err, &x509Err)
}

func isConnectionError(err error) bool {
	return errors.Is(err, context.DeadlineExceeded) || 
		   errors.Is(err, context.Canceled)
}

func parseInt(s string) (int, error) {
	var val int
	_, err := fmt.Sscanf(s, "%d", &val)
	return val, err
}

func (s *HeadServer) runHealthChecks() {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            // Check model client health
            if s.model != nil && s.model.conn != nil {
                // Check connection state
                state := s.model.conn.GetState()
                if state != grpc.ConnectivityReady {
                    log.Printf("Model client connection state: %s", state)
                    s.SetHealthStatus("NOT_SERVING")
                    // Try to reconnect
                    err := s.model.reconnect(context.Background())
                    if err != nil {
                        log.Printf("Failed to reconnect: %v", err)
                    }
                } else {
                    s.SetHealthStatus("SERVING")
                }
            }

            // Check active requests
            active := atomic.LoadInt32(&s.activeRequests)
            if active > int32(s.maxRequests)*90/100 {
                log.Printf("High load: %d active requests (limit: %d)", active, s.maxRequests)
            }

            // Check for configuration updates
            if s.networkConfigManager != nil {
                err := s.networkConfigManager.LoadConfig()
                if err != nil {
                    log.Printf("Failed to reload network config: %v", err)
                } else {
                    log.Printf("Network config reloaded successfully")
                    // Reconnect model client with new config
                    err := s.model.reconnect(context.Background())
                    if err != nil {
                        log.Printf("Failed to reconnect with new config: %v", err)
                    }
                }
            }
        }
    }
}

// Health check implementation
func (s *HeadServer) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	s.healthMutex.RLock()
	defer s.healthMutex.RUnlock()

	// Enhanced health check with additional validations
	if err := s.performHealthCheck(); err != nil {
		s.logger.Warn().
			Err(err).
			Str("service", req.Service).
			Msg("Health check failed")

		return &grpc_health_v1.HealthCheckResponse{
			Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
		}, nil
	}

	return &grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_ServingStatus(
			grpc_health_v1.HealthCheckResponse_ServingStatus_value[s.healthStatus],
		),
	}, nil
}

// performHealthCheck performs comprehensive health checks
func (s *HeadServer) performHealthCheck() error {
	var errs []error

	// Check model client health
	if s.model != nil && s.model.conn != nil {
		state := s.model.conn.GetState()
		if state != grpc.ConnectivityReady {
			errs = append(errs, fmt.Errorf("model client connection state: %s", state))
		}
	} else {
		errs = append(errs, errors.New("model client not initialized"))
	}

	// Check active requests vs limit
	active := atomic.LoadInt32(&s.activeRequests)
	if active > int32(s.maxRequests)*90/100 {
		errs = append(errs, fmt.Errorf("high load: %d active requests (limit: %d)", active, s.maxRequests))
	}

	// Check configuration validity
	if err := validateConfig(s.cfg); err != nil {
		errs = append(errs, fmt.Errorf("configuration invalid: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("health check failed: %v", errs)
	}

	return nil
}

func (s *HeadServer) Watch(req *grpc_health_v1.HealthCheckRequest, stream grpc_health_v1.Health_WatchServer) error {
    // Simple implementation - could be enhanced with actual state changes
    for {
        select {
        case <-stream.Context().Done():
            return nil
        case <-time.After(5 * time.Second):
            resp, err := s.Check(stream.Context(), req)
            if err != nil {
                return err
            }
            if err := stream.Send(resp); err != nil {
                return err
            }
        }
    }
}

// Update health status
func (s *HeadServer) SetHealthStatus(status string) {
    s.healthMutex.Lock()
    defer s.healthMutex.Unlock()
    s.healthStatus = status
}

// Batch processing method
func (s *HeadServer) BatchGenerate(ctx context.Context, req *model.BatchGenRequest) (*model.BatchGenResponse, error) {
    start := time.Now()
    var responses []*model.GenResponse

    // Start tracing span
    ctx, span := tracer.Start(ctx, "BatchGenerate",
        trace.WithAttributes(
            attribute.Int("requests", len(req.Requests)),
        ),
    )
    defer span.End()

    // Increment active request count
    atomic.AddInt32(&s.activeRequests, 1)
    defer atomic.AddInt32(&s.activeRequests, -1)

    // Update active connections metric
    activeConnections.Set(float64(atomic.LoadInt32(&s.activeRequests)))

    // Process each request in the batch
    for _, singleReq := range req.Requests {
        // Execute with circuit breaker
        var responseText string
        var tokensUsed int
        err := hystrix.Do("model_proxy", func() error {
            var err error
            responseText, tokensUsed, err = s.model.Generate(ctx, singleReq.Model, singleReq.Messages, singleReq.Temperature, singleReq.MaxTokens)
            if err != nil {
                requestErrors.WithLabelValues(singleReq.Model, "model_error").Inc()
                circuitBreakerState.WithLabelValues("model_proxy", "open").Set(1)
                return fmt.Errorf("model error: %w", err)
            }
            return nil
        }, nil)

        if err != nil {
            requestErrors.WithLabelValues(singleReq.Model, "circuit_breaker").Inc()
            // Add error response for this request
            responses = append(responses, &model.GenResponse{
                RequestId: singleReq.RequestId,
                Text:      fmt.Sprintf("Error: %v", err),
                TokensUsed: 0,
            })
            continue
        }

        // Update circuit breaker state
        circuitBreakerState.WithLabelValues("model_proxy", "closed").Set(1)

        // Add successful response
        responses = append(responses, &model.GenResponse{
            RequestId: singleReq.RequestId,
            Text:      responseText,
            TokensUsed: int32(tokensUsed),
        })
    }

    requestLatency.WithLabelValues("batch").Observe(time.Since(start).Seconds())
    requestsTotal.WithLabelValues("batch", "ok").Inc()

    return &model.BatchGenResponse{
        Responses: responses,
    }, nil
}

// runHealthChecks performs periodic health checks and maintenance
func (s *HeadServer) runHealthChecks() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.performPeriodicHealthCheck()
		}
	}
}

// performPeriodicHealthCheck performs health checks and maintenance tasks
func (s *HeadServer) performPeriodicHealthCheck() {
	// Check model client health with retry logic
	if err := s.checkModelClientHealth(); err != nil {
		s.logger.Warn().
			Err(err).
			Msg("Model client health check failed, attempting recovery")
		if err := s.attemptModelClientRecovery(); err != nil {
			s.logger.Error().
				Err(err).
				Msg("Failed to recover model client")
		}
	}

	// Check load and performance metrics
	s.checkLoadMetrics()

	// Check configuration updates
	if err := s.checkConfigurationUpdates(); err != nil {
		s.logger.Warn().
			Err(err).
			Msg("Configuration update check failed")
	}

	// Update circuit breaker metrics
	s.updateCircuitBreakerMetrics()
}

// checkModelClientHealth checks the health of the model client
func (s *HeadServer) checkModelClientHealth() error {
	if s.model == nil || s.model.conn == nil {
		return errors.New("model client not initialized")
	}

	state := s.model.conn.GetState()
	switch state {
	case grpc.ConnectivityReady:
		return nil
	case grpc.Connecting:
		s.logger.Debug().Msg("Model client is connecting")
		return nil
	case grpc.TransientFailure:
		return errors.New("model client in transient failure state")
	case grpc.Idle:
		s.logger.Debug().Msg("Model client is idle")
		return nil
	case grpc.Shutdown:
		return errors.New("model client is shutdown")
	default:
		return fmt.Errorf("unknown model client state: %v", state)
	}
}

// attemptModelClientRecovery attempts to recover the model client
func (s *HeadServer) attemptModelClientRecovery() error {
	s.logger.Info().Msg("Attempting to recover model client")
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.model.reconnect(ctx); err != nil {
		return fmt.Errorf("failed to reconnect: %w", err)
	}

	s.logger.Info().Msg("Model client recovery successful")
	return nil
}

// checkLoadMetrics checks current load and performance metrics
func (s *HeadServer) checkLoadMetrics() {
	active := atomic.LoadInt32(&s.activeRequests)
	loadPercentage := float64(active) / float64(s.maxRequests) * 100

	switch {
	case loadPercentage > 90:
		s.logger.Warn().
			Int("active_requests", int(active)).
			Int("max_requests", s.maxRequests).
			Float64("load_percentage", loadPercentage).
			Msg("Critical load detected")
	case loadPercentage > 75:
		s.logger.Info().
			Int("active_requests", int(active)).
			Int("max_requests", s.maxRequests).
			Float64("load_percentage", loadPercentage).
			Msg("High load detected")
	}

	// Update metrics
	activeConnections.Set(float64(active))
}

// checkConfigurationUpdates checks for configuration updates
func (s *HeadServer) checkConfigurationUpdates() error {
	if s.networkConfigManager == nil {
		return nil
	}

	if err := s.networkConfigManager.LoadConfig(); err != nil {
		return fmt.Errorf("failed to reload network config: %w", err)
	}

	s.logger.Debug().Msg("Network config reloaded successfully")
	
	// Attempt to reconnect with new configuration
	if err := s.model.reconnect(context.Background()); err != nil {
		return fmt.Errorf("failed to reconnect with new config: %w", err)
	}

	return nil
}

// updateCircuitBreakerMetrics updates circuit breaker state metrics
func (s *HeadServer) updateCircuitBreakerMetrics() {
	// This would typically query the actual circuit breaker state
	// For now, we'll just log the current state
	s.logger.Debug().Msg("Circuit breaker metrics updated")
}

// HTTP health check handler
func (s *HeadServer) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	
	// Perform health check
	if err := s.performHealthCheck(); err != nil {
		s.logger.Warn().
			Err(err).
			Str("remote_addr", r.RemoteAddr).
			Dur("duration", time.Since(start)).
			Msg("HTTP health check failed")

		http.Error(w, "Service Unhealthy", http.StatusServiceUnavailable)
		return
	}

	// Return detailed health status
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	healthStatus := map[string]interface{}{
		"status":     "healthy",
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"uptime":     time.Since(s.startTime).String(),
		"active_requests": int(atomic.LoadInt32(&s.activeRequests)),
		"max_requests":    s.maxRequests,
		"version":         getVersion(),
		"environment":     getEnvironment(),
	}

	// Add metrics if available
	if s.metrics != nil {
		healthStatus["metrics"] = s.metrics.GetMetrics()
	}

	if err := writeJSON(w, healthStatus); err != nil {
		s.logger.Warn().
			Err(err).
			Str("remote_addr", r.RemoteAddr).
			Msg("Failed to write health check response")
	}
}

// writeJSON writes JSON response with proper error handling
func writeJSON(w http.ResponseWriter, data interface{}) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}
func (s *HeadServer) ChatCompletion(ctx context.Context, req *gen.ChatRequest) (*gen.ChatResponse, error) {
    start := time.Now()
    modelName := req.Model
    if modelName == "" {
        modelName = "gpt-4o"
    }

    // Start tracing span
    ctx, span := tracer.Start(ctx, "ChatCompletion",
        trace.WithAttributes(
            attribute.String("model", modelName),
            attribute.Int("messages", len(req.Messages)),
        ),
    )
    defer span.End()

    // Increment active request count
    atomic.AddInt32(&s.activeRequests, 1)
    defer atomic.AddInt32(&s.activeRequests, -1)

    // Update active connections metric
    activeConnections.Set(float64(atomic.LoadInt32(&s.activeRequests)))

    messages := make([]string, 0, len(req.Messages))
    for _, m := range req.Messages {
        messages = append(messages, m.Content)
    }

    // Execute with circuit breaker
    var responseText string
    var tokensUsed int
    err := hystrix.Do("model_proxy", func() error {
        var err error
        responseText, tokensUsed, err = s.model.Generate(ctx, modelName, messages, float32(req.Temperature), req.MaxTokens)
        if err != nil {
            requestErrors.WithLabelValues(modelName, "model_error").Inc()
            circuitBreakerState.WithLabelValues("model_proxy", "open").Set(1)
            return fmt.Errorf("model error: %w", err)
        }
        return nil
    }, nil)

    if err != nil {
        requestErrors.WithLabelValues(modelName, "circuit_breaker").Inc()
        requestsTotal.WithLabelValues(modelName, "error").Inc()
        return nil, status.Errorf(codes.Internal, "request failed: %v", err)
    }

    // Update circuit breaker state
    circuitBreakerState.WithLabelValues("model_proxy", "closed").Set(1)

    requestLatency.WithLabelValues(modelName).Observe(time.Since(start).Seconds())
    requestsTotal.WithLabelValues(modelName, "ok").Inc()

    return &gen.ChatResponse{
        RequestId:  req.RequestId,
        FullText:   responseText,
        Model:      modelName,
        Provider:  "litellm",
        TokensUsed: int32(tokensUsed),
    }, nil
}

// Shutdown performs graceful shutdown of the server
func (s *HeadServer) Shutdown(ctx context.Context) error {
	s.logger.Info().Msg("Starting graceful shutdown")

	// Set shutdown flag
	s.shutdownMutex.Lock()
	s.shutdown = true
	s.shutdownMutex.Unlock()

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Perform graceful shutdown procedures
	if err := s.performGracefulShutdown(shutdownCtx); err != nil {
		s.logger.Error().
			Err(err).
			Msg("Graceful shutdown failed, forcing shutdown")
		return err
	}

	s.logger.Info().Msg("Graceful shutdown completed successfully")
	return nil
}

// performGracefulShutdown performs the actual shutdown procedures
func (s *HeadServer) performGracefulShutdown(ctx context.Context) error {
	var errs []error

	// Stop accepting new requests (handled by gRPC server)
	// Wait for existing requests to complete
	if err := s.waitForActiveRequests(ctx, 20*time.Second); err != nil {
		errs = append(errs, fmt.Errorf("timeout waiting for active requests: %w", err))
	}

	// Shutdown model client
	if err := s.model.Shutdown(ctx); err != nil {
		errs = append(errs, fmt.Errorf("model client shutdown failed: %w", err))
	}

	// Shutdown network config manager
	if s.networkConfigManager != nil {
		if err := s.networkConfigManager.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("network config manager shutdown failed: %w", err))
		}
	}

	// Shutdown webhook client
	if err := s.webhook.Shutdown(ctx); err != nil {
		errs = append(errs, fmt.Errorf("webhook client shutdown failed: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}

	return nil
}

// waitForActiveRequests waits for active requests to complete
func (s *HeadServer) waitForActiveRequests(ctx context.Context, timeout time.Duration) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	deadline := time.After(timeout)
	
	for {
		select {
		case <-deadline:
			active := atomic.LoadInt32(&s.activeRequests)
			if active > 0 {
				return fmt.Errorf("timeout reached with %d active requests remaining", active)
			}
			return nil
		case <-ticker.C:
			active := atomic.LoadInt32(&s.activeRequests)
			if active == 0 {
				s.logger.Info().Msg("All active requests completed")
				return nil
			}
			s.logger.Debug().
				Int("active_requests", int(active)).
				Msg("Waiting for active requests to complete")
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// GetStatus returns detailed server status information
func (s *HeadServer) GetStatus() map[string]interface{} {
	s.healthMutex.RLock()
	defer s.healthMutex.RUnlock()

	status := map[string]interface{}{
		"service":        "head-go",
		"status":         s.getServerStatus(),
		"health_status":  s.healthStatus,
		"uptime":         time.Since(s.startTime).String(),
		"active_requests": int(atomic.LoadInt32(&s.activeRequests)),
		"max_requests":   s.maxRequests,
		"grpc_addr":     s.cfg.GRPCAddr,
		"metrics_port":  s.cfg.MetricsPort,
		"redis_addr":    s.cfg.RedisAddr,
		"version":       getVersion(),
		"environment":   getEnvironment(),
	}

	// Add configuration information (excluding sensitive data)
	status["config"] = s.getConfigSummary()

	// Add metrics if available
	if s.metrics != nil {
		status["metrics"] = s.metrics.GetMetrics()
	}

	// Add circuit breaker status
	status["circuit_breakers"] = s.getCircuitBreakerStatus()

	return status
}

// getServerStatus returns the current server status
func (s *HeadServer) getServerStatus() string {
	s.shutdownMutex.RLock()
	defer s.shutdownMutex.RUnlock()
	
	if s.shutdown {
		return "shutting_down"
	}
	return "running"
}

// getConfigSummary returns a summary of the server configuration
func (s *HeadServer) getConfigSummary() map[string]interface{} {
	return map[string]interface{}{
		"grpc_addr":         s.cfg.GRPCAddr,
		"metrics_port":      s.cfg.MetricsPort,
		"model_proxy_addr":  s.cfg.ModelProxyAddr,
		"redis_addr":        s.cfg.RedisAddr,
		"reload_interval":   s.cfg.ReloadInterval.String(),
		"max_retries":       s.cfg.MaxRetries,
		"request_timeout":   s.cfg.RequestTimeout.String(),
		"log_level":        s.cfg.LogLevel,
		"features":         s.getEnabledFeatures(),
		"network_config":   s.getNetworkConfigSummary(),
	}
}

// getEnabledFeatures returns a list of enabled features
func (s *HeadServer) getEnabledFeatures() []string {
	var features []string
	
	if s.cfg.FeaturesConfig.IsEnabled("authentication") {
		features = append(features, "authentication")
	}
	if s.cfg.FeaturesConfig.IsEnabled("webhook") {
		features = append(features, "webhook")
	}
	if s.cfg.FeaturesConfig.IsEnabled("metrics") {
		features = append(features, "metrics")
	}
	if s.cfg.FeaturesConfig.IsEnabled("tracing") {
		features = append(features, "tracing")
	}
	
	return features
}

// getNetworkConfigSummary returns a summary of network configuration
func (s *HeadServer) getNetworkConfigSummary() map[string]interface{} {
	return map[string]interface{}{
		"enabled":      s.cfg.NetworkConfig.Enabled,
		"interface":    s.cfg.NetworkConfig.Interface,
		"mtu":          s.cfg.NetworkConfig.MTU,
		"enable_ipv6":  s.cfg.NetworkConfig.EnableIPv6,
		"dhcp_timeout": s.cfg.NetworkConfig.DHCPTimeout,
	}
}

// getCircuitBreakerStatus returns the status of circuit breakers
func (s *HeadServer) getCircuitBreakerStatus() map[string]interface{} {
	// This would typically query the actual circuit breaker state
	// For now, return a placeholder structure
	return map[string]interface{}{
		"model_proxy": map[string]interface{}{
			"state": "closed", // This would be retrieved from the actual circuit breaker
			"requests": 0,
			"errors": 0,
		},
	}
}

// Обычный (не стриминговый) запрос — возвращает полный текст сразу
func (s *HeadServer) ChatCompletionStream(req *gen.ChatRequest, stream gen.ChatService_ChatCompletionStreamServer) error {
    ctx := stream.Context()
    modelName := req.Model
    if modelName == "" {
        modelName = "gpt-4o"
    }

    // Start tracing span
    ctx, span := tracer.Start(ctx, "ChatCompletionStream",
        trace.WithAttributes(
            attribute.String("model", modelName),
            attribute.Int("messages", len(req.Messages)),
        ),
    )
    defer span.End()

    // Increment active request count
    atomic.AddInt32(&s.activeRequests, 1)
    defer atomic.AddInt32(&s.activeRequests, -1)

    messages := make([]string, 0, len(req.Messages))
    for _, m := range req.Messages {
        messages = append(messages, m.Content)
    }

    // Execute with circuit breaker
    var responseText string
    var tokensUsed int

    streamCh, errCh := s.model.GenerateStream(ctx, modelName, messages, float32(req.Temperature), req.MaxTokens)

    for {
        select {
        case resp, ok := <-streamCh:
            if !ok {
                return nil
            }
            if err := stream.Send(&gen.ChatStreamResponse{
                Chunk: resp.Text,
            }); err != nil {
                return err
            }
        case err, ok := <-errCh:
            if !ok {
                return nil
            }
            requestErrors.WithLabelValues(modelName, "stream_error").Inc()
            return status.Errorf(codes.Internal, "stream error: %v", err)
        }
    }
}
