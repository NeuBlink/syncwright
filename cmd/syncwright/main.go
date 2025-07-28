package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/NeuBlink/syncwright/internal/commands"
	"github.com/NeuBlink/syncwright/internal/format"
	"github.com/NeuBlink/syncwright/internal/gitutils"
	"github.com/NeuBlink/syncwright/internal/iojson"
	"github.com/NeuBlink/syncwright/internal/logging"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// Build information - set via ldflags during build
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
	builtBy = "unknown"
)

func main() {
	// Initialize logging
	config := logging.GetDefaultConfig()
	logging.MustInitialize(config)
	defer logging.Sync()

	if err := newRootCmd().Execute(); err != nil {
		logging.Logger.ErrorSafe("Command execution failed", zap.Error(err))
		os.Exit(1)
	}
}

// buildVersionString creates a detailed version string with build metadata
func buildVersionString() string {
	if commit == "none" || date == "unknown" {
		return version
	}

	// Safely truncate commit hash to at most 8 characters
	commitDisplay := commit
	if len(commit) > 8 {
		commitDisplay = commit[:8]
	}

	return fmt.Sprintf("%s (commit: %s, built: %s, by: %s)", version, commitDisplay, date, builtBy)
}

func newRootCmd() *cobra.Command {
	var (
		logLevel  string
		logFormat string
		verbose   bool
		debug     bool
	)

	cmd := &cobra.Command{
		Use:   "syncwright",
		Short: "Syncwright - Git merge conflict resolution toolkit",
		Long: `Syncwright is a CLI tool for detecting, analyzing, and resolving Git merge conflicts
through a pipeline of JSON-based operations that can be automated or AI-assisted.`,
		Version: buildVersionString(),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Configure logging based on flags
			config := logging.GetDefaultConfig()

			// Handle debug flag
			if debug {
				config.Level = zap.DebugLevel
				config.Development = true
				config.Verbose = true
			}

			// Handle verbose flag
			if verbose {
				config.Verbose = true
			}

			// Handle log level
			switch strings.ToLower(logLevel) {
			case "debug":
				config.Level = zap.DebugLevel
				config.Development = true
				config.Verbose = true
			case "info":
				config.Level = zap.InfoLevel
			case "warn", "warning":
				config.Level = zap.WarnLevel
			case "error":
				config.Level = zap.ErrorLevel
			default:
				// Keep default level
			}

			// Handle log format
			switch strings.ToLower(logFormat) {
			case "console":
				config.Development = true
			case "json":
				config.Development = false
			default:
				// Keep default format
			}

			// Reinitialize logger with new config
			return logging.InitializeLogger(config)
		},
	}

	// Add persistent flags for logging configuration
	cmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Set log level (debug, info, warn, error)")
	cmd.PersistentFlags().StringVar(&logFormat, "log-format", "json", "Set log format (json, console)")
	cmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
	cmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug logging (overrides log-level)")

	// Add subcommands
	cmd.AddCommand(
		newDetectCmd(),
		newPayloadCmd(),
		newAIApplyCmd(),
		newBatchCmd(),
		newFormatCmd(),
		newValidateCmd(),
		newCommitCmd(),
		newResolveCmd(),
	)

	return cmd
}

