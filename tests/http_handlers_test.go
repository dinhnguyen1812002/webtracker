package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"web-tracker/domain"
	httppkg "web-tracker/interface/http"
	"web-tracker/usecase"
)

// MockMonitorService is a mock implementation of MonitorService
type MockMonitorService struct {
	mock.Mock
}

func (m *MockMonitorService) CreateMonitor(ctx context.Context, req usecase.CreateMonitorRequest) (*domain.Monitor, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*domain.Monitor), args.Error(1)
}

func (m *MockMonitorService) GetMonitor(ctx context.Context, id string) (*domain.Monitor, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Monitor), args.Error(1)
}

func (m *MockMonitorService) ListMonitors(ctx context.Context, filters domain.ListFilters) ([]*domain.Monitor, error) {
	args := m.Called(ctx, filters)
	return args.Get(0).([]*domain.Monitor), args.Error(1)
}

func (m *MockMonitorService) UpdateMonitor(ctx context.Context, id string, req usecase.UpdateMonitorRequest) (*domain.Monitor, error) {
	args := m.Called(ctx, id, req)
	return args.Get(0).(*domain.Monitor), args.Error(1)
}

func (m *MockMonitorService) DeleteMonitor(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func TestMonitorHandler_CreateMonitor(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mock service
	mockService := new(MockMonitorService)
	handler := httppkg.NewMonitorHandler(mockService)

	// Create test monitor
	testMonitor := &domain.Monitor{
		ID:            "test-id",
		Name:          "Test Monitor",
		URL:           "https://example.com",
		CheckInterval: 5 * time.Minute,
		Enabled:       true,
		AlertChannels: []domain.AlertChannel{},
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Setup mock expectation
	mockService.On("CreateMonitor", mock.Anything, mock.MatchedBy(func(req usecase.CreateMonitorRequest) bool {
		return req.Name == "Test Monitor" && req.URL == "https://example.com"
	})).Return(testMonitor, nil)

	// Create request
	requestBody := httppkg.CreateMonitorRequest{
		Name:          "Test Monitor",
		URL:           "https://example.com",
		CheckInterval: 5, // 5 minutes
		Enabled:       true,
		AlertChannels: []domain.AlertChannel{},
	}

	jsonBody, _ := json.Marshal(requestBody)

	// Create HTTP request
	req, _ := http.NewRequest("POST", "/api/v1/monitors", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	// Create response recorder
	w := httptest.NewRecorder()

	// Create Gin context
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	// Call handler
	handler.CreateMonitor(c)

	// Assert response
	assert.Equal(t, http.StatusCreated, w.Code)

	var response httppkg.MonitorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "test-id", response.ID)
	assert.Equal(t, "Test Monitor", response.Name)
	assert.Equal(t, "https://example.com", response.URL)
	assert.Equal(t, 5, response.CheckInterval)
	assert.True(t, response.Enabled)

	// Verify mock was called
	mockService.AssertExpectations(t)
}

func TestMonitorHandler_GetMonitor(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mock service
	mockService := new(MockMonitorService)
	handler := httppkg.NewMonitorHandler(mockService)

	// Create test monitor
	testMonitor := &domain.Monitor{
		ID:            "test-id",
		Name:          "Test Monitor",
		URL:           "https://example.com",
		CheckInterval: 5 * time.Minute,
		Enabled:       true,
		AlertChannels: []domain.AlertChannel{},
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Setup mock expectation
	mockService.On("GetMonitor", mock.Anything, "test-id").Return(testMonitor, nil)

	// Create HTTP request
	req, _ := http.NewRequest("GET", "/api/v1/monitors/test-id", nil)

	// Create response recorder
	w := httptest.NewRecorder()

	// Create Gin context
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{gin.Param{Key: "id", Value: "test-id"}}

	// Call handler
	handler.GetMonitor(c)

	// Assert response
	assert.Equal(t, http.StatusOK, w.Code)

	var response httppkg.MonitorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "test-id", response.ID)
	assert.Equal(t, "Test Monitor", response.Name)

	// Verify mock was called
	mockService.AssertExpectations(t)
}

func TestSystemHandler_Health(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create handler
	handler := httppkg.NewSystemHandler(nil, nil, nil, nil)

	// Create HTTP request
	req, _ := http.NewRequest("GET", "/health", nil)

	// Create response recorder
	w := httptest.NewRecorder()

	// Create Gin context
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	// Call handler
	handler.Health(c)

	// Assert response
	assert.Equal(t, http.StatusOK, w.Code)

	var response httppkg.HealthResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "ok", response.Status)
	assert.False(t, response.Timestamp.IsZero())
}

func TestDashboardHandler_Dashboard_NilMonitorEntries(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockMonitorService := new(MockMonitorService)
	handler := httppkg.NewDashboardHandler(mockMonitorService, nil, nil, nil)

	testMonitor := &domain.Monitor{
		ID:            "monitor-1",
		Name:          "Test Monitor",
		URL:           "https://example.com",
		CheckInterval: 5 * time.Minute,
		Enabled:       true,
		AlertChannels: []domain.AlertChannel{},
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	mockMonitorService.On("ListMonitors", mock.Anything, mock.Anything).Return([]*domain.Monitor{nil, testMonitor}, nil)

	req, _ := http.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	// Provide logged-in user context to avoid nil user path issues
	c.Set("user", &domain.User{ID: "u1", Email: "u1@example.com", Name: "U1"})

	handler.Dashboard(c)

	assert.Equal(t, http.StatusOK, w.Code)
	mockMonitorService.AssertExpectations(t)
}

func TestAuthHandler_Login_WithoutAuthService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := httppkg.NewAuthHandler(nil)

	// Build request with form body
	token, _ := domain.GenerateCSRFToken()

	form := url.Values{}
	form.Set("email", "user@example.com")
	form.Set("password", "password123")
	form.Set("csrf_token", token)

	req, _ := http.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: token, Path: "/"})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	handler.Login(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
