





package models

import (
    "context"
    "math/rand"
    "sync"
    "time"

    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/trace"
)

// ModelConfig holds model configuration
type ModelConfig struct {
    Name        string
    Provider    string
    Endpoint    string
    APIKey      string
    Weight      int
    Enabled     bool
    MaxTokens   int
    Temperature  float32
}

// ModelRegistry manages available models
type ModelRegistry struct {
    mu      sync.RWMutex
    models  map[string]*ModelConfig
    weights []string
}

// NewModelRegistry creates a new model registry
func NewModelRegistry() *ModelRegistry {
    return &ModelRegistry{
        models: make(map[string]*ModelConfig),
    }
}

// RegisterModel adds a model to the registry
func (r *ModelRegistry) RegisterModel(config ModelConfig) {
    r.mu.Lock()
    defer r.mu.Unlock()

    r.models[config.Name] = &config

    // Update weights
    for i := 0; i < config.Weight; i++ {
        r.weights = append(r.weights, config.Name)
    }
}

// GetModel returns a model configuration
func (r *ModelRegistry) GetModel(name string) (*ModelConfig, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()

    model, ok := r.models[name]
    return model, ok
}

// GetRandomModel returns a random model based on weights
func (r *ModelRegistry) GetRandomModel() (*ModelConfig, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()

    if len(r.weights) == 0 {
        return nil, false
    }

    idx := rand.Intn(len(r.weights))
    modelName := r.weights[idx]
    return r.models[modelName], true
}

// GetAllModels returns all registered models
func (r *ModelRegistry) GetAllModels() map[string]*ModelConfig {
    r.mu.RLock()
    defer r.mu.RUnlock()

    modelsCopy := make(map[string]*ModelConfig)
    for k, v := range r.models {
        modelsCopy[k] = v
    }
    return modelsCopy
}

// EnableModel enables a model
func (r *ModelRegistry) EnableModel(name string) {
    r.mu.Lock()
    defer r.mu.Unlock()

    if model, ok := r.models[name]; ok {
        model.Enabled = true
    }
}

// DisableModel disables a model
func (r *ModelRegistry) DisableModel(name string) {
    r.mu.Lock()
    defer r.mu.Unlock()

    if model, ok := r.models[name]; ok {
        model.Enabled = false
    }
}

// IsModelEnabled checks if a model is enabled
func (r *ModelRegistry) IsModelEnabled(name string) bool {
    r.mu.RLock()
    defer r.mu.RUnlock()

    if model, ok := r.models[name]; ok {
        return model.Enabled
    }
    return false
}

// SelectModelForABTest selects a model for A/B testing
func (r *ModelRegistry) SelectModelForABTest(ctx context.Context, testName string, userID string) (*ModelConfig, bool) {
    // Start a span for the model selection
    tracer := otel.GetTracerProvider().Tracer("head-go")
    ctx, span := tracer.Start(ctx, "ModelRegistry.SelectModelForABTest")
    defer span.End()

    span.SetAttributes(
        attribute.String("test_name", testName),
        attribute.String("user_id", userID),
    )

    r.mu.RLock()
    defer r.mu.RUnlock()

    // Simple A/B testing: alternate between available models
    var availableModels []*ModelConfig
    for _, model := range r.models {
        if model.Enabled {
            availableModels = append(availableModels, model)
        }
    }

    if len(availableModels) == 0 {
        span.SetStatus(trace.StatusCodeError, "no models available")
        return nil, false
    }

    // Use a simple hash of userID to determine model assignment
    hash := 0
    for _, char := range userID {
        hash = (hash*31 + int(char)) % len(availableModels)
    }

    selectedModel := availableModels[hash]
    span.SetAttributes(attribute.String("selected_model", selectedModel.Name))

    return selectedModel, true
}

// DefaultModelRegistry returns a registry with default models
func DefaultModelRegistry() *ModelRegistry {
    registry := NewModelRegistry()

    // Add default models
    registry.RegisterModel(ModelConfig{
        Name:       "gpt-4o",
        Provider:   "litellm",
        Endpoint:   "https://api.litellm.com/v1/models/gpt-4o",
        APIKey:     "default-api-key",
        Weight:     5,
        Enabled:    true,
        MaxTokens:  4096,
        Temperature: 0.7,
    })

    registry.RegisterModel(ModelConfig{
        Name:       "gpt-3.5-turbo",
        Provider:   "litellm",
        Endpoint:   "https://api.litellm.com/v1/models/gpt-3.5-turbo",
        APIKey:     "default-api-key",
        Weight:     3,
        Enabled:    true,
        MaxTokens:  4096,
        Temperature: 0.7,
    })

    registry.RegisterModel(ModelConfig{
        Name:       "claude-2",
        Provider:   "litellm",
        Endpoint:   "https://api.litellm.com/v1/models/claude-2",
        APIKey:     "default-api-key",
        Weight:     2,
        Enabled:    true,
        MaxTokens:  4096,
        Temperature: 0.7,
    })

    return registry
}





