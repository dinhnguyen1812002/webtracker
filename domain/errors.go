package domain

import "errors"

// Common domain errors
var (
	// ErrMonitorNotFound is returned when a monitor is not found
	ErrMonitorNotFound = errors.New("monitor not found")

	// ErrHealthCheckNotFound is returned when a health check is not found
	ErrHealthCheckNotFound = errors.New("health check not found")

	// ErrAlertNotFound is returned when an alert is not found
	ErrAlertNotFound = errors.New("alert not found")

	// ErrInvalidMonitorConfig is returned when monitor configuration is invalid
	ErrInvalidMonitorConfig = errors.New("invalid monitor configuration")

	// ErrInvalidURL is returned when a URL is invalid
	ErrInvalidURL = errors.New("invalid URL")

	// ErrInvalidCheckInterval is returned when check interval is invalid
	ErrInvalidCheckInterval = errors.New("invalid check interval")
)
