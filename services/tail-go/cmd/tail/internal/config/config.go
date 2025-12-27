package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Services ServicesConfig `mapstructure:"services"`
	Security SecurityConfig `mapstructure:"security"`
}

type ServerConfig struct {
	Host string    `mapstructure:"host"`
	Port int       `mapstructure:"port"`
	TLS  TLSConfig `mapstructure:"tls"`
}

type TLSConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	CertFile string `mapstructure:"cert_file"`
	KeyFile  string `mapstructure:"key_file"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type ServicesConfig struct {
	SecretService ServiceConfig `mapstructure:"secret_service"`
	AuthService   ServiceConfig `mapstructure:"auth_service"`
	RateLimiter   ServiceConfig `mapstructure:"rate_limiter"`
}

type ServiceConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

type SecurityConfig struct {
	ContentFilteringEnabled bool `mapstructure:"content_filtering_enabled"`
	AuditLoggingEnabled     bool `mapstructure:"audit_logging_enabled"`
	DataIsolationEnabled    bool `mapstructure:"data_isolation_enabled"`
	RateLimitEnabled        bool `mapstructure:"rate_limit_enabled"`
}

var AppConfig Config

func Load() error {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	viper.AddConfigPath(".")

	// Set defaults
	setDefaults()

	// Read config
	if err := viper.ReadInConfig(); err != nil {
		// Config file not found, use defaults
	}

	if err := viper.Unmarshal(&AppConfig); err != nil {
		return err
	}

	return nil
}

func setDefaults() {
	// Server defaults
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8443)
	viper.SetDefault("server.tls.enabled", true)
	viper.SetDefault("server.tls.cert_file", "/certs/tail-go.pem")
	viper.SetDefault("server.tls.key_file", "/certs/tail-go-key.pem")

	// Redis defaults
	viper.SetDefault("redis.host", "redis")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)

	// Services defaults
	viper.SetDefault("services.secret_service.host", "secret-service")
	viper.SetDefault("services.secret_service.port", 50053)
	viper.SetDefault("services.auth_service.host", "auth-service")
	viper.SetDefault("services.auth_service.port", 50051)
	viper.SetDefault("services.rate_limiter.host", "rate-limiter")
	viper.SetDefault("services.rate_limiter.port", 50051)

	// Security defaults
	viper.SetDefault("security.content_filtering_enabled", true)
	viper.SetDefault("security.audit_logging_enabled", true)
	viper.SetDefault("security.data_isolation_enabled", true)
	viper.SetDefault("security.rate_limit_enabled", true)

	// Environment variable overrides
	viper.SetEnvPrefix("TAIL")
	viper.AutomaticEnv()
}

// GetRedisAddr returns Redis address
func (c *Config) GetRedisAddr() string {
	return fmt.Sprintf("%s:%d", c.Redis.Host, c.Redis.Port)
}

// GetServerAddr returns server address
func (c *Config) GetServerAddr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}
