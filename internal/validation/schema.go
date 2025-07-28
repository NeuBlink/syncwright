package validation

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
)

const (
	// Maximum payload sizes to prevent DoS
	MaxPayloadSize      = 10 * 1024 * 1024 // 10MB
	MaxConflictFiles    = 1000             // Maximum files per payload
	MaxConflictsPerFile = 100              // Maximum conflicts per file
	MaxLineLength       = 10000            // Maximum characters per line
	MaxContextLines     = 50               // Maximum context lines
	MaxTotalConflicts   = 5000             // Maximum total conflicts across all files
)

// PayloadValidator handles JSON payload validation
type PayloadValidator struct {
	validator *validator.Validate
}

// NewPayloadValidator creates a new payload validator
func NewPayloadValidator() *PayloadValidator {
	v := validator.New()

	// Register custom validation rules
	v.RegisterValidation("filepath", validateFilePath)
	v.RegisterValidation("language", validateLanguage)
	v.RegisterValidation("conflict_id", validateConflictID)
	v.RegisterValidation("safe_content", validateSafeContent)
	v.RegisterValidation("repo_path", validateRepoPath)

	return &PayloadValidator{validator: v}
}

// ValidatedConflictPayload represents a validated conflict payload
type ValidatedConflictPayload struct {
	Files    []ValidatedFilePayload `json:"files" validate:"required,max=1000,dive"`
	Metadata PayloadMetadata        `json:"metadata" validate:"required"`
}

// ValidatedFilePayload represents a validated file payload
type ValidatedFilePayload struct {
	Path      string                  `json:"path" validate:"required,filepath,max=500"`
	Language  string                  `json:"language" validate:"required,language"`
	Conflicts []ValidatedConflictHunk `json:"conflicts" validate:"required,max=100,dive"`
	Context   ValidatedFileContext    `json:"context" validate:"required"`
}

// ValidatedConflictHunk represents a validated conflict hunk
type ValidatedConflictHunk struct {
	ID          string   `json:"id,omitempty" validate:"omitempty,conflict_id,max=100"`
	StartLine   int      `json:"start_line" validate:"required,min=1,max=1000000"`
	EndLine     int      `json:"end_line" validate:"required,min=1,max=1000000,gtfield=StartLine"`
	OursLines   []string `json:"ours_lines" validate:"required,dive,safe_content,max=10000"`
	TheirsLines []string `json:"theirs_lines" validate:"required,dive,safe_content,max=10000"`
	BaseLines   []string `json:"base_lines,omitempty" validate:"dive,safe_content,max=10000"`
}

// ValidatedFileContext represents validated file context
type ValidatedFileContext struct {
	BeforeLines []string `json:"before_lines,omitempty" validate:"max=50,dive,safe_content,max=10000"`
	AfterLines  []string `json:"after_lines,omitempty" validate:"max=50,dive,safe_content,max=10000"`
}

// PayloadMetadata represents payload metadata
type PayloadMetadata struct {
	Timestamp      time.Time `json:"timestamp,omitempty"`
	RepoPath       string    `json:"repo_path" validate:"required,repo_path,max=1000"`
	TotalFiles     int       `json:"total_files,omitempty" validate:"omitempty,min=0,max=1000"`
	TotalConflicts int       `json:"total_conflicts,omitempty" validate:"omitempty,min=0,max=5000"`
	Version        string    `json:"version,omitempty" validate:"omitempty,max=20"`
}

// ValidationError provides detailed validation error information
type ValidationError struct {
	Field       string `json:"field"`
	Value       string `json:"value"`
	Tag         string `json:"tag"`
	Message     string `json:"message"`
	ActualValue string `json:"actual_value,omitempty"`
}

// ValidationResult contains the validation results
type ValidationResult struct {
	Valid   bool              `json:"valid"`
	Errors  []ValidationError `json:"errors,omitempty"`
	Summary ValidationSummary `json:"summary"`
}

// ValidationSummary provides validation statistics
type ValidationSummary struct {
	TotalFiles     int `json:"total_files"`
	TotalConflicts int `json:"total_conflicts"`
	PayloadSize    int `json:"payload_size_bytes"`
	MaxLineLength  int `json:"max_line_length"`
}

