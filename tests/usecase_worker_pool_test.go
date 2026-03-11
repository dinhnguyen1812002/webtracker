package tests

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"web-tracker/domain"
	"web-tracker/infrastructure/redis"
	"web-tracker/usecase"
)

// mockHealthCheckExecutor implements HealthCheckExecutor for testing
type mockHealthCheckExecutor struct {
	mu            sync.Mutex
	executedJobs  []string
	executeDelay  time.Duration
	executeError  error
	executeResult *domain.HealthCheck
}

func (m *mockHealthCheckExecutor) ExecuteCheck(ctx context.Context, monitorID string) (*domain.HealthCheck, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.executeDelay > 0 {
		time.Sleep(m.executeDelay)
	}

	m.executedJobs = append(m.executedJobs, monitorID)

	if m.executeError != nil {
		return nil, m.executeError
	}

	if m.executeResult != nil {
		return m.executeResult, nil
	}

	return &domain.HealthCheck{
		ID:        "test-check",
		MonitorID: monitorID,
		Status:    domain.StatusSuccess,
	}, nil
}

func (m *mockHealthCheckExecutor) getExecutedJobs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]string, len(m.executedJobs))
	copy(result, m.executedJobs)
	return result
}

func (m *mockHealthCheckExecutor) reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.executedJobs = nil
}

// TestWorkerPool_NewWorkerPool tests worker pool creation
func TestWorkerPool_NewWorkerPool(t *testing.T) {
	redisClient := &redis.Client{}
	healthService := &mockHealthCheckExecutor{}
	queueName := "test-queue"

	wp := usecase.NewWorkerPool(redisClient, healthService, queueName)

	if wp == nil {
		t.Fatal("Expected worker pool to be created, got nil")
	}

	// Test that the worker pool is not running initially
	if wp.IsRunning() {
		t.Error("Expected worker pool to not be running initially")
	}
}

// TestWorkerPool_StartStop tests starting and stopping the worker pool
func TestWorkerPool_StartStop(t *testing.T) {
	// Create Redis client for testing
	redisConfig := redis.DefaultConfig()
	redisConfig.DB = 1 // Use different DB for testing
	redisClient, err := redis.NewClient(redisConfig)
	if err != nil {
		t.Skipf("Redis not available for testing: %v", err)
	}
	defer redisClient.Close()

	healthService := &mockHealthCheckExecutor{}
	queueName := "test-queue-start-stop"

	wp := usecase.NewWorkerPool(redisClient, healthService, queueName)

	// Test starting with default workers
	ctx := context.Background()
	err = wp.Start(ctx, 0) // 0 should use default
	if err != nil {
		t.Fatalf("Failed to start worker pool: %v", err)
	}

	if !wp.IsRunning() {
		t.Error("Expected worker pool to be running after start")
	}

	stats := wp.GetStats()
	if stats.ActiveWorkers < 0 {
		t.Errorf("Expected non-negative active workers, got %d", stats.ActiveWorkers)
	}

	// Test stopping
	err = wp.Stop()
	if err != nil {
		t.Fatalf("Failed to stop worker pool: %v", err)
	}

	if wp.IsRunning() {
		t.Error("Expected worker pool to not be running after stop")
	}
}

// TestWorkerPool_StartWithCustomWorkers tests starting with custom number of workers
func TestWorkerPool_StartWithCustomWorkers(t *testing.T) {
	redisConfig := redis.DefaultConfig()
	redisConfig.DB = 1
	redisClient, err := redis.NewClient(redisConfig)
	if err != nil {
		t.Skipf("Redis not available for testing: %v", err)
	}
	defer redisClient.Close()

	healthService := &mockHealthCheckExecutor{}
	queueName := "test-queue-custom-workers"

	wp := usecase.NewWorkerPool(redisClient, healthService, queueName)

	// Start with 5 workers
	ctx := context.Background()
	err = wp.Start(ctx, 5)
	if err != nil {
		t.Fatalf("Failed to start worker pool: %v", err)
	}
	defer wp.Stop()

	// Test that the worker pool is running after start
	if !wp.IsRunning() {
		t.Error("Expected worker pool to be running after start")
	}

	// Test worker pool stats
	stats := wp.GetStats()
	if stats.ActiveWorkers < 0 {
		t.Errorf("Expected non-negative active workers, got %d", stats.ActiveWorkers)
	}
}

// TestWorkerPool_DoubleStart tests that starting an already running pool returns error
func TestWorkerPool_DoubleStart(t *testing.T) {
	redisConfig := redis.DefaultConfig()
	redisConfig.DB = 1
	redisClient, err := redis.NewClient(redisConfig)
	if err != nil {
		t.Skipf("Redis not available for testing: %v", err)
	}
	defer redisClient.Close()

	healthService := &mockHealthCheckExecutor{}
	queueName := "test-queue-double-start"

	wp := usecase.NewWorkerPool(redisClient, healthService, queueName)

	ctx := context.Background()
	err = wp.Start(ctx, 3)
	if err != nil {
		t.Fatalf("Failed to start worker pool: %v", err)
	}
	defer wp.Stop()

	// Try to start again
	err = wp.Start(ctx, 3)
	if err == nil {
		t.Error("Expected error when starting already running worker pool")
	}
}

