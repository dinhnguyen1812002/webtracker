package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"web-tracker/domain"
	"web-tracker/usecase"
)

// AuthMiddleware provides authentication and CSRF middleware
type AuthMiddleware struct {
	authService *usecase.AuthService
}

// NewAuthMiddleware creates a new auth middleware instance
func NewAuthMiddleware(authService *usecase.AuthService) *AuthMiddleware {
	return &AuthMiddleware{
		authService: authService,
	}
}

// LoadUser is a Gin middleware that loads the user into context if a valid
// session cookie is present. It does not enforce authentication.
func (m *AuthMiddleware) LoadUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		if m == nil || m.authService == nil {
			c.Next()
			return
		}

		sessionID, err := c.Cookie("session_id")
		if err != nil || sessionID == "" {
			c.Next()
			return
		}

		user, session, err := m.authService.ValidateSession(c.Request.Context(), sessionID)
		if err != nil || user == nil {
			// Clear invalid session cookie to avoid repeated lookups
			c.SetCookie("session_id", "", -1, "/", "", false, true)
			c.Next()
			return
		}

		c.Set("user", user)
		c.Set("session", session)
		c.Next()
	}
}

// RequireAuth is a Gin middleware that enforces authentication.
// It reads the session_id cookie, validates the session, and loads the user
// into the Gin context. If the session is invalid, it redirects to /login.
func (m *AuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID, err := c.Cookie("session_id")
		if err != nil || sessionID == "" {
			c.Redirect(http.StatusSeeOther, "/login")
			c.Abort()
			return
		}

		user, session, err := m.authService.ValidateSession(c.Request.Context(), sessionID)
		if err != nil || user == nil {
			// Clear invalid session cookie
			c.SetCookie("session_id", "", -1, "/", "", false, true)
			c.Redirect(http.StatusSeeOther, "/login")
			c.Abort()
			return
		}

		// Store user and session in context for downstream handlers
		c.Set("user", user)
		c.Set("session", session)
		c.Next()
	}
}

// CSRFProtection is a Gin middleware that validates CSRF tokens on
// state-changing requests (POST, PUT, DELETE).
// It compares the csrf_token form field against the session's CSRF token.
func (m *AuthMiddleware) CSRFProtection() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only validate on state-changing methods
		if c.Request.Method == "GET" || c.Request.Method == "HEAD" || c.Request.Method == "OPTIONS" {
			c.Next()
			return
		}

		// Skip CSRF check for logout (uses session cookie auth only)
		if c.Request.URL.Path == "/logout" {
			c.Next()
			return
		}

		sessionVal, exists := c.Get("session")
		if !exists {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		session, ok := sessionVal.(*domain.Session)
		if !ok {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		formCSRF := c.PostForm("csrf_token")
		if formCSRF == "" || formCSRF != session.CSRFToken {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		c.Next()
	}
}

// GetUserFromContext extracts the authenticated user from the Gin context.
// Returns nil if no user is present (unauthenticated request).
func GetUserFromContext(c *gin.Context) *domain.User {
	userVal, exists := c.Get("user")
	if !exists {
		return nil
	}
	user, ok := userVal.(*domain.User)
	if !ok {
		return nil
	}
	return user
}

// GetSessionFromContext extracts the session from the Gin context.
func GetSessionFromContext(c *gin.Context) *domain.Session {
	sessionVal, exists := c.Get("session")
	if !exists {
		return nil
	}
	session, ok := sessionVal.(*domain.Session)
	if !ok {
		return nil
	}
	return session
}
