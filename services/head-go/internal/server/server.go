

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

    "github.com/grpc-ecosystem/go-grpc-middleware"
    "github.com/grpc-ecosystem/go-grpc-prometheus"
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
    "github.com/prometheus/client_golang/prometheus/promhttp"
    "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/codes"
    "go.opentelemetry.io/otel/trace"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"
    "google.golang.org/grpc/status"
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
    circuitBreakerState = promauto.NewGaugeVec(
        prometheus.GaugeOpts{Name: "head_circuit_breaker_state", Help: "Circuit breaker state"},
        []string{"name", "state"},
    )
    activeConnections = promauto.NewGauge(
        prometheus.GaugeOpts{Name: "head_active_connections", Help: "Number of active gRPC connections"},
    )
)

type HeadServer struct {
    gen.UnimplementedChatServiceServer // встраиваем, чтобы не писать заглушки
    gen.UnimplementedEmbeddingServiceServer // встраиваем, чтобы не писать заглушки
    cfg        *config.Config
    model      *modelclient.ModelClient
    auth       *auth.Authenticator
    webhook    *webhook.WebhookClient
    registry   *models.ModelRegistry
    embedding  *embedding.EmbeddingService
}

func New(cfg *config.Config) *HeadServer {
    modelClient := modelclient.NewModelClient(cfg.ModelProxyAddr)
    return &HeadServer{
        cfg:       cfg,
        model:     modelClient,
        auth:      auth.NewAuthenticator(cfg.AuthConfig),
        webhook:   webhook.NewWebhookClient(cfg.WebhookConfig),
        registry:  cfg.ModelRegistry,
        embedding: embedding.NewEmbeddingService(cfg, modelClient),
    }
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

    if err := s.model.Init(initCtx); err != nil {
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
        grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(streamInterceptors...)),
        grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(unaryInterceptors...)),
    )

    // Register services
    gen.RegisterChatServiceServer(srv, s)
    grpc_prometheus.Register(srv)

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

func (s *HeadServer) Shutdown(ctx context.Context) error {
    done := make(chan struct{})
    go func() {
        s.model.Close()
        metrics.ShutdownTracing(ctx)
        close(done)
    }()

    select {
    case <-ctx.Done():
        return ctx.Err()
    case <-done:
        return nil
    }
}

// healthCheckHandler provides health check endpoint
func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
    // For now, just return a simple health check
    // In a real implementation, we would check the actual circuit breaker state
    w.WriteHeader(http.StatusOK)
    fmt.Fprintf(w, "Healthy")
}

