package tests

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"web-tracker/domain"
	"web-tracker/infrastructure/httpclient"
	"web-tracker/usecase"
)

// Mock repositories for testing
type mockMonitorRepo struct {
	monitor *domain.Monitor
	err     error
}

func (m *mockMonitorRepo) Create(ctx context.Context, monitor *domain.Monitor) error {
	return nil
}

func (m *mockMonitorRepo) GetByID(ctx context.Context, id string) (*domain.Monitor, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.monitor, nil
}

func (m *mockMonitorRepo) List(ctx context.Context, filters domain.ListFilters) ([]*domain.Monitor, error) {
	return nil, nil
}

func (m *mockMonitorRepo) Update(ctx context.Context, monitor *domain.Monitor) error {
	return nil
}

func (m *mockMonitorRepo) Delete(ctx context.Context, id string) error {
	return nil
}

type mockHealthCheckRepo struct {
	checks []*domain.HealthCheck
	err    error
}

func (m *mockHealthCheckRepo) Create(ctx context.Context, check *domain.HealthCheck) error {
	if m.err != nil {
		return m.err
	}
	m.checks = append(m.checks, check)
	return nil
}

func (m *mockHealthCheckRepo) GetByMonitorID(ctx context.Context, monitorID string, limit int) ([]*domain.HealthCheck, error) {
	return m.checks, m.err
}

func (m *mockHealthCheckRepo) GetByDateRange(ctx context.Context, monitorID string, start, end time.Time) ([]*domain.HealthCheck, error) {
	return m.checks, m.err
}

func (m *mockHealthCheckRepo) DeleteOlderThan(ctx context.Context, before time.Time) error {
	return m.err
}

type mockRedisClient struct {
	deletedKeys []string
	err         error
}

func (m *mockRedisClient) Delete(ctx context.Context, key string) error {
	if m.err != nil {
		return m.err
	}
	m.deletedKeys = append(m.deletedKeys, key)
	return nil
}

// TestExecuteCheck_Success tests successful health check execution
func TestExecuteCheck_Success(t *testing.T) {
	// Setup
	monitor := &domain.Monitor{
		ID:            "test-monitor-1",
		Name:          "Test Monitor",
		URL:           "http://example.com",
		CheckInterval: domain.CheckInterval1Min,
		Enabled:       true,
	}

	monitorRepo := &mockMonitorRepo{monitor: monitor}
	healthCheckRepo := &mockHealthCheckRepo{}
	redisClient := &mockRedisClient{}
	httpClient := newStubHTTPClient(func(req *http.Request) (*http.Response, error) {
		return newStubResponse(http.StatusOK, "OK"), nil
	})
	service := usecase.NewHealthCheckService(httpClient, healthCheckRepo, monitorRepo, redisClient, NewMockHealthCheckAlertService(), nil)

	// Execute
	ctx := context.Background()
	result, err := service.ExecuteCheck(ctx, monitor.ID)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected health check result, got nil")
	}

	if result.Status != domain.StatusSuccess {
		t.Errorf("Expected status %s, got %s", domain.StatusSuccess, result.Status)
	}

	if result.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, result.StatusCode)
	}

	if result.ResponseTime <= 0 {
		t.Error("Expected positive response time")
	}

	if result.MonitorID != monitor.ID {
		t.Errorf("Expected monitor ID %s, got %s", monitor.ID, result.MonitorID)
	}

	// Verify health check was persisted
	if len(healthCheckRepo.checks) != 1 {
		t.Errorf("Expected 1 health check to be persisted, got %d", len(healthCheckRepo.checks))
	}

	// Verify metrics cache was invalidated
	if len(redisClient.deletedKeys) == 0 {
		t.Error("Expected metrics cache to be invalidated")
	}
}

