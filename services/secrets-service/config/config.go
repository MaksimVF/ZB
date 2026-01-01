package config

import (
	"os"
	"regexp"
	"time"
)

// Config содержит конфигурацию сервиса
type Config struct {
	// Vault configuration
	VaultAddr  string
	VaultToken string

	// Server configuration
	GRPCPort    string
	HTTPPort    string
	ServiceName string

	// Security configuration
	AdminKey       string
	AllowedOrigins string

	// Rate limiting
	RateLimitWindow time.Duration
	RateLimitMax    int

	// Validation
	MaxSecretValueSize int
	AdminKeyRegex      *regexp.Regexp

	// TLS certificates
	TLSCertPath string
	TLSKeyPath  string
}

// Load загружает конфигурацию из переменных окружения
func Load() *Config {
	return &Config{
		VaultAddr:          getEnv("VAULT_ADDR", "http://vault:8200"),
		VaultToken:         getEnv("VAULT_TOKEN", ""),
		GRPCPort:           getEnv("GRPC_PORT", ":50053"),
		HTTPPort:           getEnv("HTTP_PORT", ":8082"),
		ServiceName:        getEnv("SERVICE_NAME", "secret-service"),
		AdminKey:           getEnv("ADMIN_KEY", ""),
		AllowedOrigins:     getEnv("ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:3001"),
		RateLimitWindow:    5 * time.Second,
		RateLimitMax:       1,
		MaxSecretValueSize: 4096,
		AdminKeyRegex:      regexp.MustCompile(`^[a-zA-Z0-9\-_]{16,64}$`),
		TLSCertPath:        getEnv("TLS_CERT_PATH", "/certs/secret-service.pem"),
		TLSKeyPath:         getEnv("TLS_KEY_PATH", "/certs/secret-service-key.pem"),
	}
}

// Validate проверяет конфигурацию
func (c *Config) Validate() error {
	if c.VaultAddr == "" {
		return ErrConfig("VAULT_ADDR is required")
	}
	if c.VaultToken == "" {
		return ErrConfig("VAULT_TOKEN is required")
	}
	if c.AdminKey == "" {
		return ErrConfig("ADMIN_KEY is required")
	}
	return nil
}

// ErrConfig возвращает ошибку конфигурации
func ErrConfig(msg string) error {
	return &ConfigError{Message: msg}
}

// ConfigError представляет ошибку конфигурации
type ConfigError struct {
	Message string
}

func (e *ConfigError) Error() string {
	return "config error: " + e.Message
}

// getEnv возвращает значение переменной окружения или default значение
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
