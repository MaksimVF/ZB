






package embedding

import (
    "context"
    "fmt"
    "time"

    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/trace"
    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"

    gen "github.com/yourorg/head/gen"
    model "github.com/yourorg/head/gen_model"
    "github.com/yourorg/head/internal/config"
    "github.com/yourorg/head/internal/metrics"
    "github.com/yourorg/head/internal/models"
    "github.com/yourorg/head/internal/webhook"
)

// EmbeddingService handles embedding requests
type EmbeddingService struct {
    cfg        *config.Config
    model      model.ModelServiceClient
    registry   *models.ModelRegistry
    webhook    *webhook.WebhookClient
}

// NewEmbeddingService creates a new embedding service
func NewEmbeddingService(cfg *config.Config, model model.ModelServiceClient) *EmbeddingService {
    return &EmbeddingService{
        cfg:      cfg,
        model:    model,
        registry: cfg.ModelRegistry,
        webhook:  webhook.NewWebhookClient(cfg.WebhookConfig),
    }
}

// CreateEmbedding creates an embedding for the given text
func (s *EmbeddingService) CreateEmbedding(ctx context.Context, req *gen.EmbeddingRequest) (*gen.EmbeddingResponse, error) {
    start := time.Now()

    // Start a span for the embedding operation
    tracer := otel.GetTracerProvider().Tracer("head-go")
    ctx, span := tracer.Start(ctx, "EmbeddingService.CreateEmbedding")
    defer span.End()

    span.SetAttributes(
        attribute.String("model", req.Model),
        attribute.Int("input_length", len(req.Text)),
    )

    // Check if model registry is enabled
    if s.cfg.FeaturesConfig.IsEnabled("model_registry") {
        // Use model registry to get model configuration
        modelConfig, ok := s.registry.GetModel(req.Model)
        if !ok {
            metrics.requestsTotal.WithLabelValues(req.Model, "error").Inc()
            span.SetStatus(codes.Error, "model not found")
            return nil, status.Errorf(codes.InvalidArgument, "model %s not found", req.Model)
        }

        if !modelConfig.Enabled {
            metrics.requestsTotal.WithLabelValues(req.Model, "error").Inc()
            span.SetStatus(codes.Error, "model disabled")
            return nil, status.Errorf(codes.Unavailable, "model %s is disabled", req.Model)
        }
    }

    // Create embedding request for the model service
    modelReq := &model.GenRequest{
        RequestId:   req.RequestId,
        Model:       req.Model,
        Messages:    []string{req.Text},
        Temperature: 0.0, // Embeddings typically don't use temperature
        MaxTokens:   0,   // Not applicable for embeddings
        Stream:      false,
    }

    // Call the model service
    modelResp, err := s.model.Generate(ctx, modelReq)
    if err != nil {
        metrics.requestsTotal.WithLabelValues(req.Model, "error").Inc()
        span.SetStatus(codes.Error, "model error")
        span.RecordError(err)
        return nil, status.Errorf(codes.Internal, "embedding error: %v", err)
    }

    // Parse embedding response
    var embedding []float32
    // Assuming the model returns a JSON array of floats
    // In a real implementation, this would be properly parsed
    // For now, we'll simulate with some dummy data
    embedding = []float32{0.1, 0.2, 0.3, 0.4, 0.5} // Simulated embedding

    metrics.requestsTotal.WithLabelValues(req.Model, "ok").Inc()
    metrics.requestLatency.WithLabelValues(req.Model).Observe(time.Since(start).Seconds())

    // Send webhook notification
    if s.cfg.FeaturesConfig.IsEnabled("webhook") {
        webhookData := map[string]interface{}{
            "request_id":   req.RequestId,
            "model":       req.Model,
            "duration_ms": time.Since(start).Milliseconds(),
            "embedding_size": len(embedding),
        }
        s.webhook.SendAsyncWebhook("embedding_created", webhookData)
    }

    return &gen.EmbeddingResponse{
        RequestId: req.RequestId,
        Model:     req.Model,
        Embedding: embedding,
        Dimensions: int32(len(embedding)),
    }, nil
}

// CreateEmbeddingBatch creates embeddings for a batch of texts
func (s *EmbeddingService) CreateEmbeddingBatch(ctx context.Context, req *gen.EmbeddingBatchRequest) (*gen.EmbeddingBatchResponse, error) {
    start := time.Now()

    // Start a span for the batch embedding operation
    tracer := otel.GetTracerProvider().Tracer("head-go")
    ctx, span := tracer.Start(ctx, "EmbeddingService.CreateEmbeddingBatch")
    defer span.End()

    span.SetAttributes(
        attribute.String("model", req.Model),
        attribute.Int("batch_size", len(req.Texts)),
    )

    // Check if model registry is enabled
    if s.cfg.FeaturesConfig.IsEnabled("model_registry") {
        // Use model registry to get model configuration
        modelConfig, ok := s.registry.GetModel(req.Model)
        if !ok {
            metrics.requestsTotal.WithLabelValues(req.Model, "error").Inc()
            span.SetStatus(codes.Error, "model not found")
            return nil, status.Errorf(codes.InvalidArgument, "model %s not found", req.Model)
        }

        if !modelConfig.Enabled {
            metrics.requestsTotal.WithLabelValues(req.Model, "error").Inc()
            span.SetStatus(codes.Error, "model disabled")
            return nil, status.Errorf(codes.Unavailable, "model %s is disabled", req.Model)
        }
    }

    // Create batch embedding request
    modelReq := &model.GenRequest{
        RequestId:   req.RequestId,
        Model:       req.Model,
        Messages:    req.Texts,
        Temperature: 0.0,
        MaxTokens:   0,
        Stream:      false,
    }

    // Call the model service
    modelResp, err := s.model.Generate(ctx, modelReq)
    if err != nil {
        metrics.requestsTotal.WithLabelValues(req.Model, "error").Inc()
        span.SetStatus(codes.Error, "model error")
        span.RecordError(err)
        return nil, status.Errorf(codes.Internal, "embedding batch error: %v", err)
    }

    // Parse batch embedding response
    batchEmbeddings := make([]*gen.Embedding, len(req.Texts))
    for i, text := range req.Texts {
        // Simulated embedding for each text
        embedding := &gen.Embedding{
            Text:       text,
            Vector:     []float32{0.1, 0.2, 0.3, 0.4, 0.5}, // Simulated
            Dimensions: 5,
        }
        batchEmbeddings[i] = embedding
    }

    metrics.requestsTotal.WithLabelValues(req.Model, "ok").Inc()
    metrics.requestLatency.WithLabelValues(req.Model).Observe(time.Since(start).Seconds())

    // Send webhook notification
    if s.cfg.FeaturesConfig.IsEnabled("webhook") {
        webhookData := map[string]interface{}{
            "request_id":   req.RequestId,
            "model":       req.Model,
            "duration_ms": time.Since(start).Milliseconds(),
            "batch_size":  len(req.Texts),
        }
        s.webhook.SendAsyncWebhook("embedding_batch_created", webhookData)
    }

    return &gen.EmbeddingBatchResponse{
        RequestId: req.RequestId,
        Model:     req.Model,
        Embeddings: batchEmbeddings,
    }, nil
}





