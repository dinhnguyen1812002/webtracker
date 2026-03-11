package domain

import (
	"errors"
	"regexp"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// User represents a registered user of the system
type User struct {
	ID           string
	Email        string
	PasswordHash string
	Name         string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// bcrypt cost factor — 12 provides a good balance between security and performance.
// Higher values exponentially increase hashing time, making brute-force attacks harder.
const bcryptCost = 12

// emailRegex validates email format (simplified but practical)
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// Validate validates user fields (excluding password, which is already hashed)
func (u *User) Validate() error {
	if u.Name == "" {
		return errors.New("name is required")
	}
	if len(u.Name) > 255 {
		return errors.New("name must be 255 characters or less")
	}
	if err := ValidateEmail(u.Email); err != nil {
		return err
	}
	if u.PasswordHash == "" {
		return errors.New("password hash is required")
	}
	return nil
}

// SetPassword hashes and stores the given plaintext password using bcrypt
func (u *User) SetPassword(plaintext string) error {
	if err := ValidatePassword(plaintext); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcryptCost)
	if err != nil {
		return errors.New("failed to hash password")
	}
	u.PasswordHash = string(hash)
	return nil
}

// CheckPassword compares the given plaintext password against the stored hash.
// Uses bcrypt's constant-time comparison to prevent timing attacks.
func (u *User) CheckPassword(plaintext string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(plaintext))
	return err == nil
}

// ValidateEmail checks that the email is non-empty and matches a valid format
func ValidateEmail(email string) error {
	if email == "" {
		return errors.New("email is required")
	}
	if len(email) > 255 {
		return errors.New("email must be 255 characters or less")
	}
	if !emailRegex.MatchString(email) {
		return errors.New("invalid email format")
	}
	return nil
}

// ValidatePassword checks password strength requirements:
// minimum 8 characters
func ValidatePassword(password string) error {
	if password == "" {
		return errors.New("password is required")
	}
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	if len(password) > 72 {
		// bcrypt has a 72-byte input limit
		return errors.New("password must be 72 characters or less")
	}
	return nil
}
