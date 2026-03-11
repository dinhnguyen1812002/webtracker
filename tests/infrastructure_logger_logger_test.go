package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	logger "web-tracker/infrastructure/logger"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name     string
		level    logger.LogLevel
		expected slog.Level
	}{
		{"debug level", logger.LevelDebug, slog.LevelDebug},
		{"info level", logger.LevelInfo, slog.LevelInfo},
		{"warn level", logger.LevelWarn, slog.LevelWarn},
		{"error level", logger.LevelError, slog.LevelError},
		{"invalid level defaults to info", logger.LogLevel("invalid"), slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := logger.New(tt.level)
			assert.NotNil(t, log)
			assert.NotNil(t, log.Logger)
		})
	}
}

func TestLoggerWithFields(t *testing.T) {
	// Capture output
	var buf bytes.Buffer

	// Create logger with custom handler for testing
	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	handler := slog.NewJSONHandler(&buf, opts)
	log := &logger.Logger{
		Logger: slog.New(handler),
	}

	fields := logger.Fields{
		"monitor_id": "test-monitor",
		"url":        "https://example.com",
		"status":     "success",
	}

	loggerWithFields := log.WithFields(fields)
	loggerWithFields.Info("Health check completed")

	// Parse the JSON output
	var logEntry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)

	assert.Equal(t, "Health check completed", logEntry["msg"])
	assert.Equal(t, "test-monitor", logEntry["monitor_id"])
	assert.Equal(t, "https://example.com", logEntry["url"])
	assert.Equal(t, "success", logEntry["status"])
}

func TestLoggerWithContext(t *testing.T) {
	log := logger.New(logger.LevelInfo)

	// Test with context containing known keys
	ctx := context.Background()
	ctx = context.WithValue(ctx, "request_id", "req-123")
	ctx = context.WithValue(ctx, "monitor_id", "mon-456")

	loggerWithCtx := log.WithContext(ctx)
	assert.NotNil(t, loggerWithCtx)

	// Test with empty context
	emptyCtx := context.Background()
	loggerWithEmptyCtx := log.WithContext(emptyCtx)
	assert.NotNil(t, loggerWithEmptyCtx)
}

func TestLogLevels(t *testing.T) {
	var buf bytes.Buffer

	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	handler := slog.NewJSONHandler(&buf, opts)
	log := &logger.Logger{
		Logger: slog.New(handler),
	}

	tests := []struct {
		name     string
		logFunc  func(string, ...logger.Fields)
		expected string
	}{
		{"debug", log.Debug, "DEBUG"},
		{"info", log.Info, "INFO"},
		{"warn", log.Warn, "WARN"},
		{"error", log.Error, "ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			tt.logFunc("test message")

			var logEntry map[string]interface{}
			err := json.Unmarshal(buf.Bytes(), &logEntry)
			require.NoError(t, err)

			assert.Equal(t, "test message", logEntry["msg"])
			assert.Equal(t, tt.expected, logEntry["level"])
		})
	}
}

func TestErrorWithErr(t *testing.T) {
	var buf bytes.Buffer

	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	handler := slog.NewJSONHandler(&buf, opts)
	log := &logger.Logger{
		Logger: slog.New(handler),
	}

	testErr := assert.AnError
	fields := logger.Fields{"monitor_id": "test-monitor"}

	log.ErrorWithErr("Operation failed", testErr, fields)

	var logEntry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)

	assert.Equal(t, "Operation failed", logEntry["msg"])
	assert.Equal(t, "ERROR", logEntry["level"])
	assert.Equal(t, testErr.Error(), logEntry["error"])
	assert.Equal(t, "test-monitor", logEntry["monitor_id"])
}

func TestGlobalLogger(t *testing.T) {
	// Reset global logger
	// Test GetLogger creates default logger
	global := logger.GetLogger()
	assert.NotNil(t, global)

	// Test Init sets global logger
	logger.Init(logger.LevelDebug)
	logger2 := logger.GetLogger()
	assert.NotNil(t, logger2)

	// Test global convenience functions don't panic
	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")
	logger.ErrorWithErr("error with err", assert.AnError)
}

func TestJSONOutput(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	log := logger.New(logger.LevelInfo)
	log.Info("test message", logger.Fields{
		"component": "test",
		"action":    "testing",
	})

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	// Verify it's valid JSON
	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(strings.TrimSpace(output)), &logEntry)
	require.NoError(t, err)

	assert.Equal(t, "test message", logEntry["msg"])
	assert.Equal(t, "test", logEntry["component"])
	assert.Equal(t, "testing", logEntry["action"])
	assert.Contains(t, logEntry, "timestamp")
	assert.Contains(t, logEntry, "severity")
}
