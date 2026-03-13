package http

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"web-tracker/domain"
	"web-tracker/interface/http/templates"
	"web-tracker/usecase"
)

const (
	csrfCookieTTL = 600 // seconds
	sessionCookie = "session_id"
	csrfCookie    = "csrf_token"
)

// AuthHandler handles HTTP requests for authentication pages.
type AuthHandler struct {
	authService *usecase.AuthService
}

// NewAuthHandler creates a new authentication handler.
func NewAuthHandler(authService *usecase.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

// ShowLoginForm handles GET /login — renders the login page.
func (h *AuthHandler) ShowLoginForm(c *gin.Context) {
	if _, exists := c.Get("user"); exists {
		c.Redirect(http.StatusSeeOther, "/")
		return
	}

	csrfToken, err := h.setCSRFCookie(c)
	if err != nil {
		c.String(http.StatusInternalServerError, "Internal server error")
		return
	}

	h.renderLogin(c, http.StatusOK, templates.LoginFormData{CSRFToken: csrfToken})
}

// Login handles POST /login — authenticates the user.
func (h *AuthHandler) Login(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	if !h.validateCSRF(c) {
		h.renderLoginError(c, "", "Invalid request. Please try again.")
		return
	}

	email := c.PostForm("email")
	password := c.PostForm("password")

	if email == "" || password == "" {
		h.renderLoginError(c, email, "Email and password are required.")
		return
	}

	if h.authService == nil {
		c.String(http.StatusInternalServerError, "Internal server error")
		return
	}

	session, err := h.authService.Login(ctx, email, password)
	if err != nil {
		// Generic message prevents email enumeration.
		h.renderLoginError(c, email, "Invalid email or password.")
		return
	}

	h.setSessionCookie(c, session.ID)
	h.clearCSRFCookie(c)
	c.Redirect(http.StatusSeeOther, "/")
}

// ShowRegisterForm handles GET /register — renders the registration page.
func (h *AuthHandler) ShowRegisterForm(c *gin.Context) {
	if _, exists := c.Get("user"); exists {
		c.Redirect(http.StatusSeeOther, "/")
		return
	}

	csrfToken, err := h.setCSRFCookie(c)
	if err != nil {
		c.String(http.StatusInternalServerError, "Internal server error")
		return
	}

	h.renderRegister(c, http.StatusOK, templates.RegisterFormData{
		CSRFToken: csrfToken,
		Errors:    make(map[string]string),
	})
}

// Register handles POST /register — creates a new user account.
func (h *AuthHandler) Register(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	if !h.validateCSRF(c) {
		h.renderRegisterError(c, "", "", map[string]string{
			"general": "Invalid request. Please try again.",
		})
		return
	}

	name := c.PostForm("name")
	email := c.PostForm("email")
	password := c.PostForm("password")
	passwordConfirm := c.PostForm("password_confirm")

	if errs := validateRegistration(name, email, password, passwordConfirm); len(errs) > 0 {
		h.renderRegisterError(c, name, email, errs)
		return
	}

	if h.authService == nil {
		c.String(http.StatusInternalServerError, "Internal server error")
		return
	}

	if _, err := h.authService.Register(ctx, email, password, name); err != nil {
		h.renderRegisterError(c, name, email, map[string]string{"general": err.Error()})
		return
	}

	// Auto-login after successful registration.
	session, err := h.authService.Login(ctx, email, password)
	if err != nil {
		// Registration succeeded but login failed — let the user log in manually.
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}

	h.setSessionCookie(c, session.ID)
	h.clearCSRFCookie(c)
	c.Redirect(http.StatusSeeOther, "/")
}

// Logout handles POST /logout — destroys the session.
func (h *AuthHandler) Logout(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	if sessionID, err := c.Cookie(sessionCookie); err == nil && sessionID != "" {
		if h.authService != nil {
			_ = h.authService.Logout(ctx, sessionID)
		}
	}

	h.clearSessionCookie(c)
	c.Redirect(http.StatusSeeOther, "/login")
}

// --- render helpers ---

func (h *AuthHandler) renderLogin(c *gin.Context, status int, data templates.LoginFormData) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Status(status)
	_ = templates.LoginForm(data).Render(c.Request.Context(), c.Writer)
}

func (h *AuthHandler) renderRegister(c *gin.Context, status int, data templates.RegisterFormData) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Status(status)
	_ = templates.RegisterForm(data).Render(c.Request.Context(), c.Writer)
}

func (h *AuthHandler) renderLoginError(c *gin.Context, email, msg string) {
	csrfToken, err := h.setCSRFCookie(c)
	if err != nil {
		c.String(http.StatusInternalServerError, "Internal server error")
		return
	}
	h.renderLogin(c, http.StatusUnauthorized, templates.LoginFormData{
		Email:     email,
		Error:     msg,
		CSRFToken: csrfToken,
	})
}

func (h *AuthHandler) renderRegisterError(c *gin.Context, name, email string, errors map[string]string) {
	csrfToken, err := h.setCSRFCookie(c)
	if err != nil {
		c.String(http.StatusInternalServerError, "Internal server error")
		return
	}
	h.renderRegister(c, http.StatusBadRequest, templates.RegisterFormData{
		Name:      name,
		Email:     email,
		Errors:    errors,
		CSRFToken: csrfToken,
	})
}

// --- CSRF helpers ---

// setCSRFCookie generates a new CSRF token, stores it in a cookie, and returns it.
// Returns an error if token generation fails — callers must not proceed on error.
func (h *AuthHandler) setCSRFCookie(c *gin.Context) (string, error) {
	token, err := domain.GenerateCSRFToken()
	if err != nil {
		return "", err
	}
	c.SetCookie(csrfCookie, token, csrfCookieTTL, "/", "", false, true)
	return token, nil
}

// validateCSRF checks that the form CSRF token matches the cookie.
func (h *AuthHandler) validateCSRF(c *gin.Context) bool {
	formToken := c.PostForm("csrf_token")
	cookieToken, err := c.Cookie(csrfCookie)
	return err == nil && formToken != "" && formToken == cookieToken
}

func (h *AuthHandler) clearCSRFCookie(c *gin.Context) {
	c.SetCookie(csrfCookie, "", -1, "/", "", false, true)
}

// --- session helpers ---

func (h *AuthHandler) setSessionCookie(c *gin.Context, sessionID string) {
	maxAge := 24 * 3600 // fallback to 24h
	if h.authService != nil {
		maxAge = int(h.authService.GetSessionTTL().Seconds())
	}
	c.SetCookie(sessionCookie, sessionID, maxAge, "/", "", false, true)
}

func (h *AuthHandler) clearSessionCookie(c *gin.Context) {
	c.SetCookie(sessionCookie, "", -1, "/", "", false, true)
}

// --- validation ---

// validateRegistration validates registration form fields and returns a map of field errors.
func validateRegistration(name, email, password, passwordConfirm string) map[string]string {
	errs := make(map[string]string)
	if name == "" {
		errs["name"] = "Name is required"
	}
	if email == "" {
		errs["email"] = "Email is required"
	}
	if password == "" {
		errs["password"] = "Password is required"
	} else if len(password) < 8 {
		errs["password"] = "Password must be at least 8 characters"
	}
	if passwordConfirm == "" {
		errs["password_confirm"] = "Please confirm your password"
	} else if password != passwordConfirm {
		errs["password_confirm"] = "Passwords do not match"
	}
	return errs
}
