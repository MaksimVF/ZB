




package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	pb "github.com/MaksimVF/ZB/services/secrets-service/pb"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/hashicorp/vault/api"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
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
	// Rate limiting for admin API
	adminAPILimiter = make(map[string]int64)
	adminKeyRegex   = regexp.MustCompile(`^[a-zA-Z0-9\-_]{16,64}$`) // Admin key must be 16-64 chars, alphanumeric with dashes/underscores
)

const (
	SecretNotFoundError   = "secret not found"
	PermissionDeniedError = "permission denied"
	VaultConnectionError  = "vault connection error"
	InvalidInputError     = "invalid input"
	InternalServerError   = "internal server error"
)

// Helper function to check if an origin is allowed
func isOriginAllowed(origin, allowedOrigins string) bool {
	// Split allowed origins by comma and check if the request origin is in the list
	for _, allowedOrigin := range strings.Split(allowedOrigins, ",") {
		if strings.TrimSpace(allowedOrigin) == origin {
			return true
		}
	}
	return false
}

// Centralized error handler for gRPC methods
func handleGRPCError(ctx context.Context, err error, operation string) error {
	logger.Error().Err(err).Str("operation", operation).Msg("Operation failed")

	// Check for specific error types and convert to gRPC status
	switch {
	case strings.Contains(err.Error(), "not found"):
		secretCounter.WithLabelValues(operation, "not_found").Inc()
		return status.Errorf(codes.NotFound, "%s: %s", SecretNotFoundError, err.Error())
	case strings.Contains(err.Error(), "permission denied"):
		secretCounter.WithLabelValues(operation, "permission_denied").Inc()
		return status.Errorf(codes.PermissionDenied, "%s: %s", PermissionDeniedError, err.Error())
	case strings.Contains(err.Error(), "invalid"):
		secretCounter.WithLabelValues(operation, "invalid_input").Inc()
		return status.Errorf(codes.InvalidArgument, "%s: %s", InvalidInputError, err.Error())
	default:
		secretCounter.WithLabelValues(operation, "error").Inc()
		return status.Errorf(codes.Internal, "%s: %s", InternalServerError, err.Error())
	}
}

// Helper functions for Vault KV v2 operations
func getSecretFromVault(path string) (*api.Secret, error) {
	return vaultClient.KVv2("secret").Get(context.Background(), path)
}

func putSecretInVault(path string, data map[string]interface{}) error {
	_, err := vaultClient.KVv2("secret").Put(context.Background(), path, data)
	return err
}

func deleteSecretFromVault(path string) error {
	return vaultClient.KVv2("secret").Delete(context.Background(), path)
}

func listSecretsFromVault(prefix string) (*api.Secret, error) {
	return vaultClient.KVv2("secret").List(context.Background(), prefix)
}

// Helper function to get client IP address
func getClientIP(r *http.Request) string {
	// Try to get IP from X-Forwarded-For header first
	xForwardedFor := r.Header.Get("X-Forwarded-For")
	if xForwardedFor != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		ips := strings.Split(xForwardedFor, ",")
		return strings.TrimSpace(ips[0])
	}

	// Fall back to remote address
	remoteAddr := r.RemoteAddr
	if strings.Contains(remoteAddr, ":") {
		// Handle IPv6 or port suffix
		remoteAddr, _, err := net.SplitHostPort(remoteAddr)
		if err != nil {
			return remoteAddr
		}
		return remoteAddr
	}

	return remoteAddr
}

// Helper function to handle HTTP errors consistently
func handleHTTPError(w http.ResponseWriter, r *http.Request, statusCode int, errorMsg string, start time.Time) {
	clientIP := getClientIP(r)
	logger.Error().
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("client_ip", clientIP).
		Str("error", errorMsg).
		Msg("HTTP error")

	http.Error(w, fmt.Sprintf("error: %s", errorMsg), statusCode)
	httpDuration.WithLabelValues(r.Method, r.URL.Path, fmt.Sprintf("%d", statusCode)).Observe(time.Since(start).Seconds())
}

func init() {
	// Initialize structured logger
	logger = zerolog.New(os.Stdout).With().
		Timestamp().
		Str("service", "secret-service").
		Logger()

	// Register Prometheus metrics
	prometheus.MustRegister(secretCounter, httpDuration)

	// Initialize OpenTelemetry
	initOpenTelemetry()

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

func initOpenTelemetry() {
	// Create a new stdout exporter
	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create stdout exporter")
	}

	// Create a new tracer provider with a batch span processor and the stdout exporter
	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("secret-service"),
		)),
	)

	// Set the global tracer provider
	otel.SetTracerProvider(tp)

	logger.Info().Msg("OpenTelemetry initialized successfully")
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

