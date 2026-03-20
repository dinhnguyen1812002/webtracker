package usecase

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"web-tracker/domain"
	"web-tracker/infrastructure/httpclient"
)

// HealthCheckService handles health check execution logic
type HealthCheckService struct {
	httpClient           *httpclient.Client
	healthCheckRepo      domain.HealthCheckRepository
	monitorRepo          domain.MonitorRepository
	redisClient          RedisClient
	retryConfig          httpclient.RetryConfig
	requestTimeout       time.Duration
	alertService         HealthCheckAlertService
	performanceThreshold time.Duration // Default threshold for performance alerts
	websocketBroadcaster WebSocketBroadcaster
}

// HealthCheckAlertService defines the interface for alert operations needed by HealthCheckService
type HealthCheckAlertService interface {
	GeneratePerformanceAlert(ctx context.Context, monitor *domain.Monitor, responseTime, threshold time.Duration) error
}

// RedisClient defines the interface for Redis operations needed by HealthCheckService
type RedisClient interface {
	Delete(ctx context.Context, key string) error
}

// NewHealthCheckService creates a new health check service
func NewHealthCheckService(
	httpClient *httpclient.Client,
	healthCheckRepo domain.HealthCheckRepository,
	monitorRepo domain.MonitorRepository,
	redisClient RedisClient,
	alertService HealthCheckAlertService,
	websocketBroadcaster WebSocketBroadcaster,
) *HealthCheckService {
	return &HealthCheckService{
		httpClient:           httpClient,
		healthCheckRepo:      healthCheckRepo,
		monitorRepo:          monitorRepo,
		redisClient:          redisClient,
		retryConfig:          httpclient.DefaultRetryConfig(),
		requestTimeout:       30 * time.Second,
		alertService:         alertService,
		performanceThreshold: 5 * time.Second, // Default 5-second threshold
		websocketBroadcaster: websocketBroadcaster,
	}
}

// SetPerformanceThreshold sets the performance alert threshold
func (s *HealthCheckService) SetPerformanceThreshold(threshold time.Duration) {
	s.performanceThreshold = threshold
}

// SetRetryConfig overrides the retry behavior used for outgoing health check requests.
func (s *HealthCheckService) SetRetryConfig(config httpclient.RetryConfig) {
	s.retryConfig = config
}

// SetRequestTimeout overrides the per-request timeout used for outgoing health check requests.
func (s *HealthCheckService) SetRequestTimeout(timeout time.Duration) {
	s.requestTimeout = timeout
}

// ExecuteCheck performs a health check for the specified monitor
// Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 14.1
func (s *HealthCheckService) ExecuteCheck(ctx context.Context, monitorID string) (*domain.HealthCheck, error) {
	// Get monitor configuration
	monitor, err := s.monitorRepo.GetByID(ctx, monitorID)
	if err != nil {
		return nil, fmt.Errorf("failed to get monitor: %w", err)
	}

	// Create health check record
	healthCheck := &domain.HealthCheck{
		ID:        uuid.New().String(),
		MonitorID: monitorID,
		CheckedAt: time.Now(),
	}

	// Execute HTTP request with timeout and retry
	startTime := time.Now()
	resp, err := s.executeHTTPRequest(ctx, monitor.URL)
	healthCheck.ResponseTime = time.Since(startTime)

	// Handle request errors
	if err != nil {
		healthCheck.Status = s.classifyError(err)
		healthCheck.ErrorMessage = err.Error()

		// Persist health check result to database (Requirement 14.1)
		if persistErr := s.persistHealthCheck(ctx, healthCheck); persistErr != nil {
			return healthCheck, fmt.Errorf("failed to persist health check: %w", persistErr)
		}

		return healthCheck, nil
	}
	defer resp.Body.Close()

	// Classify status code (Requirement 1.2, 1.3)
	healthCheck.StatusCode = resp.StatusCode
	healthCheck.Status = s.classifyStatusCode(resp.StatusCode)

	// Extract SSL certificate info for HTTPS (Requirement 2.1, 2.6)
	if resp.TLS != nil {
		healthCheck.SSLInfo = s.extractSSLInfo(resp.TLS)

		// Requirement 2.6: Invalid or unverifiable SSL certificate = failed health check
		if healthCheck.SSLInfo != nil && !healthCheck.SSLInfo.Valid {
			healthCheck.Status = domain.StatusFailure
			if healthCheck.ErrorMessage == "" {
				healthCheck.ErrorMessage = "SSL certificate is invalid or cannot be verified"
			}
		}
	}

	// Persist health check result to database (Requirement 14.1)
	if err := s.persistHealthCheck(ctx, healthCheck); err != nil {
		return healthCheck, fmt.Errorf("failed to persist health check: %w", err)
	}

	// Check for performance alerts if the health check was successful (Requirement 8.4)
	if healthCheck.IsSuccessful() && s.alertService != nil {
		if err := s.checkPerformanceThreshold(ctx, monitor, healthCheck); err != nil {
			// Log error but don't fail the health check
			// Performance alerting is not critical to the health check itself
			_ = err
		}
	}

	return healthCheck, nil
}

