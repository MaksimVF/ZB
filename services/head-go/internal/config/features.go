


package config

import (
    "sync"
)

// Feature represents a feature toggle
type Feature struct {
    Name        string
    Enabled     bool
    Description string
}

// FeaturesConfig holds feature toggle configuration
type FeaturesConfig struct {
    mu       sync.RWMutex
    features map[string]*Feature
}

// NewFeaturesConfig creates a new features configuration
func NewFeaturesConfig() *FeaturesConfig {
    return &FeaturesConfig{
        features: make(map[string]*Feature),
    }
}

// AddFeature adds a new feature toggle
func (f *FeaturesConfig) AddFeature(name, description string, enabled bool) {
    f.mu.Lock()
    defer f.mu.Unlock()

    f.features[name] = &Feature{
        Name:        name,
        Enabled:     enabled,
        Description: description,
    }
}

// IsEnabled checks if a feature is enabled
func (f *FeaturesConfig) IsEnabled(name string) bool {
    f.mu.RLock()
    defer f.mu.RUnlock()

    if feature, ok := f.features[name]; ok {
        return feature.Enabled
    }
    return false
}

// SetEnabled enables or disables a feature
func (f *FeaturesConfig) SetEnabled(name string, enabled bool) {
    f.mu.Lock()
    defer f.mu.Unlock()

    if feature, ok := f.features[name]; ok {
        feature.Enabled = enabled
    }
}

// GetAllFeatures returns all feature toggles
func (f *FeaturesConfig) GetAllFeatures() map[string]*Feature {
    f.mu.RLock()
    defer f.mu.RUnlock()

    featuresCopy := make(map[string]*Feature)
    for k, v := range f.features {
        featuresCopy[k] = v
    }
    return featuresCopy
}

// DefaultFeatures returns the default feature configuration
func DefaultFeatures() *FeaturesConfig {
    features := NewFeaturesConfig()

    // Add default features
    features.AddFeature("streaming", "Enable streaming responses", true)
    features.AddFeature("advanced_metrics", "Enable advanced metrics collection", true)
    features.AddFeature("circuit_breaker", "Enable circuit breaker protection", true)
    features.AddFeature("retry_logic", "Enable retry logic for failed requests", true)
    features.AddFeature("authentication", "Enable token-based authentication", true)
    features.AddFeature("webhook", "Enable webhook notifications", true)
    features.AddFeature("model_registry", "Enable model registry and A/B testing", true)
    features.AddFeature("ab_testing", "Enable A/B testing for models", true)
    features.AddFeature("embedding", "Enable embedding functionality", true)

    return features
}


