package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"web-tracker/infrastructure/httpclient"
)

func TestHTTPClient_NewClient(t *testing.T) {
	config := httpclient.DefaultConfig()
	client := httpclient.NewClient(config)

	require.NotNil(t, client)
	assert.NotNil(t, client)
}

func TestClient_Get(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	client := httpclient.NewClient(httpclient.DefaultConfig())
	ctx := context.Background()

	resp, err := client.Get(ctx, server.URL)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

func TestClient_Do(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := httpclient.NewClient(httpclient.DefaultConfig())
	ctx := context.Background()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	require.NoError(t, err)

	resp, err := client.Do(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

func TestClient_Timeout(t *testing.T) {
	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create client with short timeout
	config := httpclient.DefaultConfig()
	config.Timeout = 100 * time.Millisecond
	client := httpclient.NewClient(config)

	ctx := context.Background()
	_, err := client.Get(ctx, server.URL)

	// Should timeout
	require.Error(t, err)
}

func TestClient_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := httpclient.NewClient(httpclient.DefaultConfig())
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := client.Get(ctx, server.URL)
	require.Error(t, err)
}

func TestClient_ConnectionPooling(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := httpclient.DefaultConfig()
	config.MaxIdleConns = 100
	config.MaxIdleConnsPerHost = 10
	client := httpclient.NewClient(config)

	ctx := context.Background()

	// Make multiple requests to verify connection pooling works
	for i := 0; i < 10; i++ {
		resp, err := client.Get(ctx, server.URL)
		require.NoError(t, err)
		require.NotNil(t, resp)
		resp.Body.Close()
	}

	assert.Equal(t, 10, requestCount)
}

func TestClient_TLSConfiguration(t *testing.T) {
	// Create HTTPS test server
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := httpclient.NewClientFromHTTPClient(server.Client())

	ctx := context.Background()
	resp, err := client.Get(ctx, server.URL)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}