// TestExecuteCheck_StatusCodeClassification tests status code classification
// Requirement 1.2: 200-299 = success
// Requirement 1.3: Others = failure
func TestExecuteCheck_StatusCodeClassification(t *testing.T) {
	testCases := []struct {
		name           string
		statusCode     int
		expectedStatus domain.HealthCheckStatus
	}{
		{"200 OK", 200, domain.StatusSuccess},
		{"201 Created", 201, domain.StatusSuccess},
		{"204 No Content", 204, domain.StatusSuccess},
		{"299 Edge Success", 299, domain.StatusSuccess},
		{"300 Multiple Choices", 300, domain.StatusFailure},
		{"400 Bad Request", 400, domain.StatusFailure},
		{"404 Not Found", 404, domain.StatusFailure},
		{"500 Internal Server Error", 500, domain.StatusFailure},
		{"503 Service Unavailable", 503, domain.StatusFailure},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			monitor := &domain.Monitor{
				ID:            "test-monitor",
				Name:          "Test Monitor",
				URL:           "http://example.com",
				CheckInterval: domain.CheckInterval1Min,
				Enabled:       true,
			}

			monitorRepo := &mockMonitorRepo{monitor: monitor}
			healthCheckRepo := &mockHealthCheckRepo{}
			redisClient := &mockRedisClient{}
			httpClient := newStubHTTPClient(func(req *http.Request) (*http.Response, error) {
				return newStubResponse(tc.statusCode, ""), nil
			})
			service := usecase.NewHealthCheckService(httpClient, healthCheckRepo, monitorRepo, redisClient, NewMockHealthCheckAlertService(), nil)

			ctx := context.Background()
			result, err := service.ExecuteCheck(ctx, monitor.ID)

			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}

			if result.Status != tc.expectedStatus {
				t.Errorf("Expected status %s, got %s", tc.expectedStatus, result.Status)
			}

			if result.StatusCode != tc.statusCode {
				t.Errorf("Expected status code %d, got %d", tc.statusCode, result.StatusCode)
			}
		})
	}
}

// TestExecuteCheck_Timeout tests timeout handling
// Requirement 1.5: 30-second timeout
func TestExecuteCheck_Timeout(t *testing.T) {
	monitor := &domain.Monitor{
		ID:            "test-monitor",
		Name:          "Test Monitor",
		URL:           "http://example.com",
		CheckInterval: domain.CheckInterval1Min,
		Enabled:       true,
	}

	monitorRepo := &mockMonitorRepo{monitor: monitor}
	healthCheckRepo := &mockHealthCheckRepo{}
	redisClient := &mockRedisClient{}
	httpClient := newStubHTTPClient(newDelayedResponse(http.StatusOK, 200*time.Millisecond))
	service := usecase.NewHealthCheckService(httpClient, healthCheckRepo, monitorRepo, redisClient, NewMockHealthCheckAlertService(), nil)
	service.SetRequestTimeout(50 * time.Millisecond)
	service.SetRetryConfig(httpclient.RetryConfig{MaxAttempts: 1, InitialDelay: 10 * time.Millisecond})

	ctx := context.Background()
	result, err := service.ExecuteCheck(ctx, monitor.ID)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.Status != domain.StatusTimeout {
		t.Errorf("Expected status %s, got %s", domain.StatusTimeout, result.Status)
	}

	if result.ErrorMessage == "" {
		t.Error("Expected error message for timeout")
	}
}

// TestExecuteCheck_NetworkError tests network error handling
// Requirement 1.3: Network errors = failure
func TestExecuteCheck_NetworkError(t *testing.T) {
	monitor := &domain.Monitor{
		ID:            "test-monitor",
		Name:          "Test Monitor",
		URL:           "http://example.com",
		CheckInterval: domain.CheckInterval1Min,
		Enabled:       true,
	}

	monitorRepo := &mockMonitorRepo{monitor: monitor}
	healthCheckRepo := &mockHealthCheckRepo{}
	redisClient := &mockRedisClient{}
	httpClient := newStubHTTPClient(func(req *http.Request) (*http.Response, error) {
		return nil, errors.New("simulated network error")
	})
	service := usecase.NewHealthCheckService(httpClient, healthCheckRepo, monitorRepo, redisClient, NewMockHealthCheckAlertService(), nil)
	service.SetRetryConfig(httpclient.RetryConfig{MaxAttempts: 1, InitialDelay: 10 * time.Millisecond})

	ctx := context.Background()
	result, err := service.ExecuteCheck(ctx, monitor.ID)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.Status != domain.StatusFailure {
		t.Errorf("Expected status %s, got %s", domain.StatusFailure, result.Status)
	}

	if result.ErrorMessage == "" {
		t.Error("Expected error message for network error")
	}
}

