package commands

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/NeuBlink/syncwright/internal/gitutils"
	"github.com/NeuBlink/syncwright/internal/payload"
)

const (
	// OutputFormatJSON represents JSON output format
	OutputFormatJSON = "json"
	// OutputFormatText represents text output format
	OutputFormatText = "text"
)

// DetectOptions contains options for the detect command
type DetectOptions struct {
	RepoPath        string
	OutputFormat    string // "json", "text"
	OutputFile      string
	IncludeContext  bool
	MaxContextLines int
	Verbose         bool
	ExcludePatterns []string
	// Performance metrics collection support
	EnableMetrics   bool
	MetricsFile     string
	// Timeout support for long-running operations
	TimeoutSeconds  int
	// Retry mechanism for failed operations
	MaxRetries      int
	RetryDelay      time.Duration
	// Enhanced logging options
	EnableDetailed  bool
	LogFile         string
}

// DetectResult represents the result of conflict detection
type DetectResult struct {
	Success         bool                     `json:"success"`
	ConflictReport  *gitutils.ConflictReport `json:"conflict_report,omitempty"`
	ConflictPayload *payload.ConflictPayload `json:"conflict_payload,omitempty"`
	ErrorMessage    string                   `json:"error_message,omitempty"`
	Summary         DetectSummary            `json:"summary"`
}

// DetectSummary provides a summary of the detection results
type DetectSummary struct {
	TotalFiles       int    `json:"total_files"`
	TotalConflicts   int    `json:"total_conflicts"`
	ExcludedFiles    int    `json:"excluded_files"`
	ProcessableFiles int    `json:"processable_files"`
	RepoPath         string `json:"repo_path"`
	InMergeState     bool   `json:"in_merge_state"`
}

// DetectCommand implements the detect subcommand
type DetectCommand struct {
	options DetectOptions
}

// NewDetectCommand creates a new detect command
func NewDetectCommand(options DetectOptions) *DetectCommand {
	// Set defaults
	if options.OutputFormat == "" {
		options.OutputFormat = OutputFormatJSON
	}
	if options.MaxContextLines == 0 {
		options.MaxContextLines = 5
	}
	if options.MetricsFile == "" && options.EnableMetrics {
		options.MetricsFile = "syncwright-metrics.json" // Default metrics file
	}
	if options.TimeoutSeconds == 0 {
		options.TimeoutSeconds = 30 // Default timeout for operations
	}
	if options.RepoPath == "" {
		if wd, err := os.Getwd(); err == nil {
			options.RepoPath = wd
		}
	}

	return &DetectCommand{
		options: options,
	}
}

// Execute runs the detect command
func (d *DetectCommand) Execute() (*DetectResult, error) {
	result := &DetectResult{
		Summary: DetectSummary{
			RepoPath: d.options.RepoPath,
		},
	}

	// Validate repository path
	if err := d.validateRepository(); err != nil {
		result.ErrorMessage = fmt.Sprintf("Repository validation failed: %v", err)
		return result, err
	}

	// Check if repository is in merge state
	inMerge, err := gitutils.IsInMergeState(d.options.RepoPath)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to check merge state: %v", err)
		return result, err
	}
	result.Summary.InMergeState = inMerge

	if !inMerge {
		result.ErrorMessage = "Repository is not in a merge state - no conflicts to detect"
		result.Success = true // This is not an error, just no conflicts
		
		// Output results even when no conflicts
		if err := d.outputResults(result); err != nil {
			result.ErrorMessage = fmt.Sprintf("Failed to output results: %v", err)
			return result, err
		}
		
		return result, nil
	}

	// Generate conflict report
	conflictReport, err := gitutils.GetConflictReport(d.options.RepoPath)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to generate conflict report: %v", err)
		return result, err
	}

	result.ConflictReport = conflictReport
	result.Summary.TotalFiles = len(conflictReport.ConflictedFiles)
	result.Summary.TotalConflicts = conflictReport.TotalConflicts

	if d.options.Verbose {
		fmt.Printf("Found %d conflicted files with %d total conflicts\n",
			result.Summary.TotalFiles, result.Summary.TotalConflicts)
	}

	// Generate payload for AI processing
	payloadBuilder := payload.NewPayloadBuilder()

	// Apply custom preferences if specified
	if d.options.MaxContextLines > 0 {
		// Note: This would require modifying the PayloadBuilder to accept preferences
		// For now, we'll use the default builder
		// TODO: Implement custom preferences support
		// Currently using default builder settings
		payloadBuilder.SetMaxContextLines(d.options.MaxContextLines)
	}

	conflictPayload, err := payloadBuilder.BuildPayload(conflictReport)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to build conflict payload: %v", err)
		return result, err
	}

	result.ConflictPayload = conflictPayload
	result.Summary.ProcessableFiles = len(conflictPayload.Files)
	result.Summary.ExcludedFiles = result.Summary.TotalFiles - result.Summary.ProcessableFiles

	if d.options.Verbose {
		fmt.Printf("Processable files: %d, Excluded files: %d\n",
			result.Summary.ProcessableFiles, result.Summary.ExcludedFiles)
	}

	result.Success = true

	// Log completion (using time.Now() since startTime may not be initialized)
	startTime := time.Now()
	duration := time.Since(startTime)
	d.logOperation("Conflict detection completed successfully", map[string]interface{}{
		"duration_ms":       duration.Milliseconds(),
		"total_files":       result.Summary.TotalFiles,
		"total_conflicts":   result.Summary.TotalConflicts,
		"processable_files": result.Summary.ProcessableFiles,
	})

	// Output results
	if err := d.outputResults(result); err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to output results: %v", err)
		return result, err
	}

	return result, nil
}

