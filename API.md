# Uptime Monitoring & Alert System - API Documentation

## Overview

The Uptime Monitoring & Alert System provides a comprehensive REST API for managing monitors, viewing health check history, accessing alerts, and retrieving metrics. The API follows RESTful principles and returns JSON responses.

## Base URL

```
http://localhost:8080/api/v1
```

## Authentication

Currently, the API does not require authentication. In production deployments, consider implementing authentication and authorization mechanisms.

## Content Type

All API requests and responses use `application/json` content type.

## Error Handling

The API returns standard HTTP status codes and error messages in JSON format:

```json
{
  "error": "Error description"
}
```

Common HTTP status codes:
- `200 OK` - Request successful
- `201 Created` - Resource created successfully
- `204 No Content` - Request successful, no content returned
- `400 Bad Request` - Invalid request parameters
- `404 Not Found` - Resource not found
- `500 Internal Server Error` - Server error

## Table of Contents

- [Monitor Management](#monitor-management)
- [Health Check History](#health-check-history)
- [Alert History](#alert-history)
- [Metrics](#metrics)
- [System Health](#system-health)
- [WebSocket Real-time Updates](#websocket-real-time-updates)
- [Data Types](#data-types)

## Monitor Management

### Create Monitor

Create a new monitor to track website uptime.

**Endpoint:** `POST /api/v1/monitors`

**Request Body:**
```json
{
  "name": "My Website",
  "url": "https://example.com",
  "check_interval": 5,
  "enabled": true,
  "alert_channels": [
    {
      "type": "telegram",
      "config": {
        "bot_token": "your_bot_token",
        "chat_id": "your_chat_id"
      }
    },
    {
      "type": "email",
      "config": {
        "smtp_host": "smtp.gmail.com",
        "smtp_port": "587",
        "username": "your_email@gmail.com",
        "password": "your_app_password",
        "from_address": "your_email@gmail.com"
      }
    },
    {
      "type": "webhook",
      "config": {
        "url": "https://your-webhook.com/alerts"
      }
    }
  ]
}
```

**Request Parameters:**
- `name` (string, required): Display name for the monitor
- `url` (string, required): HTTP/HTTPS URL to monitor
- `check_interval` (integer, required): Check interval in minutes (1, 5, 15, or 60)
- `enabled` (boolean, optional): Whether the monitor is enabled (default: true)
- `alert_channels` (array, optional): Alert delivery channels

**Response:** `201 Created`
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "My Website",
  "url": "https://example.com",
  "check_interval": 5,
  "enabled": true,
  "alert_channels": [
    {
      "type": "telegram",
      "config": {
        "bot_token": "your_bot_token",
        "chat_id": "your_chat_id"
      }
    }
  ],
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z"
}
```

**Example:**
```bash
curl -X POST http://localhost:8080/api/v1/monitors \
  -H "Content-Type: application/json" \
  -d '{
    "name": "My Website",
    "url": "https://example.com",
    "check_interval": 5,
    "enabled": true
  }'
```

### List Monitors

Retrieve a list of all monitors with optional filtering.

**Endpoint:** `GET /api/v1/monitors`

**Query Parameters:**
- `enabled` (boolean, optional): Filter by enabled status
- `limit` (integer, optional): Maximum number of results (default: 100, max: 1000)
- `offset` (integer, optional): Number of results to skip (default: 0)

**Response:** `200 OK`
```json
{
  "monitors": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "name": "My Website",
      "url": "https://example.com",
      "check_interval": 5,
      "enabled": true,
      "alert_channels": [],
      "created_at": "2024-01-01T00:00:00Z",
      "updated_at": "2024-01-01T00:00:00Z"
    }
  ],
  "count": 1
}
```

**Examples:**
```bash
# Get all monitors
curl http://localhost:8080/api/v1/monitors

# Get only enabled monitors
curl http://localhost:8080/api/v1/monitors?enabled=true

# Get monitors with pagination
curl http://localhost:8080/api/v1/monitors?limit=10&offset=20
```

### Get Monitor

Retrieve details of a specific monitor.

**Endpoint:** `GET /api/v1/monitors/{id}`

**Path Parameters:**
- `id` (string, required): Monitor UUID

**Response:** `200 OK`
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "My Website",
  "url": "https://example.com",
  "check_interval": 5,
  "enabled": true,
  "alert_channels": [],
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z"
}
```

**Example:**
```bash
curl http://localhost:8080/api/v1/monitors/550e8400-e29b-41d4-a716-446655440000
```

### Update Monitor

Update an existing monitor's configuration.

**Endpoint:** `PUT /api/v1/monitors/{id}`

**Path Parameters:**
- `id` (string, required): Monitor UUID

**Request Body:**
```json
{
  "name": "Updated Website Name",
  "check_interval": 15,
  "enabled": false
}
```

**Request Parameters:** (all optional)
- `name` (string): Display name for the monitor
- `url` (string): HTTP/HTTPS URL to monitor
- `check_interval` (integer): Check interval in minutes (1, 5, 15, or 60)
- `enabled` (boolean): Whether the monitor is enabled
- `alert_channels` (array): Alert delivery channels

**Response:** `200 OK`
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "Updated Website Name",
  "url": "https://example.com",
  "check_interval": 15,
  "enabled": false,
  "alert_channels": [],
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:05:00Z"
}
```

**Example:**
```bash
curl -X PUT http://localhost:8080/api/v1/monitors/550e8400-e29b-41d4-a716-446655440000 \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Updated Website Name",
    "check_interval": 15
  }'
```

### Delete Monitor

Delete a monitor and all associated data.

**Endpoint:** `DELETE /api/v1/monitors/{id}`

**Path Parameters:**
- `id` (string, required): Monitor UUID

**Response:** `204 No Content`

**Example:**
```bash
curl -X DELETE http://localhost:8080/api/v1/monitors/550e8400-e29b-41d4-a716-446655440000
```

## Health Check History

### Get Health Check History

Retrieve health check history for a specific monitor.

**Endpoint:** `GET /api/v1/monitors/{id}/checks`

**Path Parameters:**
- `id` (string, required): Monitor UUID

**Query Parameters:**
- `limit` (integer, optional): Maximum number of results (default: 100, max: 1000)
- `start` (string, optional): Start date in RFC3339 format (e.g., "2024-01-01T00:00:00Z")
- `end` (string, optional): End date in RFC3339 format (e.g., "2024-01-02T00:00:00Z")

**Response:** `200 OK`
```json
{
  "checks": [
    {
      "id": "660e8400-e29b-41d4-a716-446655440000",
      "monitor_id": "550e8400-e29b-41d4-a716-446655440000",
      "status": "success",
      "status_code": 200,
      "response_time_ms": 150,
      "ssl_info": {
        "valid": true,
        "expires_at": "2024-12-31T23:59:59Z",
        "days_until": 365,
        "issuer": "Let's Encrypt"
      },
      "checked_at": "2024-01-01T00:00:00Z"
    },
    {
      "id": "770e8400-e29b-41d4-a716-446655440000",
      "monitor_id": "550e8400-e29b-41d4-a716-446655440000",
      "status": "failure",
      "status_code": 0,
      "response_time_ms": 30000,
      "error_message": "connection timeout",
      "checked_at": "2024-01-01T00:05:00Z"
    }
  ],
  "count": 2
}
```

**Examples:**
```bash
# Get recent health checks
curl http://localhost:8080/api/v1/monitors/550e8400-e29b-41d4-a716-446655440000/checks

# Get health checks with limit
curl http://localhost:8080/api/v1/monitors/550e8400-e29b-41d4-a716-446655440000/checks?limit=50

# Get health checks for date range
curl "http://localhost:8080/api/v1/monitors/550e8400-e29b-41d4-a716-446655440000/checks?start=2024-01-01T00:00:00Z&end=2024-01-02T00:00:00Z"
```

### Get Latest Health Check

Retrieve the most recent health check for a specific monitor.

**Endpoint:** `GET /api/v1/monitors/{id}/checks/latest`

**Path Parameters:**
- `id` (string, required): Monitor UUID

**Response:** `200 OK`
```json
{
  "id": "660e8400-e29b-41d4-a716-446655440000",
  "monitor_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "success",
  "status_code": 200,
  "response_time_ms": 150,
  "ssl_info": {
    "valid": true,
    "expires_at": "2024-12-31T23:59:59Z",
    "days_until": 365,
    "issuer": "Let's Encrypt"
  },
  "checked_at": "2024-01-01T00:00:00Z"
}
```

**Example:**
```bash
curl http://localhost:8080/api/v1/monitors/550e8400-e29b-41d4-a716-446655440000/checks/latest
```

## Alert History

### Get Alert History

Retrieve alert history for a specific monitor.

**Endpoint:** `GET /api/v1/monitors/{id}/alerts`

**Path Parameters:**
- `id` (string, required): Monitor UUID

**Query Parameters:**
- `limit` (integer, optional): Maximum number of results (default: 100, max: 1000)
- `start` (string, optional): Start date in RFC3339 format
- `end` (string, optional): End date in RFC3339 format
- `type` (string, optional): Filter by alert type (`downtime`, `recovery`, `ssl_expiring`, `ssl_expired`, `performance`)

**Response:** `200 OK`
```json
{
  "alerts": [
    {
      "id": "880e8400-e29b-41d4-a716-446655440000",
      "monitor_id": "550e8400-e29b-41d4-a716-446655440000",
      "type": "downtime",
      "severity": "critical",
      "message": "Monitor is down - connection timeout",
      "details": {
        "status_code": 0,
        "error": "connection timeout",
        "response_time_ms": 30000
      },
      "sent_at": "2024-01-01T00:05:00Z",
      "channels": ["telegram", "email"]
    },
    {
      "id": "990e8400-e29b-41d4-a716-446655440000",
      "monitor_id": "550e8400-e29b-41d4-a716-446655440000",
      "type": "recovery",
      "severity": "info",
      "message": "Monitor is back online",
      "details": {
        "status_code": 200,
        "response_time_ms": 150
      },
      "sent_at": "2024-01-01T00:10:00Z",
      "channels": ["telegram", "email"]
    }
  ],
  "count": 2
}
```

**Examples:**
```bash
# Get all alerts
curl http://localhost:8080/api/v1/monitors/550e8400-e29b-41d4-a716-446655440000/alerts

# Get only downtime alerts
curl http://localhost:8080/api/v1/monitors/550e8400-e29b-41d4-a716-446655440000/alerts?type=downtime

# Get alerts for date range
curl "http://localhost:8080/api/v1/monitors/550e8400-e29b-41d4-a716-446655440000/alerts?start=2024-01-01T00:00:00Z&end=2024-01-02T00:00:00Z"
```

## Metrics

### Get Uptime Metrics

Retrieve uptime percentage metrics for a specific monitor.

**Endpoint:** `GET /api/v1/monitors/{id}/uptime`

**Path Parameters:**
- `id` (string, required): Monitor UUID

**Response:** `200 OK`
```json
{
  "monitor_id": "550e8400-e29b-41d4-a716-446655440000",
  "period_24h": 99.95,
  "period_7d": 99.87,
  "period_30d": 99.92
}
```

**Example:**
```bash
curl http://localhost:8080/api/v1/monitors/550e8400-e29b-41d4-a716-446655440000/uptime
```

### Get Response Time Metrics

Retrieve response time statistics for a specific monitor.

**Endpoint:** `GET /api/v1/monitors/{id}/response`

**Path Parameters:**
- `id` (string, required): Monitor UUID

**Query Parameters:**
- `period` (string, optional): Time period (`1h`, `24h`, `7d`) (default: `24h`)

**Response:** `200 OK`
```json
{
  "monitor_id": "550e8400-e29b-41d4-a716-446655440000",
  "period": "24h",
  "average_ms": 245,
  "min_ms": 89,
  "max_ms": 1250,
  "p95_ms": 450,
  "p99_ms": 890
}
```

**Examples:**
```bash
# Get 24-hour response time stats (default)
curl http://localhost:8080/api/v1/monitors/550e8400-e29b-41d4-a716-446655440000/response

# Get 1-hour response time stats
curl http://localhost:8080/api/v1/monitors/550e8400-e29b-41d4-a716-446655440000/response?period=1h

# Get 7-day response time stats
curl http://localhost:8080/api/v1/monitors/550e8400-e29b-41d4-a716-446655440000/response?period=7d
```

## System Health

### Basic Health Check

Check if the application is running.

**Endpoint:** `GET /health`

**Response:** `200 OK`
```json
{
  "status": "ok",
  "timestamp": "2024-01-01T00:00:00Z"
}
```

**Example:**
```bash
curl http://localhost:8080/health
```

### Readiness Check

Check if the application is ready to serve requests (includes dependency checks).

**Endpoint:** `GET /health/ready`

**Response:** `200 OK` (if ready) or `503 Service Unavailable` (if not ready)
```json
{
  "status": "ready",
  "timestamp": "2024-01-01T00:00:00Z",
  "checks": {
    "database": "ok",
    "worker_pool": "ok",
    "scheduler": "ok"
  }
}
```

**Example:**
```bash
curl http://localhost:8080/health/ready
```

### Liveness Check

Check if the application is alive (for Kubernetes liveness probes).

**Endpoint:** `GET /health/live`

**Response:** `200 OK`
```json
{
  "status": "alive",
  "timestamp": "2024-01-01T00:00:00Z"
}
```

**Example:**
```bash
curl http://localhost:8080/health/live
```

### System Metrics

Get detailed system metrics and statistics.

**Endpoint:** `GET /metrics`

**Response:** `200 OK`
```json
{
  "worker_pool": {
    "active_workers": 8,
    "queue_depth": 12,
    "processed_jobs": 15420,
    "is_running": true
  },
  "database": {
    "total_monitors": 25,
    "enabled_monitors": 23,
    "disabled_monitors": 2
  },
  "system": {
    "scheduler_running": true,
    "uptime": "unknown",
    "timestamp": "2024-01-01T00:00:00Z"
  }
}
```

**Example:**
```bash
curl http://localhost:8080/metrics
```

## WebSocket Real-time Updates

The system provides real-time updates via WebSocket connections for immediate notification of health check results and alerts.

### Connection

**Endpoint:** `WS /ws`

**Example (JavaScript):**
```javascript
const ws = new WebSocket('ws://localhost:8080/ws');

ws.onopen = function(event) {
    console.log('WebSocket connected');
};

ws.onmessage = function(event) {
    const message = JSON.parse(event.data);
    console.log('Received:', message);
    
    if (message.type === 'health_check_update') {
        handleHealthCheckUpdate(message.data);
    } else if (message.type === 'alert') {
        handleAlert(message.data);
    }
};

ws.onclose = function(event) {
    console.log('WebSocket disconnected');
    // Implement reconnection logic
    setTimeout(() => {
        connectWebSocket();
    }, 5000);
};

ws.onerror = function(error) {
    console.error('WebSocket error:', error);
};
```

### Message Types

#### Health Check Update

Sent when a health check completes.

```json
{
  "type": "health_check_update",
  "data": {
    "monitor_id": "550e8400-e29b-41d4-a716-446655440000",
    "status": "success",
    "status_code": 200,
    "response_time_ms": 150,
    "checked_at": "2024-01-01T00:00:00Z",
    "error_message": ""
  }
}
```

#### Alert

Sent when an alert is generated.

```json
{
  "type": "alert",
  "data": {
    "monitor_id": "550e8400-e29b-41d4-a716-446655440000",
    "type": "downtime",
    "severity": "critical",
    "message": "Monitor is down - connection timeout",
    "details": {
      "status_code": 0,
      "error": "connection timeout"
    },
    "sent_at": "2024-01-01T00:05:00Z",
    "channels": ["telegram", "email"]
  }
}
```

### Client-side Reconnection

Implement automatic reconnection with exponential backoff:

```javascript
let reconnectAttempts = 0;
const maxReconnectAttempts = 10;

function connectWebSocket() {
    const ws = new WebSocket('ws://localhost:8080/ws');
    
    ws.onopen = function() {
        reconnectAttempts = 0;
        console.log('WebSocket connected');
    };
    
    ws.onclose = function() {
        if (reconnectAttempts < maxReconnectAttempts) {
            const delay = Math.min(1000 * Math.pow(2, reconnectAttempts), 30000);
            setTimeout(() => {
                reconnectAttempts++;
                connectWebSocket();
            }, delay);
        }
    };
}
```

## Data Types

### Monitor

```json
{
  "id": "string (UUID)",
  "name": "string",
  "url": "string (HTTP/HTTPS URL)",
  "check_interval": "integer (1, 5, 15, or 60 minutes)",
  "enabled": "boolean",
  "alert_channels": "array of AlertChannel",
  "created_at": "string (RFC3339 timestamp)",
  "updated_at": "string (RFC3339 timestamp)"
}
```

### AlertChannel

```json
{
  "type": "string (telegram, email, webhook)",
  "config": "object (channel-specific configuration)"
}
```

### HealthCheck

```json
{
  "id": "string (UUID)",
  "monitor_id": "string (UUID)",
  "status": "string (success, failure, timeout)",
  "status_code": "integer (HTTP status code)",
  "response_time_ms": "integer (milliseconds)",
  "ssl_info": "SSLInfo object (optional)",
  "error_message": "string (optional)",
  "checked_at": "string (RFC3339 timestamp)"
}
```

### SSLInfo

```json
{
  "valid": "boolean",
  "expires_at": "string (RFC3339 timestamp)",
  "days_until": "integer",
  "issuer": "string"
}
```

### Alert

```json
{
  "id": "string (UUID)",
  "monitor_id": "string (UUID)",
  "type": "string (downtime, recovery, ssl_expiring, ssl_expired, performance)",
  "severity": "string (info, warning, critical)",
  "message": "string",
  "details": "object (alert-specific details)",
  "sent_at": "string (RFC3339 timestamp)",
  "channels": "array of strings (channel types)"
}
```

## Rate Limits

Currently, the API does not implement rate limiting. In production deployments, consider implementing rate limiting to prevent abuse.

## Pagination

For endpoints that return lists (monitors, health checks, alerts), use the following parameters:
- `limit`: Maximum number of results (default varies by endpoint)
- `offset`: Number of results to skip for pagination

## Date Formats

All timestamps use RFC3339 format: `2024-01-01T00:00:00Z`

## Error Codes

| HTTP Status | Description |
|-------------|-------------|
| 200 | OK - Request successful |
| 201 | Created - Resource created |
| 204 | No Content - Request successful, no content |
| 400 | Bad Request - Invalid parameters |
| 404 | Not Found - Resource not found |
| 500 | Internal Server Error - Server error |
| 503 | Service Unavailable - Service not ready |

## Examples

### Complete Monitor Lifecycle

```bash
# 1. Create a monitor
MONITOR_ID=$(curl -s -X POST http://localhost:8080/api/v1/monitors \
  -H "Content-Type: application/json" \
  -d '{
    "name": "My Website",
    "url": "https://example.com",
    "check_interval": 5,
    "enabled": true
  }' | jq -r '.id')

# 2. Get monitor details
curl http://localhost:8080/api/v1/monitors/$MONITOR_ID

# 3. Wait for some health checks, then get history
curl http://localhost:8080/api/v1/monitors/$MONITOR_ID/checks

# 4. Get uptime metrics
curl http://localhost:8080/api/v1/monitors/$MONITOR_ID/uptime

# 5. Update monitor
curl -X PUT http://localhost:8080/api/v1/monitors/$MONITOR_ID \
  -H "Content-Type: application/json" \
  -d '{"check_interval": 15}'

# 6. Delete monitor
curl -X DELETE http://localhost:8080/api/v1/monitors/$MONITOR_ID
```

### Monitoring Multiple Websites

```bash
# Create multiple monitors
for url in "https://example.com" "https://google.com" "https://github.com"; do
  curl -X POST http://localhost:8080/api/v1/monitors \
    -H "Content-Type: application/json" \
    -d "{
      \"name\": \"Monitor for $url\",
      \"url\": \"$url\",
      \"check_interval\": 5,
      \"enabled\": true
    }"
done

# List all monitors
curl http://localhost:8080/api/v1/monitors
```

This API documentation provides comprehensive coverage of all endpoints, request/response formats, and usage examples for the Uptime Monitoring & Alert System.