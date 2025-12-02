


package handlers

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"llm-gateway-pro/services/agentic-service/internal/secrets"
)

var rdbAgentic = redis.NewClient(&redis.Options{Addr: "redis:6379"})

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

	apiKey, _ := secrets.Get(fmt.Sprintf("llm/%s/api_key_premium", provider)) // premium keys!
	url := "https://api.openai.com/v1/chat/completions"
	if provider == "anthropic" {
		url = "https://api.anthropic.com/v1/messages"
	}

	// Prepare request body
	body := map[string]interface{}{
		"model":              req.Model,
		"messages":           req.Messages,
		"tools":              req.Tools,
		"tool_choice":        req.ToolChoice,
		"parallel_tool_calls": true,
		"response_format":    req.ResponseFormat,
		"max_tokens":         8192,
	}
	jsonBody, _ := json.Marshal(body)

	httpReq, _ := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	if provider == "anthropic" {
		httpReq.Header.Set("x-api-key", apiKey)
		httpReq.Header.Set("anthropic-version", "2023-06-01")
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		http.Error(w, `{"error":"provider error"}`, 502)
		return
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	var providerResp map[string]interface{}
	json.Unmarshal(bodyBytes, &providerResp)

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


