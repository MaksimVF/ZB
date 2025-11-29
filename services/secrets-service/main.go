

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/hashicorp/vault/api"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	pb "llm-gateway-pro/services/secret-service/pb"
)

var (
	vaultClient   *api.Client
	logger        zerolog.Logger
	secretCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "secret_operations_total",
			Help: "Total number of secret operations",
		},
		[]string{"operation", "status"},
	)
	httpDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)
)

const (
	SecretNotFoundError    = "secret not found"
	PermissionDeniedError  = "permission denied"
	VaultConnectionError  = "vault connection error"
	InvalidInputError     = "invalid input"
	InternalServerError   = "internal server error"
)

func init() {
	// Initialize structured logger
	logger = zerolog.New(os.Stdout).With().
		Timestamp().
		Str("service", "secret-service").
		Logger()

	// Register Prometheus metrics
	prometheus.MustRegister(secretCounter, httpDuration)

	// Initialize Vault client
	config := api.DefaultConfig()
	config.Address = os.Getenv("VAULT_ADDR") // http://vault:8200
	client, err := api.NewClient(config)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize Vault client")
	}
	client.SetToken(os.Getenv("VAULT_TOKEN")) // token with proper rights

	// Test Vault connection
	_, err = client.Sys().Health()
	if err != nil {
		logger.Fatal().Err(err).Msg("Vault health check failed")
	}

	vaultClient = client
	logger.Info().Msg("Vault client initialized successfully")
}

// Custom error types
type SecretError struct {
	Code    codes.Code
	Message string
	Details string
}

func (e *SecretError) Error() string {
	return fmt.Sprintf("%s: %s", e.Message, e.Details)
}

func newSecretError(code codes.Code, message, details string) *SecretError {
	return &SecretError{Code: code, Message: message, Details: details}
}

// ===================== gRPC =====================
func (s *server) GetSecret(ctx context.Context, req *pb.GetSecretRequest) (*pb.GetSecretResponse, error) {
	logger.Info().
		Str("method", "GetSecret").
		Str("secret_name", req.Name).
		Msg("Received GetSecret request")

	// Validate input
	if req.Name == "" {
		err := newSecretError(codes.InvalidArgument, InvalidInputError, "secret name is required")
		secretCounter.WithLabelValues("get_secret", "error").Inc()
		return nil, status.Errorf(err.Code, "%s: %s", err.Message, err.Details)
	}

	// Get secret from Vault
	secret, err := vaultClient.Logical().Read("secret/data/" + req.Name)
	if err != nil {
		logger.Error().
			Err(err).
			Str("method", "GetSecret").
			Str("secret_name", req.Name).
			Msg("Vault read error")
		secretCounter.WithLabelValues("get_secret", "error").Inc()
		return nil, status.Errorf(codes.Internal, "%s: %s", VaultConnectionError, err.Error())
	}

	if secret == nil {
		err := newSecretError(codes.NotFound, SecretNotFoundError, fmt.Sprintf("secret %s not found", req.Name))
		secretCounter.WithLabelValues("get_secret", "not_found").Inc()
		return nil, status.Errorf(err.Code, "%s: %s", err.Message, err.Details)
	}

	data, ok := secret.Data["data"].(map[string]interface{})
	if !ok {
		err := newSecretError(codes.Internal, InternalServerError, "invalid data format in vault response")
		secretCounter.WithLabelValues("get_secret", "error").Inc()
		return nil, status.Errorf(err.Code, "%s: %s", err.Message, err.Details)
	}

	value, ok := data["value"].(string)
	if !ok {
		err := newSecretError(codes.Internal, InternalServerError, "invalid value format in vault response")
		secretCounter.WithLabelValues("get_secret", "error").Inc()
		return nil, status.Errorf(err.Code, "%s: %s", err.Message, err.Details)
	}

	logger.Info().
		Str("method", "GetSecret").
		Str("secret_name", req.Name).
		Msg("Secret retrieved successfully")

	secretCounter.WithLabelValues("get_secret", "success").Inc()
	return &pb.GetSecretResponse{Value: value}, nil
}

