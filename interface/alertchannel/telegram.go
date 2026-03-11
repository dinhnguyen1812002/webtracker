package alertchannel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"web-tracker/domain"
)

// TelegramAdapter sends alerts via Telegram Bot API
type TelegramAdapter struct {
	httpClient *http.Client
}

// NewTelegramAdapter creates a new Telegram adapter
func NewTelegramAdapter() *TelegramAdapter {
	return &TelegramAdapter{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Send sends an alert via Telegram Bot API
// Requirements: 4.2, 4.5
func (t *TelegramAdapter) Send(ctx context.Context, alert *domain.Alert, config map[string]string) error {
	botToken, ok := config["bot_token"]
	if !ok || botToken == "" {
		return fmt.Errorf("telegram bot_token is required in config")
	}

	chatID, ok := config["chat_id"]
	if !ok || chatID == "" {
		return fmt.Errorf("telegram chat_id is required in config")
	}

	// Format message with Markdown
	message := t.formatMessage(alert)

	// Implement retry logic with exponential backoff (3 attempts)
	// Requirement 4.5
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		err := t.sendMessage(ctx, botToken, chatID, message)
		if err == nil {
			return nil
		}
		lastErr = err
	}

	return fmt.Errorf("failed to send telegram message after 3 attempts: %w", lastErr)
}

// sendMessage sends a single message to Telegram Bot API
func (t *TelegramAdapter) sendMessage(ctx context.Context, botToken, chatID, message string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)

	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       message,
		"parse_mode": "Markdown",
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal telegram payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create telegram request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send telegram request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// formatMessage formats an alert into a Markdown message for Telegram
func (t *TelegramAdapter) formatMessage(alert *domain.Alert) string {
	var buf bytes.Buffer

	// Add severity emoji
	switch alert.Severity {
	case domain.SeverityCritical:
		buf.WriteString("🔴 *CRITICAL*\n")
	case domain.SeverityWarning:
		buf.WriteString("⚠️ *WARNING*\n")
	case domain.SeverityInfo:
		buf.WriteString("ℹ️ *INFO*\n")
	}

	// Add alert type
	buf.WriteString(fmt.Sprintf("*Type:* %s\n", alert.Type))

	// Add message
	buf.WriteString(fmt.Sprintf("\n%s\n", alert.Message))

	// Add details
	if len(alert.Details) > 0 {
		buf.WriteString("\n*Details:*\n")
		if url, ok := alert.Details["url"].(string); ok {
			buf.WriteString(fmt.Sprintf("URL: %s\n", url))
		}
		if statusCode, ok := alert.Details["status_code"].(int); ok {
			buf.WriteString(fmt.Sprintf("Status Code: %d\n", statusCode))
		}
		if errorMsg, ok := alert.Details["error_message"].(string); ok && errorMsg != "" {
			buf.WriteString(fmt.Sprintf("Error: %s\n", errorMsg))
		}
		if responseTime, ok := alert.Details["response_time"].(int64); ok {
			buf.WriteString(fmt.Sprintf("Response Time: %dms\n", responseTime))
		}
		if daysUntil, ok := alert.Details["days_until"].(int); ok {
			buf.WriteString(fmt.Sprintf("Days Until Expiration: %d\n", daysUntil))
		}
		if issuer, ok := alert.Details["issuer"].(string); ok && issuer != "" {
			buf.WriteString(fmt.Sprintf("Issuer: %s\n", issuer))
		}
	}

	// Add timestamp
	buf.WriteString(fmt.Sprintf("\n*Time:* %s", alert.SentAt.Format("2006-01-02 15:04:05 MST")))

	return buf.String()
}
