




package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
)

var (
	// List of bad words and patterns to filter
	badWords = []string{"malicious", "exploit", "hack", "injection", "xss", "sql", "script"}
	badPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)<script.*?>.*?</script>`), // HTML script tags
		regexp.MustCompile(`(?i)SELECT.*FROM.*WHERE`),      // SQL injection patterns
		regexp.MustCompile(`(?i)UNION.*SELECT`),           // SQL injection patterns
		regexp.MustCompile(`(?i)javascript:`),              // JavaScript protocols
		regexp.MustCompile(`(?i)onerror=`),                // XSS patterns
	}
	redisClient = redis.NewClient(&redis.Options{
		Addr: "redis:6379",
	})
)

// SecurityConfig represents the security configuration for a client
type SecurityConfig struct {
	ContentFilteringEnabled bool `json:"content_filtering_enabled"`
	AuditLoggingEnabled    bool `json:"audit_logging_enabled"`
	DataIsolationEnabled   bool `json:"data_isolation_enabled"`
}

// ContentFilteringMiddleware filters and sanitizes incoming requests
func ContentFilteringMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if content filtering is enabled for this client
		if !isContentFilteringEnabled(r) {
			next.ServeHTTP(w, r)
			return
		}

		// Check URL path for sensitive endpoints
		if strings.Contains(r.URL.Path, "/v1/") {
			// Check for bad words in query parameters
			for _, param := range r.URL.Query() {
				for _, value := range param {
					if containsBadWords(value) || containsBadPatterns(value) {
						http.Error(w, "Request contains prohibited content", http.StatusBadRequest)
						return
					}
				}
			}

			// For POST/PUT requests, check the body content
			if r.Method == http.MethodPost || r.Method == http.MethodPut {
				// Read the body
				body := make([]byte, r.ContentLength)
				_, err := r.Body.Read(body)
				if err == nil {
					r.Body.Close()
					r.Body = io.NopCloser(bytes.NewBuffer(body))
				}

				if containsBadWords(string(body)) || containsBadPatterns(string(body)) {
					http.Error(w, "Request contains prohibited content", http.StatusBadRequest)
					return
				}
			}
		}

		next.ServeHTTP(w, r)
	})
}

// containsBadWords checks if text contains any prohibited words
func containsBadWords(text string) bool {
	lowerText := strings.ToLower(text)
	for _, word := range badWords {
		if strings.Contains(lowerText, word) {
			return true
		}
	}
	return false
}

// containsBadPatterns checks if text matches any prohibited patterns
func containsBadPatterns(text string) bool {
	for _, pattern := range badPatterns {
		if pattern.MatchString(text) {
			return true
		}
	}
	return false
}

// isContentFilteringEnabled checks if content filtering is enabled for the client
func isContentFilteringEnabled(r *http.Request) bool {
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

	return config.ContentFilteringEnabled
}




