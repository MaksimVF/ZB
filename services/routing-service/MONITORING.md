




# Routing Service Monitoring

## Overview

This document describes the monitoring setup for the routing service using Prometheus and Grafana.

## Metrics Collected

### Routing Metrics

- `routing_decisions_total`: Total number of routing decisions made (labeled by strategy, model_type, region)
- `head_registrations_total`: Total number of head registrations
- `head_status_updates_total`: Total number of head status updates
- `active_heads`: Current number of active heads

### HTTP Metrics

- `http_requests_total`: Total number of HTTP requests (labeled by method, endpoint, status)
- `http_request_duration_seconds`: HTTP request latency distribution

### Cache Metrics

- `cache_hits_total`: Total number of cache hits
- `cache_misses_total`: Total number of cache misses

### External Service Metrics

- `external_service_calls_total`: Total number of external service calls (labeled by service, status)

### System Metrics

- `process_cpu_seconds_total`: CPU usage
- `process_resident_memory_bytes`: Memory usage

### Real-time Metrics

- `sse_connections`: Number of active SSE connections
- `websocket_connections`: Number of active WebSocket connections

### Message Queue Metrics

- `message_queue_messages_total`: Total number of message queue messages (labeled by queue, status)

## Prometheus Configuration

The Prometheus configuration is located in `prometheus/prometheus.yml`. It scrapes metrics from the routing service at `localhost:8080/metrics`.

## Grafana Dashboard

Two Grafana dashboard configurations are provided:

1. **Basic Dashboard**: `grafana/dashboards/routing-service.json`
   - Routing Decisions by Strategy
   - Active Heads
   - HTTP Requests
   - Head Operations
   - Cache Performance

2. **Enhanced Dashboard**: `grafana/dashboards/routing-service-enhanced.json`
   - All basic metrics plus:
   - HTTP Request Latency
   - HTTP Error Rates
   - Routing Errors
   - System CPU and Memory Usage
   - Real-time Connections
   - Message Queue Activity
   - Routing Decisions by Model Type and Region
   - External Service Errors and Success

## Audit Logging

The routing service includes audit logging for sensitive operations:

- Logs to file: `/var/log/routing_audit.log`
- Logs to Redis channel: `audit:routing:logs`

Sensitive operations include:
- Admin operations
- Policy changes
- Head management
- Routing configuration

## Setup Instructions

### Prometheus

1. Start Prometheus with the provided configuration:
   ```bash
   prometheus --config.file=prometheus/prometheus.yml
   ```

### Grafana

1. Import the dashboard JSON file into Grafana
2. Configure the Prometheus data source
3. Set the dashboard to use the Prometheus data source

## Alerting

The following alerts can be configured in Prometheus:

- **HighRoutingErrors**: Alert when routing decision errors exceed a threshold
- **LowActiveHeads**: Alert when the number of active heads falls below a threshold
- **HighHTTPErrors**: Alert when HTTP error rates exceed a threshold
- **HighLatency**: Alert when HTTP request latency exceeds thresholds
- **HighMemoryUsage**: Alert when memory usage is too high
- **HighCPUUsage**: Alert when CPU usage is too high

## Example Alert Configuration

```yaml
groups:
- name: routing-service-alerts
  rules:
  - alert: HighRoutingErrors
    expr: rate(routing_decisions_total{strategy="none"}[5m]) > 0.1
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "High routing errors"
      description: "Routing service is making too many failed routing decisions"

  - alert: LowActiveHeads
    expr: active_heads < 2
    for: 5m
    labels:
      severity: critical
    annotations:
      summary: "Low active heads"
      description: "Number of active heads is below minimum threshold"

  - alert: HighHTTPLatency
    expr: histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[5m])) by (le)) > 1
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "High HTTP latency"
      description: "95th percentile HTTP request latency is above 1 second"

  - alert: HighMemoryUsage
    expr: process_resident_memory_bytes > 1e9  # 1GB
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "High memory usage"
      description: "Memory usage exceeds 1GB"
```

## Monitoring Best Practices

1. **Regularly review metrics** to identify performance bottlenecks
2. **Set appropriate alert thresholds** based on your environment
3. **Monitor routing strategy effectiveness** to optimize performance
4. **Track HTTP error rates** to identify API issues
5. **Monitor system resources** (CPU, memory) to ensure service health
6. **Review audit logs** regularly for security and compliance
7. **Analyze latency patterns** to optimize performance



