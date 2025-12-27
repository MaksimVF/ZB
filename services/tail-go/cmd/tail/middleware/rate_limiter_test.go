package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/redis/go-redis/v9"
)

func TestRateLimiter(t *testing.T) {
	// Setup test Redis client
	testRedis := redis.NewClient(&redis.Options{
		Addr: "localhost:6379", // Use test Redis instance
	})

	// Override the global redisClient for testing
	originalRedis := redisClient
	redisClient = testRedis
	defer func() { redisClient = originalRedis }()

	// Clean up test keys
	defer testRedis.FlushAll(testRedis.Context())

	// Create test request
	req := httptest.NewRequest("GET", "/v1/chat/completions", nil)
	req.RemoteAddr = "127.0.0.1:12345"

	// Create test response recorder
	w := httptest.NewRecorder()

	// Test handler that just returns OK
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap with rate limiter
	rateLimitedHandler := RateLimiter(testHandler)

	// Test 1: First request should succeed
	rateLimitedHandler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Reset recorder
	w = httptest.NewRecorder()

	// Test 2: Second request should also succeed (within limits)
	rateLimitedHandler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Test 3: Health check should bypass rate limiting
	req.URL.Path = "/health"
	w = httptest.NewRecorder()
	rateLimitedHandler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Health check should bypass rate limiting, got %d", w.Code)
	}
}

func TestGetClientID(t *testing.T) {
	tests := []struct {
		name         string
		userID       string
		remoteAddr   string
		expectedType string
	}{
		{
			name:         "with user ID header",
			userID:       "user123",
			remoteAddr:   "127.0.0.1:12345",
			expectedType: "user",
		},
		{
			name:         "without user ID header",
			userID:       "",
			remoteAddr:   "127.0.0.1:12345",
			expectedType: "ip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.userID != "" {
				req.Header.Set("X-User-ID", tt.userID)
			}

			clientID := getClientID(req)

			if tt.expectedType == "user" && clientID != "user:"+tt.userID {
				t.Errorf("Expected client ID 'user:%s', got '%s'", tt.userID, clientID)
			}

			if tt.expectedType == "ip" && clientID != "ip:127.0.0.1" {
				t.Errorf("Expected client ID 'ip:127.0.0.1', got '%s'", clientID)
			}
		})
	}
	
	func BenchmarkRateLimiter(b *testing.B) {
		// Setup test Redis client
		testRedis := redis.NewClient(&redis.Options{
			Addr: "localhost:6379", // Use test Redis instance
		})
	
		// Override the global redisClient for testing
		originalRedis := redisClient
		redisClient = testRedis
		defer func() { redisClient = originalRedis }()
	
		// Clean up test keys
		defer testRedis.FlushAll(context.Background())
}

func BenchmarkRateLimiter(b *testing.B) {
	// Setup test Redis client
	testRedis := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	// Override the global redisClient for testing
	originalRedis := redisClient
	redisClient = testRedis
	defer func() { redisClient = originalRedis }()

	// Clean up test keys
	defer testRedis.FlushAll(context.Background())

	req := httptest.NewRequest("GET", "/v1/chat/completions", nil)
	req.RemoteAddr = "127.0.0.1:12345"

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	rateLimitedHandler := RateLimiter(testHandler)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			rateLimitedHandler.ServeHTTP(w, req)
		}
	})
}
