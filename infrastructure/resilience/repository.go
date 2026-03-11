package resilience

import (
	"context"
	"errors"
	"sync"
	"time"

	"web-tracker/domain"
	"web-tracker/infrastructure/logger"
)

// ResilientMonitorRepository wraps a monitor repository with resilience features
type ResilientMonitorRepository struct {
	primary   domain.MonitorRepository
	cache     map[string]*domain.Monitor
	cacheMu   sync.RWMutex
	cacheTime map[string]time.Time
	cacheTTL  time.Duration
	degrader  *DegradationManager
	logger    *logger.Logger
}

// NewResilientMonitorRepository creates a new resilient monitor repository
func NewResilientMonitorRepository(
	primary domain.MonitorRepository,
	degrader *DegradationManager,
) *ResilientMonitorRepository {
	return &ResilientMonitorRepository{
		primary:   primary,
		cache:     make(map[string]*domain.Monitor),
		cacheTime: make(map[string]time.Time),
		cacheTTL:  5 * time.Minute,
		degrader:  degrader,
		logger:    logger.GetLogger(),
	}
}

// Create creates a new monitor
func (r *ResilientMonitorRepository) Create(ctx context.Context, monitor *domain.Monitor) error {
	if !r.degrader.IsDatabaseAvailable() {
		return errors.New("database unavailable, cannot create monitor")
	}

	err := r.primary.Create(ctx, monitor)
	if err != nil {
		r.degrader.SetDatabaseDown(err)
		return err
	}

	// Cache the created monitor
	r.cacheMonitor(monitor)
	return nil
}

// GetByID retrieves a monitor by ID with fallback to cache
func (r *ResilientMonitorRepository) GetByID(ctx context.Context, id string) (*domain.Monitor, error) {
	// Try primary repository first if database is available
	if r.degrader.IsDatabaseAvailable() {
		monitor, err := r.primary.GetByID(ctx, id)
		if err == nil {
			r.cacheMonitor(monitor)
			return monitor, nil
		}

		// Mark database as down if error is not "not found"
		if !errors.Is(err, domain.ErrMonitorNotFound) {
			r.degrader.SetDatabaseDown(err)
		} else {
			return nil, err // Return not found error immediately
		}
	}

	// Fallback to cache
	r.cacheMu.RLock()
	defer r.cacheMu.RUnlock()

	monitor, exists := r.cache[id]
	if !exists {
		r.logger.Warn("Monitor not found in cache during degraded mode", logger.Fields{
			"monitor_id": id,
			"mode":       r.degrader.GetModeString(),
		})
		return nil, domain.ErrMonitorNotFound
	}

	// Check cache freshness
	if time.Since(r.cacheTime[id]) > r.cacheTTL {
		r.logger.Warn("Serving stale monitor data from cache", logger.Fields{
			"monitor_id": id,
			"age":        time.Since(r.cacheTime[id]).String(),
		})
	}

	return monitor, nil
}

// List retrieves monitors with fallback to cache
func (r *ResilientMonitorRepository) List(ctx context.Context, filters domain.ListFilters) ([]*domain.Monitor, error) {
	// Try primary repository first if database is available
	if r.degrader.IsDatabaseAvailable() {
		monitors, err := r.primary.List(ctx, filters)
		if err == nil {
			// Cache all monitors
			for _, monitor := range monitors {
				r.cacheMonitor(monitor)
			}
			return monitors, nil
		}

		r.degrader.SetDatabaseDown(err)
	}

	// Fallback to cache
	r.cacheMu.RLock()
	defer r.cacheMu.RUnlock()

	var monitors []*domain.Monitor
	for _, monitor := range r.cache {
		// Apply basic filtering (simplified)
		if filters.Enabled != nil && monitor.Enabled != *filters.Enabled {
			continue
		}
		monitors = append(monitors, monitor)
	}

	if len(monitors) > 0 {
		r.logger.Warn("Serving monitor list from cache during degraded mode", logger.Fields{
			"count": len(monitors),
			"mode":  r.degrader.GetModeString(),
		})
	}

	return monitors, nil
}

// Update updates a monitor
func (r *ResilientMonitorRepository) Update(ctx context.Context, monitor *domain.Monitor) error {
	if !r.degrader.IsDatabaseAvailable() {
		return errors.New("database unavailable, cannot update monitor")
	}

	err := r.primary.Update(ctx, monitor)
	if err != nil {
		r.degrader.SetDatabaseDown(err)
		return err
	}

	// Update cache
	r.cacheMonitor(monitor)
	return nil
}

