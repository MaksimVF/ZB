
# Service Architecture Overview

## Core Services

### 1. Gateway Service (`services/gateway`)
**Purpose**: Agent-focused LLM API Gateway

- **Primary Function**: Provides a unified gateway for LLM APIs with special support for LangChain integration
- **Key Features**:
  - OpenAI-compatible API
  - LangChain-specific endpoint with usage tracking
  - Multi-provider support (OpenAI, Anthropic, Google, Meta, etc.)
  - Comprehensive monitoring and health checks
  - LiteLLM integration for dynamic provider routing
- **Main Endpoints**:
  - `/v1/langchain/chat/completions` - LangChain-specific endpoint
  - `/v1/chat/completions` - Standard OpenAI-compatible endpoint
  - `/v1/providers` - Provider management API

### 2. Tail Service (`services/tail-go`)
**Purpose**: Main working service with core functionality

- **Primary Function**: Core business logic and API handling
- **Key Features**:
  - OpenAI-compatible API endpoints
  - Batch processing
  - Embeddings support
  - Agentic functionality
  - Rate limiting middleware
  - Secure communication with other services
- **Main Endpoints**:
  - `/v1/chat/completions` - Chat completion API
  - `/v1/completions` - Completion API
  - `/v1/batch` - Batch processing
  - `/v1/embeddings` - Embeddings API
  - `/v1/agentic` - Agentic functionality

## Core Services

### 3. Agentic Service (`services/agentic-service`)
- **Purpose**: Dedicated service for advanced agentic operations
- **Key Features**:
  - Advanced agentic endpoint with parallel tool calls
  - Tool result caching for performance optimization
  - Premium model access for agentic operations
  - Specialized billing and monitoring capabilities
  - Secure communication with other services

## Supporting Services

### 4. Auth Service (`services/auth-service`)
- **Purpose**: Authentication and authorization
- **Key Features**: User management, token validation, access control

### 5. Billing Service (`services/billing`)
- **Purpose**: Billing and usage tracking
- **Key Features**: Usage monitoring, invoicing, payment processing

### 6. Secrets Service (`services/secrets-service`)
- **Purpose**: Secure secret management
- **Key Features**: API key storage, secret rotation, secure access

### 7. Model Proxy (`services/model-proxy`)
- **Purpose**: Model communication proxy
- **Key Features**: Request routing, load balancing, model abstraction

### 8. Rate Limiter (`services/rate-limiter`)
- **Purpose**: Request rate limiting
- **Key Features**: Traffic control, abuse prevention, fair usage

## Service Relationships

```
[External Clients] → [Gateway Service] → [Head Service] → [Model Proxy] → [LLM Providers]
                    ↓       ↑
              [Auth Service] ←→ [Secrets Service]
                    ↓       ↑
                [Billing Service]

[External Clients] → [Agentic Service] → [Head Service] → [Model Proxy] → [LLM Providers]
                    ↓       ↑
                [Billing Service]
```

## Key Differences

| Aspect          | Gateway Service | Tail Service | Agentic Service |
|----------------|----------------|--------------|-----------------|
| **Focus**      | Agent/LLM API Gateway | Core business logic | Advanced agentic operations |
| **Endpoints**  | LangChain-specific, provider management, agentic proxy | Chat, batch, embeddings | Advanced agentic processing |
| **Integration** | LiteLLM, multi-provider routing | Secure service communication | Premium models, tool caching |
| **Monitoring** | Prometheus metrics, health checks | Redis health checks, gRPC monitoring | Specialized billing, performance metrics |

## Naming Convention

To avoid confusion:
- **Gateway Service**: Always referred to as "gateway" or "agent gateway"
- **Tail Service**: Always referred to as "tail" or "main service"
- **Configuration**: Use service names consistently in all config files
