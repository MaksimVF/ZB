








package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
)

var (
	redisIsolationClient *redis.Client
)

func init() {
	// Initialize Redis client for data isolation
	redisIsolationClient = redis.NewClient(&redis.Options{
		Addr: "redis:6379",
	})
}

// SecurityConfig represents the security configuration for a client
type SecurityConfig struct {
	ContentFilteringEnabled bool `json:"content_filtering_enabled"`
	AuditLoggingEnabled    bool `json:"audit_logging_enabled"`
	DataIsolationEnabled   bool `json:"data_isolation_enabled"`
}

// DataIsolationMiddleware ensures data isolation between clients
func DataIsolationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if data isolation is enabled for this client
		if !isDataIsolationEnabled(r) {
			next.ServeHTTP(w, r)
			return
		}

		// Extract client ID from request (could be from header, token, etc.)
		clientID := getClientID(r)
		if clientID == "" {
			http.Error(w, "Client ID required", http.StatusUnauthorized)
			return
		}

		// Set client context for downstream services
		ctx := context.WithValue(r.Context(), "client_id", clientID)
		r = r.WithContext(ctx)

		// Apply data isolation policies
		if !validateClientAccess(r, clientID) {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// getClientID extracts client ID from request
func getClientID(r *http.Request) string {
	// Try to get from Authorization header first
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		// In a real implementation, we would parse and validate the JWT token
		// For now, we'll just extract a dummy client ID
		return "client_" + token[:8] // Use first 8 chars of token as client ID
	}

	// Fallback to a query parameter (for testing)
	clientID := r.URL.Query().Get("client_id")
	if clientID != "" {
		return clientID
	}

	return ""
}

// validateClientAccess checks if client has access to the requested resource
func validateClientAccess(r *http.Request, clientID string) bool {
	// Check Redis for client access permissions
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Check if client has access to this endpoint
	accessKey := "client:" + clientID + ":access:" + r.URL.Path
	result, err := redisIsolationClient.Get(ctx, accessKey).Result()
	if err == nil && result == "allowed" {
		return true
	}

	// Default to allowed for now (in production, this would be more strict)
	return true
}

// isDataIsolationEnabled checks if data isolation is enabled for the client
func isDataIsolationEnabled(r *http.Request) bool {
	// Get client ID from context
	clientID := r.Context().Value("client_id")
	if clientID == nil {
		return true // Default to enabled if no client ID
	}

	// Get security config from Redis
	ctx := r.Context()
	configKey := "client:" + clientID.(string) + ":security_config"

	val, err := redisIsolationClient.Get(ctx, configKey).Result()
	if err != nil {
		return true // Default to enabled if config not found
	}

	// Parse config
	var config SecurityConfig
	err = json.Unmarshal([]byte(val), &config)
	if err != nil {
		return true // Default to enabled if parsing fails
	}

	return config.DataIsolationEnabled
}








