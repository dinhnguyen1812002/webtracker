package usecase

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"web-tracker/domain"
	"web-tracker/infrastructure/redis"
)

// WorkerPoolImpl manages a pool of workers that process health check jobs from Redis queue
// Implements the WorkerPool interface
type WorkerPoolImpl struct {
	redisClient   *redis.Client
	healthService HealthCheckExecutor
	queueName     string
	numWorkers    int
	workers       []*worker
	stopCh        chan struct{}
	wg            sync.WaitGroup
	stats         *WorkerPoolStats
	mu            sync.RWMutex
	running       bool
}

// WorkerPoolStats tracks worker pool statistics
type WorkerPoolStats struct {
	ActiveWorkers int64 `json:"active_workers"`
	QueueDepth    int64 `json:"queue_depth"`
	ProcessedJobs int64 `json:"processed_jobs"`
}

// HealthCheckExecutor defines the interface for executing health checks
type HealthCheckExecutor interface {
	ExecuteCheck(ctx context.Context, monitorID string) (*domain.HealthCheck, error)
}

// worker represents a single worker goroutine
type worker struct {
	id            int
	pool          *WorkerPoolImpl
	active        int64 // atomic: 1 if active, 0 if idle
	processedJobs int64 // atomic: total jobs processed by this worker
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(redisClient *redis.Client, healthService HealthCheckExecutor, queueName string) WorkerPool {
	return &WorkerPoolImpl{
		redisClient:   redisClient,
		healthService: healthService,
		queueName:     queueName,
		numWorkers:    10, // Default 10 workers (Requirement 6.1)
		stopCh:        make(chan struct{}),
		stats: &WorkerPoolStats{
			ActiveWorkers: 0,
			QueueDepth:    0,
			ProcessedJobs: 0,
		},
	}
}

// Start initializes and starts the worker pool with the specified number of workers
// Requirement 6.1: Initialize Worker_Pool with configurable number of concurrent workers
func (wp *WorkerPoolImpl) Start(ctx context.Context, numWorkers int) error {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	if wp.running {
		return fmt.Errorf("worker pool is already running")
	}

	if numWorkers <= 0 {
		numWorkers = wp.numWorkers // Use default if invalid
	}

	wp.numWorkers = numWorkers
	wp.workers = make([]*worker, numWorkers)
	wp.running = true

	// Start workers (Requirement 6.2: Distribute Health_Checks across available workers)
	for i := 0; i < numWorkers; i++ {
		worker := &worker{
			id:   i,
			pool: wp,
		}
		wp.workers[i] = worker

		wp.wg.Add(1)
		go worker.run(ctx)
	}

	log.Printf("Worker pool started with %d workers", numWorkers)
	return nil
}

// Stop gracefully shuts down the worker pool
// Requirement: Implement graceful shutdown
func (wp *WorkerPoolImpl) Stop() error {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	if !wp.running {
		return nil
	}

	log.Println("Stopping worker pool...")
	wp.running = false

	// Signal all workers to stop
	close(wp.stopCh)

	// Wait for all workers to finish their current jobs (graceful shutdown)
	wp.wg.Wait()

	log.Println("Worker pool stopped")
	return nil
}

// GetStats returns current worker pool statistics
// Requirement 6.4: Track worker pool statistics (active workers, queue depth)
func (wp *WorkerPoolImpl) GetStats() WorkerPoolStats {
	wp.mu.RLock()
	defer wp.mu.RUnlock()

	// Update queue depth from Redis
	queueDepth, err := wp.redisClient.GetQueueLength(context.Background(), wp.queueName)
	if err != nil {
		log.Printf("Failed to get queue depth: %v", err)
		queueDepth = 0
	}

	// Calculate active workers and total processed jobs
	var activeWorkers int64
	var totalProcessedJobs int64

	for _, worker := range wp.workers {
		if atomic.LoadInt64(&worker.active) == 1 {
			activeWorkers++
		}
		totalProcessedJobs += atomic.LoadInt64(&worker.processedJobs)
	}

	return WorkerPoolStats{
		ActiveWorkers: activeWorkers,
		QueueDepth:    queueDepth,
		ProcessedJobs: totalProcessedJobs,
	}
}

// IsRunning returns whether the worker pool is currently running
func (wp *WorkerPoolImpl) IsRunning() bool {
	wp.mu.RLock()
	defer wp.mu.RUnlock()
	return wp.running
}

// run is the main worker loop that processes jobs from the Redis queue
// Requirement 6.2: Workers consume jobs from Redis queue
// Requirement 6.3: Queue pending Health_Checks when all workers are busy
// Requirement 6.4: Immediately assign next queued Health_Check when worker completes
func (w *worker) run(ctx context.Context) {
	defer w.pool.wg.Done()

	log.Printf("Worker %d started", w.id)

	for {
		select {
		case <-w.pool.stopCh:
			log.Printf("Worker %d stopping", w.id)
			return
		default:
			// Try to dequeue a job from Redis (Requirement 6.2)
			// Use a short timeout to allow checking for stop signal
			job, err := w.pool.redisClient.Dequeue(ctx, w.pool.queueName, 1*time.Second)
			if err != nil {
				log.Printf("Worker %d: Failed to dequeue job: %v", w.id, err)
				continue
			}

			// No job available, continue polling
			if job == nil {
				continue
			}

			// Process the job
			w.processJob(ctx, job)
		}
	}
}

// processJob processes a single health check job
// Requirement 6.5: Ensure no single Monitor blocks other Monitors from being checked
func (w *worker) processJob(ctx context.Context, job *redis.Job) {
	// Mark worker as active
	atomic.StoreInt64(&w.active, 1)
	defer atomic.StoreInt64(&w.active, 0)

	// Increment processed jobs counter
	defer atomic.AddInt64(&w.processedJobs, 1)

	log.Printf("Worker %d processing job %s (type: %s)", w.id, job.ID, job.Type)

	// Extract monitor ID from job payload
	monitorID, ok := job.Payload["monitor_id"].(string)
	if !ok {
		log.Printf("Worker %d: Invalid job payload - missing monitor_id", w.id)
		return
	}

	// Create a context with timeout to prevent blocking other monitors (Requirement 6.5)
	// Use a reasonable timeout that's longer than the health check timeout (30s + buffer)
	jobCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	// Execute the health check
	_, err := w.pool.healthService.ExecuteCheck(jobCtx, monitorID)
	if err != nil {
		log.Printf("Worker %d: Health check failed for monitor %s: %v", w.id, monitorID, err)
		return
	}

	log.Printf("Worker %d completed job %s for monitor %s", w.id, job.ID, monitorID)
}
