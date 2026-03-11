package domain

import (
	"context"
	"time"
)

// MonitorRepository defines the interface for monitor persistence
type MonitorRepository interface {
	Create(ctx context.Context, monitor *Monitor) error
	GetByID(ctx context.Context, id string) (*Monitor, error)
	List(ctx context.Context, filters ListFilters) ([]*Monitor, error)
	Update(ctx context.Context, monitor *Monitor) error
	Delete(ctx context.Context, id string) error
}

// HealthCheckRepository defines the interface for health check persistence
type HealthCheckRepository interface {
	Create(ctx context.Context, check *HealthCheck) error
	GetByMonitorID(ctx context.Context, monitorID string, limit int) ([]*HealthCheck, error)
	GetByDateRange(ctx context.Context, monitorID string, start, end time.Time) ([]*HealthCheck, error)
	DeleteOlderThan(ctx context.Context, before time.Time) error
}

// AggregatedHealthCheckRepository defines the interface for aggregated health check persistence
type AggregatedHealthCheckRepository interface {
	Create(ctx context.Context, aggregated *AggregatedHealthCheck) error
	GetByMonitorID(ctx context.Context, monitorID string, limit int) ([]*AggregatedHealthCheck, error)
	GetByDateRange(ctx context.Context, monitorID string, start, end time.Time) ([]*AggregatedHealthCheck, error)
	DeleteOlderThan(ctx context.Context, before time.Time) error
}

// AlertRepository defines the interface for alert persistence
type AlertRepository interface {
	Create(ctx context.Context, alert *Alert) error
	GetByMonitorID(ctx context.Context, monitorID string, limit int) ([]*Alert, error)
	GetByDateRange(ctx context.Context, monitorID string, start, end time.Time) ([]*Alert, error)
	GetLastAlertTime(ctx context.Context, monitorID string, alertType AlertType) (*time.Time, error)
	DeleteOlderThan(ctx context.Context, before time.Time) error
}

// UserRepository defines the interface for user persistence
type UserRepository interface {
	Create(ctx context.Context, user *User) error
	GetByID(ctx context.Context, id string) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
}

// SessionRepository defines the interface for session persistence
type SessionRepository interface {
	Create(ctx context.Context, session *Session) error
	GetByID(ctx context.Context, id string) (*Session, error)
	DeleteByID(ctx context.Context, id string) error
	DeleteByUserID(ctx context.Context, userID string) error
	DeleteExpired(ctx context.Context) error
}

// ListFilters defines filters for listing monitors
type ListFilters struct {
	Enabled *bool
	Limit   int
	Offset  int
}
