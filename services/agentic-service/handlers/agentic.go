


package handlers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"llm-gateway-pro/services/agentic-service/internal/secrets"
	"llm-gateway-pro/services/head-go/gen"
)

var (
	rdbAgentic = redis.NewClient(&redis.Options{Addr: "redis:6379"})
	headClient  gen.ChatServiceClient
)

func init() {
	// Initialize gRPC client to head service
	conn, err := grpc.Dial(
		"head-service:50051",
		grpc.WithTransportCredentials(loadHeadTLSCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(10*time.Second),
	)
	if err != nil {
		log.Fatalf("Failed to connect to head service: %v", err)
	}

	headClient = gen.NewChatServiceClient(conn)
}

func loadHeadTLSCredentials() grpc.TransportCredentials {
	// Load client certificate and key
	cert, err := tls.LoadX509KeyPair("/certs/agentic-service.pem", "/certs/agentic-service-key.pem")
	if err != nil {
		log.Fatalf("Failed to load client certificate: %v", err)
	}

	// Load CA certificate
	caCert, err := os.ReadFile("/certs/ca.pem")
	if err != nil {
		log.Fatalf("Failed to load CA certificate: %v", err)
	}
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		log.Fatalf("Failed to add CA certificate to pool")
	}

	// Create TLS config
	config := &tls.Config{
		ServerName:   "head-service",
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
	}

	return credentials.NewTLS(config)
}

type AgenticRequest struct {
	Model    string                   `json:"model"`
	Messages []map[string]interface{} `json:"messages"`
	Tools    []map[string]interface{} `json:"tools,omitempty"`
	ToolChoice interface{}            `json:"tool_choice,omitempty"`
	ParallelToolCalls bool          `json:"parallel_tool_calls,omitempty"`
	ResponseFormat    *struct {
		Type   string `json:"type"`
		Schema json.RawMessage `json:"schema,omitempty"`
	} `json:"response_format,omitempty"`
}

type ToolCall struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Function map[string]interface{} `json:"function"`
}

type AgenticResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int `json:"index"`
		Message      map[string]interface{} `json:"message"`
		FinishReason string                 `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
		ToolCalls        int `json:"tool_calls,omitempty"`
	} `json:"usage"`
	XParallelCalls int `json:"x_parallel_calls,omitempty"`
	XCachedTools   int `json:"x_cached_tools,omitempty"`
}

func AgenticHandler(w http.ResponseWriter, r *http.Request) {
	var req AgenticRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, 400)
		return
	}

	// Force agentic settings
	req.ParallelToolCalls = true
	if req.ResponseFormat == nil {
		req.ResponseFormat = &struct {
			Type   string          `json:"type"`
			Schema json.RawMessage `json:"schema,omitempty"`
		}{Type: "json_object"}
	}

	// Only top reasoning models allowed
	allowed := map[string]string{
		"gpt-4o-2024-11-20": "openai",
		"claude-3-5-sonnet-20241022": "anthropic",
		"o1-preview": "openai",
		"o1-mini":    "openai",
	}
	provider, ok := allowed[req.Model]
	if !ok {
		http.Error(w, `{"error":"model not allowed for agentic endpoint"}`, 400)
		return
	}

	// Convert messages to the format expected by head service
	var messages []string
	for _, msg := range req.Messages {
		if content, ok := msg["content"].(string); ok {
			messages = append(messages, content)
		}
	}

	// Call head service via gRPC
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	chatReq := &gen.ChatRequest{
		RequestId:   fmt.Sprintf("agentic-%d", time.Now().UnixNano()),
		Model:       req.Model,
		Messages:    messages,
		Temperature:  0.7, // Default temperature for agentic
		MaxTokens:   8192,
		Stream:      false,
	}

	chatResp, err := headClient.ChatCompletion(ctx, chatReq)
	if err != nil {
		http.Error(w, `{"error":"head service error"}`, 502)
		return
	}

	// Parse the response from head service
	var providerResp map[string]interface{}
	if err := json.Unmarshal([]byte(chatResp.FullText), &providerResp); err != nil {
		http.Error(w, `{"error":"invalid response format"}`, 500)
		return
	}

	// Parallel tool_calls + caching
	toolCalls := extractToolCalls(providerResp)
	cached := 0
	if len(toolCalls) > 0 {
		var wg sync.WaitGroup
		for i := range toolCalls {
			wg.Add(1)
			go func(tc *ToolCall) {
				defer wg.Done()
				cacheKey := "tool:" + hashToolCall(tc)
				if cached, _ := rdbAgentic.Get(r.Context(), cacheKey).Result(); cached != "" {
					tc.Function["result"] = json.RawMessage(cached)
					cached++
					return
				}
				// Here you can add real tool calls
				// result := callRealTool(tc)
				// rdbAgentic.SetEX(r.Context(), cacheKey, result, 7*24*time.Hour)
			}(&toolCalls[i])
		}
		wg.Wait()
	}

	final := AgenticResponse{
		ID:      "agentic-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		Object:  "agentic.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []struct {
			Index        int                    `json:"index"`
			Message      map[string]interface{} `json:"message"`
			FinishReason string                 `json:"finish_reason"`
		}{
			{
				Index:        0,
				Message:      map[string]interface{}{"role": "assistant", "tool_calls": toolCalls},
				FinishReason: "tool_calls",
			},
		},
		Usage: struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
			ToolCalls        int `json:"tool_calls,omitempty"`
		}{
			ToolCalls: len(toolCalls),
		},
		XParallelCalls: len(toolCalls),
		XCachedTools:   cached,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Agentic-Endpoint", "true")
	json.NewEncoder(w).Encode(final)
}

func extractToolCalls(resp map[string]interface{}) []ToolCall {
	var calls []ToolCall
	choices := resp["choices"].([]interface{})
	for _, c := range choices {
		choice := c.(map[string]interface{})
		msg := choice["message"].(map[string]interface{})
		if tc, ok := msg["tool_calls"]; ok {
			for _, t := range tc.([]interface{}) {
				tool := t.(map[string]interface{})
				calls = append(calls, ToolCall{
					ID:       tool["id"].(string),
					Type:     tool["type"].(string),
					Function: tool["function"].(map[string]interface{}),
				})
			}
		}
	}
	return calls
}

func hashToolCall(tc *ToolCall) string {
	h := sha256.Sum256([]byte(tc.Function["name"].(string) + tc.Function["arguments"].(string)))
	return hex.EncodeToString(h[:])
}


