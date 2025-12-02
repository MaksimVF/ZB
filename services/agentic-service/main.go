


/*
Agentic Service
===============

Purpose: This service provides specialized agentic functionality with advanced tool calling,
parallel processing, and caching capabilities. It serves as the dedicated service for
agent-related operations in our architecture.

Key Features:
- Advanced agentic endpoint with parallel tool calls
- Tool result caching for performance optimization
- Premium model access for agentic operations
- Specialized billing and monitoring capabilities
- Secure communication with other services

Role: The Agentic Service handles all agent-related processing, providing enhanced capabilities
beyond standard LLM APIs. It integrates with premium models and offers specialized
features for agent frameworks.
*/

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

	pb "llm-gateway-pro/services/secret-service/pb"
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
	r.HandleFunc("/v1/agentic", middleware.RateLimiter(handlers.AgenticHandler)).Methods("POST")

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

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		log.Fatalf("Failed to add CA certificate to pool")
	}

	return credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      certPool,
		ServerName:   "secret-service",
		MinVersion:   tls.VersionTLS12,
	})
}

// watchSecretsUpdates watches for secret updates from Redis
func watchSecretsUpdates() {
	retryDelay := 5 * time.Second
	maxRetryDelay := 60 * time.Second

	for {
		log.Println("Connecting to Redis for secret updates...")
		pubsub := redisClient.Subscribe(context.Background(), "secrets:updated")

		// Wait for subscription confirmation
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := pubsub.Receive(ctx); err != nil {
			log.Printf("Failed to subscribe to Redis: %v. Retrying in %v...", err, retryDelay)
			time.Sleep(retryDelay)
			retryDelay = time.Duration(float64(retryDelay) * 1.5) // exponential backoff
			if retryDelay > maxRetryDelay {
				retryDelay = maxRetryDelay
			}
			continue
		}

		// Reset retry delay on successful connection
		retryDelay = 5 * time.Second

		log.Println("Successfully subscribed to Redis secret updates")

		// Process messages
		for {
			msg, err := pubsub.ReceiveMessage()
			if err != nil {
				log.Printf("Redis subscription error: %v. Reconnecting...", err)
				break
			}
			log.Printf("Secret updated: %s - clearing cache", msg.Payload)
			secretsCache.Delete(msg.Payload)
		}

		// Clean up and reconnect
		pubsub.Close()
		time.Sleep(retryDelay)
	}
}

