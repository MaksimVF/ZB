



# Observability Stack

## Overview

This repository contains a comprehensive observability stack for the entire system, including all services:

- **Prometheus** for metrics collection
- **Grafana** for visualization
- **Loki** for log aggregation
- **Jaeger** for distributed tracing

## Services Covered

1. **System Services:**
   - head-go
   - tail-go
   - ui
   - auth
   - proxy
   - secret
   - rate-limiter

2. **Billing Services:**
   - billing-core
   - pricing-service
   - exchange-service
   - monitoring-service
   - admin-service

## Components

### 1. Prometheus

**Port:** 9090
**Responsibilities:**
- Metrics collection from all services
- Alerting based on metrics
- Time-series database

### 2. Grafana

**Port:** 3000
**Responsibilities:**
- Visualization of metrics
- Dashboards for system and billing services
- Alert management

### 3. Loki

**Port:** 3100
**Responsibilities:**
- Log aggregation
- Log storage
- Log querying

### 4. Promtail

**Responsibilities:**
- Log collection from all services
- Log forwarding to Loki

### 5. Jaeger

**Ports:**
- 16686 (UI)
- 4317 (OTLP gRPC)
- 4318 (OTLP HTTP)
**Responsibilities:**
- Distributed tracing
- Trace visualization
- Performance analysis

## Deployment

### Docker Compose

The observability stack can be deployed using Docker Compose:

```bash
docker-compose -f observability-stack.yml up --build
```

### Environment Variables

Each service requires the following environment variables:

- `REDIS_URL`: Redis connection URL (default: redis://redis:6379)
- `JWT_SECRET`: JWT secret key
- `ADMIN_KEY`: Admin API key
- `STRIPE_WEBHOOK_SECRET`: Stripe webhook secret (for Admin Service)

## Dashboards

### 1. System Overview

- Service availability
- Request rate
- CPU usage
- Memory usage
- Average request duration
- Error rate

### 2. Billing Overview

- Request rate by service
- Average request duration
- Error rate by service
- Tokens used by user
- User balances
- Service health

### 3. Service Health

- Service availability
- Error rate
- CPU usage
- Memory usage
- 95th percentile latency

### 4. Usage Metrics

- Tokens used by user
- Tokens used by model
- Token usage rate
- User balances
- Charge requests
- Reserve requests

### 5. Error Rates

- Errors by service
- Errors by type
- Error rate
- Errors per hour
- Errors per day
- Errors per week

### 6. Log Analysis

- Billing errors
- System errors
- Billing warnings
- System warnings
- Billing info logs
- System info logs

## Alerts

### System Alerts

- **ServiceDown:** Service has been down for more than 2 minutes
- **HighErrorRate:** Error rate is above 5% for the last 5 minutes
- **HighLatency:** 95th percentile latency is above 1 second
- **HighMemoryUsage:** Memory usage is above 500MB
- **HighCPUUsage:** CPU usage is above 80%

### Billing Alerts

- **LowBalance:** User balance is below $10
- **HighTokenUsage:** User has used more than 1M tokens in the last hour
- **ReservationTimeout:** Reservation has been active for more than 1 hour
- **ExchangeRateStale:** Exchange rates haven't been updated in the last 24 hours

## Benefits

- **Unified Monitoring:** Consistent metrics collection across all services
- **Comprehensive Alerting:** Proactive issue detection
- **Centralized Logging:** Easy log analysis and troubleshooting
- **Distributed Tracing:** Performance analysis and debugging
- **Scalability:** Independent scaling of observability components

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

The observability stack provides a comprehensive solution for monitoring, logging, and tracing all services in the system.