func newDetectCmd() *cobra.Command {
	var outputFile string
	var outputFormat string
	var verbose bool

	cmd := &cobra.Command{
		Use:   "detect",
		Short: "Detect merge conflicts in the current repository",
		Long:  "Scans the repository for merge conflicts and outputs a JSON report of findings.",
		RunE: func(cmd *cobra.Command, args []string) error {
			options := commands.DetectOptions{
				OutputFile:   outputFile,
				OutputFormat: outputFormat,
				Verbose:      verbose,
			}

			detectCmd := commands.NewDetectCommand(options)
			_, err := detectCmd.Execute()
			return err
		},
	}

	cmd.Flags().StringVarP(&outputFile, "out", "o", "", "Output file for conflicts JSON (default: stdout)")
	cmd.Flags().StringVar(&outputFormat, "format", "json", "Output format: json, text")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")

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
			var input commands.DetectResult
			if err := iojson.ReadInput(inputFile, &input); err != nil {
				logging.Logger.ErrorSafe("Failed to read conflict input", zap.Error(err))
				return fmt.Errorf("failed to read input: %w", err)
			}

			// Validate input has conflict data
			if input.ConflictPayload == nil {
				logging.Logger.WarnSafe("Input does not contain conflict payload data")
				return fmt.Errorf("input does not contain conflict payload data")
			}

			// Convert SimplifiedConflictPayload to PayloadResult format
			var conflicts []ConflictPayload
			for _, file := range input.ConflictPayload.Files {
				for i, conflict := range file.Conflicts {
					conflictPayload := ConflictPayload{
						ConflictID: fmt.Sprintf("%s:%d", file.Path, i),
						Context:    fmt.Sprintf("File: %s (lines %d-%d) [%s]", file.Path, conflict.StartLine, conflict.EndLine, file.Language),
						Options:    fmt.Sprintf("Ours: %d lines, Theirs: %d lines", len(conflict.OursLines), len(conflict.TheirsLines)),
					}
					conflicts = append(conflicts, conflictPayload)
				}
			}

			// Create result
			result := PayloadResult{
				Conflicts: conflicts,
				Metadata: PayloadMetadata{
					SourceFile: inputFile,
					Timestamp:  time.Now().Format(time.RFC3339),
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
	ConflictID string  `json:"conflict_id"`
	Resolution string  `json:"resolution"`
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
			// Read payload data - could be from payload command output or direct conflict data
			var payloadData []byte
			var err error

			if inputFile == "" || inputFile == "-" {
				payloadData, err = io.ReadAll(os.Stdin)
			} else {
				payloadData, err = os.ReadFile(inputFile)
			}
			if err != nil {
				logging.Logger.ErrorSafe("Failed to read AI apply input", zap.Error(err))
				return fmt.Errorf("failed to read input: %w", err)
			}

			// Get current working directory if not specified
			repoPath, err := os.Getwd()
			if err != nil {
				logging.Logger.ErrorSafe("Failed to get current directory", zap.Error(err))
				return fmt.Errorf("failed to get current directory: %w", err)
			}

			// Get API key from environment
			apiKey := os.Getenv("CLAUDE_CODE_OAUTH_TOKEN")
			if apiKey == "" {
				logging.Logger.ErrorSafe("Claude API key not configured")
				return fmt.Errorf("CLAUDE_CODE_OAUTH_TOKEN environment variable not set")
			}

			// Create AI apply options
			options := commands.AIApplyOptions{
				RepoPath:       repoPath,
				OutputFile:     outputFile,
				DryRun:         false,
				Verbose:        true,
				AutoApply:      false,
				MinConfidence:  0.7,
				BackupFiles:    true,
				MaxRetries:     3,
				TimeoutSeconds: 300,
			}

			// Create temporary file for payload data
			tmpFile, err := os.CreateTemp("", "syncwright-payload-*.json")
			if err != nil {
				logging.Logger.ErrorSafe("Failed to create temporary file", zap.Error(err))
				return fmt.Errorf("failed to create temporary file: %w", err)
			}
			defer func() {
				tmpFile.Close()
				os.Remove(tmpFile.Name())
			}()

			if _, err := tmpFile.Write(payloadData); err != nil {
				logging.Logger.ErrorSafe("Failed to write payload data", zap.Error(err))
				return fmt.Errorf("failed to write payload data: %w", err)
			}
			tmpFile.Close()

			options.PayloadFile = tmpFile.Name()

			// Create and execute AI apply command
			aiCmd, err := commands.NewAIApplyCommand(options)
			if err != nil {
				logging.Logger.ErrorSafe("Failed to create AI apply command", zap.Error(err))
				return fmt.Errorf("failed to create AI apply command: %w", err)
			}

			aiResult, err := aiCmd.Execute()
			if err != nil {
				logging.Logger.ErrorSafe("AI apply command execution failed", zap.Error(err))
				return fmt.Errorf("AI apply command failed: %w", err)
			}

			// Convert AIApplyResult to the expected output format
			var resolutions []ConflictResolution
			for _, res := range aiResult.Resolutions {
				resolutions = append(resolutions, ConflictResolution{
					ConflictID: fmt.Sprintf("%s:%d", res.FilePath, res.StartLine),
					Resolution: strings.Join(res.ResolvedLines, "\n"),
					Confidence: res.Confidence,
				})
			}

			status := "success"
			if !aiResult.Success {
				status = "failed"
			}

			result := AIApplyResult{
				Resolutions: resolutions,
				Status:      status,
				Metadata: AIApplyMetadata{
					SourceFile: inputFile,
					Timestamp:  time.Now().Format(time.RFC3339),
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
	cmd.Flags().StringSliceVar(&preferredFormatters, "prefer-formatter", nil,
		"Preferred formatters to use (e.g., goimports,prettier)")
	cmd.Flags().StringSliceVar(&excludeFormatters, "exclude-formatter", nil, "Formatters to exclude (e.g., gofmt,eslint)")

	// File selection
	cmd.Flags().StringSliceVar(&includeExtensions, "include-ext", nil,
		"Only format files with these extensions (e.g., go,js,py)")
	cmd.Flags().StringSliceVar(&excludeExtensions, "exclude-ext", nil, "Exclude files with these extensions")
	cmd.Flags().BoolVar(&scanRecent, "recent", false, "Format only recently modified files")
	cmd.Flags().IntVar(&recentDays, "recent-days", 7, "Number of days to look back for recent files")

	return cmd
}

// ValidateResult represents the output of the validate command
type ValidateResult struct {
	ValidationPassed bool              `json:"validation_passed"`
	Issues           []ValidationIssue `json:"issues"`
	Metadata         ValidateMetadata  `json:"metadata"`
}

type ValidationIssue struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
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
			// Get current working directory
			repoPath, err := os.Getwd()
			if err != nil {
				logging.Logger.ErrorSafe("Failed to get current directory for commit", zap.Error(err))
				return fmt.Errorf("failed to get current directory: %w", err)
			}

			// Check if we're in a git repository
			if !gitutils.IsGitRepositoryPath(repoPath) {
				return fmt.Errorf("not in a git repository")
			}

			// Check if there are any conflicted files remaining
			conflictedFiles, err := gitutils.GetConflictedFiles()
			if err != nil {
				return fmt.Errorf("failed to check for conflicted files: %w", err)
			}

			if len(conflictedFiles) > 0 {
				logging.Logger.WarnSafe("Conflicted files remain before commit",
					zap.Int("conflicted_files_count", len(conflictedFiles)),
					zap.Strings("conflicted_files", conflictedFiles))
				fmt.Printf("Warning: %d conflicted files still remain:\n", len(conflictedFiles))
				for _, file := range conflictedFiles {
					fmt.Printf("  - %s\n", file)
				}
				fmt.Print("Continue with commit anyway? [y/N]: ")
				var response string
				if _, err := fmt.Scanln(&response); err != nil {
					response = "n"
				}
				if strings.ToLower(strings.TrimSpace(response)) != "y" {
					return fmt.Errorf("commit cancelled due to remaining conflicts")
				}
			}

			// Generate commit message
			commitMessage := "resolve: merge conflicts resolved"
			if len(conflictedFiles) == 0 {
				commitMessage = "resolve: all merge conflicts resolved"
			}

			// Add Claude Code attribution
			commitMessage += "\n\n🤖 Generated with [Claude Code](https://claude.ai/code)\n\nCo-Authored-By: Claude <noreply@anthropic.com>"

			// Commit the changes
			logging.Logger.InfoSafe("Committing resolved changes", zap.String("commit_message", strings.Split(commitMessage, "\n")[0]))
			fmt.Println("Committing resolved changes...")
			if err := gitutils.CommitChanges(commitMessage); err != nil {
				logging.Logger.ErrorSafe("Failed to commit changes", zap.Error(err))
				return fmt.Errorf("failed to commit changes: %w", err)
			}

			logging.Logger.InfoSafe("Changes committed successfully")
			fmt.Println("✅ Changes committed successfully")
			fmt.Printf("Commit message: %s\n", strings.Split(commitMessage, "\n")[0])

			return nil
		},
	}

	return cmd
}

func newResolveCmd() *cobra.Command {
	var (
		maxTokens    int
		aiMode       bool
		verbose      bool
		dryRun       bool
		confidence   float64
		apiKey       string
		autoApply    bool
		skipFormat   bool
		skipValidate bool
	)

	cmd := &cobra.Command{
		Use:   "resolve",
		Short: "Automated conflict resolution pipeline",
		Long: `Runs the complete conflict resolution pipeline: detect conflicts, 
generate AI payload, apply resolutions, format files, and validate results.

This command combines all Syncwright operations into a single workflow
suitable for CI/CD environments and automated conflict resolution.

Examples:
  # Basic conflict resolution with AI
  syncwright resolve --ai

  # Dry run to preview resolutions
  syncwright resolve --ai --dry-run

  # Auto-apply high-confidence resolutions
  syncwright resolve --ai --auto-apply --confidence 0.8

  # Skip formatting and validation steps
  syncwright resolve --ai --skip-format --skip-validate`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeResolveCommand(resolveOptions{
				maxTokens:    maxTokens,
				aiMode:       aiMode,
				verbose:      verbose,
				dryRun:       dryRun,
				confidence:   confidence,
				apiKey:       apiKey,
				autoApply:    autoApply,
				skipFormat:   skipFormat,
				skipValidate: skipValidate,
			})
		},
	}

	cmd.Flags().IntVar(&maxTokens, "max-tokens", -1, "Maximum tokens for AI processing (-1 for unlimited)")
	cmd.Flags().BoolVar(&aiMode, "ai", false, "Enable AI-powered conflict resolution")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without applying")
	cmd.Flags().Float64Var(&confidence, "confidence", 0.7, "Minimum confidence threshold for applying resolutions")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "Claude Code API key (or set CLAUDE_CODE_OAUTH_TOKEN env var)")
	cmd.Flags().BoolVar(&autoApply, "auto-apply", false, "Automatically apply high-confidence resolutions without confirmation")
	cmd.Flags().BoolVar(&skipFormat, "skip-format", false, "Skip code formatting step")
	cmd.Flags().BoolVar(&skipValidate, "skip-validate", false, "Skip validation step")

	return cmd
}

