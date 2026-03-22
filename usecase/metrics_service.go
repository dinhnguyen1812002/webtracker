package usecase

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"web-tracker/domain"
)

// MetricsServiceImpl provides uptime and response time metrics for monitors
type MetricsServiceImpl struct {
	healthCheckRepo domain.HealthCheckRepository
	redisClient     MetricsRedisClient
}

const metricsCacheTimeout = 250 * time.Millisecond

// NewMetricsService creates a new metrics service
func NewMetricsService(healthCheckRepo domain.HealthCheckRepository, redisClient MetricsRedisClient) MetricsService {
	// Return nil if healthCheckRepo is nil to prevent panics
	if healthCheckRepo == nil {
		return nil
	}

	return &MetricsServiceImpl{
		healthCheckRepo: healthCheckRepo,
		redisClient:     redisClient,
	}
}

// GetUptimePercentage calculates uptime percentage for a monitor over different time periods
func (s *MetricsServiceImpl) GetUptimePercentage(ctx context.Context, monitorID string) (*UptimeStats, error) {
	// Defensive check
	if s == nil || s.healthCheckRepo == nil {
		return nil, fmt.Errorf("metrics service or health check repository is nil")
	}

	// Try to get from cache first
	cacheKey := fmt.Sprintf("cache:metrics:%s:uptime", monitorID)
	var stats UptimeStats
	if found, err := s.getCachedJSON(ctx, cacheKey, &stats); err != nil {
		fmt.Printf("Failed to get uptime from cache: %v\n", err)
	} else if found {
		return &stats, nil
	}

	// Calculate uptime for each period
	now := time.Now()

	// 24 hours
	start24h := now.Add(-24 * time.Hour)
	uptime24h, err := s.calculateUptimeForPeriod(ctx, monitorID, start24h, now)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate 24h uptime: %w", err)
	}

	// 7 days
	start7d := now.Add(-7 * 24 * time.Hour)
	uptime7d, err := s.calculateUptimeForPeriod(ctx, monitorID, start7d, now)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate 7d uptime: %w", err)
	}

	// 30 days
	start30d := now.Add(-30 * 24 * time.Hour)
	uptime30d, err := s.calculateUptimeForPeriod(ctx, monitorID, start30d, now)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate 30d uptime: %w", err)
	}

	stats = UptimeStats{
		Period24h: uptime24h,
		Period7d:  uptime7d,
		Period30d: uptime30d,
	}

	// Cache the results for 1 minute
	if err := s.setCachedJSON(ctx, cacheKey, stats, time.Minute); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Failed to cache uptime stats: %v\n", err)
	}

	return &stats, nil
}

// calculateUptimeForPeriod calculates uptime percentage for a specific time period
func (s *MetricsServiceImpl) calculateUptimeForPeriod(ctx context.Context, monitorID string, start, end time.Time) (float64, error) {
	checks, err := s.healthCheckRepo.GetByDateRange(ctx, monitorID, start, end)
	if err != nil {
		return 0, fmt.Errorf("failed to get health checks: %w", err)
	}

	if len(checks) == 0 {
		return 0.0, nil // Return 0% for empty history
	}

	successfulChecks := 0
	for _, check := range checks {
		if check.IsSuccessful() {
			successfulChecks++
		}
	}

	// Calculate percentage: (successful checks / total checks) × 100
	percentage := (float64(successfulChecks) / float64(len(checks))) * 100

	// Round to 2 decimal places
	return math.Round(percentage*100) / 100, nil
}

// GetResponseTimeStats calculates response time statistics for different time periods
func (s *MetricsServiceImpl) GetResponseTimeStats(ctx context.Context, monitorID string, period time.Duration) (*ResponseTimeStats, error) {
	// Try to get from cache first
	cacheKey := fmt.Sprintf("cache:metrics:%s:response:%s", monitorID, period.String())
	var stats ResponseTimeStats
	if found, err := s.getCachedJSON(ctx, cacheKey, &stats); err != nil {
		fmt.Printf("Failed to get response time stats from cache: %v\n", err)
	} else if found {
		return &stats, nil
	}

	// Calculate response time stats
	now := time.Now()
	start := now.Add(-period)

	checks, err := s.healthCheckRepo.GetByDateRange(ctx, monitorID, start, now)
	if err != nil {
		return nil, fmt.Errorf("failed to get health checks: %w", err)
	}

	// Filter only successful checks for response time stats
	var responseTimes []time.Duration
	for _, check := range checks {
		if check.IsSuccessful() && check.ResponseTime > 0 {
			responseTimes = append(responseTimes, check.ResponseTime)
		}
	}

	if len(responseTimes) == 0 {
		// Return zero stats if no successful checks
		return &ResponseTimeStats{}, nil
	}

	// Sort response times for percentile calculations
	sort.Slice(responseTimes, func(i, j int) bool {
		return responseTimes[i] < responseTimes[j]
	})

	// Calculate statistics
	stats = ResponseTimeStats{
		Min:     responseTimes[0],
		Max:     responseTimes[len(responseTimes)-1],
		Average: s.calculateAverage(responseTimes),
		P95:     s.calculatePercentile(responseTimes, 95),
		P99:     s.calculatePercentile(responseTimes, 99),
	}

	// Cache the results for 1 minute
	if err := s.setCachedJSON(ctx, cacheKey, stats, time.Minute); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Failed to cache response time stats: %v\n", err)
	}

	return &stats, nil
}

