# HTTP Client Infrastructure

This package provides an HTTP client with connection pooling, TLS configuration, and retry logic with exponential backoff for the Uptime Monitoring System.

## Features

- **Connection Pooling**: Reuses HTTP connections to improve performance
  - MaxIdleConns: 100
  - MaxIdleConnsPerHost: 10
  
- **TLS Configuration**: Captures SSL certificate details for HTTPS monitoring
  - Minimum TLS version: 1.2
  - Certificate validation enabled
  
- **Timeout Configuration**: 30-second default timeout for requests

- **Retry Logic**: Exponential backoff for failed requests
  - Max attempts: 3 (configurable)
  - Backoff delays: 1s, 2s, 4s
  - Context-aware cancellation

## Usage

### Basic Usage

```go
import "web-tracker/infrastructure/httpclient"

// Create client with default configuration
client := httpclient.NewClient(httpclient.DefaultConfig())

// Perform a simple GET request
ctx := context.Background()
resp, err := client.Get(ctx, "https://example.com")
if err != nil {
    log.Fatal(err)
}
defer resp.Body.Close()
```

### Custom Configuration

```go
config := httpclient.Config{
    Timeout:              30 * time.Second,
    MaxIdleConns:         100,
    MaxIdleConnsPerHost:  10,
    IdleConnTimeout:      90 * time.Second,
    TLSHandshakeTimeout:  10 * time.Second,
    ExpectContinueTimeout: 1 * time.Second,
}

client := httpclient.NewClient(config)
```

### Using Retry Logic

```go
// Create client
client := httpclient.NewClient(httpclient.DefaultConfig())

// Use default retry configuration (3 attempts, 1s initial delay)
retryConfig := httpclient.DefaultRetryConfig()

// Perform GET request with retry
ctx := context.Background()
resp, err := client.GetWithRetry(ctx, "https://example.com", retryConfig)
if err != nil {
    log.Fatal(err)
}
defer resp.Body.Close()
```

### Custom Retry Configuration

```go
retryConfig := httpclient.RetryConfig{
    MaxAttempts:  5,
    InitialDelay: 2 * time.Second,
}

req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com", nil)
resp, err := client.DoWithRetry(ctx, req, retryConfig)
```

### Context Cancellation

```go
// Create context with timeout
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

// Request will be cancelled if it exceeds the timeout
resp, err := client.Get(ctx, "https://example.com")
```

## Design Decisions

### Connection Pooling

The client uses connection pooling to reuse TCP connections across multiple HTTP requests. This significantly improves performance when making multiple requests to the same host, which is common in health check scenarios.

**Configuration:**
- `MaxIdleConns`: Maximum number of idle connections across all hosts (100)
- `MaxIdleConnsPerHost`: Maximum idle connections per host (10)
- `IdleConnTimeout`: How long an idle connection remains in the pool (90s)

### TLS Configuration

The client is configured to capture SSL certificate details during HTTPS requests. This enables the monitoring system to:
- Validate certificate chains
- Extract expiration dates
- Identify certificate issuers
- Detect SSL-related issues

### Retry Logic with Exponential Backoff

Failed requests are retried with exponential backoff to handle transient network issues:

**Retry Schedule:**
- Attempt 1: Immediate
- Attempt 2: After 1 second
- Attempt 3: After 2 seconds (total 3s from attempt 1)

**When to Retry:**
- Network errors (connection refused, timeout, DNS failure)
- Context cancellation stops retries immediately

**When NOT to Retry:**
- Successful HTTP responses (even with error status codes like 500)
- Invalid request configuration (malformed URL)

### No Automatic Redirects

The client is configured to NOT follow redirects automatically (`CheckRedirect` returns `http.ErrUseLastResponse`). This allows the monitoring system to:
- Detect redirect chains
- Monitor redirect behavior
- Record the actual response from the monitored endpoint

## Testing

Run tests with:

```bash
go test ./infrastructure/httpclient/...
```

Run tests with coverage:

```bash
go test -cover ./infrastructure/httpclient/...
```

## Requirements Satisfied

- **Requirement 1.1**: HTTP/HTTPS request execution
- **Requirement 1.5**: 30-second timeout
- **Requirement 1.6**: Retry logic with exponential backoff (3 attempts)
- **Requirement 12.4**: HTTP connection reuse via connection pooling