// TestExecuteCheck_ResponseTimeRecording tests response time measurement
// Requirement 1.4: Measure and record response time
func TestExecuteCheck_ResponseTimeRecording(t *testing.T) {
	delay := 100 * time.Millisecond
	monitor := &domain.Monitor{
		ID:            "test-monitor",
		Name:          "Test Monitor",
		URL:           "http://example.com",
		CheckInterval: domain.CheckInterval1Min,
		Enabled:       true,
	}

	monitorRepo := &mockMonitorRepo{monitor: monitor}
	healthCheckRepo := &mockHealthCheckRepo{}
	redisClient := &mockRedisClient{}
	httpClient := newStubHTTPClient(newDelayedResponse(http.StatusOK, delay))
	service := usecase.NewHealthCheckService(httpClient, healthCheckRepo, monitorRepo, redisClient, NewMockHealthCheckAlertService(), nil)

	ctx := context.Background()
	result, err := service.ExecuteCheck(ctx, monitor.ID)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Response time should be at least the delay
	if result.ResponseTime < delay {
		t.Errorf("Expected response time >= %v, got %v", delay, result.ResponseTime)
	}

	// Response time should be non-negative
	if result.ResponseTime < 0 {
		t.Error("Expected non-negative response time")
	}
}

// TestExecuteCheck_MonitorNotFound tests error handling when monitor doesn't exist
func TestExecuteCheck_MonitorNotFound(t *testing.T) {
	monitorRepo := &mockMonitorRepo{err: errors.New("monitor not found")}
	healthCheckRepo := &mockHealthCheckRepo{}
	redisClient := &mockRedisClient{}
	httpClient := httpclient.NewClient(httpclient.DefaultConfig())
	service := usecase.NewHealthCheckService(httpClient, healthCheckRepo, monitorRepo, redisClient, NewMockHealthCheckAlertService(), nil)

	ctx := context.Background()
	result, err := service.ExecuteCheck(ctx, "non-existent-monitor")

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if result != nil {
		t.Error("Expected nil result when monitor not found")
	}
}

// TestClassifyStatusCode tests the status code classification logic
// classifyStatusCode is unexported; behavior validated via ExecuteCheck_StatusCodeClassification.

// TestGetCheckHistory tests retrieving health check history
func TestGetCheckHistory(t *testing.T) {
	checks := []*domain.HealthCheck{
		{
			ID:        "check-1",
			MonitorID: "monitor-1",
			Status:    domain.StatusSuccess,
			CheckedAt: time.Now(),
		},
		{
			ID:        "check-2",
			MonitorID: "monitor-1",
			Status:    domain.StatusFailure,
			CheckedAt: time.Now(),
		},
	}

	monitorRepo := &mockMonitorRepo{}
	healthCheckRepo := &mockHealthCheckRepo{checks: checks}
	redisClient := &mockRedisClient{}
	httpClient := httpclient.NewClient(httpclient.DefaultConfig())
	service := usecase.NewHealthCheckService(httpClient, healthCheckRepo, monitorRepo, redisClient, NewMockHealthCheckAlertService(), nil)

	ctx := context.Background()
	result, err := service.GetCheckHistory(ctx, "monitor-1", 10)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(result) != len(checks) {
		t.Errorf("Expected %d checks, got %d", len(checks), len(result))
	}
}

// TestExecuteCheck_HTTPS_ValidSSL tests SSL certificate extraction for valid HTTPS
// Requirement 2.1: Extract and validate SSL certificate
func TestExecuteCheck_HTTPS_ValidSSL(t *testing.T) {
	monitor := &domain.Monitor{
		ID:            "test-monitor",
		Name:          "Test Monitor",
		URL:           "https://example.com",
		CheckInterval: domain.CheckInterval1Min,
		Enabled:       true,
	}

	monitorRepo := &mockMonitorRepo{monitor: monitor}
	healthCheckRepo := &mockHealthCheckRepo{}
	redisClient := &mockRedisClient{}

	httpClient := newStubHTTPClient(func(req *http.Request) (*http.Response, error) {
		return newTLSResponse(http.StatusOK, true), nil
	})

	service := usecase.NewHealthCheckService(httpClient, healthCheckRepo, monitorRepo, redisClient, NewMockHealthCheckAlertService(), nil)

	ctx := context.Background()
	result, err := service.ExecuteCheck(ctx, monitor.ID)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.Status != domain.StatusSuccess {
		t.Errorf("Expected status %s, got %s", domain.StatusSuccess, result.Status)
	}

	// Verify SSL info was extracted
	if result.SSLInfo == nil {
		t.Fatal("Expected SSL info to be extracted, got nil")
	}

	if !result.SSLInfo.Valid {
		t.Error("Expected SSL certificate to be valid")
	}

	if result.SSLInfo.ExpiresAt.IsZero() {
		t.Error("Expected SSL expiration date to be set")
	}

	// Test server certificate should expire in the future
	if result.SSLInfo.DaysUntil < 0 {
		t.Errorf("Expected positive days until expiration, got %d", result.SSLInfo.DaysUntil)
	}
}

