package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"web-tracker/domain"
	"web-tracker/interface/alertchannel"
)

// RateLimiter defines the interface for rate limiting operations
type RateLimiter interface {
	// CheckAndSetDowntimeAlert checks if a downtime alert should be sent and updates the rate limit
	// Returns true if the alert should be sent, false if it should be suppressed
	CheckAndSetDowntimeAlert(ctx context.Context, monitorID string) (bool, error)

	// CheckAndSetSSLAlert checks if an SSL alert should be sent for a specific warning level
	// Returns true if the alert should be sent, false if it should be suppressed
	CheckAndSetSSLAlert(ctx context.Context, monitorID string, daysUntil int) (bool, error)

	// GetLastDowntimeAlert returns the timestamp of the last downtime alert for a monitor
	GetLastDowntimeAlert(ctx context.Context, monitorID string) (*time.Time, error)

	// ClearDowntimeAlert clears the downtime alert rate limit (used when monitor recovers)
	ClearDowntimeAlert(ctx context.Context, monitorID string) error
}

// AlertService handles alert generation and delivery logic
type AlertService struct {
	alertRepo            domain.AlertRepository
	monitorRepo          domain.MonitorRepository
	rateLimiter          RateLimiter
	deliveryService      *alertchannel.DeliveryService
	websocketBroadcaster WebSocketBroadcaster
}

// NewAlertService creates a new alert service
func NewAlertService(
	alertRepo domain.AlertRepository,
	monitorRepo domain.MonitorRepository,
	rateLimiter RateLimiter,
	deliveryService *alertchannel.DeliveryService,
	websocketBroadcaster WebSocketBroadcaster,
) *AlertService {
	return &AlertService{
		alertRepo:            alertRepo,
		monitorRepo:          monitorRepo,
		rateLimiter:          rateLimiter,
		deliveryService:      deliveryService,
		websocketBroadcaster: websocketBroadcaster,
	}
}

// GenerateDowntimeAlert generates an alert when a monitor fails
// Requirements: 2.2, 5.1
func (s *AlertService) GenerateDowntimeAlert(ctx context.Context, monitor *domain.Monitor, healthCheck *domain.HealthCheck) error {
	alert := &domain.Alert{
		ID:        uuid.New().String(),
		MonitorID: monitor.ID,
		Type:      domain.AlertTypeDowntime,
		Severity:  domain.SeverityCritical,
		Message:   fmt.Sprintf("Monitor '%s' is down", monitor.Name),
		Details: map[string]interface{}{
			"url":           monitor.URL,
			"status_code":   healthCheck.StatusCode,
			"error_message": healthCheck.ErrorMessage,
			"checked_at":    healthCheck.CheckedAt,
		},
		SentAt:   time.Now(),
		Channels: s.extractChannelTypes(monitor.AlertChannels),
	}

	// Persist alert to database (Requirement 5.1)
	if err := s.alertRepo.Create(ctx, alert); err != nil {
		return fmt.Errorf("failed to persist downtime alert: %w", err)
	}

	// Broadcast alert to WebSocket clients (Requirement 9.3)
	if s.websocketBroadcaster != nil {
		s.websocketBroadcaster.BroadcastAlert(alert)
	}

	return nil
}

// GenerateRecoveryAlert generates an alert when a monitor recovers from failure
// Requirements: 2.3, 5.1
func (s *AlertService) GenerateRecoveryAlert(ctx context.Context, monitor *domain.Monitor, healthCheck *domain.HealthCheck) error {
	alert := &domain.Alert{
		ID:        uuid.New().String(),
		MonitorID: monitor.ID,
		Type:      domain.AlertTypeRecovery,
		Severity:  domain.SeverityInfo,
		Message:   fmt.Sprintf("Monitor '%s' has recovered", monitor.Name),
		Details: map[string]interface{}{
			"url":           monitor.URL,
			"status_code":   healthCheck.StatusCode,
			"response_time": healthCheck.ResponseTime.Milliseconds(),
			"checked_at":    healthCheck.CheckedAt,
		},
		SentAt:   time.Now(),
		Channels: s.extractChannelTypes(monitor.AlertChannels),
	}

	// Persist alert to database (Requirement 5.1)
	if err := s.alertRepo.Create(ctx, alert); err != nil {
		return fmt.Errorf("failed to persist recovery alert: %w", err)
	}

	// Broadcast alert to WebSocket clients (Requirement 9.3)
	if s.websocketBroadcaster != nil {
		s.websocketBroadcaster.BroadcastAlert(alert)
	}

	return nil
}

