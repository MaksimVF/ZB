// services/batch-processor/main.go
package main

import (
"bytes"
"context"
"encoding/json"
"fmt"
"io"
"log"
"net/http"
"time"

"github.com/go-redis/redis/v8"
"llm-gateway-pro/services/gateway/internal/secrets" // ← единый helper!
)

var (
rdb        = redis.NewClient(&redis.Options{Addr: "redis:6379"})
client     = &http.Client{Timeout: 300 * time.Second}
ctx        = context.Background()
providerConfig = map[string]struct {
Provider string
BaseURL  string
}{
"gpt-4o":          {"openai", "https://api.openai.com"},
"gpt-4-turbo":     {"openai", "https://api.openai.com"},
"claude-3-opus":   {"anthropic", "https://api.anthropic.com"},
"llama3-70b":      {"groq", "https://api.groq.com/openai"},
"gemini-pro":      {"google", "https://generativelanguage.googleapis.com"},
}
)

type BatchItem struct {
Model       string          `json:"model"`
Messages    []Message       `json:"messages"`
CustomID    string          `json:"custom_id,omitempty"`
MaxTokens   *int            `json:"max_tokens,omitempty"`
Temperature *float32        `json:"temperature,omitempty"`
}

type Message struct {
Role    string `json:"role"`
Content string `json:"content"`
}

func main() {
log.Println("Batch processor started — waiting for jobs...")
for {
batchID, err := rdb.BRPopLPush(ctx, "batch_queue", "batch_processing", 0).Result()
if err != nil {
log.Printf("Redis error: %v", err)
time.Sleep(5 * time.Second)
continue
}
go processBatch(batchID)
}
}

func processBatch(batchID string) {
defer func() {
rdb.LRem(ctx, "batch_processing", 1, batchID)
}()

go func() {
    for {
        id, _ := rdb.BRPopLPush(ctx, "embeddings_queue", "embeddings_processing", 0).Result()
        go processEmbeddingBatch(id)
    }
}()

key := "batch:pending:" + batchID
data, err := rdb.HGetAll(ctx, key).Result()
if err != nil || len(data) == 0 {
log.Printf("Batch %s not found", batchID)
return
}

var items []BatchItem
if err := json.Unmarshal([]byte(data["requests"]), &items); err != nil {
log.Printf("Invalid batch data: %v", err)
return
}

resultsKey := "batch:results:" + batchID
pipe := rdb.Pipeline()

for _, item := range items {
result := processSingleItem(item)
raw, _ := json.Marshal(result)
pipe.RPush(ctx, resultsKey, raw)
}

pipe.HSet(ctx, key, "status", "completed")
pipe.HSet(ctx, key, "completed_at", time.Now().Unix())
pipe.Expire(ctx, resultsKey, 30*24*time.Hour)
pipe.Exec()

log.Printf("Batch %s completed (%d items)", batchID, len(items))
}

func processSingleItem(item BatchItem) map[string]interface{} {
cfg, ok := providerConfig[item.Model]
if !ok {
return map[string]interface{}{
"custom_id": item.CustomID,
"error":     "unsupported model",
}
}

// ← ВОБЯЗАТЕЛЬНО: получаем свежий ключ из Vault!
apiKey, err := secrets.Get(fmt.Sprintf("llm/%s/api_key", cfg.Provider))
if err != nil {
log.Printf("Secret error for %s: %v", cfg.Provider, err)
return map[string]interface{}{
"custom_id": item.CustomID,
"error":     "internal configuration error",
}
}

body := map[string]interface{}{
"model":       item.Model,
"messages":    item.Messages,
"max_tokens":  item.MaxTokens,
"temperature": item.Temperature,
}
jsonBody, _ := json.Marshal(body)

url := cfg.BaseURL + "/v1/chat/completions"
if cfg.Provider == "anthropic" {
url = cfg.BaseURL + "/v1/messages"
}

req, _ := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
req.Header.Set("Content-Type", "application/json")

switch cfg.Provider {
case "openai", "groq":
req.Header.Set("Authorization", "Bearer "+apiKey)
case "anthropic":
req.Header.Set("x-api-key", apiKey)
req.Header.SetBasicAuth("", apiKey) // на всякий случай
req.Header.Set("anthropic-version", "2023-06-01")
case "google":
req.URL.RawQuery = "key=" + apiKey
}

resp, err := client.Do(req)
if err != nil {
return map[string]interface{}{
"custom_id": item.CustomID,
"error":     err.Error(),
}
}
defer resp.Body.Close()

respBody, _ := io.ReadAll(resp.Body)
var result map[string]interface{}
json.Unmarshal(respBody, &result)

return map[string]interface{}{
"custom_id": item.CustomID,
"response":  result,
"status":    resp.StatusCode,
}
}