// ValidatePayload validates a conflict payload with comprehensive security checks
func (pv *PayloadValidator) ValidatePayload(data []byte) (*ValidatedConflictPayload, *ValidationResult, error) {
	result := &ValidationResult{
		Summary: ValidationSummary{
			PayloadSize: len(data),
		},
	}

	// Step 1: Check payload size
	if len(data) > MaxPayloadSize {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "payload",
			Tag:     "max_size",
			Message: fmt.Sprintf("Payload size %d exceeds maximum %d bytes", len(data), MaxPayloadSize),
		})
		result.Valid = false
		return nil, result, fmt.Errorf("payload size %d exceeds maximum %d bytes", len(data), MaxPayloadSize)
	}

	// Step 2: Check for empty payload
	if len(data) == 0 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "payload",
			Tag:     "required",
			Message: "Payload cannot be empty",
		})
		result.Valid = false
		return nil, result, fmt.Errorf("payload cannot be empty")
	}

	// Step 3: Validate JSON structure
	if !json.Valid(data) {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "payload",
			Tag:     "json",
			Message: "Invalid JSON format",
		})
		result.Valid = false
		return nil, result, fmt.Errorf("invalid JSON format")
	}

	// Step 4: Unmarshal with strict parsing
	var payload ValidatedConflictPayload
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields() // Strict parsing to prevent injection

	if err := decoder.Decode(&payload); err != nil {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "payload",
			Tag:     "decode",
			Message: fmt.Sprintf("JSON parsing failed: %v", err),
		})
		result.Valid = false
		return nil, result, fmt.Errorf("JSON parsing failed: %w", err)
	}

	// Step 5: Validate structure and constraints
	if err := pv.validator.Struct(&payload); err != nil {
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			for _, ve := range validationErrors {
				result.Errors = append(result.Errors, ValidationError{
					Field:       ve.Field(),
					Value:       ve.Tag(),
					Tag:         ve.Tag(),
					Message:     pv.formatValidationError(ve),
					ActualValue: fmt.Sprintf("%v", ve.Value()),
				})
			}
		}
		result.Valid = false
		return nil, result, fmt.Errorf("validation failed: %w", err)
	}

	// Step 6: Additional business logic validation
	if err := pv.validateBusinessLogic(&payload, result); err != nil {
		result.Valid = false
		return nil, result, fmt.Errorf("business logic validation failed: %w", err)
	}

	// Step 7: Calculate summary statistics
	pv.calculateSummary(&payload, result)

	result.Valid = true
	return &payload, result, nil
}

// validateBusinessLogic performs additional validation beyond struct tags
func (pv *PayloadValidator) validateBusinessLogic(payload *ValidatedConflictPayload, result *ValidationResult) error {
	totalConflicts := 0
	maxLineLength := 0

	for fileIdx, file := range payload.Files {
		totalConflicts += len(file.Conflicts)

		// Validate no duplicate file paths
		for otherIdx, otherFile := range payload.Files {
			if fileIdx != otherIdx && file.Path == otherFile.Path {
				result.Errors = append(result.Errors, ValidationError{
					Field:   fmt.Sprintf("files[%d].path", fileIdx),
					Tag:     "duplicate",
					Message: fmt.Sprintf("Duplicate file path: %s", file.Path),
				})
				return fmt.Errorf("duplicate file path: %s", file.Path)
			}
		}

		// Validate conflict hunks
		for conflictIdx, conflict := range file.Conflicts {
			// Validate line ranges
			if conflict.EndLine <= conflict.StartLine {
				result.Errors = append(result.Errors, ValidationError{
					Field:   fmt.Sprintf("files[%d].conflicts[%d]", fileIdx, conflictIdx),
					Tag:     "line_range",
					Message: fmt.Sprintf("Invalid line range in %s: end line %d <= start line %d", file.Path, conflict.EndLine, conflict.StartLine),
				})
				return fmt.Errorf("invalid line range in %s: end line %d <= start line %d", file.Path, conflict.EndLine, conflict.StartLine)
			}

			// Check for reasonable conflict size
			totalLines := len(conflict.OursLines) + len(conflict.TheirsLines) + len(conflict.BaseLines)
			if totalLines > 1000 {
				result.Errors = append(result.Errors, ValidationError{
					Field:   fmt.Sprintf("files[%d].conflicts[%d]", fileIdx, conflictIdx),
					Tag:     "conflict_size",
					Message: fmt.Sprintf("Conflict in %s is too large: %d lines", file.Path, totalLines),
				})
				return fmt.Errorf("conflict in %s is too large: %d lines", file.Path, totalLines)
			}

			// Track maximum line length
			for _, line := range conflict.OursLines {
				if len(line) > maxLineLength {
					maxLineLength = len(line)
				}
			}
			for _, line := range conflict.TheirsLines {
				if len(line) > maxLineLength {
					maxLineLength = len(line)
				}
			}
			for _, line := range conflict.BaseLines {
				if len(line) > maxLineLength {
					maxLineLength = len(line)
				}
			}

			// Validate conflict ID uniqueness within file
			if conflict.ID != "" {
				for otherIdx, otherConflict := range file.Conflicts {
					if conflictIdx != otherIdx && conflict.ID == otherConflict.ID {
						result.Errors = append(result.Errors, ValidationError{
							Field:   fmt.Sprintf("files[%d].conflicts[%d].id", fileIdx, conflictIdx),
							Tag:     "duplicate_id",
							Message: fmt.Sprintf("Duplicate conflict ID in %s: %s", file.Path, conflict.ID),
						})
						return fmt.Errorf("duplicate conflict ID in %s: %s", file.Path, conflict.ID)
					}
				}
			}
		}

		// Validate context lines
		contextLines := len(file.Context.BeforeLines) + len(file.Context.AfterLines)
		if contextLines > MaxContextLines*2 {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("files[%d].context", fileIdx),
				Tag:     "context_size",
				Message: fmt.Sprintf("Too many context lines in %s: %d", file.Path, contextLines),
			})
			return fmt.Errorf("too many context lines in %s: %d", file.Path, contextLines)
		}
	}

	// Check total conflicts across all files
	if totalConflicts > MaxTotalConflicts {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "payload",
			Tag:     "total_conflicts",
			Message: fmt.Sprintf("Too many total conflicts: %d (max: %d)", totalConflicts, MaxTotalConflicts),
		})
		return fmt.Errorf("too many total conflicts: %d", totalConflicts)
	}

	result.Summary.MaxLineLength = maxLineLength

	return nil
}

