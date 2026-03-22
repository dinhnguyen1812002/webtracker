package tests

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"web-tracker/infrastructure/resilience"
)

func TestNewCircuitBreaker(t *testing.T) {
	cb := resilience.NewCircuitBreaker("test", 5, 60*time.Second, 3)

	assert.NotNil(t, cb)
	assert.Equal(t, resilience.StateClosed, cb.GetState())
}

func TestCircuitBreakerClosed(t *testing.T) {
	cb := resilience.NewCircuitBreaker("test", 3, 60*time.Second, 2)
	ctx := context.Background()

	// Should allow execution when closed
	err := cb.Execute(ctx, func(ctx context.Context) error {
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, resilience.StateClosed, cb.GetState())

	// Should record failures but stay closed until threshold
	for i := 0; i < 2; i++ {
		err = cb.Execute(ctx, func(ctx context.Context) error {
			return errors.New("test error")
		})
		assert.Error(t, err)
		assert.Equal(t, resilience.StateClosed, cb.GetState())
	}

	// Should open after reaching failure threshold
	err = cb.Execute(ctx, func(ctx context.Context) error {
		return errors.New("test error")
	})
	assert.Error(t, err)
	assert.Equal(t, resilience.StateOpen, cb.GetState())
}

func TestCircuitBreakerOpen(t *testing.T) {
	cb := resilience.NewCircuitBreaker("test", 2, 100*time.Millisecond, 2)
	ctx := context.Background()

	// Force circuit to open
	for i := 0; i < 2; i++ {
		cb.Execute(ctx, func(ctx context.Context) error {
			return errors.New("test error")
		})
	}
	assert.Equal(t, resilience.StateOpen, cb.GetState())

	// Should reject requests when open
	err := cb.Execute(ctx, func(ctx context.Context) error {
		return nil
	})
	assert.Equal(t, resilience.ErrCircuitOpen, err)

	// Should transition to half-open after timeout
	time.Sleep(150 * time.Millisecond)

	err = cb.Execute(ctx, func(ctx context.Context) error {
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, resilience.StateHalfOpen, cb.GetState())
}

func TestCircuitBreakerStats(t *testing.T) {
	cb := resilience.NewCircuitBreaker("test", 3, 60*time.Second, 2)
	ctx := context.Background()

	// Execute some operations
	cb.Execute(ctx, func(ctx context.Context) error {
		return nil
	})
	cb.Execute(ctx, func(ctx context.Context) error {
		return errors.New("test error")
	})

	stats := cb.GetStats()
	assert.Equal(t, "test", stats.Name)
	assert.Equal(t, "closed", stats.State)
	assert.Equal(t, 1, stats.FailureCount)
	assert.False(t, stats.LastSuccessTime.IsZero())
	assert.False(t, stats.LastFailureTime.IsZero())
}

func TestCircuitBreakerManager(t *testing.T) {
	manager := resilience.NewCircuitBreakerManager()

	// Should create new circuit breaker
	cb1 := manager.GetOrCreate("test1", 5, 60*time.Second, 3)
	assert.NotNil(t, cb1)
	assert.Equal(t, "test1", cb1.GetStats().Name)

	// Should return existing circuit breaker
	cb2 := manager.GetOrCreate("test1", 10, 120*time.Second, 5)
	assert.Equal(t, cb1, cb2)                     // Should be the same instance
	assert.Equal(t, "test1", cb2.GetStats().Name) // Should keep original instance

	// Should create different circuit breaker for different name
	cb3 := manager.GetOrCreate("test2", 3, 30*time.Second, 2)
	assert.NotEqual(t, cb1, cb3)
	assert.Equal(t, "test2", cb3.GetStats().Name)
}
