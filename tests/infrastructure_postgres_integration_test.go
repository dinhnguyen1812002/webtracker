//go:build integration
// +build integration

package tests

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"web-tracker/domain"
	"web-tracker/infrastructure/postgres"
)

// TestIntegration_MonitorRepository_FullCRUD tests the full CRUD lifecycle
func TestIntegration_MonitorRepository_FullCRUD(t *testing.T) {
	ctx := context.Background()

	// Get database config from environment or use defaults
	config := postgres.DefaultPoolConfig()
	if host := os.Getenv("TEST_DB_HOST"); host != "" {
		config.Host = host
	}
	if dbName := os.Getenv("TEST_DB_NAME"); dbName != "" {
		config.Database = dbName
	}

	pool, err := postgres.NewPool(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	// Run migrations
	if err := postgres.RunMigrations(ctx, pool); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	repo := postgres.NewMonitorRepository(pool)

	// Test Create
	monitor := &domain.Monitor{
		ID:            "integration-test-1",
		Name:          "Integration Test Monitor",
		URL:           "https://example.com",
		CheckInterval: domain.CheckInterval5Min,
		Enabled:       true,
		AlertChannels: []domain.AlertChannel{
			{
				Type: domain.AlertChannelEmail,
				Config: map[string]string{
					"email": "test@example.com",
				},
			},
			{
				Type: domain.AlertChannelTelegram,
				Config: map[string]string{
					"chat_id": "123456",
					"token":   "test-token",
				},
			},
		},
	}

	err = repo.Create(ctx, monitor)
	if err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}

	// Test GetByID
	retrieved, err := repo.GetByID(ctx, monitor.ID)
	if err != nil {
		t.Fatalf("Failed to get monitor: %v", err)
	}

	if retrieved.Name != monitor.Name {
		t.Errorf("Expected name %s, got %s", monitor.Name, retrieved.Name)
	}
	if retrieved.URL != monitor.URL {
		t.Errorf("Expected URL %s, got %s", monitor.URL, retrieved.URL)
	}
	if retrieved.CheckInterval != monitor.CheckInterval {
		t.Errorf("Expected check interval %v, got %v", monitor.CheckInterval, retrieved.CheckInterval)
	}
	if len(retrieved.AlertChannels) != len(monitor.AlertChannels) {
		t.Errorf("Expected %d alert channels, got %d", len(monitor.AlertChannels), len(retrieved.AlertChannels))
	}

	// Test List
	filters := domain.ListFilters{
		Enabled: boolPtr(true),
		Limit:   10,
		Offset:  0,
	}
	monitors, err := repo.List(ctx, filters)
	if err != nil {
		t.Fatalf("Failed to list monitors: %v", err)
	}

	found := false
	for _, m := range monitors {
		if m.ID == monitor.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("Created monitor not found in list")
	}

	// Test Update
	monitor.Name = "Updated Monitor Name"
	monitor.CheckInterval = domain.CheckInterval15Min
	monitor.Enabled = false

	err = repo.Update(ctx, monitor)
	if err != nil {
		t.Fatalf("Failed to update monitor: %v", err)
	}

	updated, err := repo.GetByID(ctx, monitor.ID)
	if err != nil {
		t.Fatalf("Failed to get updated monitor: %v", err)
	}

	if updated.Name != "Updated Monitor Name" {
		t.Errorf("Expected updated name, got %s", updated.Name)
	}
	if updated.CheckInterval != domain.CheckInterval15Min {
		t.Errorf("Expected updated check interval, got %v", updated.CheckInterval)
	}
	if updated.Enabled {
		t.Error("Expected monitor to be disabled")
	}

	// Test Delete
	err = repo.Delete(ctx, monitor.ID)
	if err != nil {
		t.Fatalf("Failed to delete monitor: %v", err)
	}

	// Verify deletion
	_, err = repo.GetByID(ctx, monitor.ID)
	if err == nil {
		t.Error("Expected error when getting deleted monitor")
	}
}

