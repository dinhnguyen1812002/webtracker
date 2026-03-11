package http

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"web-tracker/domain"
	"web-tracker/usecase"
)

// MonitorServiceInterface defines the interface for monitor operations
type MonitorServiceInterface interface {
	CreateMonitor(ctx context.Context, req usecase.CreateMonitorRequest) (*domain.Monitor, error)
	GetMonitor(ctx context.Context, id string) (*domain.Monitor, error)
	ListMonitors(ctx context.Context, filters domain.ListFilters) ([]*domain.Monitor, error)
	UpdateMonitor(ctx context.Context, id string, req usecase.UpdateMonitorRequest) (*domain.Monitor, error)
	DeleteMonitor(ctx context.Context, id string) error
}

// MonitorHandler handles HTTP requests for monitor operations
type MonitorHandler struct {
	monitorService MonitorServiceInterface
}

// NewMonitorHandler creates a new monitor handler
func NewMonitorHandler(monitorService MonitorServiceInterface) *MonitorHandler {
	return &MonitorHandler{
		monitorService: monitorService,
	}
}

// CreateMonitorRequest represents the JSON request for creating a monitor
type CreateMonitorRequest struct {
	Name          string                `json:"name" binding:"required"`
	URL           string                `json:"url" binding:"required"`
	CheckInterval int                   `json:"check_interval" binding:"required"` // in minutes
	Enabled       bool                  `json:"enabled"`
	AlertChannels []domain.AlertChannel `json:"alert_channels"`
}

// UpdateMonitorRequest represents the JSON request for updating a monitor
type UpdateMonitorRequest struct {
	Name          *string               `json:"name,omitempty"`
	URL           *string               `json:"url,omitempty"`
	CheckInterval *int                  `json:"check_interval,omitempty"` // in minutes
	Enabled       *bool                 `json:"enabled,omitempty"`
	AlertChannels []domain.AlertChannel `json:"alert_channels,omitempty"`
}

// MonitorResponse represents the JSON response for monitor operations
type MonitorResponse struct {
	ID            string                `json:"id"`
	Name          string                `json:"name"`
	URL           string                `json:"url"`
	CheckInterval int                   `json:"check_interval"` // in minutes
	Enabled       bool                  `json:"enabled"`
	AlertChannels []domain.AlertChannel `json:"alert_channels"`
	CreatedAt     time.Time             `json:"created_at"`
	UpdatedAt     time.Time             `json:"updated_at"`
}

// CreateMonitor handles POST /api/v1/monitors
// Requirements: 10.1, 10.2, 10.6
func (h *MonitorHandler) CreateMonitor(c *gin.Context) {
	var req CreateMonitorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert check interval from minutes to duration
	checkInterval := time.Duration(req.CheckInterval) * time.Minute

	// Create monitor using service
	createReq := usecase.CreateMonitorRequest{
		Name:          req.Name,
		URL:           req.URL,
		CheckInterval: checkInterval,
		Enabled:       req.Enabled,
		AlertChannels: req.AlertChannels,
	}

	monitor, err := h.monitorService.CreateMonitor(c.Request.Context(), createReq)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert to response format
	response := h.toMonitorResponse(monitor)
	c.JSON(http.StatusCreated, response)
}

// GetMonitor handles GET /api/v1/monitors/:id
// Requirements: 10.5
func (h *MonitorHandler) GetMonitor(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "monitor ID is required"})
		return
	}

	monitor, err := h.monitorService.GetMonitor(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	response := h.toMonitorResponse(monitor)
	c.JSON(http.StatusOK, response)
}

// ListMonitors handles GET /api/v1/monitors
// Requirements: 10.5
func (h *MonitorHandler) ListMonitors(c *gin.Context) {
	// Parse query parameters
	filters := domain.ListFilters{}

	// Parse enabled filter
	if enabledStr := c.Query("enabled"); enabledStr != "" {
		enabled, err := strconv.ParseBool(enabledStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid enabled parameter"})
			return
		}
		filters.Enabled = &enabled
	}

	// Parse limit
	if limitStr := c.Query("limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit parameter"})
			return
		}
		filters.Limit = limit
	} else {
		filters.Limit = 100 // Default limit
	}

	// Parse offset
	if offsetStr := c.Query("offset"); offsetStr != "" {
		offset, err := strconv.Atoi(offsetStr)
		if err != nil || offset < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid offset parameter"})
			return
		}
		filters.Offset = offset
	}

	monitors, err := h.monitorService.ListMonitors(c.Request.Context(), filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Convert to response format
	responses := make([]MonitorResponse, len(monitors))
	for i, monitor := range monitors {
		responses[i] = h.toMonitorResponse(monitor)
	}

	c.JSON(http.StatusOK, gin.H{
		"monitors": responses,
		"count":    len(responses),
	})
}