// ===================== HTTP Admin API =====================
func adminHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	logger.Info().
		Str("method", "adminHandler").
		Str("http_method", r.Method).
		Str("path", r.URL.Path).
		Msg("Received admin API request")

	// CORS handling
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,DELETE,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type,X-Admin-Key")

	if r.Method == http.MethodOptions {
		httpDuration.WithLabelValues(r.Method, r.URL.Path, "200").Observe(time.Since(start).Seconds())
		return
	}

	// Authentication check
	adminKey := r.Header.Get("X-Admin-Key")
	if adminKey == "" {
		logger.Warn().Str("method", "adminHandler").Msg("Missing admin key")
		http.Error(w, "forbidden: missing admin key", 403)
		httpDuration.WithLabelValues(r.Method, r.URL.Path, "403").Observe(time.Since(start).Seconds())
		return
	}

	if adminKey != os.Getenv("ADMIN_KEY") {
		logger.Warn().Str("method", "adminHandler").Msg("Invalid admin key")
		http.Error(w, "forbidden: invalid admin key", 403)
		httpDuration.WithLabelValues(r.Method, r.URL.Path, "403").Observe(time.Since(start).Seconds())
		return
	}

	switch r.Method {
	case http.MethodGet:
		handleGetSecrets(w, r, start)

	case http.MethodPost:
		handlePostSecret(w, r, start)

	case http.MethodDelete:
		handleDeleteSecret(w, r, start)

	default:
		logger.Warn().Str("method", "adminHandler").Str("http_method", r.Method).Msg("Invalid HTTP method")
		http.Error(w, "invalid method", 405)
		httpDuration.WithLabelValues(r.Method, r.URL.Path, "405").Observe(time.Since(start).Seconds())
	}
}

