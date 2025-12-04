






# Routing Service External Capabilities

## Overview

This document describes the external capabilities of the routing service, including webhooks, external service integration, and event-driven architecture.

## 1. Webhooks

### Head Status Webhook

**Endpoint**: `/webhook/head-status`

**Method**: POST

**Payload**:
```json
{
  "head_id": "string",
  "status": "string",
  "current_load": "integer",
  "timestamp": "integer"
}
```

**Response**:
- 200 OK: Webhook processed successfully
- 400 Bad Request: Invalid webhook payload
- 500 Internal Server Error: Failed to process webhook

### Routing Decision Webhook

**Endpoint**: `/webhook/routing-decision`

**Method**: POST

**Payload**:
```json
{
  "model_type": "string",
  "region_preference": "string",
  "routing_strategy": "string",
  "metadata": {
    "key": "value"
  }
}
```

**Response**:
```json
{
  "head_id": "string",
  "endpoint": "string",
  "strategy_used": "string",
  "reason": "string",
  "metadata": {
    "key": "value"
  }
}
```

## 2. External Service Integration

### External Service Client

The service includes an HTTP client for calling external services:

```go
func callExternalService(serviceName, endpoint string, payload interface{}) ([]byte, error)
```

### Metrics

- `external_service_calls_total`: Tracks external service calls by service and status

## 3. Event-Driven Architecture

### Head Status Events

When a head status changes, the system:
1. Updates the head status in Redis
2. Invalidates any cache entries referencing the head
3. Records metrics for the status update

### Routing Decision Events

When a routing decision is made:
1. Checks cache for existing decision
2. Applies routing strategy if cache miss
3. Updates cache with new decision
4. Records metrics for the decision

## 4. Third-Party Integration

### Integration Patterns

1. **Webhook Listeners**: Accept incoming webhooks for head status updates
2. **External API Calls**: Make outbound calls to external services
3. **Event Processing**: Process events from external systems

### Security Considerations

- Webhooks should be authenticated using JWT or API keys
- External service calls should use HTTPS
- Rate limiting should be implemented for external integrations

## 5. Future Enhancements

### Planned Features

1. **Message Queue Integration**: Add support for Kafka, RabbitMQ, or NATS
2. **Server-Sent Events (SSE)**: Real-time updates for clients
3. **GraphQL API**: Alternative to REST for complex queries
4. **WebSocket Support**: Bi-directional communication

### Scalability Considerations

- Implement connection pooling for external services
- Add circuit breakers for external service calls
- Implement retry logic with exponential backoff

## Best Practices

1. **Monitor external service calls** to detect failures and performance issues
2. **Validate all webhook payloads** to ensure data integrity
3. **Implement idempotency** for webhook processing
4. **Use async processing** for non-critical external integrations





