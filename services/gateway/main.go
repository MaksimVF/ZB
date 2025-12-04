

/*
Gateway Service (Agent Gateway)
==============================

Purpose: This service provides a unified gateway for LLM APIs with special support for
LangChain integration and agent-related functionality. It serves as the agent-focused
API gateway in our architecture.

Key Features:
- OpenAI-compatible API
- LangChain-specific endpoint with usage tracking
- Multi-provider support (OpenAI, Anthropic, Google, Meta, etc.)
- Comprehensive monitoring and health checks
- LiteLLM integration for dynamic provider routing

Role: The Gateway Service is the entry point for agent-related API calls and LLM provider management.
It routes requests to the appropriate services and handles provider-specific logic.
*/

package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	pb "github.com/MaksimVF/ZB/services/secrets-service/pb"
	"llm-gateway-pro/services/gateway/internal/handlers"
	"llm-gateway-pro/services/gateway/internal/billing"
	"llm-gateway-pro/services/gateway/internal/providers"
	"llm-gateway-pro/services/gateway/internal/resilience"
)

var (
	logger        zerolog.Logger
	secretClient  pb.SecretServiceClient
	secretConn    *grpc.ClientConn
)

func init() {
	// Initialize structured logger
	logger = zerolog.New(os.Stdout).With().
		Timestamp().
		Str("service", "gateway").
		Logger()

	// Initialize gRPC connection to secrets-service
	var err error
	secretConn, err = grpc.Dial(
		"secret-service:50053",
		grpc.WithTransportCredentials(loadClientTLSCredentials()),
	)
	if err != nil {
		log.Fatalf("Failed to connect to secret-service: %v", err)
	}

	secretClient = pb.NewSecretServiceClient(secretConn)
}

