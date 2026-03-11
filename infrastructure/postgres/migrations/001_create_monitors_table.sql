-- Create monitors table
CREATE TABLE IF NOT EXISTS monitors (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    url TEXT NOT NULL,
    check_interval INTEGER NOT NULL, -- in seconds
    enabled BOOLEAN DEFAULT true,
    alert_channels JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create index on enabled column for filtering
CREATE INDEX IF NOT EXISTS idx_monitors_enabled ON monitors(enabled);

-- Create index on created_at for ordering
CREATE INDEX IF NOT EXISTS idx_monitors_created_at ON monitors(created_at DESC);
