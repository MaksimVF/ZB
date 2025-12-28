package config

import (
	"time"

	"github.com/redis/go-redis/v9"
)

// Config holds the application configuration
type Config struct {
	GRPCPort       string
	HTTPPort       string
	NATSURL        string
	Redis          *redis.Options
	TLS            TLSConfig
	Cache          CacheConfig
	RateLimit      RateLimitConfig
	CircuitBreaker CircuitBreakerConfig
}

// TLSConfig holds TLS configuration
type TLSConfig struct {
	CertFile string
	KeyFile  string
	CAFile   string
}

// CacheConfig holds cache configuration
type CacheConfig struct {
	MaxSize int
	TTL     int
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	Threshold     int
	ResetTimeout  time.Duration
	BurstLimit    int
	BurstDuration time.Duration
}

// CircuitBreakerConfig holds circuit breaker configuration
type CircuitBreakerConfig struct {
	Threshold        int
	ResetTimeout     time.Duration
	HalfOpenDuration time.Duration
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		GRPCPort: ":50061",
		HTTPPort: ":8080",
		NATSURL:  "nats://localhost:4222",
		Redis: &redis.Options{
			Addr: "localhost:6379",
		},
		TLS: TLSConfig{
			CertFile: "certs/server.crt",
			KeyFile:  "certs/server.key",
			CAFile:   "certs/ca.crt",
		},
		Cache: CacheConfig{
			MaxSize: 1000,
			TTL:     300, // 5 minutes
		},
		RateLimit: RateLimitConfig{
			Threshold:     100,
			ResetTimeout:  time.Minute,
			BurstLimit:    10,
			BurstDuration: time.Second * 10,
		},
		CircuitBreaker: CircuitBreakerConfig{
			Threshold:        5,
			ResetTimeout:     time.Minute * 5,
			HalfOpenDuration: time.Second * 30,
		},
	}
}
