// services/batch-processor/main.go
package main

import (
"context"
"encoding/json"
"log"
"time"

"github.com/go-redis/redis/v8"
"google.golang.org/grpc"
"google.golang.org/grpc/credentials"

"llm-gateway-pro/services/batch-processor/internal/grpc" // клиент к head
)

var (
rdb        *redis.Client
headClient *grpc.HeadClient
)

func main() {
// Redis
rdb = redis.NewClient(&redis.Options{
Addr: "redis:6379",
})

// mTLS к head
creds, _ := credentials.NewClientTLSFromFile("/certs/batch-processor.pem", "")
conn, err := grpc.Dial("head:50055", grpc.WithTransportCredentials(creds))
if err != nil {
log.Fatal("cannot connect to head:", err)
}
headClient = grpc.NewHeadClient(conn)

log.Println("Batch processor started — waiting for jobs...")

for {
// BRPopLPush — атомарно и надёжно
result, err := rdb.BRPopLPush(context.Background(), "batch_queue", "batch_processing", 30*time.Second).Result()
if err == redis.Nil {
continue // таймаут
}
if err != nil {
log.Printf("Redis error: %v", err)
time.Sleep(5 * time.Second)
continue
}

go processBatch(result) // не блокируем основной цикл
}
}

func processBatch(batchID string) {
defer func() {
if r := recover(); r != nil {
log.Printf("Panic in batch %s: %v", batchID, r)
rdb.HSet(context.Background(), ":batch:"+batchID, "status", "failed")
}
// Убираем из processing-листа
rdb.LRem(context.Background(), "batch_processing", -1, batchID)
}()

ctx := context.Background()
key := ":batch:" + batchID

data, err := rdb.HGetAll(ctx, key).Result()
if err != nil || data["status"] != "processing" {
return
}

var requests []struct {
Model     string `json:"model"`
Messages  []struct {
Role    string `json:"role"`
Content string `json:"content"`
} `json:"messages"`
RequestID string `json:"custom_id,omitempty"`
}
json.Unmarshal([]byte(data["requests"]), &requests)

resultsKey := ":batch:results:" + batchID
pipe := rdb.Pipeline()

for _, req := range requests {
resp, err := headClient.Completion(context.Background(), req.Model, req.Messages)
result := map[string]interface{}{
"custom_id": req.RequestID,
"response":  resp,
}
if err != nil {
result["error"] = err.Error()
}
raw, _ := json.Marshal(result)
pipe.RPush(ctx, resultsKey, raw)
}

pipe.HSet(ctx, key, "status", "completed")
pipe.HSet(ctx, key, "completed_at", time.Now().Unix())
pipe.Expire(ctx, resultsKey, 30*24*time.Hour)
pipe.Exec()

log.Printf("Batch %s completed (%d items)", batchID, len(requests))
}