// calculateAverage calculates the average of response times
func (s *MetricsServiceImpl) calculateAverage(responseTimes []time.Duration) time.Duration {
	if len(responseTimes) == 0 {
		return 0
	}

	var total time.Duration
	for _, rt := range responseTimes {
		total += rt
	}

	return total / time.Duration(len(responseTimes))
}

// calculatePercentile calculates the specified percentile of response times
func (s *MetricsServiceImpl) calculatePercentile(responseTimes []time.Duration, percentile float64) time.Duration {
	if len(responseTimes) == 0 {
		return 0
	}

	if percentile <= 0 {
		return responseTimes[0]
	}
	if percentile >= 100 {
		return responseTimes[len(responseTimes)-1]
	}

	// Calculate index for percentile
	index := (percentile / 100) * float64(len(responseTimes)-1)

	// If index is exact, return that element
	if index == float64(int(index)) {
		return responseTimes[int(index)]
	}

	// Otherwise, interpolate between two elements
	lower := int(math.Floor(index))
	upper := int(math.Ceil(index))

	if upper >= len(responseTimes) {
		return responseTimes[len(responseTimes)-1]
	}

	// Linear interpolation
	weight := index - float64(lower)
	lowerVal := float64(responseTimes[lower])
	upperVal := float64(responseTimes[upper])

	interpolated := lowerVal + weight*(upperVal-lowerVal)
	return time.Duration(interpolated)
}

// GetResponseTimeStats1h calculates response time statistics for the last 1 hour
func (s *MetricsServiceImpl) GetResponseTimeStats1h(ctx context.Context, monitorID string) (*ResponseTimeStats, error) {
	return s.GetResponseTimeStats(ctx, monitorID, time.Hour)
}

// GetResponseTimeStats24h calculates response time statistics for the last 24 hours
func (s *MetricsServiceImpl) GetResponseTimeStats24h(ctx context.Context, monitorID string) (*ResponseTimeStats, error) {
	return s.GetResponseTimeStats(ctx, monitorID, 24*time.Hour)
}

// GetResponseTimeStats7d calculates response time statistics for the last 7 days
func (s *MetricsServiceImpl) GetResponseTimeStats7d(ctx context.Context, monitorID string) (*ResponseTimeStats, error) {
	return s.GetResponseTimeStats(ctx, monitorID, 7*24*time.Hour)
}

func (s *MetricsServiceImpl) getCachedJSON(parent context.Context, key string, dest interface{}) (bool, error) {
	if s.redisClient == nil {
		return false, nil
	}

	ctx, cancel := cacheContext(parent)
	defer cancel()

	found, err := s.redisClient.GetJSON(ctx, key, dest)
	if err == nil || errors.Is(err, context.Canceled) {
		return found, err
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return false, nil
	}
	return found, err
}

func (s *MetricsServiceImpl) setCachedJSON(parent context.Context, key string, value interface{}, ttl time.Duration) error {
	if s.redisClient == nil {
		return nil
	}

	ctx, cancel := cacheContext(parent)
	defer cancel()

	err := s.redisClient.SetJSON(ctx, key, value, ttl)
	if err == nil || errors.Is(err, context.Canceled) {
		return err
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return nil
	}
	return err
}

func cacheContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		return context.WithTimeout(context.Background(), metricsCacheTimeout)
	}

	deadline := time.Now().Add(metricsCacheTimeout)
	if existingDeadline, ok := parent.Deadline(); ok && existingDeadline.Before(deadline) {
		return context.WithDeadline(parent, existingDeadline)
	}
	return context.WithTimeout(parent, metricsCacheTimeout)
}