// executeHTTPRequest performs the HTTP request with retry logic
// Requirement 1.1: Send HTTP/HTTPS request
// Requirement 1.5: 30-second timeout
// Requirement 1.6: Retry up to 2 additional times with exponential backoff
func (s *HealthCheckService) executeHTTPRequest(ctx context.Context, url string) (*http.Response, error) {
	// Create context with 30-second timeout (Requirement 1.5)
	ctx, cancel := context.WithTimeout(ctx, s.requestTimeout)
	defer cancel()

	// Execute request with retry logic (Requirement 1.6)
	resp, err := s.httpClient.GetWithRetry(ctx, url, s.retryConfig)
	return resp, err
}

// classifyStatusCode classifies HTTP status codes
// Requirement 1.2: Status codes 200-299 = success
// Requirement 1.3: Other status codes = failure
func (s *HealthCheckService) classifyStatusCode(statusCode int) domain.HealthCheckStatus {
	if statusCode >= 200 && statusCode <= 299 {
		return domain.StatusSuccess
	}
	return domain.StatusFailure
}

// classifyError determines the health check status based on the error type
// Requirement 1.5: Timeout errors
// Requirement 1.3: Network errors = failure
func (s *HealthCheckService) classifyError(err error) domain.HealthCheckStatus {
	// Check if it's a timeout error
	if ctx, ok := err.(interface{ Timeout() bool }); ok && ctx.Timeout() {
		return domain.StatusTimeout
	}

	// Check for context deadline exceeded
	if err == context.DeadlineExceeded {
		return domain.StatusTimeout
	}

	// All other errors are classified as failure
	return domain.StatusFailure
}

// extractSSLInfo extracts SSL certificate information from TLS connection state
// Requirements: 2.1, 2.6
// Extracts SSL certificate, calculates days until expiration, and validates certificate chain
func (s *HealthCheckService) extractSSLInfo(connState *tls.ConnectionState) *domain.SSLInfo {
	// Check if TLS connection state is available
	if connState == nil || len(connState.PeerCertificates) == 0 {
		return &domain.SSLInfo{
			Valid:     false,
			ExpiresAt: time.Time{},
			DaysUntil: 0,
			Issuer:    "",
		}
	}

	// Get the leaf certificate (first in the chain)
	cert := connState.PeerCertificates[0]

	// Validate certificate chain (Requirement 2.6)
	valid := s.validateCertificateChain(connState)

	// Calculate days until expiration (Requirement 2.1)
	daysUntil := domain.CalculateSSLDaysUntilExpiry(cert.NotAfter)

	// Extract issuer information
	issuer := cert.Issuer.CommonName
	if issuer == "" && len(cert.Issuer.Organization) > 0 {
		issuer = cert.Issuer.Organization[0]
	}

	return &domain.SSLInfo{
		Valid:     valid,
		ExpiresAt: cert.NotAfter,
		DaysUntil: daysUntil,
		Issuer:    issuer,
	}
}

