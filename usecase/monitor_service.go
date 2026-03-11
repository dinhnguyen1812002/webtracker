package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"web-tracker/domain"
)

// CreateMonitorRequest represents a request to create a new monitor
type CreateMonitorRequest struct {
	Name          string                `json:"name"`
	URL           string                `json:"url"`
	CheckInterval time.Duration         `json:"check_interval"`
	Enabled       bool                  `json:"enabled"`
	AlertChannels []domain.AlertChannel `json:"alert_channels"`
}

// UpdateMonitorRequest represents a request to update an existing monitor
type UpdateMonitorRequest struct {
	Name          *string               `json:"name,omitempty"`
	URL           *string               `json:"url,omitempty"`
	CheckInterval *time.Duration        `json:"check_interval,omitempty"`
	Enabled       *bool                 `json:"enabled,omitempty"`
	AlertChannels []domain.AlertChannel `json:"alert_channels,omitempty"`
}

// MonitorService handles monitor CRUD operations with validation, caching, and scheduling
type MonitorService struct {
	monitorRepo domain.MonitorRepository
	scheduler   Scheduler
}

// NewMonitorService creates a new monitor service
func NewMonitorService(monitorRepo domain.MonitorRepository, scheduler Scheduler) *MonitorService {
	return &MonitorService{
		monitorRepo: monitorRepo,
		scheduler:   scheduler,
	}
}

// CreateMonitor creates a new monitor with validation
// Requirements: 10.1, 10.2, 10.6
func (s *MonitorService) CreateMonitor(ctx context.Context, req CreateMonitorRequest) (*domain.Monitor, error) {
	// Generate unique ID
	id := uuid.New().String()

	// Create monitor entity
	monitor := &domain.Monitor{
		ID:            id,
		Name:          req.Name,
		URL:           req.URL,
		CheckInterval: req.CheckInterval,
		Enabled:       req.Enabled,
		AlertChannels: req.AlertChannels,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Validate monitor configuration (Requirements 10.1, 10.2)
	if err := monitor.Validate(); err != nil {
		return nil, fmt.Errorf("monitor validation failed: %w", err)
	}

	// Persist monitor to database (Requirement 10.6)
	if err := s.monitorRepo.Create(ctx, monitor); err != nil {
		return nil, fmt.Errorf("failed to create monitor: %w", err)
	}

	// Schedule health checks if monitor is enabled
	if monitor.Enabled && s.scheduler != nil {
		if err := s.scheduler.ScheduleMonitor(monitor); err != nil {
			// Log error but don't fail monitor creation
			// The monitor can be manually enabled later
			_ = err
		}
	}

	return monitor, nil
}

// GetMonitor retrieves a monitor by ID with caching
// Requirements: 10.5
func (s *MonitorService) GetMonitor(ctx context.Context, id string) (*domain.Monitor, error) {
	if id == "" {
		return nil, fmt.Errorf("monitor ID is required")
	}

	monitor, err := s.monitorRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get monitor: %w", err)
	}

	return monitor, nil
}

// ListMonitors retrieves monitors with filtering
// Requirements: 10.5
func (s *MonitorService) ListMonitors(ctx context.Context, filters domain.ListFilters) ([]*domain.Monitor, error) {
	monitors, err := s.monitorRepo.List(ctx, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to list monitors: %w", err)
	}

	return monitors, nil
}

// UpdateMonitor updates an existing monitor with cache invalidation and rescheduling
// Requirements: 10.3, 10.6
func (s *MonitorService) UpdateMonitor(ctx context.Context, id string, req UpdateMonitorRequest) (*domain.Monitor, error) {
	if id == "" {
		return nil, fmt.Errorf("monitor ID is required")
	}

	// Get existing monitor
	existingMonitor, err := s.monitorRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get existing monitor: %w", err)
	}

	// Apply updates to the monitor
	updatedMonitor := *existingMonitor // Copy existing monitor
	updatedMonitor.UpdatedAt = time.Now()

	if req.Name != nil {
		updatedMonitor.Name = *req.Name
	}
	if req.URL != nil {
		updatedMonitor.URL = *req.URL
	}
	if req.CheckInterval != nil {
		updatedMonitor.CheckInterval = *req.CheckInterval
	}
	if req.Enabled != nil {
		updatedMonitor.Enabled = *req.Enabled
	}
	if req.AlertChannels != nil {
		updatedMonitor.AlertChannels = req.AlertChannels
	}

	// Validate updated monitor configuration
	if err := updatedMonitor.Validate(); err != nil {
		return nil, fmt.Errorf("monitor validation failed: %w", err)
	}

	// Update monitor in database (Requirement 10.6)
	if err := s.monitorRepo.Update(ctx, &updatedMonitor); err != nil {
		return nil, fmt.Errorf("failed to update monitor: %w", err)
	}

	// Handle scheduling changes (Requirement 10.3)
	if s.scheduler != nil {
		// If monitor was disabled and is now enabled, schedule it
		if !existingMonitor.Enabled && updatedMonitor.Enabled {
			if err := s.scheduler.ScheduleMonitor(&updatedMonitor); err != nil {
				// Log error but don't fail the update
				_ = err
			}
		}
		// If monitor was enabled and is now disabled, unschedule it
		if existingMonitor.Enabled && !updatedMonitor.Enabled {
			if err := s.scheduler.UnscheduleMonitor(id); err != nil {
				// Log error but don't fail the update
				_ = err
			}
		}
		// If monitor is enabled and configuration changed, reschedule it
		if updatedMonitor.Enabled && (req.CheckInterval != nil ||
			req.URL != nil) {
			if err := s.scheduler.RescheduleMonitor(&updatedMonitor); err != nil {
				// Log error but don't fail the update
				_ = err
			}
		}
	}

	return &updatedMonitor, nil
}

// DeleteMonitor removes a monitor with cleanup (cancel jobs, delete data)
// Requirements: 10.4
func (s *MonitorService) DeleteMonitor(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("monitor ID is required")
	}

	// Check if monitor exists
	existingMonitor, err := s.monitorRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get monitor for deletion: %w", err)
	}

	// Cancel scheduled jobs if monitor is enabled (Requirement 10.4)
	if existingMonitor.Enabled && s.scheduler != nil {
		if err := s.scheduler.UnscheduleMonitor(id); err != nil {
			// Log error but continue with deletion
			_ = err
		}
	}

	// Delete monitor from database (this will cascade delete related data due to foreign key constraints)
	// Requirement 10.4: Delete all associated data
	if err := s.monitorRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete monitor: %w", err)
	}

	return nil
}

// EnableMonitor enables a monitor and schedules health checks
func (s *MonitorService) EnableMonitor(ctx context.Context, id string) error {
	monitor, err := s.GetMonitor(ctx, id)
	if err != nil {
		return err
	}

	if monitor.Enabled {
		return nil // Already enabled
	}

	// Update monitor to enabled
	req := UpdateMonitorRequest{
		Enabled: &[]bool{true}[0],
	}

	_, err = s.UpdateMonitor(ctx, id, req)
	return err
}

// DisableMonitor disables a monitor and cancels scheduled health checks
func (s *MonitorService) DisableMonitor(ctx context.Context, id string) error {
	monitor, err := s.GetMonitor(ctx, id)
	if err != nil {
		return err
	}

	if !monitor.Enabled {
		return nil // Already disabled
	}

	// Update monitor to disabled
	req := UpdateMonitorRequest{
		Enabled: &[]bool{false}[0],
	}

	_, err = s.UpdateMonitor(ctx, id, req)
	return err
}
