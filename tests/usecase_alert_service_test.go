package tests

import (
	"context"
	"errors"
	"testing"
	"time"

	"web-tracker/domain"
	"web-tracker/interface/alertchannel"
	"web-tracker/usecase"
)

// Mock repositories for testing
type mockAlertRepository struct {
	createFunc         func(ctx context.Context, alert *domain.Alert) error
	getByMonitorIDFunc func(ctx context.Context, monitorID string, limit int) ([]*domain.Alert, error)
	getLastAlertFunc   func(ctx context.Context, monitorID string, alertType domain.AlertType) (*time.Time, error)
}

func (m *mockAlertRepository) Create(ctx context.Context, alert *domain.Alert) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, alert)
	}
	return nil
}

func (m *mockAlertRepository) GetByMonitorID(ctx context.Context, monitorID string, limit int) ([]*domain.Alert, error) {
	if m.getByMonitorIDFunc != nil {
		return m.getByMonitorIDFunc(ctx, monitorID, limit)
	}
	return nil, nil
}

func (m *mockAlertRepository) GetByDateRange(ctx context.Context, monitorID string, start, end time.Time) ([]*domain.Alert, error) {
	return nil, nil
}

func (m *mockAlertRepository) GetLastAlertTime(ctx context.Context, monitorID string, alertType domain.AlertType) (*time.Time, error) {
	if m.getLastAlertFunc != nil {
		return m.getLastAlertFunc(ctx, monitorID, alertType)
	}
	return nil, nil
}

func (m *mockAlertRepository) DeleteOlderThan(ctx context.Context, before time.Time) error {
	return nil
}

type mockMonitorRepository struct {
	getByIDFunc func(ctx context.Context, id string) (*domain.Monitor, error)
}

func (m *mockMonitorRepository) GetByID(ctx context.Context, id string) (*domain.Monitor, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *mockMonitorRepository) Create(ctx context.Context, monitor *domain.Monitor) error {
	return nil
}

func (m *mockMonitorRepository) List(ctx context.Context, filters domain.ListFilters) ([]*domain.Monitor, error) {
	return nil, nil
}

func (m *mockMonitorRepository) Update(ctx context.Context, monitor *domain.Monitor) error {
	return nil
}

func (m *mockMonitorRepository) Delete(ctx context.Context, id string) error {
	return nil
}

