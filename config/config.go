package config

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the complete application configuration
type Config struct {
	Server   ServerConfig   `yaml:"server" json:"server"`
	Database DatabaseConfig `yaml:"database" json:"database"`
	Redis    RedisConfig    `yaml:"redis" json:"redis"`
	Alert    AlertConfig    `yaml:"alert" json:"alert"`
	Worker   WorkerConfig   `yaml:"worker" json:"worker"`
	Logging  LoggingConfig  `yaml:"logging" json:"logging"`
	Session  SessionConfig  `yaml:"session" json:"session"`
}

// ServerConfig contains HTTP server configuration
type ServerConfig struct {
	Port         int           `yaml:"port" json:"port"`
	Host         string        `yaml:"host" json:"host"`
	ReadTimeout  time.Duration `yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout" json:"write_timeout"`
	IdleTimeout  time.Duration `yaml:"idle_timeout" json:"idle_timeout"`
}

// DatabaseConfig contains PostgreSQL database configuration
type DatabaseConfig struct {
	Host            string        `yaml:"host" json:"host"`
	Port            int           `yaml:"port" json:"port"`
	Database        string        `yaml:"database" json:"database"`
	User            string        `yaml:"user" json:"user"`
	Password        string        `yaml:"password" json:"password"`
	SSLMode         string        `yaml:"ssl_mode" json:"ssl_mode"`
	MaxConnections  int           `yaml:"max_connections" json:"max_connections"`
	MinConnections  int           `yaml:"min_connections" json:"min_connections"`
	ConnectTimeout  time.Duration `yaml:"connect_timeout" json:"connect_timeout"`
	MaxConnLifetime time.Duration `yaml:"max_conn_lifetime" json:"max_conn_lifetime"`
}

// RedisConfig contains Redis configuration
type RedisConfig struct {
	Addr         string        `yaml:"addr" json:"addr"`
	Password     string        `yaml:"password" json:"password"`
	DB           int           `yaml:"db" json:"db"`
	PoolSize     int           `yaml:"pool_size" json:"pool_size"`
	MinIdleConns int           `yaml:"min_idle_conns" json:"min_idle_conns"`
	DialTimeout  time.Duration `yaml:"dial_timeout" json:"dial_timeout"`
	ReadTimeout  time.Duration `yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout" json:"write_timeout"`
	Enabled      bool          `yaml:"enabled" json:"enabled"`
}

// AlertConfig contains alert channel configurations
type AlertConfig struct {
	Telegram TelegramConfig `yaml:"telegram" json:"telegram"`
	Email    EmailConfig    `yaml:"email" json:"email"`
	Webhook  WebhookConfig  `yaml:"webhook" json:"webhook"`
}

// TelegramConfig contains Telegram bot configuration
type TelegramConfig struct {
	BotToken string `yaml:"bot_token" json:"bot_token"`
	ChatID   string `yaml:"chat_id" json:"chat_id"`
	Enabled  bool   `yaml:"enabled" json:"enabled"`
}

// EmailConfig contains SMTP email configuration
type EmailConfig struct {
	SMTPHost    string `yaml:"smtp_host" json:"smtp_host"`
	SMTPPort    int    `yaml:"smtp_port" json:"smtp_port"`
	Username    string `yaml:"username" json:"username"`
	Password    string `yaml:"password" json:"password"`
	FromAddress string `yaml:"from_address" json:"from_address"`
	FromName    string `yaml:"from_name" json:"from_name"`
	UseTLS      bool   `yaml:"use_tls" json:"use_tls"`
	UseStartTLS bool   `yaml:"use_starttls" json:"use_starttls"`
	Enabled     bool   `yaml:"enabled" json:"enabled"`
}

// WebhookConfig contains webhook configuration
type WebhookConfig struct {
	URL     string            `yaml:"url" json:"url"`
	Headers map[string]string `yaml:"headers" json:"headers"`
	Timeout time.Duration     `yaml:"timeout" json:"timeout"`
	Enabled bool              `yaml:"enabled" json:"enabled"`
}

// WorkerConfig contains worker pool configuration
type WorkerConfig struct {
	PoolSize    int           `yaml:"pool_size" json:"pool_size"`
	QueueSize   int           `yaml:"queue_size" json:"queue_size"`
	JobTimeout  time.Duration `yaml:"job_timeout" json:"job_timeout"`
	IdleTimeout time.Duration `yaml:"idle_timeout" json:"idle_timeout"`
}

// LoggingConfig contains logging configuration
type LoggingConfig struct {
	Level  string `yaml:"level" json:"level"`
	Format string `yaml:"format" json:"format"` // json or text
	Output string `yaml:"output" json:"output"` // stdout, stderr, or file path
}

// SessionConfig contains session management configuration
type SessionConfig struct {
	TTL          time.Duration `yaml:"ttl" json:"ttl"`                     // Session time-to-live (default: 24h)
	CookieSecure bool          `yaml:"cookie_secure" json:"cookie_secure"` // Set Secure flag on cookies (use true in production with HTTPS)
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:         8080,
			Host:         "0.0.0.0",
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  120 * time.Second,
		},
		Database: DatabaseConfig{
			Host:            "localhost",
			Port:            5432,
			Database:        "uptime_monitor",
			User:            "postgres",
			Password:        "",
			SSLMode:         "prefer",
			MaxConnections:  20,
			MinConnections:  2,
			ConnectTimeout:  10 * time.Second,
			MaxConnLifetime: 1 * time.Hour,
		},
		Redis: RedisConfig{
			Addr:         "localhost:6379",
			Password:     "",
			DB:           0,
			PoolSize:     5, // Reduced for memory efficiency
			MinIdleConns: 1, // Reduced for memory efficiency
			DialTimeout:  5 * time.Second,
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 3 * time.Second,
			Enabled:      true,
		},
		Alert: AlertConfig{
			Telegram: TelegramConfig{
				Enabled: false,
			},
			Email: EmailConfig{
				SMTPPort:    587,
				UseTLS:      true,
				UseStartTLS: true,
				Enabled:     false,
			},
			Webhook: WebhookConfig{
				Timeout: 10 * time.Second,
				Headers: make(map[string]string),
				Enabled: false,
			},
		},
		Worker: WorkerConfig{
			PoolSize:    8,   // Reduced from 10 for memory efficiency
			QueueSize:   500, // Reduced from 1000 for memory efficiency
			JobTimeout:  60 * time.Second,
			IdleTimeout: 5 * time.Minute,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
		Session: SessionConfig{
			TTL:          24 * time.Hour,
			CookieSecure: false,
		},
	}
}

