//go:build integration
// +build integration

package tests

import (
	"context"
	"os"
	"testing"
	"time"

	"web-tracker/domain"
	"web-tracker/infrastructure/postgres"

	"github.com/google/uuid"
)

// TestIntegration_HealthCheckRepository_FullCRUD tests the full CRUD lifecycle
func TestIntegration_HealthCheckRepository_FullCRUD(t *testing.T) {
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

	healthCheckRepo := postgres.NewHealthCheckRepository(pool)
	monitorRepo := postgres.NewMonitorRepository(pool)

	// Create a test monitor first
	monitor := &domain.Monitor{
		ID:            uuid.New().String(),
		Name:          "Integration Test Monitor",
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

	// Test Create
	check := &domain.HealthCheck{
		ID:           uuid.New().String(),
		MonitorID:    monitor.ID,
		Status:       domain.StatusSuccess,
		StatusCode:   200,
		ResponseTime: 150 * time.Millisecond,
		ErrorMessage: "",
		CheckedAt:    time.Now(),
	}

	err = healthCheckRepo.Create(ctx, check)
	if err != nil {
		t.Fatalf("Failed to create health check: %v", err)
	}

	// Test GetByMonitorID
	checks, err := healthCheckRepo.GetByMonitorID(ctx, monitor.ID, 10)
	if err != nil {
		t.Fatalf("Failed to get health checks: %v", err)
	}

	if len(checks) != 1 {
		t.Errorf("Expected 1 health check, got %d", len(checks))
	}

	retrieved := checks[0]
	if retrieved.ID != check.ID {
		t.Errorf("Expected ID %s, got %s", check.ID, retrieved.ID)
	}
	if retrieved.Status != check.Status {
		t.Errorf("Expected status %s, got %s", check.Status, retrieved.Status)
	}
	if retrieved.StatusCode != check.StatusCode {
		t.Errorf("Expected status code %d, got %d", check.StatusCode, retrieved.StatusCode)
	}
	if retrieved.ResponseTime != check.ResponseTime {
		t.Errorf("Expected response time %v, got %v", check.ResponseTime, retrieved.ResponseTime)
	}

	// Test Create with SSL info
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

	err = healthCheckRepo.Create(ctx, checkWithSSL)
	if err != nil {
		t.Fatalf("Failed to create health check with SSL: %v", err)
	}

	// Verify SSL info
	checks, err = healthCheckRepo.GetByMonitorID(ctx, monitor.ID, 10)
	if err != nil {
		t.Fatalf("Failed to get health checks: %v", err)
	}

	if len(checks) != 2 {
		t.Errorf("Expected 2 health checks, got %d", len(checks))
	}

	var sslCheck *domain.HealthCheck
	for _, c := range checks {
		if c.ID == checkWithSSL.ID {
			sslCheck = c
			break
		}
	}

	if sslCheck == nil {
		t.Fatal("Health check with SSL not found")
	}

	if sslCheck.SSLInfo == nil {
		t.Fatal("SSL info not saved")
	}

	if sslCheck.SSLInfo.Valid != checkWithSSL.SSLInfo.Valid {
		t.Errorf("Expected SSL valid %v, got %v", checkWithSSL.SSLInfo.Valid, sslCheck.SSLInfo.Valid)
	}
	if sslCheck.SSLInfo.DaysUntil != checkWithSSL.SSLInfo.DaysUntil {
		t.Errorf("Expected SSL days until %d, got %d", checkWithSSL.SSLInfo.DaysUntil, sslCheck.SSLInfo.DaysUntil)
	}
	if sslCheck.SSLInfo.Issuer != checkWithSSL.SSLInfo.Issuer {
		t.Errorf("Expected SSL issuer %s, got %s", checkWithSSL.SSLInfo.Issuer, sslCheck.SSLInfo.Issuer)
	}
}

// TestIntegration_HealthCheckRepository_GetByDateRange tests date range queries
func TestIntegration_HealthCheckRepository_GetByDateRange(t *testing.T) {
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

	healthCheckRepo := postgres.NewHealthCheckRepository(pool)
	monitorRepo := postgres.NewMonitorRepository(pool)

	// Create a test monitor
	monitor := &domain.Monitor{
		ID:            uuid.New().String(),
		Name:          "Date Range Test Monitor",
		URL:           "https://example.com",
		CheckInterval: domain.CheckInterval5Min,
		Enabled:       true,
	}

	err = monitorRepo.Create(ctx, monitor)
	if err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}
	defer monitorRepo.Delete(ctx, monitor.ID)

	// Create health checks at different times
	now := time.Now()
	for i := 0; i < 10; i++ {
		check := &domain.HealthCheck{
			ID:           uuid.New().String(),
			MonitorID:    monitor.ID,
			Status:       domain.StatusSuccess,
			StatusCode:   200,
			ResponseTime: 150 * time.Millisecond,
			CheckedAt:    now.Add(-time.Duration(i) * time.Hour),
		}
		err := healthCheckRepo.Create(ctx, check)
		if err != nil {
			t.Fatalf("Failed to create health check: %v", err)
		}
	}

	// Query for checks in the last 5 hours
	start := now.Add(-5 * time.Hour)
	end := now.Add(1 * time.Hour)

	checks, err := healthCheckRepo.GetByDateRange(ctx, monitor.ID, start, end)
	if err != nil {
		t.Fatalf("Failed to get health checks by date range: %v", err)
	}

	// Should get checks 0-5 (6 checks)
	if len(checks) < 5 || len(checks) > 6 {
		t.Errorf("Expected 5-6 health checks, got %d", len(checks))
	}

	// Verify all checks are within range
	for _, check := range checks {
		if check.CheckedAt.Before(start) {
			t.Errorf("Check %s is before start time", check.ID)
		}
		if check.CheckedAt.After(end) {
			t.Errorf("Check %s is after end time", check.ID)
		}
	}
}

