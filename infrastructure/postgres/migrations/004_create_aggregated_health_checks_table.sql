-- Create aggregated health checks table for hourly summaries
CREATE TABLE IF NOT EXISTS aggregated_health_checks (
    id VARCHAR(255) PRIMARY KEY DEFAULT gen_random_uuid()::text,
    monitor_id VARCHAR(255) NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    hour_timestamp TIMESTAMP NOT NULL, -- Start of the hour (e.g., 2024-01-01 14:00:00)
    total_checks INTEGER NOT NULL DEFAULT 0,
    successful_checks INTEGER NOT NULL DEFAULT 0,
    failed_checks INTEGER NOT NULL DEFAULT 0,
    success_rate DECIMAL(5,2) NOT NULL DEFAULT 0.00, -- Percentage with 2 decimal places
    avg_response_time_ms INTEGER NOT NULL DEFAULT 0, -- Average response time in milliseconds
    min_response_time_ms INTEGER NOT NULL DEFAULT 0, -- Minimum response time in milliseconds
    max_response_time_ms INTEGER NOT NULL DEFAULT 0, -- Maximum response time in milliseconds
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_aggregated_health_checks_monitor_id ON aggregated_health_checks(monitor_id, hour_timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_aggregated_health_checks_hour_timestamp ON aggregated_health_checks(hour_timestamp);

-- Create unique constraint to prevent duplicate aggregations for the same monitor and hour
CREATE UNIQUE INDEX IF NOT EXISTS idx_aggregated_health_checks_unique ON aggregated_health_checks(monitor_id, hour_timestamp);