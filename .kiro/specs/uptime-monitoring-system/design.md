# Design Document - Uptime Monitoring & Alert System

## Overview

The Uptime Monitoring & Alert System is built using Clean Architecture principles with Go 1.25+, emphasizing separation of concerns, testability, and maintainability. The system consists of four primary layers:

1. **Domain Layer**: Core business logic and entities (Monitor, HealthCheck, Alert)
2. **Use Case Layer**: Application-specific business rules (monitoring orchestration, alert management)
3. **Interface Layer**: API handlers, WebSocket handlers, and external service adapters
4. **Infrastructure Layer**: Database, cache, HTTP clients, and external integrations

The system uses a concurrent worker pool pattern to efficiently process health checks across multiple monitors. A job queue (Redis-backed) ensures reliable scheduling and distribution of health checks. Real-time updates are delivered to the dashboard via WebSocket connections.

### Key Design Decisions

- **Goroutines and Channels**: Leverage Go's concurrency primitives for efficient parallel health checking
- **Clean Architecture**: Dependency injection enables easy testing and swapping of implementations
- **Redis for Job Queue**: Provides reliable, distributed job scheduling with persistence
- **PostgreSQL for Persistence**: Stores monitors, health check history, and alerts with ACID guarantees
- **templ for Frontend**: Type-safe Go templates compiled to efficient Go code
- **WebSocket for Real-time**: Bidirectional communication for instant dashboard updates

## Architecture

### System Architecture Diagram

```mermaid
graph TB
    subgraph "Client Layer"
        Browser[Web Browser]
        API_Client[API Client]
    end

    subgraph "Interface Layer"
        HTTP_Server[HTTP Server - Gin]
        WS_Handler[WebSocket Handler]
        REST_API[REST API Handlers]
        Dashboard_Handler[Dashboard Handler - templ]
    end

    subgraph "Use Case Layer"
        Monitor_Service[Monitor Service]
        Health_Check_Service[Health Check Service]
        Alert_Service[Alert Service]
        Metrics_Service[Metrics Service]
    end

    subgraph "Domain Layer"
        Monitor_Entity[Monitor Entity]
        Health_Check_Entity[Health Check Entity]
        Alert_Entity[Alert Entity]
        Interfaces[Repository Interfaces]
    end

    subgraph "Infrastructure Layer"
        Postgres[(PostgreSQL)]
        Redis[(Redis/Valkey)]
        HTTP_Client[HTTP Client Pool]
        Telegram[Telegram Bot API]
        SMTP[SMTP Client]
        Webhook[Webhook Client]
    end

    subgraph "Worker Pool"
        Scheduler[Job Scheduler]
        Queue[Job Queue - Redis]
        Worker1[Worker 1]
        Worker2[Worker 2]
        WorkerN[Worker N]
    end

    Browser --> HTTP_Server
    Browser --> WS_Handler
    API_Client --> REST_API
    
    HTTP_Server --> Dashboard_Handler
    HTTP_Server --> REST_API
    REST_API --> Monitor_Service
    REST_API --> Metrics_Service
    
    WS_Handler --> Health_Check_Service
    WS_Handler --> Alert_Service
    
    Monitor_Service --> Monitor_Entity
    Health_Check_Service --> Health_Check_Entity
    Alert_Service --> Alert_Entity
    Metrics_Service --> Monitor_Entity
    
    Monitor_Service --> Interfaces
    Health_Check_Service --> Interfaces
    Alert_Service --> Interfaces
    
    Interfaces --> Postgres
    Interfaces --> Redis
    
    Scheduler --> Queue
    Queue --> Worker1
    Queue --> Worker2
    Queue --> WorkerN
    
    Worker1 --> Health_Check_Service
    Worker2 --> Health_Check_Service
    WorkerN --> Health_Check_Service
    
    Health_Check_Service --> HTTP_Client
    Health_Check_Service --> Alert_Service
    
    Alert_Service --> Telegram
    Alert_Service --> SMTP
    Alert_Service --> Webhook
    Alert_Service --> WS_Handler
```

### Data Flow

1. **Monitor Creation Flow**:
   - User creates monitor via REST API
   - Monitor Service validates and persists to PostgreSQL
   - Scheduler adds job to Redis queue based on check interval
   - Cache invalidation ensures fresh data

