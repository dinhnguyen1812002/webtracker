# Uptime Monitoring & Alert System - Setup Guide

## Overview

The Uptime Monitoring & Alert System is a lightweight monitoring solution designed for freelancers and agencies managing 10-50 websites. This guide covers installation, configuration, and deployment options.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Quick Start with Docker](#quick-start-with-docker)
- [Manual Installation](#manual-installation)
- [Configuration](#configuration)
- [Alert Channels Setup](#alert-channels-setup)
- [Production Deployment](#production-deployment)
- [Troubleshooting](#troubleshooting)

## Prerequisites

### System Requirements

- **Memory**: Minimum 512MB RAM (100MB for the application + overhead)
- **Storage**: 1GB free space (for database and logs)
- **Network**: Internet access for monitoring external websites

### Software Requirements

**For Docker deployment (Recommended):**
- Docker 20.10+ 
- Docker Compose 2.0+

**For manual installation:**
- Go 1.25+
- PostgreSQL 12+
- Redis/Valkey 6+ (optional but recommended)

## Quick Start with Docker

The fastest way to get started is using Docker Compose:

### 1. Clone and Setup

```bash
# Clone the repository (or download the release)
git clone <repository-url>
cd uptime-monitoring-system

# Copy environment template
cp .env.example .env
```

### 2. Configure Environment

Edit `.env` file with your settings:

```bash
# Required: Set a secure PostgreSQL password
POSTGRES_PASSWORD=your_secure_postgres_password

# Optional: Configure alert channels (see Alert Channels Setup section)
TELEGRAM_BOT_TOKEN=your_telegram_bot_token
TELEGRAM_CHAT_ID=your_telegram_chat_id

SMTP_HOST=smtp.gmail.com
SMTP_USERNAME=your_email@gmail.com
SMTP_PASSWORD=your_app_password
SMTP_FROM_ADDRESS=your_email@gmail.com
```

### 3. Start Services

```bash
# Start all services (PostgreSQL, Redis, Application)
docker-compose up -d

# Check service status
docker-compose ps

# View logs
docker-compose logs -f uptime-monitor-app
```

### 4. Verify Installation

```bash
# Health check
curl http://localhost:8080/health

# Access dashboard
open http://localhost:8080
```

The system is now ready! You can:
- Access the web dashboard at `http://localhost:8080`
- Use the REST API at `http://localhost:8080/api/v1/`
- Add monitors through the dashboard or API

## Manual Installation

### 1. Install Dependencies

**PostgreSQL:**
```bash
# Ubuntu/Debian
sudo apt update
sudo apt install postgresql postgresql-contrib

# macOS
brew install postgresql

# Start PostgreSQL
sudo systemctl start postgresql  # Linux
brew services start postgresql   # macOS
```

**Redis (Optional but recommended):**
```bash
# Ubuntu/Debian
sudo apt install redis-server

# macOS
brew install redis

# Start Redis
sudo systemctl start redis-server  # Linux
brew services start redis          # macOS
```

### 2. Setup Database

```bash
# Connect to PostgreSQL
sudo -u postgres psql

# Create database and user
CREATE DATABASE uptime_monitor;
CREATE USER uptime_user WITH PASSWORD 'secure_password';
GRANT ALL PRIVILEGES ON DATABASE uptime_monitor TO uptime_user;
\q
```

### 3. Build Application

```bash
# Install Go dependencies
go mod download

# Install templ for template compilation
go install github.com/a-h/templ/cmd/templ@latest

# Generate templates
templ generate

# Build application
go build -o uptime-monitor ./cmd/main.go
```

### 4. Configure Application

Create a configuration file `config.yaml`:

```yaml
server:
  port: 8080
  host: "0.0.0.0"

database:
  host: "localhost"
  port: 5432
  database: "uptime_monitor"
  user: "uptime_user"
  password: "secure_password"
  ssl_mode: "prefer"

redis:
  addr: "localhost:6379"
  enabled: true

logging:
  level: "info"
  format: "json"
```

### 5. Run Application

```bash
# Run database migrations and start server
./uptime-monitor -config config.yaml
```

## Configuration

The application supports multiple configuration methods:

### 1. Configuration File (Recommended)

Create `config.yaml` or `config.json`:

```yaml
server:
  port: 8080
  host: "0.0.0.0"
  read_timeout: "30s"
  write_timeout: "30s"

database:
  host: "localhost"
  port: 5432
  database: "uptime_monitor"
  user: "postgres"
  password: "password"
  ssl_mode: "prefer"
  max_connections: 20
  min_connections: 2

redis:
  addr: "localhost:6379"
  password: ""
  db: 0
  enabled: true

alert:
  telegram:
    bot_token: "${TELEGRAM_BOT_TOKEN}"
    chat_id: "${TELEGRAM_CHAT_ID}"
    enabled: true
  
  email:
    smtp_host: "${SMTP_HOST}"
    smtp_port: 587
    username: "${SMTP_USERNAME}"
    password: "${SMTP_PASSWORD}"
    from_address: "${SMTP_FROM_ADDRESS}"
    use_tls: true
    enabled: true
  
  webhook:
    url: "${WEBHOOK_URL}"
    timeout: "10s"
    enabled: false

worker:
  pool_size: 10
  queue_size: 500
  job_timeout: "60s"

logging:
  level: "info"
  format: "json"
  output: "stdout"
```

### 2. Environment Variables

All configuration can be set via environment variables:

```bash
# Server
export PORT=8080
export HOST=0.0.0.0

# Database
export DB_HOST=localhost
export DB_PORT=5432
export DB_NAME=uptime_monitor
export DB_USER=postgres
export DB_PASSWORD=password
export DB_SSL_MODE=prefer

# Redis
export REDIS_ADDR=localhost:6379
export REDIS_ENABLED=true

# Worker Pool
export WORKER_POOL_SIZE=10

# Logging
export LOG_LEVEL=info
export LOG_FORMAT=json
```

### 3. Generate Sample Configuration

```bash
# Generate sample config file
./uptime-monitor -generate-config config.yaml
```

## Alert Channels Setup

### Telegram Bot

1. **Create Bot:**
   - Message @BotFather on Telegram
   - Send `/newbot` and follow instructions
   - Save the bot token

2. **Get Chat ID:**
   - Add bot to your chat/channel
   - Send a message to the bot
   - Visit: `https://api.telegram.org/bot<TOKEN>/getUpdates`
   - Find your chat ID in the response

3. **Configure:**
   ```bash
   export TELEGRAM_BOT_TOKEN=your_bot_token
   export TELEGRAM_CHAT_ID=your_chat_id
   ```

### Email (SMTP)

**Gmail Example:**
1. Enable 2-factor authentication
2. Generate app password: Google Account → Security → App passwords
3. Configure:
   ```bash
   export SMTP_HOST=smtp.gmail.com
   export SMTP_PORT=587
   export SMTP_USERNAME=your_email@gmail.com
   export SMTP_PASSWORD=your_app_password
   export SMTP_FROM_ADDRESS=your_email@gmail.com
   ```

**Other Providers:**
- **Outlook**: `smtp-mail.outlook.com:587`
- **Yahoo**: `smtp.mail.yahoo.com:587`
- **Custom SMTP**: Use your provider's settings

### Webhook

Configure a webhook endpoint to receive JSON alerts:

```bash
export WEBHOOK_URL=https://your-webhook-endpoint.com/alerts
```

**Webhook Payload Example:**
```json
{
  "monitor_id": "uuid",
  "monitor_name": "My Website",
  "type": "downtime",
  "severity": "critical",
  "message": "Monitor is down",
  "timestamp": "2024-01-01T00:00:00Z",
  "details": {
    "status_code": 0,
    "error": "connection timeout"
  }
}
```

## Production Deployment

### Docker Compose (Recommended)

1. **Prepare Production Environment:**
   ```bash
   # Create production directory
   mkdir /opt/uptime-monitor
   cd /opt/uptime-monitor
   
   # Copy files
   cp docker-compose.prod.yml docker-compose.yml
   cp .env.example .env
   ```

2. **Configure Production Settings:**
   ```bash
   # Edit .env with production values
   POSTGRES_PASSWORD=very_secure_password
   LOG_LEVEL=warn
   
   # Configure alert channels
   TELEGRAM_BOT_TOKEN=your_production_bot_token
   # ... other settings
   ```

3. **Deploy:**
   ```bash
   # Start services
   docker-compose up -d
   
   # Setup log rotation
   docker-compose logs --no-color > /var/log/uptime-monitor.log
   ```

### Reverse Proxy Setup

**Nginx Configuration:**
```nginx
server {
    listen 80;
    server_name monitor.yourdomain.com;
    
    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # WebSocket support
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

### Systemd Service (Manual Installation)

Create `/etc/systemd/system/uptime-monitor.service`:

```ini
[Unit]
Description=Uptime Monitor Service
After=network.target postgresql.service redis.service

[Service]
Type=simple
User=uptime-monitor
WorkingDirectory=/opt/uptime-monitor
ExecStart=/opt/uptime-monitor/uptime-monitor -config /opt/uptime-monitor/config.yaml
Restart=always
RestartSec=5

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/uptime-monitor

[Install]
WantedBy=multi-user.target
```

Enable and start:
```bash
sudo systemctl enable uptime-monitor
sudo systemctl start uptime-monitor
sudo systemctl status uptime-monitor
```

### Monitoring and Maintenance

1. **Health Checks:**
   ```bash
   # Application health
   curl http://localhost:8080/health
   
   # Detailed health with dependencies
   curl http://localhost:8080/health/ready
   ```

2. **Metrics:**
   ```bash
   # Prometheus metrics
   curl http://localhost:8080/metrics
   ```

3. **Log Monitoring:**
   ```bash
   # Docker logs
   docker-compose logs -f uptime-monitor-app
   
   # Systemd logs
   journalctl -u uptime-monitor -f
   ```

4. **Database Maintenance:**
   ```bash
   # The application automatically:
   # - Runs migrations on startup
   # - Cleans up old data (90-day retention)
   # - Aggregates old health checks
   ```

## Troubleshooting

### Common Issues

**1. Application won't start:**
```bash
# Check configuration
./uptime-monitor -config config.yaml -health-check

# Check logs
docker-compose logs uptime-monitor-app
```

**2. Database connection failed:**
```bash
# Verify PostgreSQL is running
docker-compose ps postgres
# or
sudo systemctl status postgresql

# Check connection
psql -h localhost -U postgres -d uptime_monitor
```

**3. Redis connection failed:**
```bash
# Verify Redis is running
docker-compose ps redis
# or
sudo systemctl status redis

# Check connection
redis-cli ping
```

**4. Alerts not working:**
```bash
# Check alert configuration
curl http://localhost:8080/api/v1/monitors

# Test Telegram bot
curl "https://api.telegram.org/bot<TOKEN>/getMe"

# Check logs for delivery errors
docker-compose logs uptime-monitor-app | grep -i alert
```

**5. High memory usage:**
```bash
# Check memory stats in logs
docker-compose logs uptime-monitor-app | grep -i memory

# Reduce worker pool size
export WORKER_POOL_SIZE=5

# Disable Redis if not needed
export REDIS_ENABLED=false
```

### Performance Tuning

**For monitoring 10-20 websites:**
```yaml
worker:
  pool_size: 5
  queue_size: 200

database:
  max_connections: 10
  min_connections: 2

redis:
  pool_size: 3
  min_idle_conns: 1
```

**For monitoring 50+ websites:**
```yaml
worker:
  pool_size: 15
  queue_size: 1000

database:
  max_connections: 25
  min_connections: 5

redis:
  pool_size: 8
  min_idle_conns: 2
```

### Getting Help

1. **Check logs first:**
   ```bash
   docker-compose logs uptime-monitor-app --tail=100
   ```

2. **Verify configuration:**
   ```bash
   ./uptime-monitor -generate-config sample.yaml
   # Compare with your config
   ```

3. **Test connectivity:**
   ```bash
   # Database
   ./uptime-monitor -health-check
   
   # External websites
   curl -I https://example.com
   ```

4. **Enable debug logging:**
   ```bash
   export LOG_LEVEL=debug
   docker-compose restart uptime-monitor-app
   ```

## Next Steps

After successful installation:

1. **Add your first monitor** via the dashboard at `http://localhost:8080`
2. **Configure alert channels** for notifications
3. **Set up monitoring** for the monitoring system itself
4. **Review the [API Documentation](API.md)** for programmatic access
5. **Check the [Operations Guide](OPERATIONS.md)** for maintenance procedures

The system is designed to be low-maintenance once configured. It will automatically handle data retention, performance optimization, and error recovery.
