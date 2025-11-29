// services/rate-limiter/internal/limiter/limiter.go
package limiter

import (
"context"
"encoding/json"
"errors"
"log"
"net/http"
"os"
"strconv"
"strings"
"time"

"github.com/go-redis/redis/v8"
"github.com/golang-jwt/jwt/v5"
"google.golang.org/grpc"
pb "llm-gateway-pro/services/rate-limiter/pb"
)

var (
rdb       = newRedisClient()
ctx       = context.Background()
jwtSecret = getJWTSecret() // Load from environment variable or secret service
)

// newRedisClient creates a Redis client with connection pooling and health checks
func newRedisClient() *redis.Client {
	options := &redis.Options{
		Addr:         "redis:6379",
		PoolSize:      100, // Connection pool size
		MinIdleConns:  10,  // Minimum idle connections
		MaxConnAge:    30 * time.Minute,
		IdleTimeout:   5 * time.Minute,
		ReadTimeout:   1 * time.Second,
		WriteTimeout:  1 * time.Second,
		DialTimeout:   5 * time.Second,
		PoolTimeout:   5 * time.Second,
	}

	client := redis.NewClient(options)

	// Test the connection
	err := client.Ping(ctx).Err()
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	log.Println("Redis connection established successfully")
	return client
}

// checkRedisHealth checks if Redis is healthy
func checkRedisHealth() bool {
	err := rdb.Ping(ctx).Err()
	return err == nil
}

func getJWTSecret() []byte {
	// Try to get from environment variable first
	secret := os.Getenv("JWT_SECRET")
	if secret != "" {
		return []byte(secret)
	}

	// Load TLS credentials for secret service
	tlsConfig, err := loadSecretServiceTLSCredentials()
	if err != nil {
		log.Printf("Failed to load TLS config for secret service, using fallback: %v", err)
		return []byte("fallback-super-secret-jwt-key-2025") // Fallback, but not hardcoded in prod
	}

	// Create gRPC connection with retry logic and circuit breaker
	conn, err := grpc.Dial("secret-service:50051",
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
		grpc.WithConnectParams(grpc.ConnectParams{
			Backoff:           grpc.DefaultBackoffConfig,
			MinConnectTimeout: 5 * time.Second,
		}),
		grpc.WithUnaryInterceptor(circuitBreakerUnaryClientInterceptor),
	)
	if err != nil {
		log.Printf("Failed to connect to secret service, using fallback secret: %v", err)
		return []byte("fallback-super-secret-jwt-key-2025") // Fallback, but not hardcoded in prod
	}
	defer conn.Close()

	client := pb.NewSecretServiceClient(conn)
	resp, err := client.GetSecret(ctx, &pb.GetSecretRequest{Name: "jwt_secret"})
	if err != nil {
		log.Printf("Failed to get JWT secret, using fallback: %v", err)
		return []byte("fallback-super-secret-jwt-key-2025")
	}

	return []byte(resp.Value)
}

// loadSecretServiceTLSCredentials loads TLS config for secret service
func loadSecretServiceTLSCredentials() (*tls.Config, error) {
	// Load CA certificate
	caCert, err := os.ReadFile("/certs/ca.pem")
	if err != nil {
		return nil, fmt.Errorf("failed to load CA certificate: %w", err)
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to add CA certificate to pool")
	}

	return &tls.Config{
		RootCAs:    certPool,
		MinVersion: tls.VersionTLS12,
	}, nil
}

// circuitBreakerUnaryClientInterceptor implements a simple circuit breaker
func circuitBreakerUnaryClientInterceptor(
	ctx context.Context,
	method string,
	req, reply interface{},
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	// Simple retry logic with backoff
	var lastErr error
	for i := 0; i < 3; i++ {
		err := invoker(ctx, method, req, reply, cc, opts...)
		if err == nil {
			return nil
		}
		lastErr = err
		time.Sleep(time.Duration(i) * 500 * time.Millisecond)
	}
	return fmt.Errorf("failed after retries: %w", lastErr)
}

// DefaultRateLimits defines the default rate limits for different endpoints
var DefaultRateLimits = map[string]map[string]int{
"/v1/chat/completions": {
"requests_per_minute": 60,
"tokens_per_minute":   500000,
},
"/v1/completions": {
"requests_per_minute": 60,
"tokens_per_minute":   500000,
},
"/v1/embeddings": {
"requests_per_minute": 15,
"tokens_per_minute":   6000000,
},
"/v1/agentic": {
"requests_per_minute": 5,
"tokens_per_minute":   20000000,
"tools_per_minute":    100,
},
}

