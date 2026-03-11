package profiling

import (
	"context"
	"fmt"
	"runtime"
	"runtime/debug"
	"time"

	"web-tracker/infrastructure/logger"
)

// MemoryStats holds memory usage statistics
type MemoryStats struct {
	Alloc         uint64  `json:"alloc"`           // bytes allocated and not yet freed
	TotalAlloc    uint64  `json:"total_alloc"`     // bytes allocated (even if freed)
	Sys           uint64  `json:"sys"`             // bytes obtained from system
	Lookups       uint64  `json:"lookups"`         // number of pointer lookups
	Mallocs       uint64  `json:"mallocs"`         // number of mallocs
	Frees         uint64  `json:"frees"`           // number of frees
	HeapAlloc     uint64  `json:"heap_alloc"`      // bytes allocated and not yet freed (same as Alloc)
	HeapSys       uint64  `json:"heap_sys"`        // bytes obtained from system
	HeapIdle      uint64  `json:"heap_idle"`       // bytes in idle spans
	HeapInuse     uint64  `json:"heap_inuse"`      // bytes in non-idle span
	HeapReleased  uint64  `json:"heap_released"`   // bytes released to the OS
	HeapObjects   uint64  `json:"heap_objects"`    // total number of allocated objects
	StackInuse    uint64  `json:"stack_inuse"`     // bytes used by stack spans
	StackSys      uint64  `json:"stack_sys"`       // bytes obtained from system for stack
	MSpanInuse    uint64  `json:"mspan_inuse"`     // bytes used by mspan structures
	MSpanSys      uint64  `json:"mspan_sys"`       // bytes obtained from system for mspan
	MCacheInuse   uint64  `json:"mcache_inuse"`    // bytes used by mcache structures
	MCacheSys     uint64  `json:"mcache_sys"`      // bytes obtained from system for mcache
	GCSys         uint64  `json:"gc_sys"`          // bytes used for garbage collection system metadata
	OtherSys      uint64  `json:"other_sys"`       // bytes used for other system allocations
	NextGC        uint64  `json:"next_gc"`         // next collection will happen when HeapAlloc ≥ this amount
	LastGC        uint64  `json:"last_gc"`         // time of last collection (nanoseconds since 1970)
	PauseTotalNs  uint64  `json:"pause_total_ns"`  // cumulative nanoseconds in GC stop-the-world pauses
	NumGC         uint32  `json:"num_gc"`          // number of completed GC cycles
	NumForcedGC   uint32  `json:"num_forced_gc"`   // number of GC cycles that were forced by the application
	GCCPUFraction float64 `json:"gc_cpu_fraction"` // fraction of CPU time used by GC
}

// MemoryProfiler provides memory profiling and optimization utilities
type MemoryProfiler struct {
	log *logger.Logger
}

// NewMemoryProfiler creates a new memory profiler
func NewMemoryProfiler() *MemoryProfiler {
	return &MemoryProfiler{
		log: logger.GetLogger(),
	}
}

// GetMemoryStats returns current memory statistics
func (mp *MemoryProfiler) GetMemoryStats() *MemoryStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return &MemoryStats{
		Alloc:         m.Alloc,
		TotalAlloc:    m.TotalAlloc,
		Sys:           m.Sys,
		Lookups:       m.Lookups,
		Mallocs:       m.Mallocs,
		Frees:         m.Frees,
		HeapAlloc:     m.HeapAlloc,
		HeapSys:       m.HeapSys,
		HeapIdle:      m.HeapIdle,
		HeapInuse:     m.HeapInuse,
		HeapReleased:  m.HeapReleased,
		HeapObjects:   m.HeapObjects,
		StackInuse:    m.StackInuse,
		StackSys:      m.StackSys,
		MSpanInuse:    m.MSpanInuse,
		MSpanSys:      m.MSpanSys,
		MCacheInuse:   m.MCacheInuse,
		MCacheSys:     m.MCacheSys,
		GCSys:         m.GCSys,
		OtherSys:      m.OtherSys,
		NextGC:        m.NextGC,
		LastGC:        m.LastGC,
		PauseTotalNs:  m.PauseTotalNs,
		NumGC:         m.NumGC,
		NumForcedGC:   m.NumForcedGC,
		GCCPUFraction: m.GCCPUFraction,
	}
}

// GetMemoryUsageMB returns current memory usage in megabytes
func (mp *MemoryProfiler) GetMemoryUsageMB() float64 {
	stats := mp.GetMemoryStats()
	return float64(stats.Alloc) / 1024 / 1024
}

// LogMemoryStats logs current memory statistics
func (mp *MemoryProfiler) LogMemoryStats(context string) {
	stats := mp.GetMemoryStats()
	mp.log.Info("Memory statistics", logger.Fields{
		"context":         context,
		"alloc_mb":        float64(stats.Alloc) / 1024 / 1024,
		"sys_mb":          float64(stats.Sys) / 1024 / 1024,
		"heap_alloc_mb":   float64(stats.HeapAlloc) / 1024 / 1024,
		"heap_sys_mb":     float64(stats.HeapSys) / 1024 / 1024,
		"heap_objects":    stats.HeapObjects,
		"num_gc":          stats.NumGC,
		"gc_cpu_fraction": stats.GCCPUFraction,
	})
}