// resolveOptions contains all options for the resolve command
type resolveOptions struct {
	maxTokens    int
	aiMode       bool
	verbose      bool
	dryRun       bool
	confidence   float64
	apiKey       string
	autoApply    bool
	skipFormat   bool
	skipValidate bool
}

// resolveResult represents the complete result of the resolve pipeline
type resolveResult struct {
	Success           bool     `json:"success"`
	Stage             string   `json:"stage"`
	ConflictsDetected int      `json:"conflicts_detected"`
	ConflictsResolved int      `json:"conflicts_resolved"`
	FilesModified     []string `json:"files_modified"`
	AIConfidence      float64  `json:"ai_confidence,omitempty"`
	ValidationPassed  bool     `json:"validation_passed"`
	FormattingApplied bool     `json:"formatting_applied"`
	ErrorMessage      string   `json:"error_message,omitempty"`
	Summary           string   `json:"summary"`
}

// executeResolveCommand implements the complete conflict resolution pipeline
func executeResolveCommand(opts resolveOptions) error {
	repoPath, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	if opts.verbose {
		fmt.Println("🔍 Starting automated conflict resolution pipeline...")
	}

	result := &resolveResult{
		Stage: "detection",
	}

	// Step 1: Detect conflicts
	if opts.verbose {
		fmt.Println("📋 Step 1: Detecting conflicts...")
	}

	detectResult, err := commands.DetectConflicts(repoPath)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("conflict detection failed: %v", err)
		return outputResolveResult(result)
	}

	if detectResult.ConflictReport == nil || len(detectResult.ConflictReport.ConflictedFiles) == 0 {
		result.Success = true
		result.Summary = "No conflicts detected"
		if opts.verbose {
			fmt.Println("✅ No conflicts detected")
		}
		return outputResolveResult(result)
	}

	result.ConflictsDetected = len(detectResult.ConflictReport.ConflictedFiles)
	if opts.verbose {
		fmt.Printf("📋 Found %d conflicted files with %d total conflicts\n",
			len(detectResult.ConflictReport.ConflictedFiles),
			detectResult.ConflictReport.TotalConflicts)
	}

	if !opts.aiMode {
		result.Summary = fmt.Sprintf("Found %d conflicts. Use --ai flag to resolve with AI assistance.", result.ConflictsDetected)
		if opts.verbose {
			fmt.Println(result.Summary)
		}
		return outputResolveResult(result)
	}

	// Step 2: AI Resolution
	if opts.verbose {
		fmt.Println("🤖 Step 2: Generating AI resolutions...")
	}

	result.Stage = "ai_resolution"
	aiResult, err := resolveWithAI(detectResult, opts, repoPath)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("AI resolution failed: %v", err)
		return outputResolveResult(result)
	}

	result.ConflictsResolved = aiResult.AppliedResolutions
	result.AIConfidence = aiResult.AIResponse.OverallConfidence
	result.FilesModified = aiResult.ApplicationResult.ModifiedFiles

	if opts.verbose {
		fmt.Printf("🤖 Applied %d resolutions with overall confidence %.2f\n",
			result.ConflictsResolved, result.AIConfidence)
	}

	if opts.dryRun {
		result.Success = true
		result.Summary = fmt.Sprintf("Dry run: Would resolve %d conflicts with AI assistance", result.ConflictsDetected)
		return outputResolveResult(result)
	}

	// Step 3: Format files (optional)
	if !opts.skipFormat && result.ConflictsResolved > 0 {
		if opts.verbose {
			fmt.Println("🎨 Step 3: Formatting resolved files...")
		}

		result.Stage = "formatting"
		formatResult, err := commands.FormatFiles(repoPath, result.FilesModified)
		if err != nil {
			if opts.verbose {
				fmt.Printf("⚠️  Formatting failed but continuing: %v\n", err)
			}
		} else {
			result.FormattingApplied = formatResult.Success
			if opts.verbose && formatResult.Success {
				fmt.Printf("🎨 Formatted %d files\n", formatResult.Summary.FilesFormatted)
			}
		}
	}

	// Step 4: Validate project (optional)
	if !opts.skipValidate {
		if opts.verbose {
			fmt.Println("✅ Step 4: Validating project...")
		}

		result.Stage = "validation"
		err = commands.ValidateProject(repoPath, "", 300, opts.verbose)
		if err != nil {
			if opts.verbose {
				fmt.Printf("⚠️  Validation failed but continuing: %v\n", err)
			}
			result.ValidationPassed = false
		} else {
			result.ValidationPassed = true
			if opts.verbose {
				fmt.Println("✅ Project validation completed")
			}
		}
	}

	// Final summary
	result.Success = true
	result.Stage = "completed"
	result.Summary = fmt.Sprintf("Successfully resolved %d/%d conflicts", result.ConflictsResolved, result.ConflictsDetected)

	if opts.verbose {
		fmt.Println("\n🎉 Conflict resolution pipeline completed!")
		fmt.Printf("   Conflicts resolved: %d/%d\n", result.ConflictsResolved, result.ConflictsDetected)
		fmt.Printf("   Files modified: %d\n", len(result.FilesModified))
		if !opts.skipFormat {
			fmt.Printf("   Formatting applied: %t\n", result.FormattingApplied)
		}
		if !opts.skipValidate {
			fmt.Printf("   Validation passed: %t\n", result.ValidationPassed)
		}
		fmt.Println("\n📝 Next steps:")
		fmt.Println("   1. Review the resolved conflicts")
		fmt.Println("   2. Test your changes")
		fmt.Println("   3. Commit the resolved conflicts")
	}

	return outputResolveResult(result)
}

