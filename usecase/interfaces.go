package usecase

import (
	"context"
	"time"

	"web-tracker/domain"
)

// Scheduler defines the interface for job scheduling
type Scheduler interface {
	Start(ctx context.Context) error
	Stop() error
	ScheduleMonitor(monitor *domain.Monitor) error
	UnscheduleMonitor(monitorID string) error
	RescheduleMonitor(monitor *domain.Monitor) error
	IsRunning() bool
}

// WorkerPool defines the interface for worker pool management
type WorkerPool interface {
	Start(ctx context.Context, numWorkers int) error
	Stop() error
	GetStats() WorkerPoolStats
	IsRunning() bool
}

// MetricsService defines the interface for metrics calculation
type MetricsService interface {
	GetUptimePercentage(ctx context.Context, monitorID string) (*UptimeStats, error)
	GetResponseTimeStats(ctx context.Context, monitorID string, period time.Duration) (*ResponseTimeStats, error)
}

// UptimeStats represents uptime statistics for different time periods
type UptimeStats struct {
	Period24h float64 `json:"period_24h"`
	Period7d  float64 `json:"period_7d"`
	Period30d float64 `json:"period_30d"`
}

// ResponseTimeStats represents response time statistics
type ResponseTimeStats struct {
	Average time.Duration `json:"average"`
	Min     time.Duration `json:"min"`
	Max     time.Duration `json:"max"`
	P95     time.Duration `json:"p95"`
	P99     time.Duration `json:"p99"`
}

// DataRetentionService defines the interface for data retention and cleanup
type DataRetentionService interface {
	StartCleanupScheduler(ctx context.Context) error
	StopCleanupScheduler() error
	RunCleanup(ctx context.Context) error
	IsRunning() bool
}

// HealthCheckAggregationService defines the interface for health check aggregation
type HealthCheckAggregationService interface {
	StartAggregationScheduler(ctx context.Context) error
	StopAggregationScheduler() error
	RunAggregation(ctx context.Context) error
	AggregateHour(ctx context.Context, monitorID string, hourStart time.Time) error
	IsRunning() bool
}

// MetricsRedisClient defines the interface for Redis operations needed by MetricsService
type MetricsRedisClient interface {
	GetJSON(ctx context.Context, key string, dest interface{}) (bool, error)
	SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error
}

// WebSocketBroadcaster defines the interface for WebSocket broadcasting
type WebSocketBroadcaster interface {
	// BroadcastHealthCheckUpdate broadcasts a health check update to all connected clients
	BroadcastHealthCheckUpdate(healthCheck *domain.HealthCheck)

	// BroadcastAlert broadcasts an alert to all connected clients
	BroadcastAlert(alert *domain.Alert)
}
