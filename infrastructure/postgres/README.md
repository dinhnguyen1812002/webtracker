# PostgreSQL Infrastructure

This package provides PostgreSQL implementations of the domain repositories.

## Components

### Connection Pool (`pool.go`)

Creates and manages a PostgreSQL connection pool using pgx/v5.

**Configuration:**
- MaxConns: 20 (maximum connections)
- MinConns: 2 (minimum connections)
- MaxConnLifetime: 1 hour
- MaxConnIdleTime: 30 minutes

**Usage:**
```go
ctx := context.Background()
config := postgres.DefaultPoolConfig()
config.Host = "localhost"
config.Database = "uptime"

pool, err := postgres.NewPool(ctx, config)
if err != nil {
    log.Fatal(err)
}
defer pool.Close()
```

### Monitor Repository (`monitor_repository.go`)

Implements `domain.MonitorRepository` interface for PostgreSQL.

**Features:**
- Create, Read, Update, Delete operations
- Validation of monitor data
- JSON serialization of alert channels
- Filtering by enabled status
- Pagination support

**Usage:**
```go
repo := postgres.NewMonitorRepository(pool)

monitor := &domain.Monitor{
    ID:            "monitor-1",
    Name:          "My Website",
    URL:           "https://example.com",
    CheckInterval: domain.CheckInterval5Min,
    Enabled:       true,
}

err := repo.Create(ctx, monitor)
```

### Cached Monitor Repository (`cached_monitor_repository.go`)

Wraps `MonitorRepository` with Redis caching to reduce database queries.

**Features:**
- 5-minute TTL for cached monitor configurations (per Requirement 12.5)
- Cache invalidation on updates and deletes
- Automatic cache population on reads
- Graceful degradation (continues working if Redis is unavailable)
- Decorator pattern - wraps existing repository

**Caching Strategy:**
- **Create**: Caches the new monitor immediately
- **GetByID**: Returns from cache if available, otherwise fetches from DB and caches
- **Update**: Invalidates cache to ensure fresh data on next read
- **Delete**: Invalidates cache
- **List**: Not cached (due to complex filter combinations)

**Usage:**
```go
// Create base repository
baseRepo := postgres.NewMonitorRepository(pool)

// Wrap with caching
cachedRepo := postgres.NewCachedMonitorRepository(baseRepo, redisClient)

// Use exactly like the base repository
monitor, err := cachedRepo.GetByID(ctx, "monitor-1")
// First call: cache miss, fetches from DB
// Second call: cache hit, returns from Redis (much faster)
```

**Cache Keys:**
- Format: `cache:monitor:{monitorID}`
- TTL: 5 minutes
- Automatic invalidation on updates/deletes

**Performance Benefits:**
- Reduces database load for frequently accessed monitors
- Faster response times for repeated reads
- Supports high-frequency health check operations

### Alert Repository (`alert_repository.go`)

Implements `domain.AlertRepository` interface for PostgreSQL.

**Features:**
- Create alerts with full metadata
- Query alerts by monitor ID with optional limit
- Query alerts by date range
- Get last alert time for rate limiting
- JSON serialization of details and channels

**Usage:**
```go
repo := postgres.NewAlertRepository(pool)

alert := &domain.Alert{
    ID:        "alert-1",
    MonitorID: "monitor-1",
    Type:      domain.AlertTypeDowntime,
    Severity:  domain.SeverityCritical,
    Message:   "Monitor is down",
    Details: map[string]interface{}{
        "status_code": 500,
    },
    Channels: []domain.AlertChannelType{
        domain.AlertChannelEmail,
    },
}

err := repo.Create(ctx, alert)
```

### Migrations (`migrations.go`)

Automatic database migration system using embedded SQL files.

**Features:**
- Tracks applied migrations in `schema_migrations` table
- Executes migrations in alphabetical order
- Transactional execution
- Idempotent (safe to run multiple times)

**Usage:**
```go
err := postgres.RunMigrations(ctx, pool)
if err != nil {
    log.Fatal(err)
}
```

## Database Schema

### monitors table

```sql
CREATE TABLE monitors (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    url TEXT NOT NULL,
    check_interval INTEGER NOT NULL, -- in seconds
    enabled BOOLEAN DEFAULT true,
    alert_channels JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

**Indexes:**
- `idx_monitors_enabled` - for filtering by enabled status
- `idx_monitors_created_at` - for ordering by creation date

### alerts table

```sql
CREATE TABLE alerts (
    id VARCHAR(255) PRIMARY KEY,
    monitor_id VARCHAR(255) NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    severity VARCHAR(50) NOT NULL,
    message TEXT NOT NULL,
    details JSONB,
    channels JSONB NOT NULL,
    sent_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

**Indexes:**
- `idx_alerts_monitor_id` - for efficient queries by monitor ID and sent_at
- `idx_alerts_sent_at` - for date range queries
- `idx_alerts_type` - for GetLastAlertTime queries (monitor_id, type, sent_at)

## Testing

Run tests with:
```bash
go test ./infrastructure/postgres/... -v
```

Integration tests require a PostgreSQL database and are skipped by default.

## Requirements Validated

This implementation validates:
- **Requirement 10.6**: Monitor configurations are persisted to the database immediately upon creation or update
- **Requirement 12.5**: Monitor configurations are cached in Redis with 5-minute TTL to reduce database queries
- **Requirement 13.1**: Alerts are persisted to the database with timestamp and details
- **Requirement 13.3**: Alert history supports filtering by monitor, date range, and alert type
