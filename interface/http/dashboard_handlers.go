package http

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"web-tracker/domain"
	"web-tracker/interface/http/templates"
	"web-tracker/usecase"
)

// DashboardHandler handles HTTP requests for dashboard pages.
type DashboardHandler struct {
	monitorService  MonitorServiceInterface
	healthCheckRepo domain.HealthCheckRepository
	alertRepo       domain.AlertRepository
	metricsService  usecase.MetricsService
}

// NewDashboardHandler creates a new dashboard handler.
func NewDashboardHandler(
	monitorService MonitorServiceInterface,
	healthCheckRepo domain.HealthCheckRepository,
	alertRepo domain.AlertRepository,
	metricsService usecase.MetricsService,
) *DashboardHandler {
	fmt.Printf("NewDashboardHandler: metricsService = %v\n", metricsService)
	return &DashboardHandler{
		monitorService:  monitorService,
		healthCheckRepo: healthCheckRepo,
		alertRepo:       alertRepo,
		metricsService:  metricsService,
	}
}

// monitorSummary holds the aggregated result of fetching monitors with their status and metrics.
type monitorSummary struct {
	monitors      []templates.MonitorWithStatus
	stats         templates.DashboardStats
	averageUptime float64
}

// fetchMonitorsWithStatus concurrently fetches status and metrics for all monitors.
// It uses a semaphore to cap concurrency at 10 goroutines.
func (h *DashboardHandler) fetchMonitorsWithStatus(ctx context.Context, monitors []*domain.Monitor) monitorSummary {
	const concurrencyLimit = 10

	results := make([]templates.MonitorWithStatus, len(monitors))
	sem := make(chan struct{}, concurrencyLimit)

	var (
		wg          sync.WaitGroup
		mu          sync.Mutex
		stats       templates.DashboardStats
		totalUptime float64
		uptimeCount int
	)

	// Be defensive: monitors can contain nil entries (e.g. partial failures upstream).
	// Avoid panics and report totals based on actual non-nil monitors.
	for _, m := range monitors {
		if m != nil {
			stats.TotalMonitors++
		}
	}

	for i, m := range monitors {
		if m == nil {
			continue
		}
		wg.Add(1)
		go func(idx int, m *domain.Monitor) {
			defer wg.Done()
			if m == nil {
				return
			}
			sem <- struct{}{}
			defer func() { <-sem }()

			mws := templates.MonitorWithStatus{
				Monitor: m,
				Status:  domain.HealthCheckStatus("unknown"),
			}

			var (
				latestStatus domain.HealthCheckStatus
				hasStatus    bool
			)

			if m.ID != "" {
				if h.healthCheckRepo != nil {
					if checks, err := h.healthCheckRepo.GetByMonitorID(ctx, m.ID, 1); err == nil && len(checks) > 0 {
						chk := checks[0]
						mws.Status = chk.Status
						mws.LastCheck = &chk.CheckedAt
						mws.ResponseTime = &chk.ResponseTime
						latestStatus = chk.Status
						hasStatus = true
					}
				}

				if h.metricsService != nil {
					if us, err := h.metricsService.GetUptimePercentage(ctx, m.ID); err == nil && us != nil {
						mws.UptimeStats = &templates.UptimeStats{
							Period24h: us.Period24h,
							Period7d:  us.Period7d,
							Period30d: us.Period30d,
						}
						mu.Lock()
						totalUptime += us.Period24h
						uptimeCount++
						mu.Unlock()
					} else if err != nil && err != context.DeadlineExceeded {
						fmt.Printf("failed to get uptime stats for monitor %s: %v\n", m.ID, err)
					}
				} else {
					// metricsService is nil, skip uptime calculation
					fmt.Printf("metricsService is nil, skipping uptime calculation for monitor %s\n", m.ID)
				}
			}

			mu.Lock()
			defer mu.Unlock()
			if m.Enabled {
				stats.ActiveMonitors++
				if hasStatus && latestStatus != domain.StatusSuccess {
					stats.FailingMonitors++
				}
			}
			results[idx] = mws
		}(i, m)
	}

	wg.Wait()

	// Collect non-nil results preserving order.
	out := make([]templates.MonitorWithStatus, 0, len(monitors))
	for _, mws := range results {
		if mws.Monitor != nil {
			out = append(out, mws)
		}
	}

	if uptimeCount > 0 {
		stats.AverageUptime = totalUptime / float64(uptimeCount)
	}

	return monitorSummary{monitors: out, stats: stats}
}

// listAllMonitors fetches all monitors from the service.
func (h *DashboardHandler) listAllMonitors(ctx context.Context) ([]*domain.Monitor, error) {
	return h.monitorService.ListMonitors(ctx, domain.ListFilters{Limit: 1000})
}

// renderError writes a plain-text error response.
func renderError(c *gin.Context, code int, msg string) {
	c.String(code, msg)
}

