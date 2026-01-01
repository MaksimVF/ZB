package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/hashicorp/vault/api"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/MaksimVF/ZB/services/secrets-service/config"
	"github.com/MaksimVF/ZB/services/secrets-service/core"
	"github.com/MaksimVF/ZB/services/secrets-service/grpc"
	httpHandler "github.com/MaksimVF/ZB/services/secrets-service/http"
	pb "github.com/MaksimVF/ZB/services/secrets-service/pb"
	"github.com/MaksimVF/ZB/services/secrets-service/storage"
	"github.com/MaksimVF/ZB/services/secrets-service/utils"
)

func main() {
	// Загрузка конфигурации
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	// Инициализация логгера
	logger := utils.New(cfg.ServiceName)
	logger.Info().Str("service", cfg.ServiceName).Msg("Starting secret service")

	// Регистрация метрик Prometheus
	prometheus.MustRegister(
		prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "secret_operations_total",
				Help: "Total number of secret operations",
			},
			[]string{"operation", "status"},
		),
		prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path", "status"},
		),
	)

	// Инициализация Vault клиента
	vaultClient, err := initVaultClient(cfg, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize Vault client")
	}

	// Инициализация компонентов сервиса
	validator := &utils.Validator{}
	vaultStorage := storage.NewVaultStorage(vaultClient, logger)
	secretService := core.NewSecretService(vaultStorage, validator, logger, cfg)

	// Запуск gRPC сервера
	go func() {
		if err := startGRPCServer(cfg, secretService, logger); err != nil {
			logger.Fatal().Err(err).Msg("gRPC server failed")
		}
	}()

	// Запуск HTTP сервера
	if err := startHTTPServer(cfg, secretService, logger); err != nil {
		logger.Fatal().Err(err).Msg("HTTP server failed")
	}
}

// initVaultClient инициализирует Vault клиент
func initVaultClient(cfg *config.Config, logger *utils.Logger) (*api.Client, error) {
	vaultConfig := api.DefaultConfig()
	vaultConfig.Address = cfg.VaultAddr

	client, err := api.NewClient(vaultConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Vault client: %w", err)
	}

	client.SetToken(cfg.VaultToken)

	// Проверка соединения с Vault
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = client.Sys().HealthWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("Vault health check failed: %w", err)
	}

	logger.Info().Str("vault_addr", cfg.VaultAddr).Msg("Vault client initialized successfully")
	return client, nil
}

// startGRPCServer запускает gRPC сервер
func startGRPCServer(cfg *config.Config, secretService *core.SecretService, logger *utils.Logger) error {
	lis, err := net.Listen("tcp", cfg.GRPCPort)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", cfg.GRPCPort, err)
	}

	creds, err := credentials.NewServerTLSFromFile(cfg.TLSCertPath, cfg.TLSKeyPath)
	if err != nil {
		return fmt.Errorf("failed to load TLS credentials: %w", err)
	}

	grpcServer := grpc.NewServer(grpc.Creds(creds))

	// Создание gRPC обработчика
	grpcLogger := grpc.New(cfg.ServiceName)
	grpcHandler := grpc.NewGRPCHandler(secretService, grpcLogger)
	pb.RegisterSecretServiceServer(grpcServer, grpcHandler)

	logger.Info().Str("port", cfg.GRPCPort).Msg("Starting gRPC server")
	return grpcServer.Serve(lis)
}

// startHTTPServer запускает HTTP сервер
func startHTTPServer(cfg *config.Config, secretService *core.SecretService, logger *utils.Logger) error {
	// Создание HTTP обработчика
	httpLogger := httpHandler.New(cfg.ServiceName)
	httpHandler := httpHandler.New(secretService, httpLogger, cfg)

	// Настройка роутера
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware(cfg.AllowedOrigins))

	// Маршруты админ API
	r.Route("/admin/api", func(r chi.Router) {
		r.Post("/secrets", withAuth(httpHandler.AdminCreateSecret, cfg, logger))
		r.Get("/secrets", withAuth(httpHandler.AdminListSecrets, cfg, logger))
		r.Delete("/secrets/{name}", withAuth(httpHandler.AdminDeleteSecret, cfg, logger))
	})

	// Дополнительные маршруты
	r.Get("/health", healthCheckHandler(vaultClient, logger))
	r.Handle("/metrics", promhttp.Handler())

	logger.Info().Str("port", cfg.HTTPPort).Msg("Starting HTTP server")
	return http.ListenAndServe(cfg.HTTPPort, r)
}

// corsMiddleware создает CORS middleware
func corsMiddleware(allowedOrigins string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && utils.New("").IsOriginAllowed(origin, allowedOrigins) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,DELETE,OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type,X-Admin-Key")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// withAuth создает middleware для аутентификации
func withAuth(handler http.HandlerFunc, cfg *config.Config, logger *utils.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Проверка админ ключа
		adminKey := r.Header.Get("X-Admin-Key")
		if adminKey == "" {
			logger.Warn().Msg("Missing admin key")
			http.Error(w, "forbidden: missing admin key", http.StatusForbidden)
			return
		}

		validator := &utils.Validator{}
		if err := validator.ValidateAdminKey(adminKey, cfg.AdminKeyRegex); err != nil {
			logger.Warn().Msg("Invalid admin key format")
			http.Error(w, "forbidden: invalid admin key format", http.StatusForbidden)
			return
		}

		if adminKey != cfg.AdminKey {
			logger.Warn().Msg("Invalid admin key")
			http.Error(w, "forbidden: invalid admin key", http.StatusForbidden)
			return
		}

		handler(w, r)
	}
}

// healthCheckHandler создает обработчик проверки здоровья
func healthCheckHandler(vaultClient *api.Client, logger *utils.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		health, err := vaultClient.Sys().HealthWithContext(ctx)
		if err != nil {
			logger.Error().Err(err).Msg("Vault health check failed")
			http.Error(w, "vault unhealthy", http.StatusServiceUnavailable)
			return
		}

		if !health.Initialized || health.Sealed {
			logger.Error().Msg("Vault is not initialized or sealed")
			http.Error(w, "vault not ready", http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
	}
}