// GenerateSSLExpirationAlert generates an alert for SSL certificate expiration
// Requirements: 2.2, 2.3, 2.4, 2.5, 5.1
func (s *AlertService) GenerateSSLExpirationAlert(ctx context.Context, monitor *domain.Monitor, sslInfo *domain.SSLInfo) error {
	if sslInfo == nil {
		return fmt.Errorf("SSL info is required for SSL expiration alert")
	}

	// Determine alert type and severity based on days until expiration
	// Requirement 2.2: 30 days warning
	// Requirement 2.3: 15 days warning
	// Requirement 2.4: 7 days critical
	// Requirement 2.5: expired critical
	alertType := domain.DetermineSSLAlertType(sslInfo.DaysUntil)
	severity := domain.DetermineSSLAlertSeverity(sslInfo.DaysUntil)

	// Create appropriate message based on expiration status
	var message string
	if sslInfo.DaysUntil <= 0 {
		message = fmt.Sprintf("SSL certificate for '%s' has expired", monitor.Name)
	} else if sslInfo.DaysUntil <= 7 {
		message = fmt.Sprintf("SSL certificate for '%s' expires in %d days (CRITICAL)", monitor.Name, sslInfo.DaysUntil)
	} else if sslInfo.DaysUntil <= 15 {
		message = fmt.Sprintf("SSL certificate for '%s' expires in %d days", monitor.Name, sslInfo.DaysUntil)
	} else {
		message = fmt.Sprintf("SSL certificate for '%s' expires in %d days", monitor.Name, sslInfo.DaysUntil)
	}

	alert := &domain.Alert{
		ID:        uuid.New().String(),
		MonitorID: monitor.ID,
		Type:      alertType,
		Severity:  severity,
		Message:   message,
		Details: map[string]interface{}{
			"url":        monitor.URL,
			"expires_at": sslInfo.ExpiresAt,
			"days_until": sslInfo.DaysUntil,
			"issuer":     sslInfo.Issuer,
			"valid":      sslInfo.Valid,
		},
		SentAt:   time.Now(),
		Channels: s.extractChannelTypes(monitor.AlertChannels),
	}

	// Persist alert to database (Requirement 5.1)
	if err := s.alertRepo.Create(ctx, alert); err != nil {
		return fmt.Errorf("failed to persist SSL expiration alert: %w", err)
	}

	// Broadcast alert to WebSocket clients (Requirement 9.3)
	if s.websocketBroadcaster != nil {
		s.websocketBroadcaster.BroadcastAlert(alert)
	}

	return nil
}

