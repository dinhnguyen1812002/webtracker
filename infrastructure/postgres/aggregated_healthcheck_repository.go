package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"web-tracker/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AggregatedHealthCheckRepository implements domain.AggregatedHealthCheckRepository using PostgreSQL
type AggregatedHealthCheckRepository struct {
	pool *pgxpool.Pool
}

// NewAggregatedHealthCheckRepository creates a new PostgreSQL aggregated health check repository
func NewAggregatedHealthCheckRepository(pool *pgxpool.Pool) *AggregatedHealthCheckRepository {
	return &AggregatedHealthCheckRepository{
		pool: pool,
	}
}

// Create inserts a new aggregated health check into the database
func (r *AggregatedHealthCheckRepository) Create(ctx context.Context, aggregated *domain.AggregatedHealthCheck) error {
	if aggregated == nil {
		return errors.New("aggregated health check cannot be nil")
	}

	if aggregated.ID == "" {
		return errors.New("aggregated health check ID is required")
	}

	if aggregated.MonitorID == "" {
		return errors.New("monitor ID is required")
	}

	query := `
		INSERT INTO aggregated_health_checks (
			id, monitor_id, hour_timestamp, total_checks, successful_checks, failed_checks,
			success_rate, avg_response_time_ms, min_response_time_ms, max_response_time_ms, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (monitor_id, hour_timestamp) 
		DO UPDATE SET
			total_checks = EXCLUDED.total_checks,
			successful_checks = EXCLUDED.successful_checks,
			failed_checks = EXCLUDED.failed_checks,
			success_rate = EXCLUDED.success_rate,
			avg_response_time_ms = EXCLUDED.avg_response_time_ms,
			min_response_time_ms = EXCLUDED.min_response_time_ms,
			max_response_time_ms = EXCLUDED.max_response_time_ms,
			created_at = EXCLUDED.created_at
	`

	// Set created_at to now if not set
	if aggregated.CreatedAt.IsZero() {
		aggregated.CreatedAt = time.Now()
	}

	// Convert durations to milliseconds
	avgResponseTimeMs := int(aggregated.AvgResponseTime.Milliseconds())
	minResponseTimeMs := int(aggregated.MinResponseTime.Milliseconds())
	maxResponseTimeMs := int(aggregated.MaxResponseTime.Milliseconds())

	_, err := r.pool.Exec(ctx, query,
		aggregated.ID,
		aggregated.MonitorID,
		aggregated.HourTimestamp,
		aggregated.TotalChecks,
		aggregated.SuccessfulChecks,
		aggregated.FailedChecks,
		aggregated.SuccessRate,
		avgResponseTimeMs,
		minResponseTimeMs,
		maxResponseTimeMs,
		aggregated.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create aggregated health check: %w", err)
	}

	return nil
}

// GetByMonitorID retrieves aggregated health checks for a specific monitor with a limit
func (r *AggregatedHealthCheckRepository) GetByMonitorID(ctx context.Context, monitorID string, limit int) ([]*domain.AggregatedHealthCheck, error) {
	if monitorID == "" {
		return nil, errors.New("monitor ID is required")
	}

	if limit <= 0 {
		limit = 100 // Default limit
	}

	query := `
		SELECT 
			id, monitor_id, hour_timestamp, total_checks, successful_checks, failed_checks,
			success_rate, avg_response_time_ms, min_response_time_ms, max_response_time_ms, created_at
		FROM aggregated_health_checks
		WHERE monitor_id = $1
		ORDER BY hour_timestamp DESC
		LIMIT $2
	`

	rows, err := r.pool.Query(ctx, query, monitorID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query aggregated health checks: %w", err)
	}
	defer rows.Close()

	return r.scanAggregatedHealthChecks(rows)
}

// GetByDateRange retrieves aggregated health checks for a monitor within a date range
func (r *AggregatedHealthCheckRepository) GetByDateRange(ctx context.Context, monitorID string, start, end time.Time) ([]*domain.AggregatedHealthCheck, error) {
	if monitorID == "" {
		return nil, errors.New("monitor ID is required")
	}

	if start.IsZero() || end.IsZero() {
		return nil, errors.New("start and end times are required")
	}

	if start.After(end) {
		return nil, errors.New("start time must be before end time")
	}

	query := `
		SELECT 
			id, monitor_id, hour_timestamp, total_checks, successful_checks, failed_checks,
			success_rate, avg_response_time_ms, min_response_time_ms, max_response_time_ms, created_at
		FROM aggregated_health_checks
		WHERE monitor_id = $1 AND hour_timestamp >= $2 AND hour_timestamp <= $3
		ORDER BY hour_timestamp DESC
	`

	rows, err := r.pool.Query(ctx, query, monitorID, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to query aggregated health checks by date range: %w", err)
	}
	defer rows.Close()

	return r.scanAggregatedHealthChecks(rows)
}

// DeleteOlderThan deletes aggregated health checks older than the specified time
func (r *AggregatedHealthCheckRepository) DeleteOlderThan(ctx context.Context, before time.Time) error {
	if before.IsZero() {
		return errors.New("before time is required")
	}

	query := `DELETE FROM aggregated_health_checks WHERE hour_timestamp < $1`

	result, err := r.pool.Exec(ctx, query, before)
	if err != nil {
		return fmt.Errorf("failed to delete old aggregated health checks: %w", err)
	}

	// Log the number of deleted rows (optional, but useful for monitoring)
	_ = result.RowsAffected()

	return nil
}

// scanAggregatedHealthChecks is a helper function to scan rows into AggregatedHealthCheck structs
func (r *AggregatedHealthCheckRepository) scanAggregatedHealthChecks(rows pgx.Rows) ([]*domain.AggregatedHealthCheck, error) {
	aggregated := []*domain.AggregatedHealthCheck{}

	for rows.Next() {
		var agg domain.AggregatedHealthCheck
		var avgResponseTimeMs, minResponseTimeMs, maxResponseTimeMs int

		err := rows.Scan(
			&agg.ID,
			&agg.MonitorID,
			&agg.HourTimestamp,
			&agg.TotalChecks,
			&agg.SuccessfulChecks,
			&agg.FailedChecks,
			&agg.SuccessRate,
			&avgResponseTimeMs,
			&minResponseTimeMs,
			&maxResponseTimeMs,
			&agg.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan aggregated health check: %w", err)
		}

		// Convert milliseconds back to durations
		agg.AvgResponseTime = time.Duration(avgResponseTimeMs) * time.Millisecond
		agg.MinResponseTime = time.Duration(minResponseTimeMs) * time.Millisecond
		agg.MaxResponseTime = time.Duration(maxResponseTimeMs) * time.Millisecond

		aggregated = append(aggregated, &agg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating aggregated health checks: %w", err)
	}

	return aggregated, nil
}
