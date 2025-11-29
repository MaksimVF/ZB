



# Authentication Service

## Overview

This service provides comprehensive authentication and authorization for the platform. It includes user registration, login, API key management, and JWT-based authentication.

## Features

- **User Registration & Login**: Secure user authentication
- **JWT Token Support**: Stateless authentication with JWT tokens
- **API Key Management**: Create and manage API keys
- **Rate Limiting**: Protection against brute force attacks
- **Comprehensive Error Handling**: Detailed error responses
- **Structured Logging**: Detailed logging for all operations
- **Prometheus Monitoring**: Metrics collection and health checks
- **Test Coverage**: Unit and integration tests

## Architecture

```
UI (Vercel) ─HTTPS─→ Auth Service (HTTP /register, /login, /me)
                         ↓
                   PostgreSQL (User Storage)
                         ↓
                   Redis (Rate Limiting)
                         ↓
               gRPC + mTLS + JWT Token
                         ↓
Gateway, Billing, Rate Limiter ← JWT Validation
```

## Deployment

### 1. Docker Compose

```bash
docker-compose -f docker-compose.yml up --build
```

### 2. Environment Variables

- `JWT_SECRET`: JWT signing secret
- `DB_HOST`: Database host
- `DB_USER`: Database user
- `DB_PASSWORD`: Database password
- `DB_NAME`: Database name
- `DB_PORT`: Database port
- `REDIS_ADDR`: Redis address
- `REDIS_PASSWORD`: Redis password (if any)

## Usage

### 1. User Registration

```bash
curl -X POST -H "Content-Type: application/json" -d '{
  "email": "user@example.com",
  "password": "StrongPass123"
}' http://localhost:8081/register
```

### 2. User Login

```bash
curl -X POST -H "Content-Type: application/json" -d '{
  "email": "user@example.com",
  "password": "StrongPass123"
}' http://localhost:8081/login
```

### 3. Get User Info

```bash
curl -H "Authorization: Bearer <JWT_TOKEN>" http://localhost:8081/me
```

### 4. API Key Management

```bash
# List API keys
curl -H "Authorization: Bearer <JWT_TOKEN>" http://localhost:8081/api-keys

# Create API key
curl -X POST -H "Authorization: Bearer <JWT_TOKEN>" -H "Content-Type: application/json" -d '{
  "name": "My API Key"
}' http://localhost:8081/api-keys
```

### 5. Health Check

```bash
curl http://localhost:8081/health
```

### 6. Metrics

```bash
curl http://localhost:8081/metrics
```

## Benefits

- **Security**: JWT tokens, password hashing, rate limiting
- **Scalability**: Stateless authentication with JWT
- **Observability**: Comprehensive logging and monitoring
- **Reliability**: Proper error handling and test coverage
- **Compliance**: Ready for SOC2/ISO27001

## Implementation Status

- [x] User registration and login
- [x] JWT authentication
- [x] API key management
- [x] Rate limiting
- [x] Comprehensive error handling
- [x] Detailed logging
- [x] Prometheus monitoring integration
- [x] Test cases

## Test Examples

### Unit Tests

Run tests with:
```bash
go test -v ./...
```

### Test Cases

1. **User Registration**: Test valid and invalid registrations
2. **User Login**: Test valid and invalid logins
3. **JWT Validation**: Test token validation middleware
4. **Rate Limiting**: Test brute force protection
5. **API Key Management**: Test API key creation and listing

## Monitoring

The service exposes Prometheus metrics at `/metrics` including:

- `auth_operations_total`: Count of authentication operations by type and status
- `http_request_duration_seconds`: HTTP request latency by method and path

## Contributing

1. Fork the repository
2. Create a new branch
3. Make your changes
4. Add appropriate tests
5. Submit a pull request

## License

This project is licensed under the MIT License.


