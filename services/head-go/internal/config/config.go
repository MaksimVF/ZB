package config
import (
	"errors"
	"fmt"
	"os"
	"time"
)

// Config holds the main configuration
type Config struct {
	GRPCAddr         string        `json:"grpc_addr"`
	MetricsPort      int           `json:"metrics_port"`
	ModelProxyAddr   string        `json:"model_proxy_addr"`
	RedisAddr        string        `json:"redis_addr"`
	NetworkConfig    NetworkConfig `json:"network_config"`
	AuthConfig       AuthConfig    `json:"auth_config"`
	FeaturesConfig   *FeaturesConfig `json:"features_config"`
	WebhookConfig    WebhookConfig `json:"webhook_config"`
	ModelRegistry    *ModelRegistry `json:"model_registry"`
	
	// Service configuration
	ReloadInterval   time.Duration `json:"reload_interval"`
	MaxRetries       int           `json:"max_retries"`
	RequestTimeout   time.Duration `json:"request_timeout"`
	
	// Logging configuration
	LogLevel         string        `json:"log_level"`
}

// NetworkConfig holds network-related configuration
type NetworkConfig struct {
	Enabled         bool   `json:"enabled"`
	Interface       string `json:"interface"`
	MTU             int    `json:"mtu"`
	EnableIPv6      bool   `json:"enable_ipv6"`
	DHCPTimeout     int    `json:"dhcp_timeout"`
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	JWTSecret       string        `json:"jwt_secret"`
	TokenExpiration time.Duration `json:"token_expiration"`
	JWKSURL         string        `json:"jwks_url"`
	Issuer          string        `json:"issuer"`
	Audience        string        `json:"audience"`
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
// Load loads the configuration from environment variables and defaults
func Load() *Config {
	cfg := &Config{
		GRPCAddr:         getEnv("GRPC_ADDR", ":50055"),
		MetricsPort:      getEnvAsInt("METRICS_PORT", 9001),
		ModelProxyAddr:   getEnv("MODEL_ADDR", ""),
		RedisAddr:        getEnv("REDIS_ADDR", "redis:6379"),
		NetworkConfig: NetworkConfig{
			Enabled:      getEnvAsBool("NETWORK_ENABLED", true),
			Interface:    getEnv("NETWORK_INTERFACE", "eth0"),
			MTU:          getEnvAsInt("NETWORK_MTU", 1500),
			EnableIPv6:   getEnvAsBool("NETWORK_ENABLE_IPV6", true),
			DHCPTimeout:  getEnvAsInt("NETWORK_DHCP_TIMEOUT", 30),
		},
		AuthConfig: AuthConfig{
			JWTSecret:       getEnv("JWT_SECRET", "default-secret-key-change-in-production"),
			TokenExpiration: 24 * time.Hour,
			JWKSURL:         getEnv("JWT_JWKS_URL", ""),
			Issuer:          getEnv("JWT_ISSUER", ""),
			Audience:        getEnv("JWT_AUDIENCE", ""),
		},
		FeaturesConfig:  DefaultFeatures(),
		WebhookConfig: WebhookConfig{
			URL:           getEnv("WEBHOOK_URL", "http://localhost:8080/webhook"),
			Timeout:       5 * time.Second,
			MaxRetries:    3,
			RetryDelay:    1 * time.Second,
			Enabled:       getEnvAsBool("WEBHOOK_ENABLED", true),
		},
		ModelRegistry: DefaultModelRegistry(),
		
		// Service configuration
		ReloadInterval:  getEnvAsDuration("RELOAD_INTERVAL", 10*time.Second),
		MaxRetries:      getEnvAsInt("MAX_RETRIES", 3),
		RequestTimeout:  getEnvAsDuration("REQUEST_TIMEOUT", 30*time.Second),
		
		// Logging configuration
		LogLevel: getEnv("LOG_LEVEL", "info"),
	}
	
	// Validate configuration
	if err := cfg.Validate(); err != nil {
		// In a real implementation, you might want to log this and continue
		// or fail fast depending on your requirements
		fmt.Printf("Configuration validation warning: %v\n", err)
	}
	
	return cfg
}

// Validate validates the configuration
func (c *Config) Validate() error {
	var errs []error
	
	if c.GRPCAddr == "" {
		errs = append(errs, &ConfigError{Field: "GRPCAddr", Value: c.GRPCAddr, Msg: "cannot be empty"})
	}
	
	if c.MetricsPort <= 0 || c.MetricsPort > 65535 {
		errs = append(errs, &ConfigError{Field: "MetricsPort", Value: c.MetricsPort, Msg: "must be between 1 and 65535"})
	}
	
	if c.RedisAddr == "" {
		errs = append(errs, &ConfigError{Field: "RedisAddr", Value: c.RedisAddr, Msg: "cannot be empty"})
	}
	
	if c.AuthConfig.JWTSecret == "" {
		errs = append(errs, &ConfigError{Field: "JWTSecret", Value: c.AuthConfig.JWTSecret, Msg: "cannot be empty"})
	}
	
	if c.AuthConfig.JWTSecret == "default-secret-key-change-in-production" {
		errs = append(errs, &ConfigError{Field: "JWTSecret", Value: c.AuthConfig.JWTSecret, Msg: "must be changed from default value in production"})
	}
	
	if c.ReloadInterval <= 0 {
		errs = append(errs, &ConfigError{Field: "ReloadInterval", Value: c.ReloadInterval, Msg: "must be positive"})
	}
	
	if c.MaxRetries < 0 {
		errs = append(errs, &ConfigError{Field: "MaxRetries", Value: c.MaxRetries, Msg: "must be non-negative"})
	}
	
	if c.RequestTimeout <= 0 {
		errs = append(errs, &ConfigError{Field: "RequestTimeout", Value: c.RequestTimeout, Msg: "must be positive"})
	}
	
	if len(errs) > 0 {
		return fmt.Errorf("configuration validation failed: %v", errs)
	}
	
	return nil
}

// getEnv returns the environment variable value or a default
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// getEnvAsInt returns the environment variable value as int or a default
func getEnvAsInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := parseInt(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvAsBool returns the environment variable value as bool or a default
func getEnvAsBool(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		if boolValue, err := parseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

// getEnvAsDuration returns the environment variable value as duration or a default
func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	if value, exists := os.LookupEnv(key); exists {
		if durationValue, err := parseDuration(value); err == nil {
			return durationValue
		}
	}
	return defaultValue
}

// parseInt parses a string to int
func parseInt(s string) (int, error) {
	var value int
	_, err := fmt.Sscanf(s, "%d", &value)
	return value, err
}

// parseBool parses a string to bool
func parseBool(s string) (bool, error) {
	switch s {
	case "true", "1", "yes", "on":
		return true, nil
	case "false", "0", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value: %s", s)
	}
}

// parseDuration parses a string to time.Duration
func parseDuration(s string) (time.Duration, error) {
	return time.ParseDuration(s)
}

// ConfigError represents a configuration validation error
type ConfigError struct {
	Field string
	Value interface{}
	Msg   string
}

func (e ConfigError) Error() string {
	return fmt.Sprintf("configuration error in %s: %s (value: %v)", e.Field, e.Msg, e.Value)
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Value   interface{}
	Message string
	Cause   error
}

func (e ValidationError) Error() string {
	baseMsg := fmt.Sprintf("validation error for %s: %s", e.Field, e.Message)
	if e.Value != nil {
		baseMsg += fmt.Sprintf(" (value: %v)", e.Value)
	}
	if e.Cause != nil {
		baseMsg += fmt.Sprintf(": %v", e.Cause)
	}
	return baseMsg
}