// Dashboard handles GET /.
// Requirements: 9.5
func (h *DashboardHandler) Dashboard(c *gin.Context) {
	fmt.Printf("=== Dashboard handler called ===\n")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	monitors, err := h.listAllMonitors(ctx)
	if err != nil {
		fmt.Printf("Failed to load monitors: %v\n", err)
		renderError(c, http.StatusInternalServerError, "Failed to load monitors")
		return
	}
	fmt.Printf("Loaded %d monitors\n", len(monitors))

	summary := h.fetchMonitorsWithStatus(ctx, monitors)
	fmt.Printf("Summary stats: Total=%d, Active=%d, Failing=%d, AvgUptime=%.2f\n",
		summary.stats.TotalMonitors, summary.stats.ActiveMonitors, summary.stats.FailingMonitors, summary.stats.AverageUptime)

	data := templates.DashboardData{
		Monitors: summary.monitors,
		Stats:    summary.stats,
		User:     GetUserFromContext(c),
	}

	fmt.Printf("Dashboard data: Stats = %+v, Monitors count = %d\n", summary.stats, len(summary.monitors))

	c.Header("Content-Type", "text/html; charset=utf-8")

	// Debug: Check what we're about to render
	fmt.Printf("About to render dashboard with %d monitors and stats: %+v\n", len(data.Monitors), data.Stats)

	// Try to render to a buffer first to see what we get
	var buf bytes.Buffer
	if err := templates.Dashboard(data).Render(c.Request.Context(), &buf); err != nil {
		fmt.Printf("Failed to render dashboard to buffer: %v\n", err)
		renderError(c, http.StatusInternalServerError, "Failed to render dashboard")
		return
	}

	// Check if buffer has content
	if buf.Len() == 0 {
		fmt.Printf("Dashboard template rendered empty content!\n")
	} else {
		fmt.Printf("Dashboard template rendered %d bytes\n", buf.Len())
		// Check if it contains our stats
		content := buf.String()
		if strings.Contains(content, "Total Monitors") {
			fmt.Printf("Template contains 'Total Monitors'\n")
		} else {
			fmt.Printf("Template does NOT contain 'Total Monitors'\n")
		}
	}

	// Write to response
	if _, err := c.Writer.Write(buf.Bytes()); err != nil {
		fmt.Printf("Failed to write response: %v\n", err)
	}
	fmt.Printf("=== Dashboard handler completed ===\n")
}

