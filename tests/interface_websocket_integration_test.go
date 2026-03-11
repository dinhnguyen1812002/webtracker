package tests

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"web-tracker/domain"
	ws "web-tracker/interface/websocket"
)

// TestWebSocketIntegrationWithHealthCheck tests WebSocket broadcasting integration
func TestWebSocketIntegrationWithHealthCheck(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create WebSocket manager
	manager := ws.NewWebSocketManager()
	err := manager.Start(ctx)
	require.NoError(t, err)

	// Create test health check
	healthCheck := &domain.HealthCheck{
		ID:           "test-health-check",
		MonitorID:    "monitor-123",
		Status:       domain.StatusSuccess,
		StatusCode:   200,
		ResponseTime: 150 * time.Millisecond,
		CheckedAt:    time.Now(),
	}

	// Test health check broadcasting
	manager.BroadcastHealthCheckUpdate(healthCheck)

	// Create test alert
	alert := &domain.Alert{
		ID:        "test-alert",
		MonitorID: "monitor-123",
		Type:      domain.AlertTypeDowntime,
		Severity:  domain.SeverityCritical,
		Message:   "Test alert message",
		Details: map[string]interface{}{
			"url":         "https://example.com",
			"status_code": 500,
		},
		SentAt:   time.Now(),
		Channels: []domain.AlertChannelType{domain.AlertChannelEmail},
	}

	// Test alert broadcasting
	manager.BroadcastAlert(alert)

	// Verify no connections initially
	assert.Equal(t, 0, manager.GetConnectionCount())

	// Stop manager
	err = manager.Stop()
	assert.NoError(t, err)
}

// TestWebSocketManagerLifecycle tests the complete lifecycle of WebSocket manager
func TestWebSocketManagerLifecycle(t *testing.T) {
	manager := ws.NewWebSocketManager()

	// Test initial state
	assert.Equal(t, 0, manager.GetConnectionCount())
	assert.Nil(t, manager.GetHandler())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Test start
	err := manager.Start(ctx)
	require.NoError(t, err)

	// Test handler is available after start
	handler := manager.GetHandler()
	assert.NotNil(t, handler)

	// Test connection count
	assert.Equal(t, 0, manager.GetConnectionCount())

	// Test broadcasting without connections (should not panic)
	healthCheck := &domain.HealthCheck{
		ID:        "test-id",
		MonitorID: "monitor-123",
		Status:    domain.StatusFailure,
		CheckedAt: time.Now(),
	}
	manager.BroadcastHealthCheckUpdate(healthCheck)

	alert := &domain.Alert{
		ID:        "alert-id",
		MonitorID: "monitor-123",
		Type:      domain.AlertTypeRecovery,
		Severity:  domain.SeverityInfo,
		Message:   "Recovery alert",
		SentAt:    time.Now(),
	}
	manager.BroadcastAlert(alert)

	// Test stop
	err = manager.Stop()
	assert.NoError(t, err)
}

// TestHubGracefulShutdown tests that the hub shuts down gracefully
func TestHubGracefulShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	hub := ws.NewHub(ctx)

	// Start hub in background
	done := make(chan bool)
	go func() {
		hub.Run()
		done <- true
	}()

	// Give hub time to start
	time.Sleep(10 * time.Millisecond)

	// Cancel context to trigger shutdown
	cancel()

	// Wait for hub to shutdown
	select {
	case <-done:
		// Hub shut down successfully
	case <-time.After(1 * time.Second):
		t.Fatal("Hub did not shut down within timeout")
	}
}

// TestMessageTypes tests different message types
func TestMessageTypes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := ws.NewHub(ctx)

	// Start hub
	go hub.Run()

	// Test health check message
	healthCheck := &domain.HealthCheck{
		ID:           "hc-1",
		MonitorID:    "mon-1",
		Status:       domain.StatusTimeout,
		StatusCode:   0,
		ResponseTime: 30 * time.Second,
		ErrorMessage: "Request timeout",
		CheckedAt:    time.Now(),
	}

	// Should not panic
	hub.BroadcastHealthCheckUpdate(healthCheck)

	// Test SSL alert
	sslAlert := &domain.Alert{
		ID:        "ssl-alert",
		MonitorID: "mon-1",
		Type:      domain.AlertTypeSSLExpiring,
		Severity:  domain.SeverityWarning,
		Message:   "SSL certificate expires in 7 days",
		Details: map[string]interface{}{
			"expires_at": time.Now().Add(7 * 24 * time.Hour),
			"days_until": 7,
		},
		SentAt:   time.Now(),
		Channels: []domain.AlertChannelType{domain.AlertChannelTelegram, domain.AlertChannelWebhook},
	}

	// Should not panic
	hub.BroadcastAlert(sslAlert)

	// Test performance alert
	perfAlert := &domain.Alert{
		ID:        "perf-alert",
		MonitorID: "mon-1",
		Type:      domain.AlertTypePerformance,
		Severity:  domain.SeverityWarning,
		Message:   "Response time exceeds threshold",
		Details: map[string]interface{}{
			"response_time": 5500,
			"threshold":     5000,
		},
		SentAt:   time.Now(),
		Channels: []domain.AlertChannelType{domain.AlertChannelEmail},
	}

	// Should not panic
	hub.BroadcastAlert(perfAlert)

	// Give time for processing
	time.Sleep(10 * time.Millisecond)

	// Verify no connections
	assert.Equal(t, 0, hub.GetConnectionCount())
}
