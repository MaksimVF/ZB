





# Secret Service with HashiCorp Vault

## ⚠️ IMPORTANT: New Architecture

**USE `main_new.go` WITH NEW MODULAR ARCHITECTURE**

- 📖 **Read**: [NEW_ARCHITECTURE.md](NEW_ARCHITECTURE.md) for architecture details
- 🔄 **Follow**: [MIGRATION_GUIDE.md](MIGRATION_GUIDE.md) for migration instructions
- 🚫 **Avoid**: Files with `.bak` extension (deprecated)
- 📋 **See**: [DEPRECATED.go](DEPRECATED.go) for list of deprecated files

## Overview

This service provides secure secret management using HashiCorp Vault. It replaces the previous AES-GCM encryption approach with a more secure and auditable solution.

### 🆕 NEW MODULAR ARCHITECTURE

The service now uses a clean, modular architecture with clear separation of concerns:

```
services/secrets-service/
├── main_new.go                    # ✅ NEW: Main entry point
├── config/                        # ✅ NEW: Configuration management
├── core/                          # ✅ NEW: Business logic layer
├── storage/                       # ✅ NEW: Vault storage adapter
├── utils/                         # ✅ NEW: Validation & logging
├── grpc/                          # ✅ NEW: gRPC handlers
├── http/                          # ✅ NEW: HTTP admin API
└── models/                        # ✅ NEW: Data structures
```

### 🔄 Migration Status

- [x] ✅ New modular architecture implemented
- [x] ✅ Core business logic layer
- [x] ✅ gRPC and HTTP handlers
- [x] ✅ Vault storage adapter
- [x] ✅ Validation and logging utilities
- [x] ✅ Comprehensive documentation
- [ ] 🔄 Integration with existing systems
- [ ] 🔄 Migration of old tests
- [ ] 🔄 Production deployment

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

### 1. Using New Architecture (RECOMMENDED)

```bash
# Set environment variables
export VAULT_ADDR="http://vault:8200"
export VAULT_TOKEN="your-vault-token"
export ADMIN_KEY="your-admin-key"
export SERVICE_NAME="secret-service"

# Run the new modular version
go run main_new.go
```

### 2. Docker Compose

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
- `ADMIN_KEY`: Admin API key (configure via environment variable, not hardcoded)
- `ALLOWED_ORIGINS`: Comma-separated list of allowed CORS origins (default: http://localhost:3000,http://localhost:3001)

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

## ⚠️ Test Status

**OLD TESTS (main_test.go.bak) ARE DEPRECATED**

- ❌ `main_test.go.bak` - Contains tests for old monolithic architecture
- 🔄 **NEW TESTS** - Need to be written for new modular architecture
- 📖 **See**: `MIGRATION_GUIDE.md` for test migration instructions

### Old Test Examples (DEPRECATED)

Run tests with (NOT RECOMMENDED):
```bash
go test -v ./...  # Will fail due to old architecture
```

### Test Cases to Implement

1. **Core Service Tests**: Business logic validation
2. **Storage Layer Tests**: Vault adapter testing
3. **HTTP Handler Tests**: API endpoint testing
4. **gRPC Handler Tests**: gRPC service testing
5. **Integration Tests**: End-to-end testing

## Documentation

- 📚 [NEW_ARCHITECTURE.md](NEW_ARCHITECTURE.md) - Detailed architecture documentation
- 🔄 [MIGRATION_GUIDE.md](MIGRATION_GUIDE.md) - Migration instructions
- 🚫 [DEPRECATED.go](DEPRECATED.go) - List of deprecated files

## Monitoring

The service exposes Prometheus metrics at `/metrics` including:

- `secret_operations_total`: Count of secret operations by type and status
- `http_request_duration_seconds`: HTTP request latency by method and path

## Security

For detailed security information, including configuration and best practices, please refer to the [SECURITY.md](SECURITY.md) document.

## Contributing

1. Fork the repository
2. Create a new branch
3. Make your changes
4. Add appropriate tests
5. Submit a pull request

## License

This project is licensed under the MIT License.













