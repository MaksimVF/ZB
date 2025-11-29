











package providers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProviderSelection(t *testing.T) {
	// Initialize test configuration
	config := LiteLLMConfig{
		Providers: map[string]ProviderConfig{
			"openai": {
				BaseURL:    "https://api.openai.com",
				APIKey:     "test-openai-key",
				ModelNames: []string{"gpt-4", "gpt-3.5-turbo"},
			},
			"anthropic": {
				BaseURL:    "https://api.anthropic.com",
				APIKey:     "test-anthropic-key",
				ModelNames: []string{"claude-3", "claude-2"},
			},
		},
	}

	// Initialize providers
	Init(config)

	// Test provider selection
	tests := []struct {
		model     string
		expected  string
		shouldErr bool
	}{
		{"gpt-4", "https://api.openai.com", false},
		{"gpt-3.5-turbo", "https://api.openai.com", false},
		{"claude-3", "https://api.anthropic.com", false},
		{"unknown-model", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			provider, err := GetProviderForModel(tt.model)
			if tt.shouldErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, provider.BaseURL)
			}
		})
	}
}

func TestListAvailableModels(t *testing.T) {
	// Initialize test configuration
	config := LiteLLMConfig{
		Providers: map[string]ProviderConfig{
			"openai": {
				BaseURL:    "https://api.openai.com",
				APIKey:     "test-openai-key",
				ModelNames: []string{"gpt-4", "gpt-3.5-turbo"},
			},
			"anthropic": {
				BaseURL:    "https://api.anthropic.com",
				APIKey:     "test-anthropic-key",
				ModelNames: []string{"claude-3", "claude-2"},
			},
		},
	}

	// Initialize providers
	Init(config)

	// Test available models
	models := ListAvailableModels()
	expected := []string{"gpt-4", "gpt-3.5-turbo", "claude-3", "claude-2"}
	assert.ElementsMatch(t, expected, models)
}

func TestAddRemoveProvider(t *testing.T) {
	// Initialize empty configuration
	Init(LiteLLMConfig{Providers: make(map[string]ProviderConfig)})

	// Test adding a provider
	newProvider := ProviderConfig{
		BaseURL:    "https://api.newprovider.com",
		APIKey:     "new-provider-key",
		ModelNames: []string{"new-model-1", "new-model-2"},
	}

	AddProvider("newprovider", newProvider)

	// Verify provider was added
	providers := GetAllProviders()
	assert.Len(t, providers, 1)
	assert.Contains(t, providers, "newprovider")

	// Test removing a provider
	RemoveProvider("newprovider")

	// Verify provider was removed
	providers = GetAllProviders()
	assert.Len(t, providers, 0)
}

func TestProxyRequest(t *testing.T) {
	// This would be a mock test in a real implementation
	// For now, we'll just test that the function exists and doesn't panic
	config := ProviderConfig{
		BaseURL: "https://api.test.com",
		APIKey:  "test-key",
	}

	// This would normally make an HTTP request, but we're not testing that here
	// as it would require a mock server
	assert.NotPanics(t, func() {
		_, _ = ProxyRequest(config, "GET", "/test", nil)
	})
}






