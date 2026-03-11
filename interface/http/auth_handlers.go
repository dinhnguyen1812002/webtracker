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

// AuthHandler handles HTTP requests for authentication pages
type AuthHandler struct {
	authService *usecase.AuthService
}

// NewAuthHandler creates a new authentication handler
func NewAuthHandler(authService *usecase.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

// ShowLoginForm handles GET /login — renders the login page
func (h *AuthHandler) ShowLoginForm(c *gin.Context) {
	// If user is already logged in, redirect to dashboard
	if _, exists := c.Get("user"); exists {
		c.Redirect(http.StatusSeeOther, "/")
		return
	}

	// Generate a temporary CSRF token for the login form.
	// This is stored in a short-lived session so we can validate it on POST.
	csrfToken, err := domain.GenerateCSRFToken()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "", gin.H{"error": "Internal server error"})
		return
	}

	// Store CSRF token in a temporary cookie (validated on POST /login)
	c.SetCookie("csrf_token", csrfToken, 600, "/", "", false, true)

	data := templates.LoginFormData{
		CSRFToken: csrfToken,
	}

	component := templates.LoginForm(data)
	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = component.Render(c.Request.Context(), c.Writer)
}

// Login handles POST /login — authenticates the user
func (h *AuthHandler) Login(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Validate CSRF token
	formCSRF := c.PostForm("csrf_token")
	cookieCSRF, err := c.Cookie("csrf_token")
	if err != nil || formCSRF == "" || formCSRF != cookieCSRF {
		h.renderLoginError(c, "", "Invalid request. Please try again.")
		return
	}

	email := c.PostForm("email")
	password := c.PostForm("password")

	if email == "" || password == "" {
		h.renderLoginError(c, email, "Email and password are required.")
		return
	}

	// Authenticate user
	session, err := h.authService.Login(ctx, email, password)
	if err != nil {
		// Use generic error to prevent email enumeration
		h.renderLoginError(c, email, "Invalid email or password.")
		return
	}

	// Set session cookie
	// HttpOnly prevents JavaScript access (XSS protection)
	// SameSite=Lax prevents CSRF on cross-origin requests
	maxAge := int(h.authService.GetSessionTTL().Seconds())
	c.SetCookie("session_id", session.ID, maxAge, "/", "", false, true)

	// Clear the temporary CSRF cookie
	c.SetCookie("csrf_token", "", -1, "/", "", false, true)

	c.Redirect(http.StatusSeeOther, "/")
}

// ShowRegisterForm handles GET /register — renders the registration page
func (h *AuthHandler) ShowRegisterForm(c *gin.Context) {
	// If user is already logged in, redirect to dashboard
	if _, exists := c.Get("user"); exists {
		c.Redirect(http.StatusSeeOther, "/")
		return
	}

	csrfToken, err := domain.GenerateCSRFToken()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "", gin.H{"error": "Internal server error"})
		return
	}

	c.SetCookie("csrf_token", csrfToken, 600, "/", "", false, true)

	data := templates.RegisterFormData{
		CSRFToken: csrfToken,
		Errors:    make(map[string]string),
	}

	component := templates.RegisterForm(data)
	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = component.Render(c.Request.Context(), c.Writer)
}

// Register handles POST /register — creates a new user account
func (h *AuthHandler) Register(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Validate CSRF token
	formCSRF := c.PostForm("csrf_token")
	cookieCSRF, err := c.Cookie("csrf_token")
	if err != nil || formCSRF == "" || formCSRF != cookieCSRF {
		h.renderRegisterError(c, "", "", map[string]string{
			"general": "Invalid request. Please try again.",
		})
		return
	}

	name := c.PostForm("name")
	email := c.PostForm("email")
	password := c.PostForm("password")
	passwordConfirm := c.PostForm("password_confirm")

	// Validate form fields
	errors := make(map[string]string)

	if name == "" {
		errors["name"] = "Name is required"
	}
	if email == "" {
		errors["email"] = "Email is required"
	}
	if password == "" {
		errors["password"] = "Password is required"
	} else if len(password) < 8 {
		errors["password"] = "Password must be at least 8 characters"
	}
	if passwordConfirm == "" {
		errors["password_confirm"] = "Please confirm your password"
	} else if password != passwordConfirm {
		errors["password_confirm"] = "Passwords do not match"
	}

	if len(errors) > 0 {
		h.renderRegisterError(c, name, email, errors)
		return
	}

	// Create user
	_, err = h.authService.Register(ctx, email, password, name)
	if err != nil {
		errors["general"] = err.Error()
		h.renderRegisterError(c, name, email, errors)
		return
	}

	// Auto-login after registration
	session, err := h.authService.Login(ctx, email, password)
	if err != nil {
		// Registration succeeded but auto-login failed — redirect to login
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}

	maxAge := int(h.authService.GetSessionTTL().Seconds())
	c.SetCookie("session_id", session.ID, maxAge, "/", "", false, true)
	c.SetCookie("csrf_token", "", -1, "/", "", false, true)

	c.Redirect(http.StatusSeeOther, "/")
}

// Logout handles POST /logout — destroys the session
func (h *AuthHandler) Logout(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	sessionID, err := c.Cookie("session_id")
	if err == nil && sessionID != "" {
		_ = h.authService.Logout(ctx, sessionID)
	}

	// Clear session cookie
	c.SetCookie("session_id", "", -1, "/", "", false, true)

	c.Redirect(http.StatusSeeOther, "/login")
}

// renderLoginError re-renders the login form with an error message
func (h *AuthHandler) renderLoginError(c *gin.Context, email, errorMsg string) {
	csrfToken, _ := domain.GenerateCSRFToken()
	c.SetCookie("csrf_token", csrfToken, 600, "/", "", false, true)

	data := templates.LoginFormData{
		Email:     email,
		Error:     errorMsg,
		CSRFToken: csrfToken,
	}

	component := templates.LoginForm(data)
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Status(http.StatusUnauthorized)
	_ = component.Render(c.Request.Context(), c.Writer)
}

// renderRegisterError re-renders the register form with errors
func (h *AuthHandler) renderRegisterError(c *gin.Context, name, email string, errors map[string]string) {
	csrfToken, _ := domain.GenerateCSRFToken()
	c.SetCookie("csrf_token", csrfToken, 600, "/", "", false, true)

	data := templates.RegisterFormData{
		Name:      name,
		Email:     email,
		Errors:    errors,
		CSRFToken: csrfToken,
	}

	component := templates.RegisterForm(data)
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Status(http.StatusBadRequest)
	_ = component.Render(c.Request.Context(), c.Writer)
}
