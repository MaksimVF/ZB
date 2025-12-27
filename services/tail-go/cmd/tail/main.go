package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "github.com/MaksimVF/ZB/services/secrets-service/pb" // <-- твой proto
	"github.com/MaksimVF/ZB/services/tail-go/cmd/tail/handlers"
	"github.com/MaksimVF/ZB/services/tail-go/cmd/tail/internal/config"
	"github.com/MaksimVF/ZB/services/tail-go/middleware"
)

// Глобальные клиенты
var (
	secretClient pb.SecretServiceClient
	secretConn   *grpc.ClientConnInterface
	authClient   pb.AuthServiceClient
	authConn     *grpc.ClientConnInterface
	redisClient  *redis.Client
	headClient   *grpc.HeadClient
	secretsCache sync.Map // имя → plaintext (кешируем на 30 сек)
)

func main() {
	// === 1. Подключаемся к Redis ===
	redisClient = redis.NewClient(&redis.Options{
		Addr: "redis:6379",
	})

	// === 2. Инициализируем NetworkConfigManager ===
	networkConfigManager := config.NewNetworkConfigManager("redis:6379")
	err := networkConfigManager.LoadConfig()
	if err != nil {
		log.Fatalf("Не удалось загрузить сетевую конфигурацию: %v", err)
	}
	networkConfigManager.StartAutoReload(10 * time.Second)

	// === 3. Подключаемся к secret-service (gRPC + mTLS) ===
	var err error
	secretConn, err = grpc.Dial(
		"secret-service:50053",
		grpc.WithTransportCredentials(loadClientTLSCredentials()),
	)
	if err != nil {
		log.Fatalf("Не удалось подключиться к secret-service: %v", err)
	}
	defer secretConn.Close()

	secretClient = pb.NewSecretServiceClient(secretConn)

	// === 4. Подключаемся к auth-service (gRPC + mTLS) ===
	authConn, err = grpc.Dial(
		"auth-service:50051",
		grpc.WithTransportCredentials(loadClientTLSCredentials()),
	)
	if err != nil {
		log.Fatalf("Не удалось подключиться к auth-service: %v", err)
	}
	defer authConn.Close()

	authClient = pb.NewAuthServiceClient(authConn)

	// === 5. Инициализируем HeadClient с NetworkConfigManager ===
	networkConfig := networkConfigManager.GetConfig()
	headEndpoint := networkConfig.HeadEndpoint
	if headEndpoint == "" {
		headEndpoint = "head:50055" // Default fallback
	}
	headClient = grpc.NewHeadClient(headEndpoint, networkConfigManager)

	// === 4. Фоновая задача: обновление секретов при изменении ===
	go watchSecretsUpdates()

	// === 5. HTTP → HTTPS сервер (OpenAI-совместимый API) ===
	mux := http.NewServeMux()

	// Публичные эндпоинты
	mux.HandleFunc("POST /v1/chat/completions", middleware.RateLimiter(
		middleware.ContentFilteringMiddleware(
			middleware.AuditLoggingMiddleware(
				middleware.DataIsolationMiddleware(handlers.ChatCompletion)))))
	mux.HandleFunc("POST /v1/completions", middleware.RateLimiter(
		middleware.ContentFilteringMiddleware(
			middleware.AuditLoggingMiddleware(
				middleware.DataIsolationMiddleware(handlers.ChatCompletion)))))
	mux.HandleFunc("POST /v1/batch", middleware.RateLimiter(
		middleware.ContentFilteringMiddleware(
			middleware.AuditLoggingMiddleware(
				middleware.DataIsolationMiddleware(handlers.BatchSubmit)))))
	mux.HandleFunc("POST /v1/embeddings", middleware.RateLimiter(
		middleware.ContentFilteringMiddleware(
			middleware.AuditLoggingMiddleware(
				middleware.DataIsolationMiddleware(handlers.Embeddings)))))

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		// Check Redis connection
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if err := redisClient.Ping(ctx).Err(); err != nil {
			http.Error(w, fmt.Sprintf("Redis unavailable: %v", err), http.StatusServiceUnavailable)
			return
		}

		// Check gRPC connection to secret-service
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if _, err := secretClient.GetSecret(ctx, &pb.GetSecretRequest{Name: "health_check"}); err != nil {
			http.Error(w, fmt.Sprintf("Secret service unavailable: %v", err), http.StatusServiceUnavailable)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "OK")
	})

	// Provider management endpoints
	mux.HandleFunc("GET /v1/providers", handlers.GetProviders)
	mux.HandleFunc("GET /v1/providers/health", handlers.GetProviderHealth)
	mux.HandleFunc("POST /v1/providers", handlers.AddProvider)
	mux.HandleFunc("DELETE /v1/providers", handlers.RemoveProvider)
	mux.HandleFunc("PUT /v1/providers/api-key", handlers.UpdateProviderAPIKey)

	srv := &http.Server{
		Addr:    ":8443",
		Handler: mux,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	// Graceful shutdown
	go func() {
		log.Println("Gateway запущен на https://0.0.0.0:8443")
		if err := srv.ListenAndServeTLS(
			"/certs/gateway.pem",
			"/certs/gateway-key.pem",
		); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTPS server failed: %v", err)
		}
	}()

	// Wait for termination signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c

	log.Println("Shutting down Gateway...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	log.Println("Gateway stopped")
}

// loadClientTLSCredentials loads client TLS credentials for mTLS
func loadClientTLSCredentials() credentials.TransportCredentials {
	clientCert, err := tls.LoadX509KeyPair("/certs/gateway.pem", "/certs/gateway-key.pem")
	if err != nil {
		log.Fatalf("Failed to load client certificate: %v", err)
	}

	caCert, err := os.ReadFile("/certs/ca.pem")
	if err != nil {
		log.Fatalf("Failed to load CA certificate: %v", err)
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	return credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      caCertPool,
	})
}

// watchSecretsUpdates watches for secret updates using Redis pub/sub
func watchSecretsUpdates() {
	pubsub := redisClient.Subscribe(context.Background(), "secrets:updates")
	defer pubsub.Close()

	ch := pubsub.Channel()

	log.Println("Started watching for secret updates...")

	for msg := range ch {
		log.Printf("Received secret update notification: %s", msg.Payload)

		// Parse the update message (assuming JSON format: {"secret_name": "name"})
		// Invalidate cache for the updated secret
		// For simplicity, we'll clear all cache on any update
		secretsCache.Range(func(key, value interface{}) bool {
			secretsCache.Delete(key)
			return true
		})

		log.Printf("Cleared secrets cache due to update: %s", msg.Payload)
	}
}