func handleGetSecrets(w http.ResponseWriter, r *http.Request, start time.Time) {
	logger.Info().Str("method", "handleGetSecrets").Msg("Listing secrets")

	secrets, err := vaultClient.Logical().List("secret/metadata/llm")
	if err != nil {
		logger.Error().Err(err).Msg("Failed to list secrets")
		http.Error(w, fmt.Sprintf("failed to list secrets: %v", err), 500)
		httpDuration.WithLabelValues(r.Method, r.URL.Path, "500").Observe(time.Since(start).Seconds())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(secrets); err != nil {
		logger.Error().Err(err).Msg("Failed to encode response")
		http.Error(w, "failed to encode response", 500)
		httpDuration.WithLabelValues(r.Method, r.URL.Path, "500").Observe(time.Since(start).Seconds())
		return
	}

	logger.Info().Int("count", len(secrets.Data["keys"].([]string))).Msg("Secrets listed successfully")
	httpDuration.WithLabelValues(r.Method, r.URL.Path, "200").Observe(time.Since(start).Seconds())
}

func handlePostSecret(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	logger.Info().Str("method", "handlePostSecret").Msg("Creating/updating secret")

	var input struct {
		Path  string `json:"path"`  // "llm/openai/api_key"
		Value string `json:"value"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		logger.Error().Err(err).Msg("Failed to decode request body")
		http.Error(w, fmt.Sprintf("invalid input: %v", err), 400)
		httpDuration.WithLabelValues(r.Method, r.URL.Path, "400").Observe(time.Since(start).Seconds())
		return
	}

	if input.Path == "" || input.Value == "" {
		logger.Error().Msg("Missing required fields in request")
		http.Error(w, "path and value are required", 400)
		httpDuration.WithLabelValues(r.Method, r.URL.Path, "400").Observe(time.Since(start).Seconds())
		return
	}

	_, err := vaultClient.Logical().Write("secret/data/"+input.Path, map[string]interface{}{
		"data": map[string]interface{}{"value": input.Value},
	})
	if err != nil {
		logger.Error().Err(err).Str("path", input.Path).Msg("Failed to write secret to Vault")
		http.Error(w, fmt.Sprintf("failed to save secret: %v", err), 500)
		httpDuration.WithLabelValues(r.Method, r.URL.Path, "500").Observe(time.Since(start).Seconds())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "saved"}); err != nil {
		logger.Error().Err(err).Msg("Failed to encode response")
		http.Error(w, "failed to encode response", 500)
		httpDuration.WithLabelValues(r.Method, r.URL.Path, "500").Observe(time.Since(start).Seconds())
		return
	}

	logger.Info().Str("path", input.Path).Msg("Secret saved successfully")
	httpDuration.WithLabelValues(r.Method, r.URL.Path, "200").Observe(time.Since(start).Seconds())
}

func handleDeleteSecret(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	name := r.URL.Path[len("/admin/api/secrets/"):]
	logger.Info().Str("method", "handleDeleteSecret").Str("secret_name", name).Msg("Deleting secret")

	if name == "" {
		logger.Error().Msg("Missing secret name in delete request")
		http.Error(w, "secret name is required", 400)
		httpDuration.WithLabelValues(r.Method, r.URL.Path, "400").Observe(time.Since(start).Seconds())
		return
	}

	_, err := vaultClient.Logical().Delete("secret/data/" + name)
	if err != nil {
		logger.Error().Err(err).Str("secret_name", name).Msg("Failed to delete secret")
		http.Error(w, fmt.Sprintf("failed to delete secret: %v", err), 500)
		httpDuration.WithLabelValues(r.Method, r.URL.Path, "500").Observe(time.Since(start).Seconds())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "deleted"}); err != nil {
		logger.Error().Err(err).Msg("Failed to encode response")
		http.Error(w, "failed to encode response", 500)
		httpDuration.WithLabelValues(r.Method, r.URL.Path, "500").Observe(time.Since(start).Seconds())
		return
	}

	logger.Info().Str("secret_name", name).Msg("Secret deleted successfully")
	httpDuration.WithLabelValues(r.Method, r.URL.Path, "200").Observe(time.Since(start).Seconds())
}

// Health check handler
func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Check Vault health
	health, err := vaultClient.Sys().Health()
	if err != nil {
		logger.Error().Err(err).Msg("Vault health check failed")
		http.Error(w, "vault unhealthy", 503)
		httpDuration.WithLabelValues(r.Method, r.URL.Path, "503").Observe(time.Since(start).Seconds())
		return
	}

	if !health.Initialized || health.Sealed {
		logger.Error().Msg("Vault is not initialized or sealed")
		http.Error(w, "vault not ready", 503)
		httpDuration.WithLabelValues(r.Method, r.URL.Path, "503").Observe(time.Since(start).Seconds())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "healthy"}); err != nil {
		logger.Error().Err(err).Msg("Failed to encode health response")
		http.Error(w, "failed to encode response", 500)
		httpDuration.WithLabelValues(r.Method, r.URL.Path, "500").Observe(time.Since(start).Seconds())
		return
	}

	logger.Info().Msg("Health check passed")
	httpDuration.WithLabelValues(r.Method, r.URL.Path, "200").Observe(time.Since(start).Seconds())
}

func main() {
	init()

	// gRPC (mTLS)
	lis, err := net.Listen("tcp", ":50053")
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to listen on TCP port 50053")
	}

	creds, err := credentials.NewServerTLSFromFile("/certs/secret-service.pem", "/certs/secret-service-key.pem")
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load TLS credentials")
	}

	grpcServer := grpc.NewServer(grpc.Creds(creds))
	pb.RegisterSecretServiceServer(grpcServer, &server{})

	go func() {
		logger.Info().Msg("Starting gRPC server on :50053")
		if err := grpcServer.Serve(lis); err != nil {
			logger.Fatal().Err(err).Msg("gRPC server failed")
		}
	}()

	// HTTP Admin API
	http.HandleFunc("/admin/api/secrets", adminHandler)
	http.HandleFunc("/admin/api/secrets/", adminHandler)

	// Health check endpoint
	http.HandleFunc("/health", healthCheckHandler)

	// Prometheus metrics endpoint
	http.Handle("/metrics", promhttp.Handler())

	logger.Info().Msg("Starting HTTP server on :8082")
	if err := http.ListenAndServe(":8082", nil); err != nil {
		logger.Fatal().Err(err).Msg("HTTP server failed")
	}
}

