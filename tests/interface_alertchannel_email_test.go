package tests

import (
	"context"
	"strings"
	"testing"
	"time"

	"web-tracker/domain"
	"web-tracker/interface/alertchannel"
)

func TestEmailAdapter_Send_MissingConfig(t *testing.T) {
	adapter := alertchannel.NewEmailAdapter()

	alert := &domain.Alert{
		ID:       "alert-1",
		Type:     domain.AlertTypeDowntime,
		Severity: domain.SeverityCritical,
		Message:  "Test alert",
		SentAt:   time.Now(),
	}

	ctx := context.Background()

	tests := []struct {
		name        string
		config      map[string]string
		expectedErr string
	}{
		{
			name:        "missing smtp_host",
			config:      map[string]string{},
			expectedErr: "smtp_host is required in config",
		},
		{
			name: "missing smtp_port",
			config: map[string]string{
				"smtp_host": "smtp.example.com",
			},
			expectedErr: "smtp_port is required in config",
		},
		{
			name: "invalid smtp_port",
			config: map[string]string{
				"smtp_host": "smtp.example.com",
				"smtp_port": "invalid",
			},
			expectedErr: "invalid smtp_port",
		},
		{
			name: "missing username",
			config: map[string]string{
				"smtp_host": "smtp.example.com",
				"smtp_port": "587",
			},
			expectedErr: "username is required in config",
		},
		{
			name: "missing password",
			config: map[string]string{
				"smtp_host": "smtp.example.com",
				"smtp_port": "587",
				"username":  "user@example.com",
			},
			expectedErr: "password is required in config",
		},
		{
			name: "missing from_email",
			config: map[string]string{
				"smtp_host": "smtp.example.com",
				"smtp_port": "587",
				"username":  "user@example.com",
				"password":  "password",
			},
			expectedErr: "from_email is required in config",
		},
		{
			name: "missing to_email",
			config: map[string]string{
				"smtp_host":  "smtp.example.com",
				"smtp_port":  "587",
				"username":   "user@example.com",
				"password":   "password",
				"from_email": "alerts@example.com",
			},
			expectedErr: "to_email is required in config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := adapter.Send(ctx, alert, tt.config)
			if err == nil {
				t.Errorf("expected error for %s, got nil", tt.name)
			}
			if err.Error() != tt.expectedErr && !strings.Contains(err.Error(), tt.expectedErr) {
				t.Errorf("expected error containing '%s', got '%s'", tt.expectedErr, err.Error())
			}
		})
	}
}

// generateSubject and generateHTMLBody are unexported; validation relies on Send behavior in other tests.