2. **Health Check Execution Flow**:
   - Scheduler enqueues health check job at configured interval
   - Available worker dequeues job from Redis
   - Worker executes HTTP request with timeout and retry logic
   - Worker validates SSL certificate if HTTPS
   - Worker records result to PostgreSQL
   - Worker updates metrics in Redis cache
   - If failure detected, worker triggers Alert Service
   - WebSocket handler broadcasts status update to connected clients

3. **Alert Delivery Flow**:
   - Alert Service receives alert trigger
   - Rate limiter checks if alert should be suppressed
   - Alert Service persists alert to PostgreSQL
   - Alert Service delivers to all configured channels concurrently
   - Each channel adapter handles retries independently
   - WebSocket handler broadcasts alert to dashboard

## Components and Interfaces

### Domain Entities

#### Monitor Entity

```go
type Monitor struct {
    ID              string
    Name            string
    URL             string
    CheckInterval   time.Duration // 1m, 5m, 15m, 60m
    Enabled         bool
    AlertChannels   []AlertChannel
    CreatedAt       time.Time
    UpdatedAt       time.Time
}

type AlertChannel struct {
    Type   AlertChannelType // Telegram, Email, Webhook
    Config map[string]string // Channel-specific configuration
}

type AlertChannelType string

const (
    AlertChannelTelegram AlertChannelType = "telegram"
    AlertChannelEmail    AlertChannelType = "email"
    AlertChannelWebhook  AlertChannelType = "webhook"
)
```

#### HealthCheck Entity

```go
type HealthCheck struct {
    ID           string
    MonitorID    string
    Status       HealthCheckStatus
    StatusCode   int
    ResponseTime time.Duration
    SSLInfo      *SSLInfo
    ErrorMessage string
    CheckedAt    time.Time
}

type HealthCheckStatus string

const (
    StatusSuccess HealthCheckStatus = "success"
    StatusFailure HealthCheckStatus = "failure"
    StatusTimeout HealthCheckStatus = "timeout"
)

type SSLInfo struct {
    Valid      bool
    ExpiresAt  time.Time
    DaysUntil  int
    Issuer     string
}
```

#### Alert Entity

```go
type Alert struct {
    ID          string
    MonitorID   string
    Type        AlertType
    Severity    AlertSeverity
    Message     string
    Details     map[string]interface{}
    SentAt      time.Time
    Channels    []AlertChannelType
}

type AlertType string

const (
    AlertTypeDowntime     AlertType = "downtime"
    AlertTypeRecovery     AlertType = "recovery"
    AlertTypeSSLExpiring  AlertType = "ssl_expiring"
    AlertTypeSSLExpired   AlertType = "ssl_expired"
    AlertTypePerformance  AlertType = "performance"
)

type AlertSeverity string

const (
    SeverityInfo     AlertSeverity = "info"
    SeverityWarning  AlertSeverity = "warning"
    SeverityCritical AlertSeverity = "critical"
)
```

### Repository Interfaces

```go
type MonitorRepository interface {
    Create(ctx context.Context, monitor *Monitor) error
    GetByID(ctx context.Context, id string) (*Monitor, error)
    List(ctx context.Context, filters ListFilters) ([]*Monitor, error)
    Update(ctx context.Context, monitor *Monitor) error
    Delete(ctx context.Context, id string) error
}

type HealthCheckRepository interface {
    Create(ctx context.Context, check *HealthCheck) error
    GetByMonitorID(ctx context.Context, monitorID string, limit int) ([]*HealthCheck, error)
    GetByDateRange(ctx context.Context, monitorID string, start, end time.Time) ([]*HealthCheck, error)
    DeleteOlderThan(ctx context.Context, before time.Time) error
}

type AlertRepository interface {
    Create(ctx context.Context, alert *Alert) error
    GetByMonitorID(ctx context.Context, monitorID string, limit int) ([]*Alert, error)
    GetByDateRange(ctx context.Context, monitorID string, start, end time.Time) ([]*Alert, error)
    GetLastAlertTime(ctx context.Context, monitorID string, alertType AlertType) (*time.Time, error)
}
```

### Use Case Services

#### Monitor Service

```go
type MonitorService interface {
    CreateMonitor(ctx context.Context, req CreateMonitorRequest) (*Monitor, error)
    GetMonitor(ctx context.Context, id string) (*Monitor, error)
    ListMonitors(ctx context.Context, filters ListFilters) ([]*Monitor, error)
    UpdateMonitor(ctx context.Context, id string, req UpdateMonitorRequest) (*Monitor, error)
    DeleteMonitor(ctx context.Context, id string) error
}
```

