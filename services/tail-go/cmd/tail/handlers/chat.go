







package handlers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"llm-gateway-pro/services/gateway/internal/secrets"
	"llm-gateway-pro/services/tail-go/cmd/tail/internal"
)

type OpenAIRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func ChatCompletion(w http.ResponseWriter, r *http.Request) {
	var req OpenAIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	// Определяем провайдера по модели с использованием LiteLLM
	provider, err := internal.GetProviderForModel(req.Model)
	if err != nil {
		log.Printf("Ошибка определения провайдера для модели %s: %v", req.Model, err)
		http.Error(w, `{"error":"model not supported"}`, http.StatusBadRequest)
		return
	}

	// Get user ID from request (assuming it's in the header)
	userID := r.Header.Get("X-User-ID")

	// Try to get user-specific API key
	var apiKey string
	if userID != "" {
		apiKey, err = secrets.GetUserSecret(userID, fmt.Sprintf("llm/%s/api_key", provider))
		if err != nil {
			// Fall back to shared API key
			apiKey, err = secrets.Get(fmt.Sprintf("llm/%s/api_key", provider))
			if err != nil {
				log.Printf("Ошибка получения секрета %s: %v", provider, err)
				http.Error(w, `{"error":"internal configuration error"}`, http.StatusInternalServerError)
				return
			}
			log.Printf("Using shared API key for user %s, provider %s", userID, provider)
		} else {
			log.Printf("Using user-specific API key for user %s, provider %s", userID, provider)
		}
	} else {
		// Use shared API key
		apiKey, err = secrets.Get(fmt.Sprintf("llm/%s/api_key", provider))
		if err != nil {
			log.Printf("Ошибка получения секрета %s: %v", provider, err)
			http.Error(w, `{"error":"internal configuration error"}`, http.StatusInternalServerError)
			return
		}
		log.Printf("Using shared API key for provider %s", provider)
	}

	// Формируем запрос к провайдеру
	providerURL, err := internal.GetProviderBaseURL(provider)
	if err != nil {
		log.Printf("Ошибка получения базового URL для провайдера %s: %v", provider, err)
		http.Error(w, `{"error":"provider configuration error"}`, http.StatusInternalServerError)
		return
	}

	client := &http.Client{Timeout: 180 * time.Second}

	// Пересылаем тело почти без изменений
	proxyReq, _ := http.NewRequest("POST", providerURL+"/v1/chat/completions", r.Body)
	proxyReq.Header.Set("Authorization", "Bearer "+apiKey)
	proxyReq.Header.Set("Content-Type", "application/json")

	if req.Stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		resp, err := client.Do(proxyReq)
		if err != nil {
			http.Error(w, `{"error":"provider unreachable"}`, http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				io.WriteString(w, line+"\n\n")
				if flusher, ok := w.(http.Flusher); ok {
					flusher.Flush()
				}
			}
		}
		return
	}

	// Не стриминг — обычный запрос
	resp, err := client.Do(proxyReq)
	if err != nil {
		http.Error(w, `{"error":"provider error"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}