// formatValidationError formats validation errors into user-friendly messages
func (pv *PayloadValidator) formatValidationError(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return fmt.Sprintf("Field '%s' is required", fe.Field())
	case "max":
		return fmt.Sprintf("Field '%s' exceeds maximum length of %s", fe.Field(), fe.Param())
	case "min":
		return fmt.Sprintf("Field '%s' is below minimum value of %s", fe.Field(), fe.Param())
	case "filepath":
		return fmt.Sprintf("Field '%s' contains invalid file path", fe.Field())
	case "language":
		return fmt.Sprintf("Field '%s' contains unsupported language", fe.Field())
	case "conflict_id":
		return fmt.Sprintf("Field '%s' contains invalid conflict ID format", fe.Field())
	case "safe_content":
		return fmt.Sprintf("Field '%s' contains unsafe content", fe.Field())
	case "repo_path":
		return fmt.Sprintf("Field '%s' contains invalid repository path", fe.Field())
	case "gtfield":
		return fmt.Sprintf("Field '%s' must be greater than %s", fe.Field(), fe.Param())
	default:
		return fmt.Sprintf("Field '%s' failed validation: %s", fe.Field(), fe.Tag())
	}
}

// calculateSummary calculates validation summary statistics
func (pv *PayloadValidator) calculateSummary(payload *ValidatedConflictPayload, result *ValidationResult) {
	result.Summary.TotalFiles = len(payload.Files)

	totalConflicts := 0
	for _, file := range payload.Files {
		totalConflicts += len(file.Conflicts)
	}
	result.Summary.TotalConflicts = totalConflicts
}

// Custom validation functions

// validateFilePath validates file paths for security
func validateFilePath(fl validator.FieldLevel) bool {
	path := fl.Field().String()

	// Check for dangerous characters and path traversal attempts
	dangerous := []string{"..", "\x00", "\n", "\r", "|", "&", ";", "$", "`", "\\", "//"}
	for _, char := range dangerous {
		if strings.Contains(path, char) {
			return false
		}
	}

	// Check for absolute paths starting with dangerous locations
	dangerousPaths := []string{"/etc/", "/usr/", "/bin/", "/sbin/", "/root/", "/home/", "/var/", "/tmp/", "/proc/", "/sys/"}
	lowerPath := strings.ToLower(path)
	for _, dangerousPath := range dangerousPaths {
		if strings.HasPrefix(lowerPath, dangerousPath) {
			return false
		}
	}

	// Must be a reasonable file path
	return len(path) > 0 && len(path) < 500 && path != "/"
}

// validateLanguage validates programming language identifiers
func validateLanguage(fl validator.FieldLevel) bool {
	lang := fl.Field().String()

	// Allowlist of supported languages
	supportedLanguages := map[string]bool{
		"go": true, "javascript": true, "typescript": true, "python": true,
		"java": true, "c": true, "cpp": true, "csharp": true, "ruby": true,
		"php": true, "rust": true, "swift": true, "kotlin": true, "scala": true,
		"json": true, "yaml": true, "xml": true, "markdown": true, "text": true,
		"shell": true, "bash": true, "css": true, "scss": true, "html": true,
		"sql": true, "dockerfile": true, "makefile": true, "header": true,
	}

	return supportedLanguages[strings.ToLower(lang)]
}