// Обычный (не стриминговый) запрос — возвращает полный текст сразу
func (s *HeadServer) ChatCompletion(ctx context.Context, req *gen.ChatRequest) (*gen.ChatResponse, error) {
    start := time.Now()
    modelName := req.Model
    if modelName == "" {
        modelName = "gpt-4o"
    }

    // Start a span for the ChatCompletion operation
    tracer := otel.GetTracerProvider().Tracer("head-go")
    ctx, span := tracer.Start(ctx, "ChatCompletion")
    defer span.End()

    span.SetAttributes(
        attribute.String("model", modelName),
        attribute.Int("max_tokens", int(req.MaxTokens)),
        attribute.Float64("temperature", float64(req.Temperature)),
    )

    messages := make([]string, 0, len(req.Messages))
    for _, m := range req.Messages {
        messages = append(messages, m.Content)
    }

    // Check if model registry is enabled
    if s.cfg.FeaturesConfig.IsEnabled("model_registry") {
        // Use model registry to get model configuration
        modelConfig, ok := s.registry.GetModel(modelName)
        if !ok {
            requestsTotal.WithLabelValues(modelName, "error").Inc()
            span.SetStatus(codes.Error, "model not found")
            return nil, status.Errorf(codes.InvalidArgument, "model %s not found", modelName)
        }

        if !modelConfig.Enabled {
            requestsTotal.WithLabelValues(modelName, "error").Inc()
            span.SetStatus(codes.Error, "model disabled")
            return nil, status.Errorf(codes.Unavailable, "model %s is disabled", modelName)
        }

        // Check if circuit breaker is enabled
        if s.cfg.FeaturesConfig.IsEnabled("circuit_breaker") {
            // Use the model client with circuit breaker and retry logic
            text, tokens, err := s.model.Generate(ctx, modelName, messages, float32(req.Temperature), req.MaxTokens)
            if err != nil {
                requestsTotal.WithLabelValues(modelName, "error").Inc()
                span.SetStatus(codes.Error, "model error")
                span.RecordError(err)

                // Check if circuit breaker is open
                if s.model.circuitBreaker.State() == gobreaker.StateOpen {
                    span.SetStatus(codes.Error, "circuit breaker open")
                    return nil, status.Errorf(codes.Unavailable, "service unavailable due to high error rate")
                }

                return nil, status.Errorf(codes.Internal, "model error: %v", err)
            }

            requestsTotal.WithLabelValues(modelName, "ok").Inc()
            requestLatency.WithLabelValues(modelName).Observe(time.Since(start).Seconds())

            // Send webhook notification
            if s.cfg.FeaturesConfig.IsEnabled("webhook") {
                webhookData := map[string]interface{}{
                    "request_id":   req.RequestId,
                    "model":       modelName,
                    "tokens_used":  tokens,
                    "duration_ms": time.Since(start).Milliseconds(),
                }
                s.webhook.SendAsyncWebhook("chat_completion", webhookData)
            }

            return &gen.ChatResponse{
                RequestId:  req.RequestId,
                FullText:   text,
                Model:      modelName,
                Provider:  modelConfig.Provider,
                TokensUsed: int32(tokens),
            }, nil
        } else {
            // Fallback mode: direct call without circuit breaker
            text, tokens, err := s.model.Generate(ctx, modelName, messages, float32(req.Temperature), req.MaxTokens)
            if err != nil {
                requestsTotal.WithLabelValues(modelName, "error").Inc()
                span.SetStatus(codes.Error, "model error")
                span.RecordError(err)
                return nil, status.Errorf(codes.Internal, "model error: %v", err)
            }

            requestsTotal.WithLabelValues(modelName, "ok").Inc()
            requestLatency.WithLabelValues(modelName).Observe(time.Since(start).Seconds())

            // Send webhook notification
            if s.cfg.FeaturesConfig.IsEnabled("webhook") {
                webhookData := map[string]interface{}{
                    "request_id":   req.RequestId,
                    "model":       modelName,
                    "tokens_used":  tokens,
                    "duration_ms": time.Since(start).Milliseconds(),
                }
                s.webhook.SendAsyncWebhook("chat_completion", webhookData)
            }

            return &gen.ChatResponse{
                RequestId:  req.RequestId,
                FullText:   text,
                Model:      modelName,
                Provider:  modelConfig.Provider,
                TokensUsed: int32(tokens),
            }, nil
        }
    } else {
        // Fallback mode: direct call without model registry
        text, tokens, err := s.model.Generate(ctx, modelName, messages, float32(req.Temperature), req.MaxTokens)
        if err != nil {
            requestsTotal.WithLabelValues(modelName, "error").Inc()
            span.SetStatus(codes.Error, "model error")
            span.RecordError(err)
            return nil, status.Errorf(codes.Internal, "model error: %v", err)
        }

        requestsTotal.WithLabelValues(modelName, "ok").Inc()
        requestLatency.WithLabelValues(modelName).Observe(time.Since(start).Seconds())

        // Send webhook notification
        if s.cfg.FeaturesConfig.IsEnabled("webhook") {
            webhookData := map[string]interface{}{
                "request_id":   req.RequestId,
                "model":       modelName,
                "tokens_used":  tokens,
                "duration_ms": time.Since(start).Milliseconds(),
            }
            s.webhook.SendAsyncWebhook("chat_completion", webhookData)
        }

        return &gen.ChatResponse{
            RequestId:  req.RequestId,
            FullText:   text,
            Model:      modelName,
            Provider:  "litellm",
            TokensUsed: int32(tokens),
        }, nil
    }
}

// CreateEmbedding creates an embedding for the given text
func (s *HeadServer) CreateEmbedding(ctx context.Context, req *gen.EmbeddingRequest) (*gen.EmbeddingResponse, error) {
    return s.embedding.CreateEmbedding(ctx, req)
}

// CreateEmbeddingBatch creates embeddings for a batch of texts
func (s *HeadServer) CreateEmbeddingBatch(ctx context.Context, req *gen.EmbeddingBatchRequest) (*gen.EmbeddingBatchResponse, error) {
    return s.embedding.CreateEmbeddingBatch(ctx, req)
}

