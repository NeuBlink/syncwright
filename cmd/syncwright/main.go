package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"syncwright/internal/commands"
	"syncwright/internal/format"
	"syncwright/internal/iojson"
)

// Build information - set via ldflags during build
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
	builtBy = "unknown"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// buildVersionString creates a detailed version string with build metadata
func buildVersionString() string {
	if commit == "none" || date == "unknown" {
		return version
	}
	return fmt.Sprintf("%s (commit: %s, built: %s, by: %s)", version, commit[:8], date, builtBy)
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "syncwright",
		Short: "Syncwright - Git merge conflict resolution toolkit",
		Long: `Syncwright is a CLI tool for detecting, analyzing, and resolving Git merge conflicts
through a pipeline of JSON-based operations that can be automated or AI-assisted.`,
		Version: buildVersionString(),
	}

	// Add subcommands
	cmd.AddCommand(
		newDetectCmd(),
		newPayloadCmd(),
		newAIApplyCmd(),
		newFormatCmd(),
		newValidateCmd(),
		newCommitCmd(),
		newResolveCmd(),
	)

	return cmd
}

// DetectResult represents the output of the detect command
type DetectResult struct {
	Conflicts []ConflictInfo `json:"conflicts"`
	Timestamp string         `json:"timestamp"`
}

type ConflictInfo struct {
	File       string   `json:"file"`
	LineStart  int      `json:"line_start"`
	LineEnd    int      `json:"line_end"`
	ConflictID string   `json:"conflict_id"`
	Markers    []string `json:"markers"`
}

func newDetectCmd() *cobra.Command {
	var outputFile string

	cmd := &cobra.Command{
		Use:   "detect",
		Short: "Detect merge conflicts in the current repository",
		Long:  "Scans the repository for merge conflicts and outputs a JSON report of findings.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement conflict detection logic
			result := DetectResult{
				Conflicts: []ConflictInfo{},
				Timestamp: "placeholder",
			}

			return iojson.WriteOutput(outputFile, result)
		},
	}

	cmd.Flags().StringVarP(&outputFile, "out", "o", "", "Output file for conflicts JSON (default: stdout)")

	return cmd
}

// PayloadResult represents the output of the payload command
type PayloadResult struct {
	Conflicts []ConflictPayload `json:"conflicts"`
	Metadata  PayloadMetadata   `json:"metadata"`
}

type ConflictPayload struct {
	ConflictID string `json:"conflict_id"`
	Context    string `json:"context"`
	Options    string `json:"options"`
}

type PayloadMetadata struct {
	SourceFile string `json:"source_file"`
	Timestamp  string `json:"timestamp"`
}

func newPayloadCmd() *cobra.Command {
	var inputFile, outputFile string

	cmd := &cobra.Command{
		Use:   "payload",
		Short: "Generate AI-ready payloads from conflict data",
		Long:  "Transforms conflict detection results into structured payloads suitable for AI processing.",
		RunE: func(cmd *cobra.Command, args []string) error {
			var input DetectResult
			if err := iojson.ReadInput(inputFile, &input); err != nil {
				return fmt.Errorf("failed to read input: %w", err)
			}

			// TODO: Implement payload generation logic
			result := PayloadResult{
				Conflicts: []ConflictPayload{},
				Metadata: PayloadMetadata{
					SourceFile: inputFile,
					Timestamp:  "placeholder",
				},
			}

			return iojson.WriteOutput(outputFile, result)
		},
	}

	cmd.Flags().StringVarP(&inputFile, "in", "i", "", "Input file with conflict data (default: stdin)")
	cmd.Flags().StringVarP(&outputFile, "out", "o", "", "Output file for payload JSON (default: stdout)")

	return cmd
}

// AIApplyResult represents the output of the ai-apply command
type AIApplyResult struct {
	Resolutions []ConflictResolution `json:"resolutions"`
	Status      string               `json:"status"`
	Metadata    AIApplyMetadata      `json:"metadata"`
}

type ConflictResolution struct {
	ConflictID string `json:"conflict_id"`
	Resolution string `json:"resolution"`
	Confidence float64 `json:"confidence"`
}

type AIApplyMetadata struct {
	SourceFile string `json:"source_file"`
	Timestamp  string `json:"timestamp"`
}

