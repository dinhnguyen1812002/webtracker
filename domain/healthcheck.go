package domain

import (
	"time"
)

// HealthCheck represents a single execution of monitoring logic against a Monitor
type HealthCheck struct {
	ID           string
	MonitorID    string
	Status       HealthCheckStatus
	StatusCode   int
	ResponseTime time.Duration
	SSLInfo      *SSLInfo
	ErrorMessage string
	CheckedAt    time.Time
}

// HealthCheckStatus represents the status of a health check
type HealthCheckStatus string

const (
	StatusSuccess HealthCheckStatus = "success"
	StatusFailure HealthCheckStatus = "failure"
	StatusTimeout HealthCheckStatus = "timeout"
)

// SSLInfo contains SSL certificate information
type SSLInfo struct {
	Valid     bool
	ExpiresAt time.Time
	DaysUntil int
	Issuer    string
}

// IsSuccessful returns true if the health check was successful
func (h *HealthCheck) IsSuccessful() bool {
	return h.Status == StatusSuccess
}

// CalculateSSLDaysUntilExpiry calculates days until SSL certificate expiration
func CalculateSSLDaysUntilExpiry(expiresAt time.Time) int {
	duration := time.Until(expiresAt)
	days := int(duration.Hours() / 24)
	return days
}

// AggregatedHealthCheck represents hourly aggregated health check data
type AggregatedHealthCheck struct {
	ID               string
	MonitorID        string
	HourTimestamp    time.Time // Start of the hour
	TotalChecks      int
	SuccessfulChecks int
	FailedChecks     int
	SuccessRate      float64       // Percentage of successful checks
	AvgResponseTime  time.Duration // Average response time for successful checks
	MinResponseTime  time.Duration // Minimum response time
	MaxResponseTime  time.Duration // Maximum response time
	CreatedAt        time.Time
}

// CalculateSuccessRate calculates the success rate percentage
func (a *AggregatedHealthCheck) CalculateSuccessRate() {
	if a.TotalChecks > 0 {
		a.SuccessRate = float64(a.SuccessfulChecks) / float64(a.TotalChecks) * 100
	} else {
		a.SuccessRate = 0
	}
}
