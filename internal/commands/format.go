package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/NeuBlink/syncwright/internal/format"
	"github.com/NeuBlink/syncwright/internal/gitutils"
)

// FormatOptions contains options for the format command
type FormatOptions struct {
	RepoPath            string               `json:"repo_path"`
	OutputFile          string               `json:"output_file"`
	OutputFormat        string               `json:"output_format"` // "json", "text"
	DryRun              bool                 `json:"dry_run"`
	Verbose             bool                 `json:"verbose"`
	PreferredFormatters []string             `json:"preferred_formatters,omitempty"`
	ExcludeFormatters   []string             `json:"exclude_formatters,omitempty"`
	IncludeExtensions   []string             `json:"include_extensions,omitempty"`
	ExcludeExtensions   []string             `json:"exclude_extensions,omitempty"`
	FilePaths           []string             `json:"file_paths,omitempty"`
	ScanRecent          bool                 `json:"scan_recent"`
	RecentDays          int                  `json:"recent_days"`
	FormatOptions       format.FormatOptions `json:"format_options"`
}

// FormatResult represents the result of the format command
type FormatResult struct {
	Success       bool                       `json:"success"`
	FormatCommand *format.FormatCommand      `json:"format_command,omitempty"`
	ErrorMessage  string                     `json:"error_message,omitempty"`
	Summary       FormatSummary              `json:"summary"`
	Discovery     *format.FormatterDiscovery `json:"discovery,omitempty"`
}

// FormatSummary provides a summary of the formatting results
type FormatSummary struct {
	FilesScanned        int      `json:"files_scanned"`
	FilesProcessed      int      `json:"files_processed"`
	FilesFormatted      int      `json:"files_formatted"`
	FilesFailed         int      `json:"files_failed"`
	AvailableFormatters int      `json:"available_formatters"`
	SupportedExtensions []string `json:"supported_extensions"`
	RepoPath            string   `json:"repo_path"`
	Timestamp           string   `json:"timestamp"`
	Duration            string   `json:"duration"`
}

// FormatCommand implements the format subcommand
type FormatCommand struct {
	options FormatOptions
}

// NewFormatCommand creates a new format command
func NewFormatCommand(options FormatOptions) *FormatCommand {
	// Set defaults
	if options.OutputFormat == "" {
		options.OutputFormat = "json"
	}
	if options.RecentDays == 0 {
		options.RecentDays = 7 // Default to last 7 days
	}
	if options.RepoPath == "" {
		if wd, err := os.Getwd(); err == nil {
			options.RepoPath = wd
		}
	}

	return &FormatCommand{
		options: options,
	}
}

// Execute runs the format command
func (f *FormatCommand) Execute() (*FormatResult, error) {
	start := time.Now()
	result := &FormatResult{
		Summary: FormatSummary{
			RepoPath:  f.options.RepoPath,
			Timestamp: time.Now().Format(time.RFC3339),
		},
	}

	// Discover available formatters
	discovery := format.DiscoverFormatters()
	result.Discovery = discovery

	availableCount := 0
	for _, formatter := range discovery.Formatters {
		if formatter.Available {
			availableCount++
		}
	}
	result.Summary.AvailableFormatters = availableCount
	result.Summary.SupportedExtensions = format.GetAvailableExtensions()

	if f.options.Verbose {
		fmt.Printf("Found %d available formatters\n", availableCount)
		fmt.Printf("Supported extensions: %s\n", strings.Join(result.Summary.SupportedExtensions, ", "))
	}

	// Determine which files to format
	filesToFormat, err := f.getFilesToFormat()
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to determine files to format: %v", err)
		result.Summary.Duration = time.Since(start).String()
		return result, err
	}

	result.Summary.FilesScanned = len(filesToFormat)

	if f.options.Verbose {
		fmt.Printf("Found %d files to scan for formatting\n", len(filesToFormat))
	}

	// Filter files by supported extensions
	formattableFiles := f.filterFormattableFiles(filesToFormat)
	result.Summary.FilesProcessed = len(formattableFiles)

	if f.options.Verbose && len(formattableFiles) != len(filesToFormat) {
		fmt.Printf("Filtered to %d formattable files\n", len(formattableFiles))
	}

	if len(formattableFiles) == 0 {
		result.Success = true
		result.ErrorMessage = "No formattable files found"
		result.Summary.Duration = time.Since(start).String()
		return result, nil
	}

	// Format the files
	formatCommand := format.FormatFilesWithOptions(formattableFiles, f.options.FormatOptions)
	result.FormatCommand = formatCommand
	result.Summary.FilesFormatted = formatCommand.FilesFormatted
	result.Summary.FilesFailed = formatCommand.FilesFailed

	if f.options.Verbose {
		fmt.Printf("Formatted %d files successfully, %d failures\n",
			formatCommand.FilesFormatted, formatCommand.FilesFailed)
	}

	result.Success = formatCommand.Success
	result.Summary.Duration = time.Since(start).String()

	// Output results
	if err := f.outputResults(result); err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to output results: %v", err)
		return result, err
	}

	return result, nil
}

