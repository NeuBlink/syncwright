package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"syncwright/internal/gitutils"
	"syncwright/internal/payload"
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
		options.OutputFormat = "json"
	}
	if options.MaxContextLines == 0 {
		options.MaxContextLines = 5
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
	case "json":
		output, err = json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
	case "text":
		output = d.formatTextOutput(result)
	default:
		return fmt.Errorf("unsupported output format: %s", d.options.OutputFormat)
	}

	// Write to file or stdout
	if d.options.OutputFile != "" {
		err = os.WriteFile(d.options.OutputFile, output, 0644)
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
		return []byte(fmt.Sprintf("%s\n", fmt.Sprintf("%s", output)))
	}

	output = append(output, fmt.Sprintf("Repository: %s", result.Summary.RepoPath))
	output = append(output, fmt.Sprintf("In merge state: %t", result.Summary.InMergeState))
	output = append(output, "")

	if !result.Summary.InMergeState {
		output = append(output, "✅ No conflicts detected - repository is not in merge state")
		return []byte(fmt.Sprintf("%s\n", fmt.Sprintf("%s", output)))
	}

	output = append(output, "📊 Summary:")
	output = append(output, fmt.Sprintf("  Total conflicted files: %d", result.Summary.TotalFiles))
	output = append(output, fmt.Sprintf("  Total conflicts: %d", result.Summary.TotalConflicts))
	output = append(output, fmt.Sprintf("  Processable files: %d", result.Summary.ProcessableFiles))
	output = append(output, fmt.Sprintf("  Excluded files: %d", result.Summary.ExcludedFiles))
	output = append(output, "")

	if result.ConflictPayload != nil && len(result.ConflictPayload.Files) > 0 {
		output = append(output, "📁 Conflicted Files:")
		for _, file := range result.ConflictPayload.Files {
			output = append(output, fmt.Sprintf("  %s (%s)", file.Path, file.Language))
			output = append(output, fmt.Sprintf("    Conflicts: %d", len(file.Conflicts)))

			if d.options.Verbose {
				for i, conflict := range file.Conflicts {
					output = append(output, fmt.Sprintf("      Conflict %d: lines %d-%d (%s)",
						i+1, conflict.StartLine, conflict.EndLine, conflict.ConflictType))
				}
			}
		}
		output = append(output, "")
	}

	if result.ConflictReport != nil && len(result.ConflictReport.ConflictedFiles) > 0 {
		excludedFiles := []string{}
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

		if len(excludedFiles) > 0 && d.options.Verbose {
			output = append(output, "🚫 Excluded Files:")
			for _, file := range excludedFiles {
				output = append(output, fmt.Sprintf("  %s", file))
			}
			output = append(output, "")
		}
	}

	if result.ConflictPayload != nil {
		output = append(output, "🔧 Next Steps:")
		output = append(output, "  1. Review the conflicts above")
		output = append(output, "  2. Run 'syncwright ai-apply' to get AI-suggested resolutions")
		output = append(output, "  3. Review and apply the suggested resolutions")
		output = append(output, "  4. Test your changes")
		output = append(output, "  5. Commit the resolved conflicts")
	}

	return []byte(fmt.Sprintf("%s\n", fmt.Sprintf("%s", output)))
}

// DetectConflicts is a convenience function for simple conflict detection
func DetectConflicts(repoPath string) (*DetectResult, error) {
	options := DetectOptions{
		RepoPath:     repoPath,
		OutputFormat: "json",
		Verbose:      false,
	}

	cmd := NewDetectCommand(options)
	return cmd.Execute()
}

// DetectConflictsVerbose is a convenience function for verbose conflict detection
func DetectConflictsVerbose(repoPath string, outputFile string) (*DetectResult, error) {
	options := DetectOptions{
		RepoPath:     repoPath,
		OutputFormat: "json",
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
		OutputFormat: "text",
		Verbose:      true,
	}

	cmd := NewDetectCommand(options)
	return cmd.Execute()
}
