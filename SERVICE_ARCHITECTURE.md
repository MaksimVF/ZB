
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

## Supporting Services

### 3. Auth Service (`services/auth-service`)
- **Purpose**: Authentication and authorization
- **Key Features**: User management, token validation, access control

### 4. Billing Service (`services/billing`)
- **Purpose**: Billing and usage tracking
- **Key Features**: Usage monitoring, invoicing, payment processing

### 5. Secrets Service (`services/secrets-service`)
- **Purpose**: Secure secret management
- **Key Features**: API key storage, secret rotation, secure access

### 6. Model Proxy (`services/model-proxy`)
- **Purpose**: Model communication proxy
- **Key Features**: Request routing, load balancing, model abstraction

### 7. Rate Limiter (`services/rate-limiter`)
- **Purpose**: Request rate limiting
- **Key Features**: Traffic control, abuse prevention, fair usage

## Service Relationships

```
[External Clients] → [Gateway Service] → [Tail Service] → [Model Proxy] → [LLM Providers]
                    ↓       ↑
              [Auth Service] ←→ [Secrets Service]
                    ↓       ↑
                [Billing Service]
```

## Key Differences

| Aspect          | Gateway Service | Tail Service |
|----------------|----------------|--------------|
| **Focus**      | Agent/LLM API Gateway | Core business logic |
| **Endpoints**  | LangChain-specific, provider management | Chat, batch, embeddings, agentic |
| **Integration** | LiteLLM, multi-provider routing | Secure service communication |
| **Monitoring** | Prometheus metrics, health checks | Redis health checks, gRPC monitoring |

## Naming Convention

To avoid confusion:
- **Gateway Service**: Always referred to as "gateway" or "agent gateway"
- **Tail Service**: Always referred to as "tail" or "main service"
- **Configuration**: Use service names consistently in all config files
