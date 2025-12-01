package config
import (
    "os"
    "time"
)

// Config holds the main configuration
type Config struct {
    GRPCAddr        string
    MetricsPort     int
    ModelProxyAddr  string
    AuthConfig      AuthConfig
    FeaturesConfig   *FeaturesConfig
    WebhookConfig   WebhookConfig
    ModelRegistry   *ModelRegistry
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
    JWTSecret       string
    TokenExpiration time.Duration
}

// WebhookConfig holds webhook configuration
type WebhookConfig struct {
    URL            string
    Timeout         time.Duration
    MaxRetries      int
    RetryDelay      time.Duration
    Enabled         bool
}

// Load loads the configuration from environment variables
func Load() *Config {
    return &Config{
        GRPCAddr:       ":50055",
        MetricsPort:    9001,
        ModelProxyAddr: os.Getenv("MODEL_ADDR"),
        AuthConfig: AuthConfig{
            JWTSecret:       getEnv("JWT_SECRET", "default-secret-key"),
            TokenExpiration: 24 * time.Hour,
        },
        FeaturesConfig: DefaultFeatures(),
        WebhookConfig: WebhookConfig{
            URL:           getEnv("WEBHOOK_URL", "http://localhost:8080/webhook"),
            Timeout:       5 * time.Second,
            MaxRetries:    3,
            RetryDelay:    1 * time.Second,
            Enabled:       true,
        },
        ModelRegistry: DefaultModelRegistry(),
    }
}

// getEnv returns the environment variable value or a default
func getEnv(key, defaultValue string) string {
    if value, exists := os.LookupEnv(key); exists {
        return value
    }
    return defaultValue
}
