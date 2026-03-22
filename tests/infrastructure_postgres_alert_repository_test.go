package tests

import (
	"context"
	"testing"
	"time"

	"web-tracker/domain"
	"web-tracker/infrastructure/postgres"
)

// TestAlertRepository_Create_Validation tests validation in Create method
func TestAlertRepository_Create_Validation(t *testing.T) {
	repo := postgres.NewAlertRepository(nil) // No pool needed for validation tests
	ctx := context.Background()

	tests := []struct {
		name    string
		alert   *domain.Alert
		wantErr bool
		errMsg  string
	}{
		{
			name:    "nil alert",
			alert:   nil,
			wantErr: true,
			errMsg:  "alert cannot be nil",
		},
		{
			name: "empty alert ID",
			alert: &domain.Alert{
				MonitorID: "monitor-1",
				Type:      domain.AlertTypeDowntime,
				Severity:  domain.SeverityCritical,
				Message:   "Test",
			},
			wantErr: true,
			errMsg:  "alert ID is required",
		},
		{
			name: "empty monitor ID",
			alert: &domain.Alert{
				ID:       "alert-1",
				Type:     domain.AlertTypeDowntime,
				Severity: domain.SeverityCritical,
				Message:  "Test",
			},
			wantErr: true,
			errMsg:  "monitor ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.Create(ctx, tt.alert)
			if (err != nil) != tt.wantErr {
				t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" && err.Error() != tt.errMsg {
				t.Errorf("Create() error message = %v, want %v", err.Error(), tt.errMsg)
			}
		})
	}
}

// TestAlertRepository_GetByMonitorID_Validation tests validation in GetByMonitorID method
func TestAlertRepository_GetByMonitorID_Validation(t *testing.T) {
	repo := postgres.NewAlertRepository(nil)
	ctx := context.Background()

	_, err := repo.GetByMonitorID(ctx, "", 10)
	if err == nil {
		t.Error("Expected error for empty monitor ID")
	}
	if err != nil && err.Error() != "monitor ID is required" {
		t.Errorf("Expected 'monitor ID is required' error, got: %v", err)
	}
}

// TestAlertRepository_GetByDateRange_Validation tests validation in GetByDateRange method
func TestAlertRepository_GetByDateRange_Validation(t *testing.T) {
	repo := postgres.NewAlertRepository(nil)
	ctx := context.Background()

	start := time.Now().Add(-24 * time.Hour)
	end := time.Now()

	_, err := repo.GetByDateRange(ctx, "", start, end)
	if err == nil {
		t.Error("Expected error for empty monitor ID")
	}
	if err != nil && err.Error() != "monitor ID is required" {
		t.Errorf("Expected 'monitor ID is required' error, got: %v", err)
	}
}

// TestAlertRepository_GetLastAlertTime_Validation tests validation in GetLastAlertTime method
func TestAlertRepository_GetLastAlertTime_Validation(t *testing.T) {
	repo := postgres.NewAlertRepository(nil)
	ctx := context.Background()

	_, err := repo.GetLastAlertTime(ctx, "", domain.AlertTypeDowntime)
	if err == nil {
		t.Error("Expected error for empty monitor ID")
	}
	if err != nil && err.Error() != "monitor ID is required" {
		t.Errorf("Expected 'monitor ID is required' error, got: %v", err)
	}
}
