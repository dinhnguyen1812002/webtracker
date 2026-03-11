# Redis Infrastructure

This package provides a Redis client wrapper with connection pooling, cache operations, and job queue operations for the uptime monitoring system.

## Features

- **Connection Pooling**: Efficient connection management with configurable pool size
- **Cache Operations**: Get, Set, Delete with TTL support
- **JSON Operations**: Convenient JSON serialization/deserialization
- **Job Queue**: FIFO queue with blocking dequeue operations
- **Job Scheduling**: Time-based job scheduling using sorted sets

## Usage

### Creating a Client

```go
import "web-tracker/infrastructure/redis"

// Use default configuration
cfg := redis.DefaultConfig()

// Or customize configuration
cfg := redis.Config{
    Addr:         "localhost:6379",
    Password:     "",
    DB:           0,
    PoolSize:     10,
    MinIdleConns: 2,
    MaxRetries:   3,
    DialTimeout:  5 * time.Second,
    ReadTimeout:  3 * time.Second,
    WriteTimeout: 3 * time.Second,
}

client, err := redis.NewClient(cfg)
if err != nil {
    log.Fatal(err)
}
defer client.Close()
```

### Cache Operations

```go
ctx := context.Background()

// Set a value with TTL
err := client.Set(ctx, "key", []byte("value"), 5*time.Minute)

// Get a value
value, err := client.Get(ctx, "key")

// Delete a value
err := client.Delete(ctx, "key")

// JSON operations
type Monitor struct {
    ID   string
    Name string
}

monitor := Monitor{ID: "123", Name: "example.com"}
err := client.SetJSON(ctx, "monitor:123", monitor, 5*time.Minute)

var retrieved Monitor
exists, err := client.GetJSON(ctx, "monitor:123", &retrieved)
```

### Job Queue Operations

```go
ctx := context.Background()
queueName := "health_checks"

// Enqueue a job
job := &redis.Job{
    ID:   "job-1",
    Type: "health_check",
    Payload: map[string]interface{}{
        "monitor_id": "mon-123",
    },
}
err := client.Enqueue(ctx, queueName, job)

// Dequeue a job (blocks for up to 5 seconds)
job, err := client.Dequeue(ctx, queueName, 5*time.Second)
if job == nil {
    // Queue is empty
}

// Get queue length
length, err := client.GetQueueLength(ctx, queueName)
```

### Job Scheduling Operations

```go
ctx := context.Background()
scheduleName := "scheduled_checks"

// Schedule a job for future execution
job := &redis.Job{
    ID:   "job-1",
    Type: "health_check",
    Payload: map[string]interface{}{
        "monitor_id": "mon-123",
    },
}
executeAt := time.Now().Add(5 * time.Minute)
err := client.Schedule(ctx, scheduleName, job, executeAt)

// Get jobs that are due for execution
now := time.Now()
dueJobs, err := client.GetDueJobs(ctx, scheduleName, now)
// dueJobs are automatically removed from the schedule

// Remove a specific scheduled job
err := client.RemoveScheduledJob(ctx, scheduleName, job)

// Get schedule length
length, err := client.GetScheduleLength(ctx, scheduleName)
```

## Testing

The package includes comprehensive tests for all operations. To run the tests, you need a Redis server running on `localhost:6379`:

```bash
# Start Redis with Docker
docker run -d -p 6379:6379 redis:7-alpine

# Run tests
go test -v ./infrastructure/redis/
```

Tests will be skipped if Redis is not available.

## Design Decisions

### Connection Pooling

The client uses go-redis's built-in connection pooling with configurable parameters:
- `PoolSize`: Maximum number of socket connections (default: 10)
- `MinIdleConns`: Minimum number of idle connections (default: 2)
- `MaxRetries`: Maximum number of retries before giving up (default: 3)

### Job Queue Implementation

Jobs are stored in Redis lists using `RPUSH` (enqueue) and `BLPOP` (dequeue) operations:
- FIFO ordering is guaranteed
- Blocking dequeue prevents busy-waiting
- Jobs are JSON-serialized for flexibility

### Job Scheduling Implementation

Scheduled jobs are stored in Redis sorted sets with Unix timestamps as scores:
- Jobs are ordered by execution time
- `GetDueJobs` atomically retrieves and removes due jobs
- Supports efficient time-based queries

### Error Handling

All operations return errors that wrap the underlying Redis errors with context:
- Connection errors are wrapped with descriptive messages
- Serialization errors are caught and reported
- Nil values are distinguished from errors (e.g., key not found)

## Requirements Validation

This implementation satisfies **Requirement 12.5**:
> THE System SHALL cache Monitor configurations in Redis with 5-minute TTL to reduce database queries

The cache operations support:
- TTL-based expiration (configurable per key)
- JSON serialization for complex objects
- Efficient get/set/delete operations
