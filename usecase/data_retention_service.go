package usecase

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"web-tracker/domain"
)

// dataRetentionService implements DataRetentionService
type dataRetentionService struct {
	healthCheckRepo           domain.HealthCheckRepository
	aggregatedHealthCheckRepo domain.AggregatedHealthCheckRepository
	alertRepo                 domain.AlertRepository
	retentionPeriod           time.Duration
	cleanupInterval           time.Duration
	ticker                    *time.Ticker
	stopCh                    chan struct{}
	running                   bool
	mu                        sync.RWMutex
}

// NewDataRetentionService creates a new data retention service
func NewDataRetentionService(
	healthCheckRepo domain.HealthCheckRepository,
	alertRepo domain.AlertRepository,
) DataRetentionService {
	return &dataRetentionService{
		healthCheckRepo: healthCheckRepo,
		alertRepo:       alertRepo,
		retentionPeriod: 90 * 24 * time.Hour, // 90 days
		cleanupInterval: 24 * time.Hour,      // Daily cleanup
		stopCh:          make(chan struct{}),
	}
}

// NewDataRetentionServiceWithAggregation creates a new data retention service with aggregated health check support
func NewDataRetentionServiceWithAggregation(
	healthCheckRepo domain.HealthCheckRepository,
	aggregatedHealthCheckRepo domain.AggregatedHealthCheckRepository,
	alertRepo domain.AlertRepository,
) DataRetentionService {
	return &dataRetentionService{
		healthCheckRepo:           healthCheckRepo,
		aggregatedHealthCheckRepo: aggregatedHealthCheckRepo,
		alertRepo:                 alertRepo,
		retentionPeriod:           90 * 24 * time.Hour, // 90 days
		cleanupInterval:           24 * time.Hour,      // Daily cleanup
		stopCh:                    make(chan struct{}),
	}
}

// StartCleanupScheduler starts the scheduled cleanup process
func (s *dataRetentionService) StartCleanupScheduler(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("cleanup scheduler is already running")
	}

	s.ticker = time.NewTicker(s.cleanupInterval)
	s.running = true

	go s.cleanupLoop(ctx)

	log.Printf("Data retention cleanup scheduler started (interval: %v, retention: %v)",
		s.cleanupInterval, s.retentionPeriod)

	return nil
}

// StopCleanupScheduler stops the scheduled cleanup process
func (s *dataRetentionService) StopCleanupScheduler() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return fmt.Errorf("cleanup scheduler is not running")
	}

	close(s.stopCh)
	s.ticker.Stop()
	s.running = false

	log.Printf("Data retention cleanup scheduler stopped")

	return nil
}

// RunCleanup performs a single cleanup operation
func (s *dataRetentionService) RunCleanup(ctx context.Context) error {
	cutoffTime := time.Now().Add(-s.retentionPeriod)

	log.Printf("Starting data retention cleanup for data older than %v", cutoffTime)

	// Clean up health checks
	if err := s.healthCheckRepo.DeleteOlderThan(ctx, cutoffTime); err != nil {
		return fmt.Errorf("failed to delete old health checks: %w", err)
	}

	// Clean up aggregated health checks if repository is available
	if s.aggregatedHealthCheckRepo != nil {
		if err := s.aggregatedHealthCheckRepo.DeleteOlderThan(ctx, cutoffTime); err != nil {
			return fmt.Errorf("failed to delete old aggregated health checks: %w", err)
		}
	}

	// Clean up alerts
	if err := s.alertRepo.DeleteOlderThan(ctx, cutoffTime); err != nil {
		return fmt.Errorf("failed to delete old alerts: %w", err)
	}

	log.Printf("Data retention cleanup completed successfully")

	return nil
}

// IsRunning returns whether the cleanup scheduler is currently running
func (s *dataRetentionService) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// cleanupLoop runs the cleanup process on a schedule
func (s *dataRetentionService) cleanupLoop(ctx context.Context) {
	// Run initial cleanup
	if err := s.RunCleanup(ctx); err != nil {
		log.Printf("Initial cleanup failed: %v", err)
	}

	for {
		select {
		case <-s.ticker.C:
			if err := s.RunCleanup(ctx); err != nil {
				log.Printf("Scheduled cleanup failed: %v", err)
			}
		case <-s.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}