// TestExecuteCheck_HTTPS_InvalidSSL tests handling of invalid SSL certificates
// Requirement 2.6: Invalid SSL certificate = failed health check
func TestExecuteCheck_HTTPS_InvalidSSL(t *testing.T) {
	monitor := &domain.Monitor{
		ID:            "test-monitor",
		Name:          "Test Monitor",
		URL:           "https://example.com",
		CheckInterval: domain.CheckInterval1Min,
		Enabled:       true,
	}

	monitorRepo := &mockMonitorRepo{monitor: monitor}
	healthCheckRepo := &mockHealthCheckRepo{}
	redisClient := &mockRedisClient{}

	httpClient := newStubHTTPClient(func(req *http.Request) (*http.Response, error) {
		return nil, errors.New("tls: failed to verify certificate")
	})
	service := usecase.NewHealthCheckService(httpClient, healthCheckRepo, monitorRepo, redisClient, NewMockHealthCheckAlertService(), nil)
	service.SetRetryConfig(httpclient.RetryConfig{MaxAttempts: 1, InitialDelay: 10 * time.Millisecond})

	ctx := context.Background()
	result, err := service.ExecuteCheck(ctx, monitor.ID)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should fail due to invalid certificate
	if result.Status != domain.StatusFailure {
		t.Errorf("Expected status %s for invalid SSL, got %s", domain.StatusFailure, result.Status)
	}

	if result.ErrorMessage == "" {
		t.Error("Expected error message for invalid SSL certificate")
	}
}

// TestExtractSSLInfo_ValidCertificate tests SSL info extraction from valid certificate
// Requirement 2.1: Extract SSL certificate information
// extractSSLInfo is unexported; behavior validated via ExecuteCheck_HTTPS_ValidSSL.

// TestCalculateSSLDaysUntilExpiry tests days until expiration calculation
// Requirement 2.1: Calculate days until expiration
func TestCalculateSSLDaysUntilExpiry(t *testing.T) {
	testCases := []struct {
		name          string
		expiresAt     time.Time
		expectedDays  int
		checkNegative bool
	}{
		{
			name:         "30 days in future",
			expiresAt:    time.Now().Add(30 * 24 * time.Hour),
			expectedDays: 30,
		},
		{
			name:         "15 days in future",
			expiresAt:    time.Now().Add(15 * 24 * time.Hour),
			expectedDays: 15,
		},
		{
			name:         "7 days in future",
			expiresAt:    time.Now().Add(7 * 24 * time.Hour),
			expectedDays: 7,
		},
		{
			name:          "Already expired",
			expiresAt:     time.Now().Add(-1 * 24 * time.Hour),
			checkNegative: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			days := domain.CalculateSSLDaysUntilExpiry(tc.expiresAt)

			if tc.checkNegative {
				if days >= 0 {
					t.Errorf("Expected negative days for expired certificate, got %d", days)
				}
			} else {
				// Allow for small variance due to timing
				if days < tc.expectedDays-1 || days > tc.expectedDays+1 {
					t.Errorf("Expected approximately %d days, got %d", tc.expectedDays, days)
				}
			}
		})
	}
}

