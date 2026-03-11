package alertchannel

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"html/template"
	"net/smtp"
	"strconv"
	"time"

	"web-tracker/domain"
)

// EmailAdapter sends alerts via SMTP with TLS
type EmailAdapter struct {
	// No HTTP client needed for SMTP
}

// NewEmailAdapter creates a new Email adapter
func NewEmailAdapter() *EmailAdapter {
	return &EmailAdapter{}
}

// Send sends an alert via SMTP with TLS
// Requirements: 4.3, 4.5
func (e *EmailAdapter) Send(ctx context.Context, alert *domain.Alert, config map[string]string) error {
	// Validate required configuration
	smtpHost, ok := config["smtp_host"]
	if !ok || smtpHost == "" {
		return fmt.Errorf("smtp_host is required in config")
	}

	smtpPortStr, ok := config["smtp_port"]
	if !ok || smtpPortStr == "" {
		return fmt.Errorf("smtp_port is required in config")
	}

	smtpPort, err := strconv.Atoi(smtpPortStr)
	if err != nil {
		return fmt.Errorf("invalid smtp_port: %w", err)
	}

	username, ok := config["username"]
	if !ok || username == "" {
		return fmt.Errorf("username is required in config")
	}

	password, ok := config["password"]
	if !ok || password == "" {
		return fmt.Errorf("password is required in config")
	}

	fromEmail, ok := config["from_email"]
	if !ok || fromEmail == "" {
		return fmt.Errorf("from_email is required in config")
	}

	toEmail, ok := config["to_email"]
	if !ok || toEmail == "" {
		return fmt.Errorf("to_email is required in config")
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

		err := e.sendEmail(ctx, smtpHost, smtpPort, username, password, fromEmail, toEmail, alert)
		if err == nil {
			return nil
		}
		lastErr = err
	}

	return fmt.Errorf("failed to send email after 3 attempts: %w", lastErr)
}

// sendEmail sends a single email via SMTP with TLS
func (e *EmailAdapter) sendEmail(ctx context.Context, smtpHost string, smtpPort int, username, password, fromEmail, toEmail string, alert *domain.Alert) error {
	// Create SMTP address
	smtpAddr := fmt.Sprintf("%s:%d", smtpHost, smtpPort)

	// Create TLS config
	tlsConfig := &tls.Config{
		ServerName: smtpHost,
	}

	// Connect to SMTP server with TLS
	conn, err := tls.Dial("tcp", smtpAddr, tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer conn.Close()

	// Create SMTP client
	client, err := smtp.NewClient(conn, smtpHost)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	defer client.Quit()

	// Authenticate
	auth := smtp.PlainAuth("", username, password, smtpHost)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP authentication failed: %w", err)
	}

	// Set sender
	if err := client.Mail(fromEmail); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}

	// Set recipient
	if err := client.Rcpt(toEmail); err != nil {
		return fmt.Errorf("failed to set recipient: %w", err)
	}

	// Get data writer
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to get data writer: %w", err)
	}
	defer writer.Close()

	// Generate email content
	subject := e.generateSubject(alert)
	htmlBody, err := e.generateHTMLBody(alert)
	if err != nil {
		return fmt.Errorf("failed to generate email body: %w", err)
	}

	// Write email headers and body
	message := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		fromEmail, toEmail, subject, htmlBody)

	if _, err := writer.Write([]byte(message)); err != nil {
		return fmt.Errorf("failed to write email message: %w", err)
	}

	return nil
}

// generateSubject generates an email subject based on the alert
func (e *EmailAdapter) generateSubject(alert *domain.Alert) string {
	switch alert.Type {
	case domain.AlertTypeDowntime:
		return fmt.Sprintf("[CRITICAL] Monitor Down - %s", alert.Message)
	case domain.AlertTypeRecovery:
		return fmt.Sprintf("[RECOVERY] Monitor Recovered - %s", alert.Message)
	case domain.AlertTypeSSLExpiring:
		return fmt.Sprintf("[SSL WARNING] Certificate Expiring - %s", alert.Message)
	case domain.AlertTypeSSLExpired:
		return fmt.Sprintf("[SSL CRITICAL] Certificate Expired - %s", alert.Message)
	case domain.AlertTypePerformance:
		return fmt.Sprintf("[PERFORMANCE] Slow Response - %s", alert.Message)
	default:
		return fmt.Sprintf("[ALERT] %s", alert.Message)
	}
}

// generateHTMLBody generates HTML email body using templates
func (e *EmailAdapter) generateHTMLBody(alert *domain.Alert) (string, error) {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>{{.Subject}}</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .alert { padding: 20px; border-radius: 5px; margin-bottom: 20px; }
        .critical { background-color: #ffebee; border-left: 5px solid #f44336; }
        .warning { background-color: #fff3e0; border-left: 5px solid #ff9800; }
        .info { background-color: #e3f2fd; border-left: 5px solid #2196f3; }
        .header { font-size: 18px; font-weight: bold; margin-bottom: 10px; }
        .details { margin-top: 15px; }
        .details table { border-collapse: collapse; width: 100%; }
        .details th, .details td { padding: 8px; text-align: left; border-bottom: 1px solid #ddd; }
        .details th { background-color: #f5f5f5; }
        .timestamp { color: #666; font-size: 12px; margin-top: 15px; }
    </style>
</head>
<body>
    <div class="alert {{.CSSClass}}">
        <div class="header">
            {{.Icon}} {{.Severity}} - {{.Type}}
        </div>
        <p>{{.Message}}</p>
        
        {{if .Details}}
        <div class="details">
            <table>
                {{range $key, $value := .Details}}
                <tr>
                    <th>{{$key}}</th>
                    <td>{{$value}}</td>
                </tr>
                {{end}}
            </table>
        </div>
        {{end}}
        
        <div class="timestamp">
            Alert generated at: {{.Timestamp}}
        </div>
    </div>
</body>
</html>`

	// Prepare template data
	data := struct {
		Subject   string
		Severity  string
		Type      string
		Message   string
		Details   map[string]interface{}
		Timestamp string
		Icon      string
		CSSClass  string
	}{
		Subject:   e.generateSubject(alert),
		Severity:  string(alert.Severity),
		Type:      string(alert.Type),
		Message:   alert.Message,
		Details:   alert.Details,
		Timestamp: alert.SentAt.Format("2006-01-02 15:04:05 MST"),
	}

	// Set icon and CSS class based on severity
	switch alert.Severity {
	case domain.SeverityCritical:
		data.Icon = "🔴"
		data.CSSClass = "critical"
	case domain.SeverityWarning:
		data.Icon = "⚠️"
		data.CSSClass = "warning"
	case domain.SeverityInfo:
		data.Icon = "ℹ️"
		data.CSSClass = "info"
	}

	// Parse and execute template
	t, err := template.New("email").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("failed to parse email template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute email template: %w", err)
	}

	return buf.String(), nil
}
