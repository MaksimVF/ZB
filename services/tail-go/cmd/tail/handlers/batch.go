// services/gateway/handlers/batch.go
package handlers

import (
"bytes"
"context"
"encoding/json"
"fmt"
"io"
"log"
"net/http"
"strings"
"time"

"github.com/google/uuid"
"llm-gateway-pro/services/gateway/internal/secrets"
)

type BatchItem struct {
Model       string          `json:"model"`
Messages    []Message       `json:"messages"`
CustomID    string          `json:"custom_id,omitempty"`
MaxTokens   *int            `json:"max_tokens,omitempty"`
Temperature *float32        `json:"temperature,omitempty"`
}

type BatchRequest struct {
Requests []BatchItem `json:"requests"`
Mode     string      `json:"mode,omitempty"` // "sync" или "async", по умолчанию sync
}

type BatchResponse struct {
BatchID   string `json:"batch_id"`
Status    string `json:"status"`
CreatedAt int64  `json:"created_at"`
}

// Маппинг модели → провайдер и базовый URL
var providerConfig = map[string]struct {
Provider string
BaseURL  string
}{
"gpt-4o":          {"openai", "https://api.openai.com"},
"gpt-4-turbo":     {"openai", "https://api.openai.com"},
"claude-3-opus":   {"anthropic", "https://api.anthropic.com"},
"claude-3-sonnet": {"anthropic", "https://api.anthropic.com"},
"llama3-70b":      {"groq", "https://api.groq.com/openai"},
"gemini-pro":      {"google", "https://generativelanguage.googleapis.com"},
}

func BatchSubmit(w http.ResponseWriter, r *http.Request) {
var req BatchRequest
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
return
}

if len(req.Requests) == 0 || len(req.Requests) > 500 {
http.Error(w, `{"error":"batch size must be 1-500"}`, http.StatusBadRequest)
return
}

mode := strings.ToLower(strings.TrimSpace(req.Mode))
if mode == "" || mode == "sync" {
// === SYNC режим ===
results := processBatchSync(req.Requests)
resp := map[string]interface{}{
"object":     "list",
"data":       results,
"batch_id":   "sync_" + uuid.New().String()[:8],
"status":     "completed",
"created_at": time.Now().Unix(),
}
w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(resp)
return
}

// === ASYNC режим ===
batchID := "batch_" + uuid.New().String()
created := time.Now().Unix()

// Сохраняем в Redis для batch-processor
key := "batch:pending:" + batchID
data := map[string]interface{}{
"status":     "queued",
"created_at": created,
"requests":   req.Requests,
}
if err := redisClient.HSet(r.Context(), key, data).Err(); err != nil {
log.Printf("Redis error: %v", err)
http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
return
}
redisClient.Expire(r.Context(), key, 7*24*time.Hour)
redisClient.LPush(r.Context(), "batch_queue", batchID)

resp := BatchResponse{
BatchID:   batchID,
Status:    "queued",
CreatedAt: created,
}
w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(resp)
}

// Синхронная обработка батча
func processBatchSync(items []BatchItem) []map[string]interface{} {
results := make([]map[string]interface{}, len(items))
client := &http.Client{Timeout: 300 * time.Second}

for i, item := range items {
cfg, ok := providerConfig[item.Model]
if !ok {
results[i] = map[string]interface{}{
"custom_id": item.CustomID,
"error":     "unknown model",
}
continue
}

// Получаем актуальный API-ключ из Vault
apiKey, err := secrets.Get(fmt.Sprintf("llm/%s/api_key", cfg.Provider))
if err != nil {
log.Printf("Secret error for %s: %v", cfg.Provider, err)
results[i] = map[string]interface{}{
"custom_id": item.CustomID,
"error":     "provider configuration error",
}
continue
}

// Формируем тело запроса
body := map[string]interface{}{
"model":       item.Model,
"messages":    item.Messages,
"max_tokens":  item.MaxTokens,
"temperature": item.Temperature,
}
jsonBody, _ := json.Marshal(body)

// URL зависит от провайдера
url := cfg.BaseURL + "/v1/chat/completions"
if cfg.Provider == "anthropic" {
url = cfg.BaseURL + "/v1/messages"
}

req, _ := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
req.Header.Set("Content-Type", "application/json")

// Разные заголовки авторизации
switch cfg.Provider {
case "openai", "groq":
req.Header.Set("Authorization", "Bearer "+apiKey)
case "anthropic":
req.Header.Set("x-api-key", apiKey)
req.Header.Set("anthropic-version", "2023-06-01")
case "google":
req.URL.RawQuery = "key=" + apiKey
}

resp, err := client.Do(req)
if err != nil {
results[i] = map[string]interface{}{
"custom_id": item.CustomID,
"error":     err.Error(),
}
continue
}
defer resp.Body.Close()

bodyBytes, _ := io.ReadAll(resp.Body)
var result map[string]interface{}
json.Unmarshal(bodyBytes, &result)

results[i] = map[string]interface{}{
"custom_id": item.CustomID,
"response":  result,
"status":    resp.StatusCode,
}
}
return results
}
