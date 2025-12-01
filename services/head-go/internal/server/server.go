



package server

import (
    "crypto/x509"
    "fmt"
    "context"
    "io"
    "log"
    "net"
    "time"
    "sync"
    "sync/atomic"
    "os"

    gen "github.com/yourorg/head/gen" // сюда генерируются chat.proto
    model "github.com/yourorg/head/gen_model" // сюда генерируются model.proto
    "github.com/yourorg/head/internal/config"
    modelclient "github.com/yourorg/head/internal/providers"

    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
    "google.golang.org/grpc/keepalive"
    "google.golang.org/grpc/health/grpc_health_v1"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
    "github.com/afex/hystrix-go/hystrix"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
    "go.opentelemetry.io/otel/sdk/resource"
    "go.opentelemetry.io/otel/sdk/trace"
    "go.opentelemetry.io/otel/trace/noop"
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
    cfg            *config.Config
    model          *modelclient.ModelClient
    shutdown        bool
    shutdownMutex   sync.RWMutex
    activeRequests  int32
    maxRequests     int
    healthStatus    string
    healthMutex     sync.RWMutex
}

func New(cfg *config.Config) *HeadServer {
    return &HeadServer{
        cfg:            cfg,
        model:          modelclient.NewModelClient(cfg.ModelProxyAddr),
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
    ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
    defer cancel()

    // Initialize circuit breakers
    hystrix.ConfigureCommand("model_proxy", hystrix.CommandConfig{
        Timeout:                5000, // 5 seconds
        MaxConcurrentRequests:  100,
        ErrorPercentThreshold:   25,
    })

    // Try to initialize model client with better error handling
    if err := s.model.Init(ctx); err != nil {
        log.Printf("Failed to initialize model client: %v", err)
        // Check if it is a certificate error
        if _, ok := err.(x509.UnknownAuthorityError); ok {
            log.Printf("Certificate authority error - check CA configuration")
        }
        // Check if it is a connection timeout
        if ctx.Err() == context.DeadlineExceeded {
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

    srv := grpc.NewServer(
        grpc.Creds(creds),
        grpc.KeepaliveParams(keepaliveParams),
        grpc.KeepaliveEnforcementPolicy(keepalivePolicy),
    )
    gen.RegisterChatServiceServer(srv, s)

    // Register health service
    grpc_health_v1.RegisterHealthServer(srv, s)

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

    // Update active connections metric
    activeConnections.Set(float64(atomic.LoadInt32(&s.activeRequests)))

    messages := make([]string, 0, len(req.Messages))
    for _, m := range req.Messages {
        messages = append(messages, m.Content)
    }

    // Use enhanced model client with streaming
    streamCh, errCh := s.model.GenerateStream(ctx, modelName, messages, req.Temperature, req.MaxTokens)

    var fullText string
    var totalTokens int32

    for {
        select {
        case chunk, ok := <-streamCh:
            if !ok {
                // Stream channel closed
                goto finalize
            }

            fullText += chunk.Text
            if chunk.TokensUsed > totalTokens {
                totalTokens = chunk.TokensUsed
            }

            // Отправляем клиенту (tail → пользователь) каждый кусок
            if err := stream.Send(&gen.ChatResponseChunk{
                RequestId:  req.RequestId,
                Chunk:      chunk.Text,
                IsFinal:    false,
                Provider:   "litellm",
                TokensUsed: chunk.TokensUsed,
            }); err != nil {
                return err
            }

        case err, ok := <-errCh:
            if !ok {
                // Error channel closed
                goto finalize
            }
            if err != nil {
                requestErrors.WithLabelValues(modelName, "stream_recv_error").Inc()
                return status.Errorf(codes.Internal, "stream error: %v", err)
            }
        }
    }

finalize:
    // Финальный чанк — совместимо с OpenAI
    _ = stream.Send(&gen.ChatResponseChunk{
        RequestId:  req.RequestId,
        Chunk:      "",
        IsFinal:    true,
        Provider:   "litellm",
        TokensUsed: totalTokens,
    })

    requestErrors.WithLabelValues(modelName, "ok").Inc()
    return nil
}

func (s *HeadServer) Shutdown(ctx context.Context) error {
    done := make(chan struct{})
    go func() {
        s.model.Close()
        close(done)
    }()

    select {
    case <-ctx.Done():
        return ctx.Err()
    case <-done:
        return nil
    }
}

// loadServerTLSCredentials loads gRPC TLS credentials with proper certificate validation
func loadServerTLSCredentials() (credentials.TransportCredentials, error) {
    // Load server certificate and key
    serverCert, err := tls.LoadX509KeyPair("/certs/head.pem", "/certs/head-key.pem")
    if err != nil {
        return nil, fmt.Errorf("failed to load server certificate: %w", err)
    }

    // Load CA certificate for client verification
    caCert, err := os.ReadFile("/certs/ca.pem")
    if err != nil {
        return nil, fmt.Errorf("failed to load CA certificate: %w", err)
    }

    certPool := x509.NewCertPool()
    if !certPool.AppendCertsFromPEM(caCert) {
        return nil, fmt.Errorf("failed to add CA certificate to pool")
    }

    // Create TLS config with proper validation
    tlsConfig := &tls.Config{
        Certificates: []tls.Certificate{serverCert},
        ClientAuth:   tls.RequireAndVerifyClientCert,
        ClientCAs:    certPool,
        MinVersion:   tls.VersionTLS12,
    }

    return credentials.NewTLS(tlsConfig), nil
}

