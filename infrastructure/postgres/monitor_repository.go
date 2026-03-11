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

// MonitorRepository implements domain.MonitorRepository using PostgreSQL
type MonitorRepository struct {
	pool *pgxpool.Pool
}

// NewMonitorRepository creates a new PostgreSQL monitor repository
func NewMonitorRepository(pool *pgxpool.Pool) *MonitorRepository {
	return &MonitorRepository{
		pool: pool,
	}
}

// Create inserts a new monitor into the database
func (r *MonitorRepository) Create(ctx context.Context, monitor *domain.Monitor) error {
	if monitor == nil {
		return errors.New("monitor cannot be nil")
	}

	if err := monitor.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Marshal alert channels to JSON
	alertChannelsJSON, err := json.Marshal(monitor.AlertChannels)
	if err != nil {
		return fmt.Errorf("failed to marshal alert channels: %w", err)
	}

	query := `
		INSERT INTO monitors (id, name, url, check_interval, enabled, alert_channels, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	now := time.Now()
	if monitor.CreatedAt.IsZero() {
		monitor.CreatedAt = now
	}
	if monitor.UpdatedAt.IsZero() {
		monitor.UpdatedAt = now
	}

	_, err = r.pool.Exec(ctx, query,
		monitor.ID,
		monitor.Name,
		monitor.URL,
		int(monitor.CheckInterval.Seconds()),
		monitor.Enabled,
		alertChannelsJSON,
		monitor.CreatedAt,
		monitor.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create monitor: %w", err)
	}

	return nil
}

// GetByID retrieves a monitor by its ID
func (r *MonitorRepository) GetByID(ctx context.Context, id string) (*domain.Monitor, error) {
	if id == "" {
		return nil, errors.New("monitor ID is required")
	}

	query := `
		SELECT id, name, url, check_interval, enabled, alert_channels, created_at, updated_at
		FROM monitors
		WHERE id = $1
	`

	var monitor domain.Monitor
	var checkIntervalSeconds int
	var alertChannelsJSON []byte

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&monitor.ID,
		&monitor.Name,
		&monitor.URL,
		&checkIntervalSeconds,
		&monitor.Enabled,
		&alertChannelsJSON,
		&monitor.CreatedAt,
		&monitor.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("monitor not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get monitor: %w", err)
	}

	monitor.CheckInterval = time.Duration(checkIntervalSeconds) * time.Second

	// Unmarshal alert channels
	if err := json.Unmarshal(alertChannelsJSON, &monitor.AlertChannels); err != nil {
		return nil, fmt.Errorf("failed to unmarshal alert channels: %w", err)
	}

	return &monitor, nil
}

// List retrieves monitors with optional filters
func (r *MonitorRepository) List(ctx context.Context, filters domain.ListFilters) ([]*domain.Monitor, error) {
	query := `
		SELECT id, name, url, check_interval, enabled, alert_channels, created_at, updated_at
		FROM monitors
	`

	args := []interface{}{}
	argCount := 0

	// Add enabled filter if specified
	if filters.Enabled != nil {
		argCount++
		query += fmt.Sprintf(" WHERE enabled = $%d", argCount)
		args = append(args, *filters.Enabled)
	}

	// Add ordering
	query += " ORDER BY created_at DESC"

	// Add limit if specified
	if filters.Limit > 0 {
		argCount++
		query += fmt.Sprintf(" LIMIT $%d", argCount)
		args = append(args, filters.Limit)
	}

	// Add offset if specified
	if filters.Offset > 0 {
		argCount++
		query += fmt.Sprintf(" OFFSET $%d", argCount)
		args = append(args, filters.Offset)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list monitors: %w", err)
	}
	defer rows.Close()

	monitors := []*domain.Monitor{}
	for rows.Next() {
		var monitor domain.Monitor
		var checkIntervalSeconds int
		var alertChannelsJSON []byte

		err := rows.Scan(
			&monitor.ID,
			&monitor.Name,
			&monitor.URL,
			&checkIntervalSeconds,
			&monitor.Enabled,
			&alertChannelsJSON,
			&monitor.CreatedAt,
			&monitor.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan monitor: %w", err)
		}

		monitor.CheckInterval = time.Duration(checkIntervalSeconds) * time.Second

		// Unmarshal alert channels
		if err := json.Unmarshal(alertChannelsJSON, &monitor.AlertChannels); err != nil {
			return nil, fmt.Errorf("failed to unmarshal alert channels: %w", err)
		}

		monitors = append(monitors, &monitor)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating monitors: %w", err)
	}

	return monitors, nil
}

// Update updates an existing monitor
func (r *MonitorRepository) Update(ctx context.Context, monitor *domain.Monitor) error {
	if monitor == nil {
		return errors.New("monitor cannot be nil")
	}

	if monitor.ID == "" {
		return errors.New("monitor ID is required")
	}

	if err := monitor.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Marshal alert channels to JSON
	alertChannelsJSON, err := json.Marshal(monitor.AlertChannels)
	if err != nil {
		return fmt.Errorf("failed to marshal alert channels: %w", err)
	}

	query := `
		UPDATE monitors
		SET name = $2, url = $3, check_interval = $4, enabled = $5, alert_channels = $6, updated_at = $7
		WHERE id = $1
	`

	monitor.UpdatedAt = time.Now()

	result, err := r.pool.Exec(ctx, query,
		monitor.ID,
		monitor.Name,
		monitor.URL,
		int(monitor.CheckInterval.Seconds()),
		monitor.Enabled,
		alertChannelsJSON,
		monitor.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update monitor: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("monitor not found: %s", monitor.ID)
	}

	return nil
}

// Delete removes a monitor from the database
func (r *MonitorRepository) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("monitor ID is required")
	}

	query := `DELETE FROM monitors WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete monitor: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("monitor not found: %s", id)
	}

	return nil
}
