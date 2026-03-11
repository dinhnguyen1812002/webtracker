package config

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error in %s: %s", e.Field, e.Message)
}

// ValidationErrors represents multiple validation errors
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return "no validation errors"
	}

	var messages []string
	for _, err := range e {
		messages = append(messages, err.Error())
	}
	return fmt.Sprintf("configuration validation failed:\n  - %s", strings.Join(messages, "\n  - "))
}

// Validate performs comprehensive validation of the configuration
func (c *Config) Validate(ctx context.Context) error {
	var errors ValidationErrors

	// Validate server configuration
	if err := c.validateServer(); err != nil {
		errors = append(errors, err...)
	}

	// Validate database configuration
	if err := c.validateDatabase(); err != nil {
		errors = append(errors, err...)
	}

	// Validate Redis configuration
	if err := c.validateRedis(); err != nil {
		errors = append(errors, err...)
	}

	// Validate alert configurations
	if err := c.validateAlert(); err != nil {
		errors = append(errors, err...)
	}

	// Validate worker configuration
	if err := c.validateWorker(); err != nil {
		errors = append(errors, err...)
	}

	// Validate logging configuration
	if err := c.validateLogging(); err != nil {
		errors = append(errors, err...)
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

// ValidateConnectivity tests actual connectivity to external services
func (c *Config) ValidateConnectivity(ctx context.Context) error {
	var errors ValidationErrors

	// Test database connectivity
	if err := c.testDatabaseConnectivity(ctx); err != nil {
		errors = append(errors, ValidationError{
			Field:   "database",
			Message: fmt.Sprintf("database connectivity test failed: %v", err),
		})
	}

	// Test Redis connectivity (if enabled)
	if c.Redis.Enabled {
		if err := c.testRedisConnectivity(ctx); err != nil {
			errors = append(errors, ValidationError{
				Field:   "redis",
				Message: fmt.Sprintf("redis connectivity test failed: %v", err),
			})
		}
	}

	// Test alert channel connectivity
	if err := c.testAlertChannelConnectivity(ctx); err != nil {
		errors = append(errors, err...)
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

func (c *Config) validateServer() ValidationErrors {
	var errors ValidationErrors

	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		errors = append(errors, ValidationError{
			Field:   "server.port",
			Message: "port must be between 1 and 65535",
		})
	}

	if c.Server.Host == "" {
		errors = append(errors, ValidationError{
			Field:   "server.host",
			Message: "host cannot be empty",
		})
	}

	if c.Server.ReadTimeout <= 0 {
		errors = append(errors, ValidationError{
			Field:   "server.read_timeout",
			Message: "read_timeout must be positive",
		})
	}

	if c.Server.WriteTimeout <= 0 {
		errors = append(errors, ValidationError{
			Field:   "server.write_timeout",
			Message: "write_timeout must be positive",
		})
	}

	return errors
}

func (c *Config) validateDatabase() ValidationErrors {
	var errors ValidationErrors

	if c.Database.Host == "" {
		errors = append(errors, ValidationError{
			Field:   "database.host",
			Message: "host cannot be empty",
		})
	}

	if c.Database.Port <= 0 || c.Database.Port > 65535 {
		errors = append(errors, ValidationError{
			Field:   "database.port",
			Message: "port must be between 1 and 65535",
		})
	}

	if c.Database.Database == "" {
		errors = append(errors, ValidationError{
			Field:   "database.database",
			Message: "database name cannot be empty",
		})
	}

	if c.Database.User == "" {
		errors = append(errors, ValidationError{
			Field:   "database.user",
			Message: "user cannot be empty",
		})
	}

	validSSLModes := []string{"disable", "allow", "prefer", "require", "verify-ca", "verify-full"}
	isValidSSLMode := false
	for _, mode := range validSSLModes {
		if c.Database.SSLMode == mode {
			isValidSSLMode = true
			break
		}
	}
	if !isValidSSLMode {
		errors = append(errors, ValidationError{
			Field:   "database.ssl_mode",
			Message: fmt.Sprintf("ssl_mode must be one of: %s", strings.Join(validSSLModes, ", ")),
		})
	}

	if c.Database.MaxConnections <= 0 {
		errors = append(errors, ValidationError{
			Field:   "database.max_connections",
			Message: "max_connections must be positive",
		})
	}

	if c.Database.MinConnections < 0 {
		errors = append(errors, ValidationError{
			Field:   "database.min_connections",
			Message: "min_connections cannot be negative",
		})
	}

	if c.Database.MinConnections > c.Database.MaxConnections {
		errors = append(errors, ValidationError{
			Field:   "database.min_connections",
			Message: "min_connections cannot be greater than max_connections",
		})
	}

	return errors
}

func (c *Config) validateRedis() ValidationErrors {
	var errors ValidationErrors

	if !c.Redis.Enabled {
		return errors // Skip validation if Redis is disabled
	}

	if c.Redis.Addr == "" {
		errors = append(errors, ValidationError{
			Field:   "redis.addr",
			Message: "addr cannot be empty when Redis is enabled",
		})
	}

	if c.Redis.DB < 0 {
		errors = append(errors, ValidationError{
			Field:   "redis.db",
			Message: "db cannot be negative",
		})
	}

	if c.Redis.PoolSize <= 0 {
		errors = append(errors, ValidationError{
			Field:   "redis.pool_size",
			Message: "pool_size must be positive",
		})
	}

	return errors
}

func (c *Config) validateAlert() ValidationErrors {
	var errors ValidationErrors

	// Validate Telegram configuration
	if c.Alert.Telegram.Enabled {
		if c.Alert.Telegram.BotToken == "" {
			errors = append(errors, ValidationError{
				Field:   "alert.telegram.bot_token",
				Message: "bot_token cannot be empty when Telegram is enabled",
			})
		}
		if c.Alert.Telegram.ChatID == "" {
			errors = append(errors, ValidationError{
				Field:   "alert.telegram.chat_id",
				Message: "chat_id cannot be empty when Telegram is enabled",
			})
		}
	}

	// Validate Email configuration
	if c.Alert.Email.Enabled {
		if c.Alert.Email.SMTPHost == "" {
			errors = append(errors, ValidationError{
				Field:   "alert.email.smtp_host",
				Message: "smtp_host cannot be empty when Email is enabled",
			})
		}
		if c.Alert.Email.SMTPPort <= 0 || c.Alert.Email.SMTPPort > 65535 {
			errors = append(errors, ValidationError{
				Field:   "alert.email.smtp_port",
				Message: "smtp_port must be between 1 and 65535",
			})
		}
		if c.Alert.Email.FromAddress == "" {
			errors = append(errors, ValidationError{
				Field:   "alert.email.from_address",
				Message: "from_address cannot be empty when Email is enabled",
			})
		}
		// Basic email format validation
		if !strings.Contains(c.Alert.Email.FromAddress, "@") {
			errors = append(errors, ValidationError{
				Field:   "alert.email.from_address",
				Message: "from_address must be a valid email address",
			})
		}
	}

	// Validate Webhook configuration
	if c.Alert.Webhook.Enabled {
		if c.Alert.Webhook.URL == "" {
			errors = append(errors, ValidationError{
				Field:   "alert.webhook.url",
				Message: "url cannot be empty when Webhook is enabled",
			})
		} else {
			parsedURL, err := url.Parse(c.Alert.Webhook.URL)
			if err != nil {
				errors = append(errors, ValidationError{
					Field:   "alert.webhook.url",
					Message: fmt.Sprintf("url must be a valid URL: %v", err),
				})
			} else if parsedURL.Scheme == "" || parsedURL.Host == "" {
				errors = append(errors, ValidationError{
					Field:   "alert.webhook.url",
					Message: "url must have a valid scheme (http/https) and host",
				})
			} else if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
				errors = append(errors, ValidationError{
					Field:   "alert.webhook.url",
					Message: "url scheme must be http or https",
				})
			}
		}
		if c.Alert.Webhook.Timeout <= 0 {
			errors = append(errors, ValidationError{
				Field:   "alert.webhook.timeout",
				Message: "timeout must be positive",
			})
		}
	}

	return errors
}

func (c *Config) validateWorker() ValidationErrors {
	var errors ValidationErrors

	if c.Worker.PoolSize <= 0 {
		errors = append(errors, ValidationError{
			Field:   "worker.pool_size",
			Message: "pool_size must be positive",
		})
	}

	if c.Worker.QueueSize <= 0 {
		errors = append(errors, ValidationError{
			Field:   "worker.queue_size",
			Message: "queue_size must be positive",
		})
	}

	if c.Worker.JobTimeout <= 0 {
		errors = append(errors, ValidationError{
			Field:   "worker.job_timeout",
			Message: "job_timeout must be positive",
		})
	}

	return errors
}

func (c *Config) validateLogging() ValidationErrors {
	var errors ValidationErrors

	validLevels := []string{"debug", "info", "warn", "error", "fatal", "panic"}
	isValidLevel := false
	for _, level := range validLevels {
		if strings.ToLower(c.Logging.Level) == level {
			isValidLevel = true
			break
		}
	}
	if !isValidLevel {
		errors = append(errors, ValidationError{
			Field:   "logging.level",
			Message: fmt.Sprintf("level must be one of: %s", strings.Join(validLevels, ", ")),
		})
	}

	validFormats := []string{"json", "text"}
	isValidFormat := false
	for _, format := range validFormats {
		if strings.ToLower(c.Logging.Format) == format {
			isValidFormat = true
			break
		}
	}
	if !isValidFormat {
		errors = append(errors, ValidationError{
			Field:   "logging.format",
			Message: fmt.Sprintf("format must be one of: %s", strings.Join(validFormats, ", ")),
		})
	}

	return errors
}

func (c *Config) testDatabaseConnectivity(ctx context.Context) error {
	connStr := fmt.Sprintf("host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
		c.Database.Host, c.Database.Port, c.Database.Database,
		c.Database.User, c.Database.Password, c.Database.SSLMode)

	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return fmt.Errorf("invalid database configuration: %w", err)
	}

	config.MaxConns = int32(c.Database.MaxConnections)
	config.MinConns = int32(c.Database.MinConnections)
	config.MaxConnLifetime = c.Database.MaxConnLifetime
	config.ConnConfig.ConnectTimeout = c.Database.ConnectTimeout

	// Create a test connection with a short timeout
	testCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(testCtx, config)
	if err != nil {
		return fmt.Errorf("failed to create database pool: %w", err)
	}
	defer pool.Close()

	// Test the connection
	if err := pool.Ping(testCtx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	return nil
}

func (c *Config) testRedisConnectivity(ctx context.Context) error {
	client := redis.NewClient(&redis.Options{
		Addr:         c.Redis.Addr,
		Password:     c.Redis.Password,
		DB:           c.Redis.DB,
		PoolSize:     c.Redis.PoolSize,
		MinIdleConns: c.Redis.MinIdleConns,
		DialTimeout:  c.Redis.DialTimeout,
		ReadTimeout:  c.Redis.ReadTimeout,
		WriteTimeout: c.Redis.WriteTimeout,
	})
	defer client.Close()

	// Test the connection with a short timeout
	testCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := client.Ping(testCtx).Err(); err != nil {
		return fmt.Errorf("failed to ping Redis: %w", err)
	}

	return nil
}

func (c *Config) testAlertChannelConnectivity(ctx context.Context) ValidationErrors {
	var errors ValidationErrors

	// Test Telegram connectivity
	if c.Alert.Telegram.Enabled {
		if err := c.testTelegramConnectivity(ctx); err != nil {
			errors = append(errors, ValidationError{
				Field:   "alert.telegram",
				Message: fmt.Sprintf("Telegram connectivity test failed: %v", err),
			})
		}
	}

	// Test Email connectivity
	if c.Alert.Email.Enabled {
		if err := c.testEmailConnectivity(ctx); err != nil {
			errors = append(errors, ValidationError{
				Field:   "alert.email",
				Message: fmt.Sprintf("Email connectivity test failed: %v", err),
			})
		}
	}

	// Test Webhook connectivity
	if c.Alert.Webhook.Enabled {
		if err := c.testWebhookConnectivity(ctx); err != nil {
			errors = append(errors, ValidationError{
				Field:   "alert.webhook",
				Message: fmt.Sprintf("Webhook connectivity test failed: %v", err),
			})
		}
	}

	return errors
}

func (c *Config) testTelegramConnectivity(ctx context.Context) error {
	// Test Telegram Bot API connectivity
	testURL := fmt.Sprintf("https://api.telegram.org/bot%s/getMe", c.Alert.Telegram.BotToken)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", testURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to Telegram API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Telegram API returned status %d", resp.StatusCode)
	}

	return nil
}