**Responsibilities**:
- Validate monitor configuration (URL format, check interval)
- Persist monitors to database
- Trigger job scheduler to add/update/remove scheduled checks
- Invalidate cache on updates

#### Health Check Service

```go
type HealthCheckService interface {
    ExecuteCheck(ctx context.Context, monitorID string) (*HealthCheck, error)
    GetCheckHistory(ctx context.Context, monitorID string, limit int) ([]*HealthCheck, error)
}
```

**Responsibilities**:
- Execute HTTP/HTTPS requests with timeout (30s)
- Implement retry logic with exponential backoff (3 attempts)
- Extract and validate SSL certificates
- Measure response time
- Persist health check results
- Trigger alerts on failures
- Update real-time metrics cache

#### Alert Service

```go
type AlertService interface {
    SendAlert(ctx context.Context, alert *Alert) error
    GetAlertHistory(ctx context.Context, monitorID string, filters AlertFilters) ([]*Alert, error)
}
```

**Responsibilities**:
- Apply rate limiting rules (15-minute suppression, 1-hour reminders)
- Persist alerts to database
- Deliver alerts to all configured channels concurrently
- Handle channel-specific retry logic
- Broadcast alerts to WebSocket connections

#### Metrics Service

```go
type MetricsService interface {
    GetUptimePercentage(ctx context.Context, monitorID string, period time.Duration) (float64, error)
    GetResponseTimeStats(ctx context.Context, monitorID string, period time.Duration) (*ResponseTimeStats, error)
}

type ResponseTimeStats struct {
    Average time.Duration
    Min     time.Duration
    Max     time.Duration
    P95     time.Duration
    P99     time.Duration
}
```

**Responsibilities**:
- Calculate uptime percentages for 24h, 7d, 30d periods
- Calculate response time statistics
- Cache calculations in Redis with appropriate TTL
- Invalidate cache on new health check results

### Infrastructure Components

#### HTTP Client Pool

```go
type HTTPClient interface {
    Do(ctx context.Context, req *http.Request) (*http.Response, error)
}
```

**Implementation Details**:
- Connection pooling with MaxIdleConns=100, MaxIdleConnsPerHost=10
- Timeout: 30 seconds
- TLS configuration to capture certificate details
- Retry logic: 3 attempts with exponential backoff (1s, 2s, 4s)

#### Job Scheduler

```go
type Scheduler interface {
    ScheduleMonitor(monitor *Monitor) error
    UnscheduleMonitor(monitorID string) error
    Start(ctx context.Context) error
    Stop() error
}
```

**Implementation Details**:
- Uses Redis sorted sets for time-based job scheduling
- Enqueues jobs when their scheduled time arrives
- Distributes jobs to avoid burst load (jitter: ±10% of interval)
- Handles monitor updates by rescheduling

#### Worker Pool

```go
type WorkerPool interface {
    Start(ctx context.Context, numWorkers int) error
    Stop() error
    GetStats() WorkerPoolStats
}

type WorkerPoolStats struct {
    ActiveWorkers int
    QueueDepth    int
    ProcessedJobs int64
}
```

**Implementation Details**:
- Default 10 concurrent workers (configurable)
- Workers consume jobs from Redis queue
- Each worker runs in a goroutine
- Graceful shutdown waits for in-flight jobs
- Queue depth monitoring for capacity planning

#### Alert Channel Adapters

```go
type AlertChannelAdapter interface {
    Send(ctx context.Context, alert *Alert, config map[string]string) error
}
```

**Telegram Adapter**:
- Uses Telegram Bot API
- Formats messages with Markdown
- Retries: 3 attempts with exponential backoff

**Email Adapter**:
- Uses SMTP with TLS
- HTML email templates
- Retries: 3 attempts with exponential backoff

**Webhook Adapter**:
- HTTP POST with JSON payload
- Custom headers support
- Retries: 3 attempts with exponential backoff

## Data Models

### Database Schema (PostgreSQL)

