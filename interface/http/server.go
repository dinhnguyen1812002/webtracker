package http

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"web-tracker/domain"
	"web-tracker/interface/websocket"
	"web-tracker/usecase"
)

// Server represents the HTTP server
type Server struct {
	router     *Router
	httpServer *http.Server
	port       int
}

// ServerConfig holds configuration for the HTTP server
type ServerConfig struct {
	Port              int
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ReadHeaderTimeout time.Duration
}

// DefaultServerConfig returns default server configuration
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Port:              8080,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
	}
}

// NewServer creates a new HTTP server
func NewServer(
	config ServerConfig,
	monitorService *usecase.MonitorService,
	healthCheckService *usecase.HealthCheckService,
	healthCheckRepo domain.HealthCheckRepository,
	alertRepo domain.AlertRepository,
	metricsService usecase.MetricsService,
	workerPool usecase.WorkerPool,
	scheduler usecase.Scheduler,
	monitorRepo domain.MonitorRepository,
	websocketManager websocket.Manager,
) *Server {
	// Create router with all handlers
	router := NewRouter(
		monitorService,
		healthCheckService,
		healthCheckRepo,
		alertRepo,
		metricsService,
		workerPool,
		scheduler,
		monitorRepo,
		websocketManager,
	)

	return &Server{
		router: router,
		port:   config.Port,
	}
}

// Start starts the HTTP server
func (s *Server) Start(ctx context.Context) error {
	// Create Gin engine
	gin.SetMode(gin.ReleaseMode) // Set to release mode for production
	engine := gin.New()

	// Add middleware
	engine.Use(gin.Logger())
	engine.Use(gin.Recovery())
	engine.Use(s.corsMiddleware())

	// Setup routes
	s.router.SetupRoutes(engine)

	// Create HTTP server
	s.httpServer = &http.Server{
		Addr:              fmt.Sprintf(":%d", s.port),
		Handler:           engine,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		fmt.Printf("Starting HTTP server on port %d\n", s.port)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("HTTP server error: %v\n", err)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()

	// Graceful shutdown
	return s.Stop()
}

// Stop gracefully stops the HTTP server
func (s *Server) Stop() error {
	if s.httpServer == nil {
		return nil
	}

	fmt.Println("Shutting down HTTP server...")

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown server
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown HTTP server: %w", err)
	}

	fmt.Println("HTTP server stopped")
	return nil
}

// corsMiddleware adds CORS headers for cross-origin requests
func (s *Server) corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
