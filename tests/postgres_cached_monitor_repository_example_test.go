package tests

import (
	"context"
	"fmt"

	"web-tracker/domain"
	"web-tracker/infrastructure/postgres"
	"web-tracker/infrastructure/redis"
)

// This example demonstrates how to use the cached monitor repository.
// Note: This is a documentation example and won't run without a database connection.
func ExampleCachedMonitorRepository_usage() {
	ctx := context.Background()

	// Setup PostgreSQL connection pool
	pgConfig := postgres.DefaultPoolConfig()
	pool, _ := postgres.NewPool(ctx, pgConfig)
	defer pool.Close()

	// Setup Redis client
	redisConfig := redis.DefaultConfig()
	redisClient, _ := redis.NewClient(redisConfig)
	defer redisClient.Close()

	// Create the base repository
	baseRepo := postgres.NewMonitorRepository(pool)

	// Wrap it with caching
	cachedRepo := postgres.NewCachedMonitorRepository(baseRepo, redisClient)

	// Create a monitor
	monitor := &domain.Monitor{
		ID:            "mon-123",
		Name:          "Example Website",
		URL:           "https://example.com",
		CheckInterval: domain.CheckInterval5Min,
		Enabled:       true,
	}

	_ = cachedRepo.Create(ctx, monitor)

	// First GetByID - cache miss, fetches from database
	retrieved1, _ := cachedRepo.GetByID(ctx, "mon-123")
	fmt.Printf("First fetch: %s\n", retrieved1.Name)

	// Second GetByID - cache hit, returns from Redis (faster)
	retrieved2, _ := cachedRepo.GetByID(ctx, "mon-123")
	fmt.Printf("Second fetch (cached): %s\n", retrieved2.Name)

	// Update invalidates cache
	monitor.Name = "Updated Website"
	_ = cachedRepo.Update(ctx, monitor)

	// Next GetByID will fetch fresh data from database
	retrieved3, _ := cachedRepo.GetByID(ctx, "mon-123")
	fmt.Printf("After update: %s\n", retrieved3.Name)
}
