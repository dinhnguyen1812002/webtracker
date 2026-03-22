package tests

import (
	"context"
	"testing"
	"time"

	"web-tracker/domain"
	"web-tracker/usecase"
)

func TestMonitorService_CreateMonitor(t *testing.T) {
	repo := NewMockMonitorRepository()
	scheduler := NewMockScheduler()
	service := usecase.NewMonitorService(repo, scheduler)

	ctx := context.Background()

	req := usecase.CreateMonitorRequest{
		Name:          "Test Monitor",
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
		},
	}

	monitor, err := service.CreateMonitor(ctx, req)
	if err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}

	if monitor.ID == "" {
		t.Error("Monitor ID should not be empty")
	}

	if monitor.Name != req.Name {
		t.Errorf("Expected name %s, got %s", req.Name, monitor.Name)
	}

	if monitor.URL != req.URL {
		t.Errorf("Expected URL %s, got %s", req.URL, monitor.URL)
	}

	if monitor.CheckInterval != req.CheckInterval {
		t.Errorf("Expected check interval %v, got %v", req.CheckInterval, monitor.CheckInterval)
	}

	if monitor.Enabled != req.Enabled {
		t.Errorf("Expected enabled %v, got %v", req.Enabled, monitor.Enabled)
	}

	// Verify monitor was scheduled since it's enabled
	if _, scheduled := scheduler.scheduledMonitors[monitor.ID]; !scheduled {
		t.Error("Monitor should have been scheduled")
	}
}

func TestMonitorService_CreateMonitor_ValidationError(t *testing.T) {
	repo := NewMockMonitorRepository()
	scheduler := NewMockScheduler()
	service := usecase.NewMonitorService(repo, scheduler)

	ctx := context.Background()

	// Test invalid URL
	req := usecase.CreateMonitorRequest{
		Name:          "Test Monitor",
		URL:           "invalid-url",
		CheckInterval: domain.CheckInterval5Min,
		Enabled:       true,
	}

	_, err := service.CreateMonitor(ctx, req)
	if err == nil {
		t.Error("Expected validation error for invalid URL")
	}

	// Test invalid check interval
	req = usecase.CreateMonitorRequest{
		Name:          "Test Monitor",
		URL:           "https://example.com",
		CheckInterval: 2 * time.Minute, // Invalid interval
		Enabled:       true,
	}

	_, err = service.CreateMonitor(ctx, req)
	if err == nil {
		t.Error("Expected validation error for invalid check interval")
	}
}

func TestMonitorService_GetMonitor(t *testing.T) {
	repo := NewMockMonitorRepository()
	scheduler := NewMockScheduler()
	service := usecase.NewMonitorService(repo, scheduler)

	ctx := context.Background()

	// Create a monitor first
	req := usecase.CreateMonitorRequest{
		Name:          "Test Monitor",
		URL:           "https://example.com",
		CheckInterval: domain.CheckInterval5Min,
		Enabled:       true,
	}

	createdMonitor, err := service.CreateMonitor(ctx, req)
	if err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}

	// Get the monitor
	retrievedMonitor, err := service.GetMonitor(ctx, createdMonitor.ID)
	if err != nil {
		t.Fatalf("GetMonitor failed: %v", err)
	}

	if retrievedMonitor.ID != createdMonitor.ID {
		t.Errorf("Expected ID %s, got %s", createdMonitor.ID, retrievedMonitor.ID)
	}

	if retrievedMonitor.Name != createdMonitor.Name {
		t.Errorf("Expected name %s, got %s", createdMonitor.Name, retrievedMonitor.Name)
	}
}

