







package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

var (
	auditLogFile *os.File
	redisClient  *redis.Client
)

func init() {
	// Initialize audit log file
	var err error
	auditLogFile, err = os.OpenFile("/var/log/audit.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Failed to open audit log file: %v", err)
	}

	// Initialize Redis client for audit logging
	redisClient = redis.NewClient(&redis.Options{
		Addr: "redis:6379",
	})
}

// AuditLoggingMiddleware logs sensitive operations to audit log
func AuditLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if audit logging is enabled for this client
		if !isAuditLoggingEnabled(r) {
			next.ServeHTTP(w, r)
			return
		}

		// Check if this is a sensitive operation
		if isSensitiveOperation(r) {
			// Log to file
			logAuditToFile(r)

			// Log to Redis
			logAuditToRedis(r)
		}

		next.ServeHTTP(w, r)
	})
}

// isSensitiveOperation checks if the request is a sensitive operation
func isSensitiveOperation(r *http.Request) bool {
	sensitivePaths := []string{
		"/v1/agentic",
		"/v1/providers",
		"/v1/secrets",
		"/v1/billing",
		"/v1/admin",
	}

	for _, path := range sensitivePaths {
		if strings.Contains(r.URL.Path, path) {
			return true
		}
	}

	return false
}

// logAuditToFile logs audit information to a file
func logAuditToFile(r *http.Request) {
	if auditLogFile == nil {
		return
	}

	// Prepare audit log entry
	entry := AuditLogEntry{
		Timestamp:   time.Now().Format(time.RFC3339),
		Method:      r.Method,
		Path:        r.URL.Path,
		ClientIP:    r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		QueryParams: r.URL.Query(),
	}

	// For POST/PUT requests, log body content (truncated)
	if r.Method == http.MethodPost || r.Method == http.MethodPut {
		body := make([]byte, 1024) // Limit to 1KB
		_, _ = r.Body.Read(body)
		r.Body.Close()
		r.Body = io.NopCloser(bytes.NewBuffer(body))
		entry.Body = string(body)
	}

	// Write to log file
	logEntry, _ := json.Marshal(entry)
	fmt.Fprintln(auditLogFile, string(logEntry))
}

// logAuditToRedis logs audit information to Redis
func logAuditToRedis(r *http.Request) {
	// Prepare audit log entry
	entry := AuditLogEntry{
		Timestamp:   time.Now().Format(time.RFC3339),
		Method:      r.Method,
		Path:        r.URL.Path,
		ClientIP:    r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		QueryParams: r.URL.Query(),
	}

	// For POST/PUT requests, log body content (truncated)
	if r.Method == http.MethodPost || r.Method == http.MethodPut {
		body := make([]byte, 1024) // Limit to 1KB
		_, _ = r.Body.Read(body)
		r.Body.Close()
		r.Body = io.NopCloser(bytes.NewBuffer(body))
		entry.Body = string(body)
	}

	// Publish to Redis channel
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	logEntry, _ := json.Marshal(entry)
	redisClient.Publish(ctx, "audit:logs", logEntry)
}

// isAuditLoggingEnabled checks if audit logging is enabled for the client
func isAuditLoggingEnabled(r *http.Request) bool {
	// Get client ID from context
	clientID := r.Context().Value("client_id")
	if clientID == nil {
		return true // Default to enabled if no client ID
	}

	// Get security config from Redis
	ctx := r.Context()
	configKey := "client:" + clientID.(string) + ":security_config"

	val, err := redisClient.Get(ctx, configKey).Result()
	if err != nil {
		return true // Default to enabled if config not found
	}

	// Parse config
	var config SecurityConfig
	err = json.Unmarshal([]byte(val), &config)
	if err != nil {
		return true // Default to enabled if parsing fails
	}

	return config.AuditLoggingEnabled
}

// AuditLogEntry represents an audit log entry
type AuditLogEntry struct {
	Timestamp   string            `json:"timestamp"`
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	ClientIP    string            `json:"client_ip"`
	UserAgent   string            `json:"user_agent"`
	QueryParams map[string][]string `json:"query_params"`
	Body        string            `json:"body,omitempty"`
}







