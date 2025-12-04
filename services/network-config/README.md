



# Network Config Service

This service manages dynamic network configuration for Head/Tail services.

## Features

- Centralized network configuration management
- Auto-reload configuration for Head/Tail services
- REST API for configuration updates
- Support for multiple network modes (direct, WireGuard, ZeroTier, etc.)
- Load balancing configuration
- Rate limiting configuration

## Configuration Structure

```json
{
  "head_endpoint": "grpc://10.1.1.15:9000",
  "network_mode": "wireguard",
  "wg_peer_public": "...",
  "wg_allowed_ips": "10.10.0.0/24",
  "security_token": "xxxxxx",
  "retry_policy": {
    "retries": 3,
    "backoff_ms": 200
  },
  "rate_limits": {
    "max_requests_per_user": 100,
    "max_requests_per_ip": 1000,
    "window_seconds": 60
  },
  "load_balancing": {
    "mode": "single",
    "head_endpoints": ["grpc://head1:50055", "grpc://head2:50055"]
  }
}
```

## API Endpoints

- `GET /api/config` - Get current configuration
- `PUT /api/config` - Update configuration
- `GET /api/config/history` - Get configuration history
- `GET /health` - Health check

## Integration

Head and Tail services should periodically fetch the latest configuration from this service and apply changes without restarting.

