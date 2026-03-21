package tests

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	configpkg "web-tracker/config"
)

func TestDefaultConfig(t *testing.T) {
	config := configpkg.DefaultConfig()

	assert.Equal(t, 8080, config.Server.Port)
	assert.Equal(t, "0.0.0.0", config.Server.Host)
	assert.Equal(t, "localhost", config.Database.Host)
	assert.Equal(t, 5432, config.Database.Port)
	assert.Equal(t, "uptime_monitor", config.Database.Database)
	assert.Equal(t, "postgres", config.Database.User)
	assert.Equal(t, "prefer", config.Database.SSLMode)
	assert.Equal(t, 20, config.Database.MaxConnections)
	assert.Equal(t, 2, config.Database.MinConnections)
	assert.Equal(t, "localhost:6379", config.Redis.Addr)
	assert.Equal(t, 0, config.Redis.DB)
	assert.True(t, config.Redis.Enabled)
	assert.Equal(t, 8, config.Worker.PoolSize)
	assert.Equal(t, "info", config.Logging.Level)
	assert.Equal(t, "json", config.Logging.Format)
}

func TestLoadFromEnv(t *testing.T) {
	// Set environment variables
	os.Setenv("PORT", "9090")
	os.Setenv("DB_HOST", "db.example.com")
	os.Setenv("DB_PORT", "5433")
	os.Setenv("DB_NAME", "test_db")
	os.Setenv("DB_USER", "testuser")
	os.Setenv("DB_PASSWORD", "testpass")
	os.Setenv("REDIS_ADDR", "redis.example.com:6380")
	os.Setenv("REDIS_PASSWORD", "redispass")
	os.Setenv("REDIS_DB", "1")
	os.Setenv("TELEGRAM_BOT_TOKEN", "123456:ABC-DEF")
	os.Setenv("TELEGRAM_CHAT_ID", "123456789")
	os.Setenv("SMTP_HOST", "smtp.example.com")
	os.Setenv("SMTP_PORT", "465")
	os.Setenv("SMTP_USERNAME", "user@example.com")
	os.Setenv("SMTP_PASSWORD", "smtppass")
	os.Setenv("SMTP_FROM_ADDRESS", "noreply@example.com")
	os.Setenv("WEBHOOK_URL", "https://webhook.example.com/alert")
	os.Setenv("WORKER_POOL_SIZE", "20")
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("LOG_FORMAT", "text")

	defer func() {
		// Clean up environment variables
		envVars := []string{
			"PORT", "DB_HOST", "DB_PORT", "DB_NAME", "DB_USER", "DB_PASSWORD",
			"REDIS_ADDR", "REDIS_PASSWORD", "REDIS_DB",
			"TELEGRAM_BOT_TOKEN", "TELEGRAM_CHAT_ID",
			"SMTP_HOST", "SMTP_PORT", "SMTP_USERNAME", "SMTP_PASSWORD", "SMTP_FROM_ADDRESS",
			"WEBHOOK_URL", "WORKER_POOL_SIZE", "LOG_LEVEL", "LOG_FORMAT",
		}
		for _, env := range envVars {
			os.Unsetenv(env)
		}
	}()

	config := configpkg.LoadFromEnv()

	assert.Equal(t, 9090, config.Server.Port)
	assert.Equal(t, "db.example.com", config.Database.Host)
	assert.Equal(t, 5433, config.Database.Port)
	assert.Equal(t, "test_db", config.Database.Database)
	assert.Equal(t, "testuser", config.Database.User)
	assert.Equal(t, "testpass", config.Database.Password)
	assert.Equal(t, "redis.example.com:6380", config.Redis.Addr)
	assert.Equal(t, "redispass", config.Redis.Password)
	assert.Equal(t, 1, config.Redis.DB)
	assert.Equal(t, "123456:ABC-DEF", config.Alert.Telegram.BotToken)
	assert.Equal(t, "123456789", config.Alert.Telegram.ChatID)
	assert.True(t, config.Alert.Telegram.Enabled)
	assert.Equal(t, "smtp.example.com", config.Alert.Email.SMTPHost)
	assert.Equal(t, 465, config.Alert.Email.SMTPPort)
	assert.Equal(t, "user@example.com", config.Alert.Email.Username)
	assert.Equal(t, "smtppass", config.Alert.Email.Password)
	assert.Equal(t, "noreply@example.com", config.Alert.Email.FromAddress)
	assert.True(t, config.Alert.Email.Enabled)
	assert.Equal(t, "https://webhook.example.com/alert", config.Alert.Webhook.URL)
	assert.True(t, config.Alert.Webhook.Enabled)
	assert.Equal(t, 20, config.Worker.PoolSize)
	assert.Equal(t, "debug", config.Logging.Level)
	assert.Equal(t, "text", config.Logging.Format)
}

