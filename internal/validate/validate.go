// Package validate provides comprehensive validation utilities for checking resolved files
// and running project-specific validation tools
package validate

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Constants for commonly used strings
const (
	SeverityError   = "error"
	SeverityWarning = "warning"
	SeverityInfo    = "info"

	ScriptBuild     = "build"
	ScriptTest      = "test"
	ScriptLint      = "lint"
	ScriptCheck     = "check"
	ScriptValidate  = "validate"
	ScriptFormat    = "format"
	ScriptTypeCheck = "type-check"
)

// ProjectType represents the type of project detected
type ProjectType string

const (
	ProjectTypeGo         ProjectType = "go"
	ProjectTypeJavaScript ProjectType = "javascript"
	ProjectTypeTypeScript ProjectType = "typescript"
	ProjectTypePython     ProjectType = "python"
	ProjectTypeRust       ProjectType = "rust"
	ProjectTypeGeneric    ProjectType = "generic"
)

// ProjectInfo contains information about the detected project
type ProjectInfo struct {
	Type          ProjectType `json:"type"`
	RootPath      string      `json:"root_path"`
	ConfigFiles   []string    `json:"config_files"`
	DetectedTools []string    `json:"detected_tools"`
}

// ValidationCommand represents a validation command to execute
type ValidationCommand struct {
	Name        string   `json:"name"`
	Command     string   `json:"command"`
	Args        []string `json:"args"`
	WorkingDir  string   `json:"working_dir"`
	Description string   `json:"description"`
	Required    bool     `json:"required"`
}

// ValidationResult represents the result of validating a file
type ValidationResult struct {
	File         string            `json:"file"`
	IsValid      bool              `json:"is_valid"`
	Issues       []ValidationIssue `json:"issues"`
	ConflictFree bool              `json:"conflict_free"`
}

// ValidationIssue represents a validation problem found in a file
type ValidationIssue struct {
	Line     int    `json:"line"`
	Message  string `json:"message"`
	Severity string `json:"severity"` // "error", "warning", "info"
}

// CommandResult represents the result of executing a validation command
type CommandResult struct {
	Command    ValidationCommand `json:"command"`
	Success    bool              `json:"success"`
	ExitCode   int               `json:"exit_code"`
	Stdout     string            `json:"stdout"`
	Stderr     string            `json:"stderr"`
	Duration   time.Duration     `json:"duration"`
	Error      string            `json:"error,omitempty"`
	Skipped    bool              `json:"skipped"`
	SkipReason string            `json:"skip_reason,omitempty"`
}

// ValidationReport represents the complete validation report
type ValidationReport struct {
	Project        ProjectInfo        `json:"project"`
	ValidationTime time.Time          `json:"validation_time"`
	OverallSuccess bool               `json:"overall_success"`
	CommandResults []CommandResult    `json:"command_results"`
	FileResults    []ValidationResult `json:"file_results"`
	Summary        ValidationSummary  `json:"summary"`
}

// ValidationSummary provides a summary of validation results
type ValidationSummary struct {
	TotalCommands      int `json:"total_commands"`
	SuccessfulCommands int `json:"successful_commands"`
	FailedCommands     int `json:"failed_commands"`
	SkippedCommands    int `json:"skipped_commands"`
	TotalFiles         int `json:"total_files"`
	ValidFiles         int `json:"valid_files"`
	InvalidFiles       int `json:"invalid_files"`
	TotalIssues        int `json:"total_issues"`
	ErrorIssues        int `json:"error_issues"`
	WarningIssues      int `json:"warning_issues"`
}

