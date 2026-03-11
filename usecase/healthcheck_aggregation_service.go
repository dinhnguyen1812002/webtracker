package usecase

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"web-tracker/domain"

	"github.com/google/uuid"
)

// healthCheckAggregationService implements HealthCheckAggregationService
type healthCheckAggregationService struct {
	healthCheckRepo           domain.HealthCheckRepository
	aggregatedHealthCheckRepo domain.AggregatedHealthCheckRepository
	monitorRepo               domain.MonitorRepository
	aggregationThreshold      time.Duration // Age threshold for aggregation (7 days)
	aggregationInterval       time.Duration // How often to run aggregation (daily)
	ticker                    *time.Ticker
	stopCh                    chan struct{}
	running                   bool
	mu                        sync.RWMutex
}

// NewHealthCheckAggregationService creates a new health check aggregation service
func NewHealthCheckAggregationService(
	healthCheckRepo domain.HealthCheckRepository,
	aggregatedHealthCheckRepo domain.AggregatedHealthCheckRepository,
	monitorRepo domain.MonitorRepository,
) HealthCheckAggregationService {
	return &healthCheckAggregationService{
		healthCheckRepo:           healthCheckRepo,
		aggregatedHealthCheckRepo: aggregatedHealthCheckRepo,
		monitorRepo:               monitorRepo,
		aggregationThreshold:      7 * 24 * time.Hour, // 7 days
		aggregationInterval:       24 * time.Hour,     // Daily aggregation
		stopCh:                    make(chan struct{}),
	}
}

// StartAggregationScheduler starts the scheduled aggregation process
func (s *healthCheckAggregationService) StartAggregationScheduler(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("aggregation scheduler is already running")
	}

	s.ticker = time.NewTicker(s.aggregationInterval)
	s.running = true

	go s.aggregationLoop(ctx)

	log.Printf("Health check aggregation scheduler started (interval: %v, threshold: %v)",
		s.aggregationInterval, s.aggregationThreshold)

	return nil
}

// StopAggregationScheduler stops the scheduled aggregation process
func (s *healthCheckAggregationService) StopAggregationScheduler() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return fmt.Errorf("aggregation scheduler is not running")
	}

	close(s.stopCh)
	s.ticker.Stop()
	s.running = false

	log.Printf("Health check aggregation scheduler stopped")

	return nil
}

// RunAggregation performs a single aggregation operation for all monitors
func (s *healthCheckAggregationService) RunAggregation(ctx context.Context) error {
	cutoffTime := time.Now().Add(-s.aggregationThreshold)

	log.Printf("Starting health check aggregation for data older than %v", cutoffTime)

	// Get all monitors
	monitors, err := s.monitorRepo.List(ctx, domain.ListFilters{Limit: 1000})
	if err != nil {
		return fmt.Errorf("failed to get monitors for aggregation: %w", err)
	}

	// Aggregate data for each monitor
	for _, monitor := range monitors {
		if err := s.aggregateMonitorData(ctx, monitor.ID, cutoffTime); err != nil {
			log.Printf("Failed to aggregate data for monitor %s: %v", monitor.ID, err)
			// Continue with other monitors even if one fails
		}
	}

	log.Printf("Health check aggregation completed successfully")

	return nil
}

