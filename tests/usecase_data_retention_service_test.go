package tests

import (
	"context"
	"testing"
	"time"

	"web-tracker/domain"
	"web-tracker/usecase"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAlertRepositoryForRetention is a mock implementation of AlertRepository for retention tests
type MockAlertRepositoryForRetention struct {
	mock.Mock
}

func (m *MockAlertRepositoryForRetention) Create(ctx context.Context, alert *domain.Alert) error {
	args := m.Called(ctx, alert)
	return args.Error(0)
}

func (m *MockAlertRepositoryForRetention) GetByMonitorID(ctx context.Context, monitorID string, limit int) ([]*domain.Alert, error) {
	args := m.Called(ctx, monitorID, limit)
	return args.Get(0).([]*domain.Alert), args.Error(1)
}

func (m *MockAlertRepositoryForRetention) GetByDateRange(ctx context.Context, monitorID string, start, end time.Time) ([]*domain.Alert, error) {
	args := m.Called(ctx, monitorID, start, end)
	return args.Get(0).([]*domain.Alert), args.Error(1)
}

func (m *MockAlertRepositoryForRetention) GetLastAlertTime(ctx context.Context, monitorID string, alertType domain.AlertType) (*time.Time, error) {
	args := m.Called(ctx, monitorID, alertType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*time.Time), args.Error(1)
}

func (m *MockAlertRepositoryForRetention) DeleteOlderThan(ctx context.Context, before time.Time) error {
	args := m.Called(ctx, before)
	return args.Error(0)
}

// MockHealthCheckRepositoryForRetention is a mock implementation of HealthCheckRepository for retention tests
type MockHealthCheckRepositoryForRetention struct {
	mock.Mock
}

func (m *MockHealthCheckRepositoryForRetention) Create(ctx context.Context, check *domain.HealthCheck) error {
	args := m.Called(ctx, check)
	return args.Error(0)
}

func (m *MockHealthCheckRepositoryForRetention) GetByMonitorID(ctx context.Context, monitorID string, limit int) ([]*domain.HealthCheck, error) {
	args := m.Called(ctx, monitorID, limit)
	return args.Get(0).([]*domain.HealthCheck), args.Error(1)
}

func (m *MockHealthCheckRepositoryForRetention) GetByDateRange(ctx context.Context, monitorID string, start, end time.Time) ([]*domain.HealthCheck, error) {
	args := m.Called(ctx, monitorID, start, end)
	return args.Get(0).([]*domain.HealthCheck), args.Error(1)
}

func (m *MockHealthCheckRepositoryForRetention) DeleteOlderThan(ctx context.Context, before time.Time) error {
	args := m.Called(ctx, before)
	return args.Error(0)
}

func TestNewDataRetentionService(t *testing.T) {
	healthCheckRepo := &MockHealthCheckRepositoryForRetention{}
	alertRepo := &MockAlertRepositoryForRetention{}

	service := usecase.NewDataRetentionService(healthCheckRepo, alertRepo)

	assert.NotNil(t, service)
	assert.False(t, service.IsRunning())
}

func TestDataRetentionService_RunCleanup(t *testing.T) {
	healthCheckRepo := &MockHealthCheckRepositoryForRetention{}
	alertRepo := &MockAlertRepositoryForRetention{}

	service := usecase.NewDataRetentionService(healthCheckRepo, alertRepo)

	ctx := context.Background()

	// Mock successful cleanup
	healthCheckRepo.On("DeleteOlderThan", ctx, mock.AnythingOfType("time.Time")).Return(nil)
	alertRepo.On("DeleteOlderThan", ctx, mock.AnythingOfType("time.Time")).Return(nil)

	err := service.RunCleanup(ctx)

	assert.NoError(t, err)
	healthCheckRepo.AssertExpectations(t)
	alertRepo.AssertExpectations(t)
}

func TestDataRetentionService_RunCleanup_HealthCheckError(t *testing.T) {
	healthCheckRepo := &MockHealthCheckRepositoryForRetention{}
	alertRepo := &MockAlertRepositoryForRetention{}

	service := usecase.NewDataRetentionService(healthCheckRepo, alertRepo)

	ctx := context.Background()

	// Mock health check cleanup failure
	healthCheckRepo.On("DeleteOlderThan", ctx, mock.AnythingOfType("time.Time")).Return(assert.AnError)

	err := service.RunCleanup(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete old health checks")
	healthCheckRepo.AssertExpectations(t)
}

func TestDataRetentionService_RunCleanup_AlertError(t *testing.T) {
	healthCheckRepo := &MockHealthCheckRepositoryForRetention{}
	alertRepo := &MockAlertRepositoryForRetention{}

	service := usecase.NewDataRetentionService(healthCheckRepo, alertRepo)

	ctx := context.Background()

	// Mock successful health check cleanup but alert cleanup failure
	healthCheckRepo.On("DeleteOlderThan", ctx, mock.AnythingOfType("time.Time")).Return(nil)
	alertRepo.On("DeleteOlderThan", ctx, mock.AnythingOfType("time.Time")).Return(assert.AnError)

	err := service.RunCleanup(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete old alerts")
	healthCheckRepo.AssertExpectations(t)
	alertRepo.AssertExpectations(t)
}

func TestDataRetentionService_StartStopScheduler(t *testing.T) {
	healthCheckRepo := &MockHealthCheckRepositoryForRetention{}
	alertRepo := &MockAlertRepositoryForRetention{}

	service := usecase.NewDataRetentionService(healthCheckRepo, alertRepo)

	ctx := context.Background()

	// Mock cleanup calls for initial run
	healthCheckRepo.On("DeleteOlderThan", ctx, mock.AnythingOfType("time.Time")).Return(nil)
	alertRepo.On("DeleteOlderThan", ctx, mock.AnythingOfType("time.Time")).Return(nil)

	// Test starting scheduler
	err := service.StartCleanupScheduler(ctx)
	assert.NoError(t, err)
	assert.True(t, service.IsRunning())

	// Test starting already running scheduler
	err = service.StartCleanupScheduler(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")

	// Test stopping scheduler
	err = service.StopCleanupScheduler()
	assert.NoError(t, err)
	assert.False(t, service.IsRunning())

	// Test stopping already stopped scheduler
	err = service.StopCleanupScheduler()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not running")
}