// ValidateFile performs comprehensive validation on a file
func ValidateFile(filepath string) (*ValidationResult, error) {
	result := &ValidationResult{
		File:    filepath,
		IsValid: true,
		Issues:  []ValidationIssue{},
	}

	// Check for merge conflict markers
	conflictFree, conflictIssues, err := checkConflictMarkers(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to check conflict markers: %w", err)
	}

	result.ConflictFree = conflictFree
	result.Issues = append(result.Issues, conflictIssues...)

	// Check for syntax issues based on file type
	syntaxIssues, err := checkSyntax(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to check syntax: %w", err)
	}
	result.Issues = append(result.Issues, syntaxIssues...)

	// Determine overall validity
	for _, issue := range result.Issues {
		if issue.Severity == SeverityError {
			result.IsValid = false
			break
		}
	}

	return result, nil
}

// checkConflictMarkers scans a file for Git merge conflict markers
func checkConflictMarkers(filepath string) (bool, []ValidationIssue, error) {
	file, err := os.Open(filepath) // #nosec G304 - filepath is validated by caller
	if err != nil {
		return false, nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to close file: %v\n", closeErr)
		}
	}()

	var issues []ValidationIssue
	scanner := bufio.NewScanner(file)
	lineNum := 0

	conflictMarkers := []string{"<<<<<<<", "=======", ">>>>>>>"}

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		for _, marker := range conflictMarkers {
			if strings.HasPrefix(line, marker) {
				issues = append(issues, ValidationIssue{
					Line:     lineNum,
					Message:  fmt.Sprintf("Git merge conflict marker found: %s", marker),
					Severity: SeverityError,
				})
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return false, nil, fmt.Errorf("failed to read file: %w", err)
	}

	return len(issues) == 0, issues, nil
}

// checkSyntax performs basic syntax validation based on file extension
func checkSyntax(filepath string) ([]ValidationIssue, error) {
	var issues []ValidationIssue

	ext := strings.ToLower(filepath[strings.LastIndex(filepath, ".")+1:])

	switch ext {
	case "go":
		return checkGoSyntax(filepath)
	case "json":
		return checkJSONSyntax(filepath)
	case "py":
		return checkPythonSyntax(filepath)
	case "js", "ts":
		return checkJSSyntax(filepath)
	default:
		// No specific syntax checker for this file type
		return issues, nil
	}
}

// checkGoSyntax validates Go syntax using go fmt
func checkGoSyntax(filepath string) ([]ValidationIssue, error) {
	// TODO: Implement Go syntax checking
	return []ValidationIssue{}, nil
}

// checkJSONSyntax validates JSON syntax
func checkJSONSyntax(filepath string) ([]ValidationIssue, error) {
	// TODO: Implement JSON syntax checking
	return []ValidationIssue{}, nil
}

// checkPythonSyntax validates Python syntax
func checkPythonSyntax(filepath string) ([]ValidationIssue, error) {
	// TODO: Implement Python syntax checking
	return []ValidationIssue{}, nil
}

// checkJSSyntax validates JavaScript/TypeScript syntax
func checkJSSyntax(filepath string) ([]ValidationIssue, error) {
	// TODO: Implement JS/TS syntax checking
	return []ValidationIssue{}, nil
}

