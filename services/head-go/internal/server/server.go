package server

import (
    "crypto/tls"
    "crypto/x509"
    "fmt"
    "context"
    "io"
    "log"
    "net"
    "net/http"
    "os"
    "sync"
    "sync/atomic"
    "time"

    gen "github.com/yourorg/head/gen" // сюда генерируются chat.proto
    model "github.com/yourorg/head/gen_model" // сюда генерируются model.proto
    "github.com/yourorg/head/internal/auth"
    "github.com/yourorg/head/internal/config"
    "github.com/yourorg/head/internal/docs"
    "github.com/yourorg/head/internal/embedding"
    "github.com/yourorg/head/internal/models"
    modelclient "github.com/yourorg/head/internal/providers"
    "github.com/yourorg/head/internal/metrics"
    "github.com/yourorg/head/internal/webhook"

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
)

type HeadServer struct {
    gen.UnimplementedChatServiceServer // встраиваем, чтобы не писать заглушки
    gen.UnimplementedEmbeddingServiceServer // встраиваем, чтобы не писать заглушки
    cfg            *config.Config
    model          *modelclient.ModelClient
    auth           *auth.Authenticator
    webhook        *webhook.WebhookClient
    registry       *models.ModelRegistry
    embedding      *embedding.EmbeddingService
    shutdown       bool
    shutdownMutex  sync.RWMutex
    activeRequests int32
    maxRequests    int
    healthStatus   string
    healthMutex    sync.RWMutex
}

func New(cfg *config.Config) *HeadServer {
    modelClient := modelclient.NewModelClient(cfg.ModelProxyAddr)
    return &HeadServer{
        cfg:            cfg,
        model:          modelClient,
        auth:           auth.NewAuthenticator(cfg.AuthConfig),
        webhook:        webhook.NewWebhookClient(cfg.WebhookConfig),
        registry:       cfg.ModelRegistry,
        embedding:      embedding.NewEmbeddingService(cfg, modelClient),
        shutdown:       false,
        activeRequests: 0,
        maxRequests:    1000, // Default max concurrent requests
        healthStatus:   "healthy",
    }
}

func setupTracer() trace.Tracer {
    ctx := context.Background()

    // Create OTLP exporter
    exporter, err := otlptracegrpc.New(ctx,
        otlptracegrpc.WithInsecure(),
        otlptracegrpc.WithEndpoint("otel-collector:4317"),
    )
    if err != nil {
        log.Printf("Failed to create OTLP exporter: %v", err)
        return noop.NewTracerProvider().Tracer("head")
    }

    // Create tracer provider
    tp := trace.NewTracerProvider(
        trace.WithBatcher(exporter),
        trace.WithResource(resource.NewSchemaless(
            attribute.String("service.name", "head"),
            attribute.String("service.version", "1.0.0"),
        )),
    )

    otel.SetTracerProvider(tp)
    return tp.Tracer("head")
}

func (s *HeadServer) Run() error {
    // Initialize tracing
    ctx := context.Background()
    if err := metrics.InitializeTracing(ctx); err != nil {
        log.Printf("Failed to initialize tracing: %v", err)
    }

    // Start metrics server
    go func() {
        mux := http.NewServeMux()
        mux.Handle("/metrics", promhttp.Handler())
        mux.HandleFunc("/health", healthCheckHandler)
        mux.Handle("/docs/", http.StripPrefix("/docs", docs.DocumentationHandler()))

        log.Printf("Metrics, health, and documentation server listening on :%d", s.cfg.MetricsPort)
        if err := http.ListenAndServe(fmt.Sprintf(":%d", s.cfg.MetricsPort), mux); err != nil {
            log.Printf("Metrics server failed: %v", err)
        }
    }()

    // Initialize model client with timeout
    initCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
    defer cancel()

    // Initialize circuit breakers
    hystrix.ConfigureCommand("model_proxy", hystrix.CommandConfig{
        Timeout:                5000, // 5 seconds
        MaxConcurrentRequests:  100,
        ErrorPercentThreshold:   25,
        SleepWindow:            10000, // 10 seconds recovery window
        RequestVolumeThreshold: 10,   // Minimum requests before tripping
    })

    // Try to initialize model client with better error handling
    if err := s.model.Init(ctx); err != nil {
        log.Printf("Failed to initialize model client: %v", err)
        // Check if it is a certificate error
        if _, ok := err.(x509.UnknownAuthorityError); ok {
            log.Printf("Certificate authority error - check CA configuration")
        }
        // Check if it is a connection timeout
        if initCtx.Err() == context.DeadlineExceeded {
            log.Printf("Connection timeout - model-proxy service may be unavailable")
        }
        return fmt.Errorf("model client initialization failed: %w", err)
    }

    // Load TLS credentials for mTLS
    creds, err := loadServerTLSCredentials()
    if err != nil {
        log.Printf("Failed to load TLS credentials: %v", err)
        return fmt.Errorf("failed to load TLS credentials: %w", err)
    }

    // Configure keepalive for better connection management
    keepaliveParams := grpc.KeepaliveParams{
        Time:    10 * time.Second, // ping every 10 seconds
        Timeout: 2 * time.Second, // wait 2 seconds for pong
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
        log.Printf("Authentication enabled")
        unaryInterceptors = append(unaryInterceptors, s.auth.UnaryServerInterceptor())
        streamInterceptors = append(streamInterceptors, s.auth.StreamServerInterceptor())
    } else {
        log.Printf("Authentication disabled")
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

    // Register services
    gen.RegisterChatServiceServer(srv, s)
    gen.RegisterEmbeddingServiceServer(srv, s.embedding)
    grpc_prometheus.Register(srv)

    // Register health service
    grpc_health_v1.RegisterHealthServer(srv, s)

    // Start health check goroutine
    go s.runHealthChecks()

    lis, err := net.Listen("tcp", s.cfg.GRPCAddr)
    if err != nil {
        log.Printf("Failed to listen on %s: %v", s.cfg.GRPCAddr, err)
        return fmt.Errorf("server listen failed: %w", err)
    }

    log.Printf("head gRPC+mTLS server listening on %s", s.cfg.GRPCAddr)
    if err := srv.Serve(lis); err != nil {
        log.Printf("Server failed: %v", err)
        return fmt.Errorf("server failed: %w", err)
    }

    return nil
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
                } else {
                    s.SetHealthStatus("SERVING")
                }
            }

            // Check active requests
            active := atomic.LoadInt32(&s.activeRequests)
            if active > int32(s.maxRequests)*90/100 {
                log.Printf("High load: %d active requests (limit: %d)", active, s.maxRequests)
            }
        }
    }
}

// Health check implementation
func (s *HeadServer) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
    s.healthMutex.RLock()
    defer s.healthMutex.RUnlock()

    return &grpc_health_v1.HealthCheckResponse{
        Status: grpc_health_v1.HealthCheckResponse_ServingStatus(
            grpc_health_v1.HealthCheckResponse_ServingStatus_value[s.healthStatus],
        ),
    }, nil
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

// Обычный (не стриминговый) запрос — возвращает полный текст сразу
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

// Стриминговый запрос — настоящий SSE-совместимый стриминг
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
