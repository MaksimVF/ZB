





# Secret Service with HashiCorp Vault

## Overview

This service provides secure secret management using HashiCorp Vault. It replaces the previous AES-GCM encryption approach with a more secure and auditable solution.

## Features

- **HashiCorp Vault Integration:** Secure secret storage and management
- **gRPC Interface:** Secure communication with mTLS
- **HTTP Admin API:** Web interface for secret management
- **Audit Capabilities:** Full audit trail of secret access
- **Key Rotation:** Support for automatic key rotation
- **Comprehensive Error Handling:** Detailed error responses for different failure scenarios
- **Structured Logging:** Detailed logging for all operations
- **Prometheus Monitoring:** Metrics collection and health checks
- **Test Coverage:** Unit and integration tests

## Architecture

```
UI (Vercel) ─HTTPS─→ Secret Service (HTTP /admin/api/secrets)
                         ↓
                   Vault (Central Storage)
                         ↑
               gRPC + mTLS + Vault Token
                         ↓
Gateway, Billing, Rate Limiter ← Plaintext in memory
```

## Deployment

### 1. Docker Compose

```bash
docker-compose -f docker-compose.yml up --build
```

### 2. Vault Initialization

Run the initialization script:

```bash
./init-vault.sh
```

### 3. Environment Variables

- `VAULT_ADDR`: Vault address (default: http://vault:8200)
- `VAULT_TOKEN`: Vault token with proper rights
- `ADMIN_KEY`: Admin API key

## Usage

### 1. Storing Secrets

```bash
# Through UI: http://localhost:8200 → secret/llm/openai/api_key → value: sk-...
# Or CLI:
vault kv put secret/llm/openai/api_key value=sk-XXXXXXXXXXXXXXXX
vault kv put secret/llm/anthropic/api_key value=anthropic-...
```

### 2. Accessing Secrets

The service provides a gRPC interface for other services to access secrets securely.

### 3. Admin Interface

The HTTP admin API provides endpoints for managing secrets:

- `GET /admin/api/secrets`: List secrets
- `POST /admin/api/secrets`: Create/update secret
- `DELETE /admin/api/secrets/{name}`: Delete secret

### 4. Health Check

- `GET /health`: Check service health

### 5. Metrics

- `GET /metrics`: Prometheus metrics endpoint

## Benefits

- **Security:** Secrets are never stored in Redis
- **Audit:** Full audit capabilities (who and when read a key)
- **Rotation:** Automatic rotation support
- **Compliance:** Ready for SOC2/ISO27001
- **HSM Support:** Can integrate with HSM/AWS KMS/GCP KMS
- **Observability:** Comprehensive logging and monitoring
- **Reliability:** Proper error handling and test coverage

## Implementation Status

- [x] Vault integration
- [x] gRPC interface with mTLS
- [x] HTTP admin API
- [x] Docker deployment
- [x] Comprehensive error handling
- [x] Detailed logging
- [x] Test cases
- [x] Prometheus monitoring integration

## Test Examples

### Unit Tests

Run tests with:
```bash
go test -v ./...
```

### Test Cases

1. **GetSecret Success**: Retrieve an existing secret
2. **GetSecret Not Found**: Attempt to retrieve a non-existent secret
3. **GetSecret Vault Error**: Handle Vault connection errors
4. **Admin API Authentication**: Test admin key validation
5. **Admin API Operations**: Test GET, POST, DELETE operations

## Monitoring

The service exposes Prometheus metrics at `/metrics` including:

- `secret_operations_total`: Count of secret operations by type and status
- `http_request_duration_seconds`: HTTP request latency by method and path

## Contributing

1. Fork the repository
2. Create a new branch
3. Make your changes
4. Add appropriate tests
5. Submit a pull request

## License

This project is licensed under the MIT License.