// DiscoverProject analyzes the given directory to determine project type and available tools
func DiscoverProject(rootPath string) (*ProjectInfo, error) {
	// Verify the root path exists
	if _, err := os.Stat(rootPath); err != nil {
		return nil, fmt.Errorf("root path does not exist or is not accessible: %w", err)
	}

	info := &ProjectInfo{
		Type:          ProjectTypeGeneric,
		RootPath:      rootPath,
		ConfigFiles:   []string{},
		DetectedTools: []string{},
	}

	// Check for specific project files to determine type
	projectFiles := map[string]ProjectType{
		"go.mod":           ProjectTypeGo,
		"go.sum":           ProjectTypeGo,
		"package.json":     ProjectTypeJavaScript,
		"tsconfig.json":    ProjectTypeTypeScript,
		"pyproject.toml":   ProjectTypePython,
		"setup.py":         ProjectTypePython,
		"requirements.txt": ProjectTypePython,
		"Cargo.toml":       ProjectTypeRust,
		"Cargo.lock":       ProjectTypeRust,
	}

	// Scan for project files - continue even if some checks fail
	for filename, projectType := range projectFiles {
		filePath := filepath.Join(rootPath, filename)
		if _, err := os.Stat(filePath); err == nil {
			info.ConfigFiles = append(info.ConfigFiles, filename)
			// Set project type to the most specific one found
			if info.Type == ProjectTypeGeneric ||
				(projectType == ProjectTypeTypeScript && info.Type == ProjectTypeJavaScript) {
				info.Type = projectType
			}
		}
	}

	// Detect available tools based on project type - never fail on tool detection
	func() {
		defer func() {
			if r := recover(); r != nil {
				// If tool detection panics, continue with empty tools
				info.DetectedTools = []string{}
			}
		}()

		switch info.Type {
		case ProjectTypeGo:
			info.DetectedTools = detectGoTools(rootPath)
		case ProjectTypeJavaScript, ProjectTypeTypeScript:
			info.DetectedTools = detectNodeTools(rootPath)
		case ProjectTypePython:
			info.DetectedTools = detectPythonTools(rootPath)
		case ProjectTypeRust:
			info.DetectedTools = detectRustTools(rootPath)
		default:
			info.DetectedTools = detectGenericTools(rootPath)
		}
	}()

	return info, nil
}

// detectGoTools detects available Go validation tools
func detectGoTools(rootPath string) []string {
	var tools []string

	// Check for go command
	if _, err := exec.LookPath("go"); err == nil {
		tools = append(tools, "go build", "go test", "go vet")
	}

	// Check for additional Go tools
	additionalTools := []string{"golint", "gofmt", "staticcheck", "gosec"}
	for _, tool := range additionalTools {
		if _, err := exec.LookPath(tool); err == nil {
			tools = append(tools, tool)
		}
	}

	return tools
}

// detectNodeTools detects available Node.js/npm validation tools
func detectNodeTools(rootPath string) []string {
	var tools []string

	// Check for npm/yarn
	if _, err := exec.LookPath("npm"); err == nil {
		tools = append(tools, "npm")

		// Check for common npm scripts
		packageJSONPath := filepath.Join(rootPath, "package.json")
		if scripts := detectNpmScripts(packageJSONPath); len(scripts) > 0 {
			for _, script := range scripts {
				tools = append(tools, "npm run "+script)
			}
		}
	}

	if _, err := exec.LookPath("yarn"); err == nil {
		tools = append(tools, "yarn")
	}

	// Check for standalone tools
	standaloneTools := []string{"eslint", "tsc", "prettier", "jest"}
	for _, tool := range standaloneTools {
		if _, err := exec.LookPath(tool); err == nil {
			tools = append(tools, tool)
		}
	}

	return tools
}

// detectPythonTools detects available Python validation tools
func detectPythonTools(rootPath string) []string {
	var tools []string

	// Check for Python interpreters
	pythonCommands := []string{"python3", "python", "python3.11", "python3.10", "python3.9"}
	for _, cmd := range pythonCommands {
		if _, err := exec.LookPath(cmd); err == nil {
			tools = append(tools, cmd)
			break // Only need one Python interpreter
		}
	}

	// Check for Python tools
	pythonTools := []string{"pytest", "flake8", "mypy", "black", "isort", "pylint", "bandit"}
	for _, tool := range pythonTools {
		if _, err := exec.LookPath(tool); err == nil {
			tools = append(tools, tool)
		}
	}

	return tools
}

// detectRustTools detects available Rust validation tools
func detectRustTools(rootPath string) []string {
	var tools []string

	// Check for cargo
	if _, err := exec.LookPath("cargo"); err == nil {
		tools = append(tools, "cargo build", "cargo test", "cargo clippy", "cargo fmt")
	}

	// Check for additional Rust tools
	additionalTools := []string{"rustfmt", "clippy"}
	for _, tool := range additionalTools {
		if _, err := exec.LookPath(tool); err == nil {
			tools = append(tools, tool)
		}
	}

	return tools
}

