package resilience

import (
	"context"
	"errors"
	"sync"
	"time"

	"web-tracker/infrastructure/logger"
)

// CircuitState represents the state of a circuit breaker
type CircuitState int

const (
	// StateClosed - circuit is closed, requests pass through
	StateClosed CircuitState = iota
	// StateOpen - circuit is open, requests are rejected
	StateOpen
	// StateHalfOpen - circuit is half-open, testing if service recovered
	StateHalfOpen
)

var (
	ErrCircuitOpen = errors.New("circuit breaker is open")
)

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	mu              sync.RWMutex
	state           CircuitState
	failureCount    int
	successCount    int
	lastFailureTime time.Time
	lastSuccessTime time.Time

	// Configuration
	maxFailures      int           // Number of failures before opening
	timeout          time.Duration // Time to wait before half-open
	successThreshold int           // Number of successes to close from half-open

	name   string
	logger *logger.Logger
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(name string, maxFailures int, timeout time.Duration, successThreshold int) *CircuitBreaker {
	return &CircuitBreaker{
		state:            StateClosed,
		maxFailures:      maxFailures,
		timeout:          timeout,
		successThreshold: successThreshold,
		name:             name,
		logger:           logger.GetLogger(),
	}
}

// Execute executes a function with circuit breaker protection
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func(ctx context.Context) error) error {
	// Check if we can execute
	if !cb.canExecute() {
		cb.logger.Debug("Circuit breaker rejected request", logger.Fields{
			"circuit": cb.name,
			"state":   cb.getStateString(),
		})
		return ErrCircuitOpen
	}

	// Execute the function
	err := fn(ctx)

	// Record the result
	if err != nil {
		cb.recordFailure()
		return err
	}

	cb.recordSuccess()
	return nil
}

// canExecute determines if a request can be executed
func (cb *CircuitBreaker) canExecute() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		// Check if timeout has passed to transition to half-open
		if time.Since(cb.lastFailureTime) >= cb.timeout {
			cb.mu.RUnlock()
			cb.mu.Lock()
			// Double-check after acquiring write lock
			if cb.state == StateOpen && time.Since(cb.lastFailureTime) >= cb.timeout {
				cb.state = StateHalfOpen
				cb.successCount = 0
				cb.logger.Info("Circuit breaker transitioning to half-open", logger.Fields{
					"circuit": cb.name,
				})
			}
			cb.mu.Unlock()
			cb.mu.RLock()
			return cb.state == StateHalfOpen
		}
		return false
	case StateHalfOpen:
		return true
	default:
		return false
	}
}

// recordSuccess records a successful execution
func (cb *CircuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.lastSuccessTime = time.Now()

	switch cb.state {
	case StateClosed:
		// Reset failure count on success
		cb.failureCount = 0
	case StateHalfOpen:
		cb.successCount++
		if cb.successCount >= cb.successThreshold {
			cb.state = StateClosed
			cb.failureCount = 0
			cb.successCount = 0
			cb.logger.Info("Circuit breaker closed after successful recovery", logger.Fields{
				"circuit": cb.name,
			})
		}
	}
}

// recordFailure records a failed execution
func (cb *CircuitBreaker) recordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.lastFailureTime = time.Now()
	cb.failureCount++

	switch cb.state {
	case StateClosed:
		if cb.failureCount >= cb.maxFailures {
			cb.state = StateOpen
			cb.logger.Warn("Circuit breaker opened due to failures", logger.Fields{
				"circuit":       cb.name,
				"failure_count": cb.failureCount,
				"max_failures":  cb.maxFailures,
			})
		}
	case StateHalfOpen:
		// Any failure in half-open state opens the circuit
		cb.state = StateOpen
		cb.successCount = 0
		cb.logger.Warn("Circuit breaker opened from half-open state", logger.Fields{
			"circuit": cb.name,
		})
	}
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetStats returns statistics about the circuit breaker
func (cb *CircuitBreaker) GetStats() CircuitBreakerStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return CircuitBreakerStats{
		Name:            cb.name,
		State:           cb.getStateString(),
		FailureCount:    cb.failureCount,
		SuccessCount:    cb.successCount,
		LastFailureTime: cb.lastFailureTime,
		LastSuccessTime: cb.lastSuccessTime,
	}
}

// getStateString returns string representation of state (assumes lock held)
func (cb *CircuitBreaker) getStateString() string {
	switch cb.state {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreakerStats holds statistics about a circuit breaker
type CircuitBreakerStats struct {
	Name            string
	State           string
	FailureCount    int
	SuccessCount    int
	LastFailureTime time.Time
	LastSuccessTime time.Time
}

// CircuitBreakerManager manages multiple circuit breakers
type CircuitBreakerManager struct {
	breakers map[string]*CircuitBreaker
	mu       sync.RWMutex
	logger   *logger.Logger
}

// NewCircuitBreakerManager creates a new circuit breaker manager
func NewCircuitBreakerManager() *CircuitBreakerManager {
	return &CircuitBreakerManager{
		breakers: make(map[string]*CircuitBreaker),
		logger:   logger.GetLogger(),
	}
}

// GetOrCreate gets an existing circuit breaker or creates a new one
func (cbm *CircuitBreakerManager) GetOrCreate(name string, maxFailures int, timeout time.Duration, successThreshold int) *CircuitBreaker {
	cbm.mu.RLock()
	if cb, exists := cbm.breakers[name]; exists {
		cbm.mu.RUnlock()
		return cb
	}
	cbm.mu.RUnlock()

	cbm.mu.Lock()
	defer cbm.mu.Unlock()

	// Double-check after acquiring write lock
	if cb, exists := cbm.breakers[name]; exists {
		return cb
	}

	cb := NewCircuitBreaker(name, maxFailures, timeout, successThreshold)
	cbm.breakers[name] = cb

	cbm.logger.Info("Created new circuit breaker", logger.Fields{
		"name":              name,
		"max_failures":      maxFailures,
		"timeout":           timeout.String(),
		"success_threshold": successThreshold,
	})

	return cb
}

// GetStats returns statistics for all circuit breakers
func (cbm *CircuitBreakerManager) GetStats() []CircuitBreakerStats {
	cbm.mu.RLock()
	defer cbm.mu.RUnlock()

	stats := make([]CircuitBreakerStats, 0, len(cbm.breakers))
	for _, cb := range cbm.breakers {
		stats = append(stats, cb.GetStats())
	}

	return stats
}

// Reset resets all circuit breakers to closed state
func (cbm *CircuitBreakerManager) Reset() {
	cbm.mu.RLock()
	defer cbm.mu.RUnlock()

	for name, cb := range cbm.breakers {
		cb.mu.Lock()
		cb.state = StateClosed
		cb.failureCount = 0
		cb.successCount = 0
		cb.mu.Unlock()

		cbm.logger.Info("Reset circuit breaker", logger.Fields{
			"name": name,
		})
	}
}
