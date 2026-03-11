package tests

import (
	"context"
	"testing"
	"time"

	"web-tracker/infrastructure/redis"
)

// TestNewClient tests creating a new Redis client.
func TestRedis_NewClient(t *testing.T) {
	// Skip if Redis is not available
	cfg := redis.DefaultConfig()
	client, err := redis.NewClient(cfg)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	defer client.Close()

	// Test ping
	ctx := context.Background()
	if err := client.Ping(ctx); err != nil {
		t.Errorf("Ping failed: %v", err)
	}
}

// TestCacheOperations tests basic cache operations (Get, Set, Delete).
func TestCacheOperations(t *testing.T) {
	cfg := redis.DefaultConfig()
	client, err := redis.NewClient(cfg)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	key := "test:cache:key"
	value := []byte("test value")

	// Clean up before test
	_ = client.Delete(ctx, key)

	// Test Get on non-existent key
	result, err := client.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if result != nil {
		t.Errorf("Expected nil for non-existent key, got %v", result)
	}

	// Test Set
	if err := client.Set(ctx, key, value, 10*time.Second); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Test Get on existing key
	result, err = client.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if string(result) != string(value) {
		t.Errorf("Expected %s, got %s", value, result)
	}

	// Test Delete
	if err := client.Delete(ctx, key); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deletion
	result, err = client.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if result != nil {
		t.Errorf("Expected nil after deletion, got %v", result)
	}
}

// TestCacheTTL tests that cache entries expire after TTL.
func TestCacheTTL(t *testing.T) {
	cfg := redis.DefaultConfig()
	client, err := redis.NewClient(cfg)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	key := "test:cache:ttl"
	value := []byte("expires soon")

	// Clean up before test
	_ = client.Delete(ctx, key)

	// Set with 1 second TTL
	if err := client.Set(ctx, key, value, 1*time.Second); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Verify it exists
	result, err := client.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if result == nil {
		t.Fatal("Expected value to exist")
	}

	// Wait for expiration
	time.Sleep(1500 * time.Millisecond)

	// Verify it expired
	result, err = client.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if result != nil {
		t.Errorf("Expected nil after TTL expiration, got %v", result)
	}
}

// TestJSONOperations tests JSON cache operations.
func TestJSONOperations(t *testing.T) {
	cfg := redis.DefaultConfig()
	client, err := redis.NewClient(cfg)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	key := "test:cache:json"

	// Clean up before test
	_ = client.Delete(ctx, key)

	type TestData struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	original := TestData{Name: "test", Count: 42}

	// Test SetJSON
	if err := client.SetJSON(ctx, key, original, 10*time.Second); err != nil {
		t.Fatalf("SetJSON failed: %v", err)
	}

	// Test GetJSON
	var retrieved TestData
	exists, err := client.GetJSON(ctx, key, &retrieved)
	if err != nil {
		t.Fatalf("GetJSON failed: %v", err)
	}
	if !exists {
		t.Fatal("Expected key to exist")
	}
	if retrieved.Name != original.Name || retrieved.Count != original.Count {
		t.Errorf("Expected %+v, got %+v", original, retrieved)
	}

	// Test GetJSON on non-existent key
	_ = client.Delete(ctx, key)
	exists, err = client.GetJSON(ctx, key, &retrieved)
	if err != nil {
		t.Fatalf("GetJSON failed: %v", err)
	}
	if exists {
		t.Error("Expected key to not exist")
	}
}

// TestEnqueueDequeue tests job queue operations.
func TestEnqueueDequeue(t *testing.T) {
	cfg := redis.DefaultConfig()
	client, err := redis.NewClient(cfg)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	queueName := "test:queue"

	// Clean up before test
	_ = client.Delete(ctx, queueName)

	// Test Enqueue
	job := &redis.Job{
		ID:   "job-1",
		Type: "health_check",
		Payload: map[string]interface{}{
			"monitor_id": "mon-123",
		},
	}

	if err := client.Enqueue(ctx, queueName, job); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	// Verify queue length
	length, err := client.GetQueueLength(ctx, queueName)
	if err != nil {
		t.Fatalf("GetQueueLength failed: %v", err)
	}
	if length != 1 {
		t.Errorf("Expected queue length 1, got %d", length)
	}

	// Test Dequeue
	dequeued, err := client.Dequeue(ctx, queueName, 1*time.Second)
	if err != nil {
		t.Fatalf("Dequeue failed: %v", err)
	}
	if dequeued == nil {
		t.Fatal("Expected job, got nil")
	}
	if dequeued.ID != job.ID || dequeued.Type != job.Type {
		t.Errorf("Expected job %+v, got %+v", job, dequeued)
	}

	// Verify queue is empty
	length, err = client.GetQueueLength(ctx, queueName)
	if err != nil {
		t.Fatalf("GetQueueLength failed: %v", err)
	}
	if length != 0 {
		t.Errorf("Expected queue length 0, got %d", length)
	}

	// Test Dequeue on empty queue (should timeout and return nil)
	dequeued, err = client.Dequeue(ctx, queueName, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Dequeue failed: %v", err)
	}
	if dequeued != nil {
		t.Errorf("Expected nil for empty queue, got %+v", dequeued)
	}
}

