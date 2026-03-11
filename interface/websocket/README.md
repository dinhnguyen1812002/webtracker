# WebSocket Implementation

This package provides real-time WebSocket functionality for the uptime monitoring system, enabling instant updates to connected dashboard clients.

## Features

- **Real-time Health Check Updates**: Broadcasts health check results to all connected clients within 2 seconds
- **Real-time Alert Notifications**: Broadcasts alerts to all connected clients within 2 seconds  
- **Automatic Reconnection**: Client-side reconnection with exponential backoff
- **Connection Management**: Maintains registry of active connections with cleanup
- **Graceful Shutdown**: Proper cleanup of connections and resources

## Architecture

### Server Components

- **Hub**: Central message broadcaster that manages connections and distributes messages
- **Handler**: HTTP handler for WebSocket connection upgrades
- **Manager**: High-level interface for WebSocket management and integration
- **Connection**: Individual WebSocket connection wrapper with read/write pumps

### Client Components

- **WebSocketClient**: Low-level WebSocket client with reconnection logic
- **UptimeMonitorWebSocket**: High-level client specifically for uptime monitoring

## Usage

### Server Integration

```go
// Create WebSocket manager
websocketManager := websocket.NewWebSocketManager()
err := websocketManager.Start(ctx)

// Integrate with services
healthCheckService := usecase.NewHealthCheckService(
    httpClient, healthCheckRepo, monitorRepo, 
    redisClient, alertService, websocketManager,
)

alertService := usecase.NewAlertService(
    alertRepo, monitorRepo, rateLimiter, 
    deliveryService, websocketManager,
)

// Add to HTTP router
server := http.NewServer(config, ..., websocketManager)
```

### Client Usage

```javascript
// Create WebSocket client
const wsClient = new UptimeMonitorWebSocket({
    enableLogging: true,
    maxReconnectAttempts: 10
});

// Handle connection events
wsClient.onConnection((state, event) => {
    console.log('Connection state:', state);
});

// Handle health check updates
wsClient.on('health_check_update', (data) => {
    console.log('Health check update:', data);
    updateMonitorStatus(data.monitor_id, data.status);
});

// Handle alerts
wsClient.on('alert', (data) => {
    console.log('Alert received:', data);
    showAlert(data.type, data.message);
});

// Connect
wsClient.connect();
```

## Message Format

### Health Check Update
```json
{
  "type": "health_check_update",
  "data": {
    "monitor_id": "uuid",
    "status": "success|failure|timeout",
    "status_code": 200,
    "response_time_ms": 150,
    "checked_at": "2024-01-01T00:00:00Z",
    "error_message": "optional error"
  }
}
```

### Alert
```json
{
  "type": "alert", 
  "data": {
    "monitor_id": "uuid",
    "type": "downtime|recovery|ssl_expiring|ssl_expired|performance",
    "severity": "info|warning|critical",
    "message": "Alert message",
    "details": {},
    "sent_at": "2024-01-01T00:00:00Z",
    "channels": ["email", "telegram", "webhook"]
  }
}
```

## Client Reconnection

The client implements automatic reconnection with exponential backoff:

- **Initial delay**: 1 second
- **Backoff multiplier**: 1.5x
- **Maximum delay**: 30 seconds  
- **Maximum attempts**: 10 (configurable)

## Requirements Satisfied

- **9.1**: WebSocket connection establishment on `/ws`
- **9.2**: Health check updates broadcast within 2 seconds
- **9.3**: Alert broadcasts within 2 seconds
- **9.4**: Automatic client reconnection with exponential backoff

## Testing

Run tests with:
```bash
go test ./interface/websocket/... -v
```

## Files

- `handler.go` - WebSocket connection handling and message broadcasting
- `manager.go` - High-level WebSocket management interface
- `client.js` - Client-side JavaScript WebSocket implementation
- `example.html` - Example HTML page demonstrating client usage
- `*_test.go` - Comprehensive test suite
- `README.md` - This documentation