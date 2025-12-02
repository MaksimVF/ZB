

# Tail Service (Main Service)

## Overview

**Purpose**: The Tail Service is the core working service that handles the main business logic and API functionality. It serves as the primary backend service in our architecture.

**Role in Architecture**: The Tail Service handles the core processing of requests, including chat completions, batch processing, embeddings, and agentic functionality. It communicates with other services like auth-service and secret-service for secure operations.

## Features

- **OpenAI-compatible API**: Full compatibility with OpenAI API standards
- **Batch Processing**: Efficient handling of batch requests
- **Embeddings Support**: Vector embedding generation
- **Agentic Functionality**: Advanced agent-related processing
- **Rate Limiting**: Built-in rate limiting middleware
- **Secure Communication**: gRPC with mTLS for inter-service communication
- **Health Monitoring**: Redis and gRPC health checks

## Main Endpoints

- `POST /v1/chat/completions` - Chat completion API
- `POST /v1/completions` - Completion API
- `POST /v1/batch` - Batch processing
- `POST /v1/embeddings` - Embeddings API
- `POST /v1/agentic` - Agentic functionality
- `GET /health` - Health check

## Architecture

The Tail Service sits at the core of our system, handling the main business logic while delegating specialized tasks to other services:

```
[Gateway Service] → [Tail Service] → [Model Proxy] → [LLM Providers]
       ↑               ↓       ↑
[Auth Service] ←→ [Secrets Service]
```

## Key Components

1. **HTTP Server**: Handles incoming API requests
2. **gRPC Clients**: Secure communication with auth-service and secret-service
3. **Redis Client**: Caching and rate limiting
4. **Middleware**: Rate limiting and authentication
5. **Handlers**: Business logic for different API endpoints

## Relationship to Gateway Service

While the Gateway Service focuses on agent-related API gateway functionality and provider management, the Tail Service handles the core business logic and processing. The Gateway Service may route requests to the Tail Service for processing.