```sql
-- Monitors table
CREATE TABLE monitors (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    url TEXT NOT NULL,
    check_interval INTEGER NOT NULL, -- in seconds
    enabled BOOLEAN DEFAULT true,
    alert_channels JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_monitors_enabled ON monitors(enabled);

-- Health checks table
CREATE TABLE health_checks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    monitor_id UUID NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    status VARCHAR(50) NOT NULL,
    status_code INTEGER,
    response_time_ms INTEGER NOT NULL,
    ssl_valid BOOLEAN,
    ssl_expires_at TIMESTAMP,
    ssl_days_until INTEGER,
    error_message TEXT,
    checked_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_health_checks_monitor_id ON health_checks(monitor_id, checked_at DESC);
CREATE INDEX idx_health_checks_checked_at ON health_checks(checked_at);

-- Alerts table
CREATE TABLE alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    monitor_id UUID NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    severity VARCHAR(50) NOT NULL,
    message TEXT NOT NULL,
    details JSONB,
    channels JSONB NOT NULL,
    sent_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_alerts_monitor_id ON alerts(monitor_id, sent_at DESC);
CREATE INDEX idx_alerts_sent_at ON alerts(sent_at);
CREATE INDEX idx_alerts_type ON alerts(monitor_id, type, sent_at DESC);
```

### Redis Data Structures

**Job Queue** (List):
```
queue:health_checks -> [job1, job2, job3, ...]
```

**Job Schedule** (Sorted Set):
```
schedule:health_checks -> {monitorID: nextExecutionTimestamp}
```

**Monitor Cache** (Hash):
```
cache:monitor:{monitorID} -> {monitor JSON}
TTL: 5 minutes
```

**Metrics Cache** (Hash):
```
cache:metrics:{monitorID}:uptime:24h -> percentage
cache:metrics:{monitorID}:uptime:7d -> percentage
cache:metrics:{monitorID}:uptime:30d -> percentage
cache:metrics:{monitorID}:response:1h -> stats JSON
TTL: 1 minute
```

**Rate Limit** (String):
```
ratelimit:alert:{monitorID}:{alertType} -> lastAlertTimestamp
TTL: 15 minutes (for downtime), 24 hours (for SSL)
```

## API Endpoints

### Monitor Management

```
POST   /api/v1/monitors          - Create monitor
GET    /api/v1/monitors          - List monitors
GET    /api/v1/monitors/:id      - Get monitor details
PUT    /api/v1/monitors/:id      - Update monitor
DELETE /api/v1/monitors/:id      - Delete monitor
```

### Health Checks

```
GET    /api/v1/monitors/:id/checks        - Get health check history
GET    /api/v1/monitors/:id/checks/latest - Get latest health check
```

### Alerts

```
GET    /api/v1/monitors/:id/alerts - Get alert history
```

### Metrics

```
GET    /api/v1/monitors/:id/uptime   - Get uptime percentages
GET    /api/v1/monitors/:id/response - Get response time stats
```

### System

```
GET    /health  - System health check
GET    /metrics - Prometheus-compatible metrics
```

### WebSocket

```
WS     /ws      - Real-time updates
```

**WebSocket Message Format**:
```json
{
  "type": "health_check_update",
  "data": {
    "monitor_id": "uuid",
    "status": "success",
    "response_time_ms": 150,
    "checked_at": "2024-01-01T00:00:00Z"
  }
}

{
  "type": "alert",
  "data": {
    "monitor_id": "uuid",
    "type": "downtime",
    "severity": "critical",
    "message": "Monitor is down"
  }
}
```


## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system—essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*

### Property Reflection

After analyzing all acceptance criteria, I identified several areas of redundancy:

- Properties 1.2 and 1.3 are complementary (success vs failure classification) - can be combined into one property about status classification
- Property 10.2 is redundant with 3.1 and 3.2 (check interval validation)
- Properties 2.2, 2.3, 2.4 can be combined into one property about SSL expiration thresholds
- Properties 7.2 and 8.2 are examples of time period support, not universal properties
- Properties 13.2 and 14.2 are similar data retention properties - can be generalized
- Properties 15.1, 15.2, 15.3 are specific examples of endpoint behavior, not universal properties

### Health Check Execution Properties

**Property 1: HTTP Request Execution**
*For any* monitor with a valid URL, when a health check is performed, the system should send an HTTP/HTTPS request to that URL and record the attempt.
**Validates: Requirements 1.1**

