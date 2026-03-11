package httpclient

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// RetryConfig holds configuration for retry logic
type RetryConfig struct {
	MaxAttempts  int
	InitialDelay time.Duration
}

// DefaultRetryConfig returns the default retry configuration
// Retry failed requests up to 3 times total with exponential backoff (1s, 2s, 4s)
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Second,
	}
}

// DoWithRetry executes an HTTP request with retry logic and exponential backoff
// It will retry up to MaxAttempts times (3 total attempts by default)
// with exponential backoff delays: 1s, 2s, 4s
func (c *Client) DoWithRetry(ctx context.Context, req *http.Request, config RetryConfig) (*http.Response, error) {
	var lastErr error
	var resp *http.Response

	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		// Clone the request for each attempt to avoid issues with consumed request bodies
		reqClone := req.Clone(ctx)

		resp, lastErr = c.Do(ctx, reqClone)

		// If successful (no network error), return immediately
		// This includes HTTP error status codes (4xx, 5xx) which are not retried
		if lastErr == nil {
			return resp, nil
		}

		// If this was the last attempt, don't wait
		if attempt == config.MaxAttempts-1 {
			break
		}

		// Calculate exponential backoff delay: initialDelay * 2^attempt
		// attempt 0: 1s, attempt 1: 2s, attempt 2: 4s
		delay := config.InitialDelay * time.Duration(1<<uint(attempt))

		// Wait before retrying, respecting context cancellation
		select {
		case <-ctx.Done():
			if lastErr != nil {
				return nil, lastErr
			}
			return nil, ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	// All attempts failed
	if lastErr != nil {
		return nil, lastErr
	}

	return nil, errors.New("all retry attempts failed")
}

// GetWithRetry performs a GET request with retry logic
func (c *Client) GetWithRetry(ctx context.Context, url string, config RetryConfig) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return c.DoWithRetry(ctx, req, config)
}