// detectGenericTools detects generic build tools
func detectGenericTools(rootPath string) []string {
	var tools []string

	// Check for Makefile
	makefilePath := filepath.Join(rootPath, "Makefile")
	if _, err := os.Stat(makefilePath); err == nil {
		if _, err := exec.LookPath("make"); err == nil {
			// Check for common make targets
			targets := detectMakeTargets(makefilePath)
			for _, target := range targets {
				tools = append(tools, "make "+target)
			}
		}
	}

	// Check for other build tools
	buildTools := []string{"cmake", "ninja", "bazel"}
	for _, tool := range buildTools {
		if _, err := exec.LookPath(tool); err == nil {
			tools = append(tools, tool)
		}
	}

	return tools
}

// detectNpmScripts parses package.json to find available scripts
func detectNpmScripts(packageJSONPath string) []string {
	var scripts []string

	data, err := os.ReadFile(packageJSONPath) // #nosec G304 - packageJSONPath is constructed safely from validated inputs
	if err != nil {
		return scripts
	}

	var packageJSON struct {
		Scripts map[string]string `json:"scripts"`
	}

	if err := json.Unmarshal(data, &packageJSON); err != nil {
		return scripts
	}

	// Common validation scripts
	validationScripts := []string{ScriptTest, ScriptBuild, ScriptLint, ScriptTypeCheck, ScriptFormat}
	for _, script := range validationScripts {
		if _, exists := packageJSON.Scripts[script]; exists {
			scripts = append(scripts, script)
		}
	}

	return scripts
}

// detectMakeTargets parses Makefile to find common validation targets
func detectMakeTargets(makefilePath string) []string {
	var targets []string

	file, err := os.Open(makefilePath) // #nosec G304 - makefilePath is constructed safely from validated inputs
	if err != nil {
		return targets
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to close file: %v\n", closeErr)
		}
	}()

	scanner := bufio.NewScanner(file)
	validationTargets := map[string]bool{
		ScriptTest:     true,
		ScriptBuild:    true,
		ScriptLint:     true,
		ScriptCheck:    true,
		ScriptValidate: true,
		ScriptFormat:   true,
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.Contains(line, ":") && !strings.HasPrefix(line, "#") {
			parts := strings.Split(line, ":")
			if len(parts) > 0 {
				target := strings.TrimSpace(parts[0])
				if validationTargets[target] {
					targets = append(targets, target)
				}
			}
		}
	}

	return targets
}

// BuildValidationCommands creates a list of validation commands based on project info
func BuildValidationCommands(projectInfo *ProjectInfo) []ValidationCommand {
	var commands []ValidationCommand

	switch projectInfo.Type {
	case ProjectTypeGo:
		commands = buildGoCommands(projectInfo)
	case ProjectTypeJavaScript, ProjectTypeTypeScript:
		commands = buildNodeCommands(projectInfo)
	case ProjectTypePython:
		commands = buildPythonCommands(projectInfo)
	case ProjectTypeRust:
		commands = buildRustCommands(projectInfo)
	default:
		commands = buildGenericCommands(projectInfo)
	}

	return commands
}