func main() {
	// Initialize Prometheus metrics
	handlers.InitMetrics()

	// Initialize billing system
	dbConnString := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_PORT"),
	)

	err := billing.Init(dbConnString)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize billing system")
	}
	defer billing.Close()

	// Initialize LiteLLM providers with secrets from secrets-service
	providerConfig := providers.LiteLLMConfig{
		Providers: map[string]providers.ProviderConfig{
			"openai": {
				BaseURL:    "https://api.openai.com",
				APIKey:     getSecretFromService("llm/openai/api_key"),
				ModelNames: []string{"gpt-4", "gpt-3.5-turbo", "gpt-4o"},
			},
			"anthropic": {
				BaseURL:    "https://api.anthropic.com",
				APIKey:     getSecretFromService("llm/anthropic/api_key"),
				ModelNames: []string{"claude-3", "claude-2", "claude-instant"},
			},
			"google": {
				BaseURL:    "https://api.google.com",
				APIKey:     getSecretFromService("llm/google/api_key"),
				ModelNames: []string{"gemini-1.5", "gemini-1.0", "gemini-pro"},
			},
			"meta": {
				BaseURL:    "https://api.meta.com",
				APIKey:     getSecretFromService("llm/meta/api_key"),
				ModelNames: []string{"llama-3", "llama-2", "llama-1"},
			},
			"mistral": {
				BaseURL:    "https://api.mistral.ai",
				APIKey:     getSecretFromService("llm/mistral/api_key"),
				ModelNames: []string{"mistral-large", "mistral-medium", "mistral-small"},
			},
			"cohere": {
				BaseURL:    "https://api.cohere.ai",
				APIKey:     getSecretFromService("llm/cohere/api_key"),
				ModelNames: []string{"command-r", "command-light", "command-nightly"},
			},
		},
	}

	providers.Init(providerConfig)

	// Initialize circuit breakers
	circuitBreakerConfigs := []resilience.CircuitBreakerConfig{
		{
			Name:          "openai",
			MaxRequests:    5,
			Interval:       60 * time.Second,
			Timeout:        10 * time.Second,
			ReadyToTrip:    resilience.DefaultReadyToTrip,
			OnStateChange: resilience.DefaultOnStateChange,
		},
		{
			Name:          "anthropic",
			MaxRequests:    5,
			Interval:       60 * time.Second,
			Timeout:        10 * time.Second,
			ReadyToTrip:    resilience.DefaultReadyToTrip,
			OnStateChange: resilience.DefaultOnStateChange,
		},
		{
			Name:          "google",
			MaxRequests:    5,
			Interval:       60 * time.Second,
			Timeout:        10 * time.Second,
			ReadyToTrip:    resilience.DefaultReadyToTrip,
			OnStateChange: resilience.DefaultOnStateChange,
		},
		{
			Name:          "meta",
			MaxRequests:    5,
			Interval:       60 * time.Second,
			Timeout:        10 * time.Second,
			ReadyToTrip:    resilience.DefaultReadyToTrip,
			OnStateChange: resilience.DefaultOnStateChange,
		},
		{
			Name:          "mistral",
			MaxRequests:    5,
			Interval:       60 * time.Second,
			Timeout:        10 * time.Second,
			ReadyToTrip:    resilience.DefaultReadyToTrip,
			OnStateChange: resilience.DefaultOnStateChange,
		},
		{
			Name:          "cohere",
			MaxRequests:    5,
			Interval:       60 * time.Second,
			Timeout:        10 * time.Second,
			ReadyToTrip:    resilience.DefaultReadyToTrip,
			OnStateChange: resilience.DefaultOnStateChange,
		},
	}

	resilience.InitCircuitBreakers(circuitBreakerConfigs)

	r := mux.NewRouter()

	// Apply security middlewares
	r.Use(middleware.ContentFilteringMiddleware)
	r.Use(middleware.AuditLoggingMiddleware)
	r.Use(middleware.DataIsolationMiddleware)

	// LangChain-specific endpoint
	r.HandleFunc("/v1/langchain/chat/completions", handlers.LangChainCompletion).Methods("POST")

	// Standard OpenAI-compatible endpoint
	r.HandleFunc("/v1/chat/completions", handlers.ChatCompletion).Methods("POST")

	// Agentic endpoint - proxy to agentic service
	r.HandleFunc("/v1/agentic", handlers.ProxyAgenticRequest).Methods("POST")

	// Provider management endpoints
	r.HandleFunc("/v1/providers", handlers.ListProviders).Methods("GET")
	r.HandleFunc("/v1/providers", handlers.AddProvider).Methods("POST")
	r.HandleFunc("/v1/providers/{provider}", handlers.RemoveProvider).Methods("DELETE")

	// Security configuration endpoints
	r.HandleFunc("/v1/security/config", handlers.GetSecurityConfig).Methods("GET")
	r.HandleFunc("/v1/security/config", handlers.UpdateSecurityConfig).Methods("PUT")

	// Circuit breaker endpoints
	r.HandleFunc("/v1/circuit-breakers", handlers.ListCircuitBreakers).Methods("GET")
	r.HandleFunc("/v1/circuit-breakers/{name}", handlers.GetCircuitBreakerStatus).Methods("GET")
	r.HandleFunc("/v1/circuit-breakers/{name}/reset", handlers.ResetCircuitBreaker).Methods("POST")

	// Health check
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Metrics endpoint
	r.Handle("/metrics", promhttp.Handler())

	// Start HTTP server
	server := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	logger.Info().Msg("Starting gateway service on :8080")
	if err := server.ListenAndServe(); err != nil {
		logger.Fatal().Err(err).Msg("Failed to start server")
	}
}

func loadClientTLSCredentials() credentials.TransportCredentials {
	// Load client certificates
	clientCert := []byte(os.Getenv("CLIENT_CERT"))
	clientKey := []byte(os.Getenv("CLIENT_KEY"))
	caCert := []byte(os.Getenv("CA_CERT"))

	// Create a certificate pool from the certificate authority
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		log.Fatalf("Failed to add CA certificate")
	}

	// Create the TLS credentials
	cert, err := tls.X509KeyPair(clientCert, clientKey)
	if err != nil {
		log.Fatalf("Failed to load client certificates: %v", err)
	}

	return credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      certPool,
	})
}

func getSecretFromService(key string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := secretClient.GetSecret(ctx, &pb.GetSecretRequest{Key: key})
	if err != nil {
		logger.Error().Str("key", key).Err(err).Msg("Failed to get secret from secret-service")
		return ""
	}

	return resp.Value
}