// ProcessHealthCheckAlerts processes a health check and generates appropriate alerts
// This is the main entry point for alert generation after a health check completes
// Requirements: 5.2, 5.3, 5.4, 5.5 (rate limiting)
func (s *AlertService) ProcessHealthCheckAlerts(ctx context.Context, monitorID string, healthCheck *domain.HealthCheck) error {
	// Get monitor configuration
	monitor, err := s.monitorRepo.GetByID(ctx, monitorID)
	if err != nil {
		return fmt.Errorf("failed to get monitor: %w", err)
	}

	// Check if monitor has any alert channels configured
	if len(monitor.AlertChannels) == 0 {
		// No alert channels configured, skip alert generation
		return nil
	}

	// Generate downtime alert if health check failed
	if !healthCheck.IsSuccessful() {
		// Check rate limiting before sending downtime alert
		// Requirement 5.2: 15-minute suppression
		// Requirement 5.4: 1-hour reminder for prolonged failures
		if s.rateLimiter != nil {
			shouldSend, err := s.rateLimiter.CheckAndSetDowntimeAlert(ctx, monitorID)
			if err != nil {
				return fmt.Errorf("failed to check rate limit: %w", err)
			}
			if !shouldSend {
				// Alert suppressed by rate limiter
				return nil
			}
		}

		if err := s.GenerateDowntimeAlert(ctx, monitor, healthCheck); err != nil {
			return fmt.Errorf("failed to generate downtime alert: %w", err)
		}
	} else {
		// Check if this is a recovery (previous check failed, current check succeeded)
		// Requirement 5.3: Recovery alerts bypass rate limiting
		var lastDowntimeAlert *time.Time

		if s.rateLimiter != nil {
			lastDowntimeAlert, err = s.rateLimiter.GetLastDowntimeAlert(ctx, monitorID)
			if err != nil {
				return fmt.Errorf("failed to check last downtime alert: %w", err)
			}
		} else {
			// Fallback to database if no rate limiter
			lastDowntimeAlert, err = s.alertRepo.GetLastAlertTime(ctx, monitorID, domain.AlertTypeDowntime)
			if err != nil {
				return fmt.Errorf("failed to check last downtime alert: %w", err)
			}
		}

		// If there was a downtime alert recently (within last hour), generate recovery alert
		if lastDowntimeAlert != nil && time.Since(*lastDowntimeAlert) < 1*time.Hour {
			if err := s.GenerateRecoveryAlert(ctx, monitor, healthCheck); err != nil {
				return fmt.Errorf("failed to generate recovery alert: %w", err)
			}

			// Clear the downtime rate limit on recovery (Requirement 5.3)
			if s.rateLimiter != nil {
				if err := s.rateLimiter.ClearDowntimeAlert(ctx, monitorID); err != nil {
					// Log error but don't fail the recovery alert
					_ = err
				}
			}
		}
	}

	// Generate SSL expiration alerts for HTTPS monitors
	if healthCheck.SSLInfo != nil && healthCheck.SSLInfo.Valid {
		// Check if SSL certificate is expiring soon
		// Requirements: 2.2 (30 days), 2.3 (15 days), 2.4 (7 days), 2.5 (expired)
		if s.shouldGenerateSSLAlert(healthCheck.SSLInfo.DaysUntil) {
			// Check rate limiting for SSL alerts
			// Requirement 5.5: Daily limit for SSL expiration alerts
			if s.rateLimiter != nil {
				shouldSend, err := s.rateLimiter.CheckAndSetSSLAlert(ctx, monitorID, healthCheck.SSLInfo.DaysUntil)
				if err != nil {
					return fmt.Errorf("failed to check SSL rate limit: %w", err)
				}
				if !shouldSend {
					// SSL alert suppressed by rate limiter
					return nil
				}
			}

			if err := s.GenerateSSLExpirationAlert(ctx, monitor, healthCheck.SSLInfo); err != nil {
				return fmt.Errorf("failed to generate SSL expiration alert: %w", err)
			}
		}
	}

	return nil
}

// shouldGenerateSSLAlert determines if an SSL alert should be generated based on days until expiration
func (s *AlertService) shouldGenerateSSLAlert(daysUntil int) bool {
	// Generate alerts at specific thresholds: 30, 15, 7 days, or expired
	return daysUntil <= 0 || daysUntil == 7 || daysUntil == 15 || daysUntil == 30
}

// extractChannelTypes extracts alert channel types from alert channels
func (s *AlertService) extractChannelTypes(channels []domain.AlertChannel) []domain.AlertChannelType {
	types := make([]domain.AlertChannelType, len(channels))
	for i, channel := range channels {
		types[i] = channel.Type
	}
	return types
}

// GetAlertHistory retrieves alert history for a monitor
func (s *AlertService) GetAlertHistory(ctx context.Context, monitorID string, limit int) ([]*domain.Alert, error) {
	return s.alertRepo.GetByMonitorID(ctx, monitorID, limit)
}

// DeliverAlert delivers an alert to all configured channels
// Requirements: 4.1, 4.6 - Multi-channel delivery with failure isolation
func (s *AlertService) DeliverAlert(ctx context.Context, alert *domain.Alert, channels []domain.AlertChannel) error {
	if s.deliveryService == nil {
		return fmt.Errorf("delivery service not configured")
	}

	if len(channels) == 0 {
		// No channels configured, skip delivery
		return nil
	}

	// Deliver to all channels concurrently
	results := s.deliveryService.DeliverAlert(ctx, alert, channels)

	// Check if any deliveries succeeded
	successCount := 0
	var lastError error
	for _, result := range results {
		if result.Success {
			successCount++
		} else {
			lastError = result.Error
		}
	}

	// If no deliveries succeeded, return error
	if successCount == 0 && len(results) > 0 {
		return fmt.Errorf("failed to deliver alert to any channel, last error: %w", lastError)
	}

	return nil
}

// GenerateAndDeliverDowntimeAlert generates and delivers a downtime alert
func (s *AlertService) GenerateAndDeliverDowntimeAlert(ctx context.Context, monitor *domain.Monitor, healthCheck *domain.HealthCheck) error {
	// Generate the alert
	if err := s.GenerateDowntimeAlert(ctx, monitor, healthCheck); err != nil {
		return fmt.Errorf("failed to generate downtime alert: %w", err)
	}

	// Get the latest alert to deliver
	alerts, err := s.alertRepo.GetByMonitorID(ctx, monitor.ID, 1)
	if err != nil {
		return fmt.Errorf("failed to get latest alert: %w", err)
	}

	if len(alerts) == 0 {
		return fmt.Errorf("no alert found after generation")
	}

	// Deliver the alert
	return s.DeliverAlert(ctx, alerts[0], monitor.AlertChannels)
}