// UpdateMonitor handles PUT /api/v1/monitors/:id
// Requirements: 10.3, 10.6
func (h *MonitorHandler) UpdateMonitor(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "monitor ID is required"})
		return
	}

	var req UpdateMonitorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert to service request
	updateReq := usecase.UpdateMonitorRequest{
		Name:          req.Name,
		URL:           req.URL,
		Enabled:       req.Enabled,
		AlertChannels: req.AlertChannels,
	}

	// Convert check interval from minutes to duration if provided
	if req.CheckInterval != nil {
		checkInterval := time.Duration(*req.CheckInterval) * time.Minute
		updateReq.CheckInterval = &checkInterval
	}

	monitor, err := h.monitorService.UpdateMonitor(c.Request.Context(), id, updateReq)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response := h.toMonitorResponse(monitor)
	c.JSON(http.StatusOK, response)
}

// DeleteMonitor handles DELETE /api/v1/monitors/:id
// Requirements: 10.4
func (h *MonitorHandler) DeleteMonitor(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "monitor ID is required"})
		return
	}

	err := h.monitorService.DeleteMonitor(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// toMonitorResponse converts a domain Monitor to MonitorResponse
func (h *MonitorHandler) toMonitorResponse(monitor *domain.Monitor) MonitorResponse {
	return MonitorResponse{
		ID:            monitor.ID,
		Name:          monitor.Name,
		URL:           monitor.URL,
		CheckInterval: int(monitor.CheckInterval.Minutes()),
		Enabled:       monitor.Enabled,
		AlertChannels: monitor.AlertChannels,
		CreatedAt:     monitor.CreatedAt,
		UpdatedAt:     monitor.UpdatedAt,
	}
}

// HealthCheckHandler handles HTTP requests for health check operations
type HealthCheckHandler struct {
	healthCheckService *usecase.HealthCheckService
	healthCheckRepo    domain.HealthCheckRepository
}

// NewHealthCheckHandler creates a new health check handler
func NewHealthCheckHandler(healthCheckService *usecase.HealthCheckService, healthCheckRepo domain.HealthCheckRepository) *HealthCheckHandler {
	return &HealthCheckHandler{
		healthCheckService: healthCheckService,
		healthCheckRepo:    healthCheckRepo,
	}
}

// HealthCheckResponse represents the JSON response for health check operations
type HealthCheckResponse struct {
	ID           string                   `json:"id"`
	MonitorID    string                   `json:"monitor_id"`
	Status       domain.HealthCheckStatus `json:"status"`
	StatusCode   int                      `json:"status_code"`
	ResponseTime int64                    `json:"response_time_ms"`
	SSLInfo      *domain.SSLInfo          `json:"ssl_info,omitempty"`
	ErrorMessage string                   `json:"error_message,omitempty"`
	CheckedAt    time.Time                `json:"checked_at"`
}

// GetHealthCheckHistory handles GET /api/v1/monitors/:id/checks
// Requirements: 14.3, 14.5
func (h *HealthCheckHandler) GetHealthCheckHistory(c *gin.Context) {
	monitorID := c.Param("id")
	if monitorID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "monitor ID is required"})
		return
	}

	// Parse query parameters
	limit := 100 // Default limit
	if limitStr := c.Query("limit"); limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err != nil || parsedLimit < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit parameter"})
			return
		}
		if parsedLimit > 1000 {
			parsedLimit = 1000 // Maximum limit
		}
		limit = parsedLimit
	}

	// Parse date range filters
	var start, end time.Time
	var err error

	if startStr := c.Query("start"); startStr != "" {
		start, err = time.Parse(time.RFC3339, startStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid start date format, use RFC3339"})
			return
		}
	}

	if endStr := c.Query("end"); endStr != "" {
		end, err = time.Parse(time.RFC3339, endStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid end date format, use RFC3339"})
			return
		}
	}

	var checks []*domain.HealthCheck

	// Use date range query if both start and end are provided
	if !start.IsZero() && !end.IsZero() {
		checks, err = h.healthCheckRepo.GetByDateRange(c.Request.Context(), monitorID, start, end)
	} else {
		// Use simple limit-based query
		checks, err = h.healthCheckRepo.GetByMonitorID(c.Request.Context(), monitorID, limit)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Convert to response format
	responses := make([]HealthCheckResponse, len(checks))
	for i, check := range checks {
		responses[i] = h.toHealthCheckResponse(check)
	}

	c.JSON(http.StatusOK, gin.H{
		"checks": responses,
		"count":  len(responses),
	})
}

