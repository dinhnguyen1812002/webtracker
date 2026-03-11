package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"web-tracker/domain"

	"github.com/google/uuid"
)

// AuthService handles user authentication workflows
type AuthService struct {
	userRepo    domain.UserRepository
	sessionRepo domain.SessionRepository
	sessionTTL  time.Duration
}

// NewAuthService creates a new authentication service
func NewAuthService(
	userRepo domain.UserRepository,
	sessionRepo domain.SessionRepository,
	sessionTTL time.Duration,
) *AuthService {
	if sessionTTL <= 0 {
		sessionTTL = 24 * time.Hour // Default: 24 hours
	}
	return &AuthService{
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
		sessionTTL:  sessionTTL,
	}
}

// Register creates a new user account.
// Returns the created user on success.
func (s *AuthService) Register(ctx context.Context, email, password, name string) (*domain.User, error) {
	// Normalize email to lowercase
	email = strings.ToLower(strings.TrimSpace(email))
	name = strings.TrimSpace(name)

	// Validate fields
	if err := domain.ValidateEmail(email); err != nil {
		return nil, err
	}
	if err := domain.ValidatePassword(password); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, errors.New("name is required")
	}

	// Check if email already exists
	existing, _ := s.userRepo.GetByEmail(ctx, email)
	if existing != nil {
		return nil, errors.New("email already registered")
	}

	// Create user
	user := &domain.User{
		ID:    uuid.New().String(),
		Email: email,
		Name:  name,
	}

	if err := user.SetPassword(password); err != nil {
		return nil, fmt.Errorf("failed to set password: %w", err)
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// Login authenticates a user and creates a new session.
// Returns the session on success. Uses constant-time password comparison
// to prevent timing attacks.
func (s *AuthService) Login(ctx context.Context, email, password string) (*domain.Session, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	// Look up user by email
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		// Return generic error to prevent email enumeration
		return nil, errors.New("invalid email or password")
	}

	// Verify password (bcrypt constant-time comparison)
	if !user.CheckPassword(password) {
		return nil, errors.New("invalid email or password")
	}

	// Create session
	session, err := s.createSession(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return session, nil
}

// Logout destroys the given session
func (s *AuthService) Logout(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return nil // Nothing to do
	}
	return s.sessionRepo.DeleteByID(ctx, sessionID)
}

// ValidateSession checks if a session is valid and returns the associated user.
// Returns nil user if the session is invalid or expired.
func (s *AuthService) ValidateSession(ctx context.Context, sessionID string) (*domain.User, *domain.Session, error) {
	if sessionID == "" {
		return nil, nil, errors.New("session ID is required")
	}

	// Get session
	session, err := s.sessionRepo.GetByID(ctx, sessionID)
	if err != nil {
		return nil, nil, errors.New("invalid session")
	}

	// Check expiry
	if session.IsExpired() {
		// Clean up expired session
		_ = s.sessionRepo.DeleteByID(ctx, sessionID)
		return nil, nil, errors.New("session expired")
	}

	// Get user
	user, err := s.userRepo.GetByID(ctx, session.UserID)
	if err != nil {
		// User was deleted but session still exists — clean up
		_ = s.sessionRepo.DeleteByID(ctx, sessionID)
		return nil, nil, errors.New("user not found")
	}

	return user, session, nil
}

// CleanupExpiredSessions removes all expired sessions from the database
func (s *AuthService) CleanupExpiredSessions(ctx context.Context) error {
	return s.sessionRepo.DeleteExpired(ctx)
}

// GetSessionTTL returns the configured session time-to-live
func (s *AuthService) GetSessionTTL() time.Duration {
	return s.sessionTTL
}

// createSession generates a new session with cryptographically random tokens
func (s *AuthService) createSession(ctx context.Context, userID string) (*domain.Session, error) {
	sessionToken, err := domain.GenerateSessionToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate session token: %w", err)
	}

	csrfToken, err := domain.GenerateCSRFToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate CSRF token: %w", err)
	}

	session := &domain.Session{
		ID:        sessionToken,
		UserID:    userID,
		CSRFToken: csrfToken,
		ExpiresAt: time.Now().Add(s.sessionTTL),
		CreatedAt: time.Now(),
	}

	if err := s.sessionRepo.Create(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to store session: %w", err)
	}

	return session, nil
}
