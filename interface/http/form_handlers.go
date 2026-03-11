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

// FormHandler handles HTTP requests for form pages
type FormHandler struct {
	monitorService MonitorServiceInterface
}

// NewFormHandler creates a new form handler
func NewFormHandler(monitorService MonitorServiceInterface) *FormHandler {
	return &FormHandler{
		monitorService: monitorService,
	}
}

// NewMonitorForm handles GET /monitors/new
func (h *FormHandler) NewMonitorForm(c *gin.Context) {
	data := templates.MonitorFormData{
		Monitor: nil,
		IsEdit:  false,
		Errors:  make(map[string]string),
	}

	component := templates.MonitorForm(data)
	c.Header("Content-Type", "text/html; charset=utf-8")
	err := component.Render(c.Request.Context(), c.Writer)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "", gin.H{
			"error": "Failed to render form",
		})
		return
	}
}

// EditMonitorForm handles GET /monitors/:id/edit
func (h *FormHandler) EditMonitorForm(c *gin.Context) {
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

	data := templates.MonitorFormData{
		Monitor: monitor,
		IsEdit:  true,
		Errors:  make(map[string]string),
	}

	component := templates.MonitorForm(data)
	c.Header("Content-Type", "text/html; charset=utf-8")
	err = component.Render(ctx, c.Writer)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "", gin.H{
			"error": "Failed to render form",
		})
		return
	}
}

// CreateMonitorForm handles POST /monitors (form submission)
func (h *FormHandler) CreateMonitorForm(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Parse form data
	req, errors := h.parseMonitorForm(c)
	if len(errors) > 0 {
		// Return form with errors
		data := templates.MonitorFormData{
			Monitor: &domain.Monitor{
				Name:          req.Name,
				URL:           req.URL,
				CheckInterval: req.CheckInterval,
				Enabled:       req.Enabled,
				AlertChannels: req.AlertChannels,
			},
			IsEdit: false,
			Errors: errors,
		}

		component := templates.MonitorForm(data)
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.Status(http.StatusBadRequest)
		err := component.Render(ctx, c.Writer)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "", gin.H{
				"error": "Failed to render form",
			})
		}
		return
	}

	// Create monitor
	monitor, err := h.monitorService.CreateMonitor(ctx, req)
	if err != nil {
		// Return form with error
		data := templates.MonitorFormData{
			Monitor: &domain.Monitor{
				Name:          req.Name,
				URL:           req.URL,
				CheckInterval: req.CheckInterval,
				Enabled:       req.Enabled,
				AlertChannels: req.AlertChannels,
			},
			IsEdit: false,
			Errors: map[string]string{
				"general": err.Error(),
			},
		}

		component := templates.MonitorForm(data)
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.Status(http.StatusBadRequest)
		err = component.Render(ctx, c.Writer)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "", gin.H{
				"error": "Failed to render form",
			})
		}
		return
	}

	// Redirect to monitor detail page
	c.Redirect(http.StatusSeeOther, "/monitors/"+monitor.ID)
}