// Стриминговый запрос — настоящий SSE-совместимый стриминг
func (s *HeadServer) ChatCompletionStream(req *gen.ChatRequest, stream gen.ChatService_ChatCompletionStreamServer) error {
    ctx := stream.Context()
    modelName := req.Model
    if modelName == "" {
        modelName = "gpt-4o"
    }

    // Start a span for the ChatCompletionStream operation
    tracer := otel.GetTracerProvider().Tracer("head-go")
    ctx, span := tracer.Start(ctx, "ChatCompletionStream")
    defer span.End()

    span.SetAttributes(
        attribute.String("model", modelName),
        attribute.Int("max_tokens", int(req.MaxTokens)),
        attribute.Float64("temperature", float64(req.Temperature)),
    )

    messages := make([]string, 0, len(req.Messages))
    for _, m := range req.Messages {
        messages = append(messages, m.Content)
    }

    // Check if streaming feature is enabled
    if !s.cfg.FeaturesConfig.IsEnabled("streaming") {
        span.SetStatus(codes.Error, "streaming disabled")
        return status.Errorf(codes.Unimplemented, "streaming is disabled")
    }

    // Check if model registry is enabled
    if s.cfg.FeaturesConfig.IsEnabled("model_registry") {
        // Use model registry to get model configuration
        modelConfig, ok := s.registry.GetModel(modelName)
        if !ok {
            requestsTotal.WithLabelValues(modelName, "error").Inc()
            span.SetStatus(codes.Error, "model not found")
            return status.Errorf(codes.InvalidArgument, "model %s not found", modelName)
        }

        if !modelConfig.Enabled {
            requestsTotal.WithLabelValues(modelName, "error").Inc()
            span.SetStatus(codes.Error, "model disabled")
            return status.Errorf(codes.Unavailable, "model %s is disabled", modelName)
        }
    }

    // Check if circuit breaker is enabled for streaming
    if s.cfg.FeaturesConfig.IsEnabled("circuit_breaker") {
        // Use the new streaming method from the model client with circuit breaker protection
        streamCh, errCh := s.model.GenerateStream(ctx, modelName, messages, float32(req.Temperature), req.MaxTokens)

        var fullText string
        var totalTokens int32

        for {
            select {
            case chunk, ok := <-streamCh:
                if !ok {
                    // Stream closed
                    goto sendFinal
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
                    span.SetStatus(codes.Error, "stream send error")
                    span.RecordError(err)
                    return err
                }

            case err, ok := <-errCh:
                if !ok {
                    // Error channel closed, stream completed successfully
                    goto sendFinal
                }

                // Error occurred
                requestsTotal.WithLabelValues(modelName, "error").Inc()
                span.SetStatus(codes.Error, "stream error")
                span.RecordError(err)

                // Check if circuit breaker is open
                if s.model.circuitBreaker.State() == gobreaker.StateOpen {
                    span.SetStatus(codes.Error, "circuit breaker open")
                    return status.Errorf(codes.Unavailable, "service unavailable due to high error rate")
                }

                return status.Errorf(codes.Internal, "stream error: %v", err)
            }
        }

    sendFinal:
        // Финальный чанк — совместимо с OpenAI
        _ = stream.Send(&gen.ChatResponseChunk{
            RequestId:  req.RequestId,
            Chunk:      "",
            IsFinal:    true,
            Provider:   "litellm",
            TokensUsed: totalTokens,
        })

        requestsTotal.WithLabelValues(modelName, "ok").Inc()

        // Send webhook notification
        if s.cfg.FeaturesConfig.IsEnabled("webhook") {
            webhookData := map[string]interface{}{
                "request_id":   req.RequestId,
                "model":       modelName,
                "tokens_used":  totalTokens,
                "duration_ms": time.Since(time.Now().Add(-time.Since(ctx.Value("start_time").(time.Time)))).Milliseconds(),
            }
            s.webhook.SendAsyncWebhook("chat_completion_stream", webhookData)
        }

        return nil
    } else {
        // Fallback mode: direct streaming without circuit breaker
        streamCh, errCh := s.model.GenerateStream(ctx, modelName, messages, float32(req.Temperature), req.MaxTokens)

        var fullText string
        var totalTokens int32

        for {
            select {
            case chunk, ok := <-streamCh:
                if !ok {
                    // Stream closed
                    goto sendFinalNoCB
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
                    span.SetStatus(codes.Error, "stream send error")
                    span.RecordError(err)
                    return err
                }

            case err, ok := <-errCh:
                if !ok {
                    // Error channel closed, stream completed successfully
                    goto sendFinalNoCB
                }

                // Error occurred
                requestsTotal.WithLabelValues(modelName, "error").Inc()
                span.SetStatus(codes.Error, "stream error")
                span.RecordError(err)
                return status.Errorf(codes.Internal, "stream error: %v", err)
            }
        }

    sendFinalNoCB:
        // Финальный чанк — совместимо с OpenAI
        _ = stream.Send(&gen.ChatResponseChunk{
            RequestId:  req.RequestId,
            Chunk:      "",
            IsFinal:    true,
            Provider:   "litellm",
            TokensUsed: totalTokens,
        })

        requestsTotal.WithLabelValues(modelName, "ok").Inc()

        // Send webhook notification
        if s.cfg.FeaturesConfig.IsEnabled("webhook") {
            webhookData := map[string]interface{}{
                "request_id":   req.RequestId,
                "model":       modelName,
                "tokens_used":  totalTokens,
                "duration_ms": time.Since(time.Now().Add(-time.Since(ctx.Value("start_time").(time.Time)))).Milliseconds(),
            }
            s.webhook.SendAsyncWebhook("chat_completion_stream", webhookData)
        }

        return nil
    }
}

