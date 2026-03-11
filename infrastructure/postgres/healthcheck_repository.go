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

// HealthCheckRepository implements domain.HealthCheckRepository using PostgreSQL
type HealthCheckRepository struct {
	pool *pgxpool.Pool
}

// NewHealthCheckRepository creates a new PostgreSQL health check repository
func NewHealthCheckRepository(pool *pgxpool.Pool) *HealthCheckRepository {
	return &HealthCheckRepository{
		pool: pool,
	}
}

// Create inserts a new health check into the database
func (r *HealthCheckRepository) Create(ctx context.Context, check *domain.HealthCheck) error {
	if check == nil {
		return errors.New("health check cannot be nil")
	}

	if check.ID == "" {
		return errors.New("health check ID is required")
	}

	if check.MonitorID == "" {
		return errors.New("monitor ID is required")
	}

	query := `
		INSERT INTO health_checks (
			id, monitor_id, status, status_code, response_time_ms,
			ssl_valid, ssl_expires_at, ssl_days_until, ssl_issuer,
			error_message, checked_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	// Set checked_at to now if not set
	if check.CheckedAt.IsZero() {
		check.CheckedAt = time.Now()
	}

	// Convert response time to milliseconds
	responseTimeMs := int(check.ResponseTime.Milliseconds())

	// Extract SSL info if present
	var sslValid *bool
	var sslExpiresAt *time.Time
	var sslDaysUntil *int
	var sslIssuer *string

	if check.SSLInfo != nil {
		sslValid = &check.SSLInfo.Valid
		sslExpiresAt = &check.SSLInfo.ExpiresAt
		sslDaysUntil = &check.SSLInfo.DaysUntil
		sslIssuer = &check.SSLInfo.Issuer
	}

	_, err := r.pool.Exec(ctx, query,
		check.ID,
		check.MonitorID,
		check.Status,
		check.StatusCode,
		responseTimeMs,
		sslValid,
		sslExpiresAt,
		sslDaysUntil,
		sslIssuer,
		check.ErrorMessage,
		check.CheckedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create health check: %w", err)
	}

	return nil
}

// GetByMonitorID retrieves health checks for a specific monitor with a limit
func (r *HealthCheckRepository) GetByMonitorID(ctx context.Context, monitorID string, limit int) ([]*domain.HealthCheck, error) {
	if monitorID == "" {
		return nil, errors.New("monitor ID is required")
	}

	if limit <= 0 {
		limit = 100 // Default limit
	}

	query := `
		SELECT 
			id, monitor_id, status, status_code, response_time_ms,
			ssl_valid, ssl_expires_at, ssl_days_until, ssl_issuer,
			error_message, checked_at
		FROM health_checks
		WHERE monitor_id = $1
		ORDER BY checked_at DESC
		LIMIT $2
	`

	rows, err := r.pool.Query(ctx, query, monitorID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query health checks: %w", err)
	}
	defer rows.Close()

	return r.scanHealthChecks(rows)
}

// GetByDateRange retrieves health checks for a monitor within a date range
func (r *HealthCheckRepository) GetByDateRange(ctx context.Context, monitorID string, start, end time.Time) ([]*domain.HealthCheck, error) {
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
			id, monitor_id, status, status_code, response_time_ms,
			ssl_valid, ssl_expires_at, ssl_days_until, ssl_issuer,
			error_message, checked_at
		FROM health_checks
		WHERE monitor_id = $1 AND checked_at >= $2 AND checked_at <= $3
		ORDER BY checked_at DESC
	`

	rows, err := r.pool.Query(ctx, query, monitorID, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to query health checks by date range: %w", err)
	}
	defer rows.Close()

	return r.scanHealthChecks(rows)
}

// DeleteOlderThan deletes health checks older than the specified time
func (r *HealthCheckRepository) DeleteOlderThan(ctx context.Context, before time.Time) error {
	if before.IsZero() {
		return errors.New("before time is required")
	}

	query := `DELETE FROM health_checks WHERE checked_at < $1`

	result, err := r.pool.Exec(ctx, query, before)
	if err != nil {
		return fmt.Errorf("failed to delete old health checks: %w", err)
	}

	// Log the number of deleted rows (optional, but useful for monitoring)
	_ = result.RowsAffected()

	return nil
}

// scanHealthChecks is a helper function to scan rows into HealthCheck structs
func (r *HealthCheckRepository) scanHealthChecks(rows pgx.Rows) ([]*domain.HealthCheck, error) {
	checks := []*domain.HealthCheck{}

	for rows.Next() {
		var check domain.HealthCheck
		var responseTimeMs int
		var sslValid *bool
		var sslExpiresAt *time.Time
		var sslDaysUntil *int
		var sslIssuer *string

		err := rows.Scan(
			&check.ID,
			&check.MonitorID,
			&check.Status,
			&check.StatusCode,
			&responseTimeMs,
			&sslValid,
			&sslExpiresAt,
			&sslDaysUntil,
			&sslIssuer,
			&check.ErrorMessage,
			&check.CheckedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan health check: %w", err)
		}

		// Convert milliseconds back to duration
		check.ResponseTime = time.Duration(responseTimeMs) * time.Millisecond

		// Reconstruct SSL info if present
		if sslValid != nil {
			check.SSLInfo = &domain.SSLInfo{
				Valid:     *sslValid,
				ExpiresAt: *sslExpiresAt,
				DaysUntil: *sslDaysUntil,
				Issuer:    *sslIssuer,
			}
		}

		checks = append(checks, &check)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating health checks: %w", err)
	}

	return checks, nil
}
