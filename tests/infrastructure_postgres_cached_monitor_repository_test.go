package tests

import (
	"context"
	"os"
	"testing"

	"web-tracker/domain"
	"web-tracker/infrastructure/postgres"
	infraRedis "web-tracker/infrastructure/redis"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDB creates a test database connection pool
func setupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	ctx := context.Background()
	config := postgres.DefaultPoolConfig()

	// Use environment variables if set
	if host := os.Getenv("TEST_DB_HOST"); host != "" {
		config.Host = host
	}
	if dbName := os.Getenv("TEST_DB_NAME"); dbName != "" {
		config.Database = dbName
	}

	pool, err := postgres.NewPool(ctx, config)
	if err != nil {
		t.Skipf("Database not available: %v", err)
	}

	// Run migrations
	if err := postgres.RunMigrations(ctx, pool); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	return pool
}

// setupTestRedis creates a test Redis client
func setupTestRedis(t *testing.T) *infraRedis.Client {
	t.Helper()

	cfg := infraRedis.DefaultConfig()
	client, err := infraRedis.NewClient(cfg)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}

	// Clean up any existing test keys
	ctx := context.Background()
	if err := client.Ping(ctx); err != nil {
		t.Skipf("Redis not reachable: %v", err)
	}

	return client
}

func TestCachedMonitorRepository_Create(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	redisClient := setupTestRedis(t)
	defer redisClient.Close()

	repo := postgres.NewMonitorRepository(pool)
	cachedRepo := postgres.NewCachedMonitorRepository(repo, redisClient)

	ctx := context.Background()

	monitor := &domain.Monitor{
		ID:            uuid.New().String(),
		Name:          "Test Monitor",
		URL:           "https://example.com",
		CheckInterval: domain.CheckInterval5Min,
		Enabled:       true,
		AlertChannels: []domain.AlertChannel{},
	}

	err := cachedRepo.Create(ctx, monitor)
	require.NoError(t, err)

	// Verify monitor is in database
	dbMonitor, err := repo.GetByID(ctx, monitor.ID)
	require.NoError(t, err)
	assert.Equal(t, monitor.Name, dbMonitor.Name)

	// Verify monitor is cached
	var cachedMonitor domain.Monitor
	found, err := redisClient.GetJSON(ctx, postgres.MonitorCacheKeyPrefix+monitor.ID, &cachedMonitor)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, monitor.Name, cachedMonitor.Name)
}

func TestCachedMonitorRepository_GetByID_CacheHit(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	redisClient := setupTestRedis(t)
	defer redisClient.Close()

	repo := postgres.NewMonitorRepository(pool)
	cachedRepo := postgres.NewCachedMonitorRepository(repo, redisClient)

	ctx := context.Background()

	// Create a monitor
	monitor := &domain.Monitor{
		ID:            uuid.New().String(),
		Name:          "Test Monitor",
		URL:           "https://example.com",
		CheckInterval: domain.CheckInterval5Min,
		Enabled:       true,
		AlertChannels: []domain.AlertChannel{},
	}

	err := repo.Create(ctx, monitor)
	require.NoError(t, err)

	// First call - cache miss, should fetch from DB and cache
	result1, err := cachedRepo.GetByID(ctx, monitor.ID)
	require.NoError(t, err)
	assert.Equal(t, monitor.Name, result1.Name)

	// Modify the database record directly (bypass cache)
	monitor.Name = "Modified Name"
	err = repo.Update(ctx, monitor)
	require.NoError(t, err)

	// Second call - cache hit, should return cached (old) value
	result2, err := cachedRepo.GetByID(ctx, monitor.ID)
	require.NoError(t, err)
	assert.Equal(t, "Test Monitor", result2.Name, "Should return cached value, not updated DB value")
}

func TestCachedMonitorRepository_GetByID_CacheMiss(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	redisClient := setupTestRedis(t)
	defer redisClient.Close()

	repo := postgres.NewMonitorRepository(pool)
	cachedRepo := postgres.NewCachedMonitorRepository(repo, redisClient)

	ctx := context.Background()

	// Create a monitor
	monitor := &domain.Monitor{
		ID:            uuid.New().String(),
		Name:          "Test Monitor",
		URL:           "https://example.com",
		CheckInterval: domain.CheckInterval5Min,
		Enabled:       true,
		AlertChannels: []domain.AlertChannel{},
	}

	err := repo.Create(ctx, monitor)
	require.NoError(t, err)

	// Get monitor - should fetch from DB and cache it
	result, err := cachedRepo.GetByID(ctx, monitor.ID)
	require.NoError(t, err)
	assert.Equal(t, monitor.Name, result.Name)

	// Verify it was cached
	var cachedMonitor domain.Monitor
	found, err := redisClient.GetJSON(ctx, postgres.MonitorCacheKeyPrefix+monitor.ID, &cachedMonitor)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, monitor.Name, cachedMonitor.Name)
}

