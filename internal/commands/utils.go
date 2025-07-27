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
	switch language {
	case "go":
		return validateGoSyntax(lines)
	case "javascript", "typescript":
		return validateJSSyntax(lines)
	case "python":
		return validatePythonSyntax(lines)
	case "java":
		return validateJavaSyntax(lines)
	default:
		// Generic validation
		return validateGenericSyntax(lines)
	}
}

// validateGoSyntax validates Go syntax
func validateGoSyntax(lines []string) ValidationResult {
	result := ValidationResult{Valid: true}
	counts := &bracketCounts{}

	// Validate each line and count brackets
	for i, line := range lines {
		validateGoLine(line, i+1, counts, &result)
	}

	// Check for unbalanced brackets
	validateBracketBalance(counts, &result)

	return result
}

// validateGoLine validates a single line of Go code
func validateGoLine(line string, lineNum int, counts *bracketCounts, result *ValidationResult) {
	trimmed := strings.TrimSpace(line)

	// Count braces, parentheses, brackets
	countBrackets(trimmed, counts)

	// Check for common Go syntax issues
	checkGoSyntaxIssues(trimmed, lineNum, result)
}

// countBrackets counts opening and closing brackets in a line
func countBrackets(line string, counts *bracketCounts) {
	for _, char := range line {
		switch char {
		case '{':
			counts.braces++
		case '}':
			counts.braces--
		case '(':
			counts.parens++
		case ')':
			counts.parens--
		case '[':
			counts.brackets++
		case ']':
			counts.brackets--
		}
	}
}

// checkGoSyntaxIssues checks for common Go syntax problems
func checkGoSyntaxIssues(line string, lineNum int, result *ValidationResult) {
	if strings.HasSuffix(line, ";") && !strings.Contains(line, "for") {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Line %d: Unnecessary semicolon in Go", lineNum))
	}
}

// validateBracketBalance checks if brackets are balanced
func validateBracketBalance(counts *bracketCounts, result *ValidationResult) {
	if counts.braces != 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "Unbalanced braces")
	}
	if counts.parens != 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "Unbalanced parentheses")
	}
	if counts.brackets != 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "Unbalanced brackets")
	}
}

// bracketCounts holds the counts for different bracket types
type bracketCounts struct {
	braces   int
	parens   int
	brackets int
}


// checkJSSemicolon checks for missing semicolons in JavaScript/TypeScript
func checkJSSemicolon(trimmed string, lineNum int) string {
	if trimmed != "" && !strings.HasSuffix(trimmed, ";") &&
		!strings.HasSuffix(trimmed, "{") && !strings.HasSuffix(trimmed, "}") &&
		!strings.Contains(trimmed, "//") && !strings.HasPrefix(trimmed, "//") {
		return fmt.Sprintf("Line %d: Possible missing semicolon", lineNum)
	}
	return ""
}

// validateJSSyntax validates JavaScript/TypeScript syntax
func validateJSSyntax(lines []string) ValidationResult {
	result := ValidationResult{Valid: true}
	totalCounts := bracketCounts{}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		lineCounts := &bracketCounts{}
		countBrackets(trimmed, lineCounts)
		
		// Accumulate bracket counts
		totalCounts.braces += lineCounts.braces
		totalCounts.parens += lineCounts.parens
		totalCounts.brackets += lineCounts.brackets

		// Check for missing semicolons
		if warning := checkJSSemicolon(trimmed, i+1); warning != "" {
			result.Warnings = append(result.Warnings, warning)
		}
	}

	// Validate bracket balance
	validateBracketBalance(&totalCounts, &result)
	return result
}

// checkPythonIndentation validates Python indentation
func checkPythonIndentation(line string, lineNum int) string {
	leadingSpaces := len(line) - len(strings.TrimLeft(line, " \t"))
	if strings.TrimSpace(line) != "" {
		if leadingSpaces%4 != 0 && leadingSpaces%2 != 0 {
			return fmt.Sprintf("Line %d: Inconsistent indentation", lineNum)
		}
	}
	return ""
}

