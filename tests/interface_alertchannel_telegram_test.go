package tests

import (
	"context"
	"testing"
	"time"

	"web-tracker/domain"
	"web-tracker/interface/alertchannel"
)

func TestTelegramAdapter_Send_MissingBotToken(t *testing.T) {
	adapter := alertchannel.NewTelegramAdapter()

	alert := &domain.Alert{
		ID:       "alert-1",
		Type:     domain.AlertTypeDowntime,
		Severity: domain.SeverityCritical,
		Message:  "Test alert",
		SentAt:   time.Now(),
	}

	config := map[string]string{
		"chat_id": "123456",
	}

	ctx := context.Background()
	err := adapter.Send(ctx, alert, config)

	if err == nil {
		t.Error("expected error for missing bot_token, got nil")
	}
	if err.Error() != "telegram bot_token is required in config" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestTelegramAdapter_Send_MissingChatID(t *testing.T) {
	adapter := alertchannel.NewTelegramAdapter()

	alert := &domain.Alert{
		ID:       "alert-1",
		Type:     domain.AlertTypeDowntime,
		Severity: domain.SeverityCritical,
		Message:  "Test alert",
		SentAt:   time.Now(),
	}

	config := map[string]string{
		"bot_token": "test-token",
	}

	ctx := context.Background()
	err := adapter.Send(ctx, alert, config)

	if err == nil {
		t.Error("expected error for missing chat_id, got nil")
	}
	if err.Error() != "telegram chat_id is required in config" {
		t.Errorf("unexpected error message: %v", err)
	}
}

// formatMessage is unexported; validation relies on Send behavior in integration tests.
