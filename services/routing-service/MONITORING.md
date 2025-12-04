




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

## Prometheus Configuration

The Prometheus configuration is located in `prometheus/prometheus.yml`. It scrapes metrics from the routing service at `localhost:8080/metrics`.

## Grafana Dashboard

A Grafana dashboard configuration is provided in `grafana/dashboards/routing-service.json`. The dashboard includes:

1. **Routing Decisions by Strategy**: Shows the rate of routing decisions by strategy
2. **Active Heads**: Displays the current number of active heads
3. **HTTP Requests**: Shows the rate of HTTP requests by method and endpoint
4. **Head Operations**: Shows the rate of head registrations and status updates

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
```

## Monitoring Best Practices

1. **Regularly review metrics** to identify performance bottlenecks
2. **Set appropriate alert thresholds** based on your environment
3. **Monitor routing strategy effectiveness** to optimize performance
4. **Track HTTP error rates** to identify API issues



