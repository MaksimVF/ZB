





# Secret Service with HashiCorp Vault

## Overview

This service provides secure secret management using HashiCorp Vault. It replaces the previous AES-GCM encryption approach with a more secure and auditable solution.

## Features

- **HashiCorp Vault Integration:** Secure secret storage and management
- **gRPC Interface:** Secure communication with mTLS
- **HTTP Admin API:** Web interface for secret management
- **Audit Capabilities:** Full audit trail of secret access
- **Key Rotation:** Support for automatic key rotation

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

## Benefits

- **Security:** Secrets are never stored in Redis
- **Audit:** Full audit capabilities (who and when read a key)
- **Rotation:** Automatic rotation support
- **Compliance:** Ready for SOC2/ISO27001
- **HSM Support:** Can integrate with HSM/AWS KMS/GCP KMS

## Implementation Status

- [x] Vault integration
- [x] gRPC interface with mTLS
- [x] HTTP admin API
- [x] Docker deployment
- [ ] Comprehensive error handling
- [ ] Detailed logging
- [ ] Test cases

## Next Steps

1. Add comprehensive error handling
2. Implement detailed logging
3. Add test cases
4. Integrate with monitoring

## Contributing

1. Fork the repository
2. Create a new branch
3. Make your changes
4. Submit a pull request

## License

This project is licensed under the MIT License.