// getFilesToFormat determines which files should be formatted
func (f *FormatCommand) getFilesToFormat() ([]string, error) {
	var files []string

	// If specific file paths are provided, use those
	if len(f.options.FilePaths) > 0 {
		for _, filePath := range f.options.FilePaths {
			if !filepath.IsAbs(filePath) {
				filePath = filepath.Join(f.options.RepoPath, filePath)
			}

			// Check if file exists
			if _, err := os.Stat(filePath); err == nil {
				files = append(files, filePath)
			} else if f.options.Verbose {
				fmt.Printf("Warning: File not found: %s\n", filePath)
			}
		}
		return files, nil
	}

	// If scanning recent files is requested, get recently modified files
	if f.options.ScanRecent {
		return f.getRecentlyModifiedFiles()
	}

	// Default: get all files in the repository (filtered by git)
	return f.getAllRepositoryFiles()
}

// getRecentlyModifiedFiles gets files that have been modified recently
func (f *FormatCommand) getRecentlyModifiedFiles() ([]string, error) {
	// Check if it's a git repository
	if !gitutils.IsGitRepositoryPath(f.options.RepoPath) {
		if f.options.Verbose {
			fmt.Printf("Not a git repository, scanning all files in directory\n")
		}
		return f.getAllFilesInDirectory()
	}

	// Get recently modified files from git
	files, err := gitutils.GetRecentlyModifiedFiles(f.options.RepoPath, f.options.RecentDays)
	if err != nil {
		return nil, fmt.Errorf("failed to get recently modified files: %w", err)
	}

	// Convert to absolute paths
	var absPaths []string
	for _, file := range files {
		absPath := filepath.Join(f.options.RepoPath, file)
		absPaths = append(absPaths, absPath)
	}

	return absPaths, nil
}

// getAllRepositoryFiles gets all files in the repository
func (f *FormatCommand) getAllRepositoryFiles() ([]string, error) {
	// Check if it's a git repository
	if !gitutils.IsGitRepositoryPath(f.options.RepoPath) {
		if f.options.Verbose {
			fmt.Printf("Not a git repository, scanning all files in directory\n")
		}
		return f.getAllFilesInDirectory()
	}

	// Get all tracked files from git
	files, err := gitutils.GetAllTrackedFiles(f.options.RepoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get tracked files: %w", err)
	}

	// Convert to absolute paths
	var absPaths []string
	for _, file := range files {
		absPath := filepath.Join(f.options.RepoPath, file)
		absPaths = append(absPaths, absPath)
	}

	return absPaths, nil
}

// getAllFilesInDirectory recursively gets all files in a directory
func (f *FormatCommand) getAllFilesInDirectory() ([]string, error) {
	var files []string

	err := filepath.Walk(f.options.RepoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Skip hidden files and directories
		if strings.HasPrefix(filepath.Base(path), ".") {
			return nil
		}

		files = append(files, path)
		return nil
	})

	return files, err
}

// filterFormattableFiles filters files to only include those that can be formatted
func (f *FormatCommand) filterFormattableFiles(files []string) []string {
	filter := &fileFilter{
		availableExtensions: f.buildAvailableExtensionsMap(),
		includeExtensions:   f.options.IncludeExtensions,
		excludeExtensions:   f.options.ExcludeExtensions,
	}

	var formattableFiles []string
	for _, file := range files {
		if filter.isFormattable(file) {
			formattableFiles = append(formattableFiles, file)
		}
	}

	return formattableFiles
}

// fileFilter handles filtering logic for formattable files
type fileFilter struct {
	availableExtensions map[string]bool
	includeExtensions   []string
	excludeExtensions   []string
}