// MonitorList handles GET /monitors.
func (h *DashboardHandler) MonitorList(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	monitors, err := h.listAllMonitors(ctx)
	if err != nil {
		renderError(c, http.StatusInternalServerError, "Failed to load monitors")
		return
	}

	summary := h.fetchMonitorsWithStatus(ctx, monitors)

	data := templates.MonitorListData{
		Monitors: summary.monitors,
		Stats:    summary.stats,
		User:     GetUserFromContext(c),
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := templates.MonitorList(data).Render(c.Request.Context(), c.Writer); err != nil {
		renderError(c, http.StatusInternalServerError, "Failed to render monitor list")
	}
}

// MonitorDetail handles GET /monitors/:id.
// Requirements: 9.5
func (h *DashboardHandler) MonitorDetail(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	monitorID := c.Param("id")
	if monitorID == "" {
		renderError(c, http.StatusBadRequest, "Monitor ID is required")
		return
	}

	monitor, err := h.monitorService.GetMonitor(ctx, monitorID)
	if err != nil {
		renderError(c, http.StatusNotFound, "Monitor not found")
		return
	}

	data := templates.MonitorDetailData{
		Monitor: monitor,
		User:    GetUserFromContext(c),
	}

	if checks, err := h.healthCheckRepo.GetByMonitorID(ctx, monitorID, 1); err == nil && len(checks) > 0 {
		data.LatestCheck = checks[0]
	}

	if h.metricsService != nil {
		if us, err := h.metricsService.GetUptimePercentage(ctx, monitorID); err == nil {
			data.UptimeStats = &templates.UptimeStats{
				Period24h: us.Period24h,
				Period7d:  us.Period7d,
				Period30d: us.Period30d,
			}
		}
		if rs, err := h.metricsService.GetResponseTimeStats(ctx, monitorID, 24*time.Hour); err == nil {
			data.ResponseStats = &templates.ResponseTimeStats{
				Period:  "24h",
				Average: rs.Average,
				Min:     rs.Min,
				Max:     rs.Max,
				P95:     rs.P95,
				P99:     rs.P99,
			}
		}
	}

	if checks, err := h.healthCheckRepo.GetByMonitorID(ctx, monitorID, 10); err == nil {
		data.RecentChecks = make([]domain.HealthCheck, len(checks))
		for i, chk := range checks {
			data.RecentChecks[i] = *chk
		}
	}

	if alerts, err := h.alertRepo.GetByMonitorID(ctx, monitorID, 10); err == nil {
		data.RecentAlerts = make([]domain.Alert, len(alerts))
		for i, a := range alerts {
			data.RecentAlerts[i] = *a
		}
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := templates.MonitorDetail(data).Render(c.Request.Context(), c.Writer); err != nil {
		renderError(c, http.StatusInternalServerError, "Failed to render monitor detail")
	}
}

// AlertHistory handles GET /monitors/:id/alerts.
// Requirements: 9.5
func (h *DashboardHandler) AlertHistory(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	monitorID := c.Param("id")
	if monitorID == "" {
		renderError(c, http.StatusBadRequest, "Monitor ID is required")
		return
	}

	monitor, err := h.monitorService.GetMonitor(ctx, monitorID)
	if err != nil {
		renderError(c, http.StatusNotFound, "Monitor not found")
		return
	}

	filters := parseAlertFilters(c)
	rawAlerts, err := h.fetchAlerts(ctx, monitorID, filters)
	if err != nil {
		renderError(c, http.StatusInternalServerError, "Failed to load alerts")
		return
	}

	filtered := applyAlertFilters(rawAlerts, filters)

	data := templates.AlertHistoryData{
		Monitor: monitor,
		Alerts:  filtered,
		Filters: filters,
		User:    GetUserFromContext(c),
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := templates.AlertHistory(data).Render(c.Request.Context(), c.Writer); err != nil {
		renderError(c, http.StatusInternalServerError, "Failed to render alert history")
	}
}

// parseAlertFilters reads query parameters from the request into an AlertFilters struct.
func parseAlertFilters(c *gin.Context) templates.AlertFilters {
	f := templates.AlertFilters{
		Type:      c.Query("type"),
		Severity:  c.Query("severity"),
		StartDate: c.Query("start_date"),
		EndDate:   c.Query("end_date"),
		Page:      1,
		Limit:     50,
	}
	if page, err := strconv.Atoi(c.Query("page")); err == nil && page > 0 {
		f.Page = page
	}
	if limit, err := strconv.Atoi(c.Query("limit")); err == nil && limit > 0 && limit <= 100 {
		f.Limit = limit
	}
	return f
}

// fetchAlerts retrieves alerts for a monitor, using a date range query when both dates are set.
func (h *DashboardHandler) fetchAlerts(ctx context.Context, monitorID string, f templates.AlertFilters) ([]*domain.Alert, error) {
	if f.StartDate != "" && f.EndDate != "" {
		start, err1 := time.Parse("2006-01-02", f.StartDate)
		end, err2 := time.Parse("2006-01-02", f.EndDate)
		if err1 == nil && err2 == nil {
			end = end.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			return h.alertRepo.GetByDateRange(ctx, monitorID, start, end)
		}
	}
	return h.alertRepo.GetByMonitorID(ctx, monitorID, f.Limit)
}

// applyAlertFilters filters a slice of alerts by type and severity.
func applyAlertFilters(alerts []*domain.Alert, f templates.AlertFilters) []domain.Alert {
	out := make([]domain.Alert, 0, len(alerts))
	for _, a := range alerts {
		if f.Type != "" && string(a.Type) != f.Type {
			continue
		}
		if f.Severity != "" && string(a.Severity) != f.Severity {
			continue
		}
		out = append(out, *a)
	}
	return out
}

// DashboardAPI handles GET /api/v1/dashboard and returns dashboard data as JSON.
func (h *DashboardHandler) DashboardAPI(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	monitors, err := h.listAllMonitors(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load monitors"})
		return
	}

	// Re-use the same concurrent fetch as the HTML handlers.
	summary := h.fetchMonitorsWithStatus(ctx, monitors)

	// Map to the JSON response shape, keeping the API contract stable.
	type uptimeStats struct {
		Period24h float64 `json:"period_24h"`
		Period7d  float64 `json:"period_7d"`
		Period30d float64 `json:"period_30d"`
	}
	type monitorEntry struct {
		Monitor      *domain.Monitor          `json:"monitor"`
		Status       domain.HealthCheckStatus `json:"status"`
		LastCheck    *time.Time               `json:"last_check,omitempty"`
		ResponseTime *time.Duration           `json:"response_time,omitempty"`
		UptimeStats  *uptimeStats             `json:"uptime_stats,omitempty"`
	}
	type statsEntry struct {
		TotalMonitors   int     `json:"total_monitors"`
		ActiveMonitors  int     `json:"active_monitors"`
		FailingMonitors int     `json:"failing_monitors"`
		AverageUptime   float64 `json:"average_uptime"`
	}

	entries := make([]monitorEntry, 0, len(summary.monitors))
	for _, mws := range summary.monitors {
		e := monitorEntry{
			Monitor:      mws.Monitor,
			Status:       mws.Status,
			LastCheck:    mws.LastCheck,
			ResponseTime: mws.ResponseTime,
		}
		if mws.UptimeStats != nil {
			e.UptimeStats = &uptimeStats{
				Period24h: mws.UptimeStats.Period24h,
				Period7d:  mws.UptimeStats.Period7d,
				Period30d: mws.UptimeStats.Period30d,
			}
		}
		entries = append(entries, e)
	}

	c.JSON(http.StatusOK, struct {
		Monitors []monitorEntry `json:"monitors"`
		Stats    statsEntry     `json:"stats"`
	}{
		Monitors: entries,
		Stats: statsEntry{
			TotalMonitors:   summary.stats.TotalMonitors,
			ActiveMonitors:  summary.stats.ActiveMonitors,
			FailingMonitors: summary.stats.FailingMonitors,
			AverageUptime:   summary.stats.AverageUptime,
		},
	})
}
