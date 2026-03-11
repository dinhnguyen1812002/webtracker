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

// SessionRepository implements domain.SessionRepository using PostgreSQL
type SessionRepository struct {
	pool *pgxpool.Pool
}

// NewSessionRepository creates a new PostgreSQL session repository
func NewSessionRepository(pool *pgxpool.Pool) *SessionRepository {
	return &SessionRepository{
		pool: pool,
	}
}

// Create inserts a new session into the database
func (r *SessionRepository) Create(ctx context.Context, session *domain.Session) error {
	if session == nil {
		return errors.New("session cannot be nil")
	}

	if session.CreatedAt.IsZero() {
		session.CreatedAt = time.Now()
	}

	query := `
		INSERT INTO sessions (id, user_id, csrf_token, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err := r.pool.Exec(ctx, query,
		session.ID,
		session.UserID,
		session.CSRFToken,
		session.ExpiresAt,
		session.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	return nil
}

// GetByID retrieves a session by its token ID
func (r *SessionRepository) GetByID(ctx context.Context, id string) (*domain.Session, error) {
	if id == "" {
		return nil, errors.New("session ID is required")
	}

	query := `
		SELECT id, user_id, csrf_token, expires_at, created_at
		FROM sessions
		WHERE id = $1
	`

	var session domain.Session
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&session.ID,
		&session.UserID,
		&session.CSRFToken,
		&session.ExpiresAt,
		&session.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("session not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return &session, nil
}

// DeleteByID removes a specific session (used for logout)
func (r *SessionRepository) DeleteByID(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("session ID is required")
	}

	query := `DELETE FROM sessions WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	return nil
}

// DeleteByUserID removes all sessions for a given user (force logout all devices)
func (r *SessionRepository) DeleteByUserID(ctx context.Context, userID string) error {
	if userID == "" {
		return errors.New("user ID is required")
	}

	query := `DELETE FROM sessions WHERE user_id = $1`
	_, err := r.pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user sessions: %w", err)
	}

	return nil
}

// DeleteExpired removes all sessions that have passed their expiry time.
// Should be called periodically (e.g., every hour) to keep the sessions table clean.
func (r *SessionRepository) DeleteExpired(ctx context.Context) error {
	query := `DELETE FROM sessions WHERE expires_at < $1`
	_, err := r.pool.Exec(ctx, query, time.Now())
	if err != nil {
		return fmt.Errorf("failed to delete expired sessions: %w", err)
	}

	return nil
}
