-- Create health_checks table
CREATE TABLE IF NOT EXISTS health_checks (
    id VARCHAR(255) PRIMARY KEY,
    monitor_id VARCHAR(255) NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    status VARCHAR(50) NOT NULL,
    status_code INTEGER,
    response_time_ms INTEGER NOT NULL,
    ssl_valid BOOLEAN,
    ssl_expires_at TIMESTAMP,
    ssl_days_until INTEGER,
    ssl_issuer TEXT,
    error_message TEXT,
    checked_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create index on monitor_id and checked_at for efficient queries
CREATE INDEX IF NOT EXISTS idx_health_checks_monitor_id ON health_checks(monitor_id, checked_at DESC);

-- Create index on checked_at for date range queries and cleanup operations
CREATE INDEX IF NOT EXISTS idx_health_checks_checked_at ON health_checks(checked_at);

-- Create index on monitor_id and status for filtering by status
CREATE INDEX IF NOT EXISTS idx_health_checks_monitor_status ON health_checks(monitor_id, status);
