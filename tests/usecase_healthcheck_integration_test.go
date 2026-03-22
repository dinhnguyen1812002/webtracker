package tests

import (
	"context"
	"net/http"
	"testing"
	"time"

	"web-tracker/domain"
	"web-tracker/usecase"
)

// TestHealthCheckPersistence_Integration tests the complete flow of health check persistence
// This test validates Requirement 14.1: Save health check results to database and update metrics cache
func TestHealthCheckPersistence_Integration(t *testing.T) {
	// Setup monitor
	monitor := &domain.Monitor{
		ID:            "integration-test-monitor",
		Name:          "Integration Test Monitor",
		URL:           "http://example.com",
		CheckInterval: domain.CheckInterval1Min,
		Enabled:       true,
	}

	// Setup mocks
	monitorRepo := &mockMonitorRepo{monitor: monitor}
	healthCheckRepo := &mockHealthCheckRepo{}
	redisClient := &mockRedisClient{}
	alertService := NewMockHealthCheckAlertService()
	httpClient := newStubHTTPClient(func(req *http.Request) (*http.Response, error) {
		return newStubResponse(http.StatusOK, "OK"), nil
	})

	// Create service
	service := usecase.NewHealthCheckService(httpClient, healthCheckRepo, monitorRepo, redisClient, alertService, nil)

	// Execute health check
	ctx := context.Background()
	result, err := service.ExecuteCheck(ctx, monitor.ID)

	// Verify no error
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify health check result
	if result == nil {
		t.Fatal("Expected health check result, got nil")
	}

	if result.Status != domain.StatusSuccess {
		t.Errorf("Expected status %s, got %s", domain.StatusSuccess, result.Status)
	}

	// REQUIREMENT 14.1: Verify health check was persisted to database
	if len(healthCheckRepo.checks) != 1 {
		t.Fatalf("Expected 1 health check to be persisted to database, got %d", len(healthCheckRepo.checks))
	}

	persistedCheck := healthCheckRepo.checks[0]
	if persistedCheck.ID != result.ID {
		t.Errorf("Expected persisted check ID %s, got %s", result.ID, persistedCheck.ID)
	}

	if persistedCheck.MonitorID != monitor.ID {
		t.Errorf("Expected persisted check monitor ID %s, got %s", monitor.ID, persistedCheck.MonitorID)
	}

	if persistedCheck.Status != domain.StatusSuccess {
		t.Errorf("Expected persisted check status %s, got %s", domain.StatusSuccess, persistedCheck.Status)
	}

	if persistedCheck.StatusCode != http.StatusOK {
		t.Errorf("Expected persisted check status code %d, got %d", http.StatusOK, persistedCheck.StatusCode)
	}

	if persistedCheck.ResponseTime <= 0 {
		t.Error("Expected persisted check to have positive response time")
	}

	if persistedCheck.CheckedAt.IsZero() {
		t.Error("Expected persisted check to have checked_at timestamp")
	}

	// REQUIREMENT 14.1: Verify metrics cache was invalidated in Redis
	if len(redisClient.deletedKeys) == 0 {
		t.Fatal("Expected metrics cache to be invalidated in Redis")
	}

	// Verify all expected cache keys were deleted
	expectedCacheKeys := []string{
		"cache:metrics:integration-test-monitor:uptime",
		"cache:metrics:integration-test-monitor:uptime:24h",
		"cache:metrics:integration-test-monitor:uptime:7d",
		"cache:metrics:integration-test-monitor:uptime:30d",
		"cache:metrics:integration-test-monitor:response:1h",
		"cache:metrics:integration-test-monitor:response:24h",
		"cache:metrics:integration-test-monitor:response:7d",
	}

	for _, expectedKey := range expectedCacheKeys {
		found := false
		for _, deletedKey := range redisClient.deletedKeys {
			if deletedKey == expectedKey {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected cache key %s to be deleted from Redis", expectedKey)
		}
	}

	t.Log("✓ Health check successfully persisted to database")
	t.Log("✓ Metrics cache successfully invalidated in Redis")
	t.Log("✓ Requirement 14.1 validated: Save health check results to database and update metrics cache")
}

// TestHealthCheckPersistence_FailedCheck tests that failed checks are also persisted
// This validates that both successful and failed health checks are saved to the database
func TestHealthCheckPersistence_FailedCheck(t *testing.T) {
	monitor := &domain.Monitor{
		ID:            "failed-check-monitor",
		Name:          "Failed Check Monitor",
		URL:           "http://example.com",
		CheckInterval: domain.CheckInterval1Min,
		Enabled:       true,
	}

	monitorRepo := &mockMonitorRepo{monitor: monitor}
	healthCheckRepo := &mockHealthCheckRepo{}
	redisClient := &mockRedisClient{}
	alertService := NewMockHealthCheckAlertService()
	httpClient := newStubHTTPClient(func(req *http.Request) (*http.Response, error) {
		return newStubResponse(http.StatusInternalServerError, ""), nil
	})
	service := usecase.NewHealthCheckService(httpClient, healthCheckRepo, monitorRepo, redisClient, alertService, nil)

	ctx := context.Background()
	_, err := service.ExecuteCheck(ctx, monitor.ID)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify failed check was persisted
	if len(healthCheckRepo.checks) != 1 {
		t.Fatalf("Expected 1 failed health check to be persisted, got %d", len(healthCheckRepo.checks))
	}

	persistedCheck := healthCheckRepo.checks[0]
	if persistedCheck.Status != domain.StatusFailure {
		t.Errorf("Expected persisted check status %s, got %s", domain.StatusFailure, persistedCheck.Status)
	}

	if persistedCheck.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected persisted check status code %d, got %d", http.StatusInternalServerError, persistedCheck.StatusCode)
	}

	// Verify metrics cache was invalidated even for failed checks
	if len(redisClient.deletedKeys) == 0 {
		t.Error("Expected metrics cache to be invalidated even for failed checks")
	}

	t.Log("✓ Failed health check successfully persisted to database")
	t.Log("✓ Metrics cache invalidated for failed check")
}

