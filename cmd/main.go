package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"web-tracker/config"
	"web-tracker/domain"
	"web-tracker/infrastructure/httpclient"
	"web-tracker/infrastructure/logger"
	"web-tracker/infrastructure/postgres"
	"web-tracker/infrastructure/profiling"
	"web-tracker/infrastructure/redis"
	httpInterface "web-tracker/interface/http"
	"web-tracker/interface/websocket"
	"web-tracker/usecase"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize structured logging
	logger.Init(logger.LevelInfo)
	log := logger.GetLogger()

	// Initialize memory profiler and apply optimizations
	memProfiler := profiling.NewMemoryProfiler()
	memProfiler.OptimizeForLowMemory()
	memProfiler.LogMemoryStats("startup")

	// Parse command line flags
	var configFile string
	var generateConfig string
	var healthCheck bool
	flag.StringVar(&configFile, "config", "", "Path to configuration file")
	flag.StringVar(&generateConfig, "generate-config", "", "Generate sample configuration file and exit")
	flag.BoolVar(&healthCheck, "health-check", false, "Perform health check and exit")
	flag.Parse()

	// Generate sample config if requested
	if generateConfig != "" {
		if err := config.CreateSampleConfig(generateConfig); err != nil {
			log.ErrorWithErr("Failed to generate sample config", err, logger.Fields{
				"config_file": generateConfig,
			})
			os.Exit(1)
		}
		log.Info("Sample configuration file created", logger.Fields{
			"config_file": generateConfig,
		})
		return
	}

	// Perform health check if requested
	if healthCheck {
		// Simple health check - just verify we can load config
		_, err := config.LoadAndValidateWithConnectivity(ctx, configFile)
		if err != nil {
			log.ErrorWithErr("Health check failed", err)
			os.Exit(1)
		}
		log.Info("Health check passed")
		return
	}

	// Load and validate configuration
	cfg, err := config.LoadAndValidateWithConnectivity(ctx, configFile)
	if err != nil {
		log.ErrorWithErr("Configuration error", err, logger.Fields{
			"config_file": configFile,
		})
		os.Exit(1)
	}

	log.Info("Configuration loaded and validated successfully")
	memProfiler.LogMemoryStats("config_loaded")

	// Create PostgreSQL connection pool using config
	poolConfig := postgres.DefaultPoolConfig()
	poolConfig.Host = cfg.Database.Host
	poolConfig.Port = cfg.Database.Port
	poolConfig.Database = cfg.Database.Database
	poolConfig.User = cfg.Database.User
	poolConfig.Password = cfg.Database.Password
	// Apply memory-optimized connection pool settings
	poolConfig.MaxConns = int32(cfg.Database.MaxConnections)
	poolConfig.MinConns = int32(cfg.Database.MinConnections)

	pool, err := postgres.NewPool(ctx, poolConfig)
	if err != nil {
		log.ErrorWithErr("Failed to create database pool", err, logger.Fields{
			"host":     cfg.Database.Host,
			"port":     cfg.Database.Port,
			"database": cfg.Database.Database,
		})
		os.Exit(1)
	}
	defer pool.Close()

	log.Info("Connected to PostgreSQL", logger.Fields{
		"host":     cfg.Database.Host,
		"database": cfg.Database.Database,
	})
	memProfiler.LogMemoryStats("database_connected")

	// Log initial database connection pool stats
	postgres.LogPoolStats(pool, "initial_connection")

	// Start database connection pool monitoring (check every 60 seconds)
	postgres.StartPoolMonitoring(ctx, pool, 60*time.Second)

	// Run database migrations
	if err := postgres.RunMigrations(ctx, pool); err != nil {
		log.ErrorWithErr("Failed to run migrations", err)
		os.Exit(1)
	}

	log.Info("Database migrations completed")

	// Create base repositories
	baseMonitorRepo := postgres.NewMonitorRepository(pool)
	healthCheckRepo := postgres.NewHealthCheckRepository(pool)
	alertRepo := postgres.NewAlertRepository(pool)
	userRepo := postgres.NewUserRepository(pool)
	sessionRepo := postgres.NewSessionRepository(pool)

	// Create Redis client (if enabled)
	var redisClient *redis.Client
	if cfg.Redis.Enabled {
		redisConfig := redis.DefaultConfig()
		redisConfig.Addr = cfg.Redis.Addr
		redisConfig.Password = cfg.Redis.Password
		redisConfig.DB = cfg.Redis.DB

		redisClient, err = redis.NewClient(redisConfig)
		if err != nil {
			log.Warn("Failed to connect to Redis, continuing without cache", logger.Fields{
				"error": err.Error(),
				"addr":  cfg.Redis.Addr,
			})
			// Continue without Redis - system will work with reduced performance
		} else {
			log.Info("Connected to Redis", logger.Fields{
				"addr": cfg.Redis.Addr,
			})
		}
	} else {
		log.Info("Redis disabled in configuration")
	}

	// Create monitor repository with caching if Redis is available
	var monitorRepo domain.MonitorRepository
	if redisClient != nil {
		monitorRepo = postgres.NewCachedMonitorRepository(baseMonitorRepo, redisClient)
		log.Info("Monitor repository configured with Redis caching (5-minute TTL)")
	} else {
		monitorRepo = baseMonitorRepo
		log.Info("Monitor repository configured without caching")
	}

	// Create services
	metricsService := usecase.NewMetricsService(healthCheckRepo, redisClient)

	// Create WebSocket manager
	websocketManager := websocket.NewWebSocketManager()
	if err := websocketManager.Start(ctx); err != nil {
		log.ErrorWithErr("Failed to start WebSocket manager", err)
		os.Exit(1)
	}

	// Create HTTP client for health checks
	httpClient := httpclient.NewClient(httpclient.DefaultConfig())

	// Create worker pool
	var workerPool usecase.WorkerPool
	if redisClient != nil {
		// Create health check service first for worker pool
		tempHealthCheckService := usecase.NewHealthCheckService(
			httpClient,
			healthCheckRepo,
			monitorRepo,
			redisClient,
			nil, // alert service will be set later
			websocketManager,
		)

		workerPool = usecase.NewWorkerPool(redisClient, tempHealthCheckService, "health_check_queue")
		workerPool.Start(ctx, cfg.Worker.PoolSize)
		log.Info("Worker pool started", logger.Fields{
			"pool_size": cfg.Worker.PoolSize,
		})
	} else {
		log.Warn("Worker pool disabled - Redis not available")
	}

	// Create scheduler (requires Redis)
	var scheduler usecase.Scheduler
	if redisClient != nil {
		schedulerConfig := usecase.SchedulerConfig{
			ScheduleName: "health_check_schedule",
			QueueName:    "health_check_queue",
			TickInterval: 10 * time.Second,
		}
		scheduler = usecase.NewScheduler(redisClient, schedulerConfig)
		if err := scheduler.Start(ctx); err != nil {
			log.ErrorWithErr("Failed to start scheduler", err)
			os.Exit(1)
		}
		log.Info("Scheduler started with Redis backend")
	} else {
		log.Warn("Scheduler disabled - Redis not available")
	}

	// Create monitor service with scheduler
	monitorService := usecase.NewMonitorService(monitorRepo, scheduler)

	// Create health check service with worker pool
	healthCheckService := usecase.NewHealthCheckService(
		httpClient,
		healthCheckRepo,
		monitorRepo,
		redisClient,
		nil, // alert service - will be implemented separately
		websocketManager,
	)

	// Create auth service
	authService := usecase.NewAuthService(userRepo, sessionRepo, cfg.Session.TTL)

	// Create HTTP server using config
	serverConfig := httpInterface.DefaultServerConfig()
	serverConfig.Port = cfg.Server.Port

	server := httpInterface.NewServer(
		serverConfig,
		monitorService,
		healthCheckService,
		healthCheckRepo,
		alertRepo,
		metricsService,
		workerPool,
		scheduler,
		monitorRepo,
		websocketManager,
		authService,
	)

	log.Info("Starting HTTP server", logger.Fields{
		"host": cfg.Server.Host,
		"port": cfg.Server.Port,
	})

	// Start server in background
	go func() {
		if err := server.Start(ctx); err != nil {
			log.ErrorWithErr("HTTP server error", err)
		}
	}()

	log.Info("Application ready")
	memProfiler.LogMemoryStats("application_ready")

	// Start memory monitoring (check every 30 seconds, warn if > 90MB)
	memProfiler.StartMemoryMonitoring(ctx, 30*time.Second, 90.0)

	// Check initial memory requirements
	memProfiler.CheckMemoryRequirements(10.0, "idle_startup")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Info("Shutting down...")
	memProfiler.LogMemoryStats("shutdown_start")
	cancel() // Cancel context to stop server
}