// buildGoCommands creates validation commands for Go projects
func buildGoCommands(projectInfo *ProjectInfo) []ValidationCommand {
	var commands []ValidationCommand

	// Essential Go commands
	if isToolAvailable("go") {
		commands = append(commands, ValidationCommand{
			Name:        "go_build",
			Command:     "go",
			Args:        []string{ScriptBuild, "./..."},
			WorkingDir:  projectInfo.RootPath,
			Description: "Build Go project to check for compilation errors",
			Required:    true,
		})

		commands = append(commands, ValidationCommand{
			Name:        "go_test",
			Command:     "go",
			Args:        []string{"test", "./..."},
			WorkingDir:  projectInfo.RootPath,
			Description: "Run Go tests",
			Required:    false,
		})

		commands = append(commands, ValidationCommand{
			Name:        "go_vet",
			Command:     "go",
			Args:        []string{"vet", "./..."},
			WorkingDir:  projectInfo.RootPath,
			Description: "Run Go vet to check for suspicious constructs",
			Required:    false,
		})
	}

	// Additional Go tools
	if isToolAvailable("gofmt") {
		commands = append(commands, ValidationCommand{
			Name:        "gofmt",
			Command:     "gofmt",
			Args:        []string{"-l", "."},
			WorkingDir:  projectInfo.RootPath,
			Description: "Check Go code formatting",
			Required:    false,
		})
	}

	if isToolAvailable("staticcheck") {
		commands = append(commands, ValidationCommand{
			Name:        "staticcheck",
			Command:     "staticcheck",
			Args:        []string{"./..."},
			WorkingDir:  projectInfo.RootPath,
			Description: "Run staticcheck linter",
			Required:    false,
		})
	}

	if isToolAvailable("gosec") {
		commands = append(commands, ValidationCommand{
			Name:        "gosec",
			Command:     "gosec",
			Args:        []string{"./..."},
			WorkingDir:  projectInfo.RootPath,
			Description: "Run gosec security scanner",
			Required:    false,
		})
	}

	return commands
}

// buildNodeCommands creates validation commands for Node.js projects
func buildNodeCommands(projectInfo *ProjectInfo) []ValidationCommand {
	var commands []ValidationCommand

	// Check for npm scripts first
	packageJSONPath := filepath.Join(projectInfo.RootPath, "package.json")
	npmScripts := detectNpmScripts(packageJSONPath)

	if isToolAvailable("npm") {
		for _, script := range npmScripts {
			var required bool
			var description string

			switch script {
			case ScriptBuild:
				required = true
				description = "Build the project"
			case "test":
				required = false
				description = "Run project tests"
			case "lint":
				required = false
				description = "Run code linting"
			case "type-check":
				required = false
				description = "Run TypeScript type checking"
			default:
				required = false
				description = fmt.Sprintf("Run npm script: %s", script)
			}

			commands = append(commands, ValidationCommand{
				Name:        fmt.Sprintf("npm_%s", script),
				Command:     "npm",
				Args:        []string{"run", script},
				WorkingDir:  projectInfo.RootPath,
				Description: description,
				Required:    required,
			})
		}
	}

	// Fallback to direct tool execution if npm scripts not available
	if len(commands) == 0 {
		if isToolAvailable("tsc") && projectInfo.Type == ProjectTypeTypeScript {
			commands = append(commands, ValidationCommand{
				Name:        "typescript_check",
				Command:     "tsc",
				Args:        []string{"--noEmit"},
				WorkingDir:  projectInfo.RootPath,
				Description: "TypeScript type checking",
				Required:    true,
			})
		}

		if isToolAvailable("eslint") {
			commands = append(commands, ValidationCommand{
				Name:        "eslint",
				Command:     "eslint",
				Args:        []string{"."},
				WorkingDir:  projectInfo.RootPath,
				Description: "Run ESLint",
				Required:    false,
			})
		}

		if isToolAvailable("jest") {
			commands = append(commands, ValidationCommand{
				Name:        "jest",
				Command:     "jest",
				Args:        []string{},
				WorkingDir:  projectInfo.RootPath,
				Description: "Run Jest tests",
				Required:    false,
			})
		}
	}

	return commands
}

