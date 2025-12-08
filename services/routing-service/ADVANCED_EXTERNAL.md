







# Advanced External Capabilities

## Overview

This document describes the advanced external capabilities of the routing service, including message queues, Server-Sent Events (SSE), WebSockets, and GraphQL.

## 1. Message Queue Integration

### NATS Messaging

The service integrates with NATS for asynchronous messaging and event-driven architecture.

#### Topics

- `head.status.update`: For head status updates
- `routing.decision.request`: For routing decision requests
- `routing.decision.response`: For routing decision responses
- `head.registration.request`: For head registration requests

#### Metrics

- `message_queue_messages_total`: Tracks message queue messages by queue and status

## 2. Server-Sent Events (SSE)

### Implementation

SSE provides real-time updates to clients for:
- Head status changes
- Routing decision updates
- System health monitoring

### Endpoints

- `/events/head-status`: Stream head status updates
- `/events/routing-decisions`: Stream routing decision updates

## 3. WebSocket Support

### Implementation

WebSockets provide bi-directional communication for:
- Real-time head management
- Interactive routing decisions
- System monitoring

### Endpoints

- `/ws/head-management`: WebSocket for head management
- `/ws/routing-decisions`: WebSocket for routing decisions

## 4. GraphQL API

### Implementation

GraphQL provides flexible querying capabilities for:
- Complex routing queries
- Head information retrieval
- System status monitoring

### Schema

```graphql
type Head {
  id: ID!
  endpoint: String!
  modelType: String!
  region: String!
  status: String!
  currentLoad: Int!
  lastHeartbeat: String!
}

type RoutingDecision {
  headId: String!
  endpoint: String!
  strategyUsed: String!
  reason: String!
  metadata: JSON
}

type Query {
  heads: [Head!]!
  head(id: ID!): Head
  routingDecision(modelType: String!, regionPreference: String, strategy: String): RoutingDecision!
}

type Mutation {
  registerHead(input: RegisterHeadInput!): Head!
  updateHeadStatus(id: ID!, status: String!, currentLoad: Int!): Head!
}

input RegisterHeadInput {
  endpoint: String!
  modelType: String!
  region: String!
  status: String!
  metadata: JSON
}
```

## 5. Event-Driven Architecture

### Event Types

1. **HeadStatusChanged**: Triggered when a head status changes
2. **RoutingDecisionMade**: Triggered when a routing decision is made
3. **HeadRegistered**: Triggered when a new head is registered
4. **HeadDeregistered**: Triggered when a head is removed

### Event Processing

Events are processed asynchronously and can trigger:
- Cache invalidation
- Alerts and notifications
- External service calls

## 6. Integration Patterns

### Webhook Patterns

1. **Inbound Webhooks**: Accept external system events
2. **Outbound Webhooks**: Notify external systems of changes
3. **Webhook Retries**: Implement retry logic for failed webhooks

### Message Queue Patterns

1. **Pub/Sub**: Publish and subscribe to events
2. **Request/Response**: Asynchronous request/response patterns
3. **Event Sourcing**: Capture all changes as events

### API Patterns

1. **REST**: Traditional REST API for CRUD operations
2. **GraphQL**: Flexible querying for complex data needs
3. **gRPC**: High-performance RPC for internal services

## 7. Security Considerations

### Authentication

- Webhooks: JWT or API key authentication
- WebSockets: JWT authentication on connection
- GraphQL: JWT or OAuth2 authentication

### Authorization

- Role-Based Access Control (RBAC) for all endpoints
- Fine-grained permissions for sensitive operations

### Encryption

- TLS for all external communications
- Message encryption for sensitive data

## 8. Future Enhancements

### Planned Features

1. **Kafka Integration**: Alternative to NATS for high-throughput messaging
2. **RabbitMQ Integration**: For complex routing scenarios
3. **gRPC-Web**: Browser-compatible gRPC for web clients
4. **Event Store**: Persistent event storage for audit and replay

### Scalability Considerations

- Implement connection pooling for message queues
- Add circuit breakers for external service calls
- Implement rate limiting for external integrations
- Add message compression for high-volume scenarios






