package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client wraps the Redis client with connection pooling and provides
// cache and job queue operations for the uptime monitoring system.
type Client struct {
	client *redis.Client
}

// Config holds the configuration for the Redis client.
type Config struct {
	Addr         string        // Redis server address (host:port)
	Password     string        // Redis password (empty if no auth)
	DB           int           // Redis database number
	PoolSize     int           // Maximum number of socket connections
	MinIdleConns int           // Minimum number of idle connections
	MaxRetries   int           // Maximum number of retries before giving up
	DialTimeout  time.Duration // Timeout for establishing new connections
	ReadTimeout  time.Duration // Timeout for socket reads
	WriteTimeout time.Duration // Timeout for socket writes
}

// DefaultConfig returns a Config with sensible defaults.
// Optimized for memory efficiency while maintaining performance
func DefaultConfig() Config {
	return Config{
		Addr:         "localhost:6379",
		Password:     "",
		DB:           0,
		PoolSize:     5, // Reduced from 10 to save memory
		MinIdleConns: 1, // Reduced from 2 to save memory
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}
}

// NewClient creates a new Redis client with connection pooling.
func NewClient(cfg Config) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		MaxRetries:   cfg.MaxRetries,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &Client{client: rdb}, nil
}

// Close closes the Redis client connection pool.
func (c *Client) Close() error {
	return c.client.Close()
}

// Ping checks if the Redis server is reachable.
func (c *Client) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// --- Cache Operations ---

// Get retrieves a value from the cache by key.
// Returns nil if the key does not exist.
func (c *Client) Get(ctx context.Context, key string) ([]byte, error) {
	val, err := c.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil // Key does not exist
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get key %s: %w", key, err)
	}
	return val, nil
}

// Set stores a value in the cache with the specified TTL.
// If ttl is 0, the key will not expire.
func (c *Client) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	err := c.client.Set(ctx, key, value, ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to set key %s: %w", key, err)
	}
	return nil
}

// Delete removes a key from the cache.
func (c *Client) Delete(ctx context.Context, key string) error {
	err := c.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete key %s: %w", key, err)
	}
	return nil
}

// SetJSON stores a JSON-serializable value in the cache with the specified TTL.
func (c *Client) SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}
	return c.Set(ctx, key, data, ttl)
}

// GetJSON retrieves and unmarshals a JSON value from the cache.
// Returns false if the key does not exist.
func (c *Client) GetJSON(ctx context.Context, key string, dest interface{}) (bool, error) {
	data, err := c.Get(ctx, key)
	if err != nil {
		return false, err
	}
	if data == nil {
		return false, nil // Key does not exist
	}
	if err := json.Unmarshal(data, dest); err != nil {
		return false, fmt.Errorf("failed to unmarshal value: %w", err)
	}
	return true, nil
}

// --- Job Queue Operations ---

// Job represents a job in the queue.
type Job struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Payload   map[string]interface{} `json:"payload"`
	CreatedAt time.Time              `json:"created_at"`
}

// Enqueue adds a job to the queue.
func (c *Client) Enqueue(ctx context.Context, queueName string, job *Job) error {
	if job.CreatedAt.IsZero() {
		job.CreatedAt = time.Now()
	}

	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	err = c.client.RPush(ctx, queueName, data).Err()
	if err != nil {
		return fmt.Errorf("failed to enqueue job: %w", err)
	}

	return nil
}

// Dequeue removes and returns a job from the queue.
// Returns nil if the queue is empty.
// Blocks for up to the specified timeout waiting for a job.
func (c *Client) Dequeue(ctx context.Context, queueName string, timeout time.Duration) (*Job, error) {
	result, err := c.client.BLPop(ctx, timeout, queueName).Result()
	if err == redis.Nil {
		return nil, nil // Queue is empty
	}
	if err != nil {
		return nil, fmt.Errorf("failed to dequeue job: %w", err)
	}

	// BLPop returns [queueName, value]
	if len(result) != 2 {
		return nil, fmt.Errorf("unexpected BLPop result length: %d", len(result))
	}

	var job Job
	if err := json.Unmarshal([]byte(result[1]), &job); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job: %w", err)
	}

	return &job, nil
}

// Schedule adds a job to a sorted set with a score representing the execution time.
// This allows for time-based job scheduling.
func (c *Client) Schedule(ctx context.Context, scheduleName string, job *Job, executeAt time.Time) error {
	if job.CreatedAt.IsZero() {
		job.CreatedAt = time.Now()
	}

	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	score := float64(executeAt.Unix())
	err = c.client.ZAdd(ctx, scheduleName, redis.Z{
		Score:  score,
		Member: data,
	}).Err()
	if err != nil {
		return fmt.Errorf("failed to schedule job: %w", err)
	}

	return nil
}

// GetDueJobs retrieves all jobs from the schedule that are due to be executed.
// Jobs are returned in order of their scheduled execution time.
// The jobs are removed from the schedule.
func (c *Client) GetDueJobs(ctx context.Context, scheduleName string, now time.Time) ([]*Job, error) {
	maxScore := float64(now.Unix())

	// Get all jobs with score <= now
	result, err := c.client.ZRangeByScoreWithScores(ctx, scheduleName, &redis.ZRangeBy{
		Min: "-inf",
		Max: fmt.Sprintf("%f", maxScore),
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get due jobs: %w", err)
	}

	if len(result) == 0 {
		return nil, nil
	}

	jobs := make([]*Job, 0, len(result))
	members := make([]interface{}, 0, len(result))

	for _, z := range result {
		var job Job
		if err := json.Unmarshal([]byte(z.Member.(string)), &job); err != nil {
			return nil, fmt.Errorf("failed to unmarshal job: %w", err)
		}
		jobs = append(jobs, &job)
		members = append(members, z.Member)
	}

	// Remove the jobs from the schedule
	if len(members) > 0 {
		err = c.client.ZRem(ctx, scheduleName, members...).Err()
		if err != nil {
			return nil, fmt.Errorf("failed to remove due jobs: %w", err)
		}
	}

	return jobs, nil
}

// RemoveScheduledJob removes a specific job from the schedule by its serialized form.
func (c *Client) RemoveScheduledJob(ctx context.Context, scheduleName string, job *Job) error {
	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	err = c.client.ZRem(ctx, scheduleName, data).Err()
	if err != nil {
		return fmt.Errorf("failed to remove scheduled job: %w", err)
	}

	return nil
}

// GetQueueLength returns the number of jobs in the queue.
func (c *Client) GetQueueLength(ctx context.Context, queueName string) (int64, error) {
	length, err := c.client.LLen(ctx, queueName).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get queue length: %w", err)
	}
	return length, nil
}

// GetScheduleLength returns the number of jobs in the schedule.
func (c *Client) GetScheduleLength(ctx context.Context, scheduleName string) (int64, error) {
	length, err := c.client.ZCard(ctx, scheduleName).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get schedule length: %w", err)
	}
	return length, nil
}
