package postgres

import (
	"context"
	"fmt"
	"time"
	"web-tracker/infrastructure/logger"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PoolConfig holds configuration for the PostgreSQL connection pool
type PoolConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	Database        string
	SSLMode         string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
}

// PoolStats holds connection pool statistics
type PoolStats struct {
	AcquireCount         int64 `json:"acquire_count"`
	AcquireDuration      int64 `json:"acquire_duration_ns"`
	AcquiredConns        int32 `json:"acquired_conns"`
	CanceledAcquireCount int64 `json:"canceled_acquire_count"`
	ConstructingConns    int32 `json:"constructing_conns"`
	EmptyAcquireCount    int64 `json:"empty_acquire_count"`
	IdleConns            int32 `json:"idle_conns"`
	MaxConns             int32 `json:"max_conns"`
	TotalConns           int32 `json:"total_conns"`
}

// DefaultPoolConfig returns a default pool configuration
// Optimized for memory efficiency and performance
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		Host:            "localhost",
		Port:            5432,
		User:            "postgres",
		Password:        "postgres",
		Database:        "uptime",
		SSLMode:         "disable",
		MaxConns:        20,               // Requirement 12.3: Max 20 connections
		MinConns:        2,                // Keep minimum connections for responsiveness
		MaxConnLifetime: 30 * time.Minute, // Reduced from 1 hour to prevent stale connections
		MaxConnIdleTime: 15 * time.Minute, // Reduced from 30 minutes to free idle connections sooner
	}
}

// NewPool creates a new PostgreSQL connection pool
func NewPool(ctx context.Context, config PoolConfig) (*pgxpool.Pool, error) {
	sslMode := config.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}

	connString := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		config.Host,
		config.Port,
		config.User,
		config.Password,
		config.Database,
		sslMode,
	)

	poolConfig, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pool config: %w", err)
	}

	// Configure pool settings
	poolConfig.MaxConns = config.MaxConns
	poolConfig.MinConns = config.MinConns
	poolConfig.MaxConnLifetime = config.MaxConnLifetime
	poolConfig.MaxConnIdleTime = config.MaxConnIdleTime

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create pool: %w", err)
	}

	// Test the connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return pool, nil
}

// GetPoolStats returns current connection pool statistics
func GetPoolStats(pool *pgxpool.Pool) PoolStats {
	stat := pool.Stat()
	return PoolStats{
		AcquireCount:         stat.AcquireCount(),
		AcquireDuration:      stat.AcquireDuration().Nanoseconds(),
		AcquiredConns:        stat.AcquiredConns(),
		CanceledAcquireCount: stat.CanceledAcquireCount(),
		ConstructingConns:    stat.ConstructingConns(),
		EmptyAcquireCount:    stat.EmptyAcquireCount(),
		IdleConns:            stat.IdleConns(),
		MaxConns:             stat.MaxConns(),
		TotalConns:           stat.TotalConns(),
	}
}

// LogPoolStats logs current connection pool statistics
func LogPoolStats(pool *pgxpool.Pool, context string) {
	stats := GetPoolStats(pool)
	log := logger.GetLogger()

	log.Info("Database connection pool statistics", logger.Fields{
		"context":                 context,
		"acquired_conns":          stats.AcquiredConns,
		"idle_conns":              stats.IdleConns,
		"total_conns":             stats.TotalConns,
		"max_conns":               stats.MaxConns,
		"constructing_conns":      stats.ConstructingConns,
		"acquire_count":           stats.AcquireCount,
		"avg_acquire_duration_ms": stats.AcquireDuration / 1000000, // Convert to milliseconds
		"canceled_acquire_count":  stats.CanceledAcquireCount,
		"empty_acquire_count":     stats.EmptyAcquireCount,
	})
}

// StartPoolMonitoring starts background monitoring of connection pool statistics
func StartPoolMonitoring(ctx context.Context, pool *pgxpool.Pool, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		log := logger.GetLogger()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				stats := GetPoolStats(pool)

				// Log warning if connection pool utilization is high
				utilization := float64(stats.AcquiredConns) / float64(stats.MaxConns) * 100
				if utilization > 80 {
					log.Warn("High database connection pool utilization", logger.Fields{
						"utilization_percent": utilization,
						"acquired_conns":      stats.AcquiredConns,
						"max_conns":           stats.MaxConns,
					})
				}

				// Log periodic stats (every 10 intervals to reduce noise)
				if time.Now().Unix()%10 == 0 {
					LogPoolStats(pool, "periodic_monitoring")
				}
			}
		}
	}()
}

// ParseDatabaseURL parses a PostgreSQL database URL into a PoolConfig
func ParseDatabaseURL(dbURL string) (*PoolConfig, error) {
	poolConfig, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	config := DefaultPoolConfig()
	config.Host = poolConfig.ConnConfig.Host
	config.Port = int(poolConfig.ConnConfig.Port)
	config.User = poolConfig.ConnConfig.User
	config.Password = poolConfig.ConnConfig.Password
	config.Database = poolConfig.ConnConfig.Database

	// Extract SSL mode from connection string if present
	if poolConfig.ConnConfig.TLSConfig != nil {
		config.SSLMode = "require"
	} else {
		config.SSLMode = "disable"
	}

	return &config, nil
}