// isFormattable checks if a file can be formatted based on its extension
func (f *fileFilter) isFormattable(filePath string) bool {
	ext := f.extractExtension(filePath)

	// Check if extension is supported
	supported := f.availableExtensions[ext]

	// Apply include/exclude filters
	supported = f.applyIncludeFilter(ext, supported)
	supported = f.applyExcludeFilter(ext, supported)

	return supported
}

// extractExtension extracts and normalizes the file extension
func (f *fileFilter) extractExtension(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	if len(ext) > 0 && ext[0] == '.' {
		ext = ext[1:] // Remove the leading dot
	}
	return ext
}

// applyIncludeFilter applies include extension filters
func (f *fileFilter) applyIncludeFilter(ext string, currentSupported bool) bool {
	if len(f.includeExtensions) == 0 {
		return currentSupported
	}

	for _, includeExt := range f.includeExtensions {
		if ext == includeExt {
			return currentSupported
		}
	}
	return false
}

// applyExcludeFilter applies exclude extension filters
func (f *fileFilter) applyExcludeFilter(ext string, currentSupported bool) bool {
	if !currentSupported || len(f.excludeExtensions) == 0 {
		return currentSupported
	}

	for _, excludeExt := range f.excludeExtensions {
		if ext == excludeExt {
			return false
		}
	}
	return currentSupported
}

// buildAvailableExtensionsMap creates a map of available extensions for quick lookup
func (f *FormatCommand) buildAvailableExtensionsMap() map[string]bool {
	availableExtensions := format.GetAvailableExtensions()
	extMap := make(map[string]bool)
	for _, ext := range availableExtensions {
		extMap[ext] = true
	}
	return extMap
}

// outputResults outputs the formatting results in the specified format
func (f *FormatCommand) outputResults(result *FormatResult) error {
	var output []byte
	var err error

	switch f.options.OutputFormat {
	case "json":
		output, err = json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
	case "text":
		output = f.formatTextOutput(result)
	default:
		return fmt.Errorf("unsupported output format: %s", f.options.OutputFormat)
	}

	// Write to file or stdout
	if f.options.OutputFile != "" {
		err = os.WriteFile(f.options.OutputFile, output, 0600)
		if err != nil {
			return fmt.Errorf("failed to write to file %s: %w", f.options.OutputFile, err)
		}
		if f.options.Verbose {
			fmt.Printf("Results written to: %s\n", f.options.OutputFile)
		}
	} else {
		fmt.Print(string(output))
	}

	return nil
}

// formatTextOutput formats the results as human-readable text
func (f *FormatCommand) formatTextOutput(result *FormatResult) []byte {
	formatter := &textOutputFormatter{verbose: f.options.Verbose}
	return formatter.formatResult(result)
}

// textOutputFormatter handles formatting of FormatResult as human-readable text
type textOutputFormatter struct {
	verbose bool
}

// formatResult formats the complete result
func (t *textOutputFormatter) formatResult(result *FormatResult) []byte {
	var output []string

	output = append(output, "=== Syncwright Format Report ===", "")

	if !result.Success {
		output = append(output, fmt.Sprintf("❌ Error: %s", result.ErrorMessage))
		return []byte(strings.Join(output, "\n") + "\n")
	}

	output = append(output, t.formatHeader(result)...)
	output = append(output, t.formatSummary(result)...)
	output = append(output, t.formatDiscovery(result)...)
	output = append(output, t.formatResults(result)...)
	output = append(output, t.formatNoFilesMessage(result)...)

	return []byte(strings.Join(output, "\n") + "\n")
}

// formatHeader formats the header section
func (t *textOutputFormatter) formatHeader(result *FormatResult) []string {
	return []string{
		fmt.Sprintf("Repository: %s", result.Summary.RepoPath),
		fmt.Sprintf("Duration: %s", result.Summary.Duration),
		"",
	}
}

// formatSummary formats the summary section
func (t *textOutputFormatter) formatSummary(result *FormatResult) []string {
	return []string{
		"📊 Summary:",
		fmt.Sprintf("  Files scanned: %d", result.Summary.FilesScanned),
		fmt.Sprintf("  Files processed: %d", result.Summary.FilesProcessed),
		fmt.Sprintf("  Files formatted: %d", result.Summary.FilesFormatted),
		fmt.Sprintf("  Files failed: %d", result.Summary.FilesFailed),
		fmt.Sprintf("  Available formatters: %d", result.Summary.AvailableFormatters),
		"",
	}
}

