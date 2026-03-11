package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"web-tracker/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AlertRepository implements domain.AlertRepository using PostgreSQL
type AlertRepository struct {
	pool *pgxpool.Pool
}

// NewAlertRepository creates a new PostgreSQL alert repository
func NewAlertRepository(pool *pgxpool.Pool) *AlertRepository {
	return &AlertRepository{
		pool: pool,
	}
}

// Create inserts a new alert into the database
func (r *AlertRepository) Create(ctx context.Context, alert *domain.Alert) error {
	if alert == nil {
		return errors.New("alert cannot be nil")
	}

	if alert.ID == "" {
		return errors.New("alert ID is required")
	}

	if alert.MonitorID == "" {
		return errors.New("monitor ID is required")
	}

	// Marshal details to JSON
	var detailsJSON []byte
	var err error
	if alert.Details != nil {
		detailsJSON, err = json.Marshal(alert.Details)
		if err != nil {
			return fmt.Errorf("failed to marshal details: %w", err)
		}
	}

	// Marshal channels to JSON
	channelsJSON, err := json.Marshal(alert.Channels)
	if err != nil {
		return fmt.Errorf("failed to marshal channels: %w", err)
	}

	query := `
		INSERT INTO alerts (id, monitor_id, type, severity, message, details, channels, sent_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	if alert.SentAt.IsZero() {
		alert.SentAt = time.Now()
	}

	_, err = r.pool.Exec(ctx, query,
		alert.ID,
		alert.MonitorID,
		string(alert.Type),
		string(alert.Severity),
		alert.Message,
		detailsJSON,
		channelsJSON,
		alert.SentAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create alert: %w", err)
	}

	return nil
}

// GetByMonitorID retrieves alerts for a specific monitor with a limit
func (r *AlertRepository) GetByMonitorID(ctx context.Context, monitorID string, limit int) ([]*domain.Alert, error) {
	if monitorID == "" {
		return nil, errors.New("monitor ID is required")
	}

	query := `
		SELECT id, monitor_id, type, severity, message, details, channels, sent_at
		FROM alerts
		WHERE monitor_id = $1
		ORDER BY sent_at DESC
	`

	args := []any{monitorID}

	if limit > 0 {
		query += " LIMIT $2"
		args = append(args, limit)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get alerts by monitor ID: %w", err)
	}
	defer rows.Close()

	return r.scanAlerts(rows)
}

// GetByDateRange retrieves alerts for a specific monitor within a date range
func (r *AlertRepository) GetByDateRange(ctx context.Context, monitorID string, start, end time.Time) ([]*domain.Alert, error) {
	if monitorID == "" {
		return nil, errors.New("monitor ID is required")
	}

	query := `
		SELECT id, monitor_id, type, severity, message, details, channels, sent_at
		FROM alerts
		WHERE monitor_id = $1 AND sent_at >= $2 AND sent_at <= $3
		ORDER BY sent_at DESC
	`

	rows, err := r.pool.Query(ctx, query, monitorID, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to get alerts by date range: %w", err)
	}
	defer rows.Close()

	return r.scanAlerts(rows)
}

// GetLastAlertTime retrieves the timestamp of the last alert for a specific monitor and alert type
func (r *AlertRepository) GetLastAlertTime(ctx context.Context, monitorID string, alertType domain.AlertType) (*time.Time, error) {
	if monitorID == "" {
		return nil, errors.New("monitor ID is required")
	}

	query := `
		SELECT sent_at
		FROM alerts
		WHERE monitor_id = $1 AND type = $2
		ORDER BY sent_at DESC
		LIMIT 1
	`

	var sentAt time.Time
	err := r.pool.QueryRow(ctx, query, monitorID, string(alertType)).Scan(&sentAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // No alert found, return nil without error
		}
		return nil, fmt.Errorf("failed to get last alert time: %w", err)
	}

	return &sentAt, nil
}

// DeleteOlderThan deletes alerts older than the specified time
func (r *AlertRepository) DeleteOlderThan(ctx context.Context, before time.Time) error {
	if before.IsZero() {
		return errors.New("before time is required")
	}

	query := `DELETE FROM alerts WHERE sent_at < $1`

	result, err := r.pool.Exec(ctx, query, before)
	if err != nil {
		return fmt.Errorf("failed to delete old alerts: %w", err)
	}

	// Log the number of deleted rows (optional, but useful for monitoring)
	_ = result.RowsAffected()

	return nil
}

// scanAlerts is a helper function to scan multiple alert rows
func (r *AlertRepository) scanAlerts(rows pgx.Rows) ([]*domain.Alert, error) {
	alerts := []*domain.Alert{}

	for rows.Next() {
		var alert domain.Alert
		var alertType string
		var severity string
		var detailsJSON []byte
		var channelsJSON []byte

		err := rows.Scan(
			&alert.ID,
			&alert.MonitorID,
			&alertType,
			&severity,
			&alert.Message,
			&detailsJSON,
			&channelsJSON,
			&alert.SentAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan alert: %w", err)
		}

		alert.Type = domain.AlertType(alertType)
		alert.Severity = domain.AlertSeverity(severity)

		// Unmarshal details if present
		if detailsJSON != nil {
			if err := json.Unmarshal(detailsJSON, &alert.Details); err != nil {
				return nil, fmt.Errorf("failed to unmarshal details: %w", err)
			}
		}

		// Unmarshal channels
		if err := json.Unmarshal(channelsJSON, &alert.Channels); err != nil {
			return nil, fmt.Errorf("failed to unmarshal channels: %w", err)
		}

		alerts = append(alerts, &alert)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating alerts: %w", err)
	}

	return alerts, nil
}
