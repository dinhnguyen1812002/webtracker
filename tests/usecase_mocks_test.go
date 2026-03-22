package tests

import (
	"context"
	"fmt"
	"time"

	"web-tracker/domain"

	"github.com/stretchr/testify/mock"
)

// MockMonitorRepository is a mock implementation of domain.MonitorRepository for testing
type MockMonitorRepository struct {
	monitors map[string]*domain.Monitor
	nextID   int
}

func NewMockMonitorRepository() *MockMonitorRepository {
	return &MockMonitorRepository{
		monitors: make(map[string]*domain.Monitor),
		nextID:   1,
	}
}

func (m *MockMonitorRepository) Create(ctx context.Context, monitor *domain.Monitor) error {
	if monitor.ID == "" {
		return fmt.Errorf("monitor ID is required")
	}
	m.monitors[monitor.ID] = monitor
	return nil
}

func (m *MockMonitorRepository) GetByID(ctx context.Context, id string) (*domain.Monitor, error) {
	monitor, exists := m.monitors[id]
	if !exists {
		return nil, fmt.Errorf("monitor not found: %s", id)
	}
	return monitor, nil
}

func (m *MockMonitorRepository) List(ctx context.Context, filters domain.ListFilters) ([]*domain.Monitor, error) {
	var result []*domain.Monitor
	for _, monitor := range m.monitors {
		if filters.Enabled == nil || monitor.Enabled == *filters.Enabled {
			result = append(result, monitor)
		}
	}
	return result, nil
}

func (m *MockMonitorRepository) Update(ctx context.Context, monitor *domain.Monitor) error {
	if _, exists := m.monitors[monitor.ID]; !exists {
		return fmt.Errorf("monitor not found: %s", monitor.ID)
	}
	m.monitors[monitor.ID] = monitor
	return nil
}

func (m *MockMonitorRepository) Delete(ctx context.Context, id string) error {
	if _, exists := m.monitors[id]; !exists {
		return fmt.Errorf("monitor not found: %s", id)
	}
	delete(m.monitors, id)
	return nil
}

// MockMonitorRepositoryWithMock is a testify mock implementation of MonitorRepository
type MockMonitorRepositoryWithMock struct {
	mock.Mock
}

func (m *MockMonitorRepositoryWithMock) Create(ctx context.Context, monitor *domain.Monitor) error {
	args := m.Called(ctx, monitor)
	return args.Error(0)
}

func (m *MockMonitorRepositoryWithMock) GetByID(ctx context.Context, id string) (*domain.Monitor, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Monitor), args.Error(1)
}

func (m *MockMonitorRepositoryWithMock) List(ctx context.Context, filters domain.ListFilters) ([]*domain.Monitor, error) {
	args := m.Called(ctx, filters)
	return args.Get(0).([]*domain.Monitor), args.Error(1)
}

func (m *MockMonitorRepositoryWithMock) Update(ctx context.Context, monitor *domain.Monitor) error {
	args := m.Called(ctx, monitor)
	return args.Error(0)
}

func (m *MockMonitorRepositoryWithMock) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// MockScheduler is a mock implementation of Scheduler for testing
type MockScheduler struct {
	scheduledMonitors   map[string]*domain.Monitor
	unscheduledMonitors map[string]bool
}

func NewMockScheduler() *MockScheduler {
	return &MockScheduler{
		scheduledMonitors:   make(map[string]*domain.Monitor),
		unscheduledMonitors: make(map[string]bool),
	}
}

func (m *MockScheduler) Start(ctx context.Context) error {
	return nil
}

func (m *MockScheduler) Stop() error {
	return nil
}

func (m *MockScheduler) ScheduleMonitor(monitor *domain.Monitor) error {
	m.scheduledMonitors[monitor.ID] = monitor
	delete(m.unscheduledMonitors, monitor.ID)
	return nil
}

func (m *MockScheduler) UnscheduleMonitor(monitorID string) error {
	delete(m.scheduledMonitors, monitorID)
	m.unscheduledMonitors[monitorID] = true
	return nil
}

func (m *MockScheduler) RescheduleMonitor(monitor *domain.Monitor) error {
	m.scheduledMonitors[monitor.ID] = monitor
	return nil
}

func (m *MockScheduler) IsRunning() bool {
	return true
}

// MockAggregatedHealthCheckRepository is a mock implementation of AggregatedHealthCheckRepository
type MockAggregatedHealthCheckRepository struct {
	mock.Mock
}

func (m *MockAggregatedHealthCheckRepository) Create(ctx context.Context, aggregated *domain.AggregatedHealthCheck) error {
	args := m.Called(ctx, aggregated)
	return args.Error(0)
}

func (m *MockAggregatedHealthCheckRepository) GetByMonitorID(ctx context.Context, monitorID string, limit int) ([]*domain.AggregatedHealthCheck, error) {
	args := m.Called(ctx, monitorID, limit)
	return args.Get(0).([]*domain.AggregatedHealthCheck), args.Error(1)
}

func (m *MockAggregatedHealthCheckRepository) GetByDateRange(ctx context.Context, monitorID string, start, end time.Time) ([]*domain.AggregatedHealthCheck, error) {
	args := m.Called(ctx, monitorID, start, end)
	return args.Get(0).([]*domain.AggregatedHealthCheck), args.Error(1)
}

func (m *MockAggregatedHealthCheckRepository) DeleteOlderThan(ctx context.Context, before time.Time) error {
	args := m.Called(ctx, before)
	return args.Error(0)
}

// MockHealthCheckAlertService is a mock implementation of HealthCheckAlertService
type MockHealthCheckAlertService struct {
	alerts []string
}

func NewMockHealthCheckAlertService() *MockHealthCheckAlertService {
	return &MockHealthCheckAlertService{
		alerts: make([]string, 0),
	}
}

func (m *MockHealthCheckAlertService) GeneratePerformanceAlert(ctx context.Context, monitor *domain.Monitor, responseTime, threshold time.Duration) error {
	m.alerts = append(m.alerts, fmt.Sprintf("Performance alert for monitor %s: %v > %v", monitor.ID, responseTime, threshold))
	return nil
}
