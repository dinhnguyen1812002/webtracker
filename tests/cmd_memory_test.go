package tests

import (
	"testing"
	"time"

	"web-tracker/config"
	"web-tracker/infrastructure/profiling"
)

// TestMemoryRequirements verifies that the application meets memory usage requirements
func TestMemoryRequirements(t *testing.T) {
	// Initialize memory profiler
	profiler := profiling.NewMemoryProfiler()
	profiler.OptimizeForLowMemory()

	// Test idle memory usage (Requirement 12.2: <10MB when idle)
	profiler.ForceGC()
	time.Sleep(100 * time.Millisecond) // Allow GC to complete

	idleMemoryMB := profiler.GetMemoryUsageMB()
	t.Logf("Idle memory usage: %.2f MB", idleMemoryMB)

	if idleMemoryMB > 10.0 {
		t.Errorf("Idle memory usage exceeds requirement: %.2f MB > 10 MB", idleMemoryMB)
	}

	// Simulate some application activity
	cfg := config.DefaultConfig()

	// Create some objects to simulate application load
	for i := 0; i < 100; i++ {
		_ = config.DefaultConfig()
	}

	// Check memory after activity
	activeMemoryMB := profiler.GetMemoryUsageMB()
	t.Logf("Active memory usage: %.2f MB", activeMemoryMB)

	// For this test, we'll use a reasonable limit since we're not running the full application
	if activeMemoryMB > 50.0 {
		t.Errorf("Active memory usage too high: %.2f MB > 50 MB", activeMemoryMB)
	}

	// Test memory optimization functions
	if !profiler.CheckMemoryRequirements(100.0, "test_load") {
		t.Error("Memory requirement check failed for reasonable limit")
	}

	// Log memory report
	report := profiler.GetMemoryUsageReport()
	t.Logf("Memory report:\n%s", report)

	// Verify configuration optimizations
	if cfg.Database.MaxConnections != 20 {
		t.Errorf("Database max connections not optimized: %d != 20", cfg.Database.MaxConnections)
	}

	if cfg.Redis.PoolSize != 5 {
		t.Errorf("Redis pool size not optimized: %d != 5", cfg.Redis.PoolSize)
	}

	if cfg.Worker.PoolSize != 8 {
		t.Errorf("Worker pool size not optimized: %d != 8", cfg.Worker.PoolSize)
	}

	if cfg.Worker.QueueSize != 500 {
		t.Errorf("Worker queue size not optimized: %d != 500", cfg.Worker.QueueSize)
	}
}

// TestConfigurationOptimizations verifies that all configuration values are optimized for memory efficiency
func TestConfigurationOptimizations(t *testing.T) {
	cfg := config.DefaultConfig()

	// Database optimizations
	if cfg.Database.MaxConnections != 20 {
		t.Errorf("Database MaxConnections should be 20, got %d", cfg.Database.MaxConnections)
	}

	if cfg.Database.MinConnections != 2 {
		t.Errorf("Database MinConnections should be 2, got %d", cfg.Database.MinConnections)
	}

	// Redis optimizations
	if cfg.Redis.PoolSize != 5 {
		t.Errorf("Redis PoolSize should be 5, got %d", cfg.Redis.PoolSize)
	}

	if cfg.Redis.MinIdleConns != 1 {
		t.Errorf("Redis MinIdleConns should be 1, got %d", cfg.Redis.MinIdleConns)
	}

	// Worker pool optimizations
	if cfg.Worker.PoolSize != 8 {
		t.Errorf("Worker PoolSize should be 8, got %d", cfg.Worker.PoolSize)
	}

	if cfg.Worker.QueueSize != 500 {
		t.Errorf("Worker QueueSize should be 500, got %d", cfg.Worker.QueueSize)
	}

	t.Logf("All configuration optimizations verified successfully")
}
