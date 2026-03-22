package tests

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"web-tracker/domain"
	"web-tracker/interface/alertchannel"
)

// MockAdapter is a test adapter that can be configured to succeed or fail
type MockAdapter struct {
	shouldFail bool
	callCount  int
}

func (m *MockAdapter) Send(ctx context.Context, alert *domain.Alert, config map[string]string) error {
	m.callCount++
	if m.shouldFail {
		return errors.New("mock adapter failure")
	}
	return nil
}

func TestDeliveryService_DeliverAlert_Success(t *testing.T) {
	service := alertchannel.NewDeliveryService()

	// Register mock adapters
	mockTelegram := &MockAdapter{shouldFail: false}
	mockEmail := &MockAdapter{shouldFail: false}
	mockWebhook := &MockAdapter{shouldFail: false}

	service.RegisterAdapter(domain.AlertChannelTelegram, mockTelegram)
	service.RegisterAdapter(domain.AlertChannelEmail, mockEmail)
	service.RegisterAdapter(domain.AlertChannelWebhook, mockWebhook)

	alert := &domain.Alert{
		ID:        "alert-1",
		MonitorID: "monitor-1",
		Type:      domain.AlertTypeDowntime,
		Severity:  domain.SeverityCritical,
		Message:   "Monitor is down",
		SentAt:    time.Now(),
	}

	channels := []domain.AlertChannel{
		{
			Type: domain.AlertChannelTelegram,
			Config: map[string]string{
				"bot_token": "test-token",
				"chat_id":   "123456",
			},
		},
		{
			Type: domain.AlertChannelEmail,
			Config: map[string]string{
				"smtp_host":  "smtp.example.com",
				"smtp_port":  "587",
				"username":   "user@example.com",
				"password":   "password",
				"from_email": "alerts@example.com",
				"to_email":   "admin@example.com",
			},
		},
		{
			Type: domain.AlertChannelWebhook,
			Config: map[string]string{
				"webhook_url": "https://webhook.example.com",
			},
		},
	}

	ctx := context.Background()
	results := service.DeliverAlert(ctx, alert, channels)

	// Verify all deliveries succeeded
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}

	successCount := 0
	for _, result := range results {
		if result.Success {
			successCount++
		} else {
			t.Errorf("delivery to %s failed: %v", result.Channel, result.Error)
		}
	}

	if successCount != 3 {
		t.Errorf("expected 3 successful deliveries, got %d", successCount)
	}

	// Verify all adapters were called
	if mockTelegram.callCount != 1 {
		t.Errorf("expected telegram adapter to be called once, got %d", mockTelegram.callCount)
	}
	if mockEmail.callCount != 1 {
		t.Errorf("expected email adapter to be called once, got %d", mockEmail.callCount)
	}
	if mockWebhook.callCount != 1 {
		t.Errorf("expected webhook adapter to be called once, got %d", mockWebhook.callCount)
	}
}

func TestDeliveryService_DeliverAlert_PartialFailure(t *testing.T) {
	service := alertchannel.NewDeliveryService()

	// Register mock adapters - telegram fails, others succeed
	mockTelegram := &MockAdapter{shouldFail: true}
	mockEmail := &MockAdapter{shouldFail: false}
	mockWebhook := &MockAdapter{shouldFail: false}

	service.RegisterAdapter(domain.AlertChannelTelegram, mockTelegram)
	service.RegisterAdapter(domain.AlertChannelEmail, mockEmail)
	service.RegisterAdapter(domain.AlertChannelWebhook, mockWebhook)

	alert := &domain.Alert{
		ID:        "alert-1",
		MonitorID: "monitor-1",
		Type:      domain.AlertTypeDowntime,
		Severity:  domain.SeverityCritical,
		Message:   "Monitor is down",
		SentAt:    time.Now(),
	}

	channels := []domain.AlertChannel{
		{
			Type: domain.AlertChannelTelegram,
			Config: map[string]string{
				"bot_token": "test-token",
				"chat_id":   "123456",
			},
		},
		{
			Type: domain.AlertChannelEmail,
			Config: map[string]string{
				"smtp_host":  "smtp.example.com",
				"smtp_port":  "587",
				"username":   "user@example.com",
				"password":   "password",
				"from_email": "alerts@example.com",
				"to_email":   "admin@example.com",
			},
		},
		{
			Type: domain.AlertChannelWebhook,
			Config: map[string]string{
				"webhook_url": "https://webhook.example.com",
			},
		},
	}

	ctx := context.Background()
	results := service.DeliverAlert(ctx, alert, channels)

	// Verify results - should have 1 failure and 2 successes
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}

	successCount := 0
	failureCount := 0
	for _, result := range results {
		if result.Success {
			successCount++
		} else {
			failureCount++
			// Verify the failure is from telegram
			if result.Channel == domain.AlertChannelTelegram {
				if result.Error == nil {
					t.Error("expected error for failed telegram delivery")
				}
			} else {
				t.Errorf("unexpected failure from channel %s: %v", result.Channel, result.Error)
			}
		}
	}

	if successCount != 2 {
		t.Errorf("expected 2 successful deliveries, got %d", successCount)
	}
	if failureCount != 1 {
		t.Errorf("expected 1 failed delivery, got %d", failureCount)
	}

	// Verify all adapters were called (isolation requirement 4.6)
	if mockTelegram.callCount != 1 {
		t.Errorf("expected telegram adapter to be called once, got %d", mockTelegram.callCount)
	}
	if mockEmail.callCount != 1 {
		t.Errorf("expected email adapter to be called once, got %d", mockEmail.callCount)
	}
	if mockWebhook.callCount != 1 {
		t.Errorf("expected webhook adapter to be called once, got %d", mockWebhook.callCount)
	}
}

