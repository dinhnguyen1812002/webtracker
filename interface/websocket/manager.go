package websocket

import (
	"context"

	"web-tracker/domain"
)

// Manager defines the interface for WebSocket management
type Manager interface {
	// BroadcastHealthCheckUpdate broadcasts a health check update to all connected clients
	BroadcastHealthCheckUpdate(healthCheck *domain.HealthCheck)

	// BroadcastAlert broadcasts an alert to all connected clients
	BroadcastAlert(alert *domain.Alert)

	// GetConnectionCount returns the number of active WebSocket connections
	GetConnectionCount() int

	// Start starts the WebSocket manager
	Start(ctx context.Context) error

	// Stop stops the WebSocket manager
	Stop() error

	// GetHandler returns the WebSocket HTTP handler
	GetHandler() *Handler
}

// WebSocketManager implements the Manager interface
type WebSocketManager struct {
	hub    *Hub
	ctx    context.Context
	cancel context.CancelFunc
}

// NewWebSocketManager creates a new WebSocket manager
func NewWebSocketManager() *WebSocketManager {
	return &WebSocketManager{}
}

// Start starts the WebSocket manager
func (m *WebSocketManager) Start(ctx context.Context) error {
	m.ctx, m.cancel = context.WithCancel(ctx)
	m.hub = NewHub(m.ctx)

	// Start the hub in a goroutine
	go m.hub.Run()

	return nil
}

// Stop stops the WebSocket manager
func (m *WebSocketManager) Stop() error {
	if m.cancel != nil {
		m.cancel()
	}
	return nil
}

// BroadcastHealthCheckUpdate broadcasts a health check update to all connected clients
func (m *WebSocketManager) BroadcastHealthCheckUpdate(healthCheck *domain.HealthCheck) {
	if m.hub != nil {
		m.hub.BroadcastHealthCheckUpdate(healthCheck)
	}
}

// BroadcastAlert broadcasts an alert to all connected clients
func (m *WebSocketManager) BroadcastAlert(alert *domain.Alert) {
	if m.hub != nil {
		m.hub.BroadcastAlert(alert)
	}
}

// GetConnectionCount returns the number of active WebSocket connections
func (m *WebSocketManager) GetConnectionCount() int {
	if m.hub != nil {
		return m.hub.GetConnectionCount()
	}
	return 0
}

// GetHandler returns the WebSocket HTTP handler
func (m *WebSocketManager) GetHandler() *Handler {
	if m.hub != nil {
		return NewHandler(m.hub)
	}
	return nil
}
