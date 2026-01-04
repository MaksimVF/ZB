// Package config provides configuration management for service-discovery service

package config

import (
	"log"

	"github.com/spf13/viper"
)

type Config struct {
	Server      ServerConfig      `mapstructure:"server"`
	Database    DatabaseConfig    `mapstructure:"database"`
	Redis       RedisConfig       `mapstructure:"redis"`
	Monitoring  MonitoringConfig  `mapstructure:"monitoring"`
	ServiceMesh ServiceMeshConfig `mapstructure:"service_mesh"`
}

type ServerConfig struct {
	Port         string `mapstructure:"port"`
	Mode         string `mapstructure:"mode"`
	ReadTimeout  int    `mapstructure:"read_timeout"`
	WriteTimeout int    `mapstructure:"write_timeout"`
	IdleTimeout  int    `mapstructure:"idle_timeout"`
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type MonitoringConfig struct {
	Enabled         bool   `mapstructure:"enabled"`
	HealthCheckPath string `mapstructure:"health_check_path"`
	MetricsEnabled  bool   `mapstructure:"metrics_enabled"`
	TracingEnabled  bool   `mapstructure:"tracing_enabled"`
}

type ServiceMeshConfig struct {
	Enabled          bool   `mapstructure:"enabled"`
	ConsulEnabled    bool   `mapstructure:"consul_enabled"`
	ConsulAddress    string `mapstructure:"consul_address"`
	LoadBalancerType string `mapstructure:"load_balancer_type"`
	CircuitBreaker   bool   `mapstructure:"circuit_breaker"`
}

var AppConfig *Config

// LoadConfig loads configuration from file and environment
func LoadConfig(path string) error {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(path)

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		log.Printf("Warning: No config file found at %s, using defaults and environment variables", path)
		return nil
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return err
	}

	AppConfig = &config
	return nil
}
