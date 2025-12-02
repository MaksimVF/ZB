





package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"llm-gateway-pro/services/gateway/internal/handlers"
)

func TestMain(m *testing.M) {
	// Set up environment variables for testing
	os.Setenv("OPENAI_API_KEY", "test-openai-key")
	os.Setenv("ANTHROPIC_API_KEY", "test-anthropic-key")
	os.Setenv("GOOGLE_API_KEY", "test-google-key")
	os.Setenv("META_API_KEY", "test-meta-key")

	// Initialize the service
	handlers.InitMetrics()

	// Run tests
	code := m.Run()
	os.Exit(code)
}

func TestLangChainCompletion(t *testing.T) {
	tests := []struct {
		name           string
		apiKey         string
		model          string
		messages        []map[string]interface{}
		expectedStatus  int
		expectedError   string
	}{
		{
			name:           "valid request",
			apiKey:         "langchain-12345",
			model:          "gpt-4",
			messages:       []map[string]interface{}{{"role": "user", "content": "Hello"}},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid api key",
			apiKey:         "invalid-key",
			model:          "gpt-4",
			messages:       []map[string]interface{}{{"role": "user", "content": "Hello"}},
			expectedStatus: http.StatusUnauthorized,
			expectedError: "invalid api key",
		},
		{
			name:           "missing model",
			apiKey:         "langchain-12345",
			messages:       []map[string]interface{}{{"role": "user", "content": "Hello"}},
			expectedStatus: http.StatusBadRequest,
			expectedError: "model and messages are required",
		},
		{
			name:           "unsupported model",
			apiKey:         "langchain-12345",
			model:          "unknown-model",
			messages:       []map[string]interface{}{{"role": "user", "content": "Hello"}},
			expectedStatus: http.StatusBadRequest,
			expectedError: "unsupported model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqBody := handlers.LangChainRequest{
				Model:    tt.model,
				Messages: tt.messages,
			}
			reqBytes, _ := json.Marshal(reqBody)

			req := httptest.NewRequest("POST", "/v1/langchain/chat/completions", bytes.NewReader(reqBytes))
			req.Header.Set("Authorization", "Bearer "+tt.apiKey)
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(handlers.LangChainCompletion)
			handler.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.expectedError != "" {
				assert.Contains(t, rr.Body.String(), tt.expectedError)
			} else {
				var response handlers.LangChainResponse
				err := json.NewDecoder(rr.Body).Decode(&response)
				require.NoError(t, err)
				assert.Equal(t, "chat.completion", response.Object)
				assert.Equal(t, tt.model, response.Model)
			}
		})
	}
}

func TestHealthCheck(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()

	HealthCheck(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.JSONEq(t, `{"status":"healthy"}`, rr.Body.String())
}