// LoadFromFile loads configuration from a YAML or JSON file
func LoadFromFile(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", filename, err)
	}

	// Substitute environment variables
	data = []byte(substituteEnvVars(string(data)))

	config := DefaultConfig()

	// Determine file type by extension
	if strings.HasSuffix(strings.ToLower(filename), ".json") {
		if err := json.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse JSON config file %s: %w", filename, err)
		}
	} else {
		// Default to YAML
		if err := yaml.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config file %s: %w", filename, err)
		}
	}

	return config, nil
}

// LoadFromEnv loads configuration from environment variables, using defaults as base
func LoadFromEnv() *Config {
	config := DefaultConfig()

	// Server configuration
	if port := getEnv("PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			config.Server.Port = p
		}
	}
	if host := getEnv("HOST"); host != "" {
		config.Server.Host = host
	}

	// Database configuration
	if host := getEnv("DB_HOST"); host != "" {
		config.Database.Host = host
	}
	if port := getEnv("DB_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			config.Database.Port = p
		}
	}
	if dbName := getEnv("DB_NAME"); dbName != "" {
		config.Database.Database = dbName
	}
	if user := getEnv("DB_USER"); user != "" {
		config.Database.User = user
	}
	if password := getEnv("DB_PASSWORD"); password != "" {
		config.Database.Password = password
	} else if password := getEnv("POSTGRES_PASSWORD"); password != "" {
		config.Database.Password = password
	}
	if sslMode := getEnv("DB_SSL_MODE"); sslMode != "" {
		config.Database.SSLMode = sslMode
	}

	// Redis configuration
	if addr := getEnv("REDIS_ADDR"); addr != "" {
		config.Redis.Addr = addr
	}
	if password := getEnv("REDIS_PASSWORD"); password != "" {
		config.Redis.Password = password
	}
	if db := getEnv("REDIS_DB"); db != "" {
		if d, err := strconv.Atoi(db); err == nil {
			config.Redis.DB = d
		}
	}
	if enabled := getEnv("REDIS_ENABLED"); enabled != "" {
		config.Redis.Enabled = strings.ToLower(enabled) == "true"
	}

	// Alert configuration
	if botToken := getEnv("TELEGRAM_BOT_TOKEN"); botToken != "" {
		config.Alert.Telegram.BotToken = botToken
		config.Alert.Telegram.Enabled = true
	}
	if chatID := getEnv("TELEGRAM_CHAT_ID"); chatID != "" {
		config.Alert.Telegram.ChatID = chatID
	}

	if smtpHost := getEnv("SMTP_HOST"); smtpHost != "" {
		config.Alert.Email.SMTPHost = smtpHost
		config.Alert.Email.Enabled = true
	}
	if smtpPort := getEnv("SMTP_PORT"); smtpPort != "" {
		if p, err := strconv.Atoi(smtpPort); err == nil {
			config.Alert.Email.SMTPPort = p
		}
	}
	if username := getEnv("SMTP_USERNAME"); username != "" {
		config.Alert.Email.Username = username
	}
	if password := getEnv("SMTP_PASSWORD"); password != "" {
		config.Alert.Email.Password = password
	}
	if fromAddr := getEnv("SMTP_FROM_ADDRESS"); fromAddr != "" {
		config.Alert.Email.FromAddress = fromAddr
	}

	if webhookURL := getEnv("WEBHOOK_URL"); webhookURL != "" {
		config.Alert.Webhook.URL = webhookURL
		config.Alert.Webhook.Enabled = true
	}

	// Worker configuration
	if poolSize := getEnv("WORKER_POOL_SIZE"); poolSize != "" {
		if p, err := strconv.Atoi(poolSize); err == nil {
			config.Worker.PoolSize = p
		}
	}

	// Logging configuration
	if level := getEnv("LOG_LEVEL"); level != "" {
		config.Logging.Level = level
	}
	if format := getEnv("LOG_FORMAT"); format != "" {
		config.Logging.Format = format
	}

	return config
}

// substituteEnvVars replaces ${VAR_NAME} and $VAR_NAME patterns with environment variable values
func substituteEnvVars(content string) string {
	// Pattern for ${VAR_NAME}
	re1 := regexp.MustCompile(`\$\{([^}]+)\}`)
	content = re1.ReplaceAllStringFunc(content, func(match string) string {
		varName := match[2 : len(match)-1] // Remove ${ and }
		if value := getEnv(varName); value != "" {
			return value
		}
		return match // Return original if env var not found
	})

	// Pattern for $VAR_NAME (word boundaries)
	re2 := regexp.MustCompile(`\$([A-Za-z_][A-Za-z0-9_]*)`)
	content = re2.ReplaceAllStringFunc(content, func(match string) string {
		varName := match[1:] // Remove $
		if value := getEnv(varName); value != "" {
			return value
		}
		return match // Return original if env var not found
	})

	return content
}