// TestHealthCheckPersistence_WithSSL tests persistence of health checks with SSL info
// This validates that SSL certificate information is correctly persisted
func TestHealthCheckPersistence_WithSSL(t *testing.T) {
	monitor := &domain.Monitor{
		ID:            "ssl-check-monitor",
		Name:          "SSL Check Monitor",
		URL:           "https://example.com",
		CheckInterval: domain.CheckInterval1Min,
		Enabled:       true,
	}

	monitorRepo := &mockMonitorRepo{monitor: monitor}
	healthCheckRepo := &mockHealthCheckRepo{}
	redisClient := &mockRedisClient{}
	alertService := NewMockHealthCheckAlertService()
	httpClient := newStubHTTPClient(func(req *http.Request) (*http.Response, error) {
		return newTLSResponse(http.StatusOK, true), nil
	})
	service := usecase.NewHealthCheckService(httpClient, healthCheckRepo, monitorRepo, redisClient, alertService, nil)

	ctx := context.Background()
	_, err := service.ExecuteCheck(ctx, monitor.ID)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify health check with SSL info was persisted
	if len(healthCheckRepo.checks) != 1 {
		t.Fatalf("Expected 1 health check to be persisted, got %d", len(healthCheckRepo.checks))
	}

	persistedCheck := healthCheckRepo.checks[0]

	// Verify SSL info was persisted
	if persistedCheck.SSLInfo == nil {
		t.Fatal("Expected SSL info to be persisted, got nil")
	}

	if !persistedCheck.SSLInfo.Valid {
		t.Error("Expected persisted SSL info to be valid")
	}

	if persistedCheck.SSLInfo.ExpiresAt.IsZero() {
		t.Error("Expected persisted SSL expiration date to be set")
	}

	if persistedCheck.SSLInfo.DaysUntil < 0 {
		t.Errorf("Expected persisted SSL days until expiration to be positive, got %d", persistedCheck.SSLInfo.DaysUntil)
	}

	if persistedCheck.SSLInfo.Issuer == "" {
		t.Error("Expected persisted SSL issuer to be set")
	}

	t.Log("✓ Health check with SSL info successfully persisted to database")
}

// TestHealthCheckPersistence_MultipleChecks tests that multiple health checks are persisted correctly
// This validates that the system can handle multiple sequential health checks
func TestHealthCheckPersistence_MultipleChecks(t *testing.T) {
	monitor := &domain.Monitor{
		ID:            "multi-check-monitor",
		Name:          "Multi Check Monitor",
		URL:           "http://example.com",
		CheckInterval: domain.CheckInterval1Min,
		Enabled:       true,
	}

	monitorRepo := &mockMonitorRepo{monitor: monitor}
	healthCheckRepo := &mockHealthCheckRepo{}
	redisClient := &mockRedisClient{}
	alertService := NewMockHealthCheckAlertService()
	httpClient := newStubHTTPClient(func(req *http.Request) (*http.Response, error) {
		return newStubResponse(http.StatusOK, ""), nil
	})
	service := usecase.NewHealthCheckService(httpClient, healthCheckRepo, monitorRepo, redisClient, alertService, nil)

	ctx := context.Background()

	// Execute multiple health checks
	numChecks := 5
	for i := 0; i < numChecks; i++ {
		_, err := service.ExecuteCheck(ctx, monitor.ID)
		if err != nil {
			t.Fatalf("Check %d: Expected no error, got: %v", i+1, err)
		}
		time.Sleep(10 * time.Millisecond) // Small delay between checks
	}

	// Verify all checks were persisted
	if len(healthCheckRepo.checks) != numChecks {
		t.Fatalf("Expected %d health checks to be persisted, got %d", numChecks, len(healthCheckRepo.checks))
	}

	// Verify each check has unique ID and timestamp
	seenIDs := make(map[string]bool)
	for i, check := range healthCheckRepo.checks {
		if seenIDs[check.ID] {
			t.Errorf("Check %d: Duplicate ID found: %s", i+1, check.ID)
		}
		seenIDs[check.ID] = true

		if check.CheckedAt.IsZero() {
			t.Errorf("Check %d: Expected checked_at timestamp to be set", i+1)
		}
	}

	// Verify cache was invalidated for each check
	// Each check should invalidate 7 cache keys
	expectedInvalidations := numChecks * 7
	if len(redisClient.deletedKeys) != expectedInvalidations {
		t.Errorf("Expected %d cache invalidations, got %d", expectedInvalidations, len(redisClient.deletedKeys))
	}

	t.Logf("✓ Successfully persisted %d health checks to database", numChecks)
	t.Logf("✓ Cache invalidated %d times", len(redisClient.deletedKeys))
}
