package middleware

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

var redisClient *redis.Client

func init() {
	redisClient = redis.NewClient(&redis.Options{
		Addr: "redis:6379",
	})
}

// RateLimiter implements simple rate limiting using Redis
func RateLimiter(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Skip health-check and static files
		if strings.HasPrefix(path, "/health") || strings.HasPrefix(path, "/static") {
			next(w, r)
			return
		}

		// Get client identifier (IP or user ID)
		clientID := getClientID(r)

		// Check rate limit
		allowed, retryAfter := checkRateLimit(clientID, path)
		if !allowed {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
			http.Error(w, `{"error": "rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}

		next(w, r)
	}
}

// getClientID extracts client identifier from request
func getClientID(r *http.Request) string {
	// Try to get user ID from header first
	if userID := r.Header.Get("X-User-ID"); userID != "" {
		return "user:" + userID
	}

	// Fall back to IP address
	return "ip:" + getClientIP(r)
}

// getClientIP extracts client IP address
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in case of multiple
		if idx := strings.Index(xff, ","); idx > 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr
	if idx := strings.LastIndex(r.RemoteAddr, ":"); idx > 0 {
		return r.RemoteAddr[:idx]
	}
	return r.RemoteAddr
}

// checkRateLimit checks if request is within rate limits
func checkRateLimit(clientID, path string) (bool, int) {
	ctx := context.Background()
	key := fmt.Sprintf("ratelimit:%s:%s", clientID, path)

	// Get current count
	count, err := redisClient.Get(ctx, key).Int()
	if err != nil && err != redis.Nil {
		log.Printf("Redis error in rate limiter: %v", err)
		return true, 0 // Allow on error
	}

	// Check limits based on path
	var limit int
	var window time.Duration

	switch {
	case strings.HasPrefix(path, "/v1/chat/completions"):
		limit = 60 // 60 requests per minute
		window = time.Minute
	case strings.HasPrefix(path, "/v1/embeddings"):
		limit = 100 // 100 requests per minute
		window = time.Minute
	default:
		limit = 100 // 100 requests per minute for other endpoints
		window = time.Minute
	}

	if count >= limit {
		// Check TTL to calculate retry-after
		ttl := redisClient.TTL(ctx, key).Val()
		if ttl < 0 {
			ttl = window
		}
		return false, int(ttl.Seconds())
	}

	// Increment counter
	pipe := redisClient.Pipeline()
	pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, window)
	_, err = pipe.Exec(ctx)
	if err != nil {
		log.Printf("Redis pipeline error in rate limiter: %v", err)
		return true, 0 // Allow on error
	}

	return true, 0
}
