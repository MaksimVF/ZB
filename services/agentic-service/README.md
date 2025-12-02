


# Agentic Service

## Overview

**Purpose**: The Agentic Service provides specialized agentic functionality with advanced tool calling, parallel processing, and caching capabilities. It serves as the dedicated service for agent-related operations in our architecture.

**Role in Architecture**: The Agentic Service handles all agent-related processing, providing enhanced capabilities beyond standard LLM APIs. It integrates with premium models and offers specialized features for agent frameworks.

## Features

- **Advanced Agentic Endpoint**: Specialized endpoint for agent operations
- **Parallel Tool Calls**: Support for parallel execution of tool calls
- **Tool Result Caching**: Performance optimization through caching
- **Premium Model Access**: Access to top-tier reasoning models
- **Specialized Billing**: Custom pricing and monitoring for agentic operations
- **Secure Communication**: gRPC with mTLS for inter-service communication

## Main Endpoint

- `POST /v1/agentic` - Advanced agentic processing endpoint

## Architecture

The Agentic Service provides specialized agentic capabilities:

```
[Gateway Service] → [Agentic Service] → [LLM Providers]
       ↑
[Billing Service] ←→ [Monitoring]
```

## Key Components

1. **HTTP Server**: Handles incoming agentic API requests
2. **gRPC Clients**: Secure communication with secret-service
3. **Redis Client**: Caching and rate limiting
4. **Middleware**: Rate limiting and authentication
5. **Handlers**: Business logic for agentic processing

## Relationship to Other Services

- **Gateway Service**: Routes agentic requests to this service
- **Tail Service**: Handles standard API requests
- **Billing Service**: Provides specialized billing for agentic operations
- **Secret Service**: Securely manages API keys and secrets

## Deployment

```bash
docker build -t agentic-service .
docker run -p 8081:8081 agentic-service
```

## Environment Variables

- `REDIS_ADDR`: Redis server address (default: redis:6379)
- `SECRET_SERVICE_ADDR`: Secret service address (default: secret-service:50053)

## Benefits

- **Specialized Processing**: Dedicated service for agentic operations
- **Performance Optimization**: Caching and parallel processing
- **Premium Features**: Access to top-tier models
- **Custom Billing**: Specialized pricing for agentic usage
- **Scalable Architecture**: Independent service for better scalability


