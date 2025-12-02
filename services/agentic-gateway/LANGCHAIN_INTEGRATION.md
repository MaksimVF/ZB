









# LangChain Integration Guide

## Overview

This guide explains how to integrate the Gateway Service with LangChain to provide seamless LLM access with usage tracking and billing.

## Features

- **Full OpenAI API Compatibility**: 100% compatible with LangChain's OpenAI integration
- **Usage Tracking**: Detailed tracking of token usage for billing
- **Multi-Provider Support**: Access to OpenAI, Anthropic, Google, and Meta models
- **Streaming Support**: Full streaming compatibility
- **Enhanced Monitoring**: Prometheus metrics and health checks

## Integration Steps

### 1. Configuration

Set up your environment variables:

```bash
export GATEWAY_URL="https://your-gateway.com"
export API_KEY="langchain-xxxxxxxxxxxxx"
```

### 2. LangChain Integration

```python
from langchain_openai import ChatOpenAI
from langchain_core.messages import HumanMessage

# Initialize LangChain with the gateway
llm = ChatOpenAI(
    base_url=f"{GATEWAY_URL}/v1/langchain",
    api_key=API_KEY,
    model="gpt-4",  # or any supported model
    temperature=0.7,
    streaming=True,  # or False
)

# Use it just like OpenAI's API
response = llm.invoke([HumanMessage(content="Hello, how are you?")])
print(response.content)
```

### 3. Supported Models

| Model | Provider | Status |
|-------|----------|--------|
| gpt-4 | OpenAI | ✅ |
| gpt-3.5 | OpenAI | ✅ |
| claude-3 | Anthropic | ✅ |
| gemini-1.5 | Google | ✅ |
| llama-3 | Meta | ✅ |

### 4. Usage Tracking

The gateway automatically tracks:

- **User ID**: Extracted from API key
- **Model Used**: Which LLM model was used
- **Token Count**: Number of tokens processed
- **Cost**: Calculated based on model pricing

### 5. Billing

Pricing per 1000 tokens:

| Model | Price (USD) |
|-------|------------|
| gpt-4 | $0.06 |
| gpt-3.5 | $0.002 |
| claude-3 | $0.04 |
| gemini-1.5 | $0.03 |
| llama-3 | $0.02 |

### 6. Monitoring

Prometheus metrics available at `/metrics`:

- `langchain_requests_total`: Count of LangChain requests
- `langchain_request_duration_seconds`: Request latency
- `auth_operations_total`: Authentication operations

### 7. Health Check

```bash
curl https://your-gateway.com/health
```

### 8. Testing

Run the test script:

```bash
python test_langchain.py
```

## Benefits

1. **Seamless Integration**: Works with LangChain out of the box
2. **Usage Tracking**: Detailed monitoring of LangChain usage
3. **Multi-Provider**: Access to multiple LLM providers
4. **Scalable**: Handles high volumes of requests
5. **Secure**: Proper authentication and error handling

## Monetization Strategy

### 1. LangChain-Specific Pricing

Create a separate pricing tier for LangChain users:

- **Standard API**: $0.01 per 1000 tokens
- **LangChain API**: $0.015 per 1000 tokens (premium support)

### 2. Usage-Based Billing

Track usage with the billing system and charge based on:

- Token count
- Model used
- Request volume

### 3. Premium Features

Offer enhanced features for LangChain users:

- Priority access
- Higher rate limits
- Dedicated support
- Custom integrations

### 4. Enterprise Plans

Create enterprise plans with:

- Volume discounts
- Custom SLAs
- On-premise deployment options
- Advanced analytics

## Implementation Status

- [x] LangChain-specific endpoint
- [x] Usage tracking system
- [x] Multi-provider support
- [x] Streaming compatibility
- [x] Prometheus monitoring
- [x] Health checks
- [x] Test coverage

## Support

For support, contact:

- Email: support@your-gateway.com
- Slack: #langchain-support
- GitHub: github.com/your-gateway/langchain-integration

## License

This integration is licensed under the MIT License.







