package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config структура конфигурации сервиса
type Config struct {
	// Server settings
	GRPCPort    int
	HTTPPort    int
	Environment string

	// Database settings
	Database DatabaseConfig

	// Redis settings
	Redis RedisConfig

	// RabbitMQ settings
	RabbitMQ RabbitMQConfig

	// Security settings
	AdminKey string
	JWTKey   string
}

// DatabaseConfig конфигурация базы данных
type DatabaseConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	Database string
	SSLMode  string
}

// RedisConfig конфигурация Redis
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// RabbitMQConfig конфигурация RabbitMQ
type RabbitMQConfig struct {
	URL       string
	Exchange  string
	QueueName string
}

// Load загружает конфигурацию из переменных окружения
func Load() (*Config, error) {
	// Загрузка .env файла если существует
	godotenv.Load()

	cfg := &Config{
		// Server settings
		GRPCPort:    getEnvAsInt("GRPC_PORT", 50052),
		HTTPPort:    getEnvAsInt("HTTP_PORT", 8080),
		Environment: getEnv("ENVIRONMENT", "development"),

		// Database settings
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnvAsInt("DB_PORT", 5432),
			Username: getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", ""),
			Database: getEnv("DB_NAME", "billing"),
			SSLMode:  getEnv("DB_SSL_MODE", "disable"),
		},

		// Redis settings
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvAsInt("REDIS_DB", 0),
		},

		// RabbitMQ settings
		RabbitMQ: RabbitMQConfig{
			URL:       getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
			Exchange:  getEnv("RABBITMQ_EXCHANGE", "billing"),
			QueueName: getEnv("RABBITMQ_QUEUE", "balance-events"),
		},

		// Security settings
		AdminKey: getEnv("ADMIN_KEY", "default-admin-key-2025"),
		JWTKey:   getEnv("JWT_SECRET", "default-jwt-secret-2025"),
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