func newAIApplyCmd() *cobra.Command {
	var inputFile, outputFile string

	cmd := &cobra.Command{
		Use:   "ai-apply",
		Short: "Apply AI-generated conflict resolutions",
		Long:  "Processes AI payloads and applies the suggested conflict resolutions to files.",
		RunE: func(cmd *cobra.Command, args []string) error {
			var input PayloadResult
			if err := iojson.ReadInput(inputFile, &input); err != nil {
				return fmt.Errorf("failed to read input: %w", err)
			}

			// TODO: Implement AI resolution application logic
			result := AIApplyResult{
				Resolutions: []ConflictResolution{},
				Status:      "pending",
				Metadata: AIApplyMetadata{
					SourceFile: inputFile,
					Timestamp:  "placeholder",
				},
			}

			return iojson.WriteOutput(outputFile, result)
		},
	}

	cmd.Flags().StringVarP(&inputFile, "in", "i", "", "Input file with payload data (default: stdin)")
	cmd.Flags().StringVarP(&outputFile, "out", "o", "", "Output file for AI apply results (default: stdout)")

	return cmd
}


func newFormatCmd() *cobra.Command {
	var (
		outputFile          string
		outputFormat        string
		dryRun              bool
		verbose             bool
		preferredFormatters []string
		excludeFormatters   []string
		includeExtensions   []string
		excludeExtensions   []string
		filePaths           []string
		scanRecent          bool
		recentDays          int
		timeout             int
		concurrency         int
	)

	cmd := &cobra.Command{
		Use:   "format [flags] [files...]",
		Short: "Format resolved files according to project standards",
		Long: `Applies code formatting to files that have been processed for conflict resolution.

The format command discovers available formatters on your system and applies them to
files based on their extension. It supports multiple languages and can format:
- Recently modified files (--recent)
- Specific files (passed as arguments)
- All tracked files in the repository (default)

Examples:
  # Format recently modified files (last 7 days)
  syncwright format --recent

  # Format specific files
  syncwright format main.go utils.py styles.css

  # Dry run to see what would be formatted
  syncwright format --dry-run --verbose

  # Format only Go files, excluding gofmt
  syncwright format --include-ext go --exclude-formatter gofmt`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Add any file arguments to filePaths
			if len(args) > 0 {
				filePaths = append(filePaths, args...)
			}

			// Create format options
			options := commands.FormatOptions{
				OutputFile:          outputFile,
				OutputFormat:        outputFormat,
				DryRun:              dryRun,
				Verbose:             verbose,
				PreferredFormatters: preferredFormatters,
				ExcludeFormatters:   excludeFormatters,
				IncludeExtensions:   includeExtensions,
				ExcludeExtensions:   excludeExtensions,
				FilePaths:           filePaths,
				ScanRecent:          scanRecent,
				RecentDays:          recentDays,
				FormatOptions: format.FormatOptions{
					DryRun:      dryRun,
					Timeout:     time.Duration(timeout) * time.Second,
					Concurrency: concurrency,
				},
			}

			// Execute the format command
			formatCmd := commands.NewFormatCommand(options)
			result, err := formatCmd.Execute()
			
			if err != nil {
				return fmt.Errorf("format command failed: %w", err)
			}

			// If output was already written by the command, we're done
			if outputFile != "" || outputFormat == "text" {
				return nil
			}

			// Otherwise, write JSON to stdout
			return iojson.WriteOutput("", result)
		},
	}

	// Output options
	cmd.Flags().StringVarP(&outputFile, "out", "o", "", "Output file for format results (default: stdout)")
	cmd.Flags().StringVar(&outputFormat, "format", "json", "Output format: json, text")
	
	// Execution options
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be formatted without making changes")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	cmd.Flags().IntVar(&timeout, "timeout", 30, "Timeout for each formatter in seconds")
	cmd.Flags().IntVar(&concurrency, "concurrency", 1, "Number of files to format concurrently")
	
	// Formatter selection
	cmd.Flags().StringSliceVar(&preferredFormatters, "prefer-formatter", nil, "Preferred formatters to use (e.g., goimports,prettier)")
	cmd.Flags().StringSliceVar(&excludeFormatters, "exclude-formatter", nil, "Formatters to exclude (e.g., gofmt,eslint)")
	
	// File selection
	cmd.Flags().StringSliceVar(&includeExtensions, "include-ext", nil, "Only format files with these extensions (e.g., go,js,py)")
	cmd.Flags().StringSliceVar(&excludeExtensions, "exclude-ext", nil, "Exclude files with these extensions")
	cmd.Flags().BoolVar(&scanRecent, "recent", false, "Format only recently modified files")
	cmd.Flags().IntVar(&recentDays, "recent-days", 7, "Number of days to look back for recent files")

	return cmd
}

