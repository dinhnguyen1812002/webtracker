package alertchannel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"web-tracker/domain"
)

// WebhookAdapter sends alerts via HTTP POST with JSON payload
type WebhookAdapter struct {
	httpClient *http.Client
}

// NewWebhookAdapter creates a new Webhook adapter
func NewWebhookAdapter() *WebhookAdapter {
	return &WebhookAdapter{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Send sends an alert via HTTP POST with JSON payload
// Requirements: 4.4, 4.5
func (w *WebhookAdapter) Send(ctx context.Context, alert *domain.Alert, config map[string]string) error {
	// Validate required configuration
	webhookURL, ok := config["webhook_url"]
	if !ok || webhookURL == "" {
		return fmt.Errorf("webhook_url is required in config")
	}

	// Implement retry logic with exponential backoff (3 attempts)
	// Requirement 4.5
	var lastErr error
	for attempt := range 3 {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		err := w.sendWebhook(ctx, webhookURL, alert, config)
		if err == nil {
			return nil
		}
		lastErr = err
	}

	return fmt.Errorf("failed to send webhook after 3 attempts: %w", lastErr)
}

// sendWebhook sends a single webhook request
func (w *WebhookAdapter) sendWebhook(ctx context.Context, webhookURL string, alert *domain.Alert, config map[string]string) error {
	// Create JSON payload
	payload := w.createPayload(alert)

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}

	// Set default headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "uptime-monitor/1.0")

	// Support custom headers (Requirement 4.4)
	for key, value := range config {
		if strings.HasPrefix(key, "header_") {
			headerName := strings.TrimPrefix(key, "header_")
			req.Header.Set(headerName, value)
		}
	}

	// Send request
	resp, err := w.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// createPayload creates the JSON payload for the webhook
func (w *WebhookAdapter) createPayload(alert *domain.Alert) map[string]interface{} {
	payload := map[string]interface{}{
		"id":         alert.ID,
		"monitor_id": alert.MonitorID,
		"type":       string(alert.Type),
		"severity":   string(alert.Severity),
		"message":    alert.Message,
		"sent_at":    alert.SentAt.Format(time.RFC3339),
		"channels":   alert.Channels,
	}

	// Add details if present
	if alert.Details != nil && len(alert.Details) > 0 {
		payload["details"] = alert.Details
	}

	// Add formatted timestamp for human readability
	payload["timestamp"] = alert.SentAt.Format("2006-01-02 15:04:05 MST")

	return payload
}
