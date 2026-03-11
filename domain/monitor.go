package domain

import (
	"errors"
	"net/url"
	"time"
)

// Monitor represents a website or server endpoint to be monitored
type Monitor struct {
	ID            string
	Name          string
	URL           string
	CheckInterval time.Duration // 1m, 5m, 15m, 60m
	Enabled       bool
	AlertChannels []AlertChannel
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// AlertChannel represents a delivery mechanism for alerts
type AlertChannel struct {
	Type   AlertChannelType
	Config map[string]string // Channel-specific configuration
}

// AlertChannelType defines the type of alert channel
type AlertChannelType string

const (
	AlertChannelTelegram AlertChannelType = "telegram"
	AlertChannelEmail    AlertChannelType = "email"
	AlertChannelWebhook  AlertChannelType = "webhook"
)

// Valid check intervals
var (
	CheckInterval1Min  = 1 * time.Minute
	CheckInterval5Min  = 5 * time.Minute
	CheckInterval15Min = 15 * time.Minute
	CheckInterval60Min = 60 * time.Minute
)

// Validate validates the monitor configuration
func (m *Monitor) Validate() error {
	if m.Name == "" {
		return errors.New("monitor name is required")
	}

	if err := ValidateURL(m.URL); err != nil {
		return err
	}

	if err := ValidateCheckInterval(m.CheckInterval); err != nil {
		return err
	}

	return nil
}

// ValidateURL validates that the URL is a valid HTTP/HTTPS endpoint
func ValidateURL(urlStr string) error {
	if urlStr == "" {
		return errors.New("URL is required")
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return errors.New("invalid URL format")
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return errors.New("URL must use http or https scheme")
	}

	if parsedURL.Host == "" {
		return errors.New("URL must have a valid host")
	}

	return nil
}

// ValidateCheckInterval validates that the check interval is one of the allowed values
func ValidateCheckInterval(interval time.Duration) error {
	switch interval {
	case CheckInterval1Min, CheckInterval5Min, CheckInterval15Min, CheckInterval60Min:
		return nil
	default:
		return errors.New("check interval must be 1, 5, 15, or 60 minutes")
	}
}
