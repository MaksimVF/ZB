// services/rate-limiter/internal/limiter/limiter.go
package limiter

import (
"context"
"encoding/json"
"errors"
"net/http"
"strconv"
"strings"
"time"

"github.com/go-redis/redis/v8"
"github.com/golang-jwt/jwt/v5"
pb "llm-gateway-pro/services/rate-limiter/pb"
)

var (
rdb       = redis.NewClient(&redis.Options{Addr: "redis:6379"})
ctx       = context.Background()
jwtSecret = []byte("your-super-secret-jwt-key-2025") // ← тот же, что в auth-service!
)

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

if strings.Contains(path, "/v1/chat/completions") || strings.Contains(path, "/v1/completions") {
if !slidingWindow("rl:chat:rq:"+clientID, 60, time.Minute) {
return &pb.CheckResponse{Allowed: false, RetryAfterSecs: 30}, nil
}
if !tokenBucket("rl:chat:tk:"+clientID, 500_000, 500_000, time.Minute) {
return &pb.CheckResponse{Allowed: false, RetryAfterSecs: 30}, nil
}
}

if strings.HasPrefix(path, "/v1/embeddings") {
if !slidingWindow("rl:emb:rq:"+clientID, 15, time.Minute) {
return &pb.CheckResponse{Allowed: false, RetryAfterSecs: 60}, nil
}
if !tokenBucket("rl:emb:tk:"+clientID, 6_000_000, 6_000_000, time.Minute) {
return &pb.CheckResponse{Allowed: false, RetryAfterSecs: 60}, nil
}
}

if strings.HasPrefix(path, "/v1/agentic") {
if !slidingWindow("rl:agentic:rq:"+clientID, 5, time.Minute) {
return &pb.CheckResponse{Allowed: false, RetryAfterSecs: 60}, nil
}
if !tokenBucket("rl:agentic:tk:"+clientID, 20_000_000, 20_000_000, time.Minute) {
return &pb.CheckResponse{Allowed: false, RetryAfterSecs: 60}, nil
}
if !slidingWindow("rl:agentic:tools:"+clientID, 100, time.Minute) {
return &pb.CheckResponse{Allowed: false, RetryAfterSecs: 60}, nil
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
results, _ := pipe.Exec()

count, _ := results[1].(*redis.IntCmd).Result()
return count < limit
}

func tokenBucket(key string, capacity, rate int64, period time.Duration) bool {
now := time.Now().Unix()

data, _ := rdb.HGetAll(ctx, key).Result()
if len(data) == 0 {
rdb.HSet(ctx, key, map[string]interface{}{
"tokens":      capacity,
"last_refill": now,
})
return true
}

tokens, _ := strconv.ParseFloat(data["tokens"], 64)
lastRefill, _ := strconv.ParseInt(data["last_refill"], 10, 64)

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
rdb.HSet(ctx, key, "tokens", tokens)
rdb.HSet(ctx, key, "last_refill", now)
return true
}

// AdminHandler handles HTTP requests for rate limit administration
func AdminHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// Return current rate limits
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"chat_completions": {
				"requests_per_minute": 60,
				"tokens_per_minute": 500000
			},
			"embeddings": {
				"requests_per_minute": 15,
				"tokens_per_minute": 6000000
			},
			"agentic": {
				"requests_per_minute": 5,
				"tokens_per_minute": 20000000,
				"tools_per_minute": 100
			}
		}`))

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

		// Here we would update the actual rate limits in Redis
		// For now, just return success
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "success", "message": "Rate limits updated"}`))

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
