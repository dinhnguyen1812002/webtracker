package alertchannel

import (
	"context"
	"fmt"
	"log"
	"sync"

	"web-tracker/domain"
)

// AlertChannelAdapter defines the interface for alert channel adapters
type AlertChannelAdapter interface {
	Send(ctx context.Context, alert *domain.Alert, config map[string]string) error
}

// DeliveryService handles multi-channel alert delivery
type DeliveryService struct {
	adapters map[domain.AlertChannelType]AlertChannelAdapter
}

// NewDeliveryService creates a new delivery service with all adapters
func NewDeliveryService() *DeliveryService {
	return &DeliveryService{
		adapters: map[domain.AlertChannelType]AlertChannelAdapter{
			domain.AlertChannelTelegram: NewTelegramAdapter(),
			domain.AlertChannelEmail:    NewEmailAdapter(),
			domain.AlertChannelWebhook:  NewWebhookAdapter(),
		},
	}
}

// DeliveryResult represents the result of delivering an alert to a channel
type DeliveryResult struct {
	Channel domain.AlertChannelType
	Success bool
	Error   error
}

// DeliverAlert delivers an alert to all configured channels concurrently
// Requirements: 4.1, 4.6 - Deliver to all channels, isolate failures
func (d *DeliveryService) DeliverAlert(ctx context.Context, alert *domain.Alert, channels []domain.AlertChannel) []DeliveryResult {
	if len(channels) == 0 {
		return []DeliveryResult{}
	}

	// Create a channel to collect results
	resultChan := make(chan DeliveryResult, len(channels))
	var wg sync.WaitGroup

	// Deliver to each channel concurrently (Requirement 4.1)
	for _, channel := range channels {
		wg.Add(1)
		go func(ch domain.AlertChannel) {
			defer wg.Done()

			adapter, exists := d.adapters[ch.Type]
			if !exists {
				resultChan <- DeliveryResult{
					Channel: ch.Type,
					Success: false,
					Error:   fmt.Errorf("no adapter found for channel type: %s", ch.Type),
				}
				return
			}

			// Send alert via adapter
			err := adapter.Send(ctx, alert, ch.Config)
			result := DeliveryResult{
				Channel: ch.Type,
				Success: err == nil,
				Error:   err,
			}

			// Log delivery failures after max retries (Requirement 4.6)
			if err != nil {
				log.Printf("Failed to deliver alert %s to %s channel: %v", alert.ID, ch.Type, err)
			}

			resultChan <- result
		}(channel)
	}

	// Wait for all deliveries to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	var results []DeliveryResult
	for result := range resultChan {
		results = append(results, result)
	}

	return results
}

// GetSupportedChannels returns the list of supported channel types
func (d *DeliveryService) GetSupportedChannels() []domain.AlertChannelType {
	channels := make([]domain.AlertChannelType, 0, len(d.adapters))
	for channelType := range d.adapters {
		channels = append(channels, channelType)
	}
	return channels
}

// RegisterAdapter allows registering custom adapters for testing or extensions
func (d *DeliveryService) RegisterAdapter(channelType domain.AlertChannelType, adapter AlertChannelAdapter) {
	d.adapters[channelType] = adapter
}
