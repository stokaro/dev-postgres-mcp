package mcp

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"
)

// LogLevel represents the logging level.
type LogLevel string

const (
	// LogLevelDebug enables debug logging.
	LogLevelDebug LogLevel = "debug"
	// LogLevelInfo enables info logging.
	LogLevelInfo LogLevel = "info"
	// LogLevelWarn enables warning logging.
	LogLevelWarn LogLevel = "warn"
	// LogLevelError enables error logging.
	LogLevelError LogLevel = "error"
)

// LogFormat represents the logging format.
type LogFormat string

const (
	// LogFormatText uses text format for logging.
	LogFormatText LogFormat = "text"
	// LogFormatJSON uses JSON format for logging.
	LogFormatJSON LogFormat = "json"
)

// LoggingConfig holds configuration for logging.
type LoggingConfig struct {
	Level  LogLevel
	Format LogFormat
}

// SetupLogging configures the global logger based on the provided configuration.
func SetupLogging(config LoggingConfig) {
	var level slog.Level

	switch config.Level {
	case LogLevelDebug:
		level = slog.LevelDebug
	case LogLevelInfo:
		level = slog.LevelInfo
	case LogLevelWarn:
		level = slog.LevelWarn
	case LogLevelError:
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	var handler slog.Handler

	opts := &slog.HandlerOptions{
		Level: level,
		AddSource: level == slog.LevelDebug, // Add source info for debug level
	}

	switch config.Format {
	case LogFormatJSON:
		handler = slog.NewJSONHandler(os.Stderr, opts)
	case LogFormatText:
		handler = slog.NewTextHandler(os.Stderr, opts)
	default:
		handler = slog.NewTextHandler(os.Stderr, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
}

// GetLoggingConfigFromEnv reads logging configuration from environment variables.
func GetLoggingConfigFromEnv() LoggingConfig {
	config := LoggingConfig{
		Level:  LogLevelInfo,
		Format: LogFormatText,
	}

	// Read log level from environment
	if levelStr := os.Getenv("DEV_POSTGRES_MCP_LOG_LEVEL"); levelStr != "" {
		switch strings.ToLower(levelStr) {
		case "debug":
			config.Level = LogLevelDebug
		case "info":
			config.Level = LogLevelInfo
		case "warn", "warning":
			config.Level = LogLevelWarn
		case "error":
			config.Level = LogLevelError
		}
	}

	// Read log format from environment
	if formatStr := os.Getenv("DEV_POSTGRES_MCP_LOG_FORMAT"); formatStr != "" {
		switch strings.ToLower(formatStr) {
		case "json":
			config.Format = LogFormatJSON
		case "text":
			config.Format = LogFormatText
		}
	}

	return config
}

// LoggerWithContext creates a logger with context information.
func LoggerWithContext(ctx context.Context, attrs ...slog.Attr) *slog.Logger {
	logger := slog.Default()

	// Add context values if available
	if requestID := ctx.Value("request_id"); requestID != nil {
		attrs = append(attrs, slog.String("request_id", requestID.(string)))
	}

	if len(attrs) > 0 {
		// Convert []slog.Attr to []any
		args := make([]any, len(attrs))
		for i, attr := range attrs {
			args[i] = attr
		}
		logger = logger.With(args...)
	}

	return logger
}

// LogMCPOperation logs an MCP operation with standard fields.
func LogMCPOperation(ctx context.Context, operation string, attrs ...slog.Attr) {
	logger := LoggerWithContext(ctx)
	
	baseAttrs := []slog.Attr{
		slog.String("operation", operation),
		slog.Time("timestamp", time.Now()),
	}
	
	baseAttrs = append(baseAttrs, attrs...)
	logger.LogAttrs(ctx, slog.LevelInfo, "MCP operation", baseAttrs...)
}

// LogDockerOperation logs a Docker operation with standard fields.
func LogDockerOperation(ctx context.Context, operation string, containerID string, attrs ...slog.Attr) {
	logger := LoggerWithContext(ctx)
	
	baseAttrs := []slog.Attr{
		slog.String("operation", operation),
		slog.String("container_id", containerID),
		slog.Time("timestamp", time.Now()),
	}
	
	baseAttrs = append(baseAttrs, attrs...)
	logger.LogAttrs(ctx, slog.LevelInfo, "Docker operation", baseAttrs...)
}

// LogPostgreSQLOperation logs a PostgreSQL operation with standard fields.
func LogPostgreSQLOperation(ctx context.Context, operation string, instanceID string, attrs ...slog.Attr) {
	logger := LoggerWithContext(ctx)
	
	baseAttrs := []slog.Attr{
		slog.String("operation", operation),
		slog.String("instance_id", instanceID),
		slog.Time("timestamp", time.Now()),
	}
	
	baseAttrs = append(baseAttrs, attrs...)
	logger.LogAttrs(ctx, slog.LevelInfo, "PostgreSQL operation", baseAttrs...)
}

// LogError logs an error with context and additional attributes.
func LogError(ctx context.Context, err error, message string, attrs ...slog.Attr) {
	logger := LoggerWithContext(ctx)
	
	baseAttrs := []slog.Attr{
		slog.String("error", err.Error()),
		slog.Time("timestamp", time.Now()),
	}
	
	baseAttrs = append(baseAttrs, attrs...)
	logger.LogAttrs(ctx, slog.LevelError, message, baseAttrs...)
}

// LogPerformance logs performance metrics for operations.
func LogPerformance(ctx context.Context, operation string, duration time.Duration, attrs ...slog.Attr) {
	logger := LoggerWithContext(ctx)
	
	baseAttrs := []slog.Attr{
		slog.String("operation", operation),
		slog.Duration("duration", duration),
		slog.Time("timestamp", time.Now()),
	}
	
	baseAttrs = append(baseAttrs, attrs...)
	logger.LogAttrs(ctx, slog.LevelInfo, "Performance metric", baseAttrs...)
}

// LogHealthCheck logs health check results.
func LogHealthCheck(ctx context.Context, instanceID string, status string, duration time.Duration, message string) {
	logger := LoggerWithContext(ctx)
	
	logger.LogAttrs(ctx, slog.LevelInfo, "Health check completed",
		slog.String("instance_id", instanceID),
		slog.String("status", status),
		slog.Duration("duration", duration),
		slog.String("message", message),
		slog.Time("timestamp", time.Now()),
	)
}
