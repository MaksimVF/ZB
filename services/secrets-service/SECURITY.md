
# Secret Service Security Guide

## Overview

This document outlines the security measures implemented in the Secret Service to protect sensitive data and prevent unauthorized access.

## Security Improvements

### 1. Admin Key Management

**Before**: Admin keys were hardcoded in docker-compose files.

**After**: Admin keys are now configured via environment variables:

- `ADMIN_KEY` - Main admin key for the secret service
- `UI_ADMIN_KEY` - Admin key for the UI service

**Configuration**:
```bash
# Set environment variables before starting the service
export ADMIN_KEY="your-strong-admin-key-here"
export UI_ADMIN_KEY="your-ui-admin-key-here"
```

### 2. CORS Security

**Before**: CORS allowed all origins (`*`).

**After**: CORS now only allows specific origins configured via environment variable:

- `ALLOWED_ORIGINS` - Comma-separated list of allowed origins
- Default: `http://localhost:3000,http://localhost:3001`

**Configuration**:
```bash
export ALLOWED_ORIGINS="http://yourdomain.com,http://yourotherdomain.com"
```

### 3. Enhanced Authentication

**Before**: Simple admin key check only.

**After**: Multi-layered authentication:
- Rate limiting (1 request per 5 seconds per IP)
- Admin key format validation (16-64 chars, alphanumeric with dashes/underscores)
- IP-based tracking for security monitoring

### 4. Input Validation

**Before**: Basic input validation only.

**After**: Comprehensive input validation:
- Path format validation (alphanumeric, dashes, underscores, slashes only)
- Value length validation (max 4096 characters)
- Secret name format validation
- Required field validation

## Security Best Practices

### Admin Key Requirements

Admin keys should:
- Be at least 16 characters long (recommended: 32+ characters)
- Contain only alphanumeric characters, dashes, and underscores
- Be stored securely (not in version control)
- Be rotated regularly

### CORS Configuration

For production environments:
- Set `ALLOWED_ORIGINS` to only the domains that need access
- Avoid using wildcards or allowing all origins
- Include only trusted domains

### Monitoring and Logging

The service includes:
- Detailed request logging
- Error tracking
- Prometheus metrics for monitoring
- Rate limiting logs

## Deployment Recommendations

1. **Use environment variables**: Always configure sensitive values via environment variables
2. **Enable TLS**: Use the built-in mTLS support for gRPC communications
3. **Monitor logs**: Regularly review logs for suspicious activity
4. **Rotate keys**: Periodically rotate admin keys and vault tokens
5. **Limit access**: Restrict network access to the secret service

## Security Checklist

- [ ] Configure strong admin keys via environment variables
- [ ] Set appropriate CORS origins
- [ ] Enable monitoring and alerting
- [ ] Configure proper TLS certificates
- [ ] Implement network-level security (firewalls, etc.)
- [ ] Regularly review and rotate credentials