// buildPythonCommands creates validation commands for Python projects
func buildPythonCommands(projectInfo *ProjectInfo) []ValidationCommand {
	var commands []ValidationCommand

	// Find Python interpreter
	pythonCmd := ""
	pythonCommands := []string{"python3", "python", "python3.11", "python3.10", "python3.9"}
	for _, cmd := range pythonCommands {
		if isToolAvailable(cmd) {
			pythonCmd = cmd
			break
		}
	}

	if pythonCmd != "" {
		// Basic syntax check
		commands = append(commands, ValidationCommand{
			Name:        "python_syntax",
			Command:     pythonCmd,
			Args:        []string{"-m", "py_compile", "."},
			WorkingDir:  projectInfo.RootPath,
			Description: "Check Python syntax",
			Required:    true,
		})
	}

	// Testing tools
	if isToolAvailable("pytest") {
		commands = append(commands, ValidationCommand{
			Name:        "pytest",
			Command:     "pytest",
			Args:        []string{"."},
			WorkingDir:  projectInfo.RootPath,
			Description: "Run pytest tests",
			Required:    false,
		})
	}

	// Linting tools
	if isToolAvailable("flake8") {
		commands = append(commands, ValidationCommand{
			Name:        "flake8",
			Command:     "flake8",
			Args:        []string{"."},
			WorkingDir:  projectInfo.RootPath,
			Description: "Run flake8 linter",
			Required:    false,
		})
	}

	if isToolAvailable("mypy") {
		commands = append(commands, ValidationCommand{
			Name:        "mypy",
			Command:     "mypy",
			Args:        []string{"."},
			WorkingDir:  projectInfo.RootPath,
			Description: "Run mypy type checking",
			Required:    false,
		})
	}

	if isToolAvailable("black") {
		commands = append(commands, ValidationCommand{
			Name:        "black_check",
			Command:     "black",
			Args:        []string{"--check", "."},
			WorkingDir:  projectInfo.RootPath,
			Description: "Check code formatting with Black",
			Required:    false,
		})
	}

	if isToolAvailable("bandit") {
		commands = append(commands, ValidationCommand{
			Name:        "bandit",
			Command:     "bandit",
			Args:        []string{"-r", "."},
			WorkingDir:  projectInfo.RootPath,
			Description: "Run bandit security scanner",
			Required:    false,
		})
	}

	return commands
}

// buildRustCommands creates validation commands for Rust projects
func buildRustCommands(projectInfo *ProjectInfo) []ValidationCommand {
	var commands []ValidationCommand

	if isToolAvailable("cargo") {
		commands = append(commands, ValidationCommand{
			Name:        "cargo_build",
			Command:     "cargo",
			Args:        []string{ScriptBuild},
			WorkingDir:  projectInfo.RootPath,
			Description: "Build Rust project",
			Required:    true,
		})

		commands = append(commands, ValidationCommand{
			Name:        "cargo_test",
			Command:     "cargo",
			Args:        []string{"test"},
			WorkingDir:  projectInfo.RootPath,
			Description: "Run Rust tests",
			Required:    false,
		})

		commands = append(commands, ValidationCommand{
			Name:        "cargo_clippy",
			Command:     "cargo",
			Args:        []string{"clippy", "--", "-D", "warnings"},
			WorkingDir:  projectInfo.RootPath,
			Description: "Run Clippy linter",
			Required:    false,
		})

		commands = append(commands, ValidationCommand{
			Name:        "cargo_fmt_check",
			Command:     "cargo",
			Args:        []string{"fmt", "--", "--check"},
			WorkingDir:  projectInfo.RootPath,
			Description: "Check Rust code formatting",
			Required:    false,
		})
	}

	return commands
}

// buildGenericCommands creates validation commands for generic projects
func buildGenericCommands(projectInfo *ProjectInfo) []ValidationCommand {
	var commands []ValidationCommand

	// Check for Makefile
	makefilePath := filepath.Join(projectInfo.RootPath, "Makefile")
	if _, err := os.Stat(makefilePath); err == nil && isToolAvailable("make") {
		targets := detectMakeTargets(makefilePath)
		for _, target := range targets {
			var required bool
			if target == ScriptBuild || target == ScriptTest {
				required = target == ScriptBuild
			}

			commands = append(commands, ValidationCommand{
				Name:        fmt.Sprintf("make_%s", target),
				Command:     "make",
				Args:        []string{target},
				WorkingDir:  projectInfo.RootPath,
				Description: fmt.Sprintf("Run make %s", target),
				Required:    required,
			})
		}
	}

	return commands
}

// isToolAvailable checks if a command-line tool is available
func isToolAvailable(tool string) bool {
	_, err := exec.LookPath(tool)
	return err == nil
}