// checkPythonColon checks for missing colons in Python control structures
func checkPythonColon(trimmed string, lineNum int) string {
	controlStructures := []string{"if ", "for ", "while ", "def ", "class "}
	for _, structure := range controlStructures {
		if strings.HasPrefix(trimmed, structure) {
			if !strings.HasSuffix(trimmed, ":") {
				return fmt.Sprintf("Line %d: Missing colon", lineNum)
			}
			break
		}
	}
	return ""
}

// countPythonBrackets counts parentheses and brackets for Python
func countPythonBrackets(line string) (parens, brackets int) {
	for _, char := range line {
		switch char {
		case '(':
			parens++
		case ')':
			parens--
		case '[':
			brackets++
		case ']':
			brackets--
		}
	}
	return parens, brackets
}

// validatePythonSyntax validates Python syntax
func validatePythonSyntax(lines []string) ValidationResult {
	result := ValidationResult{Valid: true}
	totalParens := 0
	totalBrackets := 0

	for i, line := range lines {
		// Check indentation
		if warning := checkPythonIndentation(line, i+1); warning != "" {
			result.Warnings = append(result.Warnings, warning)
		}

		trimmed := strings.TrimSpace(line)

		// Count brackets
		parens, brackets := countPythonBrackets(trimmed)
		totalParens += parens
		totalBrackets += brackets

		// Check for missing colons
		if errMsg := checkPythonColon(trimmed, i+1); errMsg != "" {
			result.Valid = false
			result.Errors = append(result.Errors, errMsg)
		}
	}

	// Check for unbalanced brackets
	if totalParens != 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "Unbalanced parentheses")
	}
	if totalBrackets != 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "Unbalanced brackets")
	}

	return result
}

// checkJavaSemicolon checks for missing semicolons in Java
func checkJavaSemicolon(trimmed string, lineNum int) string {
	if trimmed != "" && !strings.HasSuffix(trimmed, ";") &&
		!strings.HasSuffix(trimmed, "{") && !strings.HasSuffix(trimmed, "}") &&
		!strings.Contains(trimmed, "//") && !strings.HasPrefix(trimmed, "//") &&
		!strings.HasPrefix(trimmed, "package ") && !strings.HasPrefix(trimmed, "import ") {
		return fmt.Sprintf("Line %d: Possible missing semicolon", lineNum)
	}
	return ""
}

// validateJavaSyntax validates Java syntax
func validateJavaSyntax(lines []string) ValidationResult {
	result := ValidationResult{Valid: true}
	totalCounts := bracketCounts{}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		lineCounts := &bracketCounts{}
		countBrackets(trimmed, lineCounts)
		
		// Accumulate bracket counts
		totalCounts.braces += lineCounts.braces
		totalCounts.parens += lineCounts.parens
		totalCounts.brackets += lineCounts.brackets

		// Check for missing semicolons
		if warning := checkJavaSemicolon(trimmed, i+1); warning != "" {
			result.Warnings = append(result.Warnings, warning)
		}
	}

	// Validate bracket balance
	validateBracketBalance(&totalCounts, &result)
	return result
}

// validateGenericSyntax performs generic syntax validation
func validateGenericSyntax(lines []string) ValidationResult {
	result := ValidationResult{Valid: true}
	counts := &bracketCounts{}

	// Count brackets in all lines
	for _, line := range lines {
		countBrackets(line, counts)
	}

	// Check for unbalanced brackets
	validateGenericBracketBalance(counts, &result)

	return result
}

// validateGenericBracketBalance checks bracket balance for generic syntax
func validateGenericBracketBalance(counts *bracketCounts, result *ValidationResult) {
	if counts.braces != 0 {
		result.Warnings = append(result.Warnings, "Potentially unbalanced braces")
	}
	if counts.parens != 0 {
		result.Warnings = append(result.Warnings, "Potentially unbalanced parentheses")
	}
	if counts.brackets != 0 {
		result.Warnings = append(result.Warnings, "Potentially unbalanced brackets")
	}
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
			// Currently skipping detailed variable declaration analysis
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
func SafeApplyResolutions(
	repoPath string, 
	resolutions []gitutils.ConflictResolution, 
	options SafeApplyOptions,
) (*gitutils.ResolutionResult, error) {
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
