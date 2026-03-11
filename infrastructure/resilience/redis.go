package resilience

import (
	"context"
	"errors"
	"time"

	"web-tracker/infrastructure/logger"
	"web-tracker/infrastructure/redis"
)

var (
	ErrRedisUnavailable = errors.New("redis unavailable")
	ErrKeyNotFound      = errors.New("key not found")
)

// ResilientRedisClient wraps a Redis client with resilience features
type ResilientRedisClient struct {
	primary  *redis.Client
	degrader *DegradationManager
	logger   *logger.Logger
}

// NewResilientRedisClient creates a new resilient Redis client
func NewResilientRedisClient(
	primary *redis.Client,
	degrader *DegradationManager,
) *ResilientRedisClient {
	return &ResilientRedisClient{
		primary:  primary,
		degrader: degrader,
		logger:   logger.GetLogger(),
	}
}

// Get retrieves a value from Redis with fallback behavior
func (r *ResilientRedisClient) Get(ctx context.Context, key string) ([]byte, error) {
	if !r.degrader.IsRedisAvailable() || r.primary == nil {
		r.logger.Debug("Redis unavailable, skipping cache get", logger.Fields{
			"key": key,
		})
		return nil, ErrKeyNotFound
	}

	value, err := r.primary.Get(ctx, key)
	if err != nil {
		r.degrader.SetRedisDown(err)
		return nil, err
	}

	return value, nil
}

// Set stores a value in Redis with fallback behavior
func (r *ResilientRedisClient) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if !r.degrader.IsRedisAvailable() || r.primary == nil {
		r.logger.Debug("Redis unavailable, skipping cache set", logger.Fields{
			"key": key,
		})
		return nil // Don't return error, just skip caching
	}

	err := r.primary.Set(ctx, key, value, ttl)
	if err != nil {
		r.degrader.SetRedisDown(err)
		r.logger.Warn("Failed to set cache value, Redis may be down", logger.Fields{
			"key":   key,
			"error": err.Error(),
		})
		return nil // Don't return error, just skip caching
	}

	return nil
}

// SetJSON stores a JSON value in Redis with fallback behavior
func (r *ResilientRedisClient) SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if !r.degrader.IsRedisAvailable() || r.primary == nil {
		r.logger.Debug("Redis unavailable, skipping cache set", logger.Fields{
			"key": key,
		})
		return nil // Don't return error, just skip caching
	}

	err := r.primary.SetJSON(ctx, key, value, ttl)
	if err != nil {
		r.degrader.SetRedisDown(err)
		r.logger.Warn("Failed to set JSON cache value, Redis may be down", logger.Fields{
			"key":   key,
			"error": err.Error(),
		})
		return nil // Don't return error, just skip caching
	}

	return nil
}

// GetJSON retrieves a JSON value from Redis with fallback behavior
func (r *ResilientRedisClient) GetJSON(ctx context.Context, key string, dest interface{}) (bool, error) {
	if !r.degrader.IsRedisAvailable() || r.primary == nil {
		r.logger.Debug("Redis unavailable, skipping cache get", logger.Fields{
			"key": key,
		})
		return false, nil
	}

	found, err := r.primary.GetJSON(ctx, key, dest)
	if err != nil {
		r.degrader.SetRedisDown(err)
		return false, nil // Return false instead of error
	}

	return found, nil
}

// Delete removes a value from Redis with fallback behavior
func (r *ResilientRedisClient) Delete(ctx context.Context, key string) error {
	if !r.degrader.IsRedisAvailable() || r.primary == nil {
		r.logger.Debug("Redis unavailable, skipping cache delete", logger.Fields{
			"key": key,
		})
		return nil // Don't return error, just skip cache invalidation
	}

	err := r.primary.Delete(ctx, key)
	if err != nil {
		r.degrader.SetRedisDown(err)
		r.logger.Warn("Failed to delete cache value, Redis may be down", logger.Fields{
			"key":   key,
			"error": err.Error(),
		})
		return nil // Don't return error, just skip cache invalidation
	}

	return nil
}

// Enqueue adds a job to the Redis queue with fallback behavior
func (r *ResilientRedisClient) Enqueue(ctx context.Context, queueName string, job *redis.Job) error {
	if !r.degrader.IsRedisAvailable() || r.primary == nil {
		r.logger.Warn("Redis unavailable, cannot enqueue job", logger.Fields{
			"queue": queueName,
		})
		return ErrRedisUnavailable
	}

	err := r.primary.Enqueue(ctx, queueName, job)
	if err != nil {
		r.degrader.SetRedisDown(err)
		return err
	}

	return nil
}

