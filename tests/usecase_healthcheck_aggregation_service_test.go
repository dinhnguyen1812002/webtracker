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

func TestNewHealthCheckAggregationService(t *testing.T) {
	healthCheckRepo := &MockHealthCheckRepositoryForRetention{}
	aggregatedRepo := &MockAggregatedHealthCheckRepository{}
	monitorRepo := &MockMonitorRepositoryWithMock{}

	service := usecase.NewHealthCheckAggregationService(healthCheckRepo, aggregatedRepo, monitorRepo)

	assert.NotNil(t, service)
	assert.False(t, service.IsRunning())
}

func TestHealthCheckAggregationService_AggregateHour(t *testing.T) {
	healthCheckRepo := &MockHealthCheckRepositoryForRetention{}
	aggregatedRepo := &MockAggregatedHealthCheckRepository{}
	monitorRepo := &MockMonitorRepositoryWithMock{}

	service := usecase.NewHealthCheckAggregationService(healthCheckRepo, aggregatedRepo, monitorRepo)

	ctx := context.Background()
	monitorID := "monitor-1"
	hourStart := time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC)
	hourEnd := hourStart.Add(time.Hour)

	// Create sample health checks
	checks := []*domain.HealthCheck{
		{
			ID:           "check-1",
			MonitorID:    monitorID,
			Status:       domain.StatusSuccess,
			ResponseTime: 100 * time.Millisecond,
			CheckedAt:    hourStart.Add(10 * time.Minute),
		},
		{
			ID:           "check-2",
			MonitorID:    monitorID,
			Status:       domain.StatusSuccess,
			ResponseTime: 200 * time.Millisecond,
			CheckedAt:    hourStart.Add(30 * time.Minute),
		},
		{
			ID:           "check-3",
			MonitorID:    monitorID,
			Status:       domain.StatusFailure,
			ResponseTime: 0,
			CheckedAt:    hourStart.Add(50 * time.Minute),
		},
	}

	// Mock health check repository
	healthCheckRepo.On("GetByDateRange", ctx, monitorID, hourStart, hourEnd).Return(checks, nil)

	// Mock aggregated repository
	aggregatedRepo.On("Create", ctx, mock.AnythingOfType("*domain.AggregatedHealthCheck")).Return(nil)

	err := service.AggregateHour(ctx, monitorID, hourStart)

	assert.NoError(t, err)
	healthCheckRepo.AssertExpectations(t)

	// Verify Create was called once
	aggregatedRepo.AssertNumberOfCalls(t, "Create", 1)
}

func TestHealthCheckAggregationService_AggregateHour_NoChecks(t *testing.T) {
	healthCheckRepo := &MockHealthCheckRepositoryForRetention{}
	aggregatedRepo := &MockAggregatedHealthCheckRepository{}
	monitorRepo := &MockMonitorRepositoryWithMock{}

	service := usecase.NewHealthCheckAggregationService(healthCheckRepo, aggregatedRepo, monitorRepo)

	ctx := context.Background()
	monitorID := "monitor-1"
	hourStart := time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC)
	hourEnd := hourStart.Add(time.Hour)

	// Mock empty health checks
	healthCheckRepo.On("GetByDateRange", ctx, monitorID, hourStart, hourEnd).Return([]*domain.HealthCheck{}, nil)

	err := service.AggregateHour(ctx, monitorID, hourStart)

	assert.NoError(t, err)
	healthCheckRepo.AssertExpectations(t)
	// Should not call Create when there are no checks
	aggregatedRepo.AssertNotCalled(t, "Create")
}

func TestHealthCheckAggregationService_AggregateHour_AllFailures(t *testing.T) {
	healthCheckRepo := &MockHealthCheckRepositoryForRetention{}
	aggregatedRepo := &MockAggregatedHealthCheckRepository{}
	monitorRepo := &MockMonitorRepositoryWithMock{}

	service := usecase.NewHealthCheckAggregationService(healthCheckRepo, aggregatedRepo, monitorRepo)

	ctx := context.Background()
	monitorID := "monitor-1"
	hourStart := time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC)
	hourEnd := hourStart.Add(time.Hour)

	// Create sample health checks - all failures
	checks := []*domain.HealthCheck{
		{
			ID:           "check-1",
			MonitorID:    monitorID,
			Status:       domain.StatusFailure,
			ResponseTime: 0,
			CheckedAt:    hourStart.Add(10 * time.Minute),
		},
		{
			ID:           "check-2",
			MonitorID:    monitorID,
			Status:       domain.StatusTimeout,
			ResponseTime: 0,
			CheckedAt:    hourStart.Add(30 * time.Minute),
		},
	}

	// Mock health check repository
	healthCheckRepo.On("GetByDateRange", ctx, monitorID, hourStart, hourEnd).Return(checks, nil)

	// Mock aggregated repository
	aggregatedRepo.On("Create", ctx, mock.AnythingOfType("*domain.AggregatedHealthCheck")).Return(nil)

	err := service.AggregateHour(ctx, monitorID, hourStart)

	assert.NoError(t, err)
	healthCheckRepo.AssertExpectations(t)

	// Verify Create was called once
	aggregatedRepo.AssertNumberOfCalls(t, "Create", 1)
}

func TestHealthCheckAggregationService_StartStopScheduler(t *testing.T) {
	healthCheckRepo := &MockHealthCheckRepositoryForRetention{}
	aggregatedRepo := &MockAggregatedHealthCheckRepository{}
	monitorRepo := &MockMonitorRepositoryWithMock{}

	service := usecase.NewHealthCheckAggregationService(healthCheckRepo, aggregatedRepo, monitorRepo)

	ctx := context.Background()

	// Mock for initial aggregation run
	monitorRepo.On("List", ctx, mock.AnythingOfType("domain.ListFilters")).Return([]*domain.Monitor{}, nil)

	// Test starting scheduler
	err := service.StartAggregationScheduler(ctx)
	assert.NoError(t, err)
	assert.True(t, service.IsRunning())

	// Test starting already running scheduler
	err = service.StartAggregationScheduler(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")

	// Test stopping scheduler
	err = service.StopAggregationScheduler()
	assert.NoError(t, err)
	assert.False(t, service.IsRunning())

	// Test stopping already stopped scheduler
	err = service.StopAggregationScheduler()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not running")
}
