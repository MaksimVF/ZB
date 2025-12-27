package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/MaksimVF/ZB/services/head-go/internal/config"
	"github.com/MaksimVF/ZB/services/head-go/internal/metrics"
	"github.com/MaksimVF/ZB/services/head-go/internal/server"
)

// Constants for configuration
const (
	ShutdownTimeout      = 10 * time.Second
	ConfigReloadInterval = 10 * time.Second
	RedisDefaultAddr    = "redis:6379"
	MetricsDefaultPort  = ":9090"
)

func main() {
	// Initialize structured logging
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	
	// Add service name to all logs
	logger := log.With().Str("service", "head-go").Logger()
	log.Logger = logger

	// Setup signal handling for graceful shutdown
	ctx, cancel := setupSignalHandler()
	defer cancel()

	// Load and validate configuration
	cfg, err := loadAndValidateConfig()
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load and validate configuration")
	}

	// Initialize network config manager
	networkConfigManager, err := initNetworkConfigManager(cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize network config manager")
	}

	// Ensure graceful shutdown of network config manager
	defer func() {
		logger.Info().Msg("Shutting down network config manager...")
		if err := networkConfigManager.Stop(); err != nil {
			logger.Error().Err(err).Msg("Failed to stop network config manager")
		}
	}()

	// Run startup health checks
	if err := runStartupHealthChecks(ctx, networkConfigManager); err != nil {
		logger.Fatal().Err(err).Msg("Startup health checks failed")
	}

	// Start background services
	startBackgroundServices(cfg)

	// Initialize and start server
	srv := server.New(cfg, networkConfigManager)
	
	logger.Info().
		Str("metrics_port", cfg.MetricsPort).
		Str("redis_addr", cfg.RedisAddr).
		Msg("Head service starting")

	// Run server with error handling
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Run()
	}()

	// Wait for shutdown signal or server error
	select {
	case <-ctx.Done():
		logger.Info().Msg("Received shutdown signal")
	case err := <-errCh:
		logger.Error().Err(err).Msg("Server error occurred")
	}

	// Graceful shutdown with timeout
	if err := gracefulShutdown(srv, networkConfigManager); err != nil {
		logger.Error().Err(err).Msg("Failed to shutdown gracefully")
	}

	logger.Info().Msg("Head service shutdown complete")
}

// loadAndValidateConfig loads configuration and validates it
func loadAndValidateConfig() (*config.Config, error) {
	cfg := config.Load()
	
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}
	
	return cfg, nil
}

// validateConfig validates the configuration
func validateConfig(cfg *config.Config) error {
	var errs []error
	
	if cfg.RedisAddr == "" {
		errs = append(errs, errors.New("redis address is required"))
	}
	
	if cfg.MetricsPort <= 0 || cfg.MetricsPort > 65535 {
		errs = append(errs, errors.New("metrics port must be between 1 and 65535"))
	}
	
	if cfg.GRPCAddr == "" {
		errs = append(errs, errors.New("gRPC address is required"))
	}
	
	if cfg.AuthConfig.JWTSecret == "" {
		errs = append(errs, errors.New("JWT secret is required"))
	}
	
	if cfg.AuthConfig.JWTSecret == "default-secret-key-change-in-production" {
		errs = append(errs, errors.New("JWT secret must be changed from default value in production"))
	}
	
	if len(errs) > 0 {
		return fmt.Errorf("configuration errors: %v", errs)
	}
	
	return nil
}

// initNetworkConfigManager initializes the network configuration manager
func initNetworkConfigManager(cfg *config.Config) (*config.NetworkConfigManager, error) {
	networkConfigManager := config.NewNetworkConfigManager(cfg.RedisAddr)
	
	// Load initial configuration
	if err := networkConfigManager.LoadConfig(); err != nil {
		return nil, fmt.Errorf("failed to load network config: %w", err)
	}
	
	// Start auto-reload for network config
	networkConfigManager.StartAutoReload(ConfigReloadInterval)
	
	logger.Info().
		Str("redis_addr", cfg.RedisAddr).
		Dur("reload_interval", ConfigReloadInterval).
		Msg("Network config manager initialized")
	
	return networkConfigManager, nil
}

// runStartupHealthChecks performs health checks during startup
func runStartupHealthChecks(ctx context.Context, networkConfigManager *config.NetworkConfigManager) error {
	logger := log.With().Str("component", "startup_health_checks").Logger()
	
	// Check network config manager connectivity
	if err := networkConfigManager.Ping(); err != nil {
		return fmt.Errorf("network config manager health check failed: %w", err)
	}
	
	logger.Info().Msg("All startup health checks passed")
	return nil
}

// startBackgroundServices starts background services like metrics
func startBackgroundServices(cfg *config.Config) {
	// Start metrics server
	go func() {
		if err := metrics.Start(cfg.MetricsPort); err != nil {
			log.Error().
				Err(err).
				Str("metrics_port", cfg.MetricsPort).
				Msg("Failed to start metrics server")
		}
	}()
	
	logger := log.With().Str("component", "background_services").Logger()
	logger.Info().
		Str("metrics_port", cfg.MetricsPort).
		Msg("Background services started")
}

// setupSignalHandler sets up signal handling for graceful shutdown
func setupSignalHandler() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	
	sigCh := make(chan os.Signal, 1)
	
	// Register for common shutdown signals
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	
	go func() {
		<-sigCh
		logger := log.With().Str("component", "signal_handler").Logger()
		logger.Info().Msg("Received shutdown signal, initiating graceful shutdown")
		cancel()
	}()
	
	return ctx, cancel
}

// gracefulShutdown performs graceful shutdown of all services
func gracefulShutdown(srv *server.Server, networkConfigManager *config.NetworkConfigManager) error {
	logger := log.With().Str("component", "graceful_shutdown").Logger()
	
	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), ShutdownTimeout)
	defer shutdownCancel()
	
	// Shutdown server first
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error().
			Err(err).
			Dur("timeout", ShutdownTimeout).
			Msg("Failed to shutdown server gracefully")
		return fmt.Errorf("server shutdown failed: %w", err)
	}
	
	logger.Info().Msg("Server shutdown completed")
	return nil
}

// Health check functions for different components
func checkNetworkConfigManagerHealth() error {
	// Implement network config manager health check
	return nil
}

func checkRedisConnectivity(redisAddr string) error {
	// Implement Redis connectivity check
	return nil
}

// Error types for better error handling
type ConfigError struct {
	Field string
	Value interface{}
	Msg   string
}

func (e ConfigError) Error() string {
	return fmt.Sprintf("configuration error in %s: %s (value: %v)", e.Field, e.Msg, e.Value)
}

type ServiceError struct {
	Service string
	Err     error
}

func (e ServiceError) Error() string {
	return fmt.Sprintf("%s service error: %v", e.Service, e.Err)
}

// Logging helper functions
func logStartupInfo(cfg *config.Config) {
	log.Info().
		Str("service", "head-go").
		Str("version", getVersion()).
		Int("metrics_port", cfg.MetricsPort).
		Str("grpc_addr", cfg.GRPCAddr).
		Str("redis_addr", cfg.RedisAddr).
		Dur("reload_interval", cfg.ReloadInterval).
		Str("log_level", cfg.LogLevel).
		Msg("Head service starting up")
}

func getVersion() string {
	// In a real implementation, this would return the actual version
	return "1.0.0"
}