// Dequeue removes a job from the Redis queue with fallback behavior
func (r *ResilientRedisClient) Dequeue(ctx context.Context, queueName string, timeout time.Duration) (*redis.Job, error) {
	if !r.degrader.IsRedisAvailable() || r.primary == nil {
		r.logger.Debug("Redis unavailable, cannot dequeue job", logger.Fields{
			"queue": queueName,
		})
		return nil, ErrRedisUnavailable
	}

	job, err := r.primary.Dequeue(ctx, queueName, timeout)
	if err != nil {
		r.degrader.SetRedisDown(err)
		return nil, err
	}

	return job, nil
}

// Schedule schedules a job for future execution with fallback behavior
func (r *ResilientRedisClient) Schedule(ctx context.Context, scheduleName string, job *redis.Job, executeAt time.Time) error {
	if !r.degrader.IsRedisAvailable() || r.primary == nil {
		r.logger.Warn("Redis unavailable, cannot schedule job", logger.Fields{
			"schedule": scheduleName,
		})
		return ErrRedisUnavailable
	}

	err := r.primary.Schedule(ctx, scheduleName, job, executeAt)
	if err != nil {
		r.degrader.SetRedisDown(err)
		return err
	}

	return nil
}

// GetDueJobs retrieves scheduled jobs with fallback behavior
func (r *ResilientRedisClient) GetDueJobs(ctx context.Context, scheduleName string, now time.Time) ([]*redis.Job, error) {
	if !r.degrader.IsRedisAvailable() || r.primary == nil {
		r.logger.Debug("Redis unavailable, cannot get scheduled jobs", logger.Fields{
			"schedule": scheduleName,
		})
		return []*redis.Job{}, nil // Return empty slice instead of error
	}

	jobs, err := r.primary.GetDueJobs(ctx, scheduleName, now)
	if err != nil {
		r.degrader.SetRedisDown(err)
		return []*redis.Job{}, nil // Return empty slice instead of error
	}

	return jobs, nil
}

// RemoveScheduledJob removes a scheduled job with fallback behavior
func (r *ResilientRedisClient) RemoveScheduledJob(ctx context.Context, scheduleName string, job *redis.Job) error {
	if !r.degrader.IsRedisAvailable() || r.primary == nil {
		r.logger.Debug("Redis unavailable, cannot remove scheduled job", logger.Fields{
			"schedule": scheduleName,
		})
		return nil // Don't return error, just skip removal
	}

	err := r.primary.RemoveScheduledJob(ctx, scheduleName, job)
	if err != nil {
		r.degrader.SetRedisDown(err)
		r.logger.Warn("Failed to remove scheduled job, Redis may be down", logger.Fields{
			"schedule": scheduleName,
			"error":    err.Error(),
		})
		return nil // Don't return error, just skip removal
	}

	return nil
}

// GetQueueLength returns the queue length with fallback behavior
func (r *ResilientRedisClient) GetQueueLength(ctx context.Context, queueName string) (int64, error) {
	if !r.degrader.IsRedisAvailable() || r.primary == nil {
		return 0, nil // Return 0 instead of error
	}

	length, err := r.primary.GetQueueLength(ctx, queueName)
	if err != nil {
		r.degrader.SetRedisDown(err)
		return 0, nil // Return 0 instead of error
	}

	return length, nil
}

// GetScheduleLength returns the schedule length with fallback behavior
func (r *ResilientRedisClient) GetScheduleLength(ctx context.Context, scheduleName string) (int64, error) {
	if !r.degrader.IsRedisAvailable() || r.primary == nil {
		return 0, nil // Return 0 instead of error
	}

	length, err := r.primary.GetScheduleLength(ctx, scheduleName)
	if err != nil {
		r.degrader.SetRedisDown(err)
		return 0, nil // Return 0 instead of error
	}

	return length, nil
}

// Ping checks Redis connectivity
func (r *ResilientRedisClient) Ping(ctx context.Context) error {
	if r.primary == nil {
		return ErrRedisUnavailable
	}

	return r.primary.Ping(ctx)
}

// Close closes the Redis connection
func (r *ResilientRedisClient) Close() error {
	if r.primary == nil {
		return nil
	}

	return r.primary.Close()
}
