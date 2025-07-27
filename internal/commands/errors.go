package commands

import (
	"fmt"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// ErrorType represents different types of errors that can occur
type ErrorType string

const (
	ErrorTypeRepository     ErrorType = "repository"
	ErrorTypeConflict       ErrorType = "conflict"
	ErrorTypePayload        ErrorType = "payload"
	ErrorTypeAPI            ErrorType = "api"
	ErrorTypeFileSystem     ErrorType = "filesystem"
	ErrorTypeValidation     ErrorType = "validation"
	ErrorTypeAuthentication ErrorType = "authentication"
	ErrorTypeNetwork        ErrorType = "network"
	ErrorTypeConfiguration  ErrorType = "configuration"
)

// SyncwrightError represents a structured error with context
type SyncwrightError struct {
	Type        ErrorType `json:"type"`
	Message     string    `json:"message"`
	Details     string    `json:"details,omitempty"`
	FilePath    string    `json:"file_path,omitempty"`
	LineNumber  int       `json:"line_number,omitempty"`
	Suggestions []string  `json:"suggestions,omitempty"`
	Recoverable bool      `json:"recoverable"`
}

// Error implements the error interface
func (e *SyncwrightError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Type, e.Message, e.Details)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// NewRepositoryError creates a new repository-related error
func NewRepositoryError(message, details string, suggestions []string) *SyncwrightError {
	return &SyncwrightError{
		Type:        ErrorTypeRepository,
		Message:     message,
		Details:     details,
		Suggestions: suggestions,
		Recoverable: true,
	}
}

// NewConflictError creates a new conflict-related error
func NewConflictError(message, details, filePath string, lineNumber int) *SyncwrightError {
	return &SyncwrightError{
		Type:        ErrorTypeConflict,
		Message:     message,
		Details:     details,
		FilePath:    filePath,
		LineNumber:  lineNumber,
		Recoverable: true,
	}
}

// NewPayloadError creates a new payload-related error
func NewPayloadError(message, details string, recoverable bool) *SyncwrightError {
	return &SyncwrightError{
		Type:        ErrorTypePayload,
		Message:     message,
		Details:     details,
		Recoverable: recoverable,
	}
}

// NewAPIError creates a new API-related error
func NewAPIError(message, details string, statusCode int) *SyncwrightError {
	suggestions := []string{}

	switch statusCode {
	case 401:
		suggestions = append(suggestions, "Check your API key")
		suggestions = append(suggestions, "Verify the API key has proper permissions")
	case 403:
		suggestions = append(suggestions, "Verify API access permissions")
		suggestions = append(suggestions, "Check if your account has access to Claude Code API")
	case 429:
		suggestions = append(suggestions, "Wait before retrying")
		suggestions = append(suggestions, "Reduce request frequency")
	case 500, 502, 503, 504:
		suggestions = append(suggestions, "Retry the request")
		suggestions = append(suggestions, "Check API status page")
	}

	return &SyncwrightError{
		Type:        ErrorTypeAPI,
		Message:     message,
		Details:     details,
		Suggestions: suggestions,
		Recoverable: statusCode >= 500, // Server errors are typically recoverable
	}
}

// NewFileSystemError creates a new filesystem-related error
func NewFileSystemError(message, details, filePath string) *SyncwrightError {
	suggestions := []string{}

	if strings.Contains(strings.ToLower(message), "permission") {
		suggestions = append(suggestions, "Check file permissions")
		suggestions = append(suggestions, "Ensure you have write access to the repository")
	}
	if strings.Contains(strings.ToLower(message), "not found") {
		suggestions = append(suggestions, "Verify the file path exists")
		suggestions = append(suggestions, "Check if the file was moved or deleted")
	}

	return &SyncwrightError{
		Type:        ErrorTypeFileSystem,
		Message:     message,
		Details:     details,
		FilePath:    filePath,
		Suggestions: suggestions,
		Recoverable: true,
	}
}

// NewValidationError creates a new validation-related error
func NewValidationError(message, details string) *SyncwrightError {
	return &SyncwrightError{
		Type:        ErrorTypeValidation,
		Message:     message,
		Details:     details,
		Recoverable: false,
	}
}

// NewAuthenticationError creates a new authentication-related error
func NewAuthenticationError(message, details string) *SyncwrightError {
	suggestions := []string{
		"Set CLAUDE_API_KEY environment variable",
		"Use --api-key flag to provide API key",
		"Verify the API key is valid and active",
	}

	return &SyncwrightError{
		Type:        ErrorTypeAuthentication,
		Message:     message,
		Details:     details,
		Suggestions: suggestions,
		Recoverable: true,
	}
}

// NewNetworkError creates a new network-related error
func NewNetworkError(message, details string) *SyncwrightError {
	suggestions := []string{
		"Check your internet connection",
		"Verify firewall settings",
		"Try again in a few moments",
		"Check if the API endpoint is accessible",
	}

	return &SyncwrightError{
		Type:        ErrorTypeNetwork,
		Message:     message,
		Details:     details,
		Suggestions: suggestions,
		Recoverable: true,
	}
}

// NewConfigurationError creates a new configuration-related error
func NewConfigurationError(message, details string) *SyncwrightError {
	return &SyncwrightError{
		Type:        ErrorTypeConfiguration,
		Message:     message,
		Details:     details,
		Recoverable: true,
	}
}

// ErrorHandler provides centralized error handling and recovery
type ErrorHandler struct {
	verbose bool
}

// NewErrorHandler creates a new error handler
func NewErrorHandler(verbose bool) *ErrorHandler {
	return &ErrorHandler{verbose: verbose}
}

// Handle processes an error and returns a user-friendly message
func (eh *ErrorHandler) Handle(err error) string {
	if err == nil {
		return ""
	}

	// Check if it's a SyncwrightError
	if syncErr, ok := err.(*SyncwrightError); ok {
		return eh.formatSyncwrightError(syncErr)
	}

	// Handle standard errors
	return eh.formatStandardError(err)
}

// formatSyncwrightError formats a SyncwrightError for display
func (eh *ErrorHandler) formatSyncwrightError(err *SyncwrightError) string {
	var builder strings.Builder

	// Error header
	titleCaser := cases.Title(language.English)
	builder.WriteString(fmt.Sprintf("❌ %s Error: %s\n",
		titleCaser.String(string(err.Type)), err.Message))

	// Details
	if err.Details != "" {
		builder.WriteString(fmt.Sprintf("   Details: %s\n", err.Details))
	}

	// File context
	if err.FilePath != "" {
		if err.LineNumber > 0 {
			builder.WriteString(fmt.Sprintf("   File: %s:%d\n", err.FilePath, err.LineNumber))
		} else {
			builder.WriteString(fmt.Sprintf("   File: %s\n", err.FilePath))
		}
	}

	// Suggestions
	if len(err.Suggestions) > 0 {
		builder.WriteString("\n💡 Suggestions:\n")
		for _, suggestion := range err.Suggestions {
			builder.WriteString(fmt.Sprintf("   • %s\n", suggestion))
		}
	}

	// Recovery info
	if err.Recoverable {
		builder.WriteString("\n🔄 This error may be recoverable. Please try the suggestions above.\n")
	}

	return builder.String()
}

// formatStandardError formats a standard error for display
func (eh *ErrorHandler) formatStandardError(err error) string {
	message := err.Error()

	// Try to classify the error and provide suggestions
	suggestions := []string{}

	if strings.Contains(strings.ToLower(message), "permission denied") {
		suggestions = append(suggestions, "Check file permissions")
		suggestions = append(suggestions, "Run with appropriate privileges")
	}

	if strings.Contains(strings.ToLower(message), "network") ||
		strings.Contains(strings.ToLower(message), "connection") {
		suggestions = append(suggestions, "Check internet connection")
		suggestions = append(suggestions, "Verify network settings")
	}

	if strings.Contains(strings.ToLower(message), "not found") {
		suggestions = append(suggestions, "Verify the path exists")
		suggestions = append(suggestions, "Check spelling and case sensitivity")
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("❌ Error: %s\n", message))

	if len(suggestions) > 0 {
		builder.WriteString("\n💡 Suggestions:\n")
		for _, suggestion := range suggestions {
			builder.WriteString(fmt.Sprintf("   • %s\n", suggestion))
		}
	}

	return builder.String()
}

// WrapError wraps a standard error as a SyncwrightError
func WrapError(err error, errorType ErrorType, message string) *SyncwrightError {
	return &SyncwrightError{
		Type:        errorType,
		Message:     message,
		Details:     err.Error(),
		Recoverable: true,
	}
}

// IsRecoverable checks if an error is recoverable
func IsRecoverable(err error) bool {
	if syncErr, ok := err.(*SyncwrightError); ok {
		return syncErr.Recoverable
	}
	return true // Assume standard errors are recoverable
}

// GetErrorType returns the error type if it's a SyncwrightError
func GetErrorType(err error) ErrorType {
	if syncErr, ok := err.(*SyncwrightError); ok {
		return syncErr.Type
	}
	return ErrorType("unknown")
}

// ShouldRetry determines if an operation should be retried based on the error
func ShouldRetry(err error, attempt int, maxAttempts int) bool {
	if attempt >= maxAttempts {
		return false
	}

	if syncErr, ok := err.(*SyncwrightError); ok {
		switch syncErr.Type {
		case ErrorTypeNetwork, ErrorTypeAPI:
			return syncErr.Recoverable
		case ErrorTypeAuthentication, ErrorTypeValidation:
			return false
		default:
			return syncErr.Recoverable
		}
	}

	// For standard errors, retry on network/temporary issues
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "timeout") ||
		strings.Contains(message, "connection") ||
		strings.Contains(message, "temporary") {
		return true
	}

	return false
}
