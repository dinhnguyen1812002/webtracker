package tests

import (
	"context"
	"testing"
	"time"

	"web-tracker/domain"
	"web-tracker/infrastructure/redis"
	"web-tracker/usecase"
)

func TestScheduler_ScheduleMonitor(t *testing.T) {
	// Create a test Redis client (you might want to use a test Redis instance)
	redisClient, err := redis.NewClient(redis.DefaultConfig())
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	defer redisClient.Close()

	config := usecase.SchedulerConfig{
		ScheduleName: "test:schedule:health_checks",
		QueueName:    "test:queue:health_checks",
		TickInterval: 1 * time.Second,
	}

	scheduler := usecase.NewScheduler(redisClient, config)

	// Test monitor
	monitor := &domain.Monitor{
		ID:            "test-monitor-1",
		Name:          "Test Monitor",
		URL:           "https://example.com",
		CheckInterval: domain.CheckInterval5Min,
		Enabled:       true,
	}

	// Test scheduling
	err = scheduler.ScheduleMonitor(monitor)
	if err != nil {
		t.Fatalf("Failed to schedule monitor: %v", err)
	}

	// Verify job was scheduled
	length, err := redisClient.GetScheduleLength(context.Background(), config.ScheduleName)
	if err != nil {
		t.Fatalf("Failed to get schedule length: %v", err)
	}

	if length != 1 {
		t.Errorf("Expected 1 scheduled job, got %d", length)
	}

	// Clean up
	redisClient.Delete(context.Background(), config.ScheduleName)
}

// calculateNextExecutionWithJitter is unexported; jitter behavior is validated indirectly via scheduling integration tests.

func TestScheduler_StartStop(t *testing.T) {
	redisClient, err := redis.NewClient(redis.DefaultConfig())
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	defer redisClient.Close()

	config := usecase.SchedulerConfig{
		ScheduleName: "test:schedule",
		QueueName:    "test:queue",
		TickInterval: 100 * time.Millisecond,
	}

	scheduler := usecase.NewScheduler(redisClient, config)

	// Test start
	ctx := context.Background()
	err = scheduler.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	if !scheduler.IsRunning() {
		t.Error("Scheduler should be running after start")
	}

	// Test double start (should fail)
	err = scheduler.Start(ctx)
	if err == nil {
		t.Error("Expected error when starting already running scheduler")
	}

	// Test stop
	err = scheduler.Stop()
	if err != nil {
		t.Fatalf("Failed to stop scheduler: %v", err)
	}

	if scheduler.IsRunning() {
		t.Error("Scheduler should not be running after stop")
	}

	// Test double stop (should not fail)
	err = scheduler.Stop()
	if err != nil {
		t.Errorf("Unexpected error when stopping already stopped scheduler: %v", err)
	}
}

func TestScheduler_DisabledMonitor(t *testing.T) {
	redisClient, err := redis.NewClient(redis.DefaultConfig())
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	defer redisClient.Close()

	config := usecase.SchedulerConfig{
		ScheduleName: "test:schedule:disabled",
		QueueName:    "test:queue:disabled",
	}

	scheduler := usecase.NewScheduler(redisClient, config)

	// Test disabled monitor
	monitor := &domain.Monitor{
		ID:            "test-monitor-disabled",
		Name:          "Disabled Monitor",
		URL:           "https://example.com",
		CheckInterval: domain.CheckInterval5Min,
		Enabled:       false, // Disabled
	}

	// Test scheduling disabled monitor
	err = scheduler.ScheduleMonitor(monitor)
	if err != nil {
		t.Fatalf("Failed to schedule disabled monitor: %v", err)
	}

	// Verify no job was scheduled for disabled monitor
	length, err := redisClient.GetScheduleLength(context.Background(), config.ScheduleName)
	if err != nil {
		t.Fatalf("Failed to get schedule length: %v", err)
	}

	if length != 0 {
		t.Errorf("Expected 0 scheduled jobs for disabled monitor, got %d", length)
	}
}
