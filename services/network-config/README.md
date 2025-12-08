



# Network Config Service

This service manages dynamic network configuration for Head/Tail services.

## Features

- Centralized network configuration management
- Auto-reload configuration for Head/Tail services
- REST API for configuration updates
- Support for multiple network modes (direct, WireGuard, ZeroTier, Tailscale, etc.)
- Load balancing configuration
- Rate limiting configuration
- Tailscale integration for secure service-to-service communication

## Configuration Structure

```json
{
  "head_endpoint": "grpc://10.1.1.15:9000",
  "network_mode": "wireguard",
  "wg_peer_public": "...",
  "wg_allowed_ips": "10.10.0.0/24",
  "tailscale_auth_key": "tskey-xxx",
  "tailscale_hostname": "service-node-1",
  "tailscale_advertise_routes": "10.0.0.0/24",
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
- `GET /api/tailscale/status` - Get Tailscale status
- `POST /api/tailscale/configure` - Configure Tailscale with auth key, hostname, and routes
- `GET /health` - Health check

## Integration

Head and Tail services should periodically fetch the latest configuration from this service and apply changes without restarting.

## Tailscale Docker Integration

The service supports Tailscale Docker device integration for running Tailscale in containerized environments. When running in Docker, the service will automatically detect this and use the Tailscale Docker device for network management.

### Docker Setup

1. Install the Tailscale Docker device on your Docker host
2. Configure your containers to use the Tailscale network
3. Set the network mode to "tailscale" in the configuration

### Configuration Example

```json
{
  "head_endpoint": "grpc://10.1.1.15:9000",
  "network_mode": "tailscale",
  "tailscale_auth_key": "tskey-xxx",
  "tailscale_hostname": "service-node-1",
  "tailscale_advertise_routes": "10.0.0.0/24",
  "security_token": "xxxxxx"
}
```