// TestScheduleOperations tests job scheduling operations.
func TestScheduleOperations(t *testing.T) {
	cfg := redis.DefaultConfig()
	client, err := redis.NewClient(cfg)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	scheduleName := "test:schedule"

	// Clean up before test
	_ = client.Delete(ctx, scheduleName)

	now := time.Now()
	future := now.Add(10 * time.Second)
	past := now.Add(-10 * time.Second)

	// Schedule jobs
	job1 := &redis.Job{ID: "job-1", Type: "test"}
	job2 := &redis.Job{ID: "job-2", Type: "test"}
	job3 := &redis.Job{ID: "job-3", Type: "test"}

	if err := client.Schedule(ctx, scheduleName, job1, past); err != nil {
		t.Fatalf("Schedule failed: %v", err)
	}
	if err := client.Schedule(ctx, scheduleName, job2, past); err != nil {
		t.Fatalf("Schedule failed: %v", err)
	}
	if err := client.Schedule(ctx, scheduleName, job3, future); err != nil {
		t.Fatalf("Schedule failed: %v", err)
	}

	// Verify schedule length
	length, err := client.GetScheduleLength(ctx, scheduleName)
	if err != nil {
		t.Fatalf("GetScheduleLength failed: %v", err)
	}
	if length != 3 {
		t.Errorf("Expected schedule length 3, got %d", length)
	}

	// Get due jobs (should return job1 and job2)
	dueJobs, err := client.GetDueJobs(ctx, scheduleName, now)
	if err != nil {
		t.Fatalf("GetDueJobs failed: %v", err)
	}
	if len(dueJobs) != 2 {
		t.Errorf("Expected 2 due jobs, got %d", len(dueJobs))
	}

	// Verify jobs were removed from schedule
	length, err = client.GetScheduleLength(ctx, scheduleName)
	if err != nil {
		t.Fatalf("GetScheduleLength failed: %v", err)
	}
	if length != 1 {
		t.Errorf("Expected schedule length 1 after getting due jobs, got %d", length)
	}

	// Get due jobs again (should return nothing)
	dueJobs, err = client.GetDueJobs(ctx, scheduleName, now)
	if err != nil {
		t.Fatalf("GetDueJobs failed: %v", err)
	}
	if len(dueJobs) != 0 {
		t.Errorf("Expected 0 due jobs, got %d", len(dueJobs))
	}

	// Get due jobs in the future (should return job3)
	dueJobs, err = client.GetDueJobs(ctx, scheduleName, future.Add(1*time.Second))
	if err != nil {
		t.Fatalf("GetDueJobs failed: %v", err)
	}
	if len(dueJobs) != 1 {
		t.Errorf("Expected 1 due job, got %d", len(dueJobs))
	}
	if len(dueJobs) > 0 && dueJobs[0].ID != "job-3" {
		t.Errorf("Expected job-3, got %s", dueJobs[0].ID)
	}
}

// TestRemoveScheduledJob tests removing a specific job from the schedule.
func TestRemoveScheduledJob(t *testing.T) {
	cfg := redis.DefaultConfig()
	client, err := redis.NewClient(cfg)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	scheduleName := "test:schedule:remove"

	// Clean up before test
	_ = client.Delete(ctx, scheduleName)

	future := time.Now().Add(10 * time.Second)

	job := &redis.Job{ID: "job-remove", Type: "test"}

	// Schedule job
	if err := client.Schedule(ctx, scheduleName, job, future); err != nil {
		t.Fatalf("Schedule failed: %v", err)
	}

	// Verify it's in the schedule
	length, err := client.GetScheduleLength(ctx, scheduleName)
	if err != nil {
		t.Fatalf("GetScheduleLength failed: %v", err)
	}
	if length != 1 {
		t.Errorf("Expected schedule length 1, got %d", length)
	}

	// Remove the job
	if err := client.RemoveScheduledJob(ctx, scheduleName, job); err != nil {
		t.Fatalf("RemoveScheduledJob failed: %v", err)
	}

	// Verify it's removed
	length, err = client.GetScheduleLength(ctx, scheduleName)
	if err != nil {
		t.Fatalf("GetScheduleLength failed: %v", err)
	}
	if length != 0 {
		t.Errorf("Expected schedule length 0 after removal, got %d", length)
	}
}

// TestMultipleJobs tests enqueueing and dequeueing multiple jobs.
func TestMultipleJobs(t *testing.T) {
	cfg := redis.DefaultConfig()
	client, err := redis.NewClient(cfg)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	queueName := "test:queue:multiple"

	// Clean up before test
	_ = client.Delete(ctx, queueName)

	// Enqueue multiple jobs
	jobCount := 5
	for i := 0; i < jobCount; i++ {
		job := &redis.Job{
			ID:   string(rune('A' + i)),
			Type: "test",
		}
		if err := client.Enqueue(ctx, queueName, job); err != nil {
			t.Fatalf("Enqueue failed: %v", err)
		}
	}

	// Verify queue length
	length, err := client.GetQueueLength(ctx, queueName)
	if err != nil {
		t.Fatalf("GetQueueLength failed: %v", err)
	}
	if length != int64(jobCount) {
		t.Errorf("Expected queue length %d, got %d", jobCount, length)
	}

	// Dequeue all jobs
	for i := 0; i < jobCount; i++ {
		job, err := client.Dequeue(ctx, queueName, 1*time.Second)
		if err != nil {
			t.Fatalf("Dequeue failed: %v", err)
		}
		if job == nil {
			t.Fatalf("Expected job %d, got nil", i)
		}
	}

	// Verify queue is empty
	length, err = client.GetQueueLength(ctx, queueName)
	if err != nil {
		t.Fatalf("GetQueueLength failed: %v", err)
	}
	if length != 0 {
		t.Errorf("Expected queue length 0, got %d", length)
	}
}
