package tests

import (
	"context"
	"errors"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"web-tracker/infrastructure/httpclient"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newTestClient(fn roundTripFunc) *httpclient.Client {
	return httpclient.NewClientFromHTTPClient(&http.Client{
		Transport: fn,
	})
}

func newResponse(statusCode int) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(http.NoBody),
		Header:     make(http.Header),
	}
}

func TestDoWithRetry_Success(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		return newResponse(http.StatusOK), nil
	})
	ctx := context.Background()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.com", nil)
	require.NoError(t, err)

	config := httpclient.DefaultRetryConfig()
	resp, err := client.DoWithRetry(ctx, req, config)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

func TestDoWithRetry_HTTPErrorNotRetried(t *testing.T) {
	var attemptCount atomic.Int32

	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		attemptCount.Add(1)
		return newResponse(http.StatusInternalServerError), nil
	})
	ctx := context.Background()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.com", nil)
	require.NoError(t, err)

	config := httpclient.DefaultRetryConfig()
	resp, err := client.DoWithRetry(ctx, req, config)

	// Should succeed (no network error), but with error status code
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	// Should only attempt once - HTTP errors are not retried
	assert.Equal(t, int32(1), attemptCount.Load())
	resp.Body.Close()
}

func TestDoWithRetry_NetworkError(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("simulated network error")
	})
	ctx := context.Background()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.com", nil)
	require.NoError(t, err)

	config := httpclient.DefaultRetryConfig()
	startTime := time.Now()
	_, err = client.DoWithRetry(ctx, req, config)
	elapsed := time.Since(startTime)

	// Should fail after all retries
	require.Error(t, err)

	// Should have taken at least 1s + 2s = 3s for retries (with some tolerance)
	assert.GreaterOrEqual(t, elapsed, 3*time.Second)
}

func TestDoWithRetry_ExponentialBackoff(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("simulated network error")
	})
	ctx := context.Background()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.com", nil)
	require.NoError(t, err)

	config := httpclient.DefaultRetryConfig()
	config.MaxAttempts = 3
	config.InitialDelay = 1 * time.Second

	startTime := time.Now()
	_, err = client.DoWithRetry(ctx, req, config)
	elapsed := time.Since(startTime)

	require.Error(t, err)

	// Total delay should be approximately 1s + 2s = 3s (between attempt 1 and 2, and 2 and 3)
	// Allow some tolerance for execution time
	assert.GreaterOrEqual(t, elapsed, 3*time.Second)
	assert.Less(t, elapsed, 5*time.Second)
}

func TestDoWithRetry_ContextCancellation(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("simulated network error")
	})
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.com", nil)
	require.NoError(t, err)

	config := httpclient.DefaultRetryConfig()
	config.MaxAttempts = 5
	config.InitialDelay = 1 * time.Second

	_, err = client.DoWithRetry(ctx, req, config)

	// Should fail due to context cancellation
	require.Error(t, err)
}

func TestGetWithRetry_Success(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, http.MethodGet, r.Method)
		return newResponse(http.StatusOK), nil
	})
	ctx := context.Background()

	config := httpclient.DefaultRetryConfig()
	resp, err := client.GetWithRetry(ctx, "http://example.com", config)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

func TestGetWithRetry_InvalidURL(t *testing.T) {
	client := httpclient.NewClient(httpclient.DefaultConfig())
	ctx := context.Background()

	config := httpclient.DefaultRetryConfig()
	_, err := client.GetWithRetry(ctx, "://invalid-url", config)

	require.Error(t, err)
}

func TestDefaultRetryConfig(t *testing.T) {
	config := httpclient.DefaultRetryConfig()

	assert.Equal(t, 3, config.MaxAttempts)
	assert.Equal(t, 1*time.Second, config.InitialDelay)
}

func TestDoWithRetry_MaxAttemptsRespected(t *testing.T) {
	var attemptCount atomic.Int32
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		attemptCount.Add(1)
		return nil, errors.New("simulated network error")
	})
	ctx := context.Background()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.com", nil)
	require.NoError(t, err)

	config := httpclient.RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
	}

	startTime := time.Now()
	_, err = client.DoWithRetry(ctx, req, config)
	elapsed := time.Since(startTime)

	require.Error(t, err)

	assert.Equal(t, int32(3), attemptCount.Load())
	assert.GreaterOrEqual(t, elapsed, 300*time.Millisecond)
	assert.Less(t, elapsed, 1*time.Second)
}

func TestDoWithRetry_ImmediateSuccess(t *testing.T) {
	var attemptCount atomic.Int32

	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		attemptCount.Add(1)
		return newResponse(http.StatusOK), nil
	})
	ctx := context.Background()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.com", nil)
	require.NoError(t, err)

	config := httpclient.DefaultRetryConfig()
	resp, err := client.DoWithRetry(ctx, req, config)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Should only attempt once on immediate success
	assert.Equal(t, int32(1), attemptCount.Load())
	resp.Body.Close()
}

func TestDoWithRetry_NetworkErrorThenSuccess(t *testing.T) {
	var attemptCount atomic.Int32

	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		count := attemptCount.Add(1)
		if count == 1 {
			return nil, errors.New("simulated network error")
		}
		return newResponse(http.StatusOK), nil
	})
	ctx := context.Background()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.com", nil)
	require.NoError(t, err)

	config := httpclient.DefaultRetryConfig()
	resp, err := client.DoWithRetry(ctx, req, config)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	// Should have retried at least once
	assert.GreaterOrEqual(t, attemptCount.Load(), int32(2))
	resp.Body.Close()
}

func TestDoWithRetry_CustomBackoffTiming(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("simulated network error")
	})
	ctx := context.Background()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.com", nil)
	require.NoError(t, err)

	config := httpclient.RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 500 * time.Millisecond,
	}

	startTime := time.Now()
	_, err = client.DoWithRetry(ctx, req, config)
	elapsed := time.Since(startTime)

	require.Error(t, err)

	// Total delay should be approximately 500ms + 1s = 1.5s
	assert.GreaterOrEqual(t, elapsed, 1500*time.Millisecond)
	assert.Less(t, elapsed, 3*time.Second)
}
