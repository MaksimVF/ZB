// services/gateway/handlers/batch.go
package handlers

import (
"encoding/json"
"log"
"net/http"
"strings"

"github.com/google/uuid"
"llm-gateway-pro/services/gateway/internal/grpc"
)

// OpenAI-совместимая структура
type BatchRequest struct {
Requests []struct {
Model     string `json:"model"`
Messages  []struct {
Role    string `json:"role"`
Content string `json:"content"`
} `json:"messages"`
MaxTokens   int    `json:"max_tokens,omitempty"`
Temperature *float32 `json:"temperature,omitempty"`
RequestID   string `json:"custom_id,omitempty"`
} `json:"requests"`
Mode string `json:"mode,omitempty"` // "sync" или "async", по умолчанию sync
}

type BatchResponse struct {
BatchID   string `json:"batch_id,omitempty"`
Status    string `json:"status"`
CreatedAt int64  `json:"created_at"`
Error     string `json:"error,omitempty"`
}

func BatchSubmit(headClient *grpc.HeadClient) http.HandlerFunc {
return func(w http.ResponseWriter, r *http.Request) {
var req BatchRequest
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
return
}

if len(req.Requests) == 0 || len(req.Requests) > 500 {
http.Error(w, `{"error":"batch size must be 1–500"}`, http.StatusBadRequest)
return
}

mode := strings.ToLower(strings.TrimSpace(req.Mode))
if mode == "" {
mode = "sync"
}

// === ASYNC режим ===
if mode == "async" {
batchID := "batch_" + uuid.New().String()
created := currentUnix()

// Сохраняем в Redis
key := "batch:batch:" + batchID
data := map[string]interface{}{
"status":     "processing",
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
Status:    "processing",
CreatedAt: created,
}
w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(resp)
return
}

// === SYNC режим ===
results := make([]map[string]interface{}, len(req.Requests))
for i, item := range req.Requests {
resp, err := headClient.Completion(r.Context(), item.Model, item.Messages)
if err != nil {
results[i] = map[string]interface{}{
"custom_id": item.RequestID,
"error":     err.Error(),
}
continue
}
results[i] = map[string]interface{}{
"custom_id": item.RequestID,
"response":  resp,
}
}

resp := BatchResponse{
BatchID:   "batch_sync_" + uuid.New().String()[:8],
Status:    "completed",
CreatedAt: currentUnix(),
}
w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(map[string]interface{}{
"object":    "list",
"data":      results,
"batch_id":  resp.BatchID,
"status":    resp.Status,
"created_at": resp.CreatedAt,
})
}
}

func currentUnix() int64 {
return time.Now().Unix()
}
