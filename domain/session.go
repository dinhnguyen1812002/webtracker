package domain

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// Session represents an authenticated user session.
// Session IDs are cryptographically random tokens stored as cookies.
type Session struct {
	ID        string    // 64-char hex string (32 random bytes)
	UserID    string    // FK to users.id
	CSRFToken string    // 64-char hex string, validated on state-changing requests
	ExpiresAt time.Time // Session expiry timestamp
	CreatedAt time.Time
}

// IsExpired checks whether the session has passed its expiry time
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// GenerateSessionToken creates a cryptographically secure random token.
// Uses crypto/rand for security-critical randomness (not math/rand).
func GenerateSessionToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// GenerateCSRFToken creates a cryptographically secure CSRF token
func GenerateCSRFToken() (string, error) {
	return GenerateSessionToken() // Same generation logic
}