// ExecuteValidationCommands runs validation commands with timeout and error handling
func ExecuteValidationCommands(commands []ValidationCommand, timeoutSeconds int) []CommandResult {
	var results []CommandResult

	for _, cmd := range commands {
		result := executeCommand(cmd, timeoutSeconds)
		results = append(results, result)
	}

	return results
}

// executeCommand runs a single validation command with timeout
func executeCommand(cmd ValidationCommand, timeoutSeconds int) CommandResult {
	result := CommandResult{
		Command: cmd,
		Success: false,
	}

	// Check if command is available
	if !isToolAvailable(cmd.Command) {
		result.Skipped = true
		result.SkipReason = fmt.Sprintf("Command '%s' not available", cmd.Command)
		return result
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	// Validate command to prevent command injection
	matched, err := regexp.MatchString(`^[a-zA-Z0-9_\-]+$`, cmd.Command)
	if err != nil {
		result.Error = fmt.Sprintf("Error validating command format: %v", err)
		return result
	}
	if !matched {
		result.Error = fmt.Sprintf("Invalid command format: %s", cmd.Command)
		return result
	}

	// Create command
	// #nosec G204 - cmd.Command is validated with regex above
	execCmd := exec.CommandContext(ctx, cmd.Command, cmd.Args...)
	if cmd.WorkingDir != "" {
		execCmd.Dir = cmd.WorkingDir
	}

	// Capture output
	var stdout, stderr strings.Builder
	execCmd.Stdout = &stdout
	execCmd.Stderr = &stderr

	// Record start time
	startTime := time.Now()

	// Execute command
	err = execCmd.Run()
	result.Duration = time.Since(startTime)
	result.Stdout = stdout.String()
	result.Stderr = stderr.String()

	// Handle result
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.Error = fmt.Sprintf("Command timed out after %d seconds", timeoutSeconds)
			result.ExitCode = -1
		} else if exitError, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitError.ExitCode()
			result.Error = fmt.Sprintf("Command failed with exit code %d", result.ExitCode)
		} else {
			result.Error = fmt.Sprintf("Command execution failed: %v", err)
			result.ExitCode = -1
		}
	} else {
		result.Success = true
		result.ExitCode = 0
	}

	return result
}

// RunValidation performs complete validation on a project
func RunValidation(rootPath string, timeoutSeconds int) (*ValidationReport, error) {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 300 // Default 5 minute timeout
	}

	// Initialize empty report in case of early failures
	report := &ValidationReport{
		ValidationTime: time.Now(),
		OverallSuccess: false,
		CommandResults: []CommandResult{},
		FileResults:    []ValidationResult{},
	}

	// Discover project - if this fails, we can still proceed with minimal validation
	projectInfo, err := DiscoverProject(rootPath)
	if err != nil {
		// Create minimal project info for fallback
		projectInfo = &ProjectInfo{
			Type:          ProjectTypeGeneric,
			RootPath:      rootPath,
			ConfigFiles:   []string{},
			DetectedTools: []string{},
		}
	}
	report.Project = *projectInfo

	// Build validation commands - this should never fail but be safe
	var commands []ValidationCommand
	func() {
		defer func() {
			if r := recover(); r != nil {
				// If command building panics, continue with empty commands
				commands = []ValidationCommand{}
			}
		}()
		commands = BuildValidationCommands(projectInfo)
	}()

	// Execute validation commands - individual command failures are captured
	commandResults := ExecuteValidationCommands(commands, timeoutSeconds)
	report.CommandResults = commandResults

	// Validate individual files for conflict markers - failures are non-critical
	fileResults, err := validateProjectFiles(rootPath)
	if err != nil {
		// Don't fail the entire validation if file validation fails
		// This ensures we never block the workflow
		fileResults = []ValidationResult{}
	}
	report.FileResults = fileResults

	// Calculate summary - this should be safe with empty slices
	report.Summary = calculateSummary(commandResults, fileResults)

	// Determine overall success
	// Success if no required commands failed and no critical file issues
	report.OverallSuccess = true
	for _, cmdResult := range commandResults {
		if cmdResult.Command.Required && !cmdResult.Success && !cmdResult.Skipped {
			report.OverallSuccess = false
			break
		}
	}

	// Check for critical file issues
	hasCriticalIssues := false
	for _, fileResult := range fileResults {
		if !fileResult.ConflictFree {
			// Conflict markers are critical but don't fail overall validation
			// as the goal is to provide feedback, not block the workflow
			hasCriticalIssues = true
		}
		for _, issue := range fileResult.Issues {
			if issue.Severity == SeverityError {
				// File syntax errors also don't fail overall validation
				hasCriticalIssues = true
			}
		}
	}
	
	// Set overall success based on critical issues but don't fail validation
	// The goal is to provide feedback, not block workflows
	if hasCriticalIssues {
		report.OverallSuccess = false
	}

	return report, nil
}

