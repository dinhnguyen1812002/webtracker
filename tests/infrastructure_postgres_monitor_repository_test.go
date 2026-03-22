package tests

import (
	"context"
	"testing"
	"time"

	"web-tracker/domain"
	"web-tracker/infrastructure/postgres"
)

// TestMonitorRepository_Create tests the Create method
func TestMonitorRepository_Create(t *testing.T) {
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

	repo := postgres.NewMonitorRepository(pool)

	monitor := &domain.Monitor{
		ID:            "test-monitor-1",
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

	err = repo.Create(ctx, monitor)
	if err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}

	// Verify the monitor was created
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

	// Cleanup
	_ = repo.Delete(ctx, monitor.ID)
}

// TestMonitorRepository_Validation tests validation errors
func TestMonitorRepository_Validation(t *testing.T) {
	repo := postgres.NewMonitorRepository(nil)

	tests := []struct {
		name    string
		monitor *domain.Monitor
		wantErr bool
	}{
		{
			name:    "nil monitor",
			monitor: nil,
			wantErr: true,
		},
		{
			name: "empty name",
			monitor: &domain.Monitor{
				ID:            "test-1",
				Name:          "",
				URL:           "https://example.com",
				CheckInterval: domain.CheckInterval5Min,
			},
			wantErr: true,
		},
		{
			name: "invalid URL",
			monitor: &domain.Monitor{
				ID:            "test-2",
				Name:          "Test",
				URL:           "not-a-url",
				CheckInterval: domain.CheckInterval5Min,
			},
			wantErr: true,
		},
		{
			name: "invalid check interval",
			monitor: &domain.Monitor{
				ID:            "test-3",
				Name:          "Test",
				URL:           "https://example.com",
				CheckInterval: 10 * time.Minute,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.Create(context.Background(), tt.monitor)
			if (err != nil) != tt.wantErr {
				t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