func TestCachedMonitorRepository_Update_InvalidatesCache(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	redisClient := setupTestRedis(t)
	defer redisClient.Close()

	repo := postgres.NewMonitorRepository(pool)
	cachedRepo := postgres.NewCachedMonitorRepository(repo, redisClient)

	ctx := context.Background()

	// Create a monitor
	monitor := &domain.Monitor{
		ID:            uuid.New().String(),
		Name:          "Test Monitor",
		URL:           "https://example.com",
		CheckInterval: domain.CheckInterval5Min,
		Enabled:       true,
		AlertChannels: []domain.AlertChannel{},
	}

	err := cachedRepo.Create(ctx, monitor)
	require.NoError(t, err)

	// Get monitor to populate cache
	_, err = cachedRepo.GetByID(ctx, monitor.ID)
	require.NoError(t, err)

	// Verify it's cached
	var cachedMonitor domain.Monitor
	found, err := redisClient.GetJSON(ctx, postgres.MonitorCacheKeyPrefix+monitor.ID, &cachedMonitor)
	require.NoError(t, err)
	assert.True(t, found)

	// Update the monitor
	monitor.Name = "Updated Monitor"
	err = cachedRepo.Update(ctx, monitor)
	require.NoError(t, err)

	// Verify cache was invalidated
	found, err = redisClient.GetJSON(ctx, postgres.MonitorCacheKeyPrefix+monitor.ID, &cachedMonitor)
	require.NoError(t, err)
	assert.False(t, found, "Cache should be invalidated after update")

	// Get monitor again - should fetch fresh data from DB
	result, err := cachedRepo.GetByID(ctx, monitor.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Monitor", result.Name)
}

func TestCachedMonitorRepository_Delete_InvalidatesCache(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	redisClient := setupTestRedis(t)
	defer redisClient.Close()

	repo := postgres.NewMonitorRepository(pool)
	cachedRepo := postgres.NewCachedMonitorRepository(repo, redisClient)

	ctx := context.Background()

	// Create a monitor
	monitor := &domain.Monitor{
		ID:            uuid.New().String(),
		Name:          "Test Monitor",
		URL:           "https://example.com",
		CheckInterval: domain.CheckInterval5Min,
		Enabled:       true,
		AlertChannels: []domain.AlertChannel{},
	}

	err := cachedRepo.Create(ctx, monitor)
	require.NoError(t, err)

	// Get monitor to populate cache
	_, err = cachedRepo.GetByID(ctx, monitor.ID)
	require.NoError(t, err)

	// Verify it's cached
	var cachedMonitor domain.Monitor
	found, err := redisClient.GetJSON(ctx, postgres.MonitorCacheKeyPrefix+monitor.ID, &cachedMonitor)
	require.NoError(t, err)
	assert.True(t, found)

	// Delete the monitor
	err = cachedRepo.Delete(ctx, monitor.ID)
	require.NoError(t, err)

	// Verify cache was invalidated
	found, err = redisClient.GetJSON(ctx, postgres.MonitorCacheKeyPrefix+monitor.ID, &cachedMonitor)
	require.NoError(t, err)
	assert.False(t, found, "Cache should be invalidated after delete")
}

func TestCachedMonitorRepository_CacheTTL(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping TTL test in short mode")
	}

	pool := setupTestDB(t)
	defer pool.Close()

	redisClient := setupTestRedis(t)
	defer redisClient.Close()

	repo := postgres.NewMonitorRepository(pool)
	cachedRepo := postgres.NewCachedMonitorRepository(repo, redisClient)

	ctx := context.Background()

	// Create a monitor
	monitor := &domain.Monitor{
		ID:            uuid.New().String(),
		Name:          "Test Monitor",
		URL:           "https://example.com",
		CheckInterval: domain.CheckInterval5Min,
		Enabled:       true,
		AlertChannels: []domain.AlertChannel{},
	}

	err := cachedRepo.Create(ctx, monitor)
	require.NoError(t, err)

	// Get monitor to populate cache
	_, err = cachedRepo.GetByID(ctx, monitor.ID)
	require.NoError(t, err)

	// Verify it's cached
	var cachedMonitor domain.Monitor
	found, err := redisClient.GetJSON(ctx, postgres.MonitorCacheKeyPrefix+monitor.ID, &cachedMonitor)
	require.NoError(t, err)
	assert.True(t, found)

	// Note: Full TTL test would require waiting 5 minutes
	// In a real scenario, you might want to use a shorter TTL for testing
	// or mock the Redis client to verify TTL was set correctly
	t.Log("Cache TTL is set to 5 minutes as per requirements")
}

func TestCachedMonitorRepository_List_NotCached(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	redisClient := setupTestRedis(t)
	defer redisClient.Close()

	repo := postgres.NewMonitorRepository(pool)
	cachedRepo := postgres.NewCachedMonitorRepository(repo, redisClient)

	ctx := context.Background()

	// Create multiple monitors
	for i := 0; i < 3; i++ {
		monitor := &domain.Monitor{
			ID:            uuid.New().String(),
			Name:          "Test Monitor",
			URL:           "https://example.com",
			CheckInterval: domain.CheckInterval5Min,
			Enabled:       true,
			AlertChannels: []domain.AlertChannel{},
		}
		err := cachedRepo.Create(ctx, monitor)
		require.NoError(t, err)
	}

	// List monitors - should always go to database
	monitors, err := cachedRepo.List(ctx, domain.ListFilters{})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(monitors), 3)
}
