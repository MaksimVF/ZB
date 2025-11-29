
// services/batch-processor/main.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pb "llm-gateway-pro/services/tail-go/gen" // Generated from model.proto
)

var (
	rdb            = redis.NewClient(&redis.Options{Addr: "redis:6379"})
	ctx            = context.Background()
	// gRPC connection to model-proxy service
	modelProxyConn *grpc.ClientConn
	modelProxyClient pb.ModelServiceClient
)

func init() {
	// Connect to model-proxy gRPC service
	conn, err := grpc.Dial("model-proxy:50061", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to model-proxy: %v", err)
	}
	modelProxyConn = conn
	modelProxyClient = pb.NewModelServiceClient(conn)
}

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
	// Convert messages to string array for gRPC
	var messages []string
	for _, msg := range item.Messages {
		messages = append(messages, fmt.Sprintf("%s: %s", msg.Role, msg.Content))
	}

	// Prepare gRPC request
	req := &pb.GenRequest{
		RequestId:   item.CustomID,
		Model:       item.Model,
		Messages:    messages,
		Temperature:  float32(*item.Temperature),
		MaxTokens:   int32(*item.MaxTokens),
		Stream:      false,
	}

	// Call model-proxy gRPC service
	resp, err := modelProxyClient.Generate(ctx, req)
	if err != nil {
		return map[string]interface{}{
			"custom_id": item.CustomID,
			"error":     err.Error(),
		}
	}

	return map[string]interface{}{
		"custom_id":    item.CustomID,
		"response":     resp.Text,
		"tokens_used":  resp.TokensUsed,
		"status":       "completed",
	}
}