// GenerateAndDeliverRecoveryAlert generates and delivers a recovery alert
func (s *AlertService) GenerateAndDeliverRecoveryAlert(ctx context.Context, monitor *domain.Monitor, healthCheck *domain.HealthCheck) error {
	// Generate the alert
	if err := s.GenerateRecoveryAlert(ctx, monitor, healthCheck); err != nil {
		return fmt.Errorf("failed to generate recovery alert: %w", err)
	}

	// Get the latest alert to deliver
	alerts, err := s.alertRepo.GetByMonitorID(ctx, monitor.ID, 1)
	if err != nil {
		return fmt.Errorf("failed to get latest alert: %w", err)
	}

	if len(alerts) == 0 {
		return fmt.Errorf("no alert found after generation")
	}

	// Deliver the alert
	return s.DeliverAlert(ctx, alerts[0], monitor.AlertChannels)
}

// GenerateAndDeliverSSLExpirationAlert generates and delivers an SSL expiration alert
func (s *AlertService) GenerateAndDeliverSSLExpirationAlert(ctx context.Context, monitor *domain.Monitor, sslInfo *domain.SSLInfo) error {
	// Generate the alert
	if err := s.GenerateSSLExpirationAlert(ctx, monitor, sslInfo); err != nil {
		return fmt.Errorf("failed to generate SSL expiration alert: %w", err)
	}

	// Get the latest alert to deliver
	alerts, err := s.alertRepo.GetByMonitorID(ctx, monitor.ID, 1)
	if err != nil {
		return fmt.Errorf("failed to get latest alert: %w", err)
	}

	if len(alerts) == 0 {
		return fmt.Errorf("no alert found after generation")
	}

	// Deliver the alert
	return s.DeliverAlert(ctx, alerts[0], monitor.AlertChannels)
}

// GeneratePerformanceAlert generates an alert when response time exceeds threshold
// Requirements: 8.4
func (s *AlertService) GeneratePerformanceAlert(ctx context.Context, monitor *domain.Monitor, responseTime, threshold time.Duration) error {
	alert := &domain.Alert{
		ID:        uuid.New().String(),
		MonitorID: monitor.ID,
		Type:      domain.AlertTypePerformance,
		Severity:  domain.SeverityWarning,
		Message:   fmt.Sprintf("Monitor '%s' response time (%dms) exceeds threshold (%dms)", monitor.Name, responseTime.Milliseconds(), threshold.Milliseconds()),
		Details: map[string]interface{}{
			"url":           monitor.URL,
			"response_time": responseTime.Milliseconds(),
			"threshold":     threshold.Milliseconds(),
			"checked_at":    time.Now(),
		},
		SentAt:   time.Now(),
		Channels: s.extractChannelTypes(monitor.AlertChannels),
	}

	// Persist alert to database (Requirement 5.1)
	if err := s.alertRepo.Create(ctx, alert); err != nil {
		return fmt.Errorf("failed to persist performance alert: %w", err)
	}

	// Broadcast alert to WebSocket clients (Requirement 9.3)
	if s.websocketBroadcaster != nil {
		s.websocketBroadcaster.BroadcastAlert(alert)
	}

	return nil
}

// GenerateAndDeliverPerformanceAlert generates and delivers a performance alert
func (s *AlertService) GenerateAndDeliverPerformanceAlert(ctx context.Context, monitor *domain.Monitor, responseTime, threshold time.Duration) error {
	// Generate the alert
	if err := s.GeneratePerformanceAlert(ctx, monitor, responseTime, threshold); err != nil {
		return fmt.Errorf("failed to generate performance alert: %w", err)
	}

	// Get the latest alert to deliver
	alerts, err := s.alertRepo.GetByMonitorID(ctx, monitor.ID, 1)
	if err != nil {
		return fmt.Errorf("failed to get latest alert: %w", err)
	}

	if len(alerts) == 0 {
		return fmt.Errorf("no alert found after generation")
	}

	// Deliver the alert
	return s.DeliverAlert(ctx, alerts[0], monitor.AlertChannels)
}