// formatDiscovery formats the formatter discovery section
func (t *textOutputFormatter) formatDiscovery(result *FormatResult) []string {
	if result.Discovery == nil {
		return []string{}
	}

	var output []string
	output = append(output, "🔧 Available Formatters:")

	for _, formatter := range result.Discovery.Formatters {
		if formatter.Available {
			output = append(output, t.formatAvailableFormatter(formatter)...)
		} else if t.verbose {
			output = append(output, fmt.Sprintf("  ❌ %s - %s (not available)", formatter.Name, formatter.Description))
		}
	}

	return append(output, "")
}

// formatAvailableFormatter formats a single available formatter
func (t *textOutputFormatter) formatAvailableFormatter(formatter format.Formatter) []string {
	version := formatter.Version
	if version == "unknown" {
		version = ""
	} else {
		version = fmt.Sprintf(" (%s)", version)
	}

	return []string{
		fmt.Sprintf("  ✅ %s%s - %s", formatter.Name, version, formatter.Description),
		fmt.Sprintf("     Extensions: %s", strings.Join(formatter.Extensions, ", ")),
	}
}

// formatResults formats the file processing results section
func (t *textOutputFormatter) formatResults(result *FormatResult) []string {
	if result.FormatCommand == nil || len(result.FormatCommand.Results) == 0 {
		return []string{}
	}

	var output []string

	if result.Summary.FilesFormatted > 0 {
		output = append(output, t.formatSuccessfulFiles(result)...)
	}

	if result.Summary.FilesFailed > 0 {
		output = append(output, t.formatFailedFiles(result)...)
	}

	return output
}

// formatSuccessfulFiles formats successfully formatted files
func (t *textOutputFormatter) formatSuccessfulFiles(result *FormatResult) []string {
	var output []string
	output = append(output, "✅ Successfully Formatted Files:")

	for _, fileResult := range result.FormatCommand.Results {
		if fileResult.Success && fileResult.Formatter != "" {
			output = append(output, fmt.Sprintf("  %s (%s)", fileResult.File, fileResult.Formatter))
		}
	}

	return append(output, "")
}

// formatFailedFiles formats files that failed to format
func (t *textOutputFormatter) formatFailedFiles(result *FormatResult) []string {
	var output []string
	output = append(output, "❌ Failed Files:")

	for _, fileResult := range result.FormatCommand.Results {
		if !fileResult.Success {
			output = append(output, fmt.Sprintf("  %s: %s", fileResult.File, fileResult.Error))
			if t.verbose && fileResult.Stderr != "" {
				output = append(output, fmt.Sprintf("    stderr: %s", fileResult.Stderr))
			}
		}
	}

	return append(output, "")
}

// formatNoFilesMessage formats the message when no files were processed
func (t *textOutputFormatter) formatNoFilesMessage(result *FormatResult) []string {
	if result.Summary.FilesFormatted == 0 && result.Summary.FilesFailed == 0 {
		return []string{"ℹ️  No files required formatting"}
	}
	return []string{}
}

// FormatFiles is a convenience function for simple file formatting
func FormatFiles(repoPath string, filePaths []string) (*FormatResult, error) {
	options := FormatOptions{
		RepoPath:     repoPath,
		FilePaths:    filePaths,
		OutputFormat: "json",
		Verbose:      false,
	}

	cmd := NewFormatCommand(options)
	return cmd.Execute()
}

// FormatRecentFiles is a convenience function for formatting recently modified files
func FormatRecentFiles(repoPath string, days int) (*FormatResult, error) {
	options := FormatOptions{
		RepoPath:     repoPath,
		OutputFormat: "json",
		ScanRecent:   true,
		RecentDays:   days,
		Verbose:      false,
	}

	cmd := NewFormatCommand(options)
	return cmd.Execute()
}

// FormatAllFiles is a convenience function for formatting all files in a repository
func FormatAllFiles(repoPath string) (*FormatResult, error) {
	options := FormatOptions{
		RepoPath:     repoPath,
		OutputFormat: "json",
		ScanRecent:   false,
		Verbose:      false,
	}

	cmd := NewFormatCommand(options)
	return cmd.Execute()
}
