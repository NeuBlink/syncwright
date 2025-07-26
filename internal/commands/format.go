package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"syncwright/internal/format"
	"syncwright/internal/gitutils"
)

// FormatOptions contains options for the format command
type FormatOptions struct {
	RepoPath            string        `json:"repo_path"`
	OutputFile          string        `json:"output_file"`
	OutputFormat        string        `json:"output_format"` // "json", "text"
	DryRun              bool          `json:"dry_run"`
	Verbose             bool          `json:"verbose"`
	PreferredFormatters []string      `json:"preferred_formatters,omitempty"`
	ExcludeFormatters   []string      `json:"exclude_formatters,omitempty"`
	IncludeExtensions   []string      `json:"include_extensions,omitempty"`
	ExcludeExtensions   []string      `json:"exclude_extensions,omitempty"`
	FilePaths           []string      `json:"file_paths,omitempty"`
	ScanRecent          bool          `json:"scan_recent"`
	RecentDays          int           `json:"recent_days"`
	FormatOptions       format.FormatOptions `json:"format_options"`
}

// FormatResult represents the result of the format command
type FormatResult struct {
	Success         bool                    `json:"success"`
	FormatCommand   *format.FormatCommand   `json:"format_command,omitempty"`
	ErrorMessage    string                  `json:"error_message,omitempty"`
	Summary         FormatSummary           `json:"summary"`
	Discovery       *format.FormatterDiscovery `json:"discovery,omitempty"`
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
	availableExtensions := format.GetAvailableExtensions()
	extMap := make(map[string]bool)
	for _, ext := range availableExtensions {
		extMap[ext] = true
	}

	var formattableFiles []string

	for _, file := range files {
		ext := strings.ToLower(filepath.Ext(file))
		if len(ext) > 0 && ext[0] == '.' {
			ext = ext[1:] // Remove the leading dot
		}

		// Check if extension is supported
		supported := extMap[ext]

		// Apply include/exclude filters
		if len(f.options.IncludeExtensions) > 0 {
			included := false
			for _, includeExt := range f.options.IncludeExtensions {
				if ext == includeExt {
					included = true
					break
				}
			}
			supported = supported && included
		}

		if len(f.options.ExcludeExtensions) > 0 {
			for _, excludeExt := range f.options.ExcludeExtensions {
				if ext == excludeExt {
					supported = false
					break
				}
			}
		}

		if supported {
			formattableFiles = append(formattableFiles, file)
		}
	}

	return formattableFiles
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
		err = os.WriteFile(f.options.OutputFile, output, 0644)
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
	var output []string

	output = append(output, "=== Syncwright Format Report ===")
	output = append(output, "")

	if !result.Success {
		output = append(output, fmt.Sprintf("❌ Error: %s", result.ErrorMessage))
		return []byte(strings.Join(output, "\n") + "\n")
	}

	output = append(output, fmt.Sprintf("Repository: %s", result.Summary.RepoPath))
	output = append(output, fmt.Sprintf("Duration: %s", result.Summary.Duration))
	output = append(output, "")

	output = append(output, "📊 Summary:")
	output = append(output, fmt.Sprintf("  Files scanned: %d", result.Summary.FilesScanned))
	output = append(output, fmt.Sprintf("  Files processed: %d", result.Summary.FilesProcessed))
	output = append(output, fmt.Sprintf("  Files formatted: %d", result.Summary.FilesFormatted))
	output = append(output, fmt.Sprintf("  Files failed: %d", result.Summary.FilesFailed))
	output = append(output, fmt.Sprintf("  Available formatters: %d", result.Summary.AvailableFormatters))
	output = append(output, "")

	if result.Discovery != nil {
		output = append(output, "🔧 Available Formatters:")
		for _, formatter := range result.Discovery.Formatters {
			if formatter.Available {
				version := formatter.Version
				if version == "unknown" {
					version = ""
				} else {
					version = fmt.Sprintf(" (%s)", version)
				}
				output = append(output, fmt.Sprintf("  ✅ %s%s - %s", formatter.Name, version, formatter.Description))
				output = append(output, fmt.Sprintf("     Extensions: %s", strings.Join(formatter.Extensions, ", ")))
			} else if f.options.Verbose {
				output = append(output, fmt.Sprintf("  ❌ %s - %s (not available)", formatter.Name, formatter.Description))
			}
		}
		output = append(output, "")
	}

	if result.FormatCommand != nil && len(result.FormatCommand.Results) > 0 {
		if result.Summary.FilesFormatted > 0 {
			output = append(output, "✅ Successfully Formatted Files:")
			for _, fileResult := range result.FormatCommand.Results {
				if fileResult.Success && fileResult.Formatter != "" {
					output = append(output, fmt.Sprintf("  %s (%s)", fileResult.File, fileResult.Formatter))
				}
			}
			output = append(output, "")
		}

		if result.Summary.FilesFailed > 0 {
			output = append(output, "❌ Failed Files:")
			for _, fileResult := range result.FormatCommand.Results {
				if !fileResult.Success {
					output = append(output, fmt.Sprintf("  %s: %s", fileResult.File, fileResult.Error))
					if f.options.Verbose && fileResult.Stderr != "" {
						output = append(output, fmt.Sprintf("    stderr: %s", fileResult.Stderr))
					}
				}
			}
			output = append(output, "")
		}
	}

	if result.Summary.FilesFormatted == 0 && result.Summary.FilesFailed == 0 {
		output = append(output, "ℹ️  No files required formatting")
	}

	return []byte(strings.Join(output, "\n") + "\n")
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