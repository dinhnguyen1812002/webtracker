package tests

import (
	"context"
	"testing"
	"time"

	"web-tracker/domain"
	"web-tracker/infrastructure/redis"
	"web-tracker/usecase"
)

func TestMetricsService_Integration(t *testing.T) {
	// Skip if not running integration tests
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// Setup Redis client (this would need a real Redis instance)
	redisConfig := redis.DefaultConfig()
	redisClient, err := redis.NewClient(redisConfig)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	defer redisClient.Close()

	// Setup mock repository
	mockRepo := &MockHealthCheckRepository{}
	service := usecase.NewMetricsService(mockRepo, redisClient)

	monitorID := "integration-test-monitor"
	now := time.Now()

	// Add test data
	for i := 0; i < 5; i++ {
		mockRepo.checks = append(mockRepo.checks, &domain.HealthCheck{
			ID:           "check-" + string(rune(i)),
			MonitorID:    monitorID,
			Status:       domain.StatusSuccess,
			ResponseTime: time.Duration(100+i*50) * time.Millisecond,
			CheckedAt:    now.Add(-time.Duration(i) * time.Hour),
		})
	}

	ctx := context.Background()

	// Test uptime calculation with caching
	stats1, err := service.GetUptimePercentage(ctx, monitorID)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Second call should use cache
	stats2, err := service.GetUptimePercentage(ctx, monitorID)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Results should be the same
	if stats1.Period24h != stats2.Period24h {
		t.Errorf("Cache not working: first call %f, second call %f", stats1.Period24h, stats2.Period24h)
	}

	// Test response time stats
	responseStats, err := service.GetResponseTimeStats(ctx, monitorID, time.Hour*24)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if responseStats.Min != 100*time.Millisecond {
		t.Errorf("Expected min 100ms, got %v", responseStats.Min)
	}

	if responseStats.Max != 300*time.Millisecond {
		t.Errorf("Expected max 300ms, got %v", responseStats.Max)
	}
}
