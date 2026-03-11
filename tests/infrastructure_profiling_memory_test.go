package tests

import (
	"context"
	"strings"
	"testing"
	"time"

	"web-tracker/infrastructure/profiling"
)

func TestMemoryProfiler(t *testing.T) {
	profiler := profiling.NewMemoryProfiler()

	// Test getting memory stats
	stats := profiler.GetMemoryStats()
	if stats == nil {
		t.Fatal("GetMemoryStats returned nil")
	}

	if stats.Alloc == 0 {
		t.Error("Expected non-zero allocated memory")
	}

	// Test memory usage in MB
	memoryMB := profiler.GetMemoryUsageMB()
	if memoryMB <= 0 {
		t.Error("Expected positive memory usage in MB")
	}

	// Test memory requirements check
	// Should pass for a reasonable limit
	if !profiler.CheckMemoryRequirements(1000.0, "test") {
		t.Error("Expected memory requirement check to pass for 1000MB limit")
	}

	// Should fail for an unreasonably low limit
	if profiler.CheckMemoryRequirements(0.1, "test") {
		t.Error("Expected memory requirement check to fail for 0.1MB limit")
	}
}

func TestMemoryOptimizations(t *testing.T) {
	profiler := profiling.NewMemoryProfiler()

	// Get initial memory usage
	initialMemory := profiler.GetMemoryUsageMB()

	// Apply optimizations
	profiler.OptimizeForLowMemory()

	// Force GC to see the effect
	profiler.ForceGC()

	// Memory usage might not decrease immediately, but optimizations should be applied
	// The test mainly verifies that the optimization functions don't panic
	finalMemory := profiler.GetMemoryUsageMB()

	t.Logf("Initial memory: %.2f MB, Final memory: %.2f MB", initialMemory, finalMemory)

	// Verify that memory usage is reasonable (less than 50MB for a simple test)
	if finalMemory > 50.0 {
		t.Errorf("Memory usage too high after optimization: %.2f MB", finalMemory)
	}
}

func TestMemoryMonitoring(t *testing.T) {
	profiler := profiling.NewMemoryProfiler()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Start monitoring with a very low threshold to trigger warnings
	profiler.StartMemoryMonitoring(ctx, 100*time.Millisecond, 1.0)

	// Wait for monitoring to run
	time.Sleep(500 * time.Millisecond)

	// The test mainly verifies that monitoring doesn't panic
	// and runs without errors
}

func TestMemoryReport(t *testing.T) {
	profiler := profiling.NewMemoryProfiler()

	report := profiler.GetMemoryUsageReport()
	if report == "" {
		t.Error("Expected non-empty memory usage report")
	}

	// Verify report contains expected sections
	expectedSections := []string{
		"Memory Usage Report:",
		"Allocated:",
		"System:",
		"Heap Allocated:",
		"GC Cycles:",
	}

	for _, section := range expectedSections {
		if !strings.Contains(report, section) {
			t.Errorf("Memory report missing expected section: %s", section)
		}
	}
}

// contains helper is replaced by strings.Contains.