**Property 2: Status Code Classification**
*For any* HTTP response, the system should classify status codes 200-299 as success and all other codes (or network errors) as failure.
**Validates: Requirements 1.2, 1.3**

**Property 3: Response Time Recording**
*For any* health check execution, the system should measure and record a non-negative response time value.
**Validates: Requirements 1.4**

**Property 4: Retry with Exponential Backoff**
*For any* failed health check, the system should retry up to 2 additional times (3 total attempts) with exponentially increasing delays between attempts.
**Validates: Requirements 1.6**

### SSL Certificate Monitoring Properties

**Property 5: SSL Certificate Extraction**
*For any* HTTPS monitor, when a health check is performed, the system should extract and validate the SSL certificate information.
**Validates: Requirements 2.1**

**Property 6: SSL Expiration Alert Thresholds**
*For any* SSL certificate, the system should generate alerts with appropriate severity levels based on days until expiration: 30 days (warning), 15 days (warning), 7 days (critical), or expired (critical).
**Validates: Requirements 2.2, 2.3, 2.4, 2.5**

**Property 7: Invalid SSL Certificate Handling**
*For any* HTTPS monitor with an invalid or unverifiable SSL certificate, the system should record the health check as failed.
**Validates: Requirements 2.6**

### Monitor Configuration Properties

**Property 8: Check Interval Validation**
*For any* monitor creation or update request, the system should accept check intervals of 1, 5, 15, or 60 minutes and reject all other values.
**Validates: Requirements 3.1, 3.2, 10.2**

**Property 9: Scheduled Check Execution**
*For any* enabled monitor, when its check interval elapses, the system should schedule a health check for execution.
**Validates: Requirements 3.3**

**Property 10: Load Distribution**
*For any* set of monitors with aligned check intervals, the system should distribute health checks with jitter to avoid burst load (no more than 50% of checks within the same 10-second window).
**Validates: Requirements 3.4**

**Property 11: Timing Accuracy**
*For any* monitor, the actual check interval timing should not drift more than 5 seconds per hour from the configured interval.
**Validates: Requirements 3.5**

**Property 12: URL Validation**
*For any* monitor creation request, the system should validate that the URL is a properly formatted HTTP or HTTPS endpoint.
**Validates: Requirements 10.1**

**Property 13: Configuration Persistence**
*For any* monitor creation or update, the system should persist the configuration to the database such that retrieving the monitor immediately after returns the same configuration.
**Validates: Requirements 10.6**

**Property 14: Monitor Update Propagation**
*For any* monitor update, the next scheduled health check should use the updated configuration.
**Validates: Requirements 10.3**

**Property 15: Monitor Deletion Cleanup**
*For any* monitor deletion, the system should remove all associated data (health checks, alerts, scheduled jobs) such that no orphaned data remains.
**Validates: Requirements 10.4**

### Alert Delivery Properties

**Property 16: Multi-Channel Alert Delivery**
*For any* alert and any monitor with configured alert channels, the system should deliver the alert to all configured channels.
**Validates: Requirements 4.1**

**Property 17: Channel-Specific Delivery**
*For any* alert, the system should use the appropriate delivery mechanism for each channel type: Telegram Bot API for Telegram, SMTP for Email, HTTP POST for Webhook.
**Validates: Requirements 4.2, 4.3, 4.4**

**Property 18: Alert Delivery Retry**
*For any* alert channel delivery failure, the system should retry up to 3 times with exponential backoff before giving up.
**Validates: Requirements 4.5**

**Property 19: Alert Delivery Isolation**
*For any* alert with multiple channels, if one channel fails after all retries, the system should continue delivering to other channels.
**Validates: Requirements 4.6**

**Property 20: Alert Persistence**
*For any* generated alert, the system should persist it to the database with timestamp, type, severity, and details.
**Validates: Requirements 5.1, 13.1**

### Rate Limiting Properties

**Property 21: Downtime Alert Suppression**
*For any* monitor, if a downtime alert was sent within the last 15 minutes, subsequent downtime alerts should be suppressed.
**Validates: Requirements 5.2**

**Property 22: Recovery Alert Exception**
*For any* monitor that recovers after failure, a recovery alert should be sent immediately regardless of rate limiting.
**Validates: Requirements 5.3**

**Property 23: Prolonged Failure Reminder**
*For any* monitor that has been failing continuously for 1 hour, a reminder alert should be sent even if rate limiting is active.
**Validates: Requirements 5.4**

