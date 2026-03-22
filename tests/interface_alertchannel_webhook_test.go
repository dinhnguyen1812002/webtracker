package tests

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"web-tracker/domain"
	"web-tracker/interface/alertchannel"
)

type webhookRoundTripFunc func(*http.Request) (*http.Response, error)

func (f webhookRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newWebhookTestAdapter(fn webhookRoundTripFunc) *alertchannel.WebhookAdapter {
	return alertchannel.NewWebhookAdapterWithClient(&http.Client{
		Transport: fn,
	})
}

func TestWebhookAdapter_Send_Success(t *testing.T) {
	adapter := newWebhookTestAdapter(func(r *http.Request) (*http.Response, error) {
		// Verify request method and content type
		if r.Method != "POST" {
			t.Errorf("expected POST request, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("User-Agent") != "uptime-monitor/1.0" {
			t.Errorf("expected User-Agent uptime-monitor/1.0, got %s", r.Header.Get("User-Agent"))
		}

		// Verify custom headers
		if r.Header.Get("X-Custom-Header") != "custom-value" {
			t.Errorf("expected X-Custom-Header custom-value, got %s", r.Header.Get("X-Custom-Header"))
		}

		// Verify payload
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("failed to decode payload: %v", err)
		}

		// Check required fields
		if payload["id"] != "alert-1" {
			t.Errorf("expected id alert-1, got %v", payload["id"])
		}
		if payload["monitor_id"] != "monitor-1" {
			t.Errorf("expected monitor_id monitor-1, got %v", payload["monitor_id"])
		}
		if payload["type"] != "downtime" {
			t.Errorf("expected type downtime, got %v", payload["type"])
		}
		if payload["severity"] != "critical" {
			t.Errorf("expected severity critical, got %v", payload["severity"])
		}
		if payload["message"] != "Monitor is down" {
			t.Errorf("expected message 'Monitor is down', got %v", payload["message"])
		}

		// Check details
		details, ok := payload["details"].(map[string]interface{})
		if !ok {
			t.Error("expected details to be a map")
		} else {
			if details["url"] != "https://example.com" {
				t.Errorf("expected details.url https://example.com, got %v", details["url"])
			}
		}

		// Return success response
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"status":"received"}`)),
			Header:     make(http.Header),
		}, nil
	})

	alert := &domain.Alert{
		ID:        "alert-1",
		MonitorID: "monitor-1",
		Type:      domain.AlertTypeDowntime,
		Severity:  domain.SeverityCritical,
		Message:   "Monitor is down",
		Details: map[string]interface{}{
			"url":         "https://example.com",
			"status_code": 500,
		},
		SentAt:   time.Now(),
		Channels: []domain.AlertChannelType{domain.AlertChannelWebhook},
	}

	config := map[string]string{
		"webhook_url":            "https://webhook.example.com/alerts",
		"header_X-Custom-Header": "custom-value",
	}

	ctx := context.Background()
	err := adapter.Send(ctx, alert, config)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWebhookAdapter_Send_MissingURL(t *testing.T) {
	adapter := alertchannel.NewWebhookAdapter()

	alert := &domain.Alert{
		ID:       "alert-1",
		Type:     domain.AlertTypeDowntime,
		Severity: domain.SeverityCritical,
		Message:  "Test alert",
		SentAt:   time.Now(),
	}

	config := map[string]string{}

	ctx := context.Background()
	err := adapter.Send(ctx, alert, config)

	if err == nil {
		t.Error("expected error for missing webhook_url, got nil")
	}
	if err.Error() != "webhook_url is required in config" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestWebhookAdapter_Send_ServerError(t *testing.T) {
	var attemptCount atomic.Int32
	adapter := newWebhookTestAdapter(func(r *http.Request) (*http.Response, error) {
		attemptCount.Add(1)
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		}, nil
	})

	alert := &domain.Alert{
		ID:       "alert-1",
		Type:     domain.AlertTypeDowntime,
		Severity: domain.SeverityCritical,
		Message:  "Test alert",
		SentAt:   time.Now(),
	}

	config := map[string]string{
		"webhook_url": "https://webhook.example.com/alerts",
	}

	ctx := context.Background()
	err := adapter.Send(ctx, alert, config)

	if err == nil {
		t.Error("expected error for server error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to send webhook after 3 attempts") {
		t.Errorf("expected retry error message, got: %v", err)
	}
	if attemptCount.Load() != 3 {
		t.Errorf("expected 3 attempts, got %d", attemptCount.Load())
	}
}

// createPayload is unexported; payload is validated via Send success test.
