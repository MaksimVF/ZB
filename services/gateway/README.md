





# Gateway Service (Agent Gateway)

## Overview

**Purpose**: This service provides a unified gateway for LLM APIs with special support for LangChain integration and agent-related functionality. It serves as the agent-focused API gateway in our architecture.

**Role in Architecture**: The Gateway Service is the entry point for agent-related API calls and LLM provider management. It routes requests to the appropriate services and handles provider-specific logic.

This service includes:

- **OpenAI-compatible API**: Full compatibility with OpenAI API v1
- **LangChain-specific endpoint**: Special endpoint for LangChain with usage tracking
- **Multi-provider support**: Works with OpenAI, Anthropic, Google, and Meta models
- **Comprehensive monitoring**: Prometheus metrics and health checks
- **Error handling**: Proper error handling and logging
- **LiteLLM Integration**: Dynamic provider selection and routing

## Features

| Feature | Status | LangChain Compatibility |
|---------|--------|-------------------------|
| OpenAI API v1 compatibility | ✅ | 100% |
| Streaming support | ✅ | ✅ |
| Tool calls/function calling | ✅ | ✅ |
| Response format handling | ✅ | ✅ |
| Finish reason handling | ✅ | ✅ |
| Token usage tracking | ✅ | ✅ |
| Multi-provider support | ✅ | ✅ |
| Rate limiting | ✅ | ✅ |
| Usage tracking | ✅ | ✅ |
| Dynamic provider routing | ✅ | ✅ |
| Provider management API | ✅ | ✅ |

## Architecture

```
LangChain Clients → Gateway Service → LiteLLM Router → LLM Providers
                    ↓
               Usage Tracking
                    ↓
                Billing System
```

## Deployment

### 1. Docker

```bash
docker build -t gateway-service .
docker run -p 8080:8080 gateway-service
```

### 2. Environment Variables

- `OPENAI_API_KEY`: OpenAI API key
- `ANTHROPIC_API_KEY`: Anthropic API key
- `GOOGLE_API_KEY`: Google API key
- `META_API_KEY`: Meta API key

## Usage

### 1. LangChain Integration

```python
from langchain_openai import ChatOpenAI

llm = ChatOpenAI(
    base_url="https://your-gateway.com/v1/langchain",
    api_key="langchain-xxxxxxxxxxxxx",
    model="gpt-4o",  # Automatically routed to OpenAI
    temperature=0.7,
    streaming=True,
)

response = llm.invoke("Hello, how are you?")
print(response.content)
```

### 2. Standard OpenAI API

```bash
curl -X POST -H "Authorization: Bearer your-api-key" \
     -H "Content-Type: application/json" \
     -d '{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}' \
     https://your-gateway.com/v1/chat/completions
```

### 3. Provider Management

- **List Providers**: `GET /v1/providers`
- **Add Provider**: `POST /v1/providers`
- **Remove Provider**: `DELETE /v1/providers/{provider}`

### 4. Health Check

```bash
curl https://your-gateway.com/health
```

### 5. Metrics

```bash
curl https://your-gateway.com/metrics
```

## Monetization

The service includes usage tracking for LangChain requests, allowing you to:

1. **Create LangChain-specific API keys**: Issue special API keys that identify LangChain usage
2. **Track token usage**: Monitor token consumption by LangChain users
3. **Implement tiered pricing**: Charge different rates for LangChain vs standard API usage
4. **Offer premium features**: Provide enhanced features for LangChain users

## Implementation

### 1. LangChain Endpoint

The `/v1/langchain/chat/completions` endpoint:
- Validates LangChain-specific API keys
- Tracks usage for billing
- Provides enhanced monitoring
- Ensures full compatibility with LangChain

### 2. LiteLLM Integration

The gateway uses LiteLLM for:
- **Dynamic Provider Selection**: Automatically routes requests based on model name
- **Flexible Configuration**: Easily add, remove, or update providers via API
- **Multi-Provider Support**: Works with any LLM provider with a compatible API

### 3. Usage Tracking

Each request is tracked with:
- User ID
- Provider
- Model used
- Token count
- Request duration

### 4. Monitoring

Prometheus metrics include:
- `langchain_requests_total`: Count of LangChain requests
- `langchain_request_duration_seconds`: Request latency
- `auth_operations_total`: Authentication operations

## Benefits

- **Seamless Integration**: Works with LangChain out of the box
- **Usage Tracking**: Detailed monitoring of LangChain usage
- **Multi-Provider**: Access to multiple LLM providers
- **Scalable**: Handles high volumes of requests
- **Secure**: Proper authentication and error handling
- **Flexible**: Easily add or remove providers without code changes

## Security Features

The gateway service includes several security features:

1. **mTLS Authentication**: All services communicate using mutual TLS
2. **Rate Limiting**: Prevents abuse of the API
3. **Circuit Breakers**: Protects against cascading failures
4. **Request Validation**: Ensures proper request formats
5. **Content Filtering**: Filters malicious content and SQL injection attempts
6. **Audit Logging**: Logs sensitive operations for compliance
7. **Data Isolation**: Ensures data separation between clients
8. **User-Configurable Security**: Users can enable/disable security features via API

## Security Configuration

Users can configure security settings via the API:

- **GET /v1/security/config**: Get current security configuration
- **PUT /v1/security/config**: Update security configuration

Configuration options:
- `content_filtering_enabled`: Enable/disable content filtering
- `audit_logging_enabled`: Enable/disable audit logging
- `data_isolation_enabled`: Enable/disable data isolation

## Contributing

1. Fork the repository
2. Create a new branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

This project is licensed under the MIT License.



