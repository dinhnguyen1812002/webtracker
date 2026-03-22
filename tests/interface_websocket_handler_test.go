package tests

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"web-tracker/domain"
	ws "web-tracker/interface/websocket"
)

func TestNewHub(t *testing.T) {
	ctx := context.Background()
	hub := ws.NewHub(ctx)

	assert.NotNil(t, hub)
	assert.Equal(t, 0, hub.GetConnectionCount())
}

func TestHubBroadcastHealthCheckUpdate(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := ws.NewHub(ctx)

	// Start hub in background
	go hub.Run()

	// Create test health check
	healthCheck := &domain.HealthCheck{
		ID:           "test-id",
		MonitorID:    "monitor-123",
		Status:       domain.StatusSuccess,
		StatusCode:   200,
		ResponseTime: 150 * time.Millisecond,
		CheckedAt:    time.Now(),
	}

	// Test broadcasting (should not panic)
	hub.BroadcastHealthCheckUpdate(healthCheck)

	// Give some time for processing
	time.Sleep(10 * time.Millisecond)

	// Verify connection count is 0 (no connections)
	assert.Equal(t, 0, hub.GetConnectionCount())
}

func TestHubBroadcastAlert(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := ws.NewHub(ctx)

	// Start hub in background
	go hub.Run()

	// Create test alert
	alert := &domain.Alert{
		ID:        "alert-123",
		MonitorID: "monitor-123",
		Type:      domain.AlertTypeDowntime,
		Severity:  domain.SeverityCritical,
		Message:   "Test alert",
		Details:   map[string]interface{}{"test": "data"},
		SentAt:    time.Now(),
		Channels:  []domain.AlertChannelType{domain.AlertChannelEmail},
	}

	// Test broadcasting (should not panic)
	hub.BroadcastAlert(alert)

	// Give some time for processing
	time.Sleep(10 * time.Millisecond)

	// Verify connection count is 0 (no connections)
	assert.Equal(t, 0, hub.GetConnectionCount())
}

func TestWebSocketManager(t *testing.T) {
	manager := ws.NewWebSocketManager()
	assert.NotNil(t, manager)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Test start
	err := manager.Start(ctx)
	assert.NoError(t, err)

	// Test connection count
	count := manager.GetConnectionCount()
	assert.Equal(t, 0, count)

	// Test health check broadcast (should not panic)
	healthCheck := &domain.HealthCheck{
		ID:           "test-id",
		MonitorID:    "monitor-123",
		Status:       domain.StatusSuccess,
		StatusCode:   200,
		ResponseTime: 150 * time.Millisecond,
		CheckedAt:    time.Now(),
	}
	manager.BroadcastHealthCheckUpdate(healthCheck)

	// Test alert broadcast (should not panic)
	alert := &domain.Alert{
		ID:        "alert-123",
		MonitorID: "monitor-123",
		Type:      domain.AlertTypeDowntime,
		Severity:  domain.SeverityCritical,
		Message:   "Test alert",
		Details:   map[string]interface{}{"test": "data"},
		SentAt:    time.Now(),
		Channels:  []domain.AlertChannelType{domain.AlertChannelEmail},
	}
	manager.BroadcastAlert(alert)

	// Test get handler
	handler := manager.GetHandler()
	assert.NotNil(t, handler)

	// Test stop
	err = manager.Stop()
	assert.NoError(t, err)
}

func TestMessageSerialization(t *testing.T) {
	// Test health check update message
	healthCheck := &domain.HealthCheck{
		ID:           "test-id",
		MonitorID:    "monitor-123",
		Status:       domain.StatusSuccess,
		StatusCode:   200,
		ResponseTime: 150 * time.Millisecond,
		CheckedAt:    time.Now(),
	}

	data := ws.HealthCheckUpdateData{
		MonitorID:    healthCheck.MonitorID,
		Status:       string(healthCheck.Status),
		StatusCode:   healthCheck.StatusCode,
		ResponseTime: healthCheck.ResponseTime.Milliseconds(),
		CheckedAt:    healthCheck.CheckedAt,
		ErrorMessage: healthCheck.ErrorMessage,
	}

	message := ws.Message{
		Type: "health_check_update",
		Data: data,
	}

	messageBytes, err := json.Marshal(message)
	require.NoError(t, err)
	assert.Contains(t, string(messageBytes), "health_check_update")
	assert.Contains(t, string(messageBytes), "monitor-123")

	// Test alert message
	alert := &domain.Alert{
		ID:        "alert-123",
		MonitorID: "monitor-123",
		Type:      domain.AlertTypeDowntime,
		Severity:  domain.SeverityCritical,
		Message:   "Test alert",
		Details:   map[string]interface{}{"test": "data"},
		SentAt:    time.Now(),
		Channels:  []domain.AlertChannelType{domain.AlertChannelEmail},
	}

	alertData := ws.AlertData{
		MonitorID: alert.MonitorID,
		Type:      alert.Type,
		Severity:  alert.Severity,
		Message:   alert.Message,
		Details:   alert.Details,
		SentAt:    alert.SentAt,
		Channels:  alert.Channels,
	}

	alertMessage := ws.Message{
		Type: "alert",
		Data: alertData,
	}

	alertMessageBytes, err := json.Marshal(alertMessage)
	require.NoError(t, err)
	assert.Contains(t, string(alertMessageBytes), "alert")
	assert.Contains(t, string(alertMessageBytes), "monitor-123")
	assert.Contains(t, string(alertMessageBytes), "downtime")
}
