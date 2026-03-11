package domain

import (
	"time"
)

// Alert represents a notification sent when a Monitor fails or SSL certificate is expiring
type Alert struct {
	ID        string
	MonitorID string
	Type      AlertType
	Severity  AlertSeverity
	Message   string
	Details   map[string]interface{}
	SentAt    time.Time
	Channels  []AlertChannelType
}

// AlertType defines the type of alert
type AlertType string

const (
	AlertTypeDowntime    AlertType = "downtime"
	AlertTypeRecovery    AlertType = "recovery"
	AlertTypeSSLExpiring AlertType = "ssl_expiring"
	AlertTypeSSLExpired  AlertType = "ssl_expired"
	AlertTypePerformance AlertType = "performance"
)

// AlertSeverity defines the severity level of an alert
type AlertSeverity string

const (
	SeverityInfo     AlertSeverity = "info"
	SeverityWarning  AlertSeverity = "warning"
	SeverityCritical AlertSeverity = "critical"
)

// DetermineSSLAlertSeverity determines the severity based on days until expiration
func DetermineSSLAlertSeverity(daysUntil int) AlertSeverity {
	if daysUntil <= 0 {
		return SeverityCritical
	}
	if daysUntil <= 7 {
		return SeverityCritical
	}
	if daysUntil <= 30 {
		return SeverityWarning
	}
	return SeverityInfo
}

// DetermineSSLAlertType determines the alert type based on days until expiration
func DetermineSSLAlertType(daysUntil int) AlertType {
	if daysUntil <= 0 {
		return AlertTypeSSLExpired
	}
	return AlertTypeSSLExpiring
}
