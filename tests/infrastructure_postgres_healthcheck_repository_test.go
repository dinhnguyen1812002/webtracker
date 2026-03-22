package tests

import (
	"context"
	"testing"
	"time"

	"web-tracker/domain"
	"web-tracker/infrastructure/postgres"

	"github.com/google/uuid"
)

func TestHealthCheckRepository_Create(t *testing.T) {
	// Skip if no database connection available
	t.Skip("Integration test - requires PostgreSQL database")

	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, postgres.DefaultPoolConfig())
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	// Run migrations
	if err := postgres.RunMigrations(ctx, pool); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	repo := postgres.NewHealthCheckRepository(pool)
	monitorRepo := postgres.NewMonitorRepository(pool)

	// Create a monitor first (required for foreign key)
	monitor := &domain.Monitor{
		ID:            uuid.New().String(),
		Name:          "Test Monitor",
		URL:           "https://example.com",
		CheckInterval: domain.CheckInterval5Min,
		Enabled:       true,
		AlertChannels: []domain.AlertChannel{},
	}
	err = monitorRepo.Create(ctx, monitor)
	if err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}
	defer monitorRepo.Delete(ctx, monitor.ID)

	// Test successful creation
	check := &domain.HealthCheck{
		ID:           uuid.New().String(),
		MonitorID:    monitor.ID,
		Status:       domain.StatusSuccess,
		StatusCode:   200,
		ResponseTime: 150 * time.Millisecond,
		CheckedAt:    time.Now(),
	}

	err = repo.Create(ctx, check)
	if err != nil {
		t.Fatalf("Failed to create health check: %v", err)
	}

	// Verify it was created
	checks, err := repo.GetByMonitorID(ctx, monitor.ID, 10)
	if err != nil {
		t.Fatalf("Failed to get health checks: %v", err)
	}
	if len(checks) != 1 {
		t.Errorf("Expected 1 health check, got %d", len(checks))
	}
	if checks[0].ID != check.ID {
		t.Errorf("Expected ID %s, got %s", check.ID, checks[0].ID)
	}
	if checks[0].Status != check.Status {
		t.Errorf("Expected status %s, got %s", check.Status, checks[0].Status)
	}
	if checks[0].ResponseTime != check.ResponseTime {
		t.Errorf("Expected response time %v, got %v", check.ResponseTime, checks[0].ResponseTime)
	}

	// Test creation with SSL info
	expiresAt := time.Now().Add(30 * 24 * time.Hour)
	checkWithSSL := &domain.HealthCheck{
		ID:           uuid.New().String(),
		MonitorID:    monitor.ID,
		Status:       domain.StatusSuccess,
		StatusCode:   200,
		ResponseTime: 200 * time.Millisecond,
		SSLInfo: &domain.SSLInfo{
			Valid:     true,
			ExpiresAt: expiresAt,
			DaysUntil: 30,
			Issuer:    "Let's Encrypt",
		},
		CheckedAt: time.Now(),
	}

	err = repo.Create(ctx, checkWithSSL)
	if err != nil {
		t.Fatalf("Failed to create health check with SSL: %v", err)
	}

	// Verify SSL info was saved
	checks, err = repo.GetByMonitorID(ctx, monitor.ID, 10)
	if err != nil {
		t.Fatalf("Failed to get health checks: %v", err)
	}

	var savedCheck *domain.HealthCheck
	for _, c := range checks {
		if c.ID == checkWithSSL.ID {
			savedCheck = c
			break
		}
	}
	if savedCheck == nil {
		t.Fatal("Health check with SSL not found")
	}
	if savedCheck.SSLInfo == nil {
		t.Fatal("SSL info not saved")
	}
	if savedCheck.SSLInfo.Valid != checkWithSSL.SSLInfo.Valid {
		t.Errorf("Expected SSL valid %v, got %v", checkWithSSL.SSLInfo.Valid, savedCheck.SSLInfo.Valid)
	}
}

// TestHealthCheckRepository_Validation tests validation errors
func TestHealthCheckRepository_Validation(t *testing.T) {
	repo := postgres.NewHealthCheckRepository(nil)

	tests := []struct {
		name    string
		check   *domain.HealthCheck
		wantErr bool
	}{
		{
			name:    "nil health check",
			check:   nil,
			wantErr: true,
		},
		{
			name: "missing ID",
			check: &domain.HealthCheck{
				MonitorID:    "monitor-1",
				Status:       domain.StatusSuccess,
				StatusCode:   200,
				ResponseTime: 150 * time.Millisecond,
			},
			wantErr: true,
		},
		{
			name: "missing monitor ID",
			check: &domain.HealthCheck{
				ID:           "check-1",
				Status:       domain.StatusSuccess,
				StatusCode:   200,
				ResponseTime: 150 * time.Millisecond,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.Create(context.Background(), tt.check)
			if (err != nil) != tt.wantErr {
				t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
