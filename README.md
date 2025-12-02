# LLM Platform - Updated Architecture

## Structure
- **proto/**: Protocol buffer definitions
- **services/head-go/**: Head service (Go) - Processes requests and forwards to model proxy
- **services/tail-go/**: Tail service (Go) - REST→gRPC proxy for client requests
- **services/gateway/**: Gateway service (Go) - Alternative entry point with provider management
- **services/model-proxy/**: Model proxy (Python) - Handles LLM provider integration
- **services/secrets-service/**: Secrets management (Go) - Secure API key storage
- **ui/admin-dashboard/**: Admin UI (React) - Secret management and monitoring
- **docker-compose.yml**: Docker configuration with network separation
- **Makefile**: Build and deployment automation

## Architecture

The system now supports two request flows:

1. **Client → Tail → Head → Model Proxy → LLM Provider**
2. **Client → Gateway → Head → Model Proxy → LLM Provider**

## Key Features

### ✅ Secret Management
- API keys stored securely in Vault via secrets-service
- Admin UI for managing secrets
- Services fetch secrets via gRPC with mTLS

### ✅ Network Separation
- Client network: tail, gateway, rate-limiter, UI
- Server network: head, model-proxy, secrets-service, redis

### ✅ Compliance
- Request flows match requirements
- No hardcoded API keys
- Proper service isolation

## Deployment Steps

1. **Generate protobuf code**:
   ```bash
   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
   export PATH="$PATH:$(go env GOPATH)/bin"
   protoc -I proto --go_out=./services/head-go/gen --go-grpc_out=./services/head-go/gen proto/*.proto
   protoc -I proto --go_out=./services/tail-go/gen --go-grpc_out=./services/tail-go/gen proto/*.proto
   ```

2. **Build images**:
   ```bash
   make build
   ```

3. **Start services**:
   ```bash
   make up
   ```

4. **Configure secrets**:
   - Access Admin UI at `http://localhost:3000`
   - Add API keys for LLM providers

## Monitoring

- **Gateway**: `http://localhost:8080/metrics`
- **Admin UI**: `http://localhost:3000`
- **Service health**: `/health` endpoints on each service

## Security

- **mTLS**: All inter-service communication encrypted
- **Network isolation**: Services separated by network zones
- **Secret management**: No hardcoded credentials