// AggregateHour aggregates health checks for a specific monitor and hour
func (s *healthCheckAggregationService) AggregateHour(ctx context.Context, monitorID string, hourStart time.Time) error {
	if monitorID == "" {
		return fmt.Errorf("monitor ID is required")
	}

	// Truncate to hour boundary
	hourStart = hourStart.Truncate(time.Hour)
	hourEnd := hourStart.Add(time.Hour)

	// Get health checks for this hour
	checks, err := s.healthCheckRepo.GetByDateRange(ctx, monitorID, hourStart, hourEnd)
	if err != nil {
		return fmt.Errorf("failed to get health checks for aggregation: %w", err)
	}

	if len(checks) == 0 {
		// No checks to aggregate
		return nil
	}

	// Calculate aggregated statistics
	aggregated := &domain.AggregatedHealthCheck{
		ID:            uuid.New().String(),
		MonitorID:     monitorID,
		HourTimestamp: hourStart,
		TotalChecks:   len(checks),
		CreatedAt:     time.Now(),
	}

	var totalResponseTime time.Duration
	var minResponseTime time.Duration = time.Duration(1<<63 - 1) // Max duration
	var maxResponseTime time.Duration

	for _, check := range checks {
		if check.IsSuccessful() {
			aggregated.SuccessfulChecks++
			totalResponseTime += check.ResponseTime

			if check.ResponseTime < minResponseTime {
				minResponseTime = check.ResponseTime
			}
			if check.ResponseTime > maxResponseTime {
				maxResponseTime = check.ResponseTime
			}
		} else {
			aggregated.FailedChecks++
		}
	}

	// Calculate averages and rates
	aggregated.CalculateSuccessRate()

	if aggregated.SuccessfulChecks > 0 {
		aggregated.AvgResponseTime = totalResponseTime / time.Duration(aggregated.SuccessfulChecks)
		aggregated.MinResponseTime = minResponseTime
		aggregated.MaxResponseTime = maxResponseTime
	}

	// Save aggregated data
	if err := s.aggregatedHealthCheckRepo.Create(ctx, aggregated); err != nil {
		return fmt.Errorf("failed to save aggregated health check: %w", err)
	}

	log.Printf("Aggregated %d health checks for monitor %s at hour %v",
		len(checks), monitorID, hourStart)

	return nil
}

// IsRunning returns whether the aggregation scheduler is currently running
func (s *healthCheckAggregationService) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// aggregationLoop runs the aggregation process on a schedule
func (s *healthCheckAggregationService) aggregationLoop(ctx context.Context) {
	// Run initial aggregation
	if err := s.RunAggregation(ctx); err != nil {
		log.Printf("Initial aggregation failed: %v", err)
	}

	for {
		select {
		case <-s.ticker.C:
			if err := s.RunAggregation(ctx); err != nil {
				log.Printf("Scheduled aggregation failed: %v", err)
			}
		case <-s.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// aggregateMonitorData aggregates health check data for a specific monitor
func (s *healthCheckAggregationService) aggregateMonitorData(ctx context.Context, monitorID string, cutoffTime time.Time) error {
	// Start from 7 days ago, truncated to hour boundary
	startTime := cutoffTime.Truncate(time.Hour)

	// Aggregate hour by hour until we reach the cutoff time
	for hourStart := startTime; hourStart.Before(cutoffTime); hourStart = hourStart.Add(time.Hour) {
		if err := s.AggregateHour(ctx, monitorID, hourStart); err != nil {
			return fmt.Errorf("failed to aggregate hour %v for monitor %s: %w", hourStart, monitorID, err)
		}
	}

	// After successful aggregation, delete the original health checks
	if err := s.deleteAggregatedHealthChecks(ctx, monitorID, cutoffTime); err != nil {
		log.Printf("Warning: failed to delete original health checks for monitor %s: %v", monitorID, err)
		// Don't return error here as aggregation was successful
	}

	return nil
}

// deleteAggregatedHealthChecks deletes health checks that have been successfully aggregated
func (s *healthCheckAggregationService) deleteAggregatedHealthChecks(ctx context.Context, monitorID string, before time.Time) error {
	// Get health checks to delete
	checks, err := s.healthCheckRepo.GetByDateRange(ctx, monitorID, time.Time{}, before)
	if err != nil {
		return fmt.Errorf("failed to get health checks for deletion: %w", err)
	}

	if len(checks) == 0 {
		return nil
	}

	// Delete the health checks
	if err := s.healthCheckRepo.DeleteOlderThan(ctx, before); err != nil {
		return fmt.Errorf("failed to delete aggregated health checks: %w", err)
	}

	log.Printf("Deleted %d original health checks for monitor %s (older than %v)",
		len(checks), monitorID, before)

	return nil
}
