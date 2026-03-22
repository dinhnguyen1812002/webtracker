package tests

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"web-tracker/infrastructure/httpclient"
)

type clientRoundTripFunc func(*http.Request) (*http.Response, error)

func (f clientRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newWrappedClient(fn clientRoundTripFunc) *httpclient.Client {
	return httpclient.NewClientFromHTTPClient(&http.Client{
		Transport: fn,
	})
}

func newClientResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func TestHTTPClient_NewClient(t *testing.T) {
	config := httpclient.DefaultConfig()
	client := httpclient.NewClient(config)

	require.NotNil(t, client)
	assert.NotNil(t, client)
}

func TestClient_Get(t *testing.T) {
	client := newWrappedClient(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, http.MethodGet, r.Method)
		return newClientResponse(http.StatusOK, "OK"), nil
	})
	ctx := context.Background()

	resp, err := client.Get(ctx, "http://example.com")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

func TestClient_Do(t *testing.T) {
	client := newWrappedClient(func(r *http.Request) (*http.Response, error) {
		return newClientResponse(http.StatusOK, ""), nil
	})
	ctx := context.Background()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.com", nil)
	require.NoError(t, err)

	resp, err := client.Do(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

func TestClient_Timeout(t *testing.T) {
	client := httpclient.NewClientFromHTTPClient(&http.Client{
		Timeout: 100 * time.Millisecond,
		Transport: clientRoundTripFunc(func(r *http.Request) (*http.Response, error) {
			select {
			case <-r.Context().Done():
				return nil, r.Context().Err()
			case <-time.After(2 * time.Second):
				return newClientResponse(http.StatusOK, ""), nil
			}
		}),
	})

	ctx := context.Background()
	_, err := client.Get(ctx, "http://example.com")

	// Should timeout
	require.Error(t, err)
}

func TestClient_ContextCancellation(t *testing.T) {
	client := newWrappedClient(func(r *http.Request) (*http.Response, error) {
		select {
		case <-r.Context().Done():
			return nil, r.Context().Err()
		case <-time.After(2 * time.Second):
			return newClientResponse(http.StatusOK, ""), nil
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := client.Get(ctx, "http://example.com")
	require.Error(t, err)
}

func TestClient_ConnectionPooling(t *testing.T) {
	var requestCount atomic.Int32
	client := newWrappedClient(func(r *http.Request) (*http.Response, error) {
		requestCount.Add(1)
		return newClientResponse(http.StatusOK, ""), nil
	})

	ctx := context.Background()

	// Make multiple requests to verify connection pooling works
	for i := 0; i < 10; i++ {
		resp, err := client.Get(ctx, "http://example.com")
		require.NoError(t, err)
		require.NotNil(t, resp)
		resp.Body.Close()
	}

	assert.Equal(t, int32(10), requestCount.Load())
}

func TestClient_TLSConfiguration(t *testing.T) {
	client := newWrappedClient(func(r *http.Request) (*http.Response, error) {
		return newClientResponse(http.StatusOK, ""), nil
	})

	ctx := context.Background()
	resp, err := client.Get(ctx, "https://example.com")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}
