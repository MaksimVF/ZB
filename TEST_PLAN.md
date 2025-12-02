


# Test Plan for Architecture Compliance

## Objective
Verify that the system architecture now complies with the requirements:
1. Request flow matches: Client → Tail → Head → Model Proxy → LLM Provider OR Client → Gateway → Head → Model Proxy → LLM Provider
2. API keys are managed through secrets-service, not hardcoded
3. Network separation between client/tail services and head/model-proxy services

## Test Cases

### 1. Request Flow Verification

#### Test 1.1: Client → Tail → Head → Model Proxy → LLM Provider
- **Setup**: Start all services with docker-compose
- **Action**: Send request to Tail service endpoint
- **Expected**:
  - Request should be routed to Head service
  - Head service should forward to Model Proxy
  - Model Proxy should process and return response

#### Test 1.2: Client → Gateway → Head → Model Proxy → LLM Provider
- **Setup**: Start all services with docker-compose
- **Action**: Send request to Gateway service endpoint
- **Expected**:
  - Request should be routed to Head service
  - Head service should forward to Model Proxy
  - Model Proxy should process and return response

### 2. Secret Management Verification

#### Test 2.1: API Keys from Secrets Service
- **Setup**: Configure secrets in secrets-service
- **Action**: Check agentic-gateway logs for API key retrieval
- **Expected**:
  - Gateway should fetch API keys from secrets-service
  - No hardcoded API keys in environment variables

#### Test 2.2: Admin UI Secret Management
- **Setup**: Access admin UI
- **Action**: Add/update secrets through UI
- **Expected**:
  - Secrets should be stored in secrets-service
  - Changes should be reflected in agentic-gateway behavior

### 3. Network Separation Verification

#### Test 3.1: Network Isolation
- **Setup**: Check docker network configuration
- **Action**: Verify service network assignments
- **Expected**:
  - Client, tail, agentic-gateway in client_network
  - Head, model-proxy, secrets-service in server_network
  - Gateway in both networks for communication

#### Test 3.2: Cross-Network Communication
- **Setup**: Send requests between services
- **Action**: Verify communication works as expected
- **Expected**:
  - Tail can communicate with Head
  - Gateway can communicate with Head and secrets-service
  - Head can communicate with Model Proxy

## Verification Commands

```bash
# Start services
docker compose up --build

# Check network configuration
docker network inspect client-network
docker network inspect server-network

# Test request flow
curl -X POST http://localhost:8000/v1/chat/completions -H "Content-Type: application/json" -d '{"messages": [{"role": "user", "content": "Hello"}]}'

curl -X POST http://localhost:8080/v1/chat/completions -H "Content-Type: application/json" -d '{"messages": [{"role": "user", "content": "Hello"}]}'

# Check logs for secret retrieval
docker logs agentic-gateway-service
docker logs model-proxy-service
```

## Expected Outcomes

1. ✅ Requests flow through the correct paths
2. ✅ API keys are dynamically fetched from secrets-service
3. ✅ Network separation is properly enforced
4. ✅ Admin UI can manage secrets through secrets-service

