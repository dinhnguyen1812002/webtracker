-- Create alerts table
CREATE TABLE IF NOT EXISTS alerts (
    id VARCHAR(255) PRIMARY KEY,
    monitor_id VARCHAR(255) NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    severity VARCHAR(50) NOT NULL,
    message TEXT NOT NULL,
    details JSONB,
    channels JSONB NOT NULL,
    sent_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create index on monitor_id and sent_at for efficient queries
CREATE INDEX IF NOT EXISTS idx_alerts_monitor_id ON alerts(monitor_id, sent_at DESC);

-- Create index on sent_at for date range queries
CREATE INDEX IF NOT EXISTS idx_alerts_sent_at ON alerts(sent_at);

-- Create index on monitor_id, type, and sent_at for GetLastAlertTime queries
CREATE INDEX IF NOT EXISTS idx_alerts_type ON alerts(monitor_id, type, sent_at DESC);