// validateProjectFiles validates files in the project for basic issues
func validateProjectFiles(rootPath string) ([]ValidationResult, error) {
	var results []ValidationResult

	// Find relevant files to validate
	filesToCheck, err := findFilesToValidate(rootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to find files to validate: %w", err)
	}

	for _, filePath := range filesToCheck {
		result, err := ValidateFile(filePath)
		if err != nil {
			// Continue with other files even if one fails
			continue
		}
		results = append(results, *result)
	}

	return results, nil
}

// findFilesToValidate finds files that should be validated
func findFilesToValidate(rootPath string) ([]string, error) {
	var files []string

	// File patterns to validate
	patterns := []string{
		"*.go",
		"*.js",
		"*.ts",
		"*.tsx",
		"*.jsx",
		"*.py",
		"*.rs",
		"*.java",
		"*.c",
		"*.cpp",
		"*.h",
		"*.hpp",
		"*.json",
		"*.yaml",
		"*.yml",
	}

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue walking even if there's an error
		}

		// Skip directories and hidden files
		if info.IsDir() || strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		// Skip common directories that should not be validated
		relPath, err := filepath.Rel(rootPath, path)
		if err != nil {
			// If we can't get relative path, skip this file
			return nil
		}
		skipDirs := []string{"node_modules", "vendor", "target", ".git", "dist", ScriptBuild}
		for _, skipDir := range skipDirs {
			if strings.Contains(relPath, skipDir+string(filepath.Separator)) {
				return nil
			}
		}

		// Check if file matches any pattern
		for _, pattern := range patterns {
			matched, err := filepath.Match(pattern, info.Name())
			if err != nil {
				// If pattern matching fails, skip this pattern
				continue
			}
			if matched {
				files = append(files, path)
				break
			}
		}

		return nil
	})

	return files, err
}

// calculateSummary calculates validation summary statistics
func calculateSummary(commandResults []CommandResult, fileResults []ValidationResult) ValidationSummary {
	summary := ValidationSummary{
		TotalCommands: len(commandResults),
		TotalFiles:    len(fileResults),
	}

	// Count command results
	for _, result := range commandResults {
		if result.Success {
			summary.SuccessfulCommands++
		} else if result.Skipped {
			summary.SkippedCommands++
		} else {
			summary.FailedCommands++
		}
	}

	// Count file results and issues
	for _, result := range fileResults {
		if result.IsValid {
			summary.ValidFiles++
		} else {
			summary.InvalidFiles++
		}

		for _, issue := range result.Issues {
			summary.TotalIssues++
			switch issue.Severity {
			case SeverityError:
				summary.ErrorIssues++
			case SeverityWarning:
				summary.WarningIssues++
			default:
				// Handle other severity levels if needed
			}
		}
	}

	return summary
}

// ValidateRepository performs validation on all relevant files in the repository
func ValidateRepository() ([]ValidationResult, error) {
	// TODO: Implement repository-wide validation
	return []ValidationResult{}, nil
}
