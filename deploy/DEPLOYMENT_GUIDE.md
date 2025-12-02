

# Deployment Guide for Microservices Architecture

## 1. Introduction
This guide provides step-by-step instructions for deploying the microservices architecture across multiple servers.

## 2. Prerequisites

### 2.1 Server Requirements
- **Head Server**: 8 CPU, 16GB RAM, 100GB disk
- **Tail Server**: 16 CPU, 32GB RAM, 200GB disk
- **UI Server**: 4 CPU, 8GB RAM, 50GB disk

### 2.2 Software Requirements
- Docker 20.10+
- Ansible 2.9+
- Kubernetes 1.20+ (optional for container orchestration)

### 2.3 Network Requirements
- Ports 80, 443, 50051-50055 open between servers
- Proper firewall configuration for inter-service communication

## 3. Deployment Steps

### 3.1 Prepare Inventory File
Edit `inventory.ini` with actual server IPs:
```ini
[head]
head1 ansible_host=10.0.1.10
head2 ansible_host=10.0.1.11

[tail]
tail1 ansible_host=10.0.2.20
tail2 ansible_host=10.0.2.21

[ui]
ui1 ansible_host=10.0.3.30
ui2 ansible_host=10.0.3.31
```

Note: The batch processing functionality is now integrated directly into the head-go and model-proxy services, eliminating the need for a separate batch-processor service.

### 3.2 Configure Environment
Edit `environments/production.yml` with production settings:
```yaml
env: production
debug: false
max_connections: 1000
rate_limit: 500
```

### 3.3 Run Deployment
Execute the deployment script:
```bash
./deploy.sh production
```

### 3.4 Verify Deployment
Check service status:
```bash
ansible -i inventory.ini all -m ping
ansible -i inventory.ini all -a "systemctl status head-go"
```

## 4. Monitoring Setup

### 4.1 Install Monitoring Tools
```bash
ansible-playbook -i inventory.ini monitoring.yml
```

### 4.2 Access Monitoring Dashboard
- Prometheus: http://monitoring.example.com:9090
- Grafana: http://monitoring.example.com:3000

## 5. Troubleshooting

### 5.1 Common Issues
- **Service not starting**: Check logs with `journalctl -u head-go`
- **Connection refused**: Verify firewall settings and port availability
- **Certificate errors**: Check TLS configuration in `config.yml`

### 5.2 Rollback Procedure
```bash
ansible-playbook -i inventory.ini rollback.yml
```

## 6. Maintenance

### 6.1 Regular Tasks
- Certificate renewal (monthly)
- Dependency updates (quarterly)
- Security audits (quarterly)

### 6.2 Backup Strategy
- Daily database backups
- Weekly full system snapshots
- Offsite backup storage

## 7. Contact Information
- **Support**: support@example.com
- **Emergency**: +1-800-123-4567


