package tests

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	configpkg "web-tracker/config"
)

func TestConfigurationIntegration(t *testing.T) {
	ctx := context.Background()

	// Test loading configuration without connectivity validation
	config, err := configpkg.LoadAndValidateConfig(ctx, "")
	require.NoError(t, err)
	assert.NotNil(t, config)

	// Verify default values
	assert.Equal(t, 8080, config.Server.Port)
	assert.Equal(t, "localhost", config.Database.Host)
	assert.Equal(t, 5432, config.Database.Port)
	assert.Equal(t, "uptime_monitor", config.Database.Database)
	assert.Equal(t, "postgres", config.Database.User)
	assert.True(t, config.Redis.Enabled)
	assert.Equal(t, "localhost:6379", config.Redis.Addr)
}

func TestConfigurationWithEnvOverrides(t *testing.T) {
	ctx := context.Background()

	// Set environment variables
	os.Setenv("PORT", "9090")
	os.Setenv("DB_HOST", "test.db.com")
	os.Setenv("DB_PORT", "5433")
	os.Setenv("REDIS_ENABLED", "false")
	os.Setenv("LOG_LEVEL", "debug")

	defer func() {
		// Clean up
		os.Unsetenv("PORT")
		os.Unsetenv("DB_HOST")
		os.Unsetenv("DB_PORT")
		os.Unsetenv("REDIS_ENABLED")
		os.Unsetenv("LOG_LEVEL")
	}()

	config, err := configpkg.LoadAndValidateConfig(ctx, "")
	require.NoError(t, err)

	// Verify environment overrides
	assert.Equal(t, 9090, config.Server.Port)
	assert.Equal(t, "test.db.com", config.Database.Host)
	assert.Equal(t, 5433, config.Database.Port)
	assert.False(t, config.Redis.Enabled)
	assert.Equal(t, "debug", config.Logging.Level)
}

func TestConfigurationFromFile(t *testing.T) {
	ctx := context.Background()

	// Create a temporary config file
	configContent := `
server:
  port: 7777
  host: "127.0.0.1"

database:
  host: "file.db.com"
  port: 5434
  database: "file_db"
  user: "fileuser"
  password: "filepass"

redis:
  enabled: false

logging:
  level: "warn"
  format: "text"
`

	tmpFile, err := os.CreateTemp("", "integration_config_*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(configContent)
	require.NoError(t, err)
	tmpFile.Close()

	config, err := configpkg.LoadAndValidateConfig(ctx, tmpFile.Name())
	require.NoError(t, err)

	// Verify file values
	assert.Equal(t, 7777, config.Server.Port)
	assert.Equal(t, "127.0.0.1", config.Server.Host)
	assert.Equal(t, "file.db.com", config.Database.Host)
	assert.Equal(t, 5434, config.Database.Port)
	assert.Equal(t, "file_db", config.Database.Database)
	assert.Equal(t, "fileuser", config.Database.User)
	assert.Equal(t, "filepass", config.Database.Password)
	assert.False(t, config.Redis.Enabled)
	assert.Equal(t, "warn", config.Logging.Level)
	assert.Equal(t, "text", config.Logging.Format)
}

func TestConfigurationFileWithEnvSubstitution(t *testing.T) {
	ctx := context.Background()

	// Set environment variables for substitution
	os.Setenv("TEST_DB_PASSWORD", "secret123")
	os.Setenv("TEST_REDIS_ADDR", "redis.test.com:6380")

	defer func() {
		os.Unsetenv("TEST_DB_PASSWORD")
		os.Unsetenv("TEST_REDIS_ADDR")
	}()

	// Create config file with environment variable substitution
	configContent := `
database:
  password: "${TEST_DB_PASSWORD}"

redis:
  addr: "${TEST_REDIS_ADDR}"
`

	tmpFile, err := os.CreateTemp("", "env_subst_config_*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(configContent)
	require.NoError(t, err)
	tmpFile.Close()

	config, err := configpkg.LoadAndValidateConfig(ctx, tmpFile.Name())
	require.NoError(t, err)

	// Verify environment variable substitution
	assert.Equal(t, "secret123", config.Database.Password)
	assert.Equal(t, "redis.test.com:6380", config.Redis.Addr)
}

func TestInvalidConfiguration(t *testing.T) {
	ctx := context.Background()

	// Create config with invalid values
	configContent := `
server:
  port: 0  # Invalid port

database:
  host: ""  # Empty host
  ssl_mode: "invalid"  # Invalid SSL mode

alert:
  telegram:
    enabled: true
    bot_token: ""  # Empty token when enabled

logging:
  level: "invalid"  # Invalid log level
`

	tmpFile, err := os.CreateTemp("", "invalid_config_*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(configContent)
	require.NoError(t, err)
	tmpFile.Close()

	_, err = configpkg.LoadAndValidateConfig(ctx, tmpFile.Name())
	assert.Error(t, err)

	// Verify that multiple validation errors are reported
	assert.Contains(t, err.Error(), "server.port")
	assert.Contains(t, err.Error(), "database.host")
	assert.Contains(t, err.Error(), "database.ssl_mode")
	assert.Contains(t, err.Error(), "alert.telegram.bot_token")
	assert.Contains(t, err.Error(), "logging.level")
}