// TestIntegration_MonitorRepository_ListFilters tests list filtering
func TestIntegration_MonitorRepository_ListFilters(t *testing.T) {
	ctx := context.Background()

	config := postgres.DefaultPoolConfig()
	pool, err := postgres.NewPool(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	if err := postgres.RunMigrations(ctx, pool); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	repo := postgres.NewMonitorRepository(pool)

	// Create test monitors
	monitors := []*domain.Monitor{
		{
			ID:            "filter-test-1",
			Name:          "Enabled Monitor 1",
			URL:           "https://example1.com",
			CheckInterval: domain.CheckInterval5Min,
			Enabled:       true,
		},
		{
			ID:            "filter-test-2",
			Name:          "Enabled Monitor 2",
			URL:           "https://example2.com",
			CheckInterval: domain.CheckInterval5Min,
			Enabled:       true,
		},
		{
			ID:            "filter-test-3",
			Name:          "Disabled Monitor",
			URL:           "https://example3.com",
			CheckInterval: domain.CheckInterval5Min,
			Enabled:       false,
		},
	}

	for _, m := range monitors {
		if err := repo.Create(ctx, m); err != nil {
			t.Fatalf("Failed to create monitor: %v", err)
		}
	}

	// Cleanup
	defer func() {
		for _, m := range monitors {
			repo.Delete(ctx, m.ID)
		}
	}()

	// Test filter by enabled
	enabledFilters := domain.ListFilters{
		Enabled: boolPtr(true),
	}
	enabledMonitors, err := repo.List(ctx, enabledFilters)
	if err != nil {
		t.Fatalf("Failed to list enabled monitors: %v", err)
	}

	enabledCount := 0
	for _, m := range enabledMonitors {
		if m.ID == "filter-test-1" || m.ID == "filter-test-2" {
			enabledCount++
		}
	}
	if enabledCount != 2 {
		t.Errorf("Expected 2 enabled test monitors, got %d", enabledCount)
	}

	// Test pagination
	paginatedFilters := domain.ListFilters{
		Limit:  1,
		Offset: 0,
	}
	page1, err := repo.List(ctx, paginatedFilters)
	if err != nil {
		t.Fatalf("Failed to list with pagination: %v", err)
	}

	if len(page1) != 1 {
		t.Errorf("Expected 1 monitor in page, got %d", len(page1))
	}
}

// TestIntegration_MonitorRepository_Concurrency tests concurrent operations
func TestIntegration_MonitorRepository_Concurrency(t *testing.T) {
	ctx := context.Background()

	config := postgres.DefaultPoolConfig()
	pool, err := postgres.NewPool(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	if err := postgres.RunMigrations(ctx, pool); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	repo := postgres.NewMonitorRepository(pool)

	// Create monitors concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(index int) {
			monitor := &domain.Monitor{
				ID:            fmt.Sprintf("concurrent-test-%d", index),
				Name:          fmt.Sprintf("Concurrent Monitor %d", index),
				URL:           "https://example.com",
				CheckInterval: domain.CheckInterval5Min,
				Enabled:       true,
			}
			err := repo.Create(ctx, monitor)
			if err != nil {
				t.Errorf("Failed to create monitor %d: %v", index, err)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Cleanup
	for i := 0; i < 10; i++ {
		repo.Delete(ctx, fmt.Sprintf("concurrent-test-%d", i))
	}
}

func boolPtr(b bool) *bool {
	return &b
}

// TestIntegration_AlertRepository_FullCRUD tests the full CRUD lifecycle for alerts
func TestIntegration_AlertRepository_FullCRUD(t *testing.T) {
	ctx := context.Background()

	// Get database config from environment or use defaults
	config := postgres.DefaultPoolConfig()
	if host := os.Getenv("TEST_DB_HOST"); host != "" {
		config.Host = host
	}
	if dbName := os.Getenv("TEST_DB_NAME"); dbName != "" {
		config.Database = dbName
	}

	pool, err := postgres.NewPool(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	// Run migrations
	if err := postgres.RunMigrations(ctx, pool); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	monitorRepo := postgres.NewMonitorRepository(pool)
	alertRepo := postgres.NewAlertRepository(pool)

	// Create a test monitor first
	monitor := &domain.Monitor{
		ID:            "alert-test-monitor-1",
		Name:          "Alert Test Monitor",
		URL:           "https://example.com",
		CheckInterval: domain.CheckInterval5Min,
		Enabled:       true,
	}

	err = monitorRepo.Create(ctx, monitor)
	if err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}
	defer monitorRepo.Delete(ctx, monitor.ID)

	// Test Create Alert
	alert := &domain.Alert{
		ID:        "alert-test-1",
		MonitorID: monitor.ID,
		Type:      domain.AlertTypeDowntime,
		Severity:  domain.SeverityCritical,
		Message:   "Monitor is down",
		Details: map[string]interface{}{
			"status_code": 500,
			"error":       "Internal Server Error",
		},
		Channels: []domain.AlertChannelType{
			domain.AlertChannelEmail,
			domain.AlertChannelTelegram,
		},
	}

	err = alertRepo.Create(ctx, alert)
	if err != nil {
		t.Fatalf("Failed to create alert: %v", err)
	}

	// Test GetByMonitorID
	alerts, err := alertRepo.GetByMonitorID(ctx, monitor.ID, 10)
	if err != nil {
		t.Fatalf("Failed to get alerts by monitor ID: %v", err)
	}

	if len(alerts) != 1 {
		t.Fatalf("Expected 1 alert, got %d", len(alerts))
	}

	retrieved := alerts[0]
	if retrieved.ID != alert.ID {
		t.Errorf("Expected alert ID %s, got %s", alert.ID, retrieved.ID)
	}
	if retrieved.Type != alert.Type {
		t.Errorf("Expected alert type %s, got %s", alert.Type, retrieved.Type)
	}
	if retrieved.Severity != alert.Severity {
		t.Errorf("Expected severity %s, got %s", alert.Severity, retrieved.Severity)
	}
	if retrieved.Message != alert.Message {
		t.Errorf("Expected message %s, got %s", alert.Message, retrieved.Message)
	}
	if len(retrieved.Channels) != len(alert.Channels) {
		t.Errorf("Expected %d channels, got %d", len(alert.Channels), len(retrieved.Channels))
	}

	// Test GetLastAlertTime
	lastTime, err := alertRepo.GetLastAlertTime(ctx, monitor.ID, domain.AlertTypeDowntime)
	if err != nil {
		t.Fatalf("Failed to get last alert time: %v", err)
	}

	if lastTime == nil {
		t.Fatal("Expected last alert time, got nil")
	}

	// Test GetLastAlertTime for non-existent alert type
	lastTimeSSL, err := alertRepo.GetLastAlertTime(ctx, monitor.ID, domain.AlertTypeSSLExpiring)
	if err != nil {
		t.Fatalf("Failed to get last alert time for SSL: %v", err)
	}

	if lastTimeSSL != nil {
		t.Error("Expected nil for non-existent alert type")
	}
}

// TestIntegration_AlertRepository_DateRange tests date range queries
func TestIntegration_AlertRepository_DateRange(t *testing.T) {
	ctx := context.Background()

	config := postgres.DefaultPoolConfig()
	pool, err := postgres.NewPool(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	if err := postgres.RunMigrations(ctx, pool); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	monitorRepo := postgres.NewMonitorRepository(pool)
	alertRepo := postgres.NewAlertRepository(pool)

	// Create a test monitor
	monitor := &domain.Monitor{
		ID:            "alert-test-monitor-2",
		Name:          "Alert Test Monitor 2",
		URL:           "https://example.com",
		CheckInterval: domain.CheckInterval5Min,
		Enabled:       true,
	}

	err = monitorRepo.Create(ctx, monitor)
	if err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}
	defer monitorRepo.Delete(ctx, monitor.ID)

	// Create alerts with different timestamps
	now := time.Now()
	alerts := []*domain.Alert{
		{
			ID:        "alert-range-1",
			MonitorID: monitor.ID,
			Type:      domain.AlertTypeDowntime,
			Severity:  domain.SeverityCritical,
			Message:   "Alert 1",
			Channels:  []domain.AlertChannelType{domain.AlertChannelEmail},
			SentAt:    now.Add(-2 * time.Hour),
		},
		{
			ID:        "alert-range-2",
			MonitorID: monitor.ID,
			Type:      domain.AlertTypeRecovery,
			Severity:  domain.SeverityInfo,
			Message:   "Alert 2",
			Channels:  []domain.AlertChannelType{domain.AlertChannelEmail},
			SentAt:    now.Add(-1 * time.Hour),
		},
		{
			ID:        "alert-range-3",
			MonitorID: monitor.ID,
			Type:      domain.AlertTypeSSLExpiring,
			Severity:  domain.SeverityWarning,
			Message:   "Alert 3",
			Channels:  []domain.AlertChannelType{domain.AlertChannelEmail},
			SentAt:    now,
		},
	}

	for _, a := range alerts {
		if err := alertRepo.Create(ctx, a); err != nil {
			t.Fatalf("Failed to create alert: %v", err)
		}
	}

	// Test date range query
	start := now.Add(-90 * time.Minute)
	end := now.Add(-30 * time.Minute)

	rangeAlerts, err := alertRepo.GetByDateRange(ctx, monitor.ID, start, end)
	if err != nil {
		t.Fatalf("Failed to get alerts by date range: %v", err)
	}

	if len(rangeAlerts) != 1 {
		t.Errorf("Expected 1 alert in range, got %d", len(rangeAlerts))
	}

	if len(rangeAlerts) > 0 && rangeAlerts[0].ID != "alert-range-2" {
		t.Errorf("Expected alert-range-2, got %s", rangeAlerts[0].ID)
	}

	// Test getting all alerts
	allAlerts, err := alertRepo.GetByMonitorID(ctx, monitor.ID, 0)
	if err != nil {
		t.Fatalf("Failed to get all alerts: %v", err)
	}

	if len(allAlerts) != 3 {
		t.Errorf("Expected 3 alerts, got %d", len(allAlerts))
	}

	// Verify ordering (most recent first)
	if len(allAlerts) >= 2 {
		if allAlerts[0].SentAt.Before(allAlerts[1].SentAt) {
			t.Error("Alerts not ordered by sent_at DESC")
		}
	}
}

// TestIntegration_AlertRepository_Limit tests limit functionality
func TestIntegration_AlertRepository_Limit(t *testing.T) {
	ctx := context.Background()

	config := postgres.DefaultPoolConfig()
	pool, err := postgres.NewPool(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	if err := postgres.RunMigrations(ctx, pool); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	monitorRepo := postgres.NewMonitorRepository(pool)
	alertRepo := postgres.NewAlertRepository(pool)

	// Create a test monitor
	monitor := &domain.Monitor{
		ID:            "alert-test-monitor-3",
		Name:          "Alert Test Monitor 3",
		URL:           "https://example.com",
		CheckInterval: domain.CheckInterval5Min,
		Enabled:       true,
	}

	err = monitorRepo.Create(ctx, monitor)
	if err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}
	defer monitorRepo.Delete(ctx, monitor.ID)

	// Create multiple alerts
	for i := 0; i < 5; i++ {
		alert := &domain.Alert{
			ID:        fmt.Sprintf("alert-limit-%d", i),
			MonitorID: monitor.ID,
			Type:      domain.AlertTypeDowntime,
			Severity:  domain.SeverityCritical,
			Message:   fmt.Sprintf("Alert %d", i),
			Channels:  []domain.AlertChannelType{domain.AlertChannelEmail},
		}
		if err := alertRepo.Create(ctx, alert); err != nil {
			t.Fatalf("Failed to create alert: %v", err)
		}
	}

	// Test with limit
	limitedAlerts, err := alertRepo.GetByMonitorID(ctx, monitor.ID, 3)
	if err != nil {
		t.Fatalf("Failed to get limited alerts: %v", err)
	}

	if len(limitedAlerts) != 3 {
		t.Errorf("Expected 3 alerts with limit, got %d", len(limitedAlerts))
	}

	// Test without limit
	allAlerts, err := alertRepo.GetByMonitorID(ctx, monitor.ID, 0)
	if err != nil {
		t.Fatalf("Failed to get all alerts: %v", err)
	}

	if len(allAlerts) != 5 {
		t.Errorf("Expected 5 alerts without limit, got %d", len(allAlerts))
	}
}
