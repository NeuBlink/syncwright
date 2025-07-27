package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/NeuBlink/syncwright/internal/gitutils"
)

// ValidationResult represents the result of validating a resolution
type ValidationResult struct {
	Valid         bool     `json:"valid"`
	Errors        []string `json:"errors,omitempty"`
	Warnings      []string `json:"warnings,omitempty"`
	SyntaxValid   bool     `json:"syntax_valid"`
	SemanticValid bool     `json:"semantic_valid"`
}

// ProgressReporter provides progress reporting for long operations
type ProgressReporter struct {
	total     int
	current   int
	startTime time.Time
	verbose   bool
}

// NewProgressReporter creates a new progress reporter
func NewProgressReporter(total int, verbose bool) *ProgressReporter {
	return &ProgressReporter{
		total:     total,
		current:   0,
		startTime: time.Now(),
		verbose:   verbose,
	}
}

// Update updates the progress and optionally displays it
func (pr *ProgressReporter) Update(current int, message string) {
	pr.current = current

	if !pr.verbose {
		return
	}

	percentage := float64(current) / float64(pr.total) * 100
	elapsed := time.Since(pr.startTime)

	if current > 0 {
		estimated := time.Duration(float64(elapsed) / float64(current) * float64(pr.total))
		remaining := estimated - elapsed

		fmt.Printf("\r[%3.0f%%] %s (ETA: %v)", percentage, message, remaining.Round(time.Second))
	} else {
		fmt.Printf("\r[%3.0f%%] %s", percentage, message)
	}

	if current >= pr.total {
		fmt.Println() // New line when complete
	}
}

// Complete marks the progress as complete
func (pr *ProgressReporter) Complete(message string) {
	pr.Update(pr.total, message)
	if pr.verbose {
		fmt.Printf("✅ Completed in %v\n", time.Since(pr.startTime).Round(time.Millisecond))
	}
}

// ValidateResolution validates a conflict resolution before applying it
func ValidateResolution(resolution gitutils.ConflictResolution, repoPath string) ValidationResult {
	result := ValidationResult{
		Valid:         true,
		SyntaxValid:   true,
		SemanticValid: true,
	}

	// Basic validation
	if resolution.FilePath == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "File path is empty")
	}

	if resolution.StartLine <= 0 || resolution.EndLine <= 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "Invalid line numbers")
	}

	if resolution.StartLine > resolution.EndLine {
		result.Valid = false
		result.Errors = append(result.Errors, "Start line cannot be greater than end line")
	}

	if resolution.Confidence < 0 || resolution.Confidence > 1 {
		result.Valid = false
		result.Errors = append(result.Errors, "Confidence must be between 0 and 1")
	}

	// Check if file exists
	fullPath := filepath.Join(repoPath, resolution.FilePath)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("File does not exist: %s", resolution.FilePath))
		return result
	}

	// Validate syntax for known file types
	language := detectLanguageFromPath(resolution.FilePath)
	if language != "" {
		syntaxResult := validateSyntax(resolution.ResolvedLines, language)
		if !syntaxResult.Valid {
			result.SyntaxValid = false
			result.Warnings = append(result.Warnings, "Potential syntax issues detected")
			result.Warnings = append(result.Warnings, syntaxResult.Errors...)
		}
	}

	// Check for potential semantic issues
	semanticResult := validateSemantics(resolution.ResolvedLines, language)
	if !semanticResult.Valid {
		result.SemanticValid = false
		result.Warnings = append(result.Warnings, "Potential semantic issues detected")
		result.Warnings = append(result.Warnings, semanticResult.Errors...)
	}

	// Check for remaining conflict markers
	if hasConflictMarkers(resolution.ResolvedLines) {
		result.Valid = false
		result.Errors = append(result.Errors, "Resolution still contains conflict markers")
	}

	return result
}

// detectLanguageFromPath detects language from file path
func detectLanguageFromPath(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))

	languageMap := map[string]string{
		".go":   "go",
		".js":   "javascript",
		".ts":   "typescript",
		".py":   "python",
		".java": "java",
		".c":    "c",
		".cpp":  "cpp",
		".cs":   "csharp",
		".rb":   "ruby",
		".php":  "php",
		".rs":   "rust",
	}

	return languageMap[ext]
}

