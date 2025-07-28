package logging

import (
	"os"
	"regexp"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	// Logger is the global logger instance
	Logger *StructuredLogger

	// tokenPatterns defines regex patterns for sanitizing sensitive data
	tokenPatterns = []*regexp.Regexp{
		regexp.MustCompile(`sk-ant-[a-zA-Z0-9-_]+`),           // Claude tokens
		regexp.MustCompile(`ghp_[a-zA-Z0-9]{30,40}`),          // GitHub tokens (flexible length)
		regexp.MustCompile(`gho_[a-zA-Z0-9]{30,40}`),          // GitHub OAuth tokens
		regexp.MustCompile(`Bearer\s+[a-zA-Z0-9-_\.]+`),       // Bearer tokens
		regexp.MustCompile(`(?i)password\s*[:=]\s*[^\s]+`),    // Passwords
		regexp.MustCompile(`(?i)api[_-]?key\s*[:=]\s*[^\s]+`), // API keys
		regexp.MustCompile(`(?i)secret\s*[:=]\s*[^\s,]+`),     // Secrets
	}
)

// StructuredLogger wraps zap.Logger with Syncwright-specific functionality
type StructuredLogger struct {
	*zap.Logger
	config LogConfig
}

// LogConfig contains logging configuration options
type LogConfig struct {
	Level       zapcore.Level
	Development bool
	OutputPaths []string
	Verbose     bool
}

// InitializeLogger initializes the global logger with the provided configuration
func InitializeLogger(config LogConfig) error {
	zapConfig := zap.Config{
		Level:       zap.NewAtomicLevelAt(config.Level),
		Development: config.Development,
		Encoding:    "json",
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "timestamp",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			FunctionKey:    zapcore.OmitKey,
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      config.OutputPaths,
		ErrorOutputPaths: []string{"stderr"},
	}

	// Use console encoder for development or verbose mode
	if config.Development || config.Verbose {
		zapConfig.Encoding = "console"
		zapConfig.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		zapConfig.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("15:04:05")
	}

	logger, err := zapConfig.Build()
	if err != nil {
		return err
	}

	Logger = &StructuredLogger{
		Logger: logger,
		config: config,
	}

	return nil
}

// Sanitize removes sensitive information from log messages
func (l *StructuredLogger) Sanitize(message string) string {
	sanitized := message
	for _, pattern := range tokenPatterns {
		sanitized = pattern.ReplaceAllString(sanitized, "[REDACTED]")
	}
	return sanitized
}

// InfoSafe logs info with automatic sanitization
func (l *StructuredLogger) InfoSafe(msg string, fields ...zap.Field) {
	l.Logger.Info(l.Sanitize(msg), l.sanitizeFields(fields...)...)
}

// ErrorSafe logs error with automatic sanitization
func (l *StructuredLogger) ErrorSafe(msg string, fields ...zap.Field) {
	l.Logger.Error(l.Sanitize(msg), l.sanitizeFields(fields...)...)
}

// DebugSafe logs debug with automatic sanitization
func (l *StructuredLogger) DebugSafe(msg string, fields ...zap.Field) {
	if l.config.Verbose {
		l.Logger.Debug(l.Sanitize(msg), l.sanitizeFields(fields...)...)
	}
}

// WarnSafe logs warning with automatic sanitization
func (l *StructuredLogger) WarnSafe(msg string, fields ...zap.Field) {
	l.Logger.Warn(l.Sanitize(msg), l.sanitizeFields(fields...)...)
}

// Performance logs performance metrics
func (l *StructuredLogger) Performance(operation string, duration time.Duration, fields ...zap.Field) {
	perfFields := append([]zap.Field{
		zap.String("operation", operation),
		zap.Duration("duration", duration),
		zap.String("category", "performance"),
	}, fields...)

	l.Logger.Info("Performance metric", perfFields...)
}

// SecurityEvent logs security-related events
func (l *StructuredLogger) SecurityEvent(event string, fields ...zap.Field) {
	securityFields := append([]zap.Field{
		zap.String("event_type", "security"),
		zap.String("event", event),
		zap.Time("timestamp", time.Now()),
	}, fields...)

	l.Logger.Warn("Security event detected", securityFields...)
}

// ConflictResolution logs conflict resolution events
func (l *StructuredLogger) ConflictResolution(event string, fields ...zap.Field) {
	conflictFields := append([]zap.Field{
		zap.String("event_type", "conflict_resolution"),
		zap.String("event", event),
	}, fields...)

	l.Logger.Info("Conflict resolution event", conflictFields...)
}

// Pipeline logs pipeline stage events
func (l *StructuredLogger) Pipeline(stage string, status string, fields ...zap.Field) {
	pipelineFields := append([]zap.Field{
		zap.String("event_type", "pipeline"),
		zap.String("stage", stage),
		zap.String("status", status),
	}, fields...)

	level := zapcore.InfoLevel
	if status == "error" || status == "failed" {
		level = zapcore.ErrorLevel
	} else if status == "warning" {
		level = zapcore.WarnLevel
	}

	l.Logger.Log(level, "Pipeline stage event", pipelineFields...)
}

// sanitizeFields sanitizes zap fields to remove sensitive data
func (l *StructuredLogger) sanitizeFields(fields ...zap.Field) []zap.Field {
	sanitized := make([]zap.Field, len(fields))
	for i, field := range fields {
		switch field.Type {
		case zapcore.StringType:
			sanitized[i] = zap.String(field.Key, l.Sanitize(field.String))
		default:
			sanitized[i] = field
		}
	}
	return sanitized
}

// GetDefaultConfig returns default logging configuration based on environment
func GetDefaultConfig() LogConfig {
	level := zapcore.InfoLevel
	development := false
	verbose := false

	// Check environment variables
	if os.Getenv("SYNCWRIGHT_DEBUG") == "true" {
		level = zapcore.DebugLevel
		development = true
		verbose = true
	}

	return LogConfig{
		Level:       level,
		Development: development,
		OutputPaths: []string{"stdout"},
		Verbose:     verbose,
	}
}

// MustInitialize initializes the logger and panics on error (for use in main)
func MustInitialize(config LogConfig) {
	if err := InitializeLogger(config); err != nil {
		panic("Failed to initialize logger: " + err.Error())
	}
}

// Sync flushes any buffered log entries (call before program exit)
func Sync() {
	if Logger != nil {
		Logger.Sync()
	}
}