// resolveWithAI handles the AI-powered conflict resolution
func resolveWithAI(detectResult *commands.DetectResult, opts resolveOptions, repoPath string) (*commands.AIApplyResult, error) {
	// Validate API key
	apiKey := opts.apiKey
	if apiKey == "" {
		apiKey = os.Getenv("CLAUDE_CODE_OAUTH_TOKEN")
		if apiKey == "" {
			return nil, fmt.Errorf("API key not provided. Set CLAUDE_CODE_OAUTH_TOKEN environment variable or use --api-key flag")
		}
	}

	// Create AI apply options
	aiOptions := commands.AIApplyOptions{
		RepoPath:       repoPath,
		DryRun:         opts.dryRun,
		Verbose:        opts.verbose,
		AutoApply:      opts.autoApply,
		MinConfidence:  opts.confidence,
		BackupFiles:    true,
		MaxRetries:     3,
		TimeoutSeconds: 120,
	}

	// Create a temporary payload from detect result
	payloadData, err := json.Marshal(detectResult.ConflictPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal conflict payload: %w", err)
	}

	// Write payload to temporary file
	tmpFile, err := os.CreateTemp("", "syncwright-payload-*.json")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer func() {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
	}()

	if _, err := tmpFile.Write(payloadData); err != nil {
		return nil, fmt.Errorf("failed to write payload data: %w", err)
	}
	tmpFile.Close()

	aiOptions.PayloadFile = tmpFile.Name()

	// Execute AI application
	aiCmd, err := commands.NewAIApplyCommand(aiOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create AI apply command: %w", err)
	}
	return aiCmd.Execute()
}

