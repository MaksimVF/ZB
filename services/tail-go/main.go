package main

import (
"bytes"
"context"
"encoding/json"
"io"
"log"
"net"
"net/http"
"os"
"strconv"
"strings"
"sync"
"time"

"github.com/go-redis/redis/v8"
"google.golang.org/grpc"
"google.golang.org/grpc/credentials"
"google.golang.org/grpc/credentials/tls/certprovider"

gen "github.com/yourorg/head/gen"
)

var (
headAddr  string
rdb       *redis.Client
wg        sync.WaitGroup

// Глобальные кэши правил (обновляются из Redis каждые 5 сек)
currentRequestRules   = make(map[string]limitRule)
currentRequestRulesMu sync.RWMutex

currentTokenRules   = make(map[string]tokenLimitRule)
currentTokenRulesMu sync.RWMutex
)

// Правила лимитов
type limitRule struct {
Max   int `json:"max"`
Window int `json:"window"`
Burst  int `json:"burst"`
}

type limitRule struct {
Requests int
Window   time.Duration
Burst    int
}

type tokenLimitRule struct {
MaxTokensPerMinute int64
BurstTokens        int64
}

// Входящие/исходящие структуры
type ChatMessage struct {
Role    string `json:"role"`
Content string `json:"content"`
}

type ChatRequestIn struct {
Model       string        `json:"model"`
Messages    []ChatMessage `json:"messages"`
Temperature float32       `json:"temperature,omitempty"`
MaxTokens   int           `json:"max_tokens,omitempty"`
Stream      bool          `json:"stream,omitempty"`
RequestID   string        `json:"request_id,omitempty"`
}

func main() {
headAddr = os.Getenv("HEAD_ADDR")
if headAddr == "" {
headAddr = "head:50055"
}

rdb = redis.NewClient(&redis.Options{
Addr: os.Getenv("REDIS_ADDR"),
})
if rdb == nil || rdb.Options().Addr == "" {
rdb = redis.NewClient(&redis.Options{Addr: "redis:6379"})
}

// Запускаем загрузку правил из Redis
go ruleWatcher()

// Даем время на первую загрузку
time.Sleep(2 * time.Second)

http.HandleFunc("/v1/chat/completions", smartRateLimit(handleChat))
log.Println("tail-go tail HTTP server listening on :8000")
log.Fatal(http.ListenAndServe(":8000", nil))
}

// ruleWatcher — каждые 5 сек подтягивает актуальные лимиты из Redis
func ruleWatcher() {
for {
loadRulesFromRedis()
time.Sleep(5 * time.Second)
}
}

func loadRulesFromRedis() {
ctx := context.Background()
keys, err := rdb.Keys(ctx, "ratelimit:rule:*").Result()
if err != nil {
return
}

reqMap := make(map[string]limitRule)
tokMap := make(map[string]tokenLimitRule)

for _, key := range keys {
val, _ := rdb.Get(ctx, key).Result()
var r limit
if json.Unmarshal([]byte(val), &r) != nil {
continue
}

pathType := strings.TrimPrefix(key, "ratelimit:rule:")
if strings.HasSuffix(pathType, ":requests") {
path := strings.TrimSuffix(pathType, ":requests")
reqMap[path] = limitRule{
Requests: r.Max,
Window:   time.Duration(r.Window) * time.Second,
Burst:    r.Burst,
}
} else if strings.HasSuffix(pathType, ":tokens") {
path := strings.TrimSuffix(pathType, ":tokens")
tokMap[path] = tokenLimitRule{
MaxTokensPerMinute: int64(r.Max),
BurstTokens:        int64(r.Burst),
}
}
}

// Атомарно обновляем
currentRequestRulesMu.Lock()
currentRequestRules = reqMap
currentRequestRulesMu.Unlock()

currentTokenRulesMu.Lock()
currentTokenRules = tokMap
currentTokenRulesMu.Unlock()
}

// getClientIdentifier — API-ключ имеет приоритет над IP
func getClientIdentifier(r *http.Request) string {
if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
return "key:" + strings.TrimSpace(auth[7:])
}
if key := r.Header.Get("X-API-Key"); key != "" {
return "key:" + key
}

