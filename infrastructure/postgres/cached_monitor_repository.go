package postgres

import (
	"context"
	"fmt"
	"time"

	"web-tracker/domain"
	"web-tracker/infrastructure/redis"
)

const (
	// MonitorCacheTTL is the time-to-live for cached monitor configurations (5 minutes)
	MonitorCacheTTL = 5 * time.Minute

	// MonitorCacheKeyPrefix is the prefix for monitor cache keys
	MonitorCacheKeyPrefix = "cache:monitor:"
)

// CachedMonitorRepository wraps MonitorRepository with Redis caching
type CachedMonitorRepository struct {
	repo  *MonitorRepository
	cache *redis.Client
}

// NewCachedMonitorRepository creates a new cached monitor repository
func NewCachedMonitorRepository(repo *MonitorRepository, cache *redis.Client) *CachedMonitorRepository {
	return &CachedMonitorRepository{
		repo:  repo,
		cache: cache,
	}
}

// monitorCacheKey generates the cache key for a monitor ID
func monitorCacheKey(id string) string {
	return MonitorCacheKeyPrefix + id
}

// Create inserts a new monitor and invalidates any existing cache
func (r *CachedMonitorRepository) Create(ctx context.Context, monitor *domain.Monitor) error {
	if err := r.repo.Create(ctx, monitor); err != nil {
		return err
	}

	// Cache the newly created monitor
	if err := r.cacheMonitor(ctx, monitor); err != nil {
		// Log error but don't fail the operation
		// The cache will be populated on next read
		_ = err
	}

	return nil
}

// GetByID retrieves a monitor by ID, using cache when available
func (r *CachedMonitorRepository) GetByID(ctx context.Context, id string) (*domain.Monitor, error) {
	// Try to get from cache first
	cacheKey := monitorCacheKey(id)
	var monitor domain.Monitor
	found, err := r.cache.GetJSON(ctx, cacheKey, &monitor)
	if err != nil {
		// Log cache error but continue to database
		_ = err
	} else if found {
		// Cache hit - return cached monitor
		return &monitor, nil
	}

	// Cache miss - get from database
	dbMonitor, err := r.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Cache the monitor for future requests
	if err := r.cacheMonitor(ctx, dbMonitor); err != nil {
		// Log error but don't fail the operation
		_ = err
	}

	return dbMonitor, nil
}

// List retrieves monitors with filters (not cached due to variable filters)
func (r *CachedMonitorRepository) List(ctx context.Context, filters domain.ListFilters) ([]*domain.Monitor, error) {
	// List operations are not cached due to the complexity of filter combinations
	// Individual monitors will be cached when accessed via GetByID
	return r.repo.List(ctx, filters)
}

// Update updates a monitor and invalidates its cache
func (r *CachedMonitorRepository) Update(ctx context.Context, monitor *domain.Monitor) error {
	if err := r.repo.Update(ctx, monitor); err != nil {
		return err
	}

	// Invalidate the cache for this monitor
	if err := r.invalidateCache(ctx, monitor.ID); err != nil {
		// Log error but don't fail the operation
		_ = err
	}

	return nil
}

// Delete removes a monitor and invalidates its cache
func (r *CachedMonitorRepository) Delete(ctx context.Context, id string) error {
	if err := r.repo.Delete(ctx, id); err != nil {
		return err
	}

	// Invalidate the cache for this monitor
	if err := r.invalidateCache(ctx, id); err != nil {
		// Log error but don't fail the operation
		_ = err
	}

	return nil
}

// cacheMonitor stores a monitor in the cache with TTL
func (r *CachedMonitorRepository) cacheMonitor(ctx context.Context, monitor *domain.Monitor) error {
	cacheKey := monitorCacheKey(monitor.ID)
	if err := r.cache.SetJSON(ctx, cacheKey, monitor, MonitorCacheTTL); err != nil {
		return fmt.Errorf("failed to cache monitor: %w", err)
	}
	return nil
}

// invalidateCache removes a monitor from the cache
func (r *CachedMonitorRepository) invalidateCache(ctx context.Context, id string) error {
	cacheKey := monitorCacheKey(id)
	if err := r.cache.Delete(ctx, cacheKey); err != nil {
		return fmt.Errorf("failed to invalidate cache: %w", err)
	}
	return nil
}