// GetLatestHealthCheck handles GET /api/v1/monitors/:id/checks/latest
// Requirements: 14.3, 14.5
func (h *HealthCheckHandler) GetLatestHealthCheck(c *gin.Context) {
	monitorID := c.Param("id")
	if monitorID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "monitor ID is required"})
		return
	}

	// Get the latest health check (limit 1)
	checks, err := h.healthCheckRepo.GetByMonitorID(c.Request.Context(), monitorID, 1)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if len(checks) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "no health checks found for this monitor"})
		return
	}

	response := h.toHealthCheckResponse(checks[0])
	c.JSON(http.StatusOK, response)
}

// toHealthCheckResponse converts a domain HealthCheck to HealthCheckResponse
func (h *HealthCheckHandler) toHealthCheckResponse(check *domain.HealthCheck) HealthCheckResponse {
	return HealthCheckResponse{
		ID:           check.ID,
		MonitorID:    check.MonitorID,
		Status:       check.Status,
		StatusCode:   check.StatusCode,
		ResponseTime: check.ResponseTime.Milliseconds(),
		SSLInfo:      check.SSLInfo,
		ErrorMessage: check.ErrorMessage,
		CheckedAt:    check.CheckedAt,
	}
}

// AlertHandler handles HTTP requests for alert operations
type AlertHandler struct {
	alertRepo domain.AlertRepository
}

// NewAlertHandler creates a new alert handler
func NewAlertHandler(alertRepo domain.AlertRepository) *AlertHandler {
	return &AlertHandler{
		alertRepo: alertRepo,
	}
}

// AlertResponse represents the JSON response for alert operations
type AlertResponse struct {
	ID        string                    `json:"id"`
	MonitorID string                    `json:"monitor_id"`
	Type      domain.AlertType          `json:"type"`
	Severity  domain.AlertSeverity      `json:"severity"`
	Message   string                    `json:"message"`
	Details   map[string]interface{}    `json:"details"`
	SentAt    time.Time                 `json:"sent_at"`
	Channels  []domain.AlertChannelType `json:"channels"`
}

// GetAlertHistory handles GET /api/v1/monitors/:id/alerts
// Requirements: 13.3, 13.4, 13.5
func (h *AlertHandler) GetAlertHistory(c *gin.Context) {
	monitorID := c.Param("id")
	if monitorID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "monitor ID is required"})
		return
	}

	// Parse query parameters
	limit := 100 // Default limit
	if limitStr := c.Query("limit"); limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err != nil || parsedLimit < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit parameter"})
			return
		}
		if parsedLimit > 1000 {
			parsedLimit = 1000 // Maximum limit
		}
		limit = parsedLimit
	}

	// Parse date range filters
	var start, end time.Time
	var err error

	if startStr := c.Query("start"); startStr != "" {
		start, err = time.Parse(time.RFC3339, startStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid start date format, use RFC3339"})
			return
		}
	}

	if endStr := c.Query("end"); endStr != "" {
		end, err = time.Parse(time.RFC3339, endStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid end date format, use RFC3339"})
			return
		}
	}

	var alerts []*domain.Alert

	// Use date range query if both start and end are provided
	if !start.IsZero() && !end.IsZero() {
		alerts, err = h.alertRepo.GetByDateRange(c.Request.Context(), monitorID, start, end)
	} else {
		// Use simple limit-based query
		alerts, err = h.alertRepo.GetByMonitorID(c.Request.Context(), monitorID, limit)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Filter by alert type if specified
	if alertTypeStr := c.Query("type"); alertTypeStr != "" {
		alertType := domain.AlertType(alertTypeStr)
		filteredAlerts := make([]*domain.Alert, 0)
		for _, alert := range alerts {
			if alert.Type == alertType {
				filteredAlerts = append(filteredAlerts, alert)
			}
		}
		alerts = filteredAlerts
	}

	// Convert to response format
	responses := make([]AlertResponse, len(alerts))
	for i, alert := range alerts {
		responses[i] = h.toAlertResponse(alert)
	}

	c.JSON(http.StatusOK, gin.H{
		"alerts": responses,
		"count":  len(responses),
	})
}

