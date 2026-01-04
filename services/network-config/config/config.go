package config

import (
	"os"
	"strconv"
)

// Config represents the application configuration
type Config struct {
	Server   ServerConfig   `json:"server"`
	Redis    RedisConfig    `json:"redis"`
	Auth     AuthConfig     `json:"auth"`
	Network  NetworkConfig  `json:"network"`
	Logging  LoggingConfig  `json:"logging"`
	Security SecurityConfig `json:"security"`
}

// ServerConfig represents server configuration
type ServerConfig struct {
	Port         string `json:"port"`
	ReadTimeout  int    `json:"read_timeout"`
	WriteTimeout int    `json:"write_timeout"`
	IdleTimeout  int    `json:"idle_timeout"`
	Env          string `json:"env"`
}

// RedisConfig represents Redis configuration
type RedisConfig struct {
	Addr     string `json:"addr"`
	Password string `json:"password"`
	DB       int    `json:"db"`
	PoolSize int    `json:"pool_size"`
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	Enabled    bool   `json:"enabled"`
	SecretKey  string `json:"secret_key"`
	Algorithm  string `json:"algorithm"`
	Expiration int    `json:"expiration"`
}

// NetworkConfig represents network configuration
type NetworkConfig struct {
	Mode          string              `json:"mode"`
	Endpoints     []EndpointConfig    `json:"endpoints"`
	LoadBalancing LoadBalancingConfig `json:"load_balancing"`
	RetryPolicy   RetryPolicy         `json:"retry_policy"`
	RateLimits    RateLimits          `json:"rate_limits"`
}

// EndpointConfig represents endpoint configuration
type EndpointConfig struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	Weight      int    `json:"weight"`
	HealthCheck bool   `json:"health_check"`
}

// LoadBalancingConfig represents load balancing configuration
type LoadBalancingConfig struct {
	Strategy      string `json:"strategy"`
	HealthCheck   bool   `json:"health_check"`
	CheckInterval int    `json:"check_interval"`
}

// RetryPolicy represents retry policy configuration
type RetryPolicy struct {
	MaxRetries    int `json:"max_retries"`
	BackoffFactor int `json:"backoff_factor"`
	MaxBackoff    int `json:"max_backoff"`
}

// RateLimits represents rate limiting configuration
type RateLimits struct {
	Enabled        bool `json:"enabled"`
	RequestsPerSec int  `json:"requests_per_sec"`
	Burst          int  `json:"burst"`
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	Level  string `json:"level"`
	Format string `json:"format"`
	Output string `json:"output"`
}

// SecurityConfig represents security configuration
type SecurityConfig struct {
	CORSEnabled    bool            `json:"cors_enabled"`
	AllowedOrigins []string        `json:"allowed_origins"`
	RateLimit      RateLimitConfig `json:"rate_limit"`
}

// RateLimitConfig represents rate limit configuration
type RateLimitConfig struct {
	Enabled                bool   `json:"enabled"`
	Requests               int    `json:"requests"`
	Window                 string `json:"window"`
	SkipSuccessfulRequests bool   `json:"skip_successful_requests"`
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:         getEnv("PORT", "50060"),
			ReadTimeout:  getEnvAsInt("READ_TIMEOUT", 10),
			WriteTimeout: getEnvAsInt("WRITE_TIMEOUT", 10),
			IdleTimeout:  getEnvAsInt("IDLE_TIMEOUT", 60),
			Env:          getEnv("ENV", "production"),
		},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "redis:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvAsInt("REDIS_DB", 0),
			PoolSize: getEnvAsInt("REDIS_POOL_SIZE", 10),
		},
		Auth: AuthConfig{
			Enabled:    getEnvAsBool("AUTH_ENABLED", true),
			SecretKey:  getEnv("AUTH_SECRET_KEY", "default-secret-key-change-me"),
			Algorithm:  getEnv("AUTH_ALGORITHM", "HS256"),
			Expiration: getEnvAsInt("AUTH_EXPIRATION", 3600),
		},
		Network: NetworkConfig{
			Mode: getEnv("NETWORK_MODE", "direct"),
			LoadBalancing: LoadBalancingConfig{
				Strategy:      getEnv("LB_STRATEGY", "round_robin"),
				HealthCheck:   getEnvAsBool("LB_HEALTH_CHECK", true),
				CheckInterval: getEnvAsInt("LB_CHECK_INTERVAL", 30),
			},
			RetryPolicy: RetryPolicy{
				MaxRetries:    getEnvAsInt("RETRY_MAX_RETRIES", 3),
				BackoffFactor: getEnvAsInt("RETRY_BACKOFF_FACTOR", 2),
				MaxBackoff:    getEnvAsInt("RETRY_MAX_BACKOFF", 10),
			},
			RateLimits: RateLimits{
				Enabled:        getEnvAsBool("RATE_LIMIT_ENABLED", true),
				RequestsPerSec: getEnvAsInt("RATE_LIMIT_RPS", 100),
				Burst:          getEnvAsInt("RATE_LIMIT_BURST", 200),
			},
		},
		Logging: LoggingConfig{
			Level:  getEnv("LOG_LEVEL", "info"),
			Format: getEnv("LOG_FORMAT", "json"),
			Output: getEnv("LOG_OUTPUT", "stdout"),
		},
		Security: SecurityConfig{
			CORSEnabled:    getEnvAsBool("CORS_ENABLED", true),
			AllowedOrigins: []string{getEnv("CORS_ALLOWED_ORIGINS", "*")},
			RateLimit: RateLimitConfig{
				Enabled:                getEnvAsBool("SECURITY_RATE_LIMIT_ENABLED", true),
				Requests:               getEnvAsInt("SECURITY_RATE_LIMIT_REQUESTS", 1000),
				Window:                 getEnv("SECURITY_RATE_LIMIT_WINDOW", "1h"),
				SkipSuccessfulRequests: getEnvAsBool("SECURITY_RATE_LIMIT_SKIP_SUCCESS", false),
			},
		},
	}
}

// Helper functions for environment variable parsing
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(name string, defaultValue int) int {
	valueStr := getEnv(name, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return defaultValue
}

func getEnvAsBool(name string, defaultValue bool) bool {
	valueStr := getEnv(name, "")
	if value, err := strconv.ParseBool(valueStr); err == nil {
		return value
	}
	return defaultValue
}
