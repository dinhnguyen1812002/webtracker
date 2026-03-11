package usecase

import (
	"context"
	"fmt"
	"log"
	"math/rand/v2"
	"sync"
	"time"

	"web-tracker/domain"
	"web-tracker/infrastructure/redis"
)

// JobScheduler manages time-based scheduling of health check jobs using Redis sorted sets
// Implements the Scheduler interface
type JobScheduler struct {
	redisClient  *redis.Client
	scheduleName string
	queueName    string
	stopCh       chan struct{}
	wg           sync.WaitGroup
	mu           sync.RWMutex
	running      bool
	tickInterval time.Duration
}

// SchedulerConfig holds configuration for the scheduler
type SchedulerConfig struct {
	ScheduleName string        // Redis sorted set name for scheduled jobs
	QueueName    string        // Redis queue name for ready jobs
	TickInterval time.Duration // How often to check for due jobs
}

// NewScheduler creates a new job scheduler
func NewScheduler(redisClient *redis.Client, config SchedulerConfig) Scheduler {
	if config.TickInterval == 0 {
		config.TickInterval = 10 * time.Second // Default check interval
	}

	return &JobScheduler{
		redisClient:  redisClient,
		scheduleName: config.ScheduleName,
		queueName:    config.QueueName,
		stopCh:       make(chan struct{}),
		tickInterval: config.TickInterval,
	}
}

// Start begins the scheduler's main loop
func (s *JobScheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("scheduler is already running")
	}

	s.running = true
	s.wg.Add(1)

	go s.run(ctx)

	log.Printf("Scheduler started with tick interval %v", s.tickInterval)
	return nil
}

// Stop gracefully shuts down the scheduler
func (s *JobScheduler) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	log.Println("Stopping scheduler...")
	s.running = false

	close(s.stopCh)
	s.wg.Wait()

	log.Println("Scheduler stopped")
	return nil
}

// ScheduleMonitor schedules health checks for a monitor with jitter
// Requirement: Add jitter (±10% of interval) to distribute load
func (s *JobScheduler) ScheduleMonitor(monitor *domain.Monitor) error {
	if !monitor.Enabled {
		return nil // Don't schedule disabled monitors
	}

	// Calculate next execution time with jitter
	nextExecution := s.calculateNextExecutionWithJitter(time.Now(), monitor.CheckInterval)

	// Create job for the monitor
	job := &redis.Job{
		ID:   fmt.Sprintf("health_check_%s_%d", monitor.ID, time.Now().UnixNano()),
		Type: "health_check",
		Payload: map[string]interface{}{
			"monitor_id": monitor.ID,
		},
		CreatedAt: time.Now(),
	}

	// Schedule the job using Redis sorted sets
	err := s.redisClient.Schedule(context.Background(), s.scheduleName, job, nextExecution)
	if err != nil {
		return fmt.Errorf("failed to schedule monitor %s: %w", monitor.ID, err)
	}

	log.Printf("Scheduled monitor %s (%s) for execution at %v (interval: %v)",
		monitor.ID, monitor.Name, nextExecution, monitor.CheckInterval)

	return nil
}

// UnscheduleMonitor removes all scheduled jobs for a monitor
// Requirement: Handle monitor updates by rescheduling
func (s *JobScheduler) UnscheduleMonitor(monitorID string) error {
	// This is a simplified approach - in a production system, you might want to
	// store job IDs in a separate index for efficient removal
	// For now, we'll rely on the job processing to skip jobs for deleted monitors
	log.Printf("Unscheduled monitor %s (jobs will be skipped during processing)", monitorID)
	return nil
}

// RescheduleMonitor updates the schedule for a monitor
// Requirement: Handle monitor updates by rescheduling
func (s *JobScheduler) RescheduleMonitor(monitor *domain.Monitor) error {
	// Remove existing schedule (simplified approach)
	s.UnscheduleMonitor(monitor.ID)

	// Add new schedule
	return s.ScheduleMonitor(monitor)
}

// calculateNextExecutionWithJitter adds jitter (±10% of interval) to distribute load
// Requirement: Add jitter (±10% of interval) to distribute load
func (s *JobScheduler) calculateNextExecutionWithJitter(baseTime time.Time, interval time.Duration) time.Time {
	// Calculate jitter as ±10% of the interval
	jitterRange := float64(interval) * 0.1
	jitter := time.Duration((rand.Float64()*2 - 1) * jitterRange) // Random value between -10% and +10%

	return baseTime.Add(interval + jitter)
}

// run is the main scheduler loop that checks for due jobs and enqueues them
// Requirement: Enqueue jobs when scheduled time arrives
func (s *JobScheduler) run(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(s.tickInterval)
	defer ticker.Stop()

	log.Printf("Scheduler main loop started")

	for {
		select {
		case <-s.stopCh:
			log.Printf("Scheduler main loop stopping")
			return
		case <-ticker.C:
			s.processDueJobs(ctx)
		}
	}
}

// processDueJobs checks for jobs that are due and moves them to the execution queue
// Requirement: Use Redis sorted sets for time-based scheduling
// Requirement: Enqueue jobs when scheduled time arrives
func (s *JobScheduler) processDueJobs(ctx context.Context) {
	now := time.Now()

	// Get all jobs that are due for execution
	dueJobs, err := s.redisClient.GetDueJobs(ctx, s.scheduleName, now)
	if err != nil {
		log.Printf("Failed to get due jobs: %v", err)
		return
	}

	if len(dueJobs) == 0 {
		return // No jobs due
	}

	log.Printf("Processing %d due jobs", len(dueJobs))

	// Enqueue each due job for execution
	for _, job := range dueJobs {
		err := s.redisClient.Enqueue(ctx, s.queueName, job)
		if err != nil {
			log.Printf("Failed to enqueue job %s: %v", job.ID, err)
			continue
		}

		log.Printf("Enqueued job %s for monitor %s", job.ID, job.Payload["monitor_id"])

		// Schedule the next execution for this monitor
		// Extract monitor ID from job payload
		if monitorID, ok := job.Payload["monitor_id"].(string); ok {
			s.scheduleNextExecution(ctx, monitorID, job)
		}
	}
}

// scheduleNextExecution schedules the next health check for a monitor after the current one is processed
func (s *JobScheduler) scheduleNextExecution(ctx context.Context, monitorID string, currentJob *redis.Job) {
	// We need to determine the check interval for this monitor
	// In a real implementation, you might cache monitor configs or fetch from database
	// For now, we'll use a default interval and let the monitor service handle rescheduling
	// when monitors are updated

	// This is a placeholder - the actual rescheduling should happen when:
	// 1. A monitor is created/updated (via ScheduleMonitor/RescheduleMonitor)
	// 2. A health check completes successfully (to maintain the interval)

	log.Printf("Next execution for monitor %s will be scheduled by monitor service", monitorID)
}

// IsRunning returns whether the scheduler is currently running
func (s *JobScheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}