// ValidateResult represents the output of the validate command
type ValidateResult struct {
	ValidationPassed bool               `json:"validation_passed"`
	Issues           []ValidationIssue  `json:"issues"`
	Metadata         ValidateMetadata   `json:"metadata"`
}

type ValidationIssue struct {
	File        string `json:"file"`
	Line        int    `json:"line"`
	Message     string `json:"message"`
	Severity    string `json:"severity"`
}

type ValidateMetadata struct {
	Timestamp string `json:"timestamp"`
}

func newValidateCmd() *cobra.Command {
	var outputFile string
	var timeoutSeconds int
	var verbose bool

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate resolved files for correctness",
		Long: `Performs comprehensive validation on the project using appropriate tools based on project type.
		
The validation system:
- Detects project type (Go, JavaScript/TypeScript, Python, Rust, or generic)
- Discovers available validation tools automatically
- Runs appropriate build, test, and lint commands
- Checks for merge conflict markers in files
- Reports results in JSON format
- Never fails the workflow - always provides feedback`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.ValidateProject("", outputFile, timeoutSeconds, verbose)
		},
	}

	cmd.Flags().StringVarP(&outputFile, "out", "o", "", "Output file for validation results (default: stdout)")
	cmd.Flags().IntVarP(&timeoutSeconds, "timeout", "t", 300, "Timeout in seconds for validation commands")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")

	return cmd
}

func newCommitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "commit",
		Short: "Commit resolved changes to Git",
		Long:  "Creates a Git commit with the resolved conflicts and appropriate commit message.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement commit logic
			fmt.Println("Commit functionality not yet implemented")
			return nil
		},
	}

	return cmd
}

func newResolveCmd() *cobra.Command {
	var (
		maxTokens int
		aiMode    bool
		verbose   bool
		dryRun    bool
		confidence float64
	)

	cmd := &cobra.Command{
		Use:   "resolve",
		Short: "Automated conflict resolution pipeline",
		Long: `Runs the complete conflict resolution pipeline: detect conflicts, 
generate AI payload, apply resolutions, and validate results.

This command combines all Syncwright operations into a single workflow
suitable for CI/CD environments and automated conflict resolution.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if verbose {
				fmt.Println("🔍 Starting automated conflict resolution...")
			}

			// Step 1: Detect conflicts
			detectResult, err := commands.DetectConflicts("")
			if err != nil {
				return fmt.Errorf("conflict detection failed: %w", err)
			}

			if len(detectResult.Conflicts) == 0 {
				if verbose {
					fmt.Println("✅ No conflicts detected")
				}
				return nil
			}

			if verbose {
				fmt.Printf("📋 Found %d conflicts\n", len(detectResult.Conflicts))
			}

			if !aiMode {
				fmt.Printf("Found %d conflicts. Use --ai flag to resolve with AI assistance.\n", len(detectResult.Conflicts))
				return nil
			}

			// For now, provide basic conflict information
			if verbose {
				fmt.Printf("🤖 AI resolution would process %d conflicts\n", len(detectResult.Conflicts))
				if maxTokens == -1 {
					fmt.Println("📊 Using unlimited tokens for AI processing")
				} else {
					fmt.Printf("📊 Using %d tokens for AI processing\n", maxTokens)
				}
			}

			if dryRun {
				fmt.Printf("Dry run: Would resolve %d conflicts with AI assistance\n", len(detectResult.Conflicts))
				return nil
			}

			// TODO: Complete the pipeline with payload generation and AI application
			fmt.Printf("Detected %d conflicts. Full AI resolution pipeline coming soon.\n", len(detectResult.Conflicts))
			fmt.Println("For now, use the individual commands: detect -> payload -> ai-apply")

			return nil
		},
	}

	cmd.Flags().IntVar(&maxTokens, "max-tokens", -1, "Maximum tokens for AI processing (-1 for unlimited)")
	cmd.Flags().BoolVar(&aiMode, "ai", false, "Enable AI-powered conflict resolution")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without applying")
	cmd.Flags().Float64Var(&confidence, "confidence", 0.7, "Minimum confidence threshold for applying resolutions")

	return cmd
}