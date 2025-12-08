










# LiteLLM Integration Guide

## Overview

This guide explains how the Gateway Service integrates with LiteLLM to provide dynamic provider selection and routing for LangChain and other LLM clients.

## Features

- **Dynamic Provider Selection**: Automatically routes requests to the appropriate provider based on model name
- **Multi-Provider Support**: Works with OpenAI, Anthropic, Google, Meta, and other providers
- **Flexible Configuration**: Easily add, remove, or update providers via API
- **Usage Tracking**: Detailed tracking of token usage by provider and model
- **LangChain Compatibility**: Full compatibility with LangChain's OpenAI API integration

## Architecture

```
LangChain Clients → Gateway Service → LiteLLM Router → LLM Providers
                    ↓
               Usage Tracking
                    ↓
                Billing System
```

## Configuration

### 1. Provider Configuration

Providers are configured in the `main.go` file:

```go
providerConfig := providers.LiteLLMConfig{
    Providers: map[string]providers.ProviderConfig{
        "openai": {
            BaseURL:    "https://api.openai.com",
            APIKey:     os.Getenv("OPENAI_API_KEY"),
            ModelNames: []string{"gpt-4", "gpt-3.5-turbo", "gpt-4o"},
        },
        "anthropic": {
            BaseURL:    "https://api.anthropic.com",
            APIKey:     os.Getenv("ANTHROPIC_API_KEY"),
            ModelNames: []string{"claude-3", "claude-2", "claude-instant"},
        },
        // Add more providers as needed
    },
}
```

### 2. Environment Variables

Set the following environment variables:

```bash
export OPENAI_API_KEY="your-openai-key"
export ANTHROPIC_API_KEY="your-anthropic-key"
export GOOGLE_API_KEY="your-google-key"
export META_API_KEY="your-meta-key"
```

### 3. API Endpoints

#### Provider Management

- **List Providers**: `GET /v1/providers`
- **Add Provider**: `POST /v1/providers`
- **Remove Provider**: `DELETE /v1/providers/{provider}`

#### LangChain Integration

- **LangChain Endpoint**: `POST /v1/langchain/chat/completions`
- **Standard Endpoint**: `POST /v1/chat/completions`

## Usage

### 1. LangChain Integration

```python
from langchain_openai import ChatOpenAI
from langchain_core.messages import HumanMessage

# Initialize LangChain with the gateway
llm = ChatOpenAI(
    base_url="https://your-gateway.com/v1/langchain",
    api_key="langchain-xxxxxxxxxxxxx",
    model="gpt-4",  # Automatically routed to OpenAI
    temperature=0.7,
    streaming=True,
)

# Use it just like OpenAI's API
response = llm.invoke([HumanMessage(content="Hello, how are you?")])
print(response.content)
```

### 2. Dynamic Provider Selection

The gateway automatically routes requests based on the model name:

- `gpt-4` → OpenAI
- `claude-3` → Anthropic
- `gemini-1.5` → Google
- `llama-3` → Meta

### 3. Adding New Providers

```bash
curl -X POST -H "Content-Type: application/json" -d '{
    "base_url": "https://api.newprovider.com",
    "api_key": "new-provider-key",
    "model_names": ["new-model-1", "new-model-2"]
}' https://your-gateway.com/v1/providers
```

## Benefits

1. **Flexibility**: Easily add or remove providers without code changes
2. **Scalability**: Route requests to the best available provider
3. **Cost Optimization**: Choose providers based on pricing and performance
4. **Resilience**: Failover between providers if one is unavailable
5. **Extensibility**: Support for any LLM provider with a compatible API

## Implementation

### 1. Provider Selection

The `GetProviderForModel` function in `litellm.go` dynamically selects the appropriate provider based on the model name.

### 2. Request Proxying

The `ProxyRequest` function handles the actual request forwarding to the selected provider.

### 3. Usage Tracking

Each request is tracked with:
- User ID
- Provider
- Model
- Token count
- Request duration

### 4. Monitoring

Prometheus metrics include:
- `langchain_requests_total`: Count of requests by model and status
- `langchain_request_duration_seconds`: Request latency by model

## Testing

### 1. Provider Selection Test

```go
func TestProviderSelection(t *testing.T) {
    provider, err := providers.GetProviderForModel("gpt-4")
    assert.NoError(t, err)
    assert.Equal(t, "openai", provider.BaseURL)
}
```

### 2. Integration Test

```python
def test_provider_routing():
    # Test with different models
    models = ["gpt-4", "claude-3", "gemini-1.5", "llama-3"]

    for model in models:
        response = llm.invoke(f"Testing {model}")
        assert response.content is not None
```

## Future Enhancements

1. **Load Balancing**: Implement round-robin or performance-based load balancing
2. **Failover**: Automatic failover between providers
3. **Cost Optimization**: Route requests based on cost and performance
4. **Provider Health Checks**: Monitor provider availability and performance
5. **Rate Limiting**: Implement provider-specific rate limiting

## Support

For support, contact:

- Email: support@your-gateway.com
- Slack: #litellm-support
- GitHub: github.com/your-gateway/litellm-integration

## License

This integration is licensed under the MIT License.








