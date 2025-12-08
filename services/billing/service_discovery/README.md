






# Service Discovery and Configuration Management

## Overview

This repository contains a comprehensive service discovery and configuration management solution for the entire system, including all services:

- **System Services:** head-go, tail-go, ui, auth, proxy, secret, rate-limiter
- **Billing Services:** billing-core, pricing-service, exchange-service, monitoring-service, admin-service

## Components

### 1. Consul

**Port:** 8500
**Responsibilities:**
- Service discovery
- Configuration management
- Health checking
- Service registration

### 2. Service Discovery

**File:** service_discovery.py
**Responsibilities:**
- Service registration
- Service discovery
- Service health checking
- Service deregistration

### 3. Configuration Management

**File:** config_manager.py
**Responsibilities:**
- Configuration storage
- Configuration retrieval
- Configuration updates
- Configuration watching

### 4. Service Registration

**File:** service_registration.py
**Responsibilities:**
- Service registration
- Service deregistration
- Service updates
- Service health checks

### 5. Health Checking

**File:** health_check.py
**Responsibilities:**
- Service health checking
- Service status checking
- Service availability checking
- Service health details

## Deployment

### Docker Compose

The service discovery and configuration management can be deployed using Docker Compose:

```bash
docker-compose -f consul-config.yml up --build
```

### Environment Variables

Each service requires the following environment variables:

- `REDIS_URL`: Redis connection URL (default: redis://redis:6379)
- `JWT_SECRET`: JWT secret key
- `ADMIN_KEY`: Admin API key
- `STRIPE_WEBHOOK_SECRET`: Stripe webhook secret (for Admin Service)
- `CONSUL_HTTP_ADDR`: Consul HTTP address (default: localhost:8500)

## Service Discovery

### 1. Service Registration

Each service registers itself with Consul on startup:

```python
from service_registration import service_registration

service_registration.register_service(
    "billing-core",
    "localhost",
    50052,
    ["billing", "core"],
    "/health"
)
```

### 2. Service Discovery

Services can discover each other using Consul:

```python
from service_discovery import get_service_discovery

sd = get_service_discovery()
billing_service = sd.get_service("billing-core")
```

### 3. Configuration Management

Configuration is managed through Consul:

```python
from config_manager import get_config_manager

cm = get_config_manager()
max_balance = cm.get_config("billing/max_balance", 500)
```

### 4. Health Checking

Health checks are performed using Consul:

```python
from health_check import health_check

health = health_check.check_service_health("billing-core")
```

## Benefits

- **Service Discovery:** Dynamic service registration and discovery
- **Configuration Management:** Centralized configuration management
- **Health Checking:** Proactive health checking
- **Scalability:** Independent scaling of services
- **Resilience:** Automatic service registration and deregistration
- **Flexibility:** Easy service updates and configuration changes

## Recommendations

1. **Add Tracing:** Implement OpenTelemetry tracing in all services
2. **Add Metrics:** Add Prometheus metrics to all services
3. **Add Logging:** Implement structured logging in all services
4. **Add Alerts:** Configure alerts for critical metrics
5. **Add Dashboards:** Create dashboards for each service

## Implementation

Each service should include:

1. **Metrics:**
   - HTTP request rate
   - HTTP request duration
   - Error rate
   - Service health
   - Resource usage (CPU, memory)

2. **Logging:**
   - Structured JSON logging
   - Log level configuration
   - Log rotation

3. **Tracing:**
   - OpenTelemetry instrumentation
   - Trace context propagation
   - Span creation for critical operations

4. **Alerts:**
   - Service-specific alerts
   - Resource usage alerts
   - Error rate alerts

The service discovery and configuration management provides a comprehensive solution for service registration, discovery, and configuration management.












