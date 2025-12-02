





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
	pb "llm-gateway-pro/services/secret-service/pb"
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
	}

	resilience.InitCircuitBreakers(circuitBreakerConfigs)

	r := mux.NewRouter()

	// LangChain-specific endpoint
	r.HandleFunc("/v1/langchain/chat/completions", handlers.LangChainCompletion).Methods("POST")

	// Standard OpenAI-compatible endpoint
	r.HandleFunc("/v1/chat/completions", handlers.ChatCompletion).Methods("POST")

	// Provider management endpoints
	r.HandleFunc("/v1/providers", handlers.ListProviders).Methods("GET")
	r.HandleFunc("/v1/providers", handlers.AddProvider).Methods("POST")
	r.HandleFunc("/v1/providers/{provider}", handlers.RemoveProvider).Methods("DELETE")

	// Circuit breaker endpoints
	r.HandleFunc("/v1/circuit-breakers", handlers.ListCircuitBreakers).Methods("GET")
	r.HandleFunc("/v1/circuit-breakers/{name}", handlers.GetCircuitBreakerStatus).Methods("GET")
	r.HandleFunc("/v1/circuit-breakers/{name}/reset", handlers.ResetCircuitBreaker).Methods("POST")

	// Health check
	r.HandleFunc("/health", HealthCheck).Methods("GET")

	// Metrics endpoint
	r.Handle("/metrics", promhttp.Handler())

	logger.Info().Msg("Gateway service starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}

func HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"healthy"}`))
}

// loadClientTLSCredentials loads client TLS credentials for mTLS
func loadClientTLSCredentials() credentials.TransportCredentials {
	// Load client certificate and key
	clientCert, err := tls.LoadX509KeyPair("/certs/gateway.pem", "/certs/gateway-key.pem")
	if err != nil {
		log.Fatalf("Failed to load client certificate: %v", err)
	}

	// Load CA certificate for server verification
	caCert, err := os.ReadFile("/certs/ca.pem")
	if err != nil {
		log.Fatalf("Failed to load CA certificate: %v", err)
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		log.Fatalf("Failed to add CA certificate to pool")
	}

	// Create TLS config with proper validation
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      certPool,
		ServerName:   "secret-service", // Must match CN in server certificate
		MinVersion:   tls.VersionTLS12,
	}

	return credentials.NewTLS(tlsConfig)
}

// getSecretFromService retrieves a secret from the secrets-service
func getSecretFromService(secretName string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := secretClient.GetSecret(ctx, &pb.GetSecretRequest{Name: secretName})
	if err != nil {
		logger.Error().Err(err).Str("secret_name", secretName).Msg("Failed to get secret from secrets-service")
		return ""
	}

	return resp.Value
}