// server implements the gRPC SecretServiceServer interface.
// Embed UnimplementedSecretServiceServer to ensure forward compatibility
// when new methods are added to the service definition.
type server struct {
	pb.UnimplementedSecretServiceServer
}

// ===================== gRPC =====================
func (s *server) GetSecret(ctx context.Context, req *pb.GetSecretRequest) (*pb.GetSecretResponse, error) {
	// Start a new span for the GetSecret operation
	tracer := otel.Tracer("secret-service")
	ctx, span := tracer.Start(ctx, "GetSecret")
	defer span.End()

	logger.Info().
		Str("method", "GetSecret").
		Str("secret_name", req.Name).
		Msg("Received GetSecret request")

	// Validate input
	if req.Name == "" {
		err := fmt.Errorf("secret name is required")
		return nil, handleGRPCError(ctx, err, "get_secret")
	}

	// Get secret from Vault using KV v2
	secret, err := getSecretFromVault(req.Name)
	if err != nil {
		return nil, handleGRPCError(ctx, err, "get_secret")
	}

	if secret == nil {
		err := fmt.Errorf("secret %s not found", req.Name)
		return nil, handleGRPCError(ctx, err, "get_secret")
	}

	value, ok := secret.Data["value"].(string)
	if !ok {
		err := fmt.Errorf("invalid value format in vault response")
		return nil, handleGRPCError(ctx, err, "get_secret")
	}

	// Add metadata to the response
	metadata := map[string]string{
		"source": "vault",
		"path":   "secret/data/" + req.Name,
	}

	// Try to extract version and creation time if available
	if metadataRaw, ok := secret.Data["metadata"].(map[string]interface{}); ok {
		if version, ok := metadataRaw["version"].(float64); ok {
			metadata["version"] = fmt.Sprintf("%d", int(version))
		}
		if createdTime, ok := metadataRaw["created_time"].(string); ok {
			metadata["created_at"] = createdTime
		}
		if updatedTime, ok := metadataRaw["updated_time"].(string); ok {
			metadata["last_updated"] = updatedTime
		}
	}

	logger.Info().
		Str("method", "GetSecret").
		Str("secret_name", req.Name).
		Msg("Secret retrieved successfully")

	secretCounter.WithLabelValues("get_secret", "success").Inc()
	return &pb.GetSecretResponse{Value: value, Metadata: metadata}, nil
}

func (s *server) GetUserSecret(ctx context.Context, req *pb.GetUserSecretRequest) (*pb.GetUserSecretResponse, error) {
	logger.Info().
		Str("method", "GetUserSecret").
		Str("user_id", req.UserId).
		Str("secret_name", req.SecretName).
		Msg("Received GetUserSecret request")

	// Validate input
	if req.UserId == "" || req.SecretName == "" {
		err := fmt.Errorf("user_id and secret_name are required")
		return nil, handleGRPCError(ctx, err, "get_user_secret")
	}

	// Get user-specific secret from Vault
	secretPath := fmt.Sprintf("user-secrets/data/%s/%s", req.UserId, req.SecretName)
	secret, err := vaultClient.Logical().Read(secretPath)
	if err != nil {
		return nil, handleGRPCError(ctx, err, "get_user_secret")
	}

	if secret == nil {
		err := fmt.Errorf("user secret %s/%s not found", req.UserId, req.SecretName)
		return nil, handleGRPCError(ctx, err, "get_user_secret")
	}

	data, ok := secret.Data["data"].(map[string]interface{})
	if !ok {
		err := fmt.Errorf("invalid data format in vault response")
		return nil, handleGRPCError(ctx, err, "get_user_secret")
	}

	value, ok := data["value"].(string)
	if !ok {
		err := fmt.Errorf("invalid value format in vault response")
		return nil, handleGRPCError(ctx, err, "get_user_secret")
	}

	logger.Info().
		Str("method", "GetUserSecret").
		Str("user_id", req.UserId).
		Str("secret_name", req.SecretName).
		Msg("User secret retrieved successfully")

	secretCounter.WithLabelValues("get_user_secret", "success").Inc()
	return &pb.GetUserSecretResponse{Value: value}, nil
}

