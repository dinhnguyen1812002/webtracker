package resilience

import (
	"context"
	"sync"
	"time"

	"web-tracker/infrastructure/logger"
)

// DegradationMode represents the current system degradation state
type DegradationMode int

const (
	// ModeNormal - all systems operational
	ModeNormal DegradationMode = iota
	// ModeRedisDown - Redis unavailable, fallback to database
	ModeRedisDown
	// ModeDatabaseDown - Database unavailable, use cached data only
	ModeDatabaseDown
	// ModeCritical - Both database and Redis down, minimal functionality
	ModeCritical
)

// DegradationManager manages system degradation state
type DegradationManager struct {
	mu             sync.RWMutex
	mode           DegradationMode
	lastDBCheck    time.Time
	lastRedisCheck time.Time
	logger         *logger.Logger

	// Health check intervals
	dbCheckInterval    time.Duration
	redisCheckInterval time.Duration

	// Callbacks for health checks
	dbHealthCheck    func(ctx context.Context) error
	redisHealthCheck func(ctx context.Context) error
}

// NewDegradationManager creates a new degradation manager
func NewDegradationManager(
	dbHealthCheck func(ctx context.Context) error,
	redisHealthCheck func(ctx context.Context) error,
) *DegradationManager {
	return &DegradationManager{
		mode:               ModeNormal,
		logger:             logger.GetLogger(),
		dbCheckInterval:    30 * time.Second,
		redisCheckInterval: 15 * time.Second,
		dbHealthCheck:      dbHealthCheck,
		redisHealthCheck:   redisHealthCheck,
	}
}

// GetMode returns the current degradation mode
func (dm *DegradationManager) GetMode() DegradationMode {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.mode
}

// IsNormal returns true if system is in normal mode
func (dm *DegradationManager) IsNormal() bool {
	return dm.GetMode() == ModeNormal
}

// IsRedisAvailable returns true if Redis is available
func (dm *DegradationManager) IsRedisAvailable() bool {
	mode := dm.GetMode()
	return mode == ModeNormal
}

// IsDatabaseAvailable returns true if database is available
func (dm *DegradationManager) IsDatabaseAvailable() bool {
	mode := dm.GetMode()
	return mode == ModeNormal || mode == ModeRedisDown
}

// SetDatabaseDown marks database as unavailable
func (dm *DegradationManager) SetDatabaseDown(err error) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	oldMode := dm.mode
	if dm.mode == ModeNormal {
		dm.mode = ModeDatabaseDown
	} else if dm.mode == ModeRedisDown {
		dm.mode = ModeCritical
	}

	if oldMode != dm.mode {
		dm.logger.Error("Database marked as unavailable", logger.Fields{
			"error":    err.Error(),
			"old_mode": dm.modeString(oldMode),
			"new_mode": dm.modeString(dm.mode),
		})
	}
}

// SetRedisDown marks Redis as unavailable
func (dm *DegradationManager) SetRedisDown(err error) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	oldMode := dm.mode
	if dm.mode == ModeNormal {
		dm.mode = ModeRedisDown
	} else if dm.mode == ModeDatabaseDown {
		dm.mode = ModeCritical
	}

	if oldMode != dm.mode {
		dm.logger.Warn("Redis marked as unavailable, falling back to database", logger.Fields{
			"error":    err.Error(),
			"old_mode": dm.modeString(oldMode),
			"new_mode": dm.modeString(dm.mode),
		})
	}
}

// CheckAndRecover performs health checks and attempts recovery
func (dm *DegradationManager) CheckAndRecover(ctx context.Context) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	now := time.Now()
	oldMode := dm.mode

	// Check database health
	if dm.shouldCheckDatabase(now) {
		dm.lastDBCheck = now
		if dm.dbHealthCheck != nil {
			if err := dm.dbHealthCheck(ctx); err != nil {
				dm.markDatabaseDown(err)
			} else {
				dm.markDatabaseUp()
			}
		}
	}

	// Check Redis health
	if dm.shouldCheckRedis(now) {
		dm.lastRedisCheck = now
		if dm.redisHealthCheck != nil {
			if err := dm.redisHealthCheck(ctx); err != nil {
				dm.markRedisDown(err)
			} else {
				dm.markRedisUp()
			}
		}
	}

	// Log mode changes
	if oldMode != dm.mode {
		dm.logger.Info("System degradation mode changed", logger.Fields{
			"old_mode": dm.modeString(oldMode),
			"new_mode": dm.modeString(dm.mode),
		})
	}
}

// Start begins the health check loop
func (dm *DegradationManager) Start(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second) // Check every 10 seconds
	defer ticker.Stop()

	dm.logger.Info("Degradation manager started")

	for {
		select {
		case <-ctx.Done():
			dm.logger.Info("Degradation manager stopped")
			return
		case <-ticker.C:
			dm.CheckAndRecover(ctx)
		}
	}
}

// shouldCheckDatabase returns true if database health should be checked
func (dm *DegradationManager) shouldCheckDatabase(now time.Time) bool {
	return now.Sub(dm.lastDBCheck) >= dm.dbCheckInterval
}

// shouldCheckRedis returns true if Redis health should be checked
func (dm *DegradationManager) shouldCheckRedis(now time.Time) bool {
	return now.Sub(dm.lastRedisCheck) >= dm.redisCheckInterval
}

// markDatabaseDown marks database as down (internal, assumes lock held)
func (dm *DegradationManager) markDatabaseDown(err error) {
	if dm.mode == ModeNormal {
		dm.mode = ModeDatabaseDown
	} else if dm.mode == ModeRedisDown {
		dm.mode = ModeCritical
	}
}

// markDatabaseUp marks database as up (internal, assumes lock held)
func (dm *DegradationManager) markDatabaseUp() {
	if dm.mode == ModeDatabaseDown {
		dm.mode = ModeNormal
	} else if dm.mode == ModeCritical {
		dm.mode = ModeRedisDown
	}
}

// markRedisDown marks Redis as down (internal, assumes lock held)
func (dm *DegradationManager) markRedisDown(err error) {
	if dm.mode == ModeNormal {
		dm.mode = ModeRedisDown
	} else if dm.mode == ModeDatabaseDown {
		dm.mode = ModeCritical
	}
}

// markRedisUp marks Redis as up (internal, assumes lock held)
func (dm *DegradationManager) markRedisUp() {
	if dm.mode == ModeRedisDown {
		dm.mode = ModeNormal
	} else if dm.mode == ModeCritical {
		dm.mode = ModeDatabaseDown
	}
}

// modeString returns string representation of degradation mode
func (dm *DegradationManager) modeString(mode DegradationMode) string {
	switch mode {
	case ModeNormal:
		return "normal"
	case ModeRedisDown:
		return "redis_down"
	case ModeDatabaseDown:
		return "database_down"
	case ModeCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// GetModeString returns string representation of current mode
func (dm *DegradationManager) GetModeString() string {
	return dm.modeString(dm.GetMode())
}
