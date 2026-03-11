package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"web-tracker/infrastructure/postgres"
)

func main() {
	var (
		dbURL    = flag.String("url", "", "Database URL (postgres://user:pass@host:port/db)")
		host     = flag.String("host", "localhost", "Database host")
		port     = flag.Int("port", 5432, "Database port")
		database = flag.String("database", "uptime_monitor", "Database name")
		user     = flag.String("user", "postgres", "Database user")
		password = flag.String("password", "", "Database password")
		sslMode  = flag.String("ssl-mode", "prefer", "SSL mode")
		up       = flag.Bool("up", false, "Run migrations up")
		help     = flag.Bool("help", false, "Show help")
	)
	flag.Parse()

	if *help {
		fmt.Println("Database Migration Tool")
		fmt.Println("Usage:")
		fmt.Println("  migrate -up                    # Run all pending migrations")
		fmt.Println("  migrate -url postgres://...    # Use database URL")
		fmt.Println("  migrate -host localhost -port 5432 -database uptime_monitor -user postgres -password secret -up")
		flag.PrintDefaults()
		return
	}

	ctx := context.Background()

	// Create pool config
	var poolConfig *postgres.PoolConfig
	if *dbURL != "" {
		var err error
		poolConfig, err = postgres.ParseDatabaseURL(*dbURL)
		if err != nil {
			log.Fatalf("Failed to parse database URL: %v", err)
		}
	} else {
		poolConfig = &postgres.PoolConfig{
			Host:     *host,
			Port:     *port,
			Database: *database,
			User:     *user,
			Password: *password,
			SSLMode:  *sslMode,
		}
	}

	// Create connection pool
	pool, err := postgres.NewPool(ctx, *poolConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	if *up {
		fmt.Println("Running migrations...")
		if err := postgres.RunMigrations(ctx, pool); err != nil {
			log.Fatalf("Migration failed: %v", err)
		}
		fmt.Println("Migrations completed successfully!")
	} else {
		fmt.Println("Use -up flag to run migrations")
		fmt.Println("Use -help for more options")
	}
}
