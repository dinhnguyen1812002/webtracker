package tests

import (
	"context"
	"testing"
	"time"

	"web-tracker/usecase"
)

// Mock Redis client for testing rate limiter
type mockRateLimitRedis struct {
	data map[string][]byte
	ttls map[string]time.Duration
}

func newMockRateLimitRedis() *mockRateLimitRedis {
	return &mockRateLimitRedis{
		data: make(map[string][]byte),
		ttls: make(map[string]time.Duration),
	}
}

func (m *mockRateLimitRedis) Get(ctx context.Context, key string) ([]byte, error) {
	if val, ok := m.data[key]; ok {
		return val, nil
	}
	return nil, nil
}

func (m *mockRateLimitRedis) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	m.data[key] = value
	m.ttls[key] = ttl
	return nil
}

func (m *mockRateLimitRedis) Delete(ctx context.Context, key string) error {
	delete(m.data, key)
	delete(m.ttls, key)
	return nil
}

// TestCheckAndSetDowntimeAlert_FirstAlert tests that the first downtime alert is allowed
// Requirement 5.2: 15-minute suppression for duplicate downtime alerts
func TestCheckAndSetDowntimeAlert_FirstAlert(t *testing.T) {
	mockRedis := newMockRateLimitRedis()
	limiter := usecase.NewRedisRateLimiter(mockRedis)

	ctx := context.Background()
	monitorID := "monitor-1"

	shouldSend, err := limiter.CheckAndSetDowntimeAlert(ctx, monitorID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !shouldSend {
		t.Error("first downtime alert should be allowed")
	}

	// Verify rate limit was set with 15-minute TTL
	key := "ratelimit:alert:monitor-1:downtime"
	if _, ok := mockRedis.data[key]; !ok {
		t.Error("rate limit key should be set")
	}

	if mockRedis.ttls[key] != 15*time.Minute {
		t.Errorf("expected TTL of 15 minutes, got %v", mockRedis.ttls[key])
	}
}

// TestCheckAndSetDowntimeAlert_Suppression tests 15-minute suppression
// Requirement 5.2: 15-minute suppression for duplicate downtime alerts
func TestCheckAndSetDowntimeAlert_Suppression(t *testing.T) {
	mockRedis := newMockRateLimitRedis()
	limiter := usecase.NewRedisRateLimiter(mockRedis)

	ctx := context.Background()
	monitorID := "monitor-1"

	// Set a recent alert (5 minutes ago)
	recentTime := time.Now().Add(-5 * time.Minute)
	key := "ratelimit:alert:monitor-1:downtime"
	mockRedis.data[key] = []byte(recentTime.Format(time.RFC3339))

	shouldSend, err := limiter.CheckAndSetDowntimeAlert(ctx, monitorID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if shouldSend {
		t.Error("downtime alert should be suppressed within 15 minutes")
	}
}

// TestCheckAndSetDowntimeAlert_HourlyReminder tests 1-hour reminder
// Requirement 5.4: 1-hour reminder for prolonged failures
func TestCheckAndSetDowntimeAlert_HourlyReminder(t *testing.T) {
	mockRedis := newMockRateLimitRedis()
	limiter := usecase.NewRedisRateLimiter(mockRedis)

	ctx := context.Background()
	monitorID := "monitor-1"

	// Set an old alert (65 minutes ago)
	oldTime := time.Now().Add(-65 * time.Minute)
	key := "ratelimit:alert:monitor-1:downtime"
	mockRedis.data[key] = []byte(oldTime.Format(time.RFC3339))

	shouldSend, err := limiter.CheckAndSetDowntimeAlert(ctx, monitorID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !shouldSend {
		t.Error("downtime alert should be sent after 1 hour (reminder)")
	}
}

// TestCheckAndSetDowntimeAlert_After15Minutes tests alert after suppression period
func TestCheckAndSetDowntimeAlert_After15Minutes(t *testing.T) {
	mockRedis := newMockRateLimitRedis()
	limiter := usecase.NewRedisRateLimiter(mockRedis)

	ctx := context.Background()
	monitorID := "monitor-1"

	// Set an alert 20 minutes ago (past suppression period but before reminder)
	pastTime := time.Now().Add(-20 * time.Minute)
	key := "ratelimit:alert:monitor-1:downtime"
	mockRedis.data[key] = []byte(pastTime.Format(time.RFC3339))

	shouldSend, err := limiter.CheckAndSetDowntimeAlert(ctx, monitorID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !shouldSend {
		t.Error("downtime alert should be sent after 15 minutes")
	}
}

// TestCheckAndSetSSLAlert_FirstAlert tests that the first SSL alert is allowed
// Requirement 5.5: Daily limit for SSL expiration alerts
func TestCheckAndSetSSLAlert_FirstAlert(t *testing.T) {
	mockRedis := newMockRateLimitRedis()
	limiter := usecase.NewRedisRateLimiter(mockRedis)

	ctx := context.Background()
	monitorID := "monitor-1"
	daysUntil := 7

	shouldSend, err := limiter.CheckAndSetSSLAlert(ctx, monitorID, daysUntil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !shouldSend {
		t.Error("first SSL alert should be allowed")
	}

	// Verify rate limit was set with 24-hour TTL
	key := "ratelimit:alert:monitor-1:ssl:7"
	if _, ok := mockRedis.data[key]; !ok {
		t.Error("SSL rate limit key should be set")
	}

	if mockRedis.ttls[key] != 24*time.Hour {
		t.Errorf("expected TTL of 24 hours, got %v", mockRedis.ttls[key])
	}
}

// TestCheckAndSetSSLAlert_DailyLimit tests daily suppression
// Requirement 5.5: Daily limit for SSL expiration alerts
func TestCheckAndSetSSLAlert_DailyLimit(t *testing.T) {
	mockRedis := newMockRateLimitRedis()
	limiter := usecase.NewRedisRateLimiter(mockRedis)

	ctx := context.Background()
	monitorID := "monitor-1"
	daysUntil := 7

	// Set an alert sent today
	key := "ratelimit:alert:monitor-1:ssl:7"
	mockRedis.data[key] = []byte(time.Now().Format(time.RFC3339))

	shouldSend, err := limiter.CheckAndSetSSLAlert(ctx, monitorID, daysUntil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if shouldSend {
		t.Error("SSL alert should be suppressed within 24 hours")
	}
}

// TestCheckAndSetSSLAlert_DifferentLevels tests that different warning levels are tracked separately
func TestCheckAndSetSSLAlert_DifferentLevels(t *testing.T) {
	mockRedis := newMockRateLimitRedis()
	limiter := usecase.NewRedisRateLimiter(mockRedis)

	ctx := context.Background()
	monitorID := "monitor-1"

	// Set an alert for 30 days
	key30 := "ratelimit:alert:monitor-1:ssl:30"
	mockRedis.data[key30] = []byte(time.Now().Format(time.RFC3339))

	// Try to send an alert for 7 days (different level)
	shouldSend, err := limiter.CheckAndSetSSLAlert(ctx, monitorID, 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !shouldSend {
		t.Error("SSL alert for different warning level should be allowed")
	}
}

// TestGetLastDowntimeAlert tests retrieving last downtime alert timestamp
func TestGetLastDowntimeAlert(t *testing.T) {
	mockRedis := newMockRateLimitRedis()
	limiter := usecase.NewRedisRateLimiter(mockRedis)

	ctx := context.Background()
	monitorID := "monitor-1"

	// Test when no alert exists
	lastAlert, err := limiter.GetLastDowntimeAlert(ctx, monitorID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lastAlert != nil {
		t.Error("expected nil when no alert exists")
	}

	// Set an alert
	alertTime := time.Now().Add(-10 * time.Minute)
	key := "ratelimit:alert:monitor-1:downtime"
	mockRedis.data[key] = []byte(alertTime.Format(time.RFC3339))

	// Retrieve the alert
	lastAlert, err = limiter.GetLastDowntimeAlert(ctx, monitorID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lastAlert == nil {
		t.Fatal("expected alert timestamp")
	}

	// Check the timestamp is approximately correct (within 1 second)
	diff := lastAlert.Sub(alertTime)
	if diff < -1*time.Second || diff > 1*time.Second {
		t.Errorf("expected timestamp around %v, got %v", alertTime, *lastAlert)
	}
}

// TestClearDowntimeAlert tests clearing downtime rate limit
// Requirement 5.3: Recovery alerts bypass rate limiting
func TestClearDowntimeAlert(t *testing.T) {
	mockRedis := newMockRateLimitRedis()
	limiter := usecase.NewRedisRateLimiter(mockRedis)

	ctx := context.Background()
	monitorID := "monitor-1"

	// Set a downtime alert
	key := "ratelimit:alert:monitor-1:downtime"
	mockRedis.data[key] = []byte(time.Now().Format(time.RFC3339))

	// Clear the alert
	err := limiter.ClearDowntimeAlert(ctx, monitorID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify it was cleared
	if _, ok := mockRedis.data[key]; ok {
		t.Error("downtime rate limit should be cleared")
	}
}
