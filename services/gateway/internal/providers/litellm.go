










package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

var (
	providerCache     = make(map[string]ProviderConfig)
	cacheMutex        = &sync.RWMutex{}
	logger            = zerolog.New(os.Stdout).With().Timestamp().Str("service", "litellm-proxy").Logger()
)

type ProviderConfig struct {
	BaseURL    string
	APIKey     string
	ModelNames []string
}

type LiteLLMConfig struct {
	Providers map[string]ProviderConfig
}

func Init(config LiteLLMConfig) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	providerCache = config.Providers
	logger.Info().Msgf("Initialized LiteLLM with %d providers", len(providerCache))
}

func GetProviderForModel(model string) (ProviderConfig, error) {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()

	for provider, config := range providerCache {
		for _, modelName := range config.ModelNames {
			if strings.EqualFold(model, modelName) {
				return config, nil
			}
		}
	}

	return ProviderConfig{}, errors.New("no provider found for model")
}

func ProxyRequest(providerConfig ProviderConfig, method, path string, body interface{}) ([]byte, error) {
	url := providerConfig.BaseURL + path

	// Create request
	reqBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequest(method, url, strings.NewReader(string(reqBody)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+providerConfig.APIKey)

	// Execute request
	client := &http.Client{Timeout: 180 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("provider returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func ListAvailableModels() []string {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()

	var models []string
	for _, config := range providerCache {
		models = append(models, config.ModelNames...)
	}
	return models
}

func AddProvider(provider string, config ProviderConfig) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	providerCache[provider] = config
	logger.Info().Str("provider", provider).Msg("Added new provider")
}

func RemoveProvider(provider string) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	delete(providerCache, provider)
	logger.Info().Str("provider", provider).Msg("Removed provider")
}

func GetAllProviders() map[string]ProviderConfig {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()

	// Return a copy to prevent modification
	copy := make(map[string]ProviderConfig)
	for k, v := range providerCache {
		copy[k] = v
	}
	return copy
}