ip := r.Header.Get("X-Forwarded-For")
if ip == "" {
ip = r.Header.Get("X-Real-IP")
}
if ip == "" {
host, _, _ := net.SplitHostPort(r.RemoteAddr)
ip = host
}
if idx := strings.Index(ip, ","); idx > 0 {
ip = strings.TrimSpace(ip[:idx])
}
}
return "ip:" + ip
}

// smartRateLimit — проверяет и запросы, и токены (токены — после выполнения)
func smartRateLimit(next http.HandlerFunc) http.HandlerFunc {
return func(w http.ResponseWriter, r *http.Request) {
path := r.URL.Path
clientID := getClientIdentifier(r)

// 1. Проверка лимита по количеству запросов
if !checkRequestLimit(path, clientID) {
w.Header().Set("Retry-After", "60")
http.Error(w, `{"error":"rate limit exceeded (requests per minute)"}`, http.StatusTooManyRequests)
return
}

// Перехватываем ответ, чтобы узнать tokens_used
rec := &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK}
next(rec, r)

// Если уже ошибка — не считаем токены
if rec.statusCode >= 400 {
return
}

tokens := extractTokens(rec.body.Bytes())
if tokens > 0 && !checkTokenLimit(path, clientID, tokens) {
log.Printf("Token limit exceeded: %s used %d tokens", clientID, tokens)
// Можно вернуть ошибку, но запрос уже выполнен — просто логируем
}
}
}

// checkRequestLimit — лимит по запросам
func checkRequestLimit(path, clientID string) bool {
currentRequestRulesMu.RLock()
rule, ok := currentRequestRules[path]
if !ok {
rule = currentRequestRules["/v1/chat/completions"] // fallback
if rule.Requests == 0 {
rule = limitRule{Requests: 100, Window: time.Minute, Burst: 120}
}
}
currentRequestRulesMu.RUnlock()

key := "rl_req:" + clientID + path
ctx := context.Background()

pipe := rdb.Pipeline()
pipe.ZRemRangeByScore(ctx, key, "-inf", strconv.FormatInt(time.Now().Add(-rule.Window).UnixNano(), 10))
countCmd := pipe.ZCard(ctx, key)
pipe.ZAdd(ctx, key, &redis.Z{Score: float64(time.Now().UnixNano()), Member: time.Now().UnixNano()})
pipe.Expire(ctx, key, rule.Window+time.Minute)
pipe.Exec()

count := countCmd.Val()
return count <= int64(rule.Burst) && count <= int64(rule.Requests)
}

// checkTokenLimit — лимит по токенам
func checkTokenLimit(path, clientID string, tokens int64) bool {
currentTokenRulesMu.RLock()
rule, ok := currentTokenRules[path]
if !ok {
rule = currentTokenRules["/v1/chat/completions"]
if rule.MaxTokensPerMinute == 0 {
rule = tokenLimitRule{MaxTokensPerMinute: 400_000, BurstTokens: 500_000}
}
}
currentTokenRulesMu.RUnlock()

key := "rl_tok:" + clientID + path
ctx := context.Background()

pipe := rdb.Pipeline()
pipe.ZRemRangeByScore(ctx, key, "-inf", strconv.FormatInt(time.Now().Add(-time.Minute).UnixNano(), 10))
sumCmd := pipe.ZRangeByScoreWithScores(ctx, key, &redis.ZRangeBy{Min: "-inf", Max: "+inf"})
pipe.ZAdd(ctx, key, &redis.Z{Score: float64(tokens), Member:float64(time.Now().UnixNano())})
pipe.Expire(ctx, key, 2*time.Minute)
pipe.Exec()

var sum int64
for _, z := range sumCmd.Val() {
sum += int64(z.Score)
}
sum += tokens

return sum <= rule.BurstTokens && sum <= rule.MaxTokensPerMinute
}

// responseRecorder — перехватывает тело ответа
type responseRecorder struct {
http.ResponseWriter
body       bytes.Buffer
statusCode int
}

func (rr *responseRecorder) Write(b []byte) (int, error) {
rr.body.Write(b)
return rr.ResponseWriter.Write(b)
}

func (rr *responseRecorder) WriteHeader(code int) {
rr.statusCode = code
rr.ResponseWriter.WriteHeader(code)
}