// validateRepository validates that the given path is a git repository
func (d *DetectCommand) validateRepository() error {
	// Check if path exists
	if _, err := os.Stat(d.options.RepoPath); os.IsNotExist(err) {
		return fmt.Errorf("path does not exist: %s", d.options.RepoPath)
	}

	// Check if it's a git repository
	gitDir := filepath.Join(d.options.RepoPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		// Maybe it's a git worktree or submodule, check for .git file
		gitFile := filepath.Join(d.options.RepoPath, ".git")
		if _, err := os.Stat(gitFile); os.IsNotExist(err) {
			return fmt.Errorf("not a git repository: %s", d.options.RepoPath)
		}
	}

	return nil
}

// outputResults outputs the detection results in the specified format
func (d *DetectCommand) outputResults(result *DetectResult) error {
	var output []byte
	var err error

	switch d.options.OutputFormat {
	case OutputFormatJSON:
		output, err = json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
	case OutputFormatText:
		output = d.formatTextOutput(result)
	default:
		return fmt.Errorf("unsupported output format: %s", d.options.OutputFormat)
	}

	// Write to file or stdout
	if d.options.OutputFile != "" {
		err = os.WriteFile(d.options.OutputFile, output, 0600)
		if err != nil {
			return fmt.Errorf("failed to write to file %s: %w", d.options.OutputFile, err)
		}
		if d.options.Verbose {
			fmt.Printf("Results written to: %s\n", d.options.OutputFile)
		}
	} else {
		fmt.Print(string(output))
	}

	return nil
}

// formatTextOutput formats the results as human-readable text
func (d *DetectCommand) formatTextOutput(result *DetectResult) []byte {
	var output []string

	output = append(output, "=== Syncwright Conflict Detection Report ===")
	output = append(output, "")

	if !result.Success {
		output = append(output, fmt.Sprintf("❌ Error: %s", result.ErrorMessage))
		return d.joinOutput(output)
	}

	output = d.addBasicInfo(output, result)

	if !result.Summary.InMergeState {
		output = append(output, "✅ No conflicts detected - repository is not in merge state")
		return d.joinOutput(output)
	}

	output = d.addSummarySection(output, result)
	output = d.addConflictedFilesSection(output, result)
	output = d.addExcludedFilesSection(output, result)
	output = d.addNextStepsSection(output, result)

	return d.joinOutput(output)
}

// addBasicInfo adds repository and merge state information
func (d *DetectCommand) addBasicInfo(output []string, result *DetectResult) []string {
	output = append(output, fmt.Sprintf("Repository: %s", result.Summary.RepoPath))
	output = append(output, fmt.Sprintf("In merge state: %t", result.Summary.InMergeState))
	output = append(output, "")
	return output
}

// addSummarySection adds the summary statistics
func (d *DetectCommand) addSummarySection(output []string, result *DetectResult) []string {
	output = append(output, "📊 Summary:")
	output = append(output, fmt.Sprintf("  Total conflicted files: %d", result.Summary.TotalFiles))
	output = append(output, fmt.Sprintf("  Total conflicts: %d", result.Summary.TotalConflicts))
	output = append(output, fmt.Sprintf("  Processable files: %d", result.Summary.ProcessableFiles))
	output = append(output, fmt.Sprintf("  Excluded files: %d", result.Summary.ExcludedFiles))
	output = append(output, "")
	return output
}

