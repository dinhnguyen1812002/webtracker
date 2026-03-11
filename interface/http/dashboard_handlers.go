package http

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"web-tracker/domain"
	"web-tracker/interface/http/templates"
	"web-tracker/usecase"
)

// DashboardHandler handles HTTP requests for dashboard pages
type DashboardHandler struct {
	monitorService  MonitorServiceInterface
	healthCheckRepo domain.HealthCheckRepository
	alertRepo       domain.AlertRepository
	metricsService  usecase.MetricsService
}

// NewDashboardHandler creates a new dashboard handler
func NewDashboardHandler(
	monitorService MonitorServiceInterface,
	healthCheckRepo domain.HealthCheckRepository,
	alertRepo domain.AlertRepository,
	metricsService usecase.MetricsService,
) *DashboardHandler {
	return &DashboardHandler{
		monitorService:  monitorService,
		healthCheckRepo: healthCheckRepo,
		alertRepo:       alertRepo,
		metricsService:  metricsService,
	}
}

// Dashboard handles GET /
// Requirements: 9.5
func (h *DashboardHandler) Dashboard(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Get all monitors
	monitors, err := h.monitorService.ListMonitors(ctx, domain.ListFilters{
		Limit: 1000, // Get all monitors for dashboard
	})
	if err != nil {
		c.HTML(http.StatusInternalServerError, "", gin.H{
			"error": "Failed to load monitors",
		})
		return
	}

	// Build dashboard data
	dashboardData := templates.DashboardData{
		Monitors: make([]templates.MonitorWithStatus, 0, len(monitors)),
		Stats: templates.DashboardStats{
			TotalMonitors: len(monitors),
		},
		User: GetUserFromContext(c),
	}

	var totalUptime float64
	var uptimeCount int

	// Get status and metrics for each monitor
	for _, monitor := range monitors {
		monitorWithStatus := templates.MonitorWithStatus{
			Monitor: monitor,
			Status:  domain.HealthCheckStatus("unknown"),
		}

		// Get latest health check
		checks, err := h.healthCheckRepo.GetByMonitorID(ctx, monitor.ID, 1)
		if err == nil && len(checks) > 0 {
			latestCheck := checks[0]
			monitorWithStatus.Status = latestCheck.Status
			monitorWithStatus.LastCheck = &latestCheck.CheckedAt
			monitorWithStatus.ResponseTime = &latestCheck.ResponseTime

			// Count active/failing monitors
			if monitor.Enabled {
				dashboardData.Stats.ActiveMonitors++
				if latestCheck.Status != domain.StatusSuccess {
					dashboardData.Stats.FailingMonitors++
				}
			}
		} else if monitor.Enabled {
			dashboardData.Stats.ActiveMonitors++
		}

		// Get uptime stats
		if h.metricsService != nil {
			uptimeStats, err := h.metricsService.GetUptimePercentage(ctx, monitor.ID)
			if err == nil {
				monitorWithStatus.UptimeStats = &templates.UptimeStats{
					Period24h: uptimeStats.Period24h,
					Period7d:  uptimeStats.Period7d,
					Period30d: uptimeStats.Period30d,
				}
				totalUptime += uptimeStats.Period24h
				uptimeCount++
			}
		}

		dashboardData.Monitors = append(dashboardData.Monitors, monitorWithStatus)
	}

	// Calculate average uptime
	if uptimeCount > 0 {
		dashboardData.Stats.AverageUptime = totalUptime / float64(uptimeCount)
	}

	// Render dashboard template
	component := templates.Dashboard(dashboardData)
	c.Header("Content-Type", "text/html; charset=utf-8")
	err = component.Render(ctx, c.Writer)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "", gin.H{
			"error": "Failed to render dashboard",
		})
		return
	}
}

