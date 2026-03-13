package http

import (
	"github.com/gin-gonic/gin"

	"web-tracker/domain"
	"web-tracker/interface/websocket"
	"web-tracker/usecase"
)

// Router sets up HTTP routes for the API
type Router struct {
	monitorHandler     *MonitorHandler
	healthCheckHandler *HealthCheckHandler
	alertHandler       *AlertHandler
	metricsHandler     *MetricsHandler
	systemHandler      *SystemHandler
	dashboardHandler   *DashboardHandler
	formHandler        *FormHandler
	authHandler        *AuthHandler
	authMiddleware     *AuthMiddleware
	websocketHandler   *websocket.Handler
}

// NewRouter creates a new HTTP router with all handlers
func NewRouter(
	monitorService *usecase.MonitorService,
	healthCheckService *usecase.HealthCheckService,
	healthCheckRepo domain.HealthCheckRepository,
	alertRepo domain.AlertRepository,
	metricsService usecase.MetricsService,
	workerPool usecase.WorkerPool,
	scheduler usecase.Scheduler,
	monitorRepo domain.MonitorRepository,
	websocketManager websocket.Manager,
	authService *usecase.AuthService,
) *Router {
	return &Router{
		monitorHandler:     NewMonitorHandler(monitorService),
		healthCheckHandler: NewHealthCheckHandler(healthCheckService, healthCheckRepo),
		alertHandler:       NewAlertHandler(alertRepo),
		metricsHandler:     NewMetricsHandler(metricsService),
		systemHandler:      NewSystemHandler(workerPool, scheduler, healthCheckRepo, monitorRepo),
		dashboardHandler:   NewDashboardHandler(monitorService, healthCheckRepo, alertRepo, metricsService),
		formHandler:        NewFormHandler(monitorService),
		authHandler:        NewAuthHandler(authService),
		authMiddleware:     NewAuthMiddleware(authService),
		websocketHandler:   websocketManager.GetHandler(),
	}
}

// SetupRoutes configures all API routes
func (r *Router) SetupRoutes(engine *gin.Engine) {
	// Static assets (no auth required)
	engine.Static("/static", "./static")

	// Public auth routes (no authentication required)
	engine.GET("/login", r.authHandler.ShowLoginForm)
	engine.POST("/login", r.authHandler.Login)
	engine.GET("/register", r.authHandler.ShowRegisterForm)
	engine.POST("/register", r.authHandler.Register)
	engine.POST("/logout", r.authHandler.Logout)

	// System health endpoints (no auth required — used by load balancers/probes)
	health := engine.Group("/health")
	{
		health.GET("", r.systemHandler.Health)      // GET /health
		health.GET("/ready", r.systemHandler.Ready) // GET /health/ready
		health.GET("/live", r.systemHandler.Live)   // GET /health/live
	}

	// Metrics endpoint (no auth required — used by monitoring systems)
	engine.GET("/metrics", r.systemHandler.Metrics) // GET /metrics

	// Protected routes — require authentication
	protected := engine.Group("/")
	protected.Use(r.authMiddleware.RequireAuth())
	protected.Use(r.authMiddleware.CSRFProtection())
	{
		// Dashboard routes (HTML)
		protected.GET("/", r.dashboardHandler.Dashboard)                       // GET / - Main dashboard
		protected.GET("/monitors", r.dashboardHandler.MonitorList)             // GET /monitors - Monitor list page
		protected.GET("/monitors/:id", r.dashboardHandler.MonitorDetail)       // GET /monitors/:id - Monitor detail page
		protected.GET("/monitors/:id/alerts", r.dashboardHandler.AlertHistory) // GET /monitors/:id/alerts - Alert history page

		// Form routes (HTML)
		protected.GET("/monitors/new", r.formHandler.NewMonitorForm)       // GET /monitors/new - New monitor form
		protected.POST("/monitors", r.formHandler.CreateMonitorForm)       // POST /monitors - Create monitor (form)
		protected.GET("/monitors/:id/edit", r.formHandler.EditMonitorForm) // GET /monitors/:id/edit - Edit monitor form
		protected.POST("/monitors/:id", r.formHandler.UpdateMonitorForm)   // POST /monitors/:id - Update monitor (form with _method=PUT)
		protected.POST("/monitors/:id/delete", r.formHandler.DeleteMonitorForm) // POST /monitors/:id/delete - Delete monitor
	}

	// API v1 routes (protected)
	v1 := engine.Group("/api/v1")
	v1.Use(r.authMiddleware.RequireAuth())
	{
		// Dashboard endpoint
		v1.GET("/dashboard", r.dashboardHandler.DashboardAPI) // GET /api/v1/dashboard

		// Monitor endpoints
		monitors := v1.Group("/monitors")
		{
			monitors.POST("", r.monitorHandler.CreateMonitor)       // POST /api/v1/monitors
			monitors.GET("", r.monitorHandler.ListMonitors)         // GET /api/v1/monitors
			monitors.GET("/:id", r.monitorHandler.GetMonitor)       // GET /api/v1/monitors/:id
			monitors.PUT("/:id", r.monitorHandler.UpdateMonitor)    // PUT /api/v1/monitors/:id
			monitors.DELETE("/:id", r.monitorHandler.DeleteMonitor) // DELETE /api/v1/monitors/:id

			// Health check endpoints
			monitors.GET("/:id/checks", r.healthCheckHandler.GetHealthCheckHistory)       // GET /api/v1/monitors/:id/checks
			monitors.GET("/:id/checks/latest", r.healthCheckHandler.GetLatestHealthCheck) // GET /api/v1/monitors/:id/checks/latest

			// Alert endpoints
			monitors.GET("/:id/alerts", r.alertHandler.GetAlertHistory) // GET /api/v1/monitors/:id/alerts

			// Metrics endpoints
			monitors.GET("/:id/uptime", r.metricsHandler.GetUptimeMetrics)         // GET /api/v1/monitors/:id/uptime
			monitors.GET("/:id/response", r.metricsHandler.GetResponseTimeMetrics) // GET /api/v1/monitors/:id/response
		}
	}

	// WebSocket endpoint (protected)
	if r.websocketHandler != nil {
		ws := engine.Group("/")
		ws.Use(r.authMiddleware.RequireAuth())
		ws.GET("/ws", r.websocketHandler.HandleWebSocket) // WS /ws
	}
}
