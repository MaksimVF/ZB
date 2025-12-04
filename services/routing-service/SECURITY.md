



# Routing Service Security Implementation

## Overview

This document describes the security features implemented in the routing service.

## 1. mTLS for gRPC Communication

The routing service uses mutual TLS (mTLS) to secure gRPC communication between services.

### Certificate Generation

Certificates are generated using the script `certs/generate_certs.sh`. This creates:

- CA certificate and key
- Server certificate and key
- Client certificate and key

### Configuration

The server requires the following certificates:
- `certs/server.crt` - Server certificate
- `certs/server.key` - Server private key
- `certs/ca.crt` - CA certificate for client validation

### Usage

Clients must present a valid client certificate signed by the CA to communicate with the gRPC server.

## 2. JWT Authentication for HTTP API

The HTTP API is protected with JWT authentication.

### Tokens

For development, the following tokens are configured:

- `Bearer admin-token` - Admin access
- `Bearer operator-token` - Operator access
- `Bearer viewer-token` - Viewer access

### Implementation

The `jwtMiddleware` validates the token and extracts user information.

## 3. Role-Based Access Control (RBAC)

The service implements RBAC with the following roles:

- **Admin**: Full access to all endpoints
- **Operator**: Access to operational endpoints
- **Viewer**: Read-only access

### Endpoint Protection

- `/api/routing/policy` (PUT) - Requires Admin role
- `/api/routing/heads` (POST) - Requires Operator role
- `/api/routing/*` (GET) - Requires Viewer role
- `/health` - No authentication required

## 4. Future Enhancements

- Implement proper JWT token validation
- Add token expiration and refresh
- Integrate with OAuth2/SSO providers
- Implement audit logging

## Certificate Management

To regenerate certificates:

```bash
cd certs
./generate_certs.sh
```

This will create new certificates in the `certs` directory.