// Delete deletes a monitor
func (r *ResilientMonitorRepository) Delete(ctx context.Context, id string) error {
	if !r.degrader.IsDatabaseAvailable() {
		return errors.New("database unavailable, cannot delete monitor")
	}

	err := r.primary.Delete(ctx, id)
	if err != nil {
		r.degrader.SetDatabaseDown(err)
		return err
	}

	// Remove from cache
	r.cacheMu.Lock()
	defer r.cacheMu.Unlock()
	delete(r.cache, id)
	delete(r.cacheTime, id)

	return nil
}

// cacheMonitor stores a monitor in the cache
func (r *ResilientMonitorRepository) cacheMonitor(monitor *domain.Monitor) {
	r.cacheMu.Lock()
	defer r.cacheMu.Unlock()

	r.cache[monitor.ID] = monitor
	r.cacheTime[monitor.ID] = time.Now()
}

// ResilientHealthCheckRepository wraps a health check repository with resilience features
type ResilientHealthCheckRepository struct {
	primary  domain.HealthCheckRepository
	degrader *DegradationManager
	logger   *logger.Logger
}

// NewResilientHealthCheckRepository creates a new resilient health check repository
func NewResilientHealthCheckRepository(
	primary domain.HealthCheckRepository,
	degrader *DegradationManager,
) *ResilientHealthCheckRepository {
	return &ResilientHealthCheckRepository{
		primary:  primary,
		degrader: degrader,
		logger:   logger.GetLogger(),
	}
}

// Create creates a new health check
func (r *ResilientHealthCheckRepository) Create(ctx context.Context, check *domain.HealthCheck) error {
	if !r.degrader.IsDatabaseAvailable() {
		r.logger.Warn("Dropping health check due to database unavailability", logger.Fields{
			"monitor_id": check.MonitorID,
			"status":     string(check.Status),
		})
		return nil // Don't return error to avoid breaking the health check flow
	}

	err := r.primary.Create(ctx, check)
	if err != nil {
		r.degrader.SetDatabaseDown(err)
		r.logger.Warn("Failed to store health check, database may be down", logger.Fields{
			"monitor_id": check.MonitorID,
			"error":      err.Error(),
		})
		return nil // Don't return error to avoid breaking the health check flow
	}

	return nil
}

// GetByMonitorID retrieves health checks by monitor ID
func (r *ResilientHealthCheckRepository) GetByMonitorID(ctx context.Context, monitorID string, limit int) ([]*domain.HealthCheck, error) {
	if !r.degrader.IsDatabaseAvailable() {
		r.logger.Warn("Cannot retrieve health check history during database degradation", logger.Fields{
			"monitor_id": monitorID,
		})
		return []*domain.HealthCheck{}, nil // Return empty slice instead of error
	}

	checks, err := r.primary.GetByMonitorID(ctx, monitorID, limit)
	if err != nil {
		r.degrader.SetDatabaseDown(err)
		return []*domain.HealthCheck{}, nil // Return empty slice instead of error
	}

	return checks, nil
}

// GetByDateRange retrieves health checks by date range
func (r *ResilientHealthCheckRepository) GetByDateRange(ctx context.Context, monitorID string, start, end time.Time) ([]*domain.HealthCheck, error) {
	if !r.degrader.IsDatabaseAvailable() {
		r.logger.Warn("Cannot retrieve health check history during database degradation", logger.Fields{
			"monitor_id": monitorID,
		})
		return []*domain.HealthCheck{}, nil // Return empty slice instead of error
	}

	checks, err := r.primary.GetByDateRange(ctx, monitorID, start, end)
	if err != nil {
		r.degrader.SetDatabaseDown(err)
		return []*domain.HealthCheck{}, nil // Return empty slice instead of error
	}

	return checks, nil
}

// DeleteOlderThan deletes health checks older than the specified time
func (r *ResilientHealthCheckRepository) DeleteOlderThan(ctx context.Context, before time.Time) error {
	if !r.degrader.IsDatabaseAvailable() {
		r.logger.Warn("Cannot perform cleanup during database degradation")
		return nil // Don't return error, cleanup can be deferred
	}

	err := r.primary.DeleteOlderThan(ctx, before)
	if err != nil {
		r.degrader.SetDatabaseDown(err)
		return nil // Don't return error, cleanup can be deferred
	}

	return nil
}