**Property 24: SSL Alert Daily Limit**
*For any* monitor, SSL expiration alerts of the same warning level should be sent at most once per 24-hour period.
**Validates: Requirements 5.5**

### Worker Pool Properties

**Property 25: Worker Distribution**
*For any* set of scheduled health checks, the system should distribute them across available workers such that no worker is idle while checks are queued.
**Validates: Requirements 6.2, 6.4**

**Property 26: Queue Under Load**
*For any* situation where health checks are scheduled faster than workers can process them, the system should queue pending checks without dropping them.
**Validates: Requirements 6.3**

**Property 27: Monitor Isolation**
*For any* monitor with a slow or hanging health check, other monitors should continue to be checked without blocking.
**Validates: Requirements 6.5**

### Metrics Calculation Properties

**Property 28: Uptime Percentage Formula**
*For any* monitor and time period, the uptime percentage should equal (successful checks / total checks) × 100, rounded to 2 decimal places.
**Validates: Requirements 7.1, 7.4**

**Property 29: Metrics Update Timeliness**
*For any* completed health check, the uptime percentage and response time metrics should be updated within 1 minute.
**Validates: Requirements 7.5**

**Property 30: Response Time Statistics**
*For any* monitor and time period, the system should calculate minimum, maximum, and average response times from all successful health checks in that period.
**Validates: Requirements 8.1, 8.3**

**Property 31: Performance Alert Threshold**
*For any* health check where response time exceeds the configured threshold, the system should generate a performance alert.
**Validates: Requirements 8.4**

### Data Retention Properties

**Property 32: Historical Data Retention**
*For any* health check or alert, the system should retain the record for at least 90 days before deletion.
**Validates: Requirements 8.5, 13.2, 14.2**

**Property 33: Health Check Aggregation**
*For any* health check older than 7 days, the system should aggregate it into hourly summaries while preserving the statistical properties (count, success rate, avg response time).
**Validates: Requirements 14.4**

### Real-Time Update Properties

**Property 34: WebSocket Connection Establishment**
*For any* dashboard client connection request, the system should establish a WebSocket connection successfully.
**Validates: Requirements 9.1**

**Property 35: Health Check Broadcast Timeliness**
*For any* completed health check, the system should broadcast the status update to all connected WebSocket clients within 2 seconds.
**Validates: Requirements 9.2**

**Property 36: Alert Broadcast Timeliness**
*For any* generated alert, the system should broadcast it to all connected WebSocket clients within 2 seconds.
**Validates: Requirements 9.3**

**Property 37: WebSocket Reconnection**
*For any* WebSocket connection loss, the dashboard client should attempt to reconnect automatically.
**Validates: Requirements 9.4**

**Property 38: Dashboard Load Performance**
*For any* dashboard page load request, the system should deliver the initial page within 200 milliseconds.
**Validates: Requirements 9.5**

### Query Performance Properties

**Property 39: Alert History Query Performance**
*For any* alert history query with up to 10,000 records, the system should return results within 500 milliseconds.
**Validates: Requirements 13.5**

**Property 40: Health Check History Query Performance**
*For any* health check history query, the system should return results within 500 milliseconds.
**Validates: Requirements 14.5**

**Property 41: Query Filtering Support**
*For any* alert or health check history query, the system should correctly filter results by monitor ID, date range, and type (for alerts).
**Validates: Requirements 13.3, 14.3**

**Property 42: Pagination Support**
*For any* alert history query with pagination parameters, the system should return the correct page of results with the specified page size.
**Validates: Requirements 13.4**

### Configuration Management Properties

**Property 43: Configuration File Loading**
*For any* valid YAML or JSON configuration file, the system should load all settings correctly at startup.
**Validates: Requirements 11.1**

**Property 44: Invalid Configuration Handling**
*For any* configuration file with invalid syntax, the system should fail to start and log detailed error messages.
**Validates: Requirements 11.2**

**Property 45: Startup Validation**
*For any* configuration with alert channel credentials or database parameters, the system should validate connectivity at startup before accepting requests.
**Validates: Requirements 11.3, 11.4**

**Property 46: Environment Variable Substitution**
*For any* configuration value containing environment variable references (e.g., ${VAR_NAME}), the system should substitute the actual environment variable value.
**Validates: Requirements 11.5**

### Resource Efficiency Properties