// outputResolveResult outputs the final result of the resolve pipeline
func outputResolveResult(result *resolveResult) error {
	if result.ErrorMessage != "" {
		fmt.Fprintf(os.Stderr, "Error: %s\n", result.ErrorMessage)
		return fmt.Errorf(result.ErrorMessage)
	}

	// Always output the summary for user feedback
	if result.Summary != "" {
		fmt.Println(result.Summary)
	}

	return nil
}

func newBatchCmd() *cobra.Command {
	var (
		outputFile    string
		batchSize     int
		concurrency   int
		groupBy       string
		maxTokens     int
		timeoutSec    int
		apiKey        string
		apiEndpoint   string
		minConfidence float64
		autoApply     bool
		dryRun        bool
		verbose       bool
		progress      bool
		streaming     bool
		backupFiles   bool
		maxRetries    int
	)

	cmd := &cobra.Command{
		Use:   "batch",
		Short: "Process multiple conflict files simultaneously for improved performance",
		Long: `Batch processes multiple conflict files concurrently for better performance in large repositories.

The batch command optimizes the detect → payload → ai-apply workflow by:
- Grouping conflicts intelligently (by language, file, or size)
- Processing multiple batches concurrently
- Streaming results as they become available
- Providing detailed performance metrics

Grouping strategies:
  language  - Group conflicts by programming language (default)
  file      - Create one batch per file
  size      - Group by estimated token size
  none      - Sequential batching without grouping

Examples:
  # Basic batch processing with default settings
  syncwright batch --ai

  # High-performance processing with custom settings
  syncwright batch --ai --batch-size 15 --concurrency 5 --group-by language

  # Process with size-based grouping and progress display
  syncwright batch --ai --group-by size --max-tokens 40000 --progress

  # Dry run to preview batch organization
  syncwright batch --ai --dry-run --verbose --streaming`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate API key
			if apiKey == "" {
				apiKey = os.Getenv("CLAUDE_CODE_OAUTH_TOKEN")
				if apiKey == "" {
					return fmt.Errorf("API key not provided. Set CLAUDE_CODE_OAUTH_TOKEN environment variable or use --api-key flag")
				}
			}

			// Get current working directory
			repoPath, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}

			options := commands.BatchOptions{
				RepoPath:      repoPath,
				OutputFile:    outputFile,
				BatchSize:     batchSize,
				Concurrency:   concurrency,
				GroupBy:       groupBy,
				MaxTokens:     maxTokens,
				TimeoutSec:    timeoutSec,
				MinConfidence: minConfidence,
				AutoApply:     autoApply,
				DryRun:        dryRun,
				Verbose:       verbose,
				Progress:      progress,
				Streaming:     streaming,
				BackupFiles:   backupFiles,
				MaxRetries:    maxRetries,
			}

			batchCmd := commands.NewBatchCommand(options)
			result, err := batchCmd.Execute()

			if err != nil {
				return fmt.Errorf("batch processing failed: %w", err)
			}

			// Print summary if not already done
			if !verbose && result.Success {
				fmt.Printf("✅ Batch processing completed: %d/%d conflicts resolved\n",
					result.AppliedResolutions, result.TotalConflicts)
			}

			return nil
		},
	}

	// Core options
	cmd.Flags().StringVarP(&outputFile, "out", "o", "", "Output file for batch results (default: stdout)")
	cmd.Flags().IntVar(&batchSize, "batch-size", 10, "Number of conflicts per batch")
	cmd.Flags().IntVar(&concurrency, "concurrency", 3, "Number of concurrent batches to process")
	cmd.Flags().StringVar(&groupBy, "group-by", "language", "Grouping strategy: language, file, size, none")

	// Performance options
	cmd.Flags().IntVar(&maxTokens, "max-tokens", 50000, "Maximum tokens per batch")
	cmd.Flags().IntVar(&timeoutSec, "timeout", 300, "Timeout in seconds for batch processing")
	cmd.Flags().BoolVar(&progress, "progress", false, "Show progress bar during processing")
	cmd.Flags().BoolVar(&streaming, "streaming", false, "Stream results as batches complete")

	// AI options
	cmd.Flags().StringVar(&apiKey, "api-key", "", "Claude Code API key (or set CLAUDE_CODE_OAUTH_TOKEN env var)")
	cmd.Flags().StringVar(&apiEndpoint, "api-endpoint", "", "Claude Code API endpoint (uses default if not specified)")
	cmd.Flags().Float64Var(&minConfidence, "confidence", 0.7, "Minimum confidence threshold for applying resolutions")
	cmd.Flags().IntVar(&maxRetries, "max-retries", 3, "Maximum retry attempts for failed API requests")

	// Execution options
	cmd.Flags().BoolVar(&autoApply, "auto-apply", false, "Automatically apply resolutions without confirmation")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview batch organization and processing without applying changes")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output with detailed progress information")
	cmd.Flags().BoolVar(&backupFiles, "backup", true, "Create backup files before applying resolutions")

	return cmd
}