func extractTokens(body []byte) int64 {
var data map[string]interface{}
if json.Unmarshal(body, &data) != nil {
return 0
}
if usage, ok := data["usage"].(map[string]interface{}); ok {
if t, ok := usage["total_tokens"].(float64); ok {
return int64(t)
}
}
return 0
}

// handleChat — основной обработчик (стриминг + обычный режим)
func handleChat(w http.ResponseWriter, r *http.Request) {
if r.Method != http.MethodPost {
http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
return
}

var req ChatRequestIn
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
http.Error(w, "invalid json", http.StatusBadRequest)
return
}
if len(req.Messages) == 0 {
http.Error(w, "messages required", http.StatusBadRequest)
return
}

creds, err := loadMTLSCredentials()
if err != nil {
log.Printf("mTLS error: %v", err)
http.Error(w, "internal error", http.StatusInternalServerError)
return
}

conn, err := grpc.Dial(headAddr,
grpc.WithTransportCredentials(creds),
grpc.WithBlock(),
grpc.WithTimeout(10*time.Second),
)
if err != nil {
log.Printf("head connect error: %v", err)
http.Error(w, "upstream unavailable", http.StatusBadGateway)
return
}
defer conn.Close()

client := gen.NewChatServiceClient(conn)
ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
defer cancel()

grpcReq := &gen.ChatRequest{
RequestId:   req.RequestID,
Model:       req.Model,
Temperature: req.Temperature,
MaxTokens:   int32(req.MaxTokens),
}
for _, m := range req.Messages {
grpcReq.Messages = append(grpcReq.Messages, &gen.ChatMessage{Role: m.Role, Content: m.Content})
}

// === СТРИМИНГ ===
if req.Stream {
stream, err := client.ChatCompletionStream(ctx, grpcReq)
if err != nil {
http.Error(w, "stream error", http.StatusInternalServerError)
return
}

w.Header().Set("Content-Type", "text/event-stream")
w.Header().Set("Cache-Control", "no-cache")
w.Header().Set("Connection", "keep-alive")
flusher, _ := w.(http.Flusher)

for {
chunk, err := stream.Recv()
if err == io.EOF {
io.WriteString(w, "data: [DONE]\n\n")
flusher.Flush()
return
}
if err != nil {
return
}

delta := struct{ Content string `json:"content"` }{chunk.Chunk}
resp := map[string]interface{}{
"id":      chunk.RequestId,
"object":  "chat.completion.chunk",
"created": time.Now().Unix(),
"model":   chunk.Model,
"choices": []map[string]interface{}{
{"index": 0, "delta": delta, "finish_reason": nil},
},
}
if chunk.IsFinal {
resp["choices"].([]map[string]interface{})[0]["finish_reason"] = "stop"
}

data, _ := json.Marshal(resp)
io.WriteString(w, "data: "+string(data)+"\n\n")
flusher.Flush()
}
}

// === ОБЫЧНЫЙ РЕЖИМ ===
resp, err := client.ChatCompletion(ctx, grpcReq)
if err != nil {
http.Error(w, "completion error", http.StatusBadGateway)
return
}

response := map[string]interface{}{
"id":      resp.RequestId,
"object":  "chat.completion",
"created": time.Now().Unix(),
"model":   resp.Model,
"choices": []map[string]interface{}{
{
"index": 0,
"message": map[string]string{
"role":    "assistant",
"content": resp.FullText,
},
"finish_reason": "stop",
},
},
"usage": map[string]int{
"prompt_tokens":      0,
"completion_tokens":  0,
"total_tokens":       int(resp.TokensUsed),
},
}

w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(response)
}

// loadMTLSCredentials — mTLS к head
func loadMTLSCredentials() (credentials.TransportCredentials, error) {
cert, err := tls.LoadX509KeyPair("/certs/head.pem", "/certs/head-key.pem")
if err != nil {
return nil, err
}
caCert, err := os.ReadFile("/certs/ca.pem")
if err != nil {
return nil, err
}
caPool := x509.NewCertPool()
caPool.AppendCertsFromPEM(caCert(caCert)

return credentials.NewTLS(&tls.Config{
ServerName:   "head",
Certificates: []tls.Certificate{cert},
RootCAs:      caPool,
}), nil
}
