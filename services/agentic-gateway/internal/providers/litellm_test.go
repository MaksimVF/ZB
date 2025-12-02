











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
				UseGRPC:    false,
			},
			"anthropic": {
				BaseURL:    "https://api.anthropic.com",
				APIKey:     "test-anthropic-key",
				ModelNames: []string{"claude-3", "claude-2"},
				UseGRPC:    false,
			},
			"local": {
				BaseURL:    "local",
				APIKey:     "local-key",
				ModelNames: []string{"local-model"},
				GRPCAddress: "localhost:50061",
				UseGRPC:    true,
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
		useGRPC   bool
	}{
		{"gpt-4", "https://api.openai.com", false, false},
		{"gpt-3.5-turbo", "https://api.openai.com", false, false},
		{"claude-3", "https://api.anthropic.com", false, false},
		{"local-model", "local", false, true},
		{"unknown-model", "", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			provider, err := GetProviderForModel(tt.model)
			if tt.shouldErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, provider.BaseURL)
				assert.Equal(t, tt.useGRPC, provider.UseGRPC)
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
				UseGRPC:    false,
			},
			"anthropic": {
				BaseURL:    "https://api.anthropic.com",
				APIKey:     "test-anthropic-key",
				ModelNames: []string{"claude-3", "claude-2"},
				UseGRPC:    false,
			},
			"local": {
				BaseURL:    "local",
				APIKey:     "local-key",
				ModelNames: []string{"local-model"},
				GRPCAddress: "localhost:50061",
				UseGRPC:    true,
			},
		},
	}

	// Initialize providers
	Init(config)

	// Test available models
	models := ListAvailableModels()
	expected := []string{"gpt-4", "gpt-3.5-turbo", "claude-3", "claude-2", "local-model"}
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
		UseGRPC:    false,
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
		UseGRPC: false,
	}

	// This would normally make an HTTP request, but we're not testing that here
	// as it would require a mock server
	assert.NotPanics(t, func() {
		_, _ = ProxyRequest(config, "GET", "/test", nil)
	})
}

func TestGRPCProxyRequest(t *testing.T) {
	// Test gRPC proxy request
	config := ProviderConfig{
		BaseURL:    "local",
		APIKey:     "local-key",
		ModelNames: []string{"local-model"},
		GRPCAddress: "localhost:50061",
		UseGRPC:    true,
	}

	// This would normally make a gRPC request, but we're not testing that here
	// as it would require a mock server
	assert.NotPanics(t, func() {
		_, _ = ProxyRequest(config, "GET", "/test", map[string]interface{}{
			"model": "local-model",
			"messages": []map[string]string{
				{"role": "user", "content": "test"},
			},
			"temperature": 0.7,
			"max_tokens":   100,
		})
	})
}