func (c *Config) testEmailConnectivity(ctx context.Context) error {
	// Test SMTP connectivity
	addr := net.JoinHostPort(c.Alert.Email.SMTPHost, strconv.Itoa(c.Alert.Email.SMTPPort))

	// Create connection with timeout
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer conn.Close()

	// Test SMTP handshake
	client, err := smtp.NewClient(conn, c.Alert.Email.SMTPHost)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	defer client.Quit()

	// Test STARTTLS if enabled
	if c.Alert.Email.UseStartTLS {
		tlsConfig := &tls.Config{
			ServerName: c.Alert.Email.SMTPHost,
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("failed to start TLS: %w", err)
		}
	}

	// Test authentication if credentials provided
	if c.Alert.Email.Username != "" && c.Alert.Email.Password != "" {
		auth := smtp.PlainAuth("", c.Alert.Email.Username, c.Alert.Email.Password, c.Alert.Email.SMTPHost)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP authentication failed: %w", err)
		}
	}

	return nil
}

func (c *Config) testWebhookConnectivity(ctx context.Context) error {
	// Test webhook URL connectivity with a HEAD request
	client := &http.Client{
		Timeout: c.Alert.Webhook.Timeout,
	}

	req, err := http.NewRequestWithContext(ctx, "HEAD", c.Alert.Webhook.URL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add custom headers
	for key, value := range c.Alert.Webhook.Headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to webhook URL: %w", err)
	}
	defer resp.Body.Close()

	// Accept any 2xx or 4xx status (4xx means the endpoint exists but doesn't accept HEAD)
	if resp.StatusCode >= 500 {
		return fmt.Errorf("webhook URL returned server error: %d", resp.StatusCode)
	}

	return nil
}
