




package handlers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"llm-gateway-pro/services/gateway/internal/billing"
	"llm-gateway-pro/services/gateway/internal/providers"
	"llm-gateway-pro/services/gateway/internal/resilience"
)

var (
	langchainCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "langchain_requests_total",
			Help: "Total number of LangChain requests",
		},
		[]string{"model", "status"},
	)

	langchainDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "langchain_request_duration_seconds",
			Help:    "LangChain request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"model"},
	)
)

type LangChainRequest struct {
	Model          string                 `json:"model"`
	Messages       []map[string]interface{} `json:"messages"`
	Stream         bool                     `json:"stream,omitempty"`
	MaxTokens      *int                     `json:"max_tokens,omitempty"`
	Temperature    *float64                 `json:"temperature,omitempty"`
	ResponseFormat *struct {
		Type string `json:"type"`
	} `json:"response_format,omitempty"`
	Tools []map[string]interface{} `json:"tools,omitempty"`
}

type LangChainResponse struct {
	ID        string    `json:"id"`
	Object    string    `json:"object"`
	Created   int64     `json:"created"`
	Model     string    `json:"model"`
	Choices   []Choice  `json:"choices"`
	Usage     Usage     `json:"usage"`
}

type Choice struct {
	Index        int                    `json:"index"`
	Message      map[string]interface{} `json:"message,omitempty"`
	Delta        map[string]interface{} `json:"delta,omitempty"`
	FinishReason string                 `json:"finish_reason,omitempty"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func init() {
	prometheus.MustRegister(langchainCounter, langchainDuration)
}

func LangChainCompletion(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	logger := zerolog.New(os.Stdout).With().
		Timestamp().
		Str("service", "gateway").
		Str("handler", "langchain").
		Logger()

	logger.Info().Msg("Received LangChain request")

	// Validate API key and track usage
	apiKey := r.Header.Get("Authorization")
	if !strings.HasPrefix(apiKey, "Bearer ") {
		logger.Warn().Msg("Missing or invalid API key format")
		http.Error(w, `{"error":"invalid api key format"}`, 401)
		langchainCounter.WithLabelValues("unknown", "unauthorized").Inc()
		return
	}

	apiKey = strings.TrimPrefix(apiKey, "Bearer ")
	userID, err := validateAndTrackLangChainUsage(apiKey)
	if err != nil {
		logger.Warn().Err(err).Msg("Invalid API key")
		http.Error(w, `{"error":"invalid api key"}`, 401)
		langchainCounter.WithLabelValues("unknown", "unauthorized").Inc()
		return
	}

	var req LangChainRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error().Err(err).Msg("Invalid JSON input")
		http.Error(w, `{"error":"invalid json"}`, 400)
		langchainCounter.WithLabelValues(req.Model, "error").Inc()
		return
	}

	// Validate required fields
	if req.Model == "" || len(req.Messages) == 0 {
		logger.Warn().Msg("Missing required fields")
		http.Error(w, `{"error":"model and messages are required"}`, 400)
		langchainCounter.WithLabelValues(req.Model, "error").Inc()
		return
	}

	// Get provider for model using LiteLLM
	providerConfig, err := providers.GetProviderForModel(req.Model)
	if err != nil {
		logger.Warn().Str("model", req.Model).Msg("Unsupported model")
		http.Error(w, `{"error":"unsupported model"}`, 400)
		langchainCounter.WithLabelValues(req.Model, "error").Inc()
		return
	}

	// Execute with circuit breaker and retry logic
	providerName := getProviderName(providerConfig.BaseURL)
	result, err := resilience.ExecuteWithCircuitBreaker(providerName, func() (interface{}, error) {
		return executeWithRetry(providerConfig, req, 3, 1*time.Second)
	})

	if err != nil {
		logger.Error().Err(err).Str("provider", providerConfig.BaseURL).Msg("Provider request failed")
		http.Error(w, `{"error":"provider unavailable"}`, 502)
		langchainCounter.WithLabelValues(req.Model, "error").Inc()
		return
	}

	respBody, ok := result.([]byte)
	if !ok {
		logger.Error().Msg("Invalid response type from provider")
		http.Error(w, `{"error":"internal error"}`, 500)
		langchainCounter.WithLabelValues(req.Model, "error").Inc()
		return
	}

	// Handle streaming response
	if req.Stream {
		handleStreamingResponse(w, respBody, logger)
		langchainCounter.WithLabelValues(req.Model, "success").Inc()
		langchainDuration.WithLabelValues(req.Model).Observe(time.Since(start).Seconds())
		return
	}

	// Process and normalize response
	var providerResp map[string]interface{}
	if err := json.Unmarshal(respBody, &providerResp); err != nil {
		logger.Error().Err(err).Msg("Failed to parse provider response")
		http.Error(w, `{"error":"internal error"}`, 500)
		langchainCounter.WithLabelValues(req.Model, "error").Inc()
		return
	}

	finalResp, err := normalizeProviderResponse(providerResp, req.Model)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to normalize provider response")
		http.Error(w, `{"error":"internal error"}`, 500)
		langchainCounter.WithLabelValues(req.Model, "error").Inc()
		return
	}

	// Track usage for billing
	go trackLangChainUsage(userID, req.Model, finalResp.Usage.TotalTokens)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(finalResp); err != nil {
		logger.Error().Err(err).Msg("Failed to encode response")
		http.Error(w, `{"error":"internal error"}`, 500)
		langchainCounter.WithLabelValues(req.Model, "error").Inc()
		return
	}

	logger.Info().Str("model", req.Model).Msg("LangChain request completed successfully")
	langchainCounter.WithLabelValues(req.Model, "success").Inc()
	langchainDuration.WithLabelValues(req.Model).Observe(time.Since(start).Seconds())
}

func executeWithRetry(providerConfig providers.ProviderConfig, req LangChainRequest, maxRetries int, backoff time.Duration) ([]byte, error) {
	var err error
	var respBody []byte

	operation := func() error {
		respBody, err = providers.ProxyRequest(providerConfig, "POST", "/v1/chat/completions", req)
		return err
	}

	err = resilience.WithRetry(operation, maxRetries, backoff)
	return respBody, err
}

func getProviderName(baseURL string) string {
	switch {
	case strings.Contains(baseURL, "openai"):
		return "openai"
	case strings.Contains(baseURL, "anthropic"):
		return "anthropic"
	case strings.Contains(baseURL, "google"):
		return "google"
	case strings.Contains(baseURL, "meta"):
		return "meta"
	default:
		return "unknown"
	}
}

func handleStreamingResponse(w http.ResponseWriter, body []byte, logger zerolog.Logger) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		logger.Error().Msg("Streaming not supported")
		http.Error(w, `{"error":"streaming not supported"}`, 500)
		return
	}

	// For streaming, we need to process the response differently
	// This is a simplified version - in production, you'd want to
	// stream the response from the provider directly
	io.WriteString(w, "data: "+string(body)+"\n\n")
	io.WriteString(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func normalizeProviderResponse(providerResp map[string]interface{}, model string) (LangChainResponse, error) {
	// Extract usage information
	usageData, ok := providerResp["usage"].(map[string]interface{})
	if !ok {
		return LangChainResponse{}, errors.New("invalid usage format")
	}

	usage := Usage{
		PromptTokens:     int(usageData["prompt_tokens"].(float64)),
		CompletionTokens: int(usageData["completion_tokens"].(float64)),
		TotalTokens:      int(usageData["total_tokens"].(float64)),
	}

	// Process choices
	choicesData, ok := providerResp["choices"].([]interface{})
	if !ok {
		return LangChainResponse{}, errors.New("invalid choices format")
	}

	var choices []Choice
	for i, choiceData := range choicesData {
		choiceMap, ok := choiceData.(map[string]interface{})
		if !ok {
			continue
		}

		// Ensure finish_reason is set
		finishReason, ok := choiceMap["finish_reason"].(string)
		if !ok || finishReason == "" {
			choiceMap["finish_reason"] = "stop"
		}

		choices = append(choices, Choice{
			Index:        i,
			Message:      choiceMap["message"].(map[string]interface{}),
			FinishReason: choiceMap["finish_reason"].(string),
		})
	}

	return LangChainResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: choices,
		Usage:   usage,
	}, nil
}

func validateAndTrackLangChainUsage(apiKey string) (string, error) {
	// Validate API key and check if it's a LangChain-specific key
	// In a real implementation, this would check a database or cache
	if strings.HasPrefix(apiKey, "langchain-") {
		// Extract user ID from API key (simplified for example)
		return "user-" + apiKey[10:15], nil
	}
	return "", errors.New("invalid LangChain API key")
}

func trackLangChainUsage(userID, model string, tokens int) {
	// Track usage in billing system
	err := billing.TrackUsage(userID, model, tokens)
	if err != nil {
		logger.Error().Err(err).Str("user", userID).Str("model", model).Int("tokens", tokens).Msg("Failed to track usage")
	}
}

func ListProviders(w http.ResponseWriter, r *http.Request) {
	providers := providers.GetAllProviders()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(providers)
}

func AddProvider(w http.ResponseWriter, r *http.Request) {
	var config providers.ProviderConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, `{"error":"invalid input"}`, 400)
		return
	}

	providers.AddProvider(config.BaseURL, config)
	w.WriteHeader(http.StatusCreated)
}

func RemoveProvider(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	provider := vars["provider"]

	providers.RemoveProvider(provider)
	w.WriteHeader(http.StatusOK)
}

func ListCircuitBreakers(w http.ResponseWriter, r *http.Request) {
	status := resilience.GetAllCircuitBreakers()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func GetCircuitBreakerStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	state, counts, err := resilience.GetCircuitBreakerStatus(name)
	if err != nil {
		http.Error(w, `{"error":"circuit breaker not found"}`, 404)
		return
	}

	status := map[string]interface{}{
		"name":   name,
		"state":  state.String(),
		"counts": counts,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func ResetCircuitBreaker(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	err := resilience.ResetCircuitBreaker(name)
	if err != nil {
		http.Error(w, `{"error":"circuit breaker not found"}`, 404)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"reset"}`))
}


