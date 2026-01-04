package config

import (
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config структура конфигурации сервиса ценообразования
type Config struct {
	// Server settings
	GRPCPort    int
	HTTPPort    int
	Environment string

	// Redis settings
	Redis RedisConfig

	// Security settings
	AdminKey string
	JWTKey   string

	// Pricing settings
	PricingCacheTTL time.Duration
	MaxModels       int
}

// RedisConfig конфигурация Redis
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// Load загружает конфигурацию из переменных окружения
func Load() (*Config, error) {
	// Загрузка .env файла если существует
	godotenv.Load()

	cfg := &Config{
		// Server settings
		GRPCPort:    getEnvAsInt("GRPC_PORT", 50051),
		HTTPPort:    getEnvAsInt("HTTP_PORT", 8081),
		Environment: getEnv("ENVIRONMENT", "development"),

		// Redis settings
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvAsInt("REDIS_DB", 0),
		},

		// Security settings
		AdminKey: getEnv("ADMIN_KEY", "default-admin-key-2025"),
		JWTKey:   getEnv("JWT_SECRET", "default-jwt-secret-2025"),

		// Pricing settings
		PricingCacheTTL: getEnvAsDuration("PRICING_CACHE_TTL", 1*time.Hour),
		MaxModels:       getEnvAsInt("MAX_MODELS", 1000),
	}

	return cfg, nil
}

// getEnv получает переменную окружения или возвращает значение по умолчанию
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsInt получает переменную окружения как int или возвращает значение по умолчанию
func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvAsDuration получает переменную окружения как Duration или возвращает значение по умолчанию
func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

// SetupLogging настраивает логирование в зависимости от окружения
func SetupLogging(environment string) {
	// Настройка логирования может быть добавлена здесь
	// Пока оставляем простую настройку
	if environment == "production" {
		// Production logging settings
	} else {
		// Development logging settings
	}
}