// validateConflictID validates conflict ID format
func validateConflictID(fl validator.FieldLevel) bool {
	id := fl.Field().String()

	// Allow empty IDs (optional field)
	if id == "" {
		return true
	}

	// Conflict ID should be in format "file:index" or similar safe format
	if len(id) > 100 {
		return false
	}

	// Only allow alphanumeric, colon, hyphen, underscore, dot, slash
	for _, r := range id {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == ':' || r == '-' || r == '_' || r == '.' || r == '/') {
			return false
		}
	}

	return true
}

// validateSafeContent validates content for dangerous characters
func validateSafeContent(fl validator.FieldLevel) bool {
	content := fl.Field().String()

	// Check length
	if len(content) > MaxLineLength {
		return false
	}

	// Check for null bytes and dangerous control characters
	for _, r := range content {
		if r == 0 || (r < 32 && r != '\t' && r != '\n' && r != '\r') {
			return false
		}
	}

	// Check for potential command injection patterns
	dangerousPatterns := []string{
		"$(", "`", "|", "&", ";", "&&", "||", ">", ">>", "<",
	}
	for _, pattern := range dangerousPatterns {
		if strings.Contains(content, pattern) {
			// Allow these in certain contexts (like shell scripts)
			// More sophisticated validation could check file type
			continue
		}
	}

	return true
}

// validateRepoPath validates repository paths
func validateRepoPath(fl validator.FieldLevel) bool {
	path := fl.Field().String()

	// Check for dangerous characters
	dangerous := []string{"\x00", "\n", "\r", "|", "&", ";", "$", "`"}
	for _, char := range dangerous {
		if strings.Contains(path, char) {
			return false
		}
	}

	// Must be a reasonable repository path
	return len(path) > 0 && len(path) < 1000
}

// SanitizePayload removes potentially dangerous content
func (pv *PayloadValidator) SanitizePayload(payload *ValidatedConflictPayload) {
	for i := range payload.Files {
		file := &payload.Files[i]

		// Sanitize file path
		file.Path = pv.sanitizePath(file.Path)

		// Sanitize conflict content
		for j := range file.Conflicts {
			conflict := &file.Conflicts[j]

			conflict.OursLines = pv.sanitizeLines(conflict.OursLines)
			conflict.TheirsLines = pv.sanitizeLines(conflict.TheirsLines)
			conflict.BaseLines = pv.sanitizeLines(conflict.BaseLines)

			// Sanitize conflict ID
			if conflict.ID != "" {
				conflict.ID = pv.sanitizeConflictID(conflict.ID)
			}
		}

		// Sanitize context
		file.Context.BeforeLines = pv.sanitizeLines(file.Context.BeforeLines)
		file.Context.AfterLines = pv.sanitizeLines(file.Context.AfterLines)
	}

	// Sanitize metadata
	payload.Metadata.RepoPath = pv.sanitizePath(payload.Metadata.RepoPath)
}

// sanitizePath removes dangerous content from file paths
func (pv *PayloadValidator) sanitizePath(path string) string {
	// Remove null bytes and path traversal attempts
	cleaned := strings.ReplaceAll(path, "\x00", "")
	cleaned = strings.ReplaceAll(cleaned, "..", "")
	cleaned = strings.ReplaceAll(cleaned, "//", "/")

	// Limit length
	if len(cleaned) > 500 {
		cleaned = cleaned[:500]
	}

	return cleaned
}

// sanitizeLines removes dangerous content from lines
func (pv *PayloadValidator) sanitizeLines(lines []string) []string {
	sanitized := make([]string, len(lines))
	for i, line := range lines {
		// Remove null bytes
		cleaned := strings.ReplaceAll(line, "\x00", "")

		// Limit length
		if len(cleaned) > MaxLineLength {
			cleaned = cleaned[:MaxLineLength]
		}

		sanitized[i] = cleaned
	}
	return sanitized
}

// sanitizeConflictID removes dangerous content from conflict IDs
func (pv *PayloadValidator) sanitizeConflictID(id string) string {
	// Keep only safe characters
	var result strings.Builder
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == ':' || r == '-' || r == '_' || r == '.' || r == '/' {
			result.WriteRune(r)
		}
	}

	cleaned := result.String()
	if len(cleaned) > 100 {
		cleaned = cleaned[:100]
	}

	return cleaned
}

// ValidateAndSanitize performs both validation and sanitization in one step
func (pv *PayloadValidator) ValidateAndSanitize(data []byte) (*ValidatedConflictPayload, *ValidationResult, error) {
	payload, result, err := pv.ValidatePayload(data)
	if err != nil {
		return nil, result, err
	}

	if payload != nil {
		pv.SanitizePayload(payload)
	}

	return payload, result, nil
}
