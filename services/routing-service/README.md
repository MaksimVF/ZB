

# Routing Service

The Routing Service provides dynamic routing capabilities for the ZB network architecture. It handles head service registration, status monitoring, and routing decision making based on configurable policies.

## Features

- Head service registration and status tracking
- Dynamic routing decisions based on multiple strategies
- Configurable routing policies
- REST API for administration
- gRPC API for service-to-service communication
- Redis-backed data storage

## Architecture

The Routing Service consists of:

1. **gRPC Server**: Handles service-to-service communication
2. **HTTP Server**: Provides REST API for administration
3. **Redis Backend**: Stores head registrations and routing policies
4. **Routing Strategies**: Multiple configurable routing algorithms

## API

### gRPC Methods

- `RegisterHead`: Register a new head service
- `UpdateHeadStatus`: Update status and load information
- `GetRoutingDecision`: Get routing decision for a request
- `GetAllHeads`: Get information about all heads
- `UpdateRoutingPolicy`: Update routing policy
- `GetRoutingPolicy`: Get current routing policy

### REST Endpoints

- `GET /api/routing/policy`: Get current routing policy
- `PUT /api/routing/policy`: Update routing policy
- `GET /api/routing/heads`: Get all head services
- `GET /health`: Health check

## Configuration

The service uses Redis for persistent storage. Configuration is done via the REST API or by directly modifying Redis keys.

## Building

```bash
docker build -t routing-service .
```

## Running

```bash
docker run -p 50061:50061 -p 8080:8080 --network=zb_network routing-service
```

## Dependencies

- Redis
- gRPC
- HTTP server (for admin interface)

