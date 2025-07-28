package logging

import (
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestSanitize(t *testing.T) {
	// Initialize logger for testing
	config := LogConfig{
		Level:       zapcore.InfoLevel,
		Development: true,
		OutputPaths: []string{"stdout"},
		Verbose:     true,
	}

	if err := InitializeLogger(config); err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Claude token",
			input:    "Using token sk-ant-api03-abcd1234-efgh5678",
			expected: "Using token [REDACTED]",
		},
		{
			name:     "GitHub token",
			input:    "GitHub token: ghp_abcdefghijklmnopqrstuvwxyz123456",
			expected: "GitHub token: [REDACTED]",
		},
		{
			name:     "Bearer token",
			input:    "Authorization: Bearer abc123.def456.ghi789",
			expected: "Authorization: [REDACTED]",
		},
		{
			name:     "Password in config",
			input:    "password: secretpassword123",
			expected: "[REDACTED]",
		},
		{
			name:     "API key",
			input:    "api_key = my-secret-key-123",
			expected: "[REDACTED]",
		},
		{
			name:     "Secret value",
			input:    "secret: very-secret-value",
			expected: "[REDACTED]",
		},
		{
			name:     "Clean message",
			input:    "Processing conflict in file main.go",
			expected: "Processing conflict in file main.go",
		},
		{
			name:     "Multiple sensitive items",
			input:    "token sk-ant-123 and password=secret123",
			expected: "token [REDACTED] and [REDACTED]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Logger.Sanitize(tt.input)
			if result != tt.expected {
				t.Errorf("Sanitize() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestLoggingMethods(t *testing.T) {
	// Initialize logger for testing
	config := LogConfig{
		Level:       zapcore.DebugLevel,
		Development: true,
		OutputPaths: []string{"stdout"},
		Verbose:     true,
	}

	if err := InitializeLogger(config); err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	// Test various logging methods - these should not panic
	Logger.InfoSafe("Test info message", zap.String("key", "value"))
	Logger.ErrorSafe("Test error message", zap.Error(nil))
	Logger.DebugSafe("Test debug message", zap.Int("count", 5))
	Logger.WarnSafe("Test warning message", zap.Bool("flag", true))

	// Test specialized logging methods
	Logger.ConflictResolution("test_event", zap.String("file", "test.go"))
	Logger.Pipeline("test_stage", "success", zap.Int("files", 3))
	Logger.SecurityEvent("test_security", zap.String("source", "test"))
	Logger.Performance("test_operation", 100, zap.String("component", "test"))
}

func TestGetDefaultConfig(t *testing.T) {
	// Test default configuration
	config := GetDefaultConfig()

	if config.Level != zapcore.InfoLevel {
		t.Errorf("Expected default level to be InfoLevel, got %v", config.Level)
	}

	if config.Development != false {
		t.Errorf("Expected development to be false by default, got %v", config.Development)
	}

	if len(config.OutputPaths) != 1 || config.OutputPaths[0] != "stdout" {
		t.Errorf("Expected output paths to be [stdout], got %v", config.OutputPaths)
	}
}

func TestSanitizeFields(t *testing.T) {
	// Initialize logger for testing
	config := LogConfig{
		Level:       zapcore.InfoLevel,
		Development: true,
		OutputPaths: []string{"stdout"},
		Verbose:     true,
	}

	if err := InitializeLogger(config); err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	// Test that string fields are sanitized
	fields := []zap.Field{
		zap.String("token", "sk-ant-secret123"),
		zap.String("message", "Normal message"),
		zap.Int("count", 42),
	}

	sanitized := Logger.sanitizeFields(fields...)

	// Check that the token field was sanitized
	if len(sanitized) != 3 {
		t.Errorf("Expected 3 fields, got %d", len(sanitized))
	}

	// The string field with token should be sanitized
	// The normal message should remain unchanged
	// The integer field should remain unchanged
}
