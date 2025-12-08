









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

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "github.com/MaksimVF/ZB/services/secrets-service/pb"
	"llm-gateway-pro/services/agentic-service/handlers"
	"llm-gateway-pro/services/agentic-service/middleware"
)

var (
	secretClient  pb.SecretServiceClient
	secretConn    *grpc.ClientConn
	redisClient  *redis.Client
	secretsCache sync.Map
)

func main() {
	// Initialize Redis client
	redisClient = redis.NewClient(&redis.Options{
		Addr: "redis:6379",
	})

	// Connect to secret-service (gRPC + mTLS)
	var err error
	secretConn, err = grpc.Dial(
		"secret-service:50053",
		grpc.WithTransportCredentials(loadClientTLSCredentials()),
	)
	if err != nil {
		log.Fatalf("Failed to connect to secret-service: %v", err)
	}
	defer secretConn.Close()

	secretClient = pb.NewSecretServiceClient(secretConn)

	// Start secret update watcher
	go watchSecretsUpdates()

	// Initialize HTTP router
	r := mux.NewRouter()

	// Agentic endpoint
	r.Handle("/v1/agentic", middleware.RateLimiter(
		middleware.ContentFilteringMiddleware(
			middleware.AuditLoggingMiddleware(
				middleware.DataIsolationMiddleware(handlers.AgenticHandler))))).Methods("POST")

	// Health check
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
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
	}).Methods("GET")

	// Provider management endpoints
	r.HandleFunc("/v1/providers", handlers.GetProviders).Methods("GET")
	r.HandleFunc("/v1/providers/health", handlers.GetProviderHealth).Methods("GET")
	r.HandleFunc("/v1/providers", handlers.AddProvider).Methods("POST")
	r.HandleFunc("/v1/providers", handlers.RemoveProvider).Methods("DELETE")
	r.HandleFunc("/v1/providers/api-key", handlers.UpdateProviderAPIKey).Methods("PUT")

	// Start HTTP server
	srv := &http.Server{
		Addr:    ":8081",
		Handler: r,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	// Graceful shutdown
	go func() {
		log.Println("Agentic Service started on https://0.0.0.0:8081")
		if err := srv.ListenAndServeTLS(
			"/certs/agentic.pem",
			"/certs/agentic-key.pem",
		); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTPS server failed: %v", err)
		}
	}()

	// Wait for termination signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c

	log.Println("Shutting down Agentic Service...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	log.Println("Agentic Service stopped")
}

// loadClientTLSCredentials loads client TLS credentials for mTLS
func loadClientTLSCredentials() credentials.TransportCredentials {
	clientCert, err := tls.LoadX509KeyPair("/certs/agentic.pem", "/certs/agentic-key.pem")
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

// watchSecretsUpdates watches for secret updates
func watchSecretsUpdates() {
	// Implementation of secret updates watching
}









