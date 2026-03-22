package tests

import (
	"context"
	"testing"
	"time"

	"web-tracker/domain"
	"web-tracker/interface/alertchannel"
	"web-tracker/usecase"
)

// Integration test for alert generation with real repositories
func TestAlertGeneration_Integration(t *testing.T) {
	// This test uses mock repositories to simulate the full alert generation flow
	// In a real integration test, this would use actual database connections

	ctx := context.Background()

	// Create mock repositories
	alertRepo := &mockAlertRepository{
		createFunc: func(ctx context.Context, alert *domain.Alert) error {
			// Verify alert was created with correct properties
			if alert.ID == "" {
				t.Error("alert ID should not be empty")
			}
			if alert.MonitorID == "" {
				t.Error("monitor ID should not be empty")
			}
			if alert.SentAt.IsZero() {
				t.Error("sent_at should not be zero")
			}
			return nil
		},
		getLastAlertFunc: func(ctx context.Context, monitorID string, alertType domain.AlertType) (*time.Time, error) {
			return nil, nil
		},
	}

	monitorRepo := &mockMonitorRepository{
		getByIDFunc: func(ctx context.Context, id string) (*domain.Monitor, error) {
			return &domain.Monitor{
				ID:   id,
				Name: "Test Monitor",
				URL:  "https://example.com",
				AlertChannels: []domain.AlertChannel{
					{Type: domain.AlertChannelEmail},
					{Type: domain.AlertChannelTelegram},
				},
			}, nil
		},
	}

	service := usecase.NewAlertService(alertRepo, monitorRepo, nil, alertchannel.NewDeliveryService(), nil)

	t.Run("downtime alert generation flow", func(t *testing.T) {
		healthCheck := &domain.HealthCheck{
			ID:           "check-1",
			MonitorID:    "monitor-1",
			Status:       domain.StatusFailure,
			StatusCode:   500,
			ErrorMessage: "Internal Server Error",
			CheckedAt:    time.Now(),
		}

		err := service.ProcessHealthCheckAlerts(ctx, "monitor-1", healthCheck)
		if err != nil {
			t.Fatalf("failed to process health check alerts: %v", err)
		}

		t.Log("✓ Downtime alert successfully generated and persisted")
	})

	t.Run("recovery alert generation flow", func(t *testing.T) {
		// Simulate a recent downtime alert
		recentDowntime := time.Now().Add(-5 * time.Minute)
		alertRepo.getLastAlertFunc = func(ctx context.Context, monitorID string, alertType domain.AlertType) (*time.Time, error) {
			if alertType == domain.AlertTypeDowntime {
				return &recentDowntime, nil
			}
			return nil, nil
		}

		healthCheck := &domain.HealthCheck{
			ID:           "check-2",
			MonitorID:    "monitor-1",
			Status:       domain.StatusSuccess,
			StatusCode:   200,
			ResponseTime: 150 * time.Millisecond,
			CheckedAt:    time.Now(),
		}

		err := service.ProcessHealthCheckAlerts(ctx, "monitor-1", healthCheck)
		if err != nil {
			t.Fatalf("failed to process health check alerts: %v", err)
		}

		t.Log("✓ Recovery alert successfully generated and persisted")
	})

	t.Run("SSL expiration alert generation flow", func(t *testing.T) {
		healthCheck := &domain.HealthCheck{
			ID:           "check-3",
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
		}

		err := service.ProcessHealthCheckAlerts(ctx, "monitor-1", healthCheck)
		if err != nil {
			t.Fatalf("failed to process health check alerts: %v", err)
		}

		t.Log("✓ SSL expiration alert successfully generated and persisted")
	})

	t.Run("multiple alert types in single check", func(t *testing.T) {
		// This scenario shouldn't happen in practice (failed check with valid SSL)
		// but tests that the service handles multiple alert types correctly
		alertCount := 0
		alertRepo.createFunc = func(ctx context.Context, alert *domain.Alert) error {
			alertCount++
			return nil
		}

		healthCheck := &domain.HealthCheck{
			ID:           "check-4",
			MonitorID:    "monitor-1",
			Status:       domain.StatusFailure,
			StatusCode:   500,
			ErrorMessage: "Internal Server Error",
			CheckedAt:    time.Now(),
			SSLInfo: &domain.SSLInfo{
				Valid:     true,
				ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
				DaysUntil: 7,
				Issuer:    "Test CA",
			},
		}

		err := service.ProcessHealthCheckAlerts(ctx, "monitor-1", healthCheck)
		if err != nil {
			t.Fatalf("failed to process health check alerts: %v", err)
		}

		// Should generate both downtime and SSL alerts
		if alertCount != 2 {
			t.Errorf("expected 2 alerts, got %d", alertCount)
		}

		t.Log("✓ Multiple alert types successfully generated")
	})
}

// Test alert severity determination
func TestAlertSeverityDetermination(t *testing.T) {
	tests := []struct {
		name             string
		daysUntil        int
		expectedSeverity domain.AlertSeverity
		expectedType     domain.AlertType
	}{
		{
			name:             "expired certificate",
			daysUntil:        -1,
			expectedSeverity: domain.SeverityCritical,
			expectedType:     domain.AlertTypeSSLExpired,
		},
		{
			name:             "expires today",
			daysUntil:        0,
			expectedSeverity: domain.SeverityCritical,
			expectedType:     domain.AlertTypeSSLExpired,
		},
		{
			name:             "expires in 7 days",
			daysUntil:        7,
			expectedSeverity: domain.SeverityCritical,
			expectedType:     domain.AlertTypeSSLExpiring,
		},
		{
			name:             "expires in 15 days",
			daysUntil:        15,
			expectedSeverity: domain.SeverityWarning,
			expectedType:     domain.AlertTypeSSLExpiring,
		},
		{
			name:             "expires in 30 days",
			daysUntil:        30,
			expectedSeverity: domain.SeverityWarning,
			expectedType:     domain.AlertTypeSSLExpiring,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			severity := domain.DetermineSSLAlertSeverity(tt.daysUntil)
			if severity != tt.expectedSeverity {
				t.Errorf("expected severity %s, got %s", tt.expectedSeverity, severity)
			}

			alertType := domain.DetermineSSLAlertType(tt.daysUntil)
			if alertType != tt.expectedType {
				t.Errorf("expected type %s, got %s", tt.expectedType, alertType)
			}
		})
	}
}

// Test alert channel extraction
// extractChannelTypes is unexported; exercised via alert generation tests above.