func (s *server) SetUserSecret(ctx context.Context, req *pb.SetUserSecretRequest) (*pb.SetUserSecretResponse, error) {
	logger.Info().
		Str("method", "SetUserSecret").
		Str("user_id", req.UserId).
		Str("secret_name", req.SecretName).
		Msg("Received SetUserSecret request")

	// Validate input
	if req.UserId == "" || req.SecretName == "" || req.SecretValue == "" {
		err := fmt.Errorf("user_id, secret_name, and secret_value are required")
		return nil, handleGRPCError(ctx, err, "set_user_secret")
	}

	// Store user-specific secret in Vault
	secretPath := fmt.Sprintf("user-secrets/data/%s/%s", req.UserId, req.SecretName)
	_, err := vaultClient.Logical().Write(secretPath, map[string]interface{}{
		"data": map[string]interface{}{"value": req.SecretValue},
	})
	if err != nil {
		return nil, handleGRPCError(ctx, err, "set_user_secret")
	}

	logger.Info().
		Str("method", "SetUserSecret").
		Str("user_id", req.UserId).
		Str("secret_name", req.SecretName).
		Msg("User secret saved successfully")

	secretCounter.WithLabelValues("set_user_secret", "success").Inc()
	return &pb.SetUserSecretResponse{Status: "saved"}, nil
}