// MonitorDetail handles GET /monitors/:id
// Requirements: 9.5
func (h *DashboardHandler) MonitorDetail(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	monitorID := c.Param("id")
	if monitorID == "" {
		c.HTML(http.StatusBadRequest, "", gin.H{
			"error": "Monitor ID is required",
		})
		return
	}

	// Get monitor
	monitor, err := h.monitorService.GetMonitor(ctx, monitorID)
	if err != nil {
		c.HTML(http.StatusNotFound, "", gin.H{
			"error": "Monitor not found",
		})
		return
	}

	// Build monitor detail data
	detailData := templates.MonitorDetailData{
		Monitor: monitor,
		User:    GetUserFromContext(c),
	}

	// Get latest health check
	checks, err := h.healthCheckRepo.GetByMonitorID(ctx, monitorID, 1)
	if err == nil && len(checks) > 0 {
		detailData.LatestCheck = checks[0]
	}

	// Get uptime stats
	if h.metricsService != nil {
		uptimeStats, err := h.metricsService.GetUptimePercentage(ctx, monitorID)
		if err == nil {
			detailData.UptimeStats = &templates.UptimeStats{
				Period24h: uptimeStats.Period24h,
				Period7d:  uptimeStats.Period7d,
				Period30d: uptimeStats.Period30d,
			}
		}
	}

	// Get response time stats (24h period)
	if h.metricsService != nil {
		responseStats, err := h.metricsService.GetResponseTimeStats(ctx, monitorID, 24*time.Hour)
		if err == nil {
			detailData.ResponseStats = &templates.ResponseTimeStats{
				Period:  "24h",
				Average: responseStats.Average,
				Min:     responseStats.Min,
				Max:     responseStats.Max,
				P95:     responseStats.P95,
				P99:     responseStats.P99,
			}
		}
	}

	// Get recent health checks (last 10)
	recentChecks, err := h.healthCheckRepo.GetByMonitorID(ctx, monitorID, 10)
	if err == nil {
		detailData.RecentChecks = make([]domain.HealthCheck, len(recentChecks))
		for i, check := range recentChecks {
			detailData.RecentChecks[i] = *check
		}
	}

	// Get recent alerts (last 10)
	recentAlerts, err := h.alertRepo.GetByMonitorID(ctx, monitorID, 10)
	if err == nil {
		detailData.RecentAlerts = make([]domain.Alert, len(recentAlerts))
		for i, alert := range recentAlerts {
			detailData.RecentAlerts[i] = *alert
		}
	}

	// Render monitor detail template
	component := templates.MonitorDetail(detailData)
	c.Header("Content-Type", "text/html; charset=utf-8")
	err = component.Render(ctx, c.Writer)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "", gin.H{
			"error": "Failed to render monitor detail",
		})
		return
	}
}