// ForceGC forces a garbage collection and logs the results
func (mp *MemoryProfiler) ForceGC() {
	beforeStats := mp.GetMemoryStats()
	runtime.GC()
	afterStats := mp.GetMemoryStats()

	mp.log.Info("Forced garbage collection", logger.Fields{
		"before_alloc_mb": float64(beforeStats.Alloc) / 1024 / 1024,
		"after_alloc_mb":  float64(afterStats.Alloc) / 1024 / 1024,
		"freed_mb":        float64(beforeStats.Alloc-afterStats.Alloc) / 1024 / 1024,
		"gc_cycles":       afterStats.NumGC - beforeStats.NumGC,
	})
}

// SetGCPercent sets the garbage collection target percentage
// A lower value means more frequent GC, which can reduce memory usage but increase CPU usage
func (mp *MemoryProfiler) SetGCPercent(percent int) int {
	oldPercent := debug.SetGCPercent(percent)
	mp.log.Info("GC target percentage changed", logger.Fields{
		"old_percent": oldPercent,
		"new_percent": percent,
	})
	return oldPercent
}

// SetMemoryLimit sets a soft memory limit for the Go runtime
// This helps prevent excessive memory usage
func (mp *MemoryProfiler) SetMemoryLimit(limitMB int64) int64 {
	limitBytes := limitMB * 1024 * 1024
	oldLimit := debug.SetMemoryLimit(limitBytes)
	mp.log.Info("Memory limit set", logger.Fields{
		"old_limit_mb": oldLimit / 1024 / 1024,
		"new_limit_mb": limitMB,
	})
	return oldLimit / 1024 / 1024
}

// StartMemoryMonitoring starts a background goroutine that monitors memory usage
func (mp *MemoryProfiler) StartMemoryMonitoring(ctx context.Context, interval time.Duration, maxMemoryMB float64) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				currentMemoryMB := mp.GetMemoryUsageMB()

				if currentMemoryMB > maxMemoryMB {
					mp.log.Warn("Memory usage exceeds threshold", logger.Fields{
						"current_mb":   currentMemoryMB,
						"threshold_mb": maxMemoryMB,
					})

					// Force garbage collection if memory usage is high
					mp.ForceGC()
				}

				// Log memory stats every 10 intervals (reduce log noise)
				if time.Now().Unix()%10 == 0 {
					mp.LogMemoryStats("periodic_monitoring")
				}
			}
		}
	}()
}

// OptimizeForLowMemory applies memory optimization settings for low-memory environments
func (mp *MemoryProfiler) OptimizeForLowMemory() {
	// Set more aggressive GC (default is 100, lower means more frequent GC)
	mp.SetGCPercent(50)

	// Set memory limit to 150MB (with some buffer above the 100MB requirement)
	mp.SetMemoryLimit(150)

	// Force an initial GC
	mp.ForceGC()

	mp.log.Info("Applied low-memory optimizations", logger.Fields{
		"gc_percent":      50,
		"memory_limit_mb": 150,
	})
}

// CheckMemoryRequirements validates that memory usage meets the specified requirements
func (mp *MemoryProfiler) CheckMemoryRequirements(maxMemoryMB float64, context string) bool {
	currentMemoryMB := mp.GetMemoryUsageMB()

	if currentMemoryMB <= maxMemoryMB {
		mp.log.Info("Memory requirement check passed", logger.Fields{
			"context":     context,
			"current_mb":  currentMemoryMB,
			"required_mb": maxMemoryMB,
			"status":      "PASS",
		})
		return true
	}

	mp.log.Warn("Memory requirement check failed", logger.Fields{
		"context":     context,
		"current_mb":  currentMemoryMB,
		"required_mb": maxMemoryMB,
		"excess_mb":   currentMemoryMB - maxMemoryMB,
		"status":      "FAIL",
	})
	return false
}

// GetMemoryUsageReport returns a formatted memory usage report
func (mp *MemoryProfiler) GetMemoryUsageReport() string {
	stats := mp.GetMemoryStats()

	return fmt.Sprintf(`Memory Usage Report:
  Allocated: %.2f MB
  System: %.2f MB
  Heap Allocated: %.2f MB
  Heap System: %.2f MB
  Heap Objects: %d
  GC Cycles: %d
  GC CPU Fraction: %.4f
  Stack In Use: %.2f MB`,
		float64(stats.Alloc)/1024/1024,
		float64(stats.Sys)/1024/1024,
		float64(stats.HeapAlloc)/1024/1024,
		float64(stats.HeapSys)/1024/1024,
		stats.HeapObjects,
		stats.NumGC,
		stats.GCCPUFraction,
		float64(stats.StackInuse)/1024/1024,
	)
}
