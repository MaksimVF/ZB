


# Routing Server Deployment

This repository contains the configuration for deploying a routing server that includes multiple services:

- **routing-service**: Main routing service for managing request distribution
- **secrets-service**: Secure storage and management of API keys and secrets
- **network-config-admin**: Network configuration management UI
- **telegram-bot**: Telegram bot for notifications and integrations
- **monitoring**: Optional monitoring tools (Prometheus, Grafana)

## Prerequisites

- Docker installed
- Docker Compose installed
- Environment variables configured (Telegram_Token, ADMIN_CHAT_ID)

## Deployment Steps

### 1. Build the Docker image

```bash
docker compose -f docker-compose.routing-server.yml build
```

### 2. Start the services

```bash
docker compose -f docker-compose.routing-server.yml up -d
```

### 3. Verify services are running

```bash
docker ps -a
```

## Configuration

### Environment Variables

Create a `.env` file with the following variables:

```
Telegram_Token=your-telegram-bot-token
ADMIN_CHAT_ID=your-admin-chat-id
```

### Ports

- **8083**: Routing service HTTP
- **50053**: Secrets service gRPC
- **8082**: Network config admin
- **3000**: Telegram bot
- **3003**: Telegram bot (alternative)
- **9090**: Prometheus (if included)

## Monitoring

Access the monitoring tools:

- **Prometheus**: http://localhost:9090
- **Grafana**: http://localhost:3002 (if included)

## Maintenance

### Update services

```bash
docker compose -f docker-compose.routing-server.yml pull
docker compose -f docker-compose.routing-server.yml up -d
```

### View logs

```bash
docker logs <container_name>
```

### Stop services

```bash
docker compose -f docker-compose.routing-server.yml down
```

## Troubleshooting

1. **Service fails to start**: Check logs with `docker logs <container>`
2. **Network issues**: Verify docker network with `docker network inspect server-network`
3. **Resource limits**: Adjust in `docker-compose.routing-server.yml` if needed

## Contact

For support, contact the DevOps team at devops@example.com