// MonitorList handles GET /monitors
func (h *DashboardHandler) MonitorList(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Get all monitors
	monitors, err := h.monitorService.ListMonitors(ctx, domain.ListFilters{
		Limit: 1000, // Get all monitors
	})
	if err != nil {
		c.HTML(http.StatusInternalServerError, "", gin.H{
			"error": "Failed to load monitors",
		})
		return
	}

	// Build monitor list data (reuse dashboard logic)
	dashboardData := templates.DashboardData{
		Monitors: make([]templates.MonitorWithStatus, 0, len(monitors)),
		Stats: templates.DashboardStats{
			TotalMonitors: len(monitors),
		},
	}

	var totalUptime float64
	var uptimeCount int

	// Get status and metrics for each monitor
	for _, monitor := range monitors {
		monitorWithStatus := templates.MonitorWithStatus{
			Monitor: monitor,
			Status:  domain.HealthCheckStatus("unknown"),
		}

		// Get latest health check
		checks, err := h.healthCheckRepo.GetByMonitorID(ctx, monitor.ID, 1)
		if err == nil && len(checks) > 0 {
			latestCheck := checks[0]
			monitorWithStatus.Status = latestCheck.Status
			monitorWithStatus.LastCheck = &latestCheck.CheckedAt
			monitorWithStatus.ResponseTime = &latestCheck.ResponseTime

			// Count active/failing monitors
			if monitor.Enabled {
				dashboardData.Stats.ActiveMonitors++
				if latestCheck.Status != domain.StatusSuccess {
					dashboardData.Stats.FailingMonitors++
				}
			}
		} else if monitor.Enabled {
			dashboardData.Stats.ActiveMonitors++
		}

		// Get uptime stats
		if h.metricsService != nil {
			uptimeStats, err := h.metricsService.GetUptimePercentage(ctx, monitor.ID)
			if err == nil {
				monitorWithStatus.UptimeStats = &templates.UptimeStats{
					Period24h: uptimeStats.Period24h,
					Period7d:  uptimeStats.Period7d,
					Period30d: uptimeStats.Period30d,
				}
				totalUptime += uptimeStats.Period24h
				uptimeCount++
			}
		}

		dashboardData.Monitors = append(dashboardData.Monitors, monitorWithStatus)
	}

	// Calculate average uptime
	if uptimeCount > 0 {
		dashboardData.Stats.AverageUptime = totalUptime / float64(uptimeCount)
	}

	// Convert to monitor list data
	listData := templates.MonitorListData{
		Monitors: dashboardData.Monitors,
		Stats:    dashboardData.Stats,
		User:     GetUserFromContext(c),
	}

	// Render monitor list template
	component := templates.MonitorList(listData)
	c.Header("Content-Type", "text/html; charset=utf-8")
	err = component.Render(ctx, c.Writer)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "", gin.H{
			"error": "Failed to render monitor list",
		})
		return
	}
}

// Requirements: 9.5
func (h *DashboardHandler) AlertHistory(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	monitorID := c.Param("id")
	if monitorID == "" {
		c.HTML(http.StatusBadRequest, "", gin.H{
			"error": "Monitor ID is required",
		})
		return
	}

	// Get monitor
	monitor, err := h.monitorService.GetMonitor(ctx, monitorID)
	if err != nil {
		c.HTML(http.StatusNotFound, "", gin.H{
			"error": "Monitor not found",
		})
		return
	}

	// Parse query parameters for filters
	filters := templates.AlertFilters{
		Type:      c.Query("type"),
		Severity:  c.Query("severity"),
		StartDate: c.Query("start_date"),
		EndDate:   c.Query("end_date"),
		Page:      1,
		Limit:     50,
	}

	// Parse page
	if pageStr := c.Query("page"); pageStr != "" {
		if page, err := strconv.Atoi(pageStr); err == nil && page > 0 {
			filters.Page = page
		}
	}

	// Parse limit
	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit <= 100 {
			filters.Limit = limit
		}
	}

	// Get alerts based on filters
	var alerts []*domain.Alert

	// Use date range query if both start and end are provided
	if filters.StartDate != "" && filters.EndDate != "" {
		start, err1 := time.Parse("2006-01-02", filters.StartDate)
		end, err2 := time.Parse("2006-01-02", filters.EndDate)
		if err1 == nil && err2 == nil {
			// Add time to end date to include the entire day
			end = end.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			alerts, err = h.alertRepo.GetByDateRange(ctx, monitorID, start, end)
		} else {
			// Fall back to simple query if date parsing fails
			alerts, err = h.alertRepo.GetByMonitorID(ctx, monitorID, filters.Limit)
		}
	} else {
		// Use simple limit-based query
		alerts, err = h.alertRepo.GetByMonitorID(ctx, monitorID, filters.Limit)
	}

	if err != nil {
		c.HTML(http.StatusInternalServerError, "", gin.H{
			"error": "Failed to load alerts",
		})
		return
	}

	// Apply client-side filters (type and severity)
	filteredAlerts := make([]domain.Alert, 0)
	for _, alert := range alerts {
		// Filter by type
		if filters.Type != "" && string(alert.Type) != filters.Type {
			continue
		}
		// Filter by severity
		if filters.Severity != "" && string(alert.Severity) != filters.Severity {
			continue
		}
		filteredAlerts = append(filteredAlerts, *alert)
	}

	// Build alert history data
	historyData := templates.AlertHistoryData{
		Monitor: monitor,
		Alerts:  filteredAlerts,
		Filters: filters,
		User:    GetUserFromContext(c),
	}

	// Render alert history template
	component := templates.AlertHistory(historyData)
	c.Header("Content-Type", "text/html; charset=utf-8")
	err = component.Render(ctx, c.Writer)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "", gin.H{
			"error": "Failed to render alert history",
		})
		return
	}
}

