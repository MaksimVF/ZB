package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

var (
	auditLogFile *os.File
	redisClient  *redis.Client
	logger       *zap.Logger
)

func init() {
	// Initialize audit log file
	var err error
	auditLogFile, err = os.OpenFile("/var/log/routing_audit.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Failed to open audit log file: %v\n", err)
	}

	// Initialize Redis client for audit logging
	redisClient = redis.NewClient(&redis.Options{
		Addr: "redis:6379",
	})

	// Initialize logger
	logger, err = zap.NewProduction()
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
	}
}

// AuditLoggingMiddleware logs sensitive operations to audit log
func AuditLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		"/v1/admin",
		"/v1/policy",
		"/v1/head",
		"/v1/routing",
		"/v1/config",
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
		n, err := r.Body.Read(body)
		if err != nil && err != io.EOF {
			logger.Error("Failed to read request body for audit log", zap.Error(err))
		}
		r.Body.Close()
		r.Body = io.NopCloser(io.MultiReader(bytes.NewBuffer(body[:n]), r.Body))

		// Mask sensitive information in body
		entry.Body = maskSensitiveData(string(body[:n]))
	}

	// Validate audit log entry
	if err := validateAuditLogEntry(entry); err != nil {
		logger.Error("Invalid audit log entry", zap.Error(err))
		return
	}

	// Write to log file
	logEntry, err := json.Marshal(entry)
	if err != nil {
		logger.Error("Failed to marshal audit log entry", zap.Error(err))
		return
	}

	_, err = fmt.Fprintln(auditLogFile, string(logEntry))
	if err != nil {
		logger.Error("Failed to write audit log to file", zap.Error(err))
	}
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
	err := redisClient.Publish(ctx, "audit:routing:logs", logEntry).Err()
	if err != nil {
		logger.Error("Failed to publish audit log to Redis", zap.Error(err))
	}
}

// maskSensitiveData masks sensitive information in request data
func maskSensitiveData(data string) string {
	// Mask common sensitive patterns
	sensitivePatterns := []string{
		"password",
		"api_key",
		"secret",
		"token",
		"auth",
		"authorization",
	}

	for _, pattern := range sensitivePatterns {
		// Simple pattern matching - in production, use proper regex
		if strings.Contains(strings.ToLower(data), pattern) {
			// Replace the value with asterisks, keeping the key structure
			replacement := strings.ReplaceAll(data, pattern, strings.Repeat("*", len(pattern)))
			return replacement
		}
	}

	return data
}

// validateAuditLogEntry validates the audit log entry
func validateAuditLogEntry(entry AuditLogEntry) error {
	if entry.Timestamp == "" {
		return fmt.Errorf("timestamp is required")
	}

	if entry.Method == "" {
		return fmt.Errorf("method is required")
	}

	if entry.Path == "" {
		return fmt.Errorf("path is required")
	}

	if entry.ClientIP == "" {
		return fmt.Errorf("client IP is required")
	}

	// Validate that body doesn't contain sensitive information
	if strings.Contains(strings.ToLower(entry.Body), "password") ||
		strings.Contains(strings.ToLower(entry.Body), "api_key") ||
		strings.Contains(strings.ToLower(entry.Body), "secret") {
		return fmt.Errorf("body contains sensitive information")
	}

	return nil
}

// AuditLogEntry represents an audit log entry
type AuditLogEntry struct {
	Timestamp   string              `json:"timestamp"`
	Method      string              `json:"method"`
	Path        string              `json:"path"`
	ClientIP    string              `json:"client_ip"`
	UserAgent   string              `json:"user_agent"`
	QueryParams map[string][]string `json:"query_params"`
	Body        string              `json:"body,omitempty"`
}
