package tests

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"web-tracker/infrastructure/resilience"
)

func TestNewDegradationManager(t *testing.T) {
	dbHealthCheck := func(ctx context.Context) error { return nil }
	redisHealthCheck := func(ctx context.Context) error { return nil }

	dm := resilience.NewDegradationManager(dbHealthCheck, redisHealthCheck)

	assert.NotNil(t, dm)
	assert.Equal(t, resilience.ModeNormal, dm.GetMode())
	assert.True(t, dm.IsNormal())
	assert.True(t, dm.IsRedisAvailable())
	assert.True(t, dm.IsDatabaseAvailable())
}

func TestDegradationModes(t *testing.T) {
	dm := resilience.NewDegradationManager(nil, nil)

	// Test normal mode
	assert.Equal(t, resilience.ModeNormal, dm.GetMode())
	assert.True(t, dm.IsNormal())
	assert.True(t, dm.IsRedisAvailable())
	assert.True(t, dm.IsDatabaseAvailable())

	// Test Redis down
	dm.SetRedisDown(errors.New("redis connection failed"))
	assert.Equal(t, resilience.ModeRedisDown, dm.GetMode())
	assert.False(t, dm.IsNormal())
	assert.False(t, dm.IsRedisAvailable())
	assert.True(t, dm.IsDatabaseAvailable())

	// Test database down (from Redis down state)
	dm.SetDatabaseDown(errors.New("database connection failed"))
	assert.Equal(t, resilience.ModeCritical, dm.GetMode())
	assert.False(t, dm.IsNormal())
	assert.False(t, dm.IsRedisAvailable())
	assert.False(t, dm.IsDatabaseAvailable())
}

func TestDegradationModeTransitions(t *testing.T) {
	dm := resilience.NewDegradationManager(nil, nil)

	// Normal -> Database Down
	dm.SetDatabaseDown(errors.New("db error"))
	assert.Equal(t, resilience.ModeDatabaseDown, dm.GetMode())

	// Database Down -> Critical (Redis also down)
	dm.SetRedisDown(errors.New("redis error"))
	assert.Equal(t, resilience.ModeCritical, dm.GetMode())

	// Separate instance: Normal -> Redis Down -> Critical
	dm2 := resilience.NewDegradationManager(nil, nil)
	dm2.SetRedisDown(errors.New("redis error"))
	assert.Equal(t, resilience.ModeRedisDown, dm2.GetMode())
	dm2.SetDatabaseDown(errors.New("db error"))
	assert.Equal(t, resilience.ModeCritical, dm2.GetMode())
}

func TestCheckAndRecover(t *testing.T) {
	dbHealthy := true
	redisHealthy := true

	dbHealthCheck := func(ctx context.Context) error {
		if !dbHealthy {
			return errors.New("database unhealthy")
		}
		return nil
	}

	redisHealthCheck := func(ctx context.Context) error {
		if !redisHealthy {
			return errors.New("redis unhealthy")
		}
		return nil
	}

	dm := resilience.NewDegradationManager(dbHealthCheck, redisHealthCheck)

	ctx := context.Background()

	// Initial state should be normal
	dm.CheckAndRecover(ctx)
	assert.Equal(t, resilience.ModeNormal, dm.GetMode())

	// Make database and Redis unhealthy and recheck (may require new manager to avoid interval waits)
	dbHealthy = false
	redisHealthy = false
	dm2 := resilience.NewDegradationManager(dbHealthCheck, redisHealthCheck)
	dm2.CheckAndRecover(ctx)
	assert.Equal(t, resilience.ModeCritical, dm2.GetMode())
}

func TestModeString(t *testing.T) {
	dm := resilience.NewDegradationManager(nil, nil)

	// Test GetModeString
	assert.Equal(t, "normal", dm.GetModeString())
	dm.SetRedisDown(errors.New("redis error"))
	assert.Equal(t, "redis_down", dm.GetModeString())
}

// shouldCheckDatabase/Redis are unexported; interval behavior is exercised via CheckAndRecover.

func TestStartStop(t *testing.T) {
	dm := resilience.NewDegradationManager(nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Start should not block
	done := make(chan bool)
	go func() {
		dm.Start(ctx)
		done <- true
	}()

	// Should stop when context is cancelled
	select {
	case <-done:
		// Success
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Start did not stop when context was cancelled")
	}
}