// DashboardAPI handles GET /api/v1/dashboard
// Returns dashboard data as JSON
func (h *DashboardHandler) DashboardAPI(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Get all monitors
	monitors, err := h.monitorService.ListMonitors(ctx, domain.ListFilters{
		Limit: 1000, // Get all monitors for dashboard
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to load monitors",
		})
		return
	}

	// Build dashboard data
	type MonitorWithStatus struct {
		Monitor      *domain.Monitor          `json:"monitor"`
		Status       domain.HealthCheckStatus `json:"status"`
		LastCheck    *time.Time               `json:"last_check,omitempty"`
		ResponseTime *time.Duration           `json:"response_time,omitempty"`
		UptimeStats  *struct {
			Period24h float64 `json:"period_24h"`
			Period7d  float64 `json:"period_7d"`
			Period30d float64 `json:"period_30d"`
		} `json:"uptime_stats,omitempty"`
	}

	type DashboardStats struct {
		TotalMonitors   int     `json:"total_monitors"`
		ActiveMonitors  int     `json:"active_monitors"`
		FailingMonitors int     `json:"failing_monitors"`
		AverageUptime   float64 `json:"average_uptime"`
	}

	dashboardData := struct {
		Monitors []MonitorWithStatus `json:"monitors"`
		Stats    DashboardStats      `json:"stats"`
	}{
		Monitors: make([]MonitorWithStatus, 0, len(monitors)),
		Stats: DashboardStats{
			TotalMonitors: len(monitors),
		},
	}

	var totalUptime float64
	var uptimeCount int

	// Get status and metrics for each monitor
	for _, monitor := range monitors {
		monitorWithStatus := MonitorWithStatus{
			Monitor: monitor,
			Status:  domain.HealthCheckStatus("unknown"),
		}

		// Get latest health check
		checks, err := h.healthCheckRepo.GetByMonitorID(ctx, monitor.ID, 1)
		if err == nil && len(checks) > 0 {
			latestCheck := checks[0]
			monitorWithStatus.Status = latestCheck.Status
			monitorWithStatus.LastCheck = &latestCheck.CheckedAt
			monitorWithStatus.ResponseTime = &latestCheck.ResponseTime

			// Count active/failing monitors
			if monitor.Enabled {
				dashboardData.Stats.ActiveMonitors++
				if latestCheck.Status != domain.StatusSuccess {
					dashboardData.Stats.FailingMonitors++
				}
			}
		} else if monitor.Enabled {
			dashboardData.Stats.ActiveMonitors++
		}

		// Get uptime stats
		if h.metricsService != nil {
			uptimeStats, err := h.metricsService.GetUptimePercentage(ctx, monitor.ID)
			if err == nil {
				monitorWithStatus.UptimeStats = &struct {
					Period24h float64 `json:"period_24h"`
					Period7d  float64 `json:"period_7d"`
					Period30d float64 `json:"period_30d"`
				}{
					Period24h: uptimeStats.Period24h,
					Period7d:  uptimeStats.Period7d,
					Period30d: uptimeStats.Period30d,
				}
				totalUptime += uptimeStats.Period24h
				uptimeCount++
			}
		}

		dashboardData.Monitors = append(dashboardData.Monitors, monitorWithStatus)
	}

	// Calculate average uptime
	if uptimeCount > 0 {
		dashboardData.Stats.AverageUptime = totalUptime / float64(uptimeCount)
	}

	c.JSON(http.StatusOK, dashboardData)
}
