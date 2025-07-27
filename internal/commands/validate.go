package commands

import (
	"fmt"
	"os"
	"time"

	"github.com/NeuBlink/syncwright/internal/iojson"
	"github.com/NeuBlink/syncwright/internal/validate"
)

// ValidateOptions contains options for the validate command
type ValidateOptions struct {
	RootPath       string
	OutputFile     string
	TimeoutSeconds int
	Verbose        bool
}

// ValidateCommand implements the validate subcommand
type ValidateCommand struct {
	options ValidateOptions
}

// NewValidateCommand creates a new validate command
func NewValidateCommand(options ValidateOptions) *ValidateCommand {
	// Set defaults
	if options.RootPath == "" {
		if wd, err := os.Getwd(); err == nil {
			options.RootPath = wd
		}
	}
	if options.TimeoutSeconds == 0 {
		options.TimeoutSeconds = 300 // Default 5 minute timeout
	}

	return &ValidateCommand{
		options: options,
	}
}

// Execute runs the validate command
func (v *ValidateCommand) Execute() error {
	if v.options.Verbose {
		fmt.Fprintf(os.Stderr, "Running validation on project at: %s\n", v.options.RootPath)
		fmt.Fprintf(os.Stderr, "Timeout: %d seconds\n", v.options.TimeoutSeconds)
	}

	// Run validation - never fail the command even if validation has issues
	report, err := validate.RunValidation(v.options.RootPath, v.options.TimeoutSeconds)
	if err != nil {
		// Create a minimal report to ensure we always output something
		report = &validate.ValidationReport{
			Project: validate.ProjectInfo{
				Type:          validate.ProjectTypeGeneric,
				RootPath:      v.options.RootPath,
				ConfigFiles:   []string{},
				DetectedTools: []string{},
			},
			ValidationTime: time.Now(),
			OverallSuccess: false,
			CommandResults: []validate.CommandResult{},
			FileResults:    []validate.ValidationResult{},
			Summary: validate.ValidationSummary{
				TotalCommands:      0,
				SuccessfulCommands: 0,
				FailedCommands:     0,
				SkippedCommands:    0,
				TotalFiles:         0,
				ValidFiles:         0,
				InvalidFiles:       0,
				TotalIssues:        0,
				ErrorIssues:        0,
				WarningIssues:      0,
			},
		}

		if v.options.Verbose {
			fmt.Fprintf(os.Stderr, "Warning: Validation encountered errors but continuing: %v\n", err)
		}
	}

	// Output results - try to output even if there were validation errors
	if err := v.outputResults(report); err != nil {
		// If we can't output results, that's a critical error
		// But still print the summary to stderr to provide some feedback
		fmt.Fprintf(os.Stderr, "Error: Failed to output results: %v\n", err)
		v.printSummary(report)
		return fmt.Errorf("failed to output results: %w", err)
	}

	// Print summary to stderr if verbose or if outputting to file
	if v.options.Verbose || v.options.OutputFile != "" {
		v.printSummary(report)
	}

	return nil
}

// outputResults outputs the validation report
func (v *ValidateCommand) outputResults(report *validate.ValidationReport) error {
	return iojson.WriteOutput(v.options.OutputFile, report)
}

// printSummary prints a human-readable summary to stderr
func (v *ValidateCommand) printSummary(report *validate.ValidationReport) {
	v.printSummaryHeader(report)
	v.printCommandSummary(report)
	v.printFileSummary(report)
	v.printIssuesSummary(report)
	v.printVerboseDetails(report)
	v.printConflictDetails(report)
}

// printSummaryHeader prints the main summary header
func (v *ValidateCommand) printSummaryHeader(report *validate.ValidationReport) {
	fmt.Fprintf(os.Stderr, "\n=== Validation Summary ===\n")
	fmt.Fprintf(os.Stderr, "Project Type: %s\n", report.Project.Type)
	fmt.Fprintf(os.Stderr, "Root Path: %s\n", report.Project.RootPath)
	fmt.Fprintf(os.Stderr, "Validation Time: %s\n", report.ValidationTime.Format(time.RFC3339))
	fmt.Fprintf(os.Stderr, "Overall Success: %t\n\n", report.OverallSuccess)
}

