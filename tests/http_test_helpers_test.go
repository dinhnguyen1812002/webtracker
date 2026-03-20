package tests

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"io"
	"net/http"
	"strings"
	"time"

	"web-tracker/infrastructure/httpclient"
)

type testRoundTripFunc func(*http.Request) (*http.Response, error)

func (f testRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newStubHTTPClient(fn testRoundTripFunc) *httpclient.Client {
	return httpclient.NewClientFromHTTPClient(&http.Client{
		Transport: fn,
	})
}

func newStubResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func newDelayedResponse(statusCode int, delay time.Duration) testRoundTripFunc {
	return func(req *http.Request) (*http.Response, error) {
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		case <-time.After(delay):
			return newStubResponse(statusCode, ""), nil
		}
	}
}

func newTLSResponse(statusCode int, valid bool) *http.Response {
	now := time.Now()
	cert := &x509.Certificate{
		Subject: pkix.Name{
			CommonName:   "example.com",
			Organization: []string{"Test CA"},
		},
		Issuer: pkix.Name{
			CommonName:   "Test CA",
			Organization: []string{"Test CA"},
		},
		DNSNames:  []string{"example.com"},
		NotBefore: now.Add(-time.Hour),
		NotAfter:  now.Add(30 * 24 * time.Hour),
	}

	handshakeComplete := valid
	if !valid {
		cert.NotAfter = now.Add(-time.Hour)
	}

	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader("")),
		Header:     make(http.Header),
		TLS: &tls.ConnectionState{
			HandshakeComplete: handshakeComplete,
			PeerCertificates:  []*x509.Certificate{cert},
		},
	}
}

func newCanceledContext(ctx context.Context, delay time.Duration) (context.Context, context.CancelFunc) {
	childCtx, cancel := context.WithCancel(ctx)
	go func() {
		select {
		case <-childCtx.Done():
		case <-time.After(delay):
			cancel()
		}
	}()
	return childCtx, cancel
}