func TestMonitorService_UpdateMonitor(t *testing.T) {
	repo := NewMockMonitorRepository()
	scheduler := NewMockScheduler()
	service := usecase.NewMonitorService(repo, scheduler)

	ctx := context.Background()

	// Create a monitor first
	req := usecase.CreateMonitorRequest{
		Name:          "Test Monitor",
		URL:           "https://example.com",
		CheckInterval: domain.CheckInterval5Min,
		Enabled:       true,
	}

	createdMonitor, err := service.CreateMonitor(ctx, req)
	if err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}

	// Update the monitor
	newName := "Updated Monitor"
	newURL := "https://updated.example.com"
	updateReq := usecase.UpdateMonitorRequest{
		Name: &newName,
		URL:  &newURL,
	}

	updatedMonitor, err := service.UpdateMonitor(ctx, createdMonitor.ID, updateReq)
	if err != nil {
		t.Fatalf("UpdateMonitor failed: %v", err)
	}

	if updatedMonitor.Name != newName {
		t.Errorf("Expected name %s, got %s", newName, updatedMonitor.Name)
	}

	if updatedMonitor.URL != newURL {
		t.Errorf("Expected URL %s, got %s", newURL, updatedMonitor.URL)
	}

	// Verify monitor was rescheduled due to URL change
	if scheduledMonitor, exists := scheduler.scheduledMonitors[createdMonitor.ID]; !exists {
		t.Error("Monitor should have been rescheduled")
	} else if scheduledMonitor.URL != newURL {
		t.Error("Scheduled monitor should have updated URL")
	}
}

func TestMonitorService_DeleteMonitor(t *testing.T) {
	repo := NewMockMonitorRepository()
	scheduler := NewMockScheduler()
	service := usecase.NewMonitorService(repo, scheduler)

	ctx := context.Background()

	// Create a monitor first
	req := usecase.CreateMonitorRequest{
		Name:          "Test Monitor",
		URL:           "https://example.com",
		CheckInterval: domain.CheckInterval5Min,
		Enabled:       true,
	}

	createdMonitor, err := service.CreateMonitor(ctx, req)
	if err != nil {
		t.Fatalf("CreateMonitor failed: %v", err)
	}

	// Delete the monitor
	err = service.DeleteMonitor(ctx, createdMonitor.ID)
	if err != nil {
		t.Fatalf("DeleteMonitor failed: %v", err)
	}

	// Verify monitor was deleted from repository
	_, err = service.GetMonitor(ctx, createdMonitor.ID)
	if err == nil {
		t.Error("Monitor should have been deleted")
	}

	// Verify monitor was unscheduled
	if _, scheduled := scheduler.scheduledMonitors[createdMonitor.ID]; scheduled {
		t.Error("Monitor should have been unscheduled")
	}

	if !scheduler.unscheduledMonitors[createdMonitor.ID] {
		t.Error("Monitor should be marked as unscheduled")
	}
}

func TestMonitorService_ListMonitors(t *testing.T) {
	repo := NewMockMonitorRepository()
	scheduler := NewMockScheduler()
	service := usecase.NewMonitorService(repo, scheduler)

	ctx := context.Background()

	// Create multiple monitors
	req1 := usecase.CreateMonitorRequest{
		Name:          "Monitor 1",
		URL:           "https://example1.com",
		CheckInterval: domain.CheckInterval5Min,
		Enabled:       true,
	}

	req2 := usecase.CreateMonitorRequest{
		Name:          "Monitor 2",
		URL:           "https://example2.com",
		CheckInterval: domain.CheckInterval15Min,
		Enabled:       false,
	}

	_, err := service.CreateMonitor(ctx, req1)
	if err != nil {
		t.Fatalf("CreateMonitor 1 failed: %v", err)
	}

	_, err = service.CreateMonitor(ctx, req2)
	if err != nil {
		t.Fatalf("CreateMonitor 2 failed: %v", err)
	}

	// List all monitors
	monitors, err := service.ListMonitors(ctx, domain.ListFilters{})
	if err != nil {
		t.Fatalf("ListMonitors failed: %v", err)
	}

	if len(monitors) != 2 {
		t.Errorf("Expected 2 monitors, got %d", len(monitors))
	}

	// List only enabled monitors
	enabled := true
	monitors, err = service.ListMonitors(ctx, domain.ListFilters{Enabled: &enabled})
	if err != nil {
		t.Fatalf("ListMonitors with filter failed: %v", err)
	}

	if len(monitors) != 1 {
		t.Errorf("Expected 1 enabled monitor, got %d", len(monitors))
	}

	if monitors[0].Name != "Monitor 1" {
		t.Errorf("Expected Monitor 1, got %s", monitors[0].Name)
	}
}