// TestWorkerPool_JobProcessing tests that workers process jobs from the queue
func TestWorkerPool_JobProcessing(t *testing.T) {
	redisConfig := redis.DefaultConfig()
	redisConfig.DB = 1
	redisClient, err := redis.NewClient(redisConfig)
	if err != nil {
		t.Skipf("Redis not available for testing: %v", err)
	}
	defer redisClient.Close()

	healthService := &mockHealthCheckExecutor{}
	queueName := "test-queue-job-processing"

	// Clear the queue first
	ctx := context.Background()
	for {
		job, _ := redisClient.Dequeue(ctx, queueName, 100*time.Millisecond)
		if job == nil {
			break
		}
	}

	wp := usecase.NewWorkerPool(redisClient, healthService, queueName)

	// Start worker pool with 2 workers
	err = wp.Start(ctx, 2)
	if err != nil {
		t.Fatalf("Failed to start worker pool: %v", err)
	}
	defer wp.Stop()

	// Enqueue some test jobs
	testMonitorIDs := []string{"monitor-1", "monitor-2", "monitor-3"}
	for i, monitorID := range testMonitorIDs {
		job := &redis.Job{
			ID:   fmt.Sprintf("job-%d", i+1),
			Type: "health_check",
			Payload: map[string]interface{}{
				"monitor_id": monitorID,
			},
		}
		err = redisClient.Enqueue(ctx, queueName, job)
		if err != nil {
			t.Fatalf("Failed to enqueue job: %v", err)
		}
	}

	// Wait for jobs to be processed
	time.Sleep(2 * time.Second)

	// Check that all jobs were executed
	executedJobs := healthService.getExecutedJobs()
	if len(executedJobs) != len(testMonitorIDs) {
		t.Errorf("Expected %d jobs to be executed, got %d", len(testMonitorIDs), len(executedJobs))
	}

	// Check that all monitor IDs were processed
	for _, expectedID := range testMonitorIDs {
		found := false
		for _, executedID := range executedJobs {
			if executedID == expectedID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected monitor ID %s to be processed", expectedID)
		}
	}
}

// TestWorkerPool_GetStats tests worker pool statistics
func TestWorkerPool_GetStats(t *testing.T) {
	redisConfig := redis.DefaultConfig()
	redisConfig.DB = 1
	redisClient, err := redis.NewClient(redisConfig)
	if err != nil {
		t.Skipf("Redis not available for testing: %v", err)
	}
	defer redisClient.Close()

	healthService := &mockHealthCheckExecutor{
		executeDelay: 500 * time.Millisecond, // Add delay to keep workers active
	}
	queueName := "test-queue-stats"

	// Clear the queue first
	ctx := context.Background()
	for {
		job, _ := redisClient.Dequeue(ctx, queueName, 100*time.Millisecond)
		if job == nil {
			break
		}
	}

	wp := usecase.NewWorkerPool(redisClient, healthService, queueName)

	// Start worker pool
	err = wp.Start(ctx, 3)
	if err != nil {
		t.Fatalf("Failed to start worker pool: %v", err)
	}
	defer wp.Stop()

	// Enqueue jobs to create queue depth
	for i := 0; i < 5; i++ {
		job := &redis.Job{
			ID:   fmt.Sprintf("job-%d", i+1),
			Type: "health_check",
			Payload: map[string]interface{}{
				"monitor_id": fmt.Sprintf("monitor-%d", i+1),
			},
		}
		err = redisClient.Enqueue(ctx, queueName, job)
		if err != nil {
			t.Fatalf("Failed to enqueue job: %v", err)
		}
	}

	// Wait a bit for workers to start processing
	time.Sleep(100 * time.Millisecond)

	stats := wp.GetStats()

	// Check that stats are reasonable
	if stats.QueueDepth < 0 {
		t.Errorf("Expected non-negative queue depth, got %d", stats.QueueDepth)
	}

	if stats.ActiveWorkers < 0 {
		t.Errorf("Expected non-negative active workers, got %d", stats.ActiveWorkers)
	}

	if stats.ProcessedJobs < 0 {
		t.Errorf("Expected non-negative processed jobs, got %d", stats.ProcessedJobs)
	}

	// Wait for all jobs to complete
	time.Sleep(3 * time.Second)

	finalStats := wp.GetStats()
	if finalStats.ProcessedJobs != 5 {
		t.Errorf("Expected 5 processed jobs, got %d", finalStats.ProcessedJobs)
	}
}
