// Package format provides code formatting utilities for resolved files
package format

import (
	"bytes"
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

// Formatter represents a code formatter with its configuration
type Formatter struct {
	Name        string   `json:"name"`
	Command     string   `json:"command"`
	Args        []string `json:"args"`
	Extensions  []string `json:"extensions"`
	Description string   `json:"description"`
	Available   bool     `json:"available"`
	Version     string   `json:"version,omitempty"`
}

// FormatResult represents the result of formatting a single file
type FormatResult struct {
	File      string `json:"file"`
	Formatter string `json:"formatter"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
	Stdout    string `json:"stdout,omitempty"`
	Stderr    string `json:"stderr,omitempty"`
	Duration  string `json:"duration"`
}

// FormatterDiscovery represents the discovery of available formatters
type FormatterDiscovery struct {
	Formatters []Formatter `json:"formatters"`
	Timestamp  string      `json:"timestamp"`
}

// FormatCommand represents the complete format command result
type FormatCommand struct {
	Success             bool           `json:"success"`
	FilesProcessed      int            `json:"files_processed"`
	FilesFormatted      int            `json:"files_formatted"`
	FilesFailed         int            `json:"files_failed"`
	Results             []FormatResult `json:"results"`
	AvailableFormatters []Formatter    `json:"available_formatters"`
	Timestamp           string         `json:"timestamp"`
	Duration            string         `json:"duration"`
}

// FormatOptions contains options for formatting files
type FormatOptions struct {
	DryRun              bool          `json:"dry_run"`
	PreferredFormatters []string      `json:"preferred_formatters,omitempty"`
	ExcludeFormatters   []string      `json:"exclude_formatters,omitempty"`
	Timeout             time.Duration `json:"timeout,omitempty"`
	Concurrency         int           `json:"concurrency,omitempty"`
}

// GetSupportedFormatters returns all known formatters with their configurations
func GetSupportedFormatters() []Formatter {
	return []Formatter{
		// Go formatters
		{
			Name:        "gofmt",
			Command:     "gofmt",
			Args:        []string{"-w"},
			Extensions:  []string{"go"},
			Description: "Go standard formatter",
		},
		{
			Name:        "goimports",
			Command:     "goimports",
			Args:        []string{"-w"},
			Extensions:  []string{"go"},
			Description: "Go formatter with import management",
		},
		// JavaScript/TypeScript formatters
		{
			Name:        "prettier",
			Command:     "prettier",
			Args:        []string{"--write"},
			Extensions:  []string{"js", "jsx", "ts", "tsx", "json", "css", "scss", "md"},
			Description: "Opinionated code formatter for multiple languages",
		},
		{
			Name:        "eslint",
			Command:     "eslint",
			Args:        []string{"--fix"},
			Extensions:  []string{"js", "jsx", "ts", "tsx"},
			Description: "JavaScript linter with auto-fix capabilities",
		},
		// Python formatters
		{
			Name:        "black",
			Command:     "black",
			Args:        []string{},
			Extensions:  []string{"py"},
			Description: "The uncompromising Python code formatter",
		},
		{
			Name:        "autopep8",
			Command:     "autopep8",
			Args:        []string{"--in-place"},
			Extensions:  []string{"py"},
			Description: "A tool that automatically formats Python code to conform to PEP 8",
		},
		{
			Name:        "isort",
			Command:     "isort",
			Args:        []string{},
			Extensions:  []string{"py"},
			Description: "A Python utility / library to sort imports",
		},
		// Rust formatter
		{
			Name:        "rustfmt",
			Command:     "rustfmt",
			Args:        []string{},
			Extensions:  []string{"rs"},
			Description: "A tool for formatting Rust code according to style guidelines",
		},
		// JSON formatter
		{
			Name:        "jq",
			Command:     "jq",
			Args:        []string{"."},
			Extensions:  []string{"json"},
			Description: "Command-line JSON processor with formatting capabilities",
		},
		// YAML formatter
		{
			Name:        "yamlfmt",
			Command:     "yamlfmt",
			Args:        []string{},
			Extensions:  []string{"yaml", "yml"},
			Description: "An extensible command line tool or library to format yaml files",
		},
	}
}

// DiscoverFormatters scans the system for available formatters
func DiscoverFormatters() *FormatterDiscovery {
	formatters := GetSupportedFormatters()

	for i := range formatters {
		formatter := &formatters[i]
		formatter.Available = isFormatterAvailable(formatter.Command)

		if formatter.Available {
			formatter.Version = getFormatterVersion(formatter.Command)
		}
	}

	return &FormatterDiscovery{
		Formatters: formatters,
		Timestamp:  time.Now().Format(time.RFC3339),
	}
}

// isFormatterAvailable checks if a formatter command is available in PATH
func isFormatterAvailable(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}

// getFormatterVersion attempts to get the version of a formatter
func getFormatterVersion(command string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try common version flags
	versionFlags := []string{"--version", "-version", "-V", "version"}

	for _, flag := range versionFlags {
		cmd := exec.CommandContext(ctx, command, flag) // #nosec G204 - command is validated above
		output, err := cmd.Output()
		if err == nil && len(output) > 0 {
			// Return first line of version output, trimmed
			lines := strings.Split(strings.TrimSpace(string(output)), "\n")
			if len(lines) > 0 {
				return strings.TrimSpace(lines[0])
			}
		}
	}

	return "unknown"
}

// GetFormattersForFile returns available formatters for a specific file
func GetFormattersForFile(filePath string) []Formatter {
	ext := getFileExtension(filePath)
	discovery := DiscoverFormatters()
	var matchingFormatters []Formatter

	for _, formatter := range discovery.Formatters {
		if !formatter.Available {
			continue
		}

		for _, supportedExt := range formatter.Extensions {
			if ext == supportedExt {
				matchingFormatters = append(matchingFormatters, formatter)
				break
			}
		}
	}

	return matchingFormatters
}

// getFileExtension extracts the file extension from a file path
func getFileExtension(filePath string) string {
	ext := filepath.Ext(filePath)
	if len(ext) > 0 && ext[0] == '.' {
		ext = ext[1:] // Remove the leading dot
	}
	return strings.ToLower(ext)
}

// FormatFile applies appropriate formatting to a file based on its extension
func FormatFile(filePath string) *FormatResult {
	start := time.Now()
	result := &FormatResult{
		File:     filePath,
		Duration: time.Since(start).String(),
	}

	formatters := GetFormattersForFile(filePath)
	if len(formatters) == 0 {
		result.Success = true // No formatter available is not an error
		result.Error = "no formatter available for this file type"
		return result
	}

	// Use the first available formatter (priority can be implemented later)
	formatter := formatters[0]
	result.Formatter = formatter.Name

	// Execute the formatter
	if err := executeFormatter(formatter, filePath, result); err != nil {
		result.Success = false
		result.Error = err.Error()
	} else {
		result.Success = true
	}

	result.Duration = time.Since(start).String()
	return result
}

// executeFormatter runs a formatter on a file and captures the output
func executeFormatter(formatter Formatter, filePath string, result *FormatResult) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Validate formatter command to prevent command injection
	matched, err := regexp.MatchString(`^[a-zA-Z0-9_\-]+$`, formatter.Command)
	if err != nil {
		return fmt.Errorf("error validating formatter command: %w", err)
	}
	if !matched {
		return fmt.Errorf("invalid formatter command: %s", formatter.Command)
	}

	// Validate file path to prevent command injection
	cleanPath := filepath.Clean(filePath)
	dangerousChars := []string{"..", ";", "&", "|", "`", "$"}
	for _, char := range dangerousChars {
		if strings.Contains(cleanPath, char) {
			return fmt.Errorf("invalid file path: %s", filePath)
		}
	}

	// Build command arguments with validated path
	args := append(formatter.Args, cleanPath)

	// Special handling for jq (JSON formatter) - needs different approach
	if formatter.Name == "jq" {
		return executeJQFormatter(ctx, cleanPath, result)
	}

	cmd := exec.CommandContext(ctx, formatter.Command, args...) // #nosec G204 - formatter.Command is validated above

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	// Capture output
	result.Stdout = strings.TrimSpace(stdout.String())
	result.Stderr = strings.TrimSpace(stderr.String())

	if err != nil {
		return fmt.Errorf("formatter %s failed: %w", formatter.Name, err)
	}

	return nil
}

// executeJQFormatter handles JSON formatting with jq (special case)
func executeJQFormatter(ctx context.Context, filePath string, result *FormatResult) error {
	// Read the file content
	content, err := os.ReadFile(filePath) // #nosec G304 - filePath is validated in calling function
	if err != nil {
		return fmt.Errorf("failed to read JSON file: %w", err)
	}

	// Validate JSON first
	var jsonData interface{}
	if err := json.Unmarshal(content, &jsonData); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	// Format using jq
	cmd := exec.CommandContext(ctx, "jq", ".", filePath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	result.Stdout = strings.TrimSpace(stdout.String())
	result.Stderr = strings.TrimSpace(stderr.String())

	if err != nil {
		return fmt.Errorf("jq formatting failed: %w", err)
	}

	// Write formatted content back to file
	if err := os.WriteFile(filePath, stdout.Bytes(), 0600); err != nil {
		return fmt.Errorf("failed to write formatted JSON: %w", err)
	}

	return nil
}

// FormatFiles formats multiple files and returns a comprehensive result
func FormatFiles(filePaths []string) *FormatCommand {
	start := time.Now()
	result := &FormatCommand{
		Timestamp:           time.Now().Format(time.RFC3339),
		AvailableFormatters: DiscoverFormatters().Formatters,
	}

	result.FilesProcessed = len(filePaths)

	for _, filePath := range filePaths {
		formatResult := FormatFile(filePath)
		result.Results = append(result.Results, *formatResult)

		if formatResult.Success {
			result.FilesFormatted++
		} else {
			result.FilesFailed++
		}
	}

	result.Success = result.FilesFailed == 0
	result.Duration = time.Since(start).String()

	return result
}

// FormatFilesWithOptions formats files with custom options
func FormatFilesWithOptions(filePaths []string, options FormatOptions) *FormatCommand {
	start := time.Now()
	result := &FormatCommand{
		Timestamp:           time.Now().Format(time.RFC3339),
		AvailableFormatters: DiscoverFormatters().Formatters,
	}

	result.FilesProcessed = len(filePaths)

	// Set defaults
	if options.Concurrency <= 0 {
		options.Concurrency = 1 // Sequential by default for safety
	}
	if options.Timeout == 0 {
		options.Timeout = 30 * time.Second
	}

	// Process files
	if options.Concurrency == 1 {
		// Sequential processing
		for _, filePath := range filePaths {
			formatResult := formatFileWithOptions(filePath, options)
			result.Results = append(result.Results, *formatResult)

			if formatResult.Success {
				result.FilesFormatted++
			} else {
				result.FilesFailed++
			}
		}
	} else {
		// Concurrent processing (if requested)
		resultChan := make(chan *FormatResult, len(filePaths))
		semaphore := make(chan struct{}, options.Concurrency)

		for _, filePath := range filePaths {
			go func(fp string) {
				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				formatResult := formatFileWithOptions(fp, options)
				resultChan <- formatResult
			}(filePath)
		}

		// Collect results
		for i := 0; i < len(filePaths); i++ {
			formatResult := <-resultChan
			result.Results = append(result.Results, *formatResult)

			if formatResult.Success {
				result.FilesFormatted++
			} else {
				result.FilesFailed++
			}
		}
	}

	result.Success = result.FilesFailed == 0
	result.Duration = time.Since(start).String()

	return result
}

// formatFileWithOptions formats a file with custom options
func formatFileWithOptions(filePath string, options FormatOptions) *FormatResult {
	start := time.Now()
	result := &FormatResult{
		File:     filePath,
		Duration: time.Since(start).String(),
	}

	if options.DryRun {
		result.Success = true
		result.Error = "dry run mode - no changes made"
		result.Duration = time.Since(start).String()
		return result
	}

	formatters := GetFormattersForFile(filePath)
	if len(formatters) == 0 {
		result.Success = true
		result.Error = "no formatter available for this file type"
		result.Duration = time.Since(start).String()
		return result
	}

	// Filter formatters based on options
	formatters = filterFormatters(formatters, options)

	if len(formatters) == 0 {
		result.Success = true
		result.Error = "no suitable formatter after applying filters"
		result.Duration = time.Since(start).String()
		return result
	}

	// Use the first available formatter after filtering
	formatter := formatters[0]
	result.Formatter = formatter.Name

	// Execute the formatter with custom timeout
	ctx, cancel := context.WithTimeout(context.Background(), options.Timeout)
	defer cancel()

	if err := executeFormatterWithContext(ctx, formatter, filePath, result); err != nil {
		result.Success = false
		result.Error = err.Error()
	} else {
		result.Success = true
	}

	result.Duration = time.Since(start).String()
	return result
}

// filterFormatters filters formatters based on options
func filterFormatters(formatters []Formatter, options FormatOptions) []Formatter {
	var filtered []Formatter

	// Create maps for quick lookup
	preferredMap := make(map[string]bool)
	for _, name := range options.PreferredFormatters {
		preferredMap[name] = true
	}

	excludedMap := make(map[string]bool)
	for _, name := range options.ExcludeFormatters {
		excludedMap[name] = true
	}

	// If preferred formatters are specified, prioritize them
	if len(options.PreferredFormatters) > 0 {
		for _, formatter := range formatters {
			if preferredMap[formatter.Name] && !excludedMap[formatter.Name] {
				filtered = append(filtered, formatter)
			}
		}
		if len(filtered) > 0 {
			return filtered
		}
	}

	// Otherwise, use all non-excluded formatters
	for _, formatter := range formatters {
		if !excludedMap[formatter.Name] {
			filtered = append(filtered, formatter)
		}
	}

	return filtered
}

// executeFormatterWithContext runs a formatter with a custom context
func executeFormatterWithContext(
	ctx context.Context, 
	formatter Formatter, 
	filePath string, 
	result *FormatResult,
) error {
	// Validate formatter command to prevent command injection
	matched, err := regexp.MatchString(`^[a-zA-Z0-9_\-]+$`, formatter.Command)
	if err != nil {
		return fmt.Errorf("error validating formatter command: %w", err)
	}
	if !matched {
		return fmt.Errorf("invalid formatter command: %s", formatter.Command)
	}

	// Validate file path to prevent command injection
	cleanPath := filepath.Clean(filePath)
	dangerousChars := []string{"..", ";", "&", "|", "`", "$"}
	for _, char := range dangerousChars {
		if strings.Contains(cleanPath, char) {
			return fmt.Errorf("invalid file path: %s", filePath)
		}
	}

	// Build command arguments with validated path
	args := append(formatter.Args, cleanPath)

	// Special handling for jq (JSON formatter)
	if formatter.Name == "jq" {
		return executeJQFormatter(ctx, cleanPath, result)
	}

	cmd := exec.CommandContext(ctx, formatter.Command, args...) // #nosec G204 - formatter.Command is validated above

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	// Capture output
	result.Stdout = strings.TrimSpace(stdout.String())
	result.Stderr = strings.TrimSpace(stderr.String())

	if err != nil {
		return fmt.Errorf("formatter %s failed: %w", formatter.Name, err)
	}

	return nil
}

// GetSupportedExtensions returns a list of file extensions that can be formatted
func GetSupportedExtensions() []string {
	var extensions []string
	extMap := make(map[string]bool)

	for _, formatter := range GetSupportedFormatters() {
		for _, ext := range formatter.Extensions {
			if !extMap[ext] {
				extensions = append(extensions, ext)
				extMap[ext] = true
			}
		}
	}

	return extensions
}

// GetAvailableExtensions returns extensions for which formatters are actually available
func GetAvailableExtensions() []string {
	discovery := DiscoverFormatters()
	var extensions []string
	extMap := make(map[string]bool)

	for _, formatter := range discovery.Formatters {
		if !formatter.Available {
			continue
		}

		for _, ext := range formatter.Extensions {
			if !extMap[ext] {
				extensions = append(extensions, ext)
				extMap[ext] = true
			}
		}
	}

	return extensions
}