// toAlertResponse converts a domain Alert to AlertResponse
func (h *AlertHandler) toAlertResponse(alert *domain.Alert) AlertResponse {
	return AlertResponse{
		ID:        alert.ID,
		MonitorID: alert.MonitorID,
		Type:      alert.Type,
		Severity:  alert.Severity,
		Message:   alert.Message,
		Details:   alert.Details,
		SentAt:    alert.SentAt,
		Channels:  alert.Channels,
	}
}

// MetricsHandler handles HTTP requests for metrics operations
type MetricsHandler struct {
	metricsService usecase.MetricsService
}

// NewMetricsHandler creates a new metrics handler
func NewMetricsHandler(metricsService usecase.MetricsService) *MetricsHandler {
	return &MetricsHandler{
		metricsService: metricsService,
	}
}

// UptimeResponse represents the JSON response for uptime metrics
type UptimeResponse struct {
	MonitorID string  `json:"monitor_id"`
	Period24h float64 `json:"period_24h"`
	Period7d  float64 `json:"period_7d"`
	Period30d float64 `json:"period_30d"`
}

// ResponseTimeResponse represents the JSON response for response time metrics
type ResponseTimeResponse struct {
	MonitorID string `json:"monitor_id"`
	Period    string `json:"period"`
	Average   int64  `json:"average_ms"`
	Min       int64  `json:"min_ms"`
	Max       int64  `json:"max_ms"`
	P95       int64  `json:"p95_ms"`
	P99       int64  `json:"p99_ms"`
}

// GetUptimeMetrics handles GET /api/v1/monitors/:id/uptime
// Requirements: 7.1
func (h *MetricsHandler) GetUptimeMetrics(c *gin.Context) {
	monitorID := c.Param("id")
	if monitorID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "monitor ID is required"})
		return
	}

	stats, err := h.metricsService.GetUptimePercentage(c.Request.Context(), monitorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	response := UptimeResponse{
		MonitorID: monitorID,
		Period24h: stats.Period24h,
		Period7d:  stats.Period7d,
		Period30d: stats.Period30d,
	}

	c.JSON(http.StatusOK, response)
}

// GetResponseTimeMetrics handles GET /api/v1/monitors/:id/response
// Requirements: 8.3
func (h *MetricsHandler) GetResponseTimeMetrics(c *gin.Context) {
	monitorID := c.Param("id")
	if monitorID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "monitor ID is required"})
		return
	}

	// Parse period parameter (default to 24h)
	periodStr := c.DefaultQuery("period", "24h")
	var period time.Duration
	var err error

	switch periodStr {
	case "1h":
		period = time.Hour
	case "24h":
		period = 24 * time.Hour
	case "7d":
		period = 7 * 24 * time.Hour
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid period, use 1h, 24h, or 7d"})
		return
	}

	stats, err := h.metricsService.GetResponseTimeStats(c.Request.Context(), monitorID, period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	response := ResponseTimeResponse{
		MonitorID: monitorID,
		Period:    periodStr,
		Average:   stats.Average.Milliseconds(),
		Min:       stats.Min.Milliseconds(),
		Max:       stats.Max.Milliseconds(),
		P95:       stats.P95.Milliseconds(),
		P99:       stats.P99.Milliseconds(),
	}

	c.JSON(http.StatusOK, response)
}

// SystemHandler handles HTTP requests for system health and metrics
type SystemHandler struct {
	workerPool      usecase.WorkerPool
	scheduler       usecase.Scheduler
	healthCheckRepo domain.HealthCheckRepository
	monitorRepo     domain.MonitorRepository
}

// NewSystemHandler creates a new system handler
func NewSystemHandler(
	workerPool usecase.WorkerPool,
	scheduler usecase.Scheduler,
	healthCheckRepo domain.HealthCheckRepository,
	monitorRepo domain.MonitorRepository,
) *SystemHandler {
	return &SystemHandler{
		workerPool:      workerPool,
		scheduler:       scheduler,
		healthCheckRepo: healthCheckRepo,
		monitorRepo:     monitorRepo,
	}
}

// HealthResponse represents the JSON response for health check endpoints
type HealthResponse struct {
	Status    string            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Checks    map[string]string `json:"checks,omitempty"`
}