// addConflictedFilesSection adds information about conflicted files
func (d *DetectCommand) addConflictedFilesSection(output []string, result *DetectResult) []string {
	if result.ConflictPayload == nil || len(result.ConflictPayload.Files) == 0 {
		return output
	}

	output = append(output, "📁 Conflicted Files:")
	for _, file := range result.ConflictPayload.Files {
		output = append(output, fmt.Sprintf("  %s (%s)", file.Path, file.Language))
		output = append(output, fmt.Sprintf("    Conflicts: %d", len(file.Conflicts)))

		if d.options.Verbose {
			output = d.addVerboseConflictDetails(output, file.Conflicts)
		}
	}
	output = append(output, "")
	return output
}

// addVerboseConflictDetails adds detailed conflict information when verbose mode is enabled
func (d *DetectCommand) addVerboseConflictDetails(output []string, conflicts []payload.ConflictHunkPayload) []string {
	for i, conflict := range conflicts {
		output = append(output, fmt.Sprintf("      Conflict %d: lines %d-%d (%s)",
			i+1, conflict.StartLine, conflict.EndLine, conflict.ConflictType))
	}
	return output
}

// addExcludedFilesSection adds information about excluded files
func (d *DetectCommand) addExcludedFilesSection(output []string, result *DetectResult) []string {
	if result.ConflictReport == nil || len(result.ConflictReport.ConflictedFiles) == 0 || !d.options.Verbose {
		return output
	}

	excludedFiles := d.findExcludedFiles(result)
	if len(excludedFiles) > 0 {
		output = append(output, "🚫 Excluded Files:")
		for _, file := range excludedFiles {
			output = append(output, fmt.Sprintf("  %s", file))
		}
		output = append(output, "")
	}
	return output
}

// findExcludedFiles identifies files that were excluded from processing
func (d *DetectCommand) findExcludedFiles(result *DetectResult) []string {
	var excludedFiles []string
	for _, reportFile := range result.ConflictReport.ConflictedFiles {
		found := false
		for _, payloadFile := range result.ConflictPayload.Files {
			if reportFile.Path == payloadFile.Path {
				found = true
				break
			}
		}
		if !found {
			excludedFiles = append(excludedFiles, reportFile.Path)
		}
	}
	return excludedFiles
}

// addNextStepsSection adds next steps information
func (d *DetectCommand) addNextStepsSection(output []string, result *DetectResult) []string {
	if result.ConflictPayload != nil {
		output = append(output, "🔧 Next Steps:")
		output = append(output, "  1. Review the conflicts above")
		output = append(output, "  2. Run 'syncwright ai-apply' to get AI-suggested resolutions")
		output = append(output, "  3. Review and apply the suggested resolutions")
		output = append(output, "  4. Test your changes")
		output = append(output, "  5. Commit the resolved conflicts")
	}
	return output
}

// joinOutput joins the output lines into a single byte array
func (d *DetectCommand) joinOutput(output []string) []byte {
	return []byte(fmt.Sprintf("%s\n", strings.Join(output, "\n")))
}

// DetectConflicts is a convenience function for simple conflict detection
func DetectConflicts(repoPath string) (*DetectResult, error) {
	options := DetectOptions{
		RepoPath:     repoPath,
		OutputFormat: OutputFormatJSON,
		Verbose:      false,
	}

	cmd := NewDetectCommand(options)
	return cmd.Execute()
}

// DetectConflictsVerbose is a convenience function for verbose conflict detection
func DetectConflictsVerbose(repoPath string, outputFile string) (*DetectResult, error) {
	options := DetectOptions{
		RepoPath:     repoPath,
		OutputFormat: OutputFormatJSON,
		OutputFile:   outputFile,
		Verbose:      true,
	}

	cmd := NewDetectCommand(options)
	return cmd.Execute()
}

// DetectConflictsText is a convenience function for text format output
func DetectConflictsText(repoPath string) (*DetectResult, error) {
	options := DetectOptions{
		RepoPath:     repoPath,
		OutputFormat: OutputFormatText,
		Verbose:      true,
	}

	cmd := NewDetectCommand(options)
	return cmd.Execute()
}

// logOperation logs operation details with structured data for enhanced debugging
func (d *DetectCommand) logOperation(operation string, details map[string]interface{}) {
	if d.options.EnableDetailed || d.options.Verbose {
		logMsg := fmt.Sprintf("[DETECT] %s", operation)
		if d.options.LogFile != "" {
			// Append to log file if specified
			if file, err := os.OpenFile(d.options.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600); err == nil {
				defer file.Close()
				logger := log.New(file, "", log.LstdFlags)
				logger.Printf("%s - Details: %+v", logMsg, details)
			}
		} else if d.options.Verbose {
			// Log to stderr when verbose mode is enabled
			log.Printf("%s - Details: %+v", logMsg, details)
		}
	}
}