func TestLoadFromEnv_UsesDotEnvFile(t *testing.T) {
	originalDir, err := os.Getwd()
	require.NoError(t, err)

	tempDir := t.TempDir()
	require.NoError(t, os.Chdir(tempDir))
	defer func() {
		require.NoError(t, os.Chdir(originalDir))
	}()

	dotEnv := "PORT=9191\nDB_HOST=dotenv.db.local\nLOG_LEVEL=trace\n"
	require.NoError(t, os.WriteFile(".env", []byte(dotEnv), 0644))

	os.Unsetenv("PORT")
	os.Unsetenv("DB_HOST")
	os.Unsetenv("LOG_LEVEL")

	config := configpkg.LoadFromEnv()

	assert.Equal(t, 9191, config.Server.Port)
	assert.Equal(t, "dotenv.db.local", config.Database.Host)
	assert.Equal(t, "trace", config.Logging.Level)
}

// substituteEnvVars is unexported; environment substitution is validated via LoadFromFile tests.

func TestLoadFromFile_YAML(t *testing.T) {
	// Create a temporary YAML config file
	yamlContent := `
server:
  port: 9000
  host: "127.0.0.1"

database:
  host: "db.test.com"
  port: 5433
  database: "test_db"
  user: "testuser"
  password: "${DB_PASSWORD}"

redis:
  enabled: false
  addr: "redis.test.com:6379"

alert:
  telegram:
    enabled: true
    bot_token: "test_token"
    chat_id: "test_chat"

logging:
  level: "debug"
  format: "text"
`

	tmpFile, err := os.CreateTemp("", "config_test_*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(yamlContent)
	require.NoError(t, err)
	tmpFile.Close()

	// Set environment variable for substitution
	os.Setenv("DB_PASSWORD", "env_password")
	defer os.Unsetenv("DB_PASSWORD")

	config, err := configpkg.LoadFromFile(tmpFile.Name())
	require.NoError(t, err)

	assert.Equal(t, 9000, config.Server.Port)
	assert.Equal(t, "127.0.0.1", config.Server.Host)
	assert.Equal(t, "db.test.com", config.Database.Host)
	assert.Equal(t, 5433, config.Database.Port)
	assert.Equal(t, "test_db", config.Database.Database)
	assert.Equal(t, "testuser", config.Database.User)
	assert.Equal(t, "env_password", config.Database.Password) // Should be substituted
	assert.False(t, config.Redis.Enabled)
	assert.Equal(t, "redis.test.com:6379", config.Redis.Addr)
	assert.True(t, config.Alert.Telegram.Enabled)
	assert.Equal(t, "test_token", config.Alert.Telegram.BotToken)
	assert.Equal(t, "test_chat", config.Alert.Telegram.ChatID)
	assert.Equal(t, "debug", config.Logging.Level)
	assert.Equal(t, "text", config.Logging.Format)
}

func TestLoadFromFile_YAML_SubstitutesFromDotEnvFile(t *testing.T) {
	originalDir, err := os.Getwd()
	require.NoError(t, err)

	tempDir := t.TempDir()
	require.NoError(t, os.Chdir(tempDir))
	defer func() {
		require.NoError(t, os.Chdir(originalDir))
	}()

	require.NoError(t, os.WriteFile(".env", []byte("DB_PASSWORD=dotenv_password\n"), 0644))

	yamlContent := `
database:
  password: "${DB_PASSWORD}"
`

	tmpFile, err := os.CreateTemp(tempDir, "config_test_*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(yamlContent)
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())

	os.Unsetenv("DB_PASSWORD")

	config, err := configpkg.LoadFromFile(tmpFile.Name())
	require.NoError(t, err)

	assert.Equal(t, "dotenv_password", config.Database.Password)
}

func TestLoadFromFile_JSON(t *testing.T) {
	// Create a temporary JSON config file
	jsonContent := `{
  "server": {
    "port": 8888,
    "host": "0.0.0.0"
  },
  "database": {
    "host": "json.db.com",
    "port": 5432,
    "database": "json_db",
    "user": "jsonuser",
    "password": "jsonpass"
  },
  "redis": {
    "enabled": true,
    "addr": "json.redis.com:6379"
  },
  "logging": {
    "level": "warn",
    "format": "json"
  }
}`

	tmpFile, err := os.CreateTemp("", "config_test_*.json")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(jsonContent)
	require.NoError(t, err)
	tmpFile.Close()

	config, err := configpkg.LoadFromFile(tmpFile.Name())
	require.NoError(t, err)

	assert.Equal(t, 8888, config.Server.Port)
	assert.Equal(t, "0.0.0.0", config.Server.Host)
	assert.Equal(t, "json.db.com", config.Database.Host)
	assert.Equal(t, "json_db", config.Database.Database)
	assert.Equal(t, "jsonuser", config.Database.User)
	assert.Equal(t, "jsonpass", config.Database.Password)
	assert.True(t, config.Redis.Enabled)
	assert.Equal(t, "json.redis.com:6379", config.Redis.Addr)
	assert.Equal(t, "warn", config.Logging.Level)
	assert.Equal(t, "json", config.Logging.Format)
}

func TestLoadFromFile_InvalidFile(t *testing.T) {
	_, err := configpkg.LoadFromFile("nonexistent.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read config file")
}

func TestLoadFromFile_InvalidYAML(t *testing.T) {
	// Create a temporary file with invalid YAML
	invalidYAML := `
server:
  port: invalid_port
  host: [unclosed array
`

	tmpFile, err := os.CreateTemp("", "invalid_config_*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(invalidYAML)
	require.NoError(t, err)
	tmpFile.Close()

	_, err = configpkg.LoadFromFile(tmpFile.Name())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse YAML config file")
}

func TestCreateSampleConfig(t *testing.T) {
	// Test YAML sample config
	yamlFile, err := os.CreateTemp("", "sample_config_*.yaml")
	require.NoError(t, err)
	defer os.Remove(yamlFile.Name())
	yamlFile.Close()

	err = configpkg.CreateSampleConfig(yamlFile.Name())
	require.NoError(t, err)

	// Verify the file was created and contains expected content
	content, err := os.ReadFile(yamlFile.Name())
	require.NoError(t, err)
	assert.Contains(t, string(content), "# Uptime Monitoring System Configuration")
	assert.Contains(t, string(content), "${TELEGRAM_BOT_TOKEN}")
	assert.Contains(t, string(content), "${DB_PASSWORD}")

	// Test JSON sample config
	jsonFile, err := os.CreateTemp("", "sample_config_*.json")
	require.NoError(t, err)
	defer os.Remove(jsonFile.Name())
	jsonFile.Close()

	err = configpkg.CreateSampleConfig(jsonFile.Name())
	require.NoError(t, err)

	// Verify the file was created and contains expected content
	content, err = os.ReadFile(jsonFile.Name())
	require.NoError(t, err)
	assert.Contains(t, string(content), `"server"`)
	assert.Contains(t, string(content), `"database"`)
}

func TestValidateBasicConfig(t *testing.T) {
	ctx := context.Background()

	// Test valid config
	config := configpkg.DefaultConfig()
	err := config.Validate(ctx)
	assert.NoError(t, err)

	// Test invalid server port
	config.Server.Port = 0
	err = config.Validate(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "server.port")

	// Reset and test invalid database config
	config = configpkg.DefaultConfig()
	config.Database.Host = ""
	err = config.Validate(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database.host")

	// Test invalid SSL mode
	config = configpkg.DefaultConfig()
	config.Database.SSLMode = "invalid"
	err = config.Validate(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database.ssl_mode")

	// Test invalid logging level
	config = configpkg.DefaultConfig()
	config.Logging.Level = "invalid"
	err = config.Validate(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "logging.level")
}

func TestValidateAlertConfig(t *testing.T) {
	ctx := context.Background()

	// Test Telegram validation
	config := configpkg.DefaultConfig()
	config.Alert.Telegram.Enabled = true
	config.Alert.Telegram.BotToken = ""
	err := config.Validate(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "alert.telegram.bot_token")

	config = configpkg.DefaultConfig()
	config.Alert.Telegram.Enabled = true
	config.Alert.Telegram.BotToken = "valid_token"
	config.Alert.Telegram.ChatID = ""
	err = config.Validate(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "alert.telegram.chat_id")

	// Test Email validation
	config = configpkg.DefaultConfig()
	config.Alert.Email.Enabled = true
	config.Alert.Email.SMTPHost = ""
	err = config.Validate(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "alert.email.smtp_host")

	config = configpkg.DefaultConfig()
	config.Alert.Email.Enabled = true
	config.Alert.Email.SMTPHost = "smtp.example.com"
	config.Alert.Email.FromAddress = "invalid-email"
	err = config.Validate(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "alert.email.from_address")

	// Test Webhook validation
	config = configpkg.DefaultConfig()
	config.Alert.Webhook.Enabled = true
	config.Alert.Webhook.URL = ""
	err = config.Validate(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "alert.webhook.url")

	config = configpkg.DefaultConfig()
	config.Alert.Webhook.Enabled = true
	config.Alert.Webhook.URL = "invalid-url"
	err = config.Validate(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "alert.webhook.url")

	// Test valid webhook URL
	config = configpkg.DefaultConfig()
	config.Alert.Webhook.Enabled = true
	config.Alert.Webhook.URL = "https://example.com/webhook"
	err = config.Validate(ctx)
	assert.NoError(t, err)
}

// mergeConfigsSelectively is unexported; merging is validated via LoadAndValidateConfig tests.
