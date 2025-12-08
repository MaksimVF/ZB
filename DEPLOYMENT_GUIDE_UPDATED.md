



# Updated Deployment Guide

## Architecture Overview

The system now complies with the following architecture:

### Request Flow 1: Client → Tail → Head → Model Proxy → LLM Provider
### Request Flow 2: Client → Gateway → Head → Model Proxy → LLM Provider

## Network Configuration

- **Client Network**: Contains client-facing services
  - Tail service
  - Gateway service
  - Rate limiter
  - UI (Admin Dashboard)

- **Server Network**: Contains backend processing services
  - Head service
  - Model Proxy
  - Secrets Service
  - Redis/Cache

## Secret Management

API keys and sensitive configuration are now managed through the Secrets Service:

1. **Storage**: All secrets are stored in Vault (via secrets-service)
2. **Access**: Services fetch secrets via gRPC calls to secrets-service
3. **Management**: Admin UI provides interface to manage secrets

## Deployment Steps

### 1. Start Services

```bash
docker compose up --build
```

### 2. Configure Secrets

Access the Admin Dashboard at `http://localhost:3000` and:
1. Login with admin credentials
2. Navigate to Secrets section
3. Add required API keys:
   - `llm/openai/api_key`
   - `llm/anthropic/api_key`
   - `llm/google/api_key`
   - `llm/meta/api_key`

### 3. Verify Network Configuration

```bash
# Check client network
docker network inspect client-network

# Check server network
docker network inspect server-network
```

### 4. Test Request Flows

#### Through Tail Service:

```bash
curl -X POST http://localhost:8000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"messages": [{"role": "user", "content": "Hello"}]}'
```

#### Through Gateway Service:

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"messages": [{"role": "user", "content": "Hello"}]}'
```

## Monitoring and Logging

- **Gateway Logs**: `docker logs gateway-service`
- **Model Proxy Logs**: `docker logs model-proxy-service`
- **Secrets Service Logs**: `docker logs secret-service`

## Security Considerations

1. **mTLS**: All inter-service communication uses mutual TLS
2. **Network Isolation**: Services are separated by network zones
3. **Secret Management**: No hardcoded API keys in configuration

## Troubleshooting

### Common Issues

1. **Service Communication Failures**:
   - Check network assignments
   - Verify TLS certificates are properly configured

2. **Secret Retrieval Failures**:
   - Verify secrets-service is running
   - Check Vault configuration and health

3. **API Key Issues**:
   - Verify keys are properly stored in secrets-service
   - Check gateway logs for secret retrieval status