// UpdateMonitorForm handles POST /monitors/:id (form submission with _method=PUT)
func (h *FormHandler) UpdateMonitorForm(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	monitorID := c.Param("id")
	if monitorID == "" {
		c.HTML(http.StatusBadRequest, "", gin.H{
			"error": "Monitor ID is required",
		})
		return
	}

	// Get existing monitor
	existingMonitor, err := h.monitorService.GetMonitor(ctx, monitorID)
	if err != nil {
		c.HTML(http.StatusNotFound, "", gin.H{
			"error": "Monitor not found",
		})
		return
	}

	// Parse form data
	formReq, errors := h.parseMonitorForm(c)
	if len(errors) > 0 {
		// Return form with errors
		data := templates.MonitorFormData{
			Monitor: &domain.Monitor{
				ID:            existingMonitor.ID,
				Name:          formReq.Name,
				URL:           formReq.URL,
				CheckInterval: formReq.CheckInterval,
				Enabled:       formReq.Enabled,
				AlertChannels: formReq.AlertChannels,
				CreatedAt:     existingMonitor.CreatedAt,
				UpdatedAt:     existingMonitor.UpdatedAt,
			},
			IsEdit: true,
			Errors: errors,
		}

		component := templates.MonitorForm(data)
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.Status(http.StatusBadRequest)
		err := component.Render(ctx, c.Writer)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "", gin.H{
				"error": "Failed to render form",
			})
		}
		return
	}

	// Convert to update request
	updateReq := usecase.UpdateMonitorRequest{
		Name:          &formReq.Name,
		URL:           &formReq.URL,
		CheckInterval: &formReq.CheckInterval,
		Enabled:       &formReq.Enabled,
		AlertChannels: formReq.AlertChannels,
	}

	// Update monitor
	_, err = h.monitorService.UpdateMonitor(ctx, monitorID, updateReq)
	if err != nil {
		// Return form with error
		data := templates.MonitorFormData{
			Monitor: &domain.Monitor{
				ID:            existingMonitor.ID,
				Name:          formReq.Name,
				URL:           formReq.URL,
				CheckInterval: formReq.CheckInterval,
				Enabled:       formReq.Enabled,
				AlertChannels: formReq.AlertChannels,
				CreatedAt:     existingMonitor.CreatedAt,
				UpdatedAt:     existingMonitor.UpdatedAt,
			},
			IsEdit: true,
			Errors: map[string]string{
				"general": err.Error(),
			},
		}

		component := templates.MonitorForm(data)
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.Status(http.StatusBadRequest)
		err = component.Render(ctx, c.Writer)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "", gin.H{
				"error": "Failed to render form",
			})
		}
		return
	}

	// Redirect to monitor detail page
	c.Redirect(http.StatusSeeOther, "/monitors/"+monitorID)
}

// parseMonitorForm parses form data and returns a CreateMonitorRequest and validation errors
func (h *FormHandler) parseMonitorForm(c *gin.Context) (usecase.CreateMonitorRequest, map[string]string) {
	errors := make(map[string]string)
	req := usecase.CreateMonitorRequest{}

	// Parse basic fields
	req.Name = c.PostForm("name")
	if req.Name == "" {
		errors["name"] = "Monitor name is required"
	}

	req.URL = c.PostForm("url")
	if req.URL == "" {
		errors["url"] = "URL is required"
	}

	// Parse check interval
	checkIntervalStr := c.PostForm("check_interval")
	if checkIntervalStr == "" {
		errors["check_interval"] = "Check interval is required"
	} else {
		intervalMinutes, err := strconv.Atoi(checkIntervalStr)
		if err != nil || intervalMinutes <= 0 {
			errors["check_interval"] = "Invalid check interval"
		} else {
			req.CheckInterval = time.Duration(intervalMinutes) * time.Minute
		}
	}

	// Parse enabled checkbox
	req.Enabled = c.PostForm("enabled") == "on"

	// Parse alert channels
	req.AlertChannels = []domain.AlertChannel{}

	// Telegram
	if c.PostForm("telegram_enabled") == "on" {
		botToken := c.PostForm("telegram_bot_token")
		chatID := c.PostForm("telegram_chat_id")
		if botToken != "" && chatID != "" {
			req.AlertChannels = append(req.AlertChannels, domain.AlertChannel{
				Type: domain.AlertChannelTelegram,
				Config: map[string]string{
					"bot_token": botToken,
					"chat_id":   chatID,
				},
			})
		}
	}

	// Email
	if c.PostForm("email_enabled") == "on" {
		emailTo := c.PostForm("email_to")
		if emailTo != "" {
			req.AlertChannels = append(req.AlertChannels, domain.AlertChannel{
				Type: domain.AlertChannelEmail,
				Config: map[string]string{
					"to": emailTo,
				},
			})
		}
	}

	// Webhook
	if c.PostForm("webhook_enabled") == "on" {
		webhookURL := c.PostForm("webhook_url")
		if webhookURL != "" {
			req.AlertChannels = append(req.AlertChannels, domain.AlertChannel{
				Type: domain.AlertChannelWebhook,
				Config: map[string]string{
					"url": webhookURL,
				},
			})
		}
	}

	return req, errors
}