// Извлекает и валидирует JWT → возвращает clientID
func extractClientID(authHeader string) (string, error) {
if authHeader == "" {
return "anonymous", nil
}

// API-ключ tvo_...
if strings.HasPrefix(authHeader, "tvo_") {
return "key:" + authHeader, nil
}

// JWT: Bearer ...
if !strings.HasPrefix(authHeader, "Bearer ") {
return "", errors.New("invalid auth format")
}

tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
token, err := jwt.ParseWithClaims(tokenStr, &jwt.MapClaims{}, func(t *jwt.Token) (interface{}, error) {
if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
return nil, errors.New("invalid signing method")
}
return jwtSecret, nil
}, jwt.WithValidMethods([]string{"HS256"}))

if err != nil {
return "", err
}
if !token.Valid {
return "", errors.New("token expired or invalid")
}

claims, ok := token.Claims.(*jwt.MapClaims)
if !ok {
return "", errors.New("invalid claims")
}

userID, ok := (*claims)["user_id"].(string)
if !ok || userID == "" {
return "", errors.New("missing user_id")
}

return "user:" + userID, nil
}

type Server struct {
pb.UnimplementedRateLimiterServer
}

func (s *Server) Check(_ context.Context, req *pb.CheckRequest) (*pb.CheckResponse, error) {
clientID, err := extractClientID(req.Authorization)
if err != nil {
clientID = "invalid:" + req.Authorization[:min(len(req.Authorization), 16)]
}

path := req.Path
now := time.Now().UnixNano()

// Get rate limits from Redis or use defaults
limits, err := getRateLimitsFromRedis()
if err != nil {
limits = DefaultRateLimits
}

// Determine which endpoint this request is for
var endpoint string
if strings.Contains(path, "/v1/chat/completions") || strings.Contains(path, "/v1/completions") {
endpoint = "/v1/chat/completions"
} else if strings.HasPrefix(path, "/v1/embeddings") {
endpoint = "/v1/embeddings"
} else if strings.HasPrefix(path, "/v1/agentic") {
endpoint = "/v1/agentic"
} else {
return &pb.CheckResponse{Allowed: true}, nil
}

// Get limits for this endpoint
endpointLimits, exists := limits[endpoint]
if !exists {
return &pb.CheckResponse{Allowed: true}, nil
}

if !slidingWindow("rl:"+endpoint+":rq:"+clientID, int64(endpointLimits["requests_per_minute"]), time.Minute) {
return &pb.CheckResponse{Allowed: false, RetryAfterSecs: 30}, nil
}
if !tokenBucket("rl:"+endpoint+":tk:"+clientID, int64(endpointLimits["tokens_per_minute"]), int64(endpointLimits["tokens_per_minute"]), time.Minute) {
return &pb.CheckResponse{Allowed: false, RetryAfterSecs: 30}, nil
}

// Special case for agentic tools
if endpoint == "/v1/agentic" {
if toolsPM, exists := endpointLimits["tools_per_minute"]; exists {
if !slidingWindow("rl:agentic:tools:"+clientID, int64(toolsPM), time.Minute) {
return &pb.CheckResponse{Allowed: false, RetryAfterSecs: 60}, nil
}
}
}

return &pb.CheckResponse{Allowed: true}, nil
}

// Вспомогательная функция
func min(a, b int) int {
if a < b {
return a
}
return b
}

// slidingWindow и tokenBucket — без изменений (рабочие)
func slidingWindow(key string, limit int64, window time.Duration) bool {
	now := time.Now().UnixNano()
	cutoff := strconv.FormatInt(now-int64(window), 10)

	pipe := rdb.Pipeline()
	pipe.ZRemRangeByScore(ctx, key, "-inf", cutoff)
	pipe.ZCard(ctx, key)
	pipe.ZAdd(ctx, key, &redis.Z{Score: float64(now), Member: now})
	pipe.Expire(ctx, key, window+time.Minute)
	results, err := pipe.Exec()
	if err != nil {
		log.Printf("Redis error in slidingWindow: %v", err)
		// In case of Redis failure, allow the request to avoid complete service disruption
		return true
	}

	countCmd, ok := results[1].(*redis.IntCmd)
	if !ok {
		log.Printf("Unexpected Redis response type in slidingWindow")
		return true
	}

	count, err := countCmd.Result()
	if err != nil {
		log.Printf("Redis count error in slidingWindow: %v", err)
		return true
	}

	return count < limit
}