// MetricsResponse represents the JSON response for metrics endpoint
type MetricsResponse struct {
	WorkerPool WorkerPoolMetrics `json:"worker_pool"`
	Database   DatabaseMetrics   `json:"database"`
	System     SystemMetrics     `json:"system"`
}

// WorkerPoolMetrics represents worker pool metrics
type WorkerPoolMetrics struct {
	ActiveWorkers int64 `json:"active_workers"`
	QueueDepth    int64 `json:"queue_depth"`
	ProcessedJobs int64 `json:"processed_jobs"`
	IsRunning     bool  `json:"is_running"`
}

// DatabaseMetrics represents database metrics
type DatabaseMetrics struct {
	TotalMonitors    int `json:"total_monitors"`
	EnabledMonitors  int `json:"enabled_monitors"`
	DisabledMonitors int `json:"disabled_monitors"`
}

// SystemMetrics represents system metrics
type SystemMetrics struct {
	SchedulerRunning bool      `json:"scheduler_running"`
	Uptime           string    `json:"uptime"`
	Timestamp        time.Time `json:"timestamp"`
}

// Health handles GET /health
// Requirements: 15.1
func (h *SystemHandler) Health(c *gin.Context) {
	response := HealthResponse{
		Status:    "ok",
		Timestamp: time.Now(),
	}

	c.JSON(http.StatusOK, response)
}

// Ready handles GET /health/ready
// Requirements: 15.2
func (h *SystemHandler) Ready(c *gin.Context) {
	checks := make(map[string]string)

	// Check database connectivity
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	// Try to query monitors to test database connectivity
	_, err := h.monitorRepo.List(ctx, domain.ListFilters{Limit: 1})
	if err != nil {
		checks["database"] = "failed"
		c.JSON(http.StatusServiceUnavailable, HealthResponse{
			Status:    "not_ready",
			Timestamp: time.Now(),
			Checks:    checks,
		})
		return
	}
	checks["database"] = "ok"

	// Check worker pool status
	if h.workerPool != nil && h.workerPool.IsRunning() {
		checks["worker_pool"] = "ok"
	} else {
		checks["worker_pool"] = "not_running"
	}

	// Check scheduler status
	if h.scheduler != nil && h.scheduler.IsRunning() {
		checks["scheduler"] = "ok"
	} else {
		checks["scheduler"] = "not_running"
	}

	response := HealthResponse{
		Status:    "ready",
		Timestamp: time.Now(),
		Checks:    checks,
	}

	c.JSON(http.StatusOK, response)
}

// Live handles GET /health/live
// Requirements: 15.3
func (h *SystemHandler) Live(c *gin.Context) {
	// Liveness check - just verify the application is running
	response := HealthResponse{
		Status:    "alive",
		Timestamp: time.Now(),
	}

	c.JSON(http.StatusOK, response)
}

// Metrics handles GET /metrics
// Requirements: 15.2, 15.3
func (h *SystemHandler) Metrics(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Get worker pool stats
	var workerPoolMetrics WorkerPoolMetrics
	if h.workerPool != nil {
		stats := h.workerPool.GetStats()
		workerPoolMetrics = WorkerPoolMetrics{
			ActiveWorkers: stats.ActiveWorkers,
			QueueDepth:    stats.QueueDepth,
			ProcessedJobs: stats.ProcessedJobs,
			IsRunning:     h.workerPool.IsRunning(),
		}
	}

	// Get database metrics
	var databaseMetrics DatabaseMetrics

	// Count total monitors
	allMonitors, err := h.monitorRepo.List(ctx, domain.ListFilters{Limit: 10000})
	if err == nil {
		databaseMetrics.TotalMonitors = len(allMonitors)

		// Count enabled/disabled monitors
		for _, monitor := range allMonitors {
			if monitor.Enabled {
				databaseMetrics.EnabledMonitors++
			} else {
				databaseMetrics.DisabledMonitors++
			}
		}
	}

	// Get system metrics
	systemMetrics := SystemMetrics{
		SchedulerRunning: h.scheduler != nil && h.scheduler.IsRunning(),
		Uptime:           "unknown", // Would need to track application start time
		Timestamp:        time.Now(),
	}

	response := MetricsResponse{
		WorkerPool: workerPoolMetrics,
		Database:   databaseMetrics,
		System:     systemMetrics,
	}

	c.JSON(http.StatusOK, response)
}