// TestIntegration_HealthCheckRepository_DeleteOlderThan tests data retention
func TestIntegration_HealthCheckRepository_DeleteOlderThan(t *testing.T) {
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

	healthCheckRepo := postgres.NewHealthCheckRepository(pool)
	monitorRepo := postgres.NewMonitorRepository(pool)

	// Create a test monitor
	monitor := &domain.Monitor{
		ID:            uuid.New().String(),
		Name:          "Retention Test Monitor",
		URL:           "https://example.com",
		CheckInterval: domain.CheckInterval5Min,
		Enabled:       true,
	}

	err = monitorRepo.Create(ctx, monitor)
	if err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}
	defer monitorRepo.Delete(ctx, monitor.ID)

	now := time.Now()

	// Create old health checks (100 days ago)
	for i := 0; i < 3; i++ {
		check := &domain.HealthCheck{
			ID:           uuid.New().String(),
			MonitorID:    monitor.ID,
			Status:       domain.StatusSuccess,
			StatusCode:   200,
			ResponseTime: 150 * time.Millisecond,
			CheckedAt:    now.Add(-100 * 24 * time.Hour),
		}
		err := healthCheckRepo.Create(ctx, check)
		if err != nil {
			t.Fatalf("Failed to create old health check: %v", err)
		}
	}

	// Create recent health checks (10 days ago)
	for i := 0; i < 2; i++ {
		check := &domain.HealthCheck{
			ID:           uuid.New().String(),
			MonitorID:    monitor.ID,
			Status:       domain.StatusSuccess,
			StatusCode:   200,
			ResponseTime: 150 * time.Millisecond,
			CheckedAt:    now.Add(-10 * 24 * time.Hour),
		}
		err := healthCheckRepo.Create(ctx, check)
		if err != nil {
			t.Fatalf("Failed to create recent health check: %v", err)
		}
	}

	// Verify we have 5 checks total
	allChecks, err := healthCheckRepo.GetByMonitorID(ctx, monitor.ID, 100)
	if err != nil {
		t.Fatalf("Failed to get all health checks: %v", err)
	}
	if len(allChecks) != 5 {
		t.Errorf("Expected 5 health checks before deletion, got %d", len(allChecks))
	}

	// Delete checks older than 90 days
	cutoff := now.Add(-90 * 24 * time.Hour)
	err = healthCheckRepo.DeleteOlderThan(ctx, cutoff)
	if err != nil {
		t.Fatalf("Failed to delete old health checks: %v", err)
	}

	// Verify only recent checks remain
	remainingChecks, err := healthCheckRepo.GetByMonitorID(ctx, monitor.ID, 100)
	if err != nil {
		t.Fatalf("Failed to get remaining health checks: %v", err)
	}
	if len(remainingChecks) != 2 {
		t.Errorf("Expected 2 health checks after deletion, got %d", len(remainingChecks))
	}

	// Verify all remaining checks are recent
	for _, check := range remainingChecks {
		if check.CheckedAt.Before(cutoff) {
			t.Errorf("Check %s should have been deleted", check.ID)
		}
	}
}

// TestIntegration_HealthCheckRepository_Ordering tests result ordering
func TestIntegration_HealthCheckRepository_Ordering(t *testing.T) {
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

	healthCheckRepo := postgres.NewHealthCheckRepository(pool)
	monitorRepo := postgres.NewMonitorRepository(pool)

	// Create a test monitor
	monitor := &domain.Monitor{
		ID:            uuid.New().String(),
		Name:          "Ordering Test Monitor",
		URL:           "https://example.com",
		CheckInterval: domain.CheckInterval5Min,
		Enabled:       true,
	}

	err = monitorRepo.Create(ctx, monitor)
	if err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}
	defer monitorRepo.Delete(ctx, monitor.ID)

	// Create health checks at different times
	now := time.Now()
	for i := 0; i < 5; i++ {
		check := &domain.HealthCheck{
			ID:           uuid.New().String(),
			MonitorID:    monitor.ID,
			Status:       domain.StatusSuccess,
			StatusCode:   200,
			ResponseTime: time.Duration(100+i*10) * time.Millisecond,
			CheckedAt:    now.Add(-time.Duration(i) * time.Minute),
		}
		err := healthCheckRepo.Create(ctx, check)
		if err != nil {
			t.Fatalf("Failed to create health check: %v", err)
		}
	}

	// Get checks and verify ordering (most recent first)
	checks, err := healthCheckRepo.GetByMonitorID(ctx, monitor.ID, 10)
	if err != nil {
		t.Fatalf("Failed to get health checks: %v", err)
	}

	if len(checks) != 5 {
		t.Errorf("Expected 5 health checks, got %d", len(checks))
	}

	// Verify descending order by checked_at
	for i := 0; i < len(checks)-1; i++ {
		if checks[i].CheckedAt.Before(checks[i+1].CheckedAt) {
			t.Errorf("Checks not in descending order: check %d (%v) is before check %d (%v)",
				i, checks[i].CheckedAt, i+1, checks[i+1].CheckedAt)
		}
	}
}