// validateCertificateChain validates the SSL certificate chain
// Requirement 2.6: Validate certificate chain
func (s *HealthCheckService) validateCertificateChain(connState *tls.ConnectionState) bool {
	// Check if connection state is valid
	if connState == nil || len(connState.PeerCertificates) == 0 {
		return false
	}

	cert := connState.PeerCertificates[0]
	now := time.Now()

	// Check if certificate is expired or not yet valid
	if now.Before(cert.NotBefore) || now.After(cert.NotAfter) {
		return false
	}

	// Check if the TLS handshake was successful
	// If HandshakeComplete is true, it means the certificate chain was validated during handshake
	if !connState.HandshakeComplete {
		return false
	}

	// Check for verification errors
	// If we got here with a completed handshake, the certificate chain was validated by the TLS library
	// Additional validation: check if certificate has required fields
	if cert.Subject.CommonName == "" && len(cert.DNSNames) == 0 {
		return false
	}

	return true
}

// GetCheckHistory retrieves health check history for a monitor
func (s *HealthCheckService) GetCheckHistory(ctx context.Context, monitorID string, limit int) ([]*domain.HealthCheck, error) {
	return s.healthCheckRepo.GetByMonitorID(ctx, monitorID, limit)
}

// persistHealthCheck saves the health check to the database and invalidates metrics cache
// Requirement 14.1: Persist health check results
// Requirement 9.2: Broadcast health check updates within 2 seconds
func (s *HealthCheckService) persistHealthCheck(ctx context.Context, healthCheck *domain.HealthCheck) error {
	// Save to database
	if err := s.healthCheckRepo.Create(ctx, healthCheck); err != nil {
		return fmt.Errorf("failed to save health check to database: %w", err)
	}

	// Invalidate metrics cache in Redis to ensure fresh calculations
	// This follows the design pattern where metrics are calculated on-demand and cached
	// Cache keys follow the pattern: cache:metrics:{monitorID}:*
	s.invalidateMetricsCache(ctx, healthCheck.MonitorID)

	// Broadcast health check update to WebSocket clients (Requirement 9.2)
	if s.websocketBroadcaster != nil {
		s.websocketBroadcaster.BroadcastHealthCheckUpdate(healthCheck)
	}

	return nil
}

// invalidateMetricsCache removes cached metrics for a monitor
// This ensures that uptime percentages and response time stats are recalculated
// with the latest health check data
func (s *HealthCheckService) invalidateMetricsCache(ctx context.Context, monitorID string) {
	// Invalidate uptime percentage cache for all time periods
	cacheKeys := []string{
		fmt.Sprintf("cache:metrics:%s:uptime:24h", monitorID),
		fmt.Sprintf("cache:metrics:%s:uptime:7d", monitorID),
		fmt.Sprintf("cache:metrics:%s:uptime:30d", monitorID),
		fmt.Sprintf("cache:metrics:%s:response:1h", monitorID),
		fmt.Sprintf("cache:metrics:%s:response:24h", monitorID),
		fmt.Sprintf("cache:metrics:%s:response:7d", monitorID),
	}

	for _, key := range cacheKeys {
		// Best effort deletion - log errors but don't fail the health check
		if err := s.redisClient.Delete(ctx, key); err != nil {
			// In production, this would be logged
			// For now, we silently continue as cache invalidation is not critical
			_ = err
		}
	}
}

// checkPerformanceThreshold checks if response time exceeds threshold and generates alert
// Requirement 8.4: Generate alerts when response time exceeds threshold
func (s *HealthCheckService) checkPerformanceThreshold(ctx context.Context, monitor *domain.Monitor, healthCheck *domain.HealthCheck) error {
	if healthCheck.ResponseTime <= s.performanceThreshold {
		return nil // No alert needed
	}

	// Generate performance alert
	return s.alertService.GeneratePerformanceAlert(ctx, monitor, healthCheck.ResponseTime, s.performanceThreshold)
}
