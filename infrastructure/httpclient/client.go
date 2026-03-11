package httpclient

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

// Client wraps http.Client with connection pooling and TLS configuration
type Client struct {
	httpClient *http.Client
}

// Config holds configuration for the HTTP client
type Config struct {
	Timeout               time.Duration
	MaxIdleConns          int
	MaxIdleConnsPerHost   int
	IdleConnTimeout       time.Duration
	TLSHandshakeTimeout   time.Duration
	ExpectContinueTimeout time.Duration
}

// DefaultConfig returns the default HTTP client configuration
// Optimized for memory efficiency while maintaining performance
func DefaultConfig() Config {
	return Config{
		Timeout:               30 * time.Second,
		MaxIdleConns:          50,               // Reduced from 100 to save memory
		MaxIdleConnsPerHost:   5,                // Reduced from 10 to save memory
		IdleConnTimeout:       60 * time.Second, // Reduced from 90s to free connections sooner
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

// NewClient creates a new HTTP client with connection pooling and TLS configuration
func NewClient(config Config) *Client {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          config.MaxIdleConns,
		MaxIdleConnsPerHost:   config.MaxIdleConnsPerHost,
		IdleConnTimeout:       config.IdleConnTimeout,
		TLSHandshakeTimeout:   config.TLSHandshakeTimeout,
		ExpectContinueTimeout: config.ExpectContinueTimeout,
		// Configure TLS to capture certificate details
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
			MinVersion:         tls.VersionTLS12,
		},
	}

	return &Client{
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   config.Timeout,
			// Don't follow redirects automatically
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// Do executes an HTTP request with the configured client
func (c *Client) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	req = req.WithContext(ctx)
	return c.httpClient.Do(req)
}

// Get performs a GET request to the specified URL
func (c *Client) Get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(ctx, req)
}

// NewClientFromHTTPClient creates a Client wrapper from an existing http.Client
// This is useful for testing with custom http.Client configurations
func NewClientFromHTTPClient(httpClient *http.Client) *Client {
	return &Client{
		httpClient: httpClient,
	}
}

// GetHTTPClient returns the underlying http.Client for advanced usage
func (c *Client) GetHTTPClient() *http.Client {
	return c.httpClient
}