// printCommandSummary prints command execution summary
func (v *ValidateCommand) printCommandSummary(report *validate.ValidationReport) {
	fmt.Fprintf(os.Stderr, "Commands:\n")
	fmt.Fprintf(os.Stderr, "  Total: %d\n", report.Summary.TotalCommands)
	fmt.Fprintf(os.Stderr, "  Successful: %d\n", report.Summary.SuccessfulCommands)
	fmt.Fprintf(os.Stderr, "  Failed: %d\n", report.Summary.FailedCommands)
	fmt.Fprintf(os.Stderr, "  Skipped: %d\n\n", report.Summary.SkippedCommands)
}

// printFileSummary prints file validation summary
func (v *ValidateCommand) printFileSummary(report *validate.ValidationReport) {
	fmt.Fprintf(os.Stderr, "Files:\n")
	fmt.Fprintf(os.Stderr, "  Total: %d\n", report.Summary.TotalFiles)
	fmt.Fprintf(os.Stderr, "  Valid: %d\n", report.Summary.ValidFiles)
	fmt.Fprintf(os.Stderr, "  Invalid: %d\n\n", report.Summary.InvalidFiles)
}

// printIssuesSummary prints issues summary
func (v *ValidateCommand) printIssuesSummary(report *validate.ValidationReport) {
	fmt.Fprintf(os.Stderr, "Issues:\n")
	fmt.Fprintf(os.Stderr, "  Total: %d\n", report.Summary.TotalIssues)
	fmt.Fprintf(os.Stderr, "  Errors: %d\n", report.Summary.ErrorIssues)
	fmt.Fprintf(os.Stderr, "  Warnings: %d\n\n", report.Summary.WarningIssues)
}

// printVerboseDetails prints detailed command results when verbose mode is enabled
func (v *ValidateCommand) printVerboseDetails(report *validate.ValidationReport) {
	if !v.options.Verbose {
		return
	}

	fmt.Fprintf(os.Stderr, "Command Details:\n")
	for _, result := range report.CommandResults {
		v.printCommandResult(result)
	}
	fmt.Fprintf(os.Stderr, "\n")
}

// printCommandResult prints details for a single command result
func (v *ValidateCommand) printCommandResult(result validate.CommandResult) {
	status := v.getCommandStatus(result)
	
	fmt.Fprintf(os.Stderr, "  %s [%s] - %s (%.2fs)\n",
		result.Command.Name, status, result.Command.Description, result.Duration.Seconds())

	if result.Skipped {
		fmt.Fprintf(os.Stderr, "    Reason: %s\n", result.SkipReason)
	} else if !result.Success {
		fmt.Fprintf(os.Stderr, "    Error: %s\n", result.Error)
		if result.Stderr != "" {
			fmt.Fprintf(os.Stderr, "    Stderr: %s\n", result.Stderr)
		}
	}
}

// getCommandStatus returns the status string for a command result
func (v *ValidateCommand) getCommandStatus(result validate.CommandResult) string {
	if result.Skipped {
		return "SKIPPED"
	}
	if !result.Success {
		return "FAILED"
	}
	return "SUCCESS"
}

// printConflictDetails prints merge conflict information
func (v *ValidateCommand) printConflictDetails(report *validate.ValidationReport) {
	conflictFiles := v.findConflictFiles(report)
	if len(conflictFiles) == 0 {
		return
	}

	fmt.Fprintf(os.Stderr, "Merge Conflicts Found:\n")
	for _, file := range conflictFiles {
		fmt.Fprintf(os.Stderr, "  %s\n", file)
	}
	fmt.Fprintf(os.Stderr, "\n")
}

// findConflictFiles finds all files with merge conflicts
func (v *ValidateCommand) findConflictFiles(report *validate.ValidationReport) []string {
	var conflictFiles []string
	for _, fileResult := range report.FileResults {
		if !fileResult.ConflictFree {
			conflictFiles = append(conflictFiles, fileResult.File)
		}
	}
	return conflictFiles
}

// ValidateProject is a convenience function for running validation
func ValidateProject(rootPath, outputFile string, timeoutSeconds int, verbose bool) error {
	options := ValidateOptions{
		RootPath:       rootPath,
		OutputFile:     outputFile,
		TimeoutSeconds: timeoutSeconds,
		Verbose:        verbose,
	}

	cmd := NewValidateCommand(options)
	return cmd.Execute()
}