// validateSyntax performs basic syntax validation
func validateSyntax(lines []string, language string) ValidationResult {
	result := ValidationResult{Valid: true}

	switch language {
	case "go":
		result = validateGoSyntax(lines)
	case "javascript", "typescript":
		result = validateJSSyntax(lines)
	case "python":
		result = validatePythonSyntax(lines)
	case "java":
		result = validateJavaSyntax(lines)
	default:
		// Generic validation
		result = validateGenericSyntax(lines)
	}

	return result
}

// validateGoSyntax validates Go syntax
func validateGoSyntax(lines []string) ValidationResult {
	result := ValidationResult{Valid: true}

	openBraces := 0
	openParens := 0
	openBrackets := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Count braces, parentheses, brackets
		for _, char := range trimmed {
			switch char {
			case '{':
				openBraces++
			case '}':
				openBraces--
			case '(':
				openParens++
			case ')':
				openParens--
			case '[':
				openBrackets++
			case ']':
				openBrackets--
			}
		}

		// Check for common Go syntax issues
		if strings.HasSuffix(trimmed, ";") && !strings.Contains(trimmed, "for") {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Line %d: Unnecessary semicolon in Go", i+1))
		}
	}

	// Check for unbalanced brackets
	if openBraces != 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "Unbalanced braces")
	}
	if openParens != 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "Unbalanced parentheses")
	}
	if openBrackets != 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "Unbalanced brackets")
	}

	return result
}

// validateJSSyntax validates JavaScript/TypeScript syntax
func validateJSSyntax(lines []string) ValidationResult {
	result := ValidationResult{Valid: true}

	openBraces := 0
	openParens := 0
	openBrackets := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Count braces, parentheses, brackets (simplified)
		for _, char := range trimmed {
			switch char {
			case '{':
				openBraces++
			case '}':
				openBraces--
			case '(':
				openParens++
			case ')':
				openParens--
			case '[':
				openBrackets++
			case ']':
				openBrackets--
			}
		}

		// Check for missing semicolons (simplified)
		if trimmed != "" && !strings.HasSuffix(trimmed, ";") &&
			!strings.HasSuffix(trimmed, "{") && !strings.HasSuffix(trimmed, "}") &&
			!strings.Contains(trimmed, "//") && !strings.HasPrefix(trimmed, "//") {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Line %d: Possible missing semicolon", i+1))
		}
	}

	// Check for unbalanced brackets
	if openBraces != 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "Unbalanced braces")
	}
	if openParens != 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "Unbalanced parentheses")
	}
	if openBrackets != 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "Unbalanced brackets")
	}

	return result
}

// validatePythonSyntax validates Python syntax
func validatePythonSyntax(lines []string) ValidationResult {
	result := ValidationResult{Valid: true}

	openParens := 0
	openBrackets := 0

	for i, line := range lines {
		// Check indentation (simplified)
		leadingSpaces := len(line) - len(strings.TrimLeft(line, " \t"))
		if strings.TrimSpace(line) != "" {
			if leadingSpaces%4 != 0 && leadingSpaces%2 != 0 {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("Line %d: Inconsistent indentation", i+1))
			}
		}

		trimmed := strings.TrimSpace(line)

		// Count parentheses, brackets
		for _, char := range trimmed {
			switch char {
			case '(':
				openParens++
			case ')':
				openParens--
			case '[':
				openBrackets++
			case ']':
				openBrackets--
			}
		}

		// Check for missing colons
		if strings.HasPrefix(trimmed, "if ") || strings.HasPrefix(trimmed, "for ") ||
			strings.HasPrefix(trimmed, "while ") || strings.HasPrefix(trimmed, "def ") ||
			strings.HasPrefix(trimmed, "class ") {
			if !strings.HasSuffix(trimmed, ":") {
				result.Valid = false
				result.Errors = append(result.Errors,
					fmt.Sprintf("Line %d: Missing colon", i+1))
			}
		}
	}

	// Check for unbalanced brackets
	if openParens != 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "Unbalanced parentheses")
	}
	if openBrackets != 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "Unbalanced brackets")
	}

	return result
}

// validateJavaSyntax validates Java syntax
func validateJavaSyntax(lines []string) ValidationResult {
	result := ValidationResult{Valid: true}

	openBraces := 0
	openParens := 0
	openBrackets := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Count braces, parentheses, brackets
		for _, char := range trimmed {
			switch char {
			case '{':
				openBraces++
			case '}':
				openBraces--
			case '(':
				openParens++
			case ')':
				openParens--
			case '[':
				openBrackets++
			case ']':
				openBrackets--
			}
		}

		// Check for missing semicolons
		if trimmed != "" && !strings.HasSuffix(trimmed, ";") &&
			!strings.HasSuffix(trimmed, "{") && !strings.HasSuffix(trimmed, "}") &&
			!strings.Contains(trimmed, "//") && !strings.HasPrefix(trimmed, "//") &&
			!strings.HasPrefix(trimmed, "package ") && !strings.HasPrefix(trimmed, "import ") {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Line %d: Possible missing semicolon", i+1))
		}
	}

	// Check for unbalanced brackets
	if openBraces != 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "Unbalanced braces")
	}
	if openParens != 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "Unbalanced parentheses")
	}
	if openBrackets != 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "Unbalanced brackets")
	}

	return result
}

