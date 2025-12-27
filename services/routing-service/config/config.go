package config

import (
	"time"

	"github.com/go-redis/redis/v9"
)

// Config holds all configuration for the routing service
type Config struct {
	// Server configuration
	GRPCPort    string
	HTTPPort    string
	
	// Redis configuration
	Redis *redis.Options
	
	// NATS configuration
	NATSURL string
	
	// Security configuration
	JWTConfig JWTConfig
	
	// Rate limiting configuration
	RateLimit RateLimitConfig
	
	// Circuit breaker configuration
	CircuitBreaker CircuitBreakerConfig
	
	// Cache configuration
	Cache CacheConfig
	
	// External services configuration
	ExternalServices map[string]string
	
	// TLS configuration
	TLS TLSConfig
}

// JWTConfig holds JWT configuration
type JWTConfig struct {
	Secret     string
	Expiration time.Duration
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	Threshold      int
	BurstLimit     int
	ResetTimeout   time.Duration
	BurstDuration  time.Duration
}

// CircuitBreakerConfig holds circuit breaker configuration
type CircuitBreakerConfig struct {
	Threshold      int
	ResetTimeout   time.Duration
	HalfOpenDuration time.Duration
}

// CacheConfig holds cache configuration
type CacheConfig struct {
	MaxSize         int
	TTL             time.Duration
	CleanupInterval time.Duration
}

// TLSConfig holds TLS configuration
type TLSConfig {
	CertFile string
	KeyFile  string
	CAFile   string
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	return &Config{
		GRPCPort: ":50055",
		HTTPPort: ":8080",
		Redis: &redis.Options{
			Addr: "redis:6379",
		},
		NATSURL: nats.DefaultURL,
		JWTConfig: JWTConfig{
			Secret:     "default-secret-change-in-production",
			Expiration: 24 * time.Hour,
		},
		RateLimit: RateLimitConfig{
			Threshold:      10,
			BurstLimit:     5,
			ResetTimeout:   1 * time.Minute,
			BurstDuration:  10 * time.Second,
		},
		CircuitBreaker: CircuitBreakerConfig{
			Threshold:        3,
			ResetTimeout:     30 * time.Second,
			HalfOpenDuration: 10 * time.Second,
		},
		Cache: CacheConfig{
			MaxSize:         1000,
			TTL:             5 * time.Minute,
			CleanupInterval: 1 * time.Minute,
		},
		ExternalServices: make(map[string]string),
		TLS: TLSConfig{
			CertFile: "certs/server.crt",
			KeyFile:  "certs/server.key",
			CAFile:   "certs/ca.crt",
		},
	}
}