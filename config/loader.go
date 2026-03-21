package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// LoadConfig loads configuration from multiple sources in order of precedence:
// 1. Command line specified config file
// 2. Default config files (config.yaml, config.yml, config.json)
// 3. Environment variables
// 4. Default values
func LoadConfig(configFile string) (*Config, error) {
	var config *Config
	var err error

	// If a specific config file is provided, use it
	if configFile != "" {
		config, err = LoadFromFile(configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load config from %s: %w", configFile, err)
		}
	} else {
		// Try to find default config files
		config, err = loadDefaultConfigFile()
		if err != nil {
			// If no config file found, start with defaults
			config = DefaultConfig()
		}
	}

	// Override with environment variables (only if they are actually set)
	envConfig := LoadFromEnv()
	mergeConfigsSelectively(config, envConfig)

	return config, nil
}

// LoadAndValidateConfig loads configuration and performs validation
func LoadAndValidateConfig(ctx context.Context, configFile string) (*Config, error) {
	config, err := LoadConfig(configFile)
	if err != nil {
		return nil, err
	}

	// Validate configuration structure
	if err := config.Validate(ctx); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

// LoadAndValidateWithConnectivity loads configuration and tests connectivity
func LoadAndValidateWithConnectivity(ctx context.Context, configFile string) (*Config, error) {
	config, err := LoadAndValidateConfig(ctx, configFile)
	if err != nil {
		return nil, err
	}

	// Test connectivity to external services
	if err := config.ValidateConnectivity(ctx); err != nil {
		return nil, fmt.Errorf("connectivity validation failed: %w", err)
	}

	return config, nil
}

// loadDefaultConfigFile tries to load from default config file locations
func loadDefaultConfigFile() (*Config, error) {
	defaultFiles := []string{
		"config.yaml",
		"config.yml",
		"config.json",
		"configs/config.yaml",
		"configs/config.yml",
		"configs/config.json",
	}

	for _, filename := range defaultFiles {
		if _, err := os.Stat(filename); err == nil {
			return LoadFromFile(filename)
		}
	}

	return nil, fmt.Errorf("no default config file found")
}

// mergeConfigsSelectively merges environment config into file config
// Only merges values that are explicitly set via environment variables
func mergeConfigsSelectively(fileConfig, envConfig *Config) {
	// Server config
	if hasEnv("PORT") {
		fileConfig.Server.Port = envConfig.Server.Port
	}
	if hasEnv("HOST") {
		fileConfig.Server.Host = envConfig.Server.Host
	}

	// Database config
	if hasEnv("DB_HOST") {
		fileConfig.Database.Host = envConfig.Database.Host
	}
	if hasEnv("DB_PORT") {
		fileConfig.Database.Port = envConfig.Database.Port
	}
	if hasEnv("DB_NAME") {
		fileConfig.Database.Database = envConfig.Database.Database
	}
	if hasEnv("DB_USER") {
		fileConfig.Database.User = envConfig.Database.User
	}
	if hasEnv("DB_PASSWORD") || hasEnv("POSTGRES_PASSWORD") {
		fileConfig.Database.Password = envConfig.Database.Password
	}
	if hasEnv("DB_SSL_MODE") {
		fileConfig.Database.SSLMode = envConfig.Database.SSLMode
	}

	// Redis config
	if hasEnv("REDIS_ADDR") {
		fileConfig.Redis.Addr = envConfig.Redis.Addr
	}
	if hasEnv("REDIS_PASSWORD") {
		fileConfig.Redis.Password = envConfig.Redis.Password
	}
	if hasEnv("REDIS_DB") {
		fileConfig.Redis.DB = envConfig.Redis.DB
	}
	if hasEnv("REDIS_ENABLED") {
		fileConfig.Redis.Enabled = envConfig.Redis.Enabled
	}

	// Alert config - only merge if environment variables are set
	if hasEnv("TELEGRAM_BOT_TOKEN") {
		fileConfig.Alert.Telegram.BotToken = envConfig.Alert.Telegram.BotToken
		fileConfig.Alert.Telegram.Enabled = envConfig.Alert.Telegram.Enabled
	}
	if hasEnv("TELEGRAM_CHAT_ID") {
		fileConfig.Alert.Telegram.ChatID = envConfig.Alert.Telegram.ChatID
	}

	if hasEnv("SMTP_HOST") {
		fileConfig.Alert.Email.SMTPHost = envConfig.Alert.Email.SMTPHost
		fileConfig.Alert.Email.Enabled = envConfig.Alert.Email.Enabled
	}
	if hasEnv("SMTP_PORT") {
		fileConfig.Alert.Email.SMTPPort = envConfig.Alert.Email.SMTPPort
	}
	if hasEnv("SMTP_USERNAME") {
		fileConfig.Alert.Email.Username = envConfig.Alert.Email.Username
	}
	if hasEnv("SMTP_PASSWORD") {
		fileConfig.Alert.Email.Password = envConfig.Alert.Email.Password
	}
	if hasEnv("SMTP_FROM_ADDRESS") {
		fileConfig.Alert.Email.FromAddress = envConfig.Alert.Email.FromAddress
	}

	if hasEnv("WEBHOOK_URL") {
		fileConfig.Alert.Webhook.URL = envConfig.Alert.Webhook.URL
		fileConfig.Alert.Webhook.Enabled = envConfig.Alert.Webhook.Enabled
	}

	// Worker config
	if hasEnv("WORKER_POOL_SIZE") {
		fileConfig.Worker.PoolSize = envConfig.Worker.PoolSize
	}

	// Logging config
	if hasEnv("LOG_LEVEL") {
		fileConfig.Logging.Level = envConfig.Logging.Level
	}
	if hasEnv("LOG_FORMAT") {
		fileConfig.Logging.Format = envConfig.Logging.Format
	}
}

// CreateSampleConfig creates a sample configuration file
func CreateSampleConfig(filename string) error {
	config := DefaultConfig()

	// Add some example values
	config.Alert.Telegram.BotToken = "${TELEGRAM_BOT_TOKEN}"
	config.Alert.Telegram.ChatID = "${TELEGRAM_CHAT_ID}"
	config.Alert.Email.SMTPHost = "${SMTP_HOST}"
	config.Alert.Email.Username = "${SMTP_USERNAME}"
	config.Alert.Email.Password = "${SMTP_PASSWORD}"
	config.Alert.Email.FromAddress = "${SMTP_FROM_ADDRESS}"
	config.Alert.Webhook.URL = "${WEBHOOK_URL}"
	config.Database.Password = "${DB_PASSWORD}"
	config.Redis.Password = "${REDIS_PASSWORD}"

	// Determine format by extension
	var data []byte
	var err error

	if filepath.Ext(filename) == ".json" {
		data, err = marshalJSON(config)
	} else {
		data, err = marshalYAML(config)
	}

	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Helper functions for marshaling (simplified versions)
func marshalYAML(config *Config) ([]byte, error) {
	// This is a simplified YAML output - in a real implementation,
	// you might want to use a proper YAML library with custom marshaling
	yaml := `# Uptime Monitoring System Configuration
server:
  port: %d
  host: "%s"
  read_timeout: %s
  write_timeout: %s
  idle_timeout: %s

database:
  host: "%s"
  port: %d
  database: "%s"
  user: "%s"
  password: "%s"
  ssl_mode: "%s"
  max_connections: %d
  min_connections: %d
  connect_timeout: %s
  max_conn_lifetime: %s

redis:
  enabled: %t
  addr: "%s"
  password: "%s"
  db: %d
  pool_size: %d
  min_idle_conns: %d
  dial_timeout: %s
  read_timeout: %s
  write_timeout: %s

alert:
  telegram:
    enabled: %t
    bot_token: "%s"
    chat_id: "%s"
  email:
    enabled: %t
    smtp_host: "%s"
    smtp_port: %d
    username: "%s"
    password: "%s"
    from_address: "%s"
    from_name: "%s"
    use_tls: %t
    use_starttls: %t
  webhook:
    enabled: %t
    url: "%s"
    timeout: %s

worker:
  pool_size: %d
  queue_size: %d
  job_timeout: %s
  idle_timeout: %s

logging:
  level: "%s"
  format: "%s"
  output: "%s"
`

	return []byte(fmt.Sprintf(yaml,
		config.Server.Port, config.Server.Host, config.Server.ReadTimeout,
		config.Server.WriteTimeout, config.Server.IdleTimeout,
		config.Database.Host, config.Database.Port, config.Database.Database,
		config.Database.User, config.Database.Password, config.Database.SSLMode,
		config.Database.MaxConnections, config.Database.MinConnections,
		config.Database.ConnectTimeout, config.Database.MaxConnLifetime,
		config.Redis.Enabled, config.Redis.Addr, config.Redis.Password,
		config.Redis.DB, config.Redis.PoolSize, config.Redis.MinIdleConns,
		config.Redis.DialTimeout, config.Redis.ReadTimeout, config.Redis.WriteTimeout,
		config.Alert.Telegram.Enabled, config.Alert.Telegram.BotToken, config.Alert.Telegram.ChatID,
		config.Alert.Email.Enabled, config.Alert.Email.SMTPHost, config.Alert.Email.SMTPPort,
		config.Alert.Email.Username, config.Alert.Email.Password, config.Alert.Email.FromAddress,
		config.Alert.Email.FromName, config.Alert.Email.UseTLS, config.Alert.Email.UseStartTLS,
		config.Alert.Webhook.Enabled, config.Alert.Webhook.URL, config.Alert.Webhook.Timeout,
		config.Worker.PoolSize, config.Worker.QueueSize, config.Worker.JobTimeout, config.Worker.IdleTimeout,
		config.Logging.Level, config.Logging.Format, config.Logging.Output,
	)), nil
}

func marshalJSON(config *Config) ([]byte, error) {
	// Use the standard JSON marshaling
	return []byte(`{
  "server": {
    "port": ` + fmt.Sprintf("%d", config.Server.Port) + `,
    "host": "` + config.Server.Host + `",
    "read_timeout": "` + config.Server.ReadTimeout.String() + `",
    "write_timeout": "` + config.Server.WriteTimeout.String() + `",
    "idle_timeout": "` + config.Server.IdleTimeout.String() + `"
  },
  "database": {
    "host": "` + config.Database.Host + `",
    "port": ` + fmt.Sprintf("%d", config.Database.Port) + `,
    "database": "` + config.Database.Database + `",
    "user": "` + config.Database.User + `",
    "password": "` + config.Database.Password + `",
    "ssl_mode": "` + config.Database.SSLMode + `",
    "max_connections": ` + fmt.Sprintf("%d", config.Database.MaxConnections) + `,
    "min_connections": ` + fmt.Sprintf("%d", config.Database.MinConnections) + `,
    "connect_timeout": "` + config.Database.ConnectTimeout.String() + `",
    "max_conn_lifetime": "` + config.Database.MaxConnLifetime.String() + `"
  },
  "redis": {
    "enabled": ` + fmt.Sprintf("%t", config.Redis.Enabled) + `,
    "addr": "` + config.Redis.Addr + `",
    "password": "` + config.Redis.Password + `",
    "db": ` + fmt.Sprintf("%d", config.Redis.DB) + `,
    "pool_size": ` + fmt.Sprintf("%d", config.Redis.PoolSize) + `,
    "min_idle_conns": ` + fmt.Sprintf("%d", config.Redis.MinIdleConns) + `,
    "dial_timeout": "` + config.Redis.DialTimeout.String() + `",
    "read_timeout": "` + config.Redis.ReadTimeout.String() + `",
    "write_timeout": "` + config.Redis.WriteTimeout.String() + `"
  },
  "alert": {
    "telegram": {
      "enabled": ` + fmt.Sprintf("%t", config.Alert.Telegram.Enabled) + `,
      "bot_token": "` + config.Alert.Telegram.BotToken + `",
      "chat_id": "` + config.Alert.Telegram.ChatID + `"
    },
    "email": {
      "enabled": ` + fmt.Sprintf("%t", config.Alert.Email.Enabled) + `,
      "smtp_host": "` + config.Alert.Email.SMTPHost + `",
      "smtp_port": ` + fmt.Sprintf("%d", config.Alert.Email.SMTPPort) + `,
      "username": "` + config.Alert.Email.Username + `",
      "password": "` + config.Alert.Email.Password + `",
      "from_address": "` + config.Alert.Email.FromAddress + `",
      "from_name": "` + config.Alert.Email.FromName + `",
      "use_tls": ` + fmt.Sprintf("%t", config.Alert.Email.UseTLS) + `,
      "use_starttls": ` + fmt.Sprintf("%t", config.Alert.Email.UseStartTLS) + `
    },
    "webhook": {
      "enabled": ` + fmt.Sprintf("%t", config.Alert.Webhook.Enabled) + `,
      "url": "` + config.Alert.Webhook.URL + `",
      "timeout": "` + config.Alert.Webhook.Timeout.String() + `"
    }
  },
  "worker": {
    "pool_size": ` + fmt.Sprintf("%d", config.Worker.PoolSize) + `,
    "queue_size": ` + fmt.Sprintf("%d", config.Worker.QueueSize) + `,
    "job_timeout": "` + config.Worker.JobTimeout.String() + `",
    "idle_timeout": "` + config.Worker.IdleTimeout.String() + `"
  },
  "logging": {
    "level": "` + config.Logging.Level + `",
    "format": "` + config.Logging.Format + `",
    "output": "` + config.Logging.Output + `"
  }
}`), nil
}