func TestDeliveryService_DeliverAlert_EmptyChannels(t *testing.T) {
	service := alertchannel.NewDeliveryService()

	alert := &domain.Alert{
		ID:        "alert-1",
		MonitorID: "monitor-1",
		Type:      domain.AlertTypeDowntime,
		Severity:  domain.SeverityCritical,
		Message:   "Monitor is down",
		SentAt:    time.Now(),
	}

	ctx := context.Background()
	results := service.DeliverAlert(ctx, alert, []domain.AlertChannel{})

	if len(results) != 0 {
		t.Errorf("expected 0 results for empty channels, got %d", len(results))
	}
}

func TestDeliveryService_DeliverAlert_UnsupportedChannel(t *testing.T) {
	service := alertchannel.NewDeliveryService()

	alert := &domain.Alert{
		ID:        "alert-1",
		MonitorID: "monitor-1",
		Type:      domain.AlertTypeDowntime,
		Severity:  domain.SeverityCritical,
		Message:   "Monitor is down",
		SentAt:    time.Now(),
	}

	// Use an unsupported channel type
	channels := []domain.AlertChannel{
		{
			Type:   domain.AlertChannelType("unsupported"),
			Config: map[string]string{},
		},
	}

	ctx := context.Background()
	results := service.DeliverAlert(ctx, alert, channels)

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}

	result := results[0]
	if result.Success {
		t.Error("expected failure for unsupported channel")
	}
	if result.Error == nil {
		t.Error("expected error for unsupported channel")
	}
	if !strings.Contains(result.Error.Error(), "no adapter found") {
		t.Errorf("expected 'no adapter found' error, got: %v", result.Error)
	}
}

func TestDeliveryService_GetSupportedChannels(t *testing.T) {
	service := alertchannel.NewDeliveryService()
	channels := service.GetSupportedChannels()

	expectedChannels := []domain.AlertChannelType{
		domain.AlertChannelTelegram,
		domain.AlertChannelEmail,
		domain.AlertChannelWebhook,
	}

	if len(channels) != len(expectedChannels) {
		t.Errorf("expected %d supported channels, got %d", len(expectedChannels), len(channels))
	}

	// Check that all expected channels are present
	channelMap := make(map[domain.AlertChannelType]bool)
	for _, channel := range channels {
		channelMap[channel] = true
	}

	for _, expected := range expectedChannels {
		if !channelMap[expected] {
			t.Errorf("expected channel %s to be supported", expected)
		}
	}
}

func TestDeliveryService_RegisterAdapter(t *testing.T) {
	service := alertchannel.NewDeliveryService()
	mockAdapter := &MockAdapter{shouldFail: false}

	// Register a custom adapter
	customChannelType := domain.AlertChannelType("custom")
	service.RegisterAdapter(customChannelType, mockAdapter)

	// Verify it's now supported
	channels := service.GetSupportedChannels()
	found := false
	for _, channel := range channels {
		if channel == customChannelType {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected custom channel to be supported after registration")
	}

	// Test delivery with custom adapter
	alert := &domain.Alert{
		ID:        "alert-1",
		MonitorID: "monitor-1",
		Type:      domain.AlertTypeDowntime,
		Severity:  domain.SeverityCritical,
		Message:   "Monitor is down",
		SentAt:    time.Now(),
	}

	channels_config := []domain.AlertChannel{
		{
			Type:   customChannelType,
			Config: map[string]string{},
		},
	}

	ctx := context.Background()
	results := service.DeliverAlert(ctx, alert, channels_config)

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}

	result := results[0]
	if !result.Success {
		t.Errorf("expected success for custom adapter, got error: %v", result.Error)
	}

	if mockAdapter.callCount != 1 {
		t.Errorf("expected custom adapter to be called once, got %d", mockAdapter.callCount)
	}
}