// ===================== HTTP Admin API =====================
func adminHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Start a new span for the admin handler
	tracer := otel.Tracer("secret-service")
	ctx, span := tracer.Start(r.Context(), "adminHandler")
	defer span.End()
	r = r.WithContext(ctx)

	logger.Info().
		Str("method", "adminHandler").
		Str("http_method", r.Method).
		Str("path", r.URL.Path).
		Msg("Received admin API request")

	// CORS handling - allow only specific origins from configuration
	allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
	if allowedOrigins == "" {
		allowedOrigins = "http://localhost:3000,http://localhost:3001" // Default to UI services
	}
	origin := r.Header.Get("Origin")
	if origin != "" && isOriginAllowed(origin, allowedOrigins) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	}
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,DELETE,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type,X-Admin-Key")

	if r.Method == http.MethodOptions {
		httpDuration.WithLabelValues(r.Method, r.URL.Path, "200").Observe(time.Since(start).Seconds())
		return
	}

	// Rate limiting for admin API
	clientIP := getClientIP(r)
	currentTime := time.Now().Unix()
	if lastRequestTime, exists := adminAPILimiter[clientIP]; exists {
		if currentTime-lastRequestTime < 5 { // Allow 1 request per 5 seconds per IP
			logger.Warn().Str("method", "adminHandler").Str("client_ip", clientIP).Msg("Rate limit exceeded")
			http.Error(w, "rate limit exceeded: too many requests", 429)
			httpDuration.WithLabelValues(r.Method, r.URL.Path, "429").Observe(time.Since(start).Seconds())
			return
		}
	}
	adminAPILimiter[clientIP] = currentTime

	// Authentication check
	adminKey := r.Header.Get("X-Admin-Key")
	if adminKey == "" {
		logger.Warn().Str("method", "adminHandler").Msg("Missing admin key")
		http.Error(w, "forbidden: missing admin key", 403)
		httpDuration.WithLabelValues(r.Method, r.URL.Path, "403").Observe(time.Since(start).Seconds())
		return
	}

	// Validate admin key format
	if !adminKeyRegex.MatchString(adminKey) {
		logger.Warn().Str("method", "adminHandler").Msg("Invalid admin key format")
		http.Error(w, "forbidden: invalid admin key format", 403)
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
	clientIP := getClientIP(r)
	logger.Info().Str("method", "handleGetSecrets").Str("client_ip", clientIP).Msg("Listing secrets")

	secrets, err := listSecretsFromVault("llm")
	if err != nil {
		handleHTTPError(w, r, 500, fmt.Sprintf("failed to list secrets: %v", err), start)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(secrets); err != nil {
		handleHTTPError(w, r, 500, "failed to encode response", start)
		return
	}

	logger.Info().Str("client_ip", clientIP).Int("count", len(secrets.Data["keys"].([]string))).Msg("Secrets listed successfully")
	httpDuration.WithLabelValues(r.Method, r.URL.Path, "200").Observe(time.Since(start).Seconds())
}

func handlePostSecret(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	clientIP := getClientIP(r)
	logger.Info().Str("method", "handlePostSecret").Str("client_ip", clientIP).Msg("Creating/updating secret")

	var input struct {
		Path  string `json:"path"` // "llm/openai/api_key"
		Value string `json:"value"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		logger.Error().Err(err).Str("client_ip", clientIP).Msg("Failed to decode request body")
		http.Error(w, fmt.Sprintf("invalid input: %v", err), 400)
		httpDuration.WithLabelValues(r.Method, r.URL.Path, "400").Observe(time.Since(start).Seconds())
		return
	}

	// Enhanced input validation
	if input.Path == "" || input.Value == "" {
		errMsg := "path and value are required"
		logger.Error().Str("error", errMsg).Msg("Missing required fields in request")
		http.Error(w, fmt.Sprintf("invalid_request: %s", errMsg), 400)
		httpDuration.WithLabelValues(r.Method, r.URL.Path, "400").Observe(time.Since(start).Seconds())
		return
	}

	// Validate path format - only allow alphanumeric, dashes, underscores, and slashes
	pathRegex := regexp.MustCompile(`^[a-zA-Z0-9\-_/]+$`)
	if !pathRegex.MatchString(input.Path) {
		logger.Error().Str("path", input.Path).Msg("Invalid path format")
		http.Error(w, "invalid path format: only alphanumeric, dashes, underscores, and slashes allowed", 400)
		httpDuration.WithLabelValues(r.Method, r.URL.Path, "400").Observe(time.Since(start).Seconds())
		return
	}

	// Validate value length
	if len(input.Value) > 4096 { // Limit secret value size
		logger.Error().Msg("Secret value too long")
		http.Error(w, "secret value too long: max 4096 characters", 400)
		httpDuration.WithLabelValues(r.Method, r.URL.Path, "400").Observe(time.Since(start).Seconds())
		return
	}

	err := putSecretInVault(input.Path, map[string]interface{}{"value": input.Value})
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
	clientIP := getClientIP(r)
	name := r.URL.Path[len("/admin/api/secrets/"):]
	logger.Info().Str("method", "handleDeleteSecret").Str("client_ip", clientIP).Str("secret_name", name).Msg("Deleting secret")

	if name == "" {
		errMsg := "secret name is required"
		logger.Error().Str("client_ip", clientIP).Str("error", errMsg).Msg("Missing secret name in delete request")
		http.Error(w, fmt.Sprintf("invalid_request: %s", errMsg), 400)
		httpDuration.WithLabelValues(r.Method, r.URL.Path, "400").Observe(time.Since(start).Seconds())
		return
	}

	// Validate secret name format - only allow alphanumeric, dashes, underscores, and slashes
	pathRegex := regexp.MustCompile(`^[a-zA-Z0-9\-_/]+$`)
	if !pathRegex.MatchString(name) {
		errMsg := "invalid secret name format: only alphanumeric, dashes, underscores, and slashes allowed"
		logger.Error().Str("client_ip", clientIP).Str("secret_name", name).Str("error", errMsg).Msg("Invalid secret name format")
		http.Error(w, fmt.Sprintf("invalid_request: %s", errMsg), 400)
		httpDuration.WithLabelValues(r.Method, r.URL.Path, "400").Observe(time.Since(start).Seconds())
		return
	}

	err := deleteSecretFromVault(name)
	if err != nil {
		handleHTTPError(w, r, 500, fmt.Sprintf("failed to delete secret: %v", err), start)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "deleted"}); err != nil {
		handleHTTPError(w, r, 500, "failed to encode response", start)
		return
	}

	logger.Info().Str("client_ip", clientIP).Str("secret_name", name).Msg("Secret deleted successfully")
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

	// HTTP Admin API with chi router
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// CORS middleware
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// CORS handling - allow only specific origins from configuration
			allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
			if allowedOrigins == "" {
				allowedOrigins = "http://localhost:3000,http://localhost:3001" // Default to UI services
			}
			origin := r.Header.Get("Origin")
			if origin != "" && isOriginAllowed(origin, allowedOrigins) {
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
	})

	// Routes
	r.Route("/admin/api", func(r chi.Router) {
		r.Post("/secrets", adminHandler)
		r.Get("/secrets", adminHandler)
		r.Delete("/secrets/{name}", adminHandler)
	})

	// Health check endpoint
	r.Get("/health", healthCheckHandler)

	// Prometheus metrics endpoint
	r.Handle("/metrics", promhttp.Handler())

	logger.Info().Msg("Starting HTTP server on :8082")
	if err := http.ListenAndServe(":8082", r); err != nil {
		logger.Fatal().Err(err).Msg("HTTP server failed")
	}
}




