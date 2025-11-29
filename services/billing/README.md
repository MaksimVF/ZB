


# Billing Microservice Architecture

## Overview

This repository contains a microservice architecture for the billing system, consisting of five independent services:

1. **Billing Core Service** - Handles core billing operations
2. **Pricing Service** - Manages pricing models and calculations
3. **Exchange Rate Service** - Manages exchange rates and currency conversion
4. **Monitoring Service** - Tracks metrics and generates alerts
5. **Admin Service** - Provides admin interfaces and external integrations

## Services

### 1. Billing Core Service

**Port:** 50052
**Protocol:** gRPC
**Responsibilities:**
- Core billing operations (Charge, Reserve, Commit)
- User balance management
- Transaction handling

### 2. Pricing Service

**Port:** 50053
**Protocol:** gRPC
**Responsibilities:**
- Pricing calculations
- External pricing integration
- Pricing management

### 3. Exchange Rate Service

**Port:** 50054
**Protocol:** gRPC
**Responsibilities:**
- Currency management
- Exchange rate updates
- Currency conversion

### 4. Monitoring Service

**Port:** 50055
**Protocol:** gRPC
**Responsibilities:**
- Metrics tracking
- Alert generation
- Monitoring endpoints

### 5. Admin Service

**Ports:** 50056 (gRPC), 50057 (HTTP)
**Protocols:** gRPC, HTTP
**Responsibilities:**
- Admin endpoints
- External integrations (Stripe)
- Admin operations

## Deployment

### Docker Compose

The services can be deployed using Docker Compose:

```bash
docker-compose up --build
```

### Environment Variables

Each service requires the following environment variables:

- `REDIS_URL`: Redis connection URL (default: redis://redis:6379)
- `JWT_SECRET`: JWT secret key
- `ADMIN_KEY`: Admin API key
- `STRIPE_WEBHOOK_SECRET`: Stripe webhook secret (for Admin Service)

## Communication

- **gRPC** is used for internal service-to-service communication
- **HTTP** is used for external APIs and admin interfaces
- **Redis** is used for shared state and caching

## Benefits

- **Scalability:** Independent scaling of services
- **Maintainability:** Clear separation of concerns
- **Resilience:** Fault isolation between services
- **Deployment:** Independent deployment of services

## Development

Each service can be developed and tested independently. The services communicate through well-defined gRPC interfaces.

