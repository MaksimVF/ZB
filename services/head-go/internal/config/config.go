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
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
    JWTSecret       string
    TokenExpiration time.Duration
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
    }
}

// getEnv returns the environment variable value or a default
func getEnv(key, defaultValue string) string {
    if value, exists := os.LookupEnv(key); exists {
        return value
    }
    return defaultValue
}
