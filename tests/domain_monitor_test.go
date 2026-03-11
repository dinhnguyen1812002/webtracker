package tests

import (
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"

	"web-tracker/domain"
)

// Feature: uptime-monitoring-system, Property 8: Check Interval Validation
// **Validates: Requirements 3.1, 3.2**
//
// Property 8: Check Interval Validation
// For any monitor creation or update request, the system should accept check intervals
// of 1, 5, 15, or 60 minutes and reject all other values.
func TestProperty_CheckIntervalValidation(t *testing.T) {
	properties := gopter.NewProperties(nil)

	// Valid intervals should be accepted
	properties.Property("valid check intervals (1, 5, 15, 60 minutes) are accepted",
		prop.ForAll(
			func(intervalMinutes int) bool {
				interval := time.Duration(intervalMinutes) * time.Minute
				err := domain.ValidateCheckInterval(interval)
				return err == nil
			},
			gen.OneConstOf(1, 5, 15, 60),
		))

	// Invalid intervals should be rejected
	properties.Property("invalid check intervals are rejected",
		prop.ForAll(
			func(intervalMinutes int) bool {
				interval := time.Duration(intervalMinutes) * time.Minute
				err := domain.ValidateCheckInterval(interval)
				return err != nil
			},
			gen.IntRange(1, 120).SuchThat(func(v interface{}) bool {
				minutes := v.(int)
				return minutes != 1 && minutes != 5 && minutes != 15 && minutes != 60
			}),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Feature: uptime-monitoring-system, Property 12: URL Validation
// **Validates: Requirements 10.1**
//
// Property 12: URL Validation
// For any monitor creation request, the system should validate that the URL is a
// properly formatted HTTP or HTTPS endpoint.
func TestProperty_URLValidation(t *testing.T) {
	properties := gopter.NewProperties(nil)

	// Valid HTTP/HTTPS URLs should be accepted
	properties.Property("valid HTTP/HTTPS URLs are accepted",
		prop.ForAll(
			func(scheme string, host string, path string) bool {
				url := scheme + "://" + host + path
				err := domain.ValidateURL(url)
				return err == nil
			},
			gen.OneConstOf("http", "https"),
			gen.OneConstOf("example.com", "test.org", "localhost", "192.168.1.1", "subdomain.example.com"),
			gen.OneConstOf("", "/", "/path", "/path/to/resource", "/api/v1/health"),
		))

	// URLs with invalid schemes should be rejected
	properties.Property("URLs with non-HTTP/HTTPS schemes are rejected",
		prop.ForAll(
			func(scheme string) bool {
				url := scheme + "://example.com"
				err := domain.ValidateURL(url)
				return err != nil
			},
			gen.OneConstOf("ftp", "file", "ws", "wss", "ssh", "telnet", ""),
		))

	// Empty URLs should be rejected
	properties.Property("empty URLs are rejected",
		prop.ForAll(
			func() bool {
				err := domain.ValidateURL("")
				return err != nil
			},
		))

	// URLs without host should be rejected
	properties.Property("URLs without host are rejected",
		prop.ForAll(
			func(scheme string) bool {
				url := scheme + "://"
				err := domain.ValidateURL(url)
				return err != nil
			},
			gen.OneConstOf("http", "https"),
		))

	// Malformed URLs should be rejected
	properties.Property("malformed URLs are rejected",
		prop.ForAll(
			func(malformedURL string) bool {
				err := domain.ValidateURL(malformedURL)
				return err != nil
			},
			gen.OneConstOf(
				"not a url",
				"://missing-scheme.com",
				"http://",
				"https://",
				"http:// spaces .com",
				"http://[invalid",
			),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}