// validateGenericSyntax performs generic syntax validation
func validateGenericSyntax(lines []string) ValidationResult {
	result := ValidationResult{Valid: true}

	openBraces := 0
	openParens := 0
	openBrackets := 0

	for _, line := range lines {
		// Count braces, parentheses, brackets
		for _, char := range line {
			switch char {
			case '{':
				openBraces++
			case '}':
				openBraces--
			case '(':
				openParens++
			case ')':
				openParens--
			case '[':
				openBrackets++
			case ']':
				openBrackets--
			}
		}
	}

	// Check for unbalanced brackets
	if openBraces != 0 {
		result.Warnings = append(result.Warnings, "Potentially unbalanced braces")
	}
	if openParens != 0 {
		result.Warnings = append(result.Warnings, "Potentially unbalanced parentheses")
	}
	if openBrackets != 0 {
		result.Warnings = append(result.Warnings, "Potentially unbalanced brackets")
	}

	return result
}

// validateSemantics performs basic semantic validation
func validateSemantics(lines []string, language string) ValidationResult {
	result := ValidationResult{Valid: true}

	// Check for obvious semantic issues
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for duplicate variable declarations (simplified)
		if strings.Contains(trimmed, "var ") || strings.Contains(trimmed, "let ") ||
			strings.Contains(trimmed, "const ") {
			// This is a simplified check - a real implementation would need proper parsing
		}

		// Check for unreachable code
		if strings.Contains(trimmed, "return") && i < len(lines)-1 {
			nextLine := strings.TrimSpace(lines[i+1])
			if nextLine != "" && !strings.HasPrefix(nextLine, "}") {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("Line %d: Possible unreachable code after return", i+2))
			}
		}
	}

	return result
}

// hasConflictMarkers checks if lines contain conflict markers
func hasConflictMarkers(lines []string) bool {
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "<<<<<<<") ||
			strings.HasPrefix(trimmed, "=======") ||
			strings.HasPrefix(trimmed, ">>>>>>>") ||
			strings.HasPrefix(trimmed, "|||||||") {
			return true
		}
	}
	return false
}

// SafeApplyResolutions applies resolutions with validation and safety checks
func SafeApplyResolutions(repoPath string, resolutions []gitutils.ConflictResolution, options SafeApplyOptions) (*gitutils.ResolutionResult, error) {
	result := &gitutils.ResolutionResult{
		Success: true,
	}

	if options.ValidateBeforeApply {
		progress := NewProgressReporter(len(resolutions), options.Verbose)

		for i, resolution := range resolutions {
			progress.Update(i, fmt.Sprintf("Validating %s", resolution.FilePath))

			validation := ValidateResolution(resolution, repoPath)
			if !validation.Valid {
				result.Success = false
				result.FailedCount++
				result.Errors = append(result.Errors,
					fmt.Sprintf("Validation failed for %s: %v", resolution.FilePath, validation.Errors))
				continue
			}

			if len(validation.Warnings) > 0 && options.Verbose {
				fmt.Printf("⚠️  Warnings for %s: %v\n", resolution.FilePath, validation.Warnings)
			}
		}

		progress.Complete("Validation complete")
	}

	// Apply resolutions if validation passed
	if result.Success || options.ContinueOnValidationErrors {
		return gitutils.ApplyResolutions(repoPath, resolutions)
	}

	return result, nil
}

// SafeApplyOptions contains options for safe resolution application
type SafeApplyOptions struct {
	ValidateBeforeApply        bool
	ContinueOnValidationErrors bool
	CreateBackups              bool
	Verbose                    bool
}

// DefaultSafeApplyOptions returns default safe apply options
func DefaultSafeApplyOptions() SafeApplyOptions {
	return SafeApplyOptions{
		ValidateBeforeApply:        true,
		ContinueOnValidationErrors: false,
		CreateBackups:              true,
		Verbose:                    false,
	}
}
