package usecase

import (
	"context"
	"fmt"
	"time"
)

// RedisCache defines the interface for Redis cache operations needed by the rate limiter
type RedisCache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

// RedisRateLimiter implements rate limiting for alerts using Redis
type RedisRateLimiter struct {
	redisClient RedisCache
}

// NewRedisRateLimiter creates a new Redis-based rate limiter
func NewRedisRateLimiter(redisClient RedisCache) *RedisRateLimiter {
	return &RedisRateLimiter{
		redisClient: redisClient,
	}
}

// CheckAndSetDowntimeAlert checks if a downtime alert should be sent
// Requirement 5.2: 15-minute suppression for duplicate downtime alerts
// Requirement 5.4: 1-hour reminder for prolonged failures
func (r *RedisRateLimiter) CheckAndSetDowntimeAlert(ctx context.Context, monitorID string) (bool, error) {
	key := fmt.Sprintf("ratelimit:alert:%s:downtime", monitorID)

	// Get the last alert timestamp
	data, err := r.redisClient.Get(ctx, key)
	if err != nil {
		return false, fmt.Errorf("failed to check rate limit: %w", err)
	}

	now := time.Now()

	// If no previous alert, allow this one
	if data == nil {
		// Set the rate limit with 15-minute TTL
		if err := r.redisClient.Set(ctx, key, []byte(now.Format(time.RFC3339)), 15*time.Minute); err != nil {
			return false, fmt.Errorf("failed to set rate limit: %w", err)
		}
		return true, nil
	}

	// Parse the last alert time
	lastAlert, err := time.Parse(time.RFC3339, string(data))
	if err != nil {
		return false, fmt.Errorf("failed to parse last alert time: %w", err)
	}

	timeSinceLastAlert := now.Sub(lastAlert)

	// Requirement 5.4: Send reminder after 1 hour of continuous failure
	if timeSinceLastAlert >= 1*time.Hour {
		// Update the rate limit timestamp
		if err := r.redisClient.Set(ctx, key, []byte(now.Format(time.RFC3339)), 15*time.Minute); err != nil {
			return false, fmt.Errorf("failed to update rate limit: %w", err)
		}
		return true, nil
	}

	// Requirement 5.2: Suppress within 15 minutes
	if timeSinceLastAlert < 15*time.Minute {
		return false, nil
	}

	// More than 15 minutes but less than 1 hour - allow the alert
	if err := r.redisClient.Set(ctx, key, []byte(now.Format(time.RFC3339)), 15*time.Minute); err != nil {
		return false, fmt.Errorf("failed to update rate limit: %w", err)
	}
	return true, nil
}

// CheckAndSetSSLAlert checks if an SSL alert should be sent
// Requirement 5.5: Daily limit for SSL expiration alerts
func (r *RedisRateLimiter) CheckAndSetSSLAlert(ctx context.Context, monitorID string, daysUntil int) (bool, error) {
	// Create a unique key for each SSL warning level
	key := fmt.Sprintf("ratelimit:alert:%s:ssl:%d", monitorID, daysUntil)

	// Check if we've already sent an alert for this warning level today
	data, err := r.redisClient.Get(ctx, key)
	if err != nil {
		return false, fmt.Errorf("failed to check SSL rate limit: %w", err)
	}

	// If alert was already sent today, suppress it
	if data != nil {
		return false, nil
	}

	// Set the rate limit with 24-hour TTL
	now := time.Now()
	if err := r.redisClient.Set(ctx, key, []byte(now.Format(time.RFC3339)), 24*time.Hour); err != nil {
		return false, fmt.Errorf("failed to set SSL rate limit: %w", err)
	}

	return true, nil
}

// GetLastDowntimeAlert returns the timestamp of the last downtime alert
func (r *RedisRateLimiter) GetLastDowntimeAlert(ctx context.Context, monitorID string) (*time.Time, error) {
	key := fmt.Sprintf("ratelimit:alert:%s:downtime", monitorID)

	data, err := r.redisClient.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get last downtime alert: %w", err)
	}

	if data == nil {
		return nil, nil
	}

	lastAlert, err := time.Parse(time.RFC3339, string(data))
	if err != nil {
		return nil, fmt.Errorf("failed to parse last alert time: %w", err)
	}

	return &lastAlert, nil
}

// ClearDowntimeAlert clears the downtime alert rate limit
// Requirement 5.3: Recovery alerts bypass rate limiting
func (r *RedisRateLimiter) ClearDowntimeAlert(ctx context.Context, monitorID string) error {
	key := fmt.Sprintf("ratelimit:alert:%s:downtime", monitorID)

	if err := r.redisClient.Delete(ctx, key); err != nil {
		return fmt.Errorf("failed to clear downtime rate limit: %w", err)
	}

	return nil
}
