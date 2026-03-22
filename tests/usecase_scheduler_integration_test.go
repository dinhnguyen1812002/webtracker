package tests

import (
	"context"
	"testing"
	"time"

	"web-tracker/domain"
	"web-tracker/infrastructure/redis"
	"web-tracker/usecase"
)

// MockHealthCheckExecutor for testing
type MockHealthCheckExecutor struct {
	executedChecks []string
}

func (m *MockHealthCheckExecutor) ExecuteCheck(ctx context.Context, monitorID string) (*domain.HealthCheck, error) {
	m.executedChecks = append(m.executedChecks, monitorID)
	return &domain.HealthCheck{
		ID:        "test-check",
		MonitorID: monitorID,
		Status:    domain.StatusSuccess,
	}, nil
}

func TestSchedulerWorkerPoolIntegration(t *testing.T) {
	// Skip if Redis is not available
	redisClient, err := redis.NewClient(redis.DefaultConfig())
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	defer redisClient.Close()

	// Clean up test data
	queueName := "test:integration:queue"
	scheduleName := "test:integration:schedule"
	defer func() {
		redisClient.Delete(context.Background(), queueName)
		redisClient.Delete(context.Background(), scheduleName)
	}()

	// Create mock health check executor
	mockExecutor := &MockHealthCheckExecutor{}

	// Create scheduler
	schedulerConfig := usecase.SchedulerConfig{
		ScheduleName: scheduleName,
		QueueName:    queueName,
		TickInterval: 100 * time.Millisecond, // Fast ticking for test
	}
	scheduler := usecase.NewScheduler(redisClient, schedulerConfig)

	// Create worker pool
	workerPool := usecase.NewWorkerPool(redisClient, mockExecutor, queueName)

	// Start both scheduler and worker pool
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = scheduler.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}
	defer scheduler.Stop()

	err = workerPool.Start(ctx, 2) // Use 2 workers
	if err != nil {
		t.Fatalf("Failed to start worker pool: %v", err)
	}
	defer workerPool.Stop()

	// Create a test monitor with short interval for quick testing
	monitor := &domain.Monitor{
		ID:            "integration-test-monitor",
		Name:          "Integration Test Monitor",
		URL:           "https://example.com",
		CheckInterval: 200 * time.Millisecond, // Very short for testing
		Enabled:       true,
	}

	// Schedule the monitor
	err = scheduler.ScheduleMonitor(monitor)
	if err != nil {
		t.Fatalf("Failed to schedule monitor: %v", err)
	}

	// Wait a bit for the job to be processed
	time.Sleep(1 * time.Second)

	// Verify that the health check was executed
	if len(mockExecutor.executedChecks) == 0 {
		t.Error("Expected at least one health check to be executed")
	}

	// Verify the correct monitor was checked
	found := false
	for _, checkedID := range mockExecutor.executedChecks {
		if checkedID == monitor.ID {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected monitor %s to be checked, but it wasn't found in executed checks: %v",
			monitor.ID, mockExecutor.executedChecks)
	}

	// Check worker pool stats
	stats := workerPool.GetStats()
	if stats.ProcessedJobs == 0 {
		t.Error("Expected worker pool to have processed at least one job")
	}

	t.Logf("Integration test completed successfully. Processed jobs: %d, Executed checks: %v",
		stats.ProcessedJobs, mockExecutor.executedChecks)
}
