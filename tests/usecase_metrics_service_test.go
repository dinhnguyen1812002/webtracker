package tests

import (
	"context"
	"testing"
	"time"

	"web-tracker/domain"
	"web-tracker/usecase"
)

// MockHealthCheckRepository for testing
type MockHealthCheckRepository struct {
	checks []*domain.HealthCheck
}

func (m *MockHealthCheckRepository) Create(ctx context.Context, check *domain.HealthCheck) error {
	m.checks = append(m.checks, check)
	return nil
}

func (m *MockHealthCheckRepository) GetByMonitorID(ctx context.Context, monitorID string, limit int) ([]*domain.HealthCheck, error) {
	var result []*domain.HealthCheck
	for _, check := range m.checks {
		if check.MonitorID == monitorID {
			result = append(result, check)
		}
	}
	return result, nil
}

func (m *MockHealthCheckRepository) GetByDateRange(ctx context.Context, monitorID string, start, end time.Time) ([]*domain.HealthCheck, error) {
	var result []*domain.HealthCheck
	for _, check := range m.checks {
		if check.MonitorID == monitorID && check.CheckedAt.After(start) && check.CheckedAt.Before(end) {
			result = append(result, check)
		}
	}
	return result, nil
}

func (m *MockHealthCheckRepository) DeleteOlderThan(ctx context.Context, before time.Time) error {
	return nil
}

// MockRedisClient for testing
type MockRedisClient struct {
	cache map[string][]byte
}

func (m *MockRedisClient) Get(ctx context.Context, key string) ([]byte, error) {
	if data, exists := m.cache[key]; exists {
		return data, nil
	}
	return nil, nil
}

func (m *MockRedisClient) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if m.cache == nil {
		m.cache = make(map[string][]byte)
	}
	m.cache[key] = value
	return nil
}

func (m *MockRedisClient) Delete(ctx context.Context, key string) error {
	delete(m.cache, key)
	return nil
}

func (m *MockRedisClient) SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return nil
}

func (m *MockRedisClient) GetJSON(ctx context.Context, key string, dest interface{}) (bool, error) {
	return false, nil // Always miss cache for testing
}

func TestMetricsService_GetUptimePercentage(t *testing.T) {
	// Setup
	mockRepo := &MockHealthCheckRepository{}
	mockRedis := &MockRedisClient{}
	service := usecase.NewMetricsService(mockRepo, mockRedis)

	monitorID := "test-monitor"
	now := time.Now()

	// Add test data - 8 successful checks, 2 failed checks
	for i := 0; i < 8; i++ {
		mockRepo.checks = append(mockRepo.checks, &domain.HealthCheck{
			ID:        "success-" + string(rune(i)),
			MonitorID: monitorID,
			Status:    domain.StatusSuccess,
			CheckedAt: now.Add(-time.Duration(i) * time.Hour),
		})
	}

	for i := 0; i < 2; i++ {
		mockRepo.checks = append(mockRepo.checks, &domain.HealthCheck{
			ID:        "failure-" + string(rune(i)),
			MonitorID: monitorID,
			Status:    domain.StatusFailure,
			CheckedAt: now.Add(-time.Duration(i+8) * time.Hour),
		})
	}

	// Test
	ctx := context.Background()
	stats, err := service.GetUptimePercentage(ctx, monitorID)

	// Verify
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if stats == nil {
		t.Fatal("Expected stats, got nil")
	}

	// Should be 80% uptime (8 successful out of 10 total)
	expected := 80.0
	if stats.Period24h != expected {
		t.Errorf("Expected 24h uptime %f, got %f", expected, stats.Period24h)
	}
}

func TestMetricsService_GetResponseTimeStats(t *testing.T) {
	// Setup
	mockRepo := &MockHealthCheckRepository{}
	mockRedis := &MockRedisClient{}
	service := usecase.NewMetricsService(mockRepo, mockRedis)

	monitorID := "test-monitor"
	now := time.Now()

	// Add test data with various response times
	responseTimes := []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		300 * time.Millisecond,
		400 * time.Millisecond,
		500 * time.Millisecond,
	}

	for i, rt := range responseTimes {
		mockRepo.checks = append(mockRepo.checks, &domain.HealthCheck{
			ID:           "check-" + string(rune(i)),
			MonitorID:    monitorID,
			Status:       domain.StatusSuccess,
			ResponseTime: rt,
			CheckedAt:    now.Add(-time.Duration(i) * time.Minute),
		})
	}

	// Test
	ctx := context.Background()
	stats, err := service.GetResponseTimeStats(ctx, monitorID, time.Hour)

	// Verify
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if stats == nil {
		t.Fatal("Expected stats, got nil")
	}

	// Check that we got reasonable values
	if stats.Min != 100*time.Millisecond {
		t.Errorf("Expected min response time 100ms, got %v", stats.Min)
	}

	if stats.Max != 500*time.Millisecond {
		t.Errorf("Expected max response time 500ms, got %v", stats.Max)
	}

	if stats.Average != 300*time.Millisecond {
		t.Errorf("Expected average response time 300ms, got %v", stats.Average)
	}
}

func TestMetricsService_EmptyHistory(t *testing.T) {
	// Setup
	mockRepo := &MockHealthCheckRepository{}
	mockRedis := &MockRedisClient{}
	service := usecase.NewMetricsService(mockRepo, mockRedis)

	monitorID := "empty-monitor"

	// Test uptime with no data
	ctx := context.Background()
	uptimeStats, err := service.GetUptimePercentage(ctx, monitorID)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Should return 0% for empty history
	if uptimeStats.Period24h != 0.0 {
		t.Errorf("Expected 0%% uptime for empty history, got %f", uptimeStats.Period24h)
	}

	// Test response time stats with no data
	responseStats, err := service.GetResponseTimeStats(ctx, monitorID, time.Hour)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Should return zero stats for empty history
	if responseStats.Average != 0 {
		t.Errorf("Expected 0 average response time for empty history, got %v", responseStats.Average)
	}
}
