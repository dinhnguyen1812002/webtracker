# Deployment Guide

This guide covers different deployment options for the Uptime Monitoring & Alert System.

## Quick Start with Docker Compose

### Prerequisites

- Docker and Docker Compose installed
- At least 512MB RAM available
- Ports 8080, 5432, and 6379 available (or configure different ports)

### 1. Clone and Setup

```bash
git clone <repository-url>
cd uptime-monitor
cp .env.example .env
```

### 2. Configure Environment

Edit `.env` file with your settings:

```bash
# Required: Set a secure PostgreSQL password
POSTGRES_PASSWORD=your_secure_password

# Optional: Configure alert channels
TELEGRAM_BOT_TOKEN=your_bot_token
TELEGRAM_CHAT_ID=your_chat_id
SMTP_HOST=smtp.gmail.com
SMTP_USERNAME=your_email@gmail.com
SMTP_PASSWORD=your_app_password
```

### 3. Start Services

```bash
# Development mode
docker-compose up -d

# Production mode (optimized resource limits)
docker-compose -f docker-compose.yml -f docker-compose.prod.yml up -d
```

### 4. Verify Deployment

```bash
# Check service status
docker-compose ps

# View logs
docker-compose logs uptime-monitor-app

# Access the application
curl http://localhost:8080/health
```

## Manual Deployment

### Prerequisites

- Go 1.25+ installed
- PostgreSQL 12+ running
- Redis/Valkey running (optional but recommended)

### 1. Build Application

```bash
# Install templ for template compilation
go install github.com/a-h/templ/cmd/templ@latest

# Generate templates and build
make build
```

### 2. Database Setup

```bash
# Create database
createdb uptime_monitor

# Run migrations
make migrate-up
```

### 3. Configuration

Create a configuration file:

```yaml
# config.yml
server:
  port: 8080
  host: "0.0.0.0"

database:
  host: "localhost"
  port: 5432
  database: "uptime_monitor"
  user: "postgres"
  password: "your_password"
  ssl_mode: "prefer"

redis:
  addr: "localhost:6379"
  enabled: true

alert:
  telegram:
    bot_token: "your_bot_token"
    chat_id: "your_chat_id"
    enabled: true
  email:
    smtp_host: "smtp.gmail.com"
    smtp_port: 587
    username: "your_email@gmail.com"
    password: "your_app_password"
    from_address: "your_email@gmail.com"
    enabled: true
```

### 4. Run Application

```bash
./main -config config.yml
```

## Production Deployment

### Resource Requirements

- **Minimum**: 128MB RAM, 1 CPU core, 1GB disk
- **Recommended**: 256MB RAM, 1 CPU core, 5GB disk
- **For 100+ monitors**: 512MB RAM, 2 CPU cores, 10GB disk

### Security Considerations

1. **Database Security**:
   - Use strong passwords
   - Enable SSL/TLS connections
   - Restrict network access
   - Regular backups

2. **Application Security**:
   - Run as non-root user
   - Use environment variables for secrets
   - Enable HTTPS with reverse proxy
   - Regular security updates

3. **Network Security**:
   - Use firewall rules
   - VPN for database access
   - Rate limiting at proxy level

### Reverse Proxy Setup (Nginx)

```nginx
server {
    listen 80;
    server_name your-domain.com;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name your-domain.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location /ws {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
    }
}
```

### Systemd Service

Create `/etc/systemd/system/uptime-monitor.service`:

```ini
[Unit]
Description=Uptime Monitor Service
After=network.target postgresql.service redis.service

[Service]
Type=simple
User=uptime-monitor
Group=uptime-monitor
WorkingDirectory=/opt/uptime-monitor
ExecStart=/opt/uptime-monitor/main -config /opt/uptime-monitor/config.yml
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/uptime-monitor/logs

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl enable uptime-monitor
sudo systemctl start uptime-monitor
sudo systemctl status uptime-monitor
```

## Monitoring and Maintenance

### Health Checks

```bash
# Application health
curl http://localhost:8080/health

# Database connectivity
curl http://localhost:8080/health/ready

# Detailed metrics
curl http://localhost:8080/metrics
```

### Log Management

```bash
# View application logs
docker-compose logs -f uptime-monitor-app

# View database logs
docker-compose logs -f postgres

# System service logs
sudo journalctl -u uptime-monitor -f
```

### Backup Strategy

1. **Database Backup**:
```bash
# Daily backup
pg_dump uptime_monitor > backup_$(date +%Y%m%d).sql

# Automated backup script
#!/bin/bash
BACKUP_DIR="/backups"
DATE=$(date +%Y%m%d_%H%M%S)
pg_dump uptime_monitor | gzip > "$BACKUP_DIR/uptime_monitor_$DATE.sql.gz"
find "$BACKUP_DIR" -name "*.sql.gz" -mtime +7 -delete
```

2. **Configuration Backup**:
   - Store configuration in version control
   - Backup environment files securely
   - Document all custom settings

### Performance Tuning

1. **Database Optimization**:
   - Regular VACUUM and ANALYZE
   - Monitor slow queries
   - Adjust connection pool settings
   - Consider read replicas for high load

2. **Application Optimization**:
   - Monitor memory usage
   - Adjust worker pool size
   - Tune Redis cache settings
   - Enable compression for HTTP responses

3. **System Optimization**:
   - Monitor disk I/O
   - Optimize network settings
   - Use SSD storage for database
   - Consider horizontal scaling

## Troubleshooting

### Common Issues

1. **Application won't start**:
   - Check configuration file syntax
   - Verify database connectivity
   - Check port availability
   - Review logs for errors

2. **High memory usage**:
   - Reduce worker pool size
   - Adjust database connection limits
   - Monitor for memory leaks
   - Consider increasing system RAM

3. **Slow performance**:
   - Check database query performance
   - Monitor Redis connectivity
   - Review network latency
   - Optimize monitor check intervals

4. **Alert delivery failures**:
   - Verify alert channel credentials
   - Check network connectivity
   - Review rate limiting settings
   - Monitor external service status

### Getting Help

- Check application logs first
- Review configuration settings
- Test individual components
- Monitor system resources
- Check external service status