**Property 47: Memory Usage Under Load**
*For any* system configuration monitoring 100 websites, the total memory consumption should remain below 100MB.
**Validates: Requirements 12.1**

**Property 48: Idle Memory Usage**
*For any* system in idle state with no scheduled health checks, memory consumption should remain below 10MB.
**Validates: Requirements 12.2**

**Property 49: HTTP Connection Reuse**
*For any* sequence of health checks to the same host, the system should reuse HTTP connections rather than creating new connections for each check.
**Validates: Requirements 12.4**

**Property 50: Monitor Configuration Caching**
*For any* monitor configuration access, if the configuration was accessed within the last 5 minutes, the system should serve it from Redis cache rather than querying the database.
**Validates: Requirements 12.5**

### System Resilience Properties

**Property 51: Error Logging**
*For any* error or warning condition, the system should write a structured log entry with timestamp, severity, and context.
**Validates: Requirements 15.4**

**Property 52: Graceful Degradation**
*For any* critical error (database connection loss, Redis unavailability), the system should continue operating in degraded mode rather than crashing completely.
**Validates: Requirements 15.5**

## Error Handling

### Error Categories

1. **Network Errors**: Connection timeouts, DNS failures, connection refused
   - Strategy: Retry with exponential backoff, record as failed health check
   - User Impact: Alert sent if monitor fails after retries

2. **SSL Certificate Errors**: Invalid certificate, expired certificate, hostname mismatch
   - Strategy: Record as failed health check, generate SSL-specific alert
   - User Impact: Immediate alert with certificate details

3. **Database Errors**: Connection loss, query timeout, constraint violations
   - Strategy: Retry transient errors, log persistent errors, continue with cached data
   - User Impact: System continues operating with potentially stale data

4. **Redis Errors**: Connection loss, command timeout
   - Strategy: Fall back to database, disable caching temporarily
   - User Impact: Increased latency, no data loss

5. **Alert Delivery Errors**: Telegram API failure, SMTP error, webhook timeout
   - Strategy: Retry with exponential backoff, log failure after max retries
   - User Impact: Alert may not be delivered, logged for manual review

6. **Configuration Errors**: Invalid YAML/JSON, missing required fields, invalid values
   - Strategy: Fail fast at startup with detailed error messages
   - User Impact: System does not start until configuration is fixed

7. **Resource Exhaustion**: Worker pool full, queue depth exceeded, memory limit
   - Strategy: Apply backpressure, queue checks, log warnings
   - User Impact: Increased check latency, no data loss

### Error Recovery Strategies

**Transient Errors**: Retry with exponential backoff (1s, 2s, 4s, 8s, 16s max)

**Persistent Errors**: Log error, continue operation, alert system administrator

**Critical Errors**: Enter degraded mode, disable non-essential features, maintain core monitoring

### Circuit Breaker Pattern

For external service integrations (Telegram, SMTP, Webhooks):
- Open circuit after 5 consecutive failures
- Half-open after 60 seconds to test recovery
- Close circuit after 3 consecutive successes

## Testing Strategy

### Dual Testing Approach

The system will use both unit testing and property-based testing to ensure comprehensive coverage:

**Unit Tests**: Focus on specific examples, edge cases, and integration points
- Specific HTTP status codes (200, 404, 500)
- Edge cases (empty monitor list, no health check history)
- Error conditions (database connection failure, invalid SSL certificate)
- Integration between components (monitor service → scheduler)

**Property-Based Tests**: Verify universal properties across all inputs
- Generate random monitors with various configurations
- Generate random health check results with various status codes
- Generate random alert scenarios with multiple channels
- Test with random time periods and data volumes

Both approaches are complementary and necessary for comprehensive coverage. Unit tests catch concrete bugs in specific scenarios, while property tests verify general correctness across the input space.

### Property-Based Testing Configuration

**Library**: Use `gopter` (Go property testing library)

**Test Configuration**:
- Minimum 100 iterations per property test
- Each test tagged with: `Feature: uptime-monitoring-system, Property {N}: {property_text}`
- Custom generators for domain entities (Monitor, HealthCheck, Alert)