// Test GenerateDowntimeAlert
func TestGenerateDowntimeAlert(t *testing.T) {
	tests := []struct {
		name          string
		monitor       *domain.Monitor
		healthCheck   *domain.HealthCheck
		createError   error
		expectedError bool
	}{
		{
			name: "successful downtime alert generation",
			monitor: &domain.Monitor{
				ID:   "monitor-1",
				Name: "Test Monitor",
				URL:  "https://example.com",
				AlertChannels: []domain.AlertChannel{
					{Type: domain.AlertChannelEmail},
				},
			},
			healthCheck: &domain.HealthCheck{
				ID:           "check-1",
				MonitorID:    "monitor-1",
				Status:       domain.StatusFailure,
				StatusCode:   500,
				ErrorMessage: "Internal Server Error",
				CheckedAt:    time.Now(),
			},
			createError:   nil,
			expectedError: false,
		},
		{
			name: "downtime alert with database error",
			monitor: &domain.Monitor{
				ID:   "monitor-1",
				Name: "Test Monitor",
				URL:  "https://example.com",
				AlertChannels: []domain.AlertChannel{
					{Type: domain.AlertChannelEmail},
				},
			},
			healthCheck: &domain.HealthCheck{
				ID:           "check-1",
				MonitorID:    "monitor-1",
				Status:       domain.StatusFailure,
				StatusCode:   500,
				ErrorMessage: "Internal Server Error",
				CheckedAt:    time.Now(),
			},
			createError:   errors.New("database error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alertRepo := &mockAlertRepository{
				createFunc: func(ctx context.Context, alert *domain.Alert) error {
					// Verify alert properties
					if alert.MonitorID != tt.monitor.ID {
						t.Errorf("expected monitor ID %s, got %s", tt.monitor.ID, alert.MonitorID)
					}
					if alert.Type != domain.AlertTypeDowntime {
						t.Errorf("expected alert type downtime, got %s", alert.Type)
					}
					if alert.Severity != domain.SeverityCritical {
						t.Errorf("expected severity critical, got %s", alert.Severity)
					}
					if len(alert.Channels) != len(tt.monitor.AlertChannels) {
						t.Errorf("expected %d channels, got %d", len(tt.monitor.AlertChannels), len(alert.Channels))
					}
					return tt.createError
				},
			}

			service := usecase.NewAlertService(alertRepo, nil, nil, alertchannel.NewDeliveryService(), nil)
			err := service.GenerateDowntimeAlert(context.Background(), tt.monitor, tt.healthCheck)

			if tt.expectedError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectedError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// Test GenerateRecoveryAlert
func TestGenerateRecoveryAlert(t *testing.T) {
	tests := []struct {
		name          string
		monitor       *domain.Monitor
		healthCheck   *domain.HealthCheck
		createError   error
		expectedError bool
	}{
		{
			name: "successful recovery alert generation",
			monitor: &domain.Monitor{
				ID:   "monitor-1",
				Name: "Test Monitor",
				URL:  "https://example.com",
				AlertChannels: []domain.AlertChannel{
					{Type: domain.AlertChannelEmail},
				},
			},
			healthCheck: &domain.HealthCheck{
				ID:           "check-1",
				MonitorID:    "monitor-1",
				Status:       domain.StatusSuccess,
				StatusCode:   200,
				ResponseTime: 150 * time.Millisecond,
				CheckedAt:    time.Now(),
			},
			createError:   nil,
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alertRepo := &mockAlertRepository{
				createFunc: func(ctx context.Context, alert *domain.Alert) error {
					// Verify alert properties
					if alert.MonitorID != tt.monitor.ID {
						t.Errorf("expected monitor ID %s, got %s", tt.monitor.ID, alert.MonitorID)
					}
					if alert.Type != domain.AlertTypeRecovery {
						t.Errorf("expected alert type recovery, got %s", alert.Type)
					}
					if alert.Severity != domain.SeverityInfo {
						t.Errorf("expected severity info, got %s", alert.Severity)
					}
					return tt.createError
				},
			}

			service := usecase.NewAlertService(alertRepo, nil, nil, alertchannel.NewDeliveryService(), nil)
			err := service.GenerateRecoveryAlert(context.Background(), tt.monitor, tt.healthCheck)

			if tt.expectedError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectedError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// Test GenerateSSLExpirationAlert
func TestGenerateSSLExpirationAlert(t *testing.T) {
	tests := []struct {
		name             string
		monitor          *domain.Monitor
		sslInfo          *domain.SSLInfo
		expectedType     domain.AlertType
		expectedSeverity domain.AlertSeverity
		expectedError    bool
	}{
		{
			name: "SSL certificate expired",
			monitor: &domain.Monitor{
				ID:   "monitor-1",
				Name: "Test Monitor",
				URL:  "https://example.com",
				AlertChannels: []domain.AlertChannel{
					{Type: domain.AlertChannelEmail},
				},
			},
			sslInfo: &domain.SSLInfo{
				Valid:     true,
				ExpiresAt: time.Now().Add(-1 * 24 * time.Hour),
				DaysUntil: -1,
				Issuer:    "Test CA",
			},
			expectedType:     domain.AlertTypeSSLExpired,
			expectedSeverity: domain.SeverityCritical,
			expectedError:    false,
		},
		{
			name: "SSL certificate expires in 7 days (critical)",
			monitor: &domain.Monitor{
				ID:   "monitor-1",
				Name: "Test Monitor",
				URL:  "https://example.com",
				AlertChannels: []domain.AlertChannel{
					{Type: domain.AlertChannelEmail},
				},
			},
			sslInfo: &domain.SSLInfo{
				Valid:     true,
				ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
				DaysUntil: 7,
				Issuer:    "Test CA",
			},
			expectedType:     domain.AlertTypeSSLExpiring,
			expectedSeverity: domain.SeverityCritical,
			expectedError:    false,
		},
		{
			name: "SSL certificate expires in 15 days (warning)",
			monitor: &domain.Monitor{
				ID:   "monitor-1",
				Name: "Test Monitor",
				URL:  "https://example.com",
				AlertChannels: []domain.AlertChannel{
					{Type: domain.AlertChannelEmail},
				},
			},
			sslInfo: &domain.SSLInfo{
				Valid:     true,
				ExpiresAt: time.Now().Add(15 * 24 * time.Hour),
				DaysUntil: 15,
				Issuer:    "Test CA",
			},
			expectedType:     domain.AlertTypeSSLExpiring,
			expectedSeverity: domain.SeverityWarning,
			expectedError:    false,
		},
		{
			name: "SSL certificate expires in 30 days (warning)",
			monitor: &domain.Monitor{
				ID:   "monitor-1",
				Name: "Test Monitor",
				URL:  "https://example.com",
				AlertChannels: []domain.AlertChannel{
					{Type: domain.AlertChannelEmail},
				},
			},
			sslInfo: &domain.SSLInfo{
				Valid:     true,
				ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
				DaysUntil: 30,
				Issuer:    "Test CA",
			},
			expectedType:     domain.AlertTypeSSLExpiring,
			expectedSeverity: domain.SeverityWarning,
			expectedError:    false,
		},
		{
			name: "nil SSL info",
			monitor: &domain.Monitor{
				ID:   "monitor-1",
				Name: "Test Monitor",
				URL:  "https://example.com",
				AlertChannels: []domain.AlertChannel{
					{Type: domain.AlertChannelEmail},
				},
			},
			sslInfo:       nil,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alertRepo := &mockAlertRepository{
				createFunc: func(ctx context.Context, alert *domain.Alert) error {
					if tt.sslInfo != nil {
						// Verify alert properties
						if alert.MonitorID != tt.monitor.ID {
							t.Errorf("expected monitor ID %s, got %s", tt.monitor.ID, alert.MonitorID)
						}
						if alert.Type != tt.expectedType {
							t.Errorf("expected alert type %s, got %s", tt.expectedType, alert.Type)
						}
						if alert.Severity != tt.expectedSeverity {
							t.Errorf("expected severity %s, got %s", tt.expectedSeverity, alert.Severity)
						}
					}
					return nil
				},
			}

			service := usecase.NewAlertService(alertRepo, nil, nil, alertchannel.NewDeliveryService(), nil)
			err := service.GenerateSSLExpirationAlert(context.Background(), tt.monitor, tt.sslInfo)

			if tt.expectedError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectedError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// Test ProcessHealthCheckAlerts
func TestProcessHealthCheckAlerts(t *testing.T) {
	tests := []struct {
		name              string
		monitorID         string
		healthCheck       *domain.HealthCheck
		monitor           *domain.Monitor
		lastDowntimeAlert *time.Time
		expectedDowntime  bool
		expectedRecovery  bool
		expectedSSL       bool
		expectedError     bool
	}{
		{
			name:      "failed health check generates downtime alert",
			monitorID: "monitor-1",
			healthCheck: &domain.HealthCheck{
				ID:           "check-1",
				MonitorID:    "monitor-1",
				Status:       domain.StatusFailure,
				StatusCode:   500,
				ErrorMessage: "Internal Server Error",
				CheckedAt:    time.Now(),
			},
			monitor: &domain.Monitor{
				ID:   "monitor-1",
				Name: "Test Monitor",
				URL:  "https://example.com",
				AlertChannels: []domain.AlertChannel{
					{Type: domain.AlertChannelEmail},
				},
			},
			lastDowntimeAlert: nil,
			expectedDowntime:  true,
			expectedRecovery:  false,
			expectedSSL:       false,
			expectedError:     false,
		},
		{
			name:      "successful health check after recent downtime generates recovery alert",
			monitorID: "monitor-1",
			healthCheck: &domain.HealthCheck{
				ID:           "check-1",
				MonitorID:    "monitor-1",
				Status:       domain.StatusSuccess,
				StatusCode:   200,
				ResponseTime: 150 * time.Millisecond,
				CheckedAt:    time.Now(),
			},
			monitor: &domain.Monitor{
				ID:   "monitor-1",
				Name: "Test Monitor",
				URL:  "https://example.com",
				AlertChannels: []domain.AlertChannel{
					{Type: domain.AlertChannelEmail},
				},
			},
			lastDowntimeAlert: func() *time.Time { t := time.Now().Add(-10 * time.Minute); return &t }(),
			expectedDowntime:  false,
			expectedRecovery:  true,
			expectedSSL:       false,
			expectedError:     false,
		},
		{
			name:      "successful health check with SSL expiring in 7 days",
			monitorID: "monitor-1",
			healthCheck: &domain.HealthCheck{
				ID:           "check-1",
				MonitorID:    "monitor-1",
				Status:       domain.StatusSuccess,
				StatusCode:   200,
				ResponseTime: 150 * time.Millisecond,
				CheckedAt:    time.Now(),
				SSLInfo: &domain.SSLInfo{
					Valid:     true,
					ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
					DaysUntil: 7,
					Issuer:    "Test CA",
				},
			},
			monitor: &domain.Monitor{
				ID:   "monitor-1",
				Name: "Test Monitor",
				URL:  "https://example.com",
				AlertChannels: []domain.AlertChannel{
					{Type: domain.AlertChannelEmail},
				},
			},
			lastDowntimeAlert: nil,
			expectedDowntime:  false,
			expectedRecovery:  false,
			expectedSSL:       true,
			expectedError:     false,
		},
		{
			name:      "no alerts when monitor has no alert channels",
			monitorID: "monitor-1",
			healthCheck: &domain.HealthCheck{
				ID:           "check-1",
				MonitorID:    "monitor-1",
				Status:       domain.StatusFailure,
				StatusCode:   500,
				ErrorMessage: "Internal Server Error",
				CheckedAt:    time.Now(),
			},
			monitor: &domain.Monitor{
				ID:            "monitor-1",
				Name:          "Test Monitor",
				URL:           "https://example.com",
				AlertChannels: []domain.AlertChannel{}, // No channels
			},
			lastDowntimeAlert: nil,
			expectedDowntime:  false,
			expectedRecovery:  false,
			expectedSSL:       false,
			expectedError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			downtimeAlertCreated := false
			recoveryAlertCreated := false
			sslAlertCreated := false

			alertRepo := &mockAlertRepository{
				createFunc: func(ctx context.Context, alert *domain.Alert) error {
					switch alert.Type {
					case domain.AlertTypeDowntime:
						downtimeAlertCreated = true
					case domain.AlertTypeRecovery:
						recoveryAlertCreated = true
					case domain.AlertTypeSSLExpiring, domain.AlertTypeSSLExpired:
						sslAlertCreated = true
					}
					return nil
				},
				getLastAlertFunc: func(ctx context.Context, monitorID string, alertType domain.AlertType) (*time.Time, error) {
					if alertType == domain.AlertTypeDowntime {
						return tt.lastDowntimeAlert, nil
					}
					return nil, nil
				},
			}

			monitorRepo := &mockMonitorRepository{
				getByIDFunc: func(ctx context.Context, id string) (*domain.Monitor, error) {
					return tt.monitor, nil
				},
			}

			service := usecase.NewAlertService(alertRepo, monitorRepo, nil, alertchannel.NewDeliveryService(), nil)
			err := service.ProcessHealthCheckAlerts(context.Background(), tt.monitorID, tt.healthCheck)

			if tt.expectedError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectedError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if downtimeAlertCreated != tt.expectedDowntime {
				t.Errorf("expected downtime alert: %v, got: %v", tt.expectedDowntime, downtimeAlertCreated)
			}
			if recoveryAlertCreated != tt.expectedRecovery {
				t.Errorf("expected recovery alert: %v, got: %v", tt.expectedRecovery, recoveryAlertCreated)
			}
			if sslAlertCreated != tt.expectedSSL {
				t.Errorf("expected SSL alert: %v, got: %v", tt.expectedSSL, sslAlertCreated)
			}
		})
	}
}

// Test shouldGenerateSSLAlert
// shouldGenerateSSLAlert is unexported; its behavior is covered by SSL alert generation tests above.
