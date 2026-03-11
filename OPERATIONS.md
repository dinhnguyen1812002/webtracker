# Uptime Monitoring & Alert System - Operations Guide

## Overview

This operations guide provides comprehensive information for monitoring, maintaining, and troubleshooting the Uptime Monitoring & Alert System in production environments. It covers observability, common issues, performance tuning, and scaling recommendations.

## Table of Contents

- [Monitoring and Observability](#monitoring-and-observability)
- [Health Checks](#health-checks)
- [Logging](#logging)
- [Metrics and Performance](#metrics-and-performance)
- [Troubleshooting](#troubleshooting)
- [Maintenance Procedures](#maintenance-procedures)
- [Scaling Recommendations](#scaling-recommendations)
- [Security Considerations](#security-considerations)
- [Backup and Recovery](#backup-and-recovery)

## Monitoring and Observability

### System Health Endpoints

The application provides several health check endpoints for monitoring:

#### Basic Health Check
```bash
curl http://localhost:8080/health
```
**Use case:** Basic liveness probe for load balancers
**Expected response:** `200 OK` with `{"status": "ok"}`

#### Readiness Check
```bash
curl http://localhost:8080/health/ready
```
**Use case:** Kubernetes readiness probe, deployment verification
**Expected response:** `200 OK` when all dependencies are healthy
**Checks:** Database connectivity, worker pool status, scheduler status

#### Liveness Check
```bash
curl http://localhost:8080/health/live
```
**Use case:** Kubernetes liveness probe
**Expected response:** `200 OK` when application is responsive

### System Metrics

#### Application Metrics
```bash
curl http://localhost:8080/metrics
```

**Key metrics to monitor:**
- `worker_pool.active_workers` - Number of active worker goroutines
- `worker_pool.queue_depth` - Number of pending health checks
- `worker_pool.processed_jobs` - Total processed health checks
- `database.total_monitors` - Number of configured monitors
- `database.enabled_monitors` - Number of active monitors

#### Memory Monitoring

The application includes built-in memory profiling:

```bash
# Check current memory usage in logs
docker-compose logs app | grep -i memory

# Memory stats are logged at:
# - Application startup
# - After major operations
# - Every 30 seconds during monitoring
# - When memory exceeds thresholds
```

**Memory thresholds:**
- **Idle**: < 10MB RAM
- **Normal operation**: < 100MB RAM with 100 monitors
- **Warning threshold**: > 90MB RAM
- **Critical threshold**: > 150MB RAM (soft limit)

### External Monitoring Integration

#### Prometheus Integration

The `/metrics` endpoint provides JSON metrics that can be converted to Prometheus format:

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'uptime-monitor'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/metrics'
    scrape_interval: 30s
```

#### Grafana Dashboard

Key metrics to visualize:
- Worker pool utilization over time
- Queue depth trends
- Memory usage patterns
- Database connection pool stats
- Monitor success/failure rates

#### Alerting Rules

**Critical alerts:**
- Application down (health check fails)
- Memory usage > 150MB
- Queue depth > 1000 for > 5 minutes
- Database connection failures

**Warning alerts:**
- Memory usage > 90MB
- Queue depth > 500
- Worker pool utilization > 80%
- High error rates in logs

## Health Checks

### Application Health Monitoring

#### Docker Compose Health Checks

The application includes built-in health checks in Docker Compose:

```yaml
healthcheck:
  test: ["CMD", "/main", "-health-check"]
  interval: 30s
  timeout: 10s
  retries: 3
  start_period: 30s
```

#### Kubernetes Health Checks

```yaml
apiVersion: v1
kind: Pod
spec:
  containers:
  - name: uptime-monitor
    livenessProbe:
      httpGet:
        path: /health/live
        port: 8080
      initialDelaySeconds: 30
      periodSeconds: 30
      timeoutSeconds: 10
      failureThreshold: 3
    
    readinessProbe:
      httpGet:
        path: /health/ready
        port: 8080
      initialDelaySeconds: 10
      periodSeconds: 10
      timeoutSeconds: 5
      failureThreshold: 3
```

### Dependency Health Monitoring

#### PostgreSQL Health
```bash
# Check database connectivity
docker-compose exec postgres pg_isready -U postgres -d uptime_monitor

# Check database size
docker-compose exec postgres psql -U postgres -d uptime_monitor -c "
SELECT 
    schemaname,
    tablename,
    attname,
    n_distinct,
    correlation
FROM pg_stats 
WHERE schemaname = 'public';"
```

#### Redis Health
```bash
# Check Redis connectivity
docker-compose exec redis valkey-cli ping

# Check Redis memory usage
docker-compose exec redis valkey-cli info memory

# Check Redis key count
docker-compose exec redis valkey-cli dbsize
```

## Logging

### Log Structure

The application uses structured JSON logging with the following format:

```json
{
  "timestamp": "2024-01-01T00:00:00Z",
  "severity": "info",
  "msg": "Health check completed",
  "monitor_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "success",
  "response_time_ms": 150,
  "request_id": "req_123"
}
```

### Log Levels

- **DEBUG**: Detailed execution flow (disabled in production)
- **INFO**: Normal operations, health checks, system events
- **WARN**: Recoverable errors, performance issues, degraded mode
- **ERROR**: Failures, exceptions, critical issues

### Log Categories

#### Health Check Logs
```bash
# Filter health check logs
docker-compose logs app | jq 'select(.msg | contains("Health check"))'

# Monitor failed health checks
docker-compose logs app | jq 'select(.status == "failure")'
```

#### Alert Delivery Logs
```bash
# Filter alert delivery logs
docker-compose logs app | jq 'select(.msg | contains("Alert"))'

# Monitor failed alert deliveries
docker-compose logs app | jq 'select(.msg | contains("Alert delivery failed"))'
```

#### Memory and Performance Logs
```bash
# Monitor memory usage
docker-compose logs app | jq 'select(.msg | contains("Memory"))'

# Monitor worker pool stats
docker-compose logs app | jq 'select(.msg | contains("Worker pool"))'
```

### Log Management

#### Log Rotation (Docker)

```yaml
# docker-compose.yml
services:
  app:
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"
```

#### Log Aggregation

**ELK Stack Integration:**
```bash
# Filebeat configuration for log shipping
filebeat.inputs:
- type: container
  paths:
    - '/var/lib/docker/containers/*/*.log'
  processors:
  - add_docker_metadata: ~
  - decode_json_fields:
      fields: ["message"]
      target: ""
```

**Centralized Logging:**
```bash
# Ship logs to external system
docker-compose logs app --no-color | \
  while read line; do
    curl -X POST https://logs.example.com/ingest \
      -H "Content-Type: application/json" \
      -d "$line"
  done
```

## Metrics and Performance

### Performance Monitoring

#### Response Time Monitoring

```bash
# Monitor API response times
curl -w "@curl-format.txt" -o /dev/null -s http://localhost:8080/health

# curl-format.txt content:
#     time_namelookup:  %{time_namelookup}\n
#        time_connect:  %{time_connect}\n
#     time_appconnect:  %{time_appconnect}\n
#    time_pretransfer:  %{time_pretransfer}\n
#       time_redirect:  %{time_redirect}\n
#  time_starttransfer:  %{time_starttransfer}\n
#                     ----------\n
#          time_total:  %{time_total}\n
```

#### Database Performance

```sql
-- Monitor slow queries
SELECT query, mean_time, calls, total_time
FROM pg_stat_statements
WHERE mean_time > 100
ORDER BY mean_time DESC;

-- Monitor connection usage
SELECT 
    state,
    count(*) as connections
FROM pg_stat_activity 
WHERE datname = 'uptime_monitor'
GROUP BY state;

-- Monitor table sizes
SELECT 
    schemaname,
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as size
FROM pg_tables 
WHERE schemaname = 'public'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;
```

#### Redis Performance

```bash
# Monitor Redis performance
docker-compose exec redis valkey-cli --latency-history -i 1

# Monitor Redis memory usage
docker-compose exec redis valkey-cli info memory | grep used_memory_human

# Monitor Redis operations
docker-compose exec redis valkey-cli monitor
```

### Performance Tuning

#### Memory Optimization

The application includes automatic memory optimization:

```go
// Applied automatically on startup
- GC target: 50% (more aggressive than default 100%)
- Memory limit: 150MB soft limit
- Automatic garbage collection when memory > 90MB
```

**Manual memory optimization:**
```bash
# Force garbage collection (via API if implemented)
curl -X POST http://localhost:8080/admin/gc

# Check memory stats
curl http://localhost:8080/metrics | jq '.system.memory'
```

#### Database Optimization

```sql
-- Optimize PostgreSQL for monitoring workload
ALTER SYSTEM SET shared_buffers = '128MB';
ALTER SYSTEM SET effective_cache_size = '512MB';
ALTER SYSTEM SET maintenance_work_mem = '64MB';
ALTER SYSTEM SET checkpoint_completion_target = 0.9;
ALTER SYSTEM SET wal_buffers = '16MB';
ALTER SYSTEM SET default_statistics_target = 100;

-- Reload configuration
SELECT pg_reload_conf();
```

#### Worker Pool Tuning

**For 10-20 monitors:**
```yaml
environment:
  WORKER_POOL_SIZE: 5
```

**For 50+ monitors:**
```yaml
environment:
  WORKER_POOL_SIZE: 15
```

**For high-frequency monitoring:**
```yaml
environment:
  WORKER_POOL_SIZE: 20
  # Increase queue size
  REDIS_MAXMEMORY: 512mb
```

## Troubleshooting

### Common Issues

#### 1. High Memory Usage

**Symptoms:**
- Memory usage > 100MB
- Frequent garbage collection
- Slow response times

**Diagnosis:**
```bash
# Check memory stats
docker-compose logs app | grep -i memory | tail -10

# Check for memory leaks
curl http://localhost:8080/metrics | jq '.worker_pool'
```

**Solutions:**
```bash
# Reduce worker pool size
export WORKER_POOL_SIZE=5
docker-compose restart app

# Disable Redis if not needed
export REDIS_ENABLED=false
docker-compose restart app

# Force garbage collection
docker-compose restart app
```

#### 2. Queue Backup

**Symptoms:**
- Queue depth > 500
- Delayed health checks
- Increasing response times

**Diagnosis:**
```bash
# Check queue depth
curl http://localhost:8080/metrics | jq '.worker_pool.queue_depth'

# Check worker utilization
curl http://localhost:8080/metrics | jq '.worker_pool.active_workers'
```

**Solutions:**
```bash
# Increase worker pool size
export WORKER_POOL_SIZE=15
docker-compose restart app

# Reduce check frequency for some monitors
# (via API or database update)

# Check for stuck workers
docker-compose restart app
```

#### 3. Database Connection Issues

**Symptoms:**
- "connection refused" errors
- Health check failures
- 503 responses from /health/ready

**Diagnosis:**
```bash
# Check database connectivity
docker-compose exec postgres pg_isready

# Check connection pool
docker-compose logs app | grep -i "database\|connection"

# Check active connections
docker-compose exec postgres psql -U postgres -c "
SELECT count(*) FROM pg_stat_activity WHERE datname = 'uptime_monitor';"
```

**Solutions:**
```bash
# Restart database
docker-compose restart postgres

# Reduce connection pool size
export DB_MAX_CONNECTIONS=10
docker-compose restart app

# Check database logs
docker-compose logs postgres
```

#### 4. Alert Delivery Failures

**Symptoms:**
- Alerts not received
- "Alert delivery failed" in logs
- High error rates

**Diagnosis:**
```bash
# Check alert delivery logs
docker-compose logs app | grep -i alert | grep -i error

# Test alert channels manually
curl -X POST https://api.telegram.org/bot<TOKEN>/getMe
```

**Solutions:**
```bash
# Verify alert channel configuration
curl http://localhost:8080/api/v1/monitors/<ID>

# Test SMTP connectivity
telnet smtp.gmail.com 587

# Check webhook endpoint
curl -X POST <WEBHOOK_URL> -d '{"test": true}'
```

#### 5. SSL Certificate Monitoring Issues

**Symptoms:**
- SSL alerts not working
- Certificate validation errors
- HTTPS health checks failing

**Diagnosis:**
```bash
# Test SSL certificate manually
openssl s_client -connect example.com:443 -servername example.com

# Check SSL-related logs
docker-compose logs app | grep -i ssl | grep -i error
```

**Solutions:**
```bash
# Update CA certificates
docker-compose build --no-cache app

# Check system time
docker-compose exec app date

# Verify certificate chain
curl -vI https://example.com
```

### Debugging Tools

#### Log Analysis

```bash
# Real-time log monitoring
docker-compose logs -f app | jq -r '"\(.timestamp) [\(.severity)] \(.msg)"'

# Error analysis
docker-compose logs app | jq 'select(.severity == "error")' | jq -r '.msg' | sort | uniq -c

# Performance analysis
docker-compose logs app | jq 'select(.response_time_ms > 5000)' | jq '.monitor_id' | sort | uniq -c
```

#### Database Debugging

```sql
-- Find slow monitors
SELECT 
    m.name,
    m.url,
    AVG(hc.response_time_ms) as avg_response_time,
    COUNT(CASE WHEN hc.status = 'failure' THEN 1 END) as failures
FROM monitors m
JOIN health_checks hc ON m.id = hc.monitor_id
WHERE hc.checked_at > NOW() - INTERVAL '1 hour'
GROUP BY m.id, m.name, m.url
ORDER BY avg_response_time DESC;

-- Find monitors with high failure rates
SELECT 
    m.name,
    m.url,
    COUNT(*) as total_checks,
    COUNT(CASE WHEN hc.status = 'failure' THEN 1 END) as failures,
    ROUND(COUNT(CASE WHEN hc.status = 'failure' THEN 1 END) * 100.0 / COUNT(*), 2) as failure_rate
FROM monitors m
JOIN health_checks hc ON m.id = hc.monitor_id
WHERE hc.checked_at > NOW() - INTERVAL '24 hours'
GROUP BY m.id, m.name, m.url
HAVING COUNT(CASE WHEN hc.status = 'failure' THEN 1 END) > 0
ORDER BY failure_rate DESC;
```

## Maintenance Procedures

### Regular Maintenance Tasks

#### Daily Tasks

1. **Check system health:**
```bash
curl http://localhost:8080/health/ready
```

2. **Monitor memory usage:**
```bash
docker-compose logs app | grep -i memory | tail -5
```

3. **Check error rates:**
```bash
docker-compose logs app --since 24h | jq 'select(.severity == "error")' | wc -l
```

#### Weekly Tasks

1. **Database maintenance:**
```sql
-- Analyze tables for query optimization
ANALYZE;

-- Check for bloated tables
SELECT 
    schemaname, 
    tablename, 
    n_dead_tup, 
    n_live_tup,
    ROUND(n_dead_tup * 100.0 / (n_live_tup + n_dead_tup), 2) as dead_percentage
FROM pg_stat_user_tables 
WHERE n_dead_tup > 0
ORDER BY dead_percentage DESC;

-- Vacuum if needed
VACUUM ANALYZE;
```

2. **Log rotation:**
```bash
# Rotate Docker logs
docker-compose logs app --no-color > /var/log/uptime-monitor-$(date +%Y%m%d).log
docker-compose restart app
```

3. **Performance review:**
```bash
# Check average response times
curl http://localhost:8080/metrics | jq '.worker_pool'
```

#### Monthly Tasks

1. **Update dependencies:**
```bash
# Update Docker images
docker-compose pull
docker-compose up -d

# Update Go dependencies (if building from source)
go mod tidy
go mod download
```

2. **Backup verification:**
```bash
# Test database backup restore
pg_restore --dry-run backup.sql
```

3. **Capacity planning:**
```bash
# Review growth trends
docker-compose exec postgres psql -U postgres -d uptime_monitor -c "
SELECT 
    DATE(created_at) as date,
    COUNT(*) as monitors_created
FROM monitors 
WHERE created_at > NOW() - INTERVAL '30 days'
GROUP BY DATE(created_at)
ORDER BY date;"
```

### Data Retention

The application automatically handles data retention:

- **Health checks**: 90 days (configurable)
- **Alerts**: 90 days (configurable)
- **Aggregated data**: Indefinite (hourly summaries after 7 days)

**Manual cleanup:**
```sql
-- Clean old health checks (older than 90 days)
DELETE FROM health_checks WHERE checked_at < NOW() - INTERVAL '90 days';

-- Clean old alerts (older than 90 days)
DELETE FROM alerts WHERE sent_at < NOW() - INTERVAL '90 days';

-- Vacuum after cleanup
VACUUM ANALYZE health_checks;
VACUUM ANALYZE alerts;
```

### Updates and Upgrades

#### Application Updates

1. **Backup data:**
```bash
docker-compose exec postgres pg_dump -U postgres uptime_monitor > backup.sql
```

2. **Update application:**
```bash
# Pull latest image
docker-compose pull app

# Stop application
docker-compose stop app

# Start with new image
docker-compose up -d app
```

3. **Verify update:**
```bash
# Check health
curl http://localhost:8080/health/ready

# Check logs
docker-compose logs app --tail 50
```

#### Database Migrations

Migrations run automatically on startup, but for manual control:

```bash
# Run migrations manually
docker-compose exec app /main -migrate

# Check migration status
docker-compose exec postgres psql -U postgres -d uptime_monitor -c "
SELECT * FROM schema_migrations ORDER BY version;"
```

## Scaling Recommendations

### Vertical Scaling

#### Small Deployment (10-20 monitors)
```yaml
# docker-compose.yml
services:
  app:
    deploy:
      resources:
        limits:
          memory: 256M
          cpus: '0.5'
    environment:
      WORKER_POOL_SIZE: 5
      DB_MAX_CONNECTIONS: 10
```

#### Medium Deployment (50-100 monitors)
```yaml
services:
  app:
    deploy:
      resources:
        limits:
          memory: 512M
          cpus: '1.0'
    environment:
      WORKER_POOL_SIZE: 15
      DB_MAX_CONNECTIONS: 20
```

#### Large Deployment (100+ monitors)
```yaml
services:
  app:
    deploy:
      resources:
        limits:
          memory: 1G
          cpus: '2.0'
    environment:
      WORKER_POOL_SIZE: 25
      DB_MAX_CONNECTIONS: 30
```

### Horizontal Scaling

For very large deployments, consider:

1. **Multiple application instances:**
```yaml
services:
  app:
    deploy:
      replicas: 3
    environment:
      # Shared database and Redis
      DB_HOST: postgres
      REDIS_ADDR: redis:6379
```

2. **Load balancing:**
```nginx
upstream uptime_monitor {
    server app1:8080;
    server app2:8080;
    server app3:8080;
}

server {
    location / {
        proxy_pass http://uptime_monitor;
    }
}
```

3. **Database scaling:**
```yaml
services:
  postgres:
    deploy:
      resources:
        limits:
          memory: 2G
          cpus: '2.0'
    environment:
      POSTGRES_SHARED_BUFFERS: 512MB
      POSTGRES_EFFECTIVE_CACHE_SIZE: 1GB
```

### Performance Limits

**Single instance limits:**
- **Monitors**: ~500 monitors with 5-minute intervals
- **Memory**: ~100MB for 100 monitors
- **CPU**: ~0.5 cores for normal operation
- **Database**: ~1000 health checks per minute

**Scaling indicators:**
- Queue depth consistently > 100
- Memory usage > 80% of limit
- CPU usage > 70%
- Database connection pool exhaustion

## Security Considerations

### Network Security

1. **Firewall configuration:**
```bash
# Allow only necessary ports
ufw allow 8080/tcp  # Application
ufw allow 5432/tcp  # PostgreSQL (internal only)
ufw allow 6379/tcp  # Redis (internal only)
```

2. **TLS termination:**
```nginx
server {
    listen 443 ssl;
    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;
    
    location / {
        proxy_pass http://localhost:8080;
    }
}
```

### Data Security

1. **Database encryption:**
```yaml
services:
  postgres:
    environment:
      POSTGRES_INITDB_ARGS: "--auth-host=scram-sha-256"
    volumes:
      - postgres_data:/var/lib/postgresql/data:Z
```

2. **Secrets management:**
```yaml
services:
  app:
    secrets:
      - db_password
      - telegram_token
    environment:
      DB_PASSWORD_FILE: /run/secrets/db_password
      TELEGRAM_BOT_TOKEN_FILE: /run/secrets/telegram_token
```

### Access Control

1. **API authentication (if implemented):**
```bash
# Add API key authentication
curl -H "Authorization: Bearer <token>" http://localhost:8080/api/v1/monitors
```

2. **Database access:**
```sql
-- Create read-only user for monitoring
CREATE USER monitor_readonly WITH PASSWORD 'secure_password';
GRANT CONNECT ON DATABASE uptime_monitor TO monitor_readonly;
GRANT USAGE ON SCHEMA public TO monitor_readonly;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO monitor_readonly;
```

## Backup and Recovery

### Database Backup

#### Automated Backup
```bash
#!/bin/bash
# backup.sh
DATE=$(date +%Y%m%d_%H%M%S)
docker-compose exec -T postgres pg_dump -U postgres uptime_monitor > "backup_${DATE}.sql"

# Compress backup
gzip "backup_${DATE}.sql"

# Upload to S3 (optional)
aws s3 cp "backup_${DATE}.sql.gz" s3://your-backup-bucket/
```

#### Backup Verification
```bash
# Test backup integrity
gunzip -c backup_20240101_120000.sql.gz | head -20

# Test restore (dry run)
docker-compose exec -T postgres psql -U postgres -c "CREATE DATABASE test_restore;"
gunzip -c backup_20240101_120000.sql.gz | docker-compose exec -T postgres psql -U postgres test_restore
```

### Disaster Recovery

#### Full System Recovery

1. **Restore database:**
```bash
# Stop application
docker-compose stop app

# Restore database
gunzip -c backup_latest.sql.gz | docker-compose exec -T postgres psql -U postgres uptime_monitor

# Start application
docker-compose start app
```

2. **Verify recovery:**
```bash
# Check data integrity
curl http://localhost:8080/api/v1/monitors | jq '.count'

# Check health
curl http://localhost:8080/health/ready
```

#### Point-in-Time Recovery

```bash
# Enable WAL archiving in PostgreSQL
echo "wal_level = replica" >> postgresql.conf
echo "archive_mode = on" >> postgresql.conf
echo "archive_command = 'cp %p /backup/wal/%f'" >> postgresql.conf

# Perform PITR
pg_basebackup -D /backup/base -Ft -z -P
```

This operations guide provides comprehensive coverage of monitoring, troubleshooting, and maintaining the Uptime Monitoring & Alert System in production environments. Regular review and updates of these procedures will ensure optimal system performance and reliability.