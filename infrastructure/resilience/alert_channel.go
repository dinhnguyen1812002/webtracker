package resilience

import (
	"context"
	"time"

	"web-tracker/domain"
	"web-tracker/infrastructure/logger"
	"web-tracker/interface/alertchannel"
)

// ResilientAlertChannelAdapter wraps an alert channel adapter with circuit breaker
type ResilientAlertChannelAdapter struct {
	adapter        alertchannel.AlertChannelAdapter
	circuitBreaker *CircuitBreaker
	channelType    domain.AlertChannelType
	logger         *logger.Logger
}

// NewResilientAlertChannelAdapter creates a new resilient alert channel adapter
func NewResilientAlertChannelAdapter(
	adapter alertchannel.AlertChannelAdapter,
	channelType domain.AlertChannelType,
	cbManager *CircuitBreakerManager,
) *ResilientAlertChannelAdapter {
	// Circuit breaker configuration as per requirements:
	// - Open after 5 consecutive failures
	// - Half-open after 60 seconds
	// - Close after 3 consecutive successes
	cb := cbManager.GetOrCreate(
		string(channelType),
		5,              // maxFailures
		60*time.Second, // timeout
		3,              // successThreshold
	)

	return &ResilientAlertChannelAdapter{
		adapter:        adapter,
		circuitBreaker: cb,
		channelType:    channelType,
		logger:         logger.GetLogger(),
	}
}

// Send sends an alert through the adapter with circuit breaker protection
func (r *ResilientAlertChannelAdapter) Send(ctx context.Context, alert *domain.Alert, config map[string]string) error {
	return r.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
		err := r.adapter.Send(ctx, alert, config)
		if err != nil {
			r.logger.Warn("Alert channel delivery failed", logger.Fields{
				"channel":    string(r.channelType),
				"alert_id":   alert.ID,
				"alert_type": string(alert.Type),
				"error":      err.Error(),
			})
		} else {
			r.logger.Debug("Alert channel delivery succeeded", logger.Fields{
				"channel":    string(r.channelType),
				"alert_id":   alert.ID,
				"alert_type": string(alert.Type),
			})
		}
		return err
	})
}

// ResilientDeliveryService wraps the delivery service with circuit breakers
type ResilientDeliveryService struct {
	adapters  map[domain.AlertChannelType]*ResilientAlertChannelAdapter
	cbManager *CircuitBreakerManager
	logger    *logger.Logger
}

// NewResilientDeliveryService creates a new resilient delivery service
func NewResilientDeliveryService(cbManager *CircuitBreakerManager) *ResilientDeliveryService {
	service := &ResilientDeliveryService{
		adapters:  make(map[domain.AlertChannelType]*ResilientAlertChannelAdapter),
		cbManager: cbManager,
		logger:    logger.GetLogger(),
	}

	// Create resilient adapters for each channel type
	service.adapters[domain.AlertChannelTelegram] = NewResilientAlertChannelAdapter(
		alertchannel.NewTelegramAdapter(),
		domain.AlertChannelTelegram,
		cbManager,
	)

	service.adapters[domain.AlertChannelEmail] = NewResilientAlertChannelAdapter(
		alertchannel.NewEmailAdapter(),
		domain.AlertChannelEmail,
		cbManager,
	)

	service.adapters[domain.AlertChannelWebhook] = NewResilientAlertChannelAdapter(
		alertchannel.NewWebhookAdapter(),
		domain.AlertChannelWebhook,
		cbManager,
	)

	return service
}

// DeliverAlert delivers an alert to all configured channels with circuit breaker protection
func (r *ResilientDeliveryService) DeliverAlert(ctx context.Context, alert *domain.Alert, channels []domain.AlertChannel) []alertchannel.DeliveryResult {
	if len(channels) == 0 {
		return []alertchannel.DeliveryResult{}
	}

	results := make([]alertchannel.DeliveryResult, 0, len(channels))

	for _, channel := range channels {
		adapter, exists := r.adapters[channel.Type]
		if !exists {
			results = append(results, alertchannel.DeliveryResult{
				Channel: channel.Type,
				Success: false,
				Error:   ErrCircuitOpen, // Treat missing adapter as circuit open
			})
			continue
		}

		err := adapter.Send(ctx, alert, channel.Config)

		// Handle circuit breaker open state
		if err == ErrCircuitOpen {
			r.logger.Warn("Alert delivery skipped due to circuit breaker", logger.Fields{
				"channel":    string(channel.Type),
				"alert_id":   alert.ID,
				"alert_type": string(alert.Type),
			})
		}

		results = append(results, alertchannel.DeliveryResult{
			Channel: channel.Type,
			Success: err == nil,
			Error:   err,
		})
	}

	return results
}

// GetSupportedChannels returns the list of supported channel types
func (r *ResilientDeliveryService) GetSupportedChannels() []domain.AlertChannelType {
	channels := make([]domain.AlertChannelType, 0, len(r.adapters))
	for channelType := range r.adapters {
		channels = append(channels, channelType)
	}
	return channels
}

// GetCircuitBreakerStats returns statistics for all circuit breakers
func (r *ResilientDeliveryService) GetCircuitBreakerStats() []CircuitBreakerStats {
	return r.cbManager.GetStats()
}

// ResetCircuitBreakers resets all circuit breakers to closed state
func (r *ResilientDeliveryService) ResetCircuitBreakers() {
	r.cbManager.Reset()
}
