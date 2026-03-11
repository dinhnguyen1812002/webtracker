package logger

import (
	"context"
	"log/slog"
	"os"
	"time"
)

// Logger wraps slog.Logger with additional context and methods
type Logger struct {
	*slog.Logger
}

// LogLevel represents the severity of a log entry
type LogLevel string

const (
	LevelDebug LogLevel = "debug"
	LevelInfo  LogLevel = "info"
	LevelWarn  LogLevel = "warn"
	LevelError LogLevel = "error"
)

// Fields represents structured log fields
type Fields map[string]interface{}

// New creates a new structured logger
func New(level LogLevel) *Logger {
	var slogLevel slog.Level
	switch level {
	case LevelDebug:
		slogLevel = slog.LevelDebug
	case LevelInfo:
		slogLevel = slog.LevelInfo
	case LevelWarn:
		slogLevel = slog.LevelWarn
	case LevelError:
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: slogLevel,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Add timestamp in ISO format
			if a.Key == slog.TimeKey {
				return slog.Attr{
					Key:   "timestamp",
					Value: slog.StringValue(a.Value.Time().Format(time.RFC3339)),
				}
			}
			// Rename level to severity
			if a.Key == slog.LevelKey {
				return slog.Attr{
					Key:   "severity",
					Value: a.Value,
				}
			}
			return a
		},
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)
	return &Logger{
		Logger: slog.New(handler),
	}
}

// WithFields adds structured fields to the logger
func (l *Logger) WithFields(fields Fields) *Logger {
	args := make([]interface{}, 0, len(fields)*2)
	for k, v := range fields {
		args = append(args, k, v)
	}
	return &Logger{
		Logger: l.Logger.With(args...),
	}
}

// WithContext adds context information to the logger
func (l *Logger) WithContext(ctx context.Context) *Logger {
	// Extract common context values if they exist
	fields := Fields{}

	if requestID := ctx.Value("request_id"); requestID != nil {
		fields["request_id"] = requestID
	}

	if monitorID := ctx.Value("monitor_id"); monitorID != nil {
		fields["monitor_id"] = monitorID
	}

	if userID := ctx.Value("user_id"); userID != nil {
		fields["user_id"] = userID
	}

	if len(fields) > 0 {
		return l.WithFields(fields)
	}

	return l
}

// Debug logs a debug message with optional fields
func (l *Logger) Debug(msg string, fields ...Fields) {
	l.logWithFields(slog.LevelDebug, msg, fields...)
}

// Info logs an info message with optional fields
func (l *Logger) Info(msg string, fields ...Fields) {
	l.logWithFields(slog.LevelInfo, msg, fields...)
}

// Warn logs a warning message with optional fields
func (l *Logger) Warn(msg string, fields ...Fields) {
	l.logWithFields(slog.LevelWarn, msg, fields...)
}

// Error logs an error message with optional fields
func (l *Logger) Error(msg string, fields ...Fields) {
	l.logWithFields(slog.LevelError, msg, fields...)
}

// ErrorWithErr logs an error message with an error and optional fields
func (l *Logger) ErrorWithErr(msg string, err error, fields ...Fields) {
	allFields := Fields{"error": err.Error()}
	for _, f := range fields {
		for k, v := range f {
			allFields[k] = v
		}
	}
	l.logWithFields(slog.LevelError, msg, allFields)
}

// logWithFields is a helper method to log with structured fields
func (l *Logger) logWithFields(level slog.Level, msg string, fields ...Fields) {
	if len(fields) == 0 {
		l.Logger.Log(context.Background(), level, msg)
		return
	}

	// Merge all fields
	allFields := Fields{}
	for _, f := range fields {
		for k, v := range f {
			allFields[k] = v
		}
	}

	// Convert to slog attributes
	args := make([]interface{}, 0, len(allFields)*2)
	for k, v := range allFields {
		args = append(args, k, v)
	}

	l.Logger.Log(context.Background(), level, msg, args...)
}

// Global logger instance
var defaultLogger *Logger

// Init initializes the global logger
func Init(level LogLevel) {
	defaultLogger = New(level)
}

// GetLogger returns the global logger instance
func GetLogger() *Logger {
	if defaultLogger == nil {
		defaultLogger = New(LevelInfo)
	}
	return defaultLogger
}

// Global convenience functions
func Debug(msg string, fields ...Fields) {
	GetLogger().Debug(msg, fields...)
}

func Info(msg string, fields ...Fields) {
	GetLogger().Info(msg, fields...)
}

func Warn(msg string, fields ...Fields) {
	GetLogger().Warn(msg, fields...)
}

func Error(msg string, fields ...Fields) {
	GetLogger().Error(msg, fields...)
}

func ErrorWithErr(msg string, err error, fields ...Fields) {
	GetLogger().ErrorWithErr(msg, err, fields...)
}
