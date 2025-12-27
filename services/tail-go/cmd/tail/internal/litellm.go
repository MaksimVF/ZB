package internal

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/MaksimVF/ZB/services/tail-go/cmd/tail/internal/secrets"
	"github.com/redis/go-redis/v9"
)

var (
	// LiteLLM provider configuration
	providerConfig = map[string]ProviderConfig{
		"openai": {
			BaseURL: "https://api.openai.com",
			Models:  []string{"gpt-4o", "gpt-4-turbo", "gpt-3.5-turbo"},
		},
		"anthropic": {
			BaseURL: "https://api.anthropic.com",
			Models:  []string{"claude-3-opus", "claude-3-sonnet", "claude-3-haiku"},
		},
		"google": {
			BaseURL: "https://generativelanguage.googleapis.com",
			Models:  []string{"gemini-pro", "gemini-ultra"},
		},
		"groq": {
			BaseURL: "https://api.groq.com/openai",
			Models:  []string{"llama3-70b", "llama3-8b"},
		},
	}

	// Provider health status
	providerHealth = map[string]bool{
		"openai":    true,
		"anthropic": true,
		"google":    true,
		"groq":      true,
	}

	// Mutex for provider health updates
	healthMutex sync.RWMutex

	// Redis client for provider management
	redisClient *redis.Client
)

func init() {
	// Initialize Redis client
	redisClient = redis.NewClient(&redis.Options{
		Addr: "redis:6379",
	})

	// Start health check goroutine
	go startHealthChecks()
}

// ProviderConfig represents the configuration for an LLM provider
type ProviderConfig struct {
	BaseURL string   `json:"base_url"`
	Models  []string `json:"models"`
	APIKey  string   `json:"api_key"`
}

// GetProviderForModel returns the appropriate provider for a given model
func GetProviderForModel(model string) (string, error) {
	healthMutex.RLock()
	defer healthMutex.RUnlock()

	for provider, config := range providerConfig {
		for _, m := range config.Models {
			if m == model {
				if providerHealth[provider] {
					return provider, nil
				}
				return "", fmt.Errorf("provider %s is unhealthy", provider)
			}
		}
	}
	return "", fmt.Errorf("model %s not found", model)
}

// GetProviderBaseURL returns the base URL for a provider
func GetProviderBaseURL(provider string) (string, error) {
	healthMutex.RLock()
	defer healthMutex.RUnlock()

	if config, exists := providerConfig[provider]; exists {
		return config.BaseURL, nil
	}
	return "", fmt.Errorf("provider %s not configured", provider)
}

// GetProviderAPIKey returns the API key for a provider
func GetProviderAPIKey(provider string) (string, error) {
	healthMutex.RLock()
	defer healthMutex.RUnlock()

	if config, exists := providerConfig[provider]; exists {
		if config.APIKey != "" {
			return config.APIKey, nil
		}
	}
	return "", fmt.Errorf("API key for provider %s not configured", provider)
}

// SetProviderAPIKey sets the API key for a provider
func SetProviderAPIKey(provider, apiKey string) error {
	healthMutex.Lock()
	defer healthMutex.Unlock()

	if config, exists := providerConfig[provider]; exists {
		config.APIKey = apiKey
		providerConfig[provider] = config
		return nil
	}
	return fmt.Errorf("provider %s not configured", provider)
}

// AddProvider adds a new provider configuration
func AddProvider(provider string, config ProviderConfig) error {
	healthMutex.Lock()
	defer healthMutex.Unlock()

	providerConfig[provider] = config
	providerHealth[provider] = true
	return nil
}

// RemoveProvider removes a provider configuration
func RemoveProvider(provider string) error {
	healthMutex.Lock()
	defer healthMutex.Unlock()

	if _, exists := providerConfig[provider]; exists {
		delete(providerConfig, provider)
		delete(providerHealth, provider)
		return nil
	}
	return fmt.Errorf("provider %s not found", provider)
}

// startHealthChecks periodically checks provider health
func startHealthChecks() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		checkProviderHealth()
	}
}

// checkProviderHealth checks the health of all providers
func checkProviderHealth() {
	healthMutex.Lock()
	defer healthMutex.Unlock()

	for provider, config := range providerConfig {
		// Perform actual health check by making a test API call
		healthy := performProviderHealthCheck(provider, config.BaseURL)
		providerHealth[provider] = healthy

		if !healthy {
			log.Printf("Provider %s is unhealthy", provider)
		}
	}
}

// performProviderHealthCheck performs an actual health check for a provider
func performProviderHealthCheck(provider, baseURL string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var url string
	switch provider {
	case "openai":
		url = baseURL + "/v1/models"
	case "anthropic":
		url = baseURL + "/v1/messages" // Simple endpoint check
	case "google":
		url = baseURL + "/v1/models" // Gemini models endpoint
	case "groq":
		url = baseURL + "/openai/v1/models"
	default:
		return false
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false
	}

	// Try to get API key for authentication
	apiKey, err := getProviderAPIKeyForHealthCheck(provider)
	if err == nil && apiKey != "" {
		switch provider {
		case "openai", "groq":
			req.Header.Set("Authorization", "Bearer "+apiKey)
		case "anthropic":
			req.Header.Set("x-api-key", apiKey)
			req.Header.Set("anthropic-version", "2023-06-01")
		case "google":
			req.URL.RawQuery = "key=" + apiKey
		}
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// Consider 2xx and 4xx (auth errors) as healthy, 5xx as unhealthy
	return resp.StatusCode < 500
}

// getProviderAPIKeyForHealthCheck gets API key for health checks
func getProviderAPIKeyForHealthCheck(provider string) (string, error) {
	// Try to get from secrets service
	return secrets.Get(fmt.Sprintf("llm/%s/api_key", provider))
}

// GetAllProviders returns the list of all configured providers
func GetAllProviders() (map[string]ProviderConfig, error) {
	healthMutex.RLock()
	defer healthMutex.RUnlock()

	// Return a copy to avoid race conditions
	result := make(map[string]ProviderConfig)
	for k, v := range providerConfig {
		result[k] = v
	}
	return result, nil
}

// GetProviderHealth returns the health status of all providers
func GetProviderHealth() (map[string]bool, error) {
	healthMutex.RLock()
	defer healthMutex.RUnlock()

	// Return a copy to avoid race conditions
	result := make(map[string]bool)
	for k, v := range providerHealth {
		result[k] = v
	}
	return result, nil
}
