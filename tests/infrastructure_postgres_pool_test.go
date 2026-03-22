package tests

import (
	"testing"
	"time"

	"web-tracker/infrastructure/postgres"
)

func TestDefaultPoolConfig(t *testing.T) {
	config := postgres.DefaultPoolConfig()

	// Verify memory-optimized settings
	if config.MaxConns != 20 {
		t.Errorf("Expected MaxConns to be 20, got %d", config.MaxConns)
	}

	if config.MinConns != 2 {
		t.Errorf("Expected MinConns to be 2, got %d", config.MinConns)
	}

	// Verify optimized timeouts
	expectedMaxConnLifetime := 30 * time.Minute
	if config.MaxConnLifetime != expectedMaxConnLifetime {
		t.Errorf("Expected MaxConnLifetime to be %v, got %v", expectedMaxConnLifetime, config.MaxConnLifetime)
	}

	expectedMaxConnIdleTime := 15 * time.Minute
	if config.MaxConnIdleTime != expectedMaxConnIdleTime {
		t.Errorf("Expected MaxConnIdleTime to be %v, got %v", expectedMaxConnIdleTime, config.MaxConnIdleTime)
	}

	// Verify basic configuration
	if config.Host == "" {
		t.Error("Expected non-empty Host")
	}

	if config.Port <= 0 {
		t.Error("Expected positive Port")
	}

	if config.Database == "" {
		t.Error("Expected non-empty Database")
	}

	if config.SSLMode != "disable" {
		t.Errorf("Expected SSLMode to be disable, got %s", config.SSLMode)
	}
}

func TestPoolStats(t *testing.T) {
	// This test verifies that the PoolStats structure is properly defined
	stats := postgres.PoolStats{
		AcquireCount:         100,
		AcquiredConns:        5,
		IdleConns:            3,
		MaxConns:             20,
		TotalConns:           8,
		ConstructingConns:    0,
		CanceledAcquireCount: 2,
		EmptyAcquireCount:    10,
	}

	if stats.MaxConns != 20 {
		t.Errorf("Expected MaxConns to be 20, got %d", stats.MaxConns)
	}

	if stats.AcquiredConns != 5 {
		t.Errorf("Expected AcquiredConns to be 5, got %d", stats.AcquiredConns)
	}

	if stats.IdleConns != 3 {
		t.Errorf("Expected IdleConns to be 3, got %d", stats.IdleConns)
	}
}