// TestExecuteCheck_Persistence tests that health checks are persisted to database
// Requirement 14.1: Persist health check results
func TestExecuteCheck_Persistence(t *testing.T) {
	monitor := &domain.Monitor{
		ID:            "test-monitor",
		Name:          "Test Monitor",
		URL:           "http://example.com",
		CheckInterval: domain.CheckInterval1Min,
		Enabled:       true,
	}

	monitorRepo := &mockMonitorRepo{monitor: monitor}
	healthCheckRepo := &mockHealthCheckRepo{}
	redisClient := &mockRedisClient{}
	httpClient := newStubHTTPClient(func(req *http.Request) (*http.Response, error) {
		return newStubResponse(http.StatusOK, ""), nil
	})
	service := usecase.NewHealthCheckService(httpClient, healthCheckRepo, monitorRepo, redisClient, NewMockHealthCheckAlertService(), nil)

	ctx := context.Background()
	result, err := service.ExecuteCheck(ctx, monitor.ID)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify health check was persisted to database
	if len(healthCheckRepo.checks) != 1 {
		t.Fatalf("Expected 1 health check to be persisted, got %d", len(healthCheckRepo.checks))
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
}

// TestExecuteCheck_MetricsCacheInvalidation tests that metrics cache is invalidated
// Requirement 14.1: Update metrics cache in Redis
func TestExecuteCheck_MetricsCacheInvalidation(t *testing.T) {
	monitor := &domain.Monitor{
		ID:            "test-monitor-123",
		Name:          "Test Monitor",
		URL:           "http://example.com",
		CheckInterval: domain.CheckInterval1Min,
		Enabled:       true,
	}

	monitorRepo := &mockMonitorRepo{monitor: monitor}
	healthCheckRepo := &mockHealthCheckRepo{}
	redisClient := &mockRedisClient{}
	httpClient := newStubHTTPClient(func(req *http.Request) (*http.Response, error) {
		return newStubResponse(http.StatusOK, ""), nil
	})
	service := usecase.NewHealthCheckService(httpClient, healthCheckRepo, monitorRepo, redisClient, NewMockHealthCheckAlertService(), nil)

	ctx := context.Background()
	_, err := service.ExecuteCheck(ctx, monitor.ID)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify metrics cache keys were deleted
	if len(redisClient.deletedKeys) == 0 {
		t.Fatal("Expected metrics cache to be invalidated")
	}

	// Verify expected cache keys were deleted
	expectedKeys := []string{
		"cache:metrics:test-monitor-123:uptime:24h",
		"cache:metrics:test-monitor-123:uptime:7d",
		"cache:metrics:test-monitor-123:uptime:30d",
		"cache:metrics:test-monitor-123:response:1h",
		"cache:metrics:test-monitor-123:response:24h",
		"cache:metrics:test-monitor-123:response:7d",
	}

	for _, expectedKey := range expectedKeys {
		found := false
		for _, deletedKey := range redisClient.deletedKeys {
			if deletedKey == expectedKey {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected cache key %s to be deleted", expectedKey)
		}
	}
}

// TestExecuteCheck_PersistenceError tests handling of database persistence errors
// Requirement 14.1: Handle persistence errors
func TestExecuteCheck_PersistenceError(t *testing.T) {
	monitor := &domain.Monitor{
		ID:            "test-monitor",
		Name:          "Test Monitor",
		URL:           "http://example.com",
		CheckInterval: domain.CheckInterval1Min,
		Enabled:       true,
	}

	monitorRepo := &mockMonitorRepo{monitor: monitor}
	healthCheckRepo := &mockHealthCheckRepo{err: errors.New("database error")}
	redisClient := &mockRedisClient{}
	httpClient := newStubHTTPClient(func(req *http.Request) (*http.Response, error) {
		return newStubResponse(http.StatusOK, ""), nil
	})
	service := usecase.NewHealthCheckService(httpClient, healthCheckRepo, monitorRepo, redisClient, NewMockHealthCheckAlertService(), nil)

	ctx := context.Background()
	result, err := service.ExecuteCheck(ctx, monitor.ID)

	// Should return error when persistence fails
	if err == nil {
		t.Fatal("Expected error when persistence fails, got nil")
	}

	// Result should still be returned even if persistence fails
	if result == nil {
		t.Error("Expected health check result even when persistence fails")
	}
}

// TestExecuteCheck_FailedCheckPersistence tests that failed checks are also persisted
// Requirement 14.1: Persist all health check results (success and failure)
func TestExecuteCheck_FailedCheckPersistence(t *testing.T) {
	monitor := &domain.Monitor{
		ID:            "test-monitor",
		Name:          "Test Monitor",
		URL:           "http://example.com",
		CheckInterval: domain.CheckInterval1Min,
		Enabled:       true,
	}

	monitorRepo := &mockMonitorRepo{monitor: monitor}
	healthCheckRepo := &mockHealthCheckRepo{}
	redisClient := &mockRedisClient{}
	httpClient := newStubHTTPClient(func(req *http.Request) (*http.Response, error) {
		return newStubResponse(http.StatusInternalServerError, ""), nil
	})
	service := usecase.NewHealthCheckService(httpClient, healthCheckRepo, monitorRepo, redisClient, NewMockHealthCheckAlertService(), nil)

	ctx := context.Background()
	_, err := service.ExecuteCheck(ctx, monitor.ID)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify failed check was persisted
	if len(healthCheckRepo.checks) != 1 {
		t.Fatalf("Expected 1 health check to be persisted, got %d", len(healthCheckRepo.checks))
	}

	persistedCheck := healthCheckRepo.checks[0]
	if persistedCheck.Status != domain.StatusFailure {
		t.Errorf("Expected persisted check status %s, got %s", domain.StatusFailure, persistedCheck.Status)
	}

	// Verify metrics cache was still invalidated
	if len(redisClient.deletedKeys) == 0 {
		t.Error("Expected metrics cache to be invalidated even for failed checks")
	}
}