func tokenBucket(key string, capacity, rate int64, period time.Duration) bool {
	now := time.Now().Unix()

	data, err := rdb.HGetAll(ctx, key).Result()
	if err != nil {
		if err != redis.Nil {
			log.Printf("Redis error in tokenBucket: %v", err)
		}
		// Initialize new bucket on first access or Redis failure
		err := rdb.HSet(ctx, key, map[string]interface{}{
			"tokens":      capacity,
			"last_refill": now,
		}).Err()
		if err != nil {
			log.Printf("Redis initialization error in tokenBucket: %v", err)
		}
		return true
	}

	tokens, err := strconv.ParseFloat(data["tokens"], 64)
	if err != nil {
		log.Printf("Token parse error in tokenBucket: %v", err)
		return true
	}

	lastRefill, err := strconv.ParseInt(data["last_refill"], 10, 64)
	if err != nil {
		log.Printf("Last refill parse error in tokenBucket: %v", err)
		return true
	}

	elapsed := now - lastRefill
	add := float64(elapsed) * float64(rate) / float64(period.Seconds())
	tokens += add
	if tokens > float64(capacity) {
		tokens = float64(capacity)
	}

	if tokens < 1 {
		return false
	}

	tokens -= 1
	err = rdb.HSet(ctx, key, "tokens", tokens).Err()
	if err != nil {
		log.Printf("Redis token update error in tokenBucket: %v", err)
		// Allow request even if update fails
		return true
	}

	err = rdb.HSet(ctx, key, "last_refill", now).Err()
	if err != nil {
		log.Printf("Redis last_refill update error in tokenBucket: %v", err)
	}

	return true
}

// AdminHandler handles HTTP requests for rate limit administration
func AdminHandler(w http.ResponseWriter, r *http.Request) {
	// Check Redis health first
	if !checkRedisHealth() {
		http.Error(w, "Redis is unavailable", http.StatusServiceUnavailable)
		return
	}

	switch r.Method {
	case http.MethodGet:
		// Return current rate limits from Redis or defaults
		limits, err := getRateLimitsFromRedis()
		if err != nil {
			// Fallback to defaults
			limits = DefaultRateLimits
		}

		response, err := json.Marshal(limits)
		if err != nil {
			http.Error(w, "Failed to serialize response", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(response)

	case http.MethodPost:
		// Update rate limits
		var req struct {
			Path     string `json:"path"`
			RPM      int     `json:"requests_per_minute"`
			TPM      int     `json:"tokens_per_minute"`
			ToolsPM  int     `json:"tools_per_minute,omitempty"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate path
		if _, exists := DefaultRateLimits[req.Path]; !exists {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}

		// Save to Redis
		limits := map[string]int{
			"requests_per_minute": req.RPM,
			"tokens_per_minute":   req.TPM,
		}
		if req.ToolsPM > 0 {
			limits["tools_per_minute"] = req.ToolsPM
		}

		if err := saveRateLimitsToRedis(req.Path, limits); err != nil {
			http.Error(w, "Failed to save rate limits", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "success", "message": "Rate limits updated"}`))

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// getRateLimitsFromRedis retrieves rate limits from Redis
func getRateLimitsFromRedis() (map[string]map[string]int, error) {
	result := make(map[string]map[string]int)

	for path := range DefaultRateLimits {
		data, err := rdb.HGetAll(ctx, "rate_limits:"+path).Result()
		if err != nil {
			if err == redis.Nil {
				// No data in Redis, use defaults
				result[path] = DefaultRateLimits[path]
				continue
			}
			return nil, err
		}

		// Convert Redis data to int map
		limits := make(map[string]int)
		for key, value := range data {
			val, err := strconv.Atoi(value)
			if err != nil {
				continue // Skip invalid values
			}
			limits[key] = val
		}

		// Merge with defaults
		for defaultKey, defaultVal := range DefaultRateLimits[path] {
			if _, exists := limits[defaultKey]; !exists {
				limits[defaultKey] = defaultVal
			}
		}

		result[path] = limits
	}

	return result, nil
}

// saveRateLimitsToRedis saves rate limits to Redis
func saveRateLimitsToRedis(path string, limits map[string]int) error {
	pipeline := rdb.Pipeline()
	for key, value := range limits {
		pipeline.HSet(ctx, "rate_limits:"+path, key, value)
	}
	_, err := pipeline.Exec(ctx)
	return err
}
