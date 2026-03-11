package tests

import (
	"context"
	"os"
	"testing"
	"time"

	"web-tracker/infrastructure/redis"
)

// TestIntegration runs a comprehensive integration test with Redis.
// This test requires a running Redis instance.
// Run with: go test -v -tags=integration ./infrastructure/redis/
func TestIntegration(t *testing.T) {
	if os.Getenv("REDIS_URL") == "" && os.Getenv("CI") == "" {
		t.Skip("Skipping integration test: set REDIS_URL environment variable to run")
	}

	cfg := redis.DefaultConfig()
	if redisURL := os.Getenv("REDIS_URL"); redisURL != "" {
		cfg.Addr = redisURL
	}

	client, err := redis.NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	t.Run("CacheWorkflow", func(t *testing.T) {
		key := "integration:cache:test"
		defer client.Delete(ctx, key)

		// Set and get
		value := []byte("integration test value")
		if err := client.Set(ctx, key, value, 1*time.Minute); err != nil {
			t.Fatalf("Set failed: %v", err)
		}

		retrieved, err := client.Get(ctx, key)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}

		if string(retrieved) != string(value) {
			t.Errorf("Expected %s, got %s", value, retrieved)
		}
	})

	t.Run("QueueWorkflow", func(t *testing.T) {
		queueName := "integration:queue:test"
		defer client.Delete(ctx, queueName)

		// Enqueue multiple jobs
		for i := 0; i < 3; i++ {
			job := &redis.Job{
				ID:   string(rune('A' + i)),
				Type: "test",
				Payload: map[string]interface{}{
					"index": i,
				},
			}
			if err := client.Enqueue(ctx, queueName, job); err != nil {
				t.Fatalf("Enqueue failed: %v", err)
			}
		}

		// Dequeue and verify order
		for i := 0; i < 3; i++ {
			job, err := client.Dequeue(ctx, queueName, 1*time.Second)
			if err != nil {
				t.Fatalf("Dequeue failed: %v", err)
			}
			if job == nil {
				t.Fatal("Expected job, got nil")
			}
			expectedID := string(rune('A' + i))
			if job.ID != expectedID {
				t.Errorf("Expected job ID %s, got %s", expectedID, job.ID)
			}
		}
	})

	t.Run("ScheduleWorkflow", func(t *testing.T) {
		scheduleName := "integration:schedule:test"
		defer client.Delete(ctx, scheduleName)

		now := time.Now()
		past := now.Add(-1 * time.Minute)
		future := now.Add(1 * time.Hour)

		// Schedule jobs at different times
		pastJob := &redis.Job{ID: "past", Type: "test"}
		futureJob := &redis.Job{ID: "future", Type: "test"}

		if err := client.Schedule(ctx, scheduleName, pastJob, past); err != nil {
			t.Fatalf("Schedule failed: %v", err)
		}
		if err := client.Schedule(ctx, scheduleName, futureJob, future); err != nil {
			t.Fatalf("Schedule failed: %v", err)
		}

		// Get due jobs (should only return past job)
		dueJobs, err := client.GetDueJobs(ctx, scheduleName, now)
		if err != nil {
			t.Fatalf("GetDueJobs failed: %v", err)
		}

		if len(dueJobs) != 1 {
			t.Errorf("Expected 1 due job, got %d", len(dueJobs))
		}
		if len(dueJobs) > 0 && dueJobs[0].ID != "past" {
			t.Errorf("Expected past job, got %s", dueJobs[0].ID)
		}

		// Verify future job is still in schedule
		length, err := client.GetScheduleLength(ctx, scheduleName)
		if err != nil {
			t.Fatalf("GetScheduleLength failed: %v", err)
		}
		if length != 1 {
			t.Errorf("Expected 1 job in schedule, got %d", length)
		}
	})

	t.Run("JSONWorkflow", func(t *testing.T) {
		key := "integration:json:test"
		defer client.Delete(ctx, key)

		type TestStruct struct {
			Name    string   `json:"name"`
			Count   int      `json:"count"`
			Tags    []string `json:"tags"`
			Enabled bool     `json:"enabled"`
		}

		original := TestStruct{
			Name:    "integration test",
			Count:   42,
			Tags:    []string{"test", "integration"},
			Enabled: true,
		}

		// Set JSON
		if err := client.SetJSON(ctx, key, original, 1*time.Minute); err != nil {
			t.Fatalf("SetJSON failed: %v", err)
		}

		// Get JSON
		var retrieved TestStruct
		exists, err := client.GetJSON(ctx, key, &retrieved)
		if err != nil {
			t.Fatalf("GetJSON failed: %v", err)
		}
		if !exists {
			t.Fatal("Expected key to exist")
		}

		// Verify all fields
		if retrieved.Name != original.Name {
			t.Errorf("Name: expected %s, got %s", original.Name, retrieved.Name)
		}
		if retrieved.Count != original.Count {
			t.Errorf("Count: expected %d, got %d", original.Count, retrieved.Count)
		}
		if len(retrieved.Tags) != len(original.Tags) {
			t.Errorf("Tags length: expected %d, got %d", len(original.Tags), len(retrieved.Tags))
		}
		if retrieved.Enabled != original.Enabled {
			t.Errorf("Enabled: expected %v, got %v", original.Enabled, retrieved.Enabled)
		}
	})

	t.Run("ConcurrentOperations", func(t *testing.T) {
		queueName := "integration:concurrent:queue"
		defer client.Delete(ctx, queueName)

		// Enqueue jobs concurrently
		jobCount := 10
		done := make(chan bool, jobCount)

		for i := 0; i < jobCount; i++ {
			go func(index int) {
				job := &redis.Job{
					ID:   string(rune('A' + index)),
					Type: "concurrent",
				}
				if err := client.Enqueue(ctx, queueName, job); err != nil {
					t.Errorf("Concurrent enqueue failed: %v", err)
				}
				done <- true
			}(i)
		}

		// Wait for all enqueues to complete
		for i := 0; i < jobCount; i++ {
			<-done
		}

		// Verify all jobs were enqueued
		length, err := client.GetQueueLength(ctx, queueName)
		if err != nil {
			t.Fatalf("GetQueueLength failed: %v", err)
		}
		if length != int64(jobCount) {
			t.Errorf("Expected %d jobs, got %d", jobCount, length)
		}

		// Dequeue all jobs concurrently
		for i := 0; i < jobCount; i++ {
			go func() {
				_, err := client.Dequeue(ctx, queueName, 1*time.Second)
				if err != nil {
					t.Errorf("Concurrent dequeue failed: %v", err)
				}
				done <- true
			}()
		}

		// Wait for all dequeues to complete
		for i := 0; i < jobCount; i++ {
			<-done
		}

		// Verify queue is empty
		length, err = client.GetQueueLength(ctx, queueName)
		if err != nil {
			t.Fatalf("GetQueueLength failed: %v", err)
		}
		if length != 0 {
			t.Errorf("Expected empty queue, got %d jobs", length)
		}
	})
}