**Example Property Test Structure**:
```go
// Feature: uptime-monitoring-system, Property 2: Status Code Classification
func TestProperty_StatusCodeClassification(t *testing.T) {
    properties := gopter.NewProperties(nil)
    
    properties.Property("status codes 200-299 classified as success", 
        prop.ForAll(
            func(statusCode int) bool {
                result := classifyStatusCode(statusCode)
                if statusCode >= 200 && statusCode <= 299 {
                    return result == StatusSuccess
                } else {
                    return result == StatusFailure
                }
            },
            gen.IntRange(100, 599),
        ))
    
    properties.TestingRun(t, gopter.ConsoleReporter(false))
}
```

### Test Coverage Requirements

- Overall code coverage: minimum 80%
- Critical paths (health check execution, alert delivery): minimum 95%
- Error handling paths: minimum 70%
- Property tests: one test per correctness property (52 properties)
- Unit tests: focus on edge cases and integration points

### Integration Testing

**Database Integration**:
- Test with real PostgreSQL instance (Docker container)
- Test connection pooling and transaction handling
- Test query performance with large datasets

**Redis Integration**:
- Test with real Redis instance (Docker container)
- Test job queue operations
- Test cache invalidation

**External Services**:
- Mock Telegram Bot API, SMTP server, Webhook endpoints
- Test retry logic and circuit breaker behavior
- Test error handling for service unavailability

### Performance Testing

**Load Testing**:
- Simulate 100 monitors with 1-minute check intervals
- Measure memory usage, CPU usage, response times
- Verify system meets performance requirements

**Stress Testing**:
- Gradually increase monitor count beyond 100
- Identify breaking points and bottlenecks
- Verify graceful degradation under extreme load

**Endurance Testing**:
- Run system for 24+ hours under normal load
- Monitor for memory leaks, connection leaks
- Verify long-term stability

## Deployment Architecture

### Docker Compose Setup

```yaml
version: '3.8'

services:
  app:
    build: .
    ports:
      - "8080:8080"
    environment:
      - DATABASE_URL=postgresql://user:pass@postgres:5432/uptime
      - REDIS_URL=redis://redis:6379
    depends_on:
      - postgres
      - redis
    restart: unless-stopped

  postgres:
    image: postgres:16-alpine
    environment:
      - POSTGRES_DB=uptime
      - POSTGRES_USER=user
      - POSTGRES_PASSWORD=pass
    volumes:
      - postgres_data:/var/lib/postgresql/data
    restart: unless-stopped

  redis:
    image: valkey/valkey:7-alpine
    volumes:
      - redis_data:/data
    restart: unless-stopped

volumes:
  postgres_data:
  redis_data:
```

### Scaling Considerations

**Horizontal Scaling**:
- Multiple app instances can run concurrently
- Redis job queue ensures work distribution
- PostgreSQL handles concurrent connections
- WebSocket connections distributed via load balancer with sticky sessions

**Vertical Scaling**:
- Increase worker pool size for more concurrent checks
- Increase database connection pool for higher throughput
- Increase memory limit for larger monitor counts

### Monitoring and Observability

**Metrics** (Prometheus format):
- `uptime_monitor_checks_total{status}` - Total health checks by status
- `uptime_monitor_check_duration_seconds` - Health check duration histogram
- `uptime_alert_deliveries_total{channel,status}` - Alert deliveries by channel and status
- `uptime_worker_pool_utilization` - Worker pool utilization percentage
- `uptime_queue_depth` - Current job queue depth

**Logging**:
- Structured JSON logs
- Log levels: DEBUG, INFO, WARN, ERROR, FATAL
- Correlation IDs for request tracing

**Health Checks**:
- `/health` - Basic health check (200 OK if operational)
- `/health/ready` - Readiness check (database and Redis connectivity)
- `/health/live` - Liveness check (application responsive)

## Security Considerations

**Authentication**: API endpoints protected with JWT tokens or API keys

**Authorization**: Role-based access control (admin, user, read-only)

**Data Encryption**:
- TLS for all HTTP traffic
- Encrypted database connections
- Sensitive configuration values encrypted at rest

**Input Validation**:
- URL validation to prevent SSRF attacks
- Check interval validation to prevent resource exhaustion
- Alert channel configuration validation

**Rate Limiting**:
- API rate limiting per user/IP
- Alert delivery rate limiting per monitor
- WebSocket connection limits per client

**Secrets Management**:
- Environment variables for sensitive values
- Support for external secret managers (HashiCorp Vault, AWS Secrets Manager)
- No secrets in configuration files or code
