package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/NeuBlink/syncwright/internal/claude"
	"github.com/NeuBlink/syncwright/internal/gitutils"
	"github.com/NeuBlink/syncwright/internal/logging"
	"github.com/NeuBlink/syncwright/internal/payload"
	"github.com/NeuBlink/syncwright/internal/validation"
	"go.uber.org/zap"
)

// AIApplyOptions contains options for the ai-apply command
type AIApplyOptions struct {
	PayloadFile    string
	RepoPath       string
	OutputFile     string
	DryRun         bool
	Verbose        bool
	AutoApply      bool
	MinConfidence  float64
	BackupFiles    bool
	MaxRetries     int
	TimeoutSeconds int
}

// AIApplyResult represents the result of the AI application
type AIApplyResult struct {
	Success            bool                          `json:"success"`
	ProcessedFiles     int                           `json:"processed_files"`
	AppliedResolutions int                           `json:"applied_resolutions"`
	SkippedResolutions int                           `json:"skipped_resolutions"`
	FailedResolutions  int                           `json:"failed_resolutions"`
	Resolutions        []gitutils.ConflictResolution `json:"resolutions"`
	ApplicationResult  *gitutils.ResolutionResult    `json:"application_result,omitempty"`
	ErrorMessage       string                        `json:"error_message,omitempty"`
	AIResponse         *AIResolveResponse            `json:"ai_response,omitempty"`
	ValidationResult   *validation.ValidationResult  `json:"validation_result,omitempty"`
}

// AIResolveResponse represents the response from Claude Code API
type AIResolveResponse struct {
	Success           bool                          `json:"success"`
	Resolutions       []gitutils.ConflictResolution `json:"resolutions"`
	OverallConfidence float64                       `json:"overall_confidence"`
	Reasoning         string                        `json:"reasoning,omitempty"`
	Warnings          []string                      `json:"warnings,omitempty"`
	ErrorMessage      string                        `json:"error_message,omitempty"`
	RequestID         string                        `json:"request_id,omitempty"`
	ProcessingTime    float64                       `json:"processing_time,omitempty"`
}

// AIApplyCommand implements the ai-apply subcommand
type AIApplyCommand struct {
	options  AIApplyOptions
	resolver *claude.ConflictResolver
}

// NewAIApplyCommand creates a new ai-apply command
func NewAIApplyCommand(options AIApplyOptions) (*AIApplyCommand, error) {
	// Set defaults
	if options.MinConfidence == 0 {
		options.MinConfidence = 0.7
	}
	if options.MaxRetries == 0 {
		options.MaxRetries = 3
	}
	if options.TimeoutSeconds == 0 {
		options.TimeoutSeconds = 300 // Extended for Claude CLI operations
	}
	if options.RepoPath == "" {
		if wd, err := os.Getwd(); err == nil {
			options.RepoPath = wd
		}
	}

	// Create ConflictResolver with proper Claude CLI configuration
	config := &claude.ConflictResolverConfig{
		ClaudeConfig: &claude.Config{
			CLIPath:          "claude",
			PrintMode:        true,
			OutputFormat:     "json",
			MaxTurns:         3,
			TimeoutSeconds:   options.TimeoutSeconds,
			AllowedTools:     []string{"Read", "Write", "Edit", "MultiEdit", "Bash", "Grep", "Glob", "LS"},
			WorkingDirectory: options.RepoPath,
			Verbose:          options.Verbose,
		},
		RepoPath:         options.RepoPath,
		MinConfidence:    options.MinConfidence,
		MaxBatchSize:     10,
		IncludeReasoning: true,
		Verbose:          options.Verbose,
		EnableMultiTurn:  false, // Disable for batch processing
	}

	resolver, err := claude.NewConflictResolver(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create conflict resolver: %w", err)
	}

	return &AIApplyCommand{
		options:  options,
		resolver: resolver,
	}, nil
}

// Execute runs the ai-apply command
func (a *AIApplyCommand) Execute() (*AIApplyResult, error) {
	result := &AIApplyResult{}

	// Step 1: Load and validate input
	conflictPayload, err := a.prepareInput(result)
	if err != nil {
		return result, err
	}

	// Step 2: Get AI resolutions
	aiResponse, err := a.getAIResolutions(conflictPayload, result)
	if err != nil {
		return result, err
	}

	// Step 3: Process and filter resolutions
	filteredResolutions := a.processResolutions(aiResponse, result)

	// Step 4: Apply resolutions if appropriate
	err = a.applyResolutionsIfNeeded(filteredResolutions, result)
	if err != nil {
		return result, err
	}

	// Step 5: Finalize results
	result.ProcessedFiles = len(conflictPayload.Files)
	if err := a.outputResults(result); err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to output results: %v", err)
		return result, err
	}

	return result, nil
}

// prepareInput loads payload and validates API key
func (a *AIApplyCommand) prepareInput(result *AIApplyResult) (*payload.ConflictPayload, error) {
	// Load payload
	conflictPayload, err := a.loadPayload()
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to load payload: %v", err)
		return nil, err
	}

	logging.Logger.ConflictResolution("payload_loaded", zap.Int("conflicted_files", len(conflictPayload.Files)))
	if a.options.Verbose {
		fmt.Printf("Loaded payload with %d conflicted files\n", len(conflictPayload.Files))
	}

	// Validate Claude CLI availability
	if !a.resolver.IsAvailable() {
		result.ErrorMessage = "Claude Code CLI is not available. Please ensure 'claude' is installed and in your PATH"
		return nil, fmt.Errorf("Claude CLI not available")
	}

	// Create backup if requested
	if a.options.BackupFiles {
		if err := a.createBackups(conflictPayload); err != nil {
			result.ErrorMessage = fmt.Sprintf("Failed to create backups: %v", err)
			return nil, err
		}
	}

	return conflictPayload, nil
}

// getAIResolutions sends payload to AI and gets response
func (a *AIApplyCommand) getAIResolutions(
	conflictPayload *payload.ConflictPayload,
	result *AIApplyResult,
) (*AIResolveResponse, error) {
	aiResponse, err := a.sendToAI(conflictPayload)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to get AI resolution: %v", err)
		return nil, err
	}

	result.AIResponse = aiResponse

	if !aiResponse.Success {
		result.ErrorMessage = fmt.Sprintf("AI resolution failed: %s", aiResponse.ErrorMessage)
		return nil, fmt.Errorf("AI resolution failed")
	}

	logging.Logger.ConflictResolution("ai_resolutions_generated",
		zap.Int("resolutions_count", len(aiResponse.Resolutions)),
		zap.Float64("overall_confidence", aiResponse.OverallConfidence))
	if a.options.Verbose {
		fmt.Printf("AI generated %d resolutions with overall confidence %.2f\n",
			len(aiResponse.Resolutions), aiResponse.OverallConfidence)
	}

	return aiResponse, nil
}

// processResolutions filters resolutions by confidence
func (a *AIApplyCommand) processResolutions(
	aiResponse *AIResolveResponse,
	result *AIApplyResult,
) []gitutils.ConflictResolution {
	filteredResolutions := a.filterResolutionsByConfidence(aiResponse.Resolutions)
	result.Resolutions = filteredResolutions
	result.SkippedResolutions = len(aiResponse.Resolutions) - len(filteredResolutions)

	if result.SkippedResolutions > 0 {
		logging.Logger.ConflictResolution("resolutions_skipped",
			zap.Int("skipped_count", result.SkippedResolutions),
			zap.Float64("min_confidence", a.options.MinConfidence))
	}
	if a.options.Verbose && result.SkippedResolutions > 0 {
		fmt.Printf("Skipped %d resolutions due to low confidence (< %.2f)\n",
			result.SkippedResolutions, a.options.MinConfidence)
	}

	return filteredResolutions
}

// applyResolutionsIfNeeded applies resolutions based on options
func (a *AIApplyCommand) applyResolutionsIfNeeded(
	filteredResolutions []gitutils.ConflictResolution,
	result *AIApplyResult,
) error {
	if a.options.DryRun {
		logging.Logger.ConflictResolution("dry_run_completed", zap.Int("would_apply_count", len(filteredResolutions)))
		result.Success = true
		fmt.Printf("Dry run: Would apply %d resolutions\n", len(filteredResolutions))
		return nil
	}

	if len(filteredResolutions) == 0 {
		result.Success = true
		return nil
	}

	if a.options.AutoApply {
		return a.applyResolutionsAutomatically(filteredResolutions, result)
	}

	return a.applyResolutionsInteractively(filteredResolutions, result)
}

// applyResolutionsAutomatically applies resolutions without user interaction
func (a *AIApplyCommand) applyResolutionsAutomatically(
	filteredResolutions []gitutils.ConflictResolution,
	result *AIApplyResult,
) error {
	applicationResult, err := gitutils.ApplyResolutions(a.options.RepoPath, filteredResolutions)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to apply resolutions: %v", err)
		return err
	}

	result.ApplicationResult = applicationResult
	result.AppliedResolutions = applicationResult.AppliedCount
	result.FailedResolutions = applicationResult.FailedCount
	result.Success = applicationResult.Success

	logging.Logger.ConflictResolution("resolutions_applied",
		zap.Int("applied_count", result.AppliedResolutions),
		zap.Int("failed_count", result.FailedResolutions))
	if a.options.Verbose {
		fmt.Printf("Applied %d resolutions, %d failed\n",
			result.AppliedResolutions, result.FailedResolutions)
	}

	return nil
}

// applyResolutionsInteractively applies resolutions with user confirmation
func (a *AIApplyCommand) applyResolutionsInteractively(
	filteredResolutions []gitutils.ConflictResolution,
	result *AIApplyResult,
) error {
	if a.askForConfirmation(filteredResolutions) {
		applicationResult, err := gitutils.ApplyResolutions(a.options.RepoPath, filteredResolutions)
		if err != nil {
			result.ErrorMessage = fmt.Sprintf("Failed to apply resolutions: %v", err)
			return err
		}

		result.ApplicationResult = applicationResult
		result.AppliedResolutions = applicationResult.AppliedCount
		result.FailedResolutions = applicationResult.FailedCount
		result.Success = applicationResult.Success
	} else {
		logging.Logger.ConflictResolution("application_cancelled", zap.String("reason", "user_choice"))
		result.Success = true
		fmt.Println("Resolution application cancelled by user")
	}

	return nil
}

// loadPayload loads and validates the conflict payload from file or stdin
func (a *AIApplyCommand) loadPayload() (*payload.ConflictPayload, error) {
	var data []byte
	var err error

	if a.options.PayloadFile == "" || a.options.PayloadFile == "-" {
		// Read from stdin
		data, err = io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("failed to read from stdin: %w", err)
		}
	} else {
		// Read from file
		data, err = os.ReadFile(a.options.PayloadFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", a.options.PayloadFile, err)
		}
	}

	// Validate payload with security checks
	validator := validation.NewPayloadValidator()
	validatedPayload, validationResult, err := validator.ValidateAndSanitize(data)
	if err != nil {
		if validationResult != nil {
			logging.Logger.ErrorSafe("Payload validation failed",
				zap.Int("error_count", len(validationResult.Errors)),
				zap.Error(err))
		}
		if a.options.Verbose && validationResult != nil {
			fmt.Printf("Validation failed with %d errors:\n", len(validationResult.Errors))
			for _, valErr := range validationResult.Errors {
				fmt.Printf("  - %s: %s\n", valErr.Field, valErr.Message)
			}
		}
		return nil, fmt.Errorf("payload validation failed: %w", err)
	}

	if validationResult != nil {
		logging.Logger.ConflictResolution("payload_validation_successful",
			zap.Int("total_files", validationResult.Summary.TotalFiles),
			zap.Int("total_conflicts", validationResult.Summary.TotalConflicts),
			zap.Int("payload_size_bytes", validationResult.Summary.PayloadSize))
	}
	if a.options.Verbose && validationResult != nil {
		fmt.Printf("Payload validation successful: %d files, %d conflicts, %d bytes\n",
			validationResult.Summary.TotalFiles,
			validationResult.Summary.TotalConflicts,
			validationResult.Summary.PayloadSize)
	}

	// Convert validated payload back to original format
	return a.convertValidatedPayload(validatedPayload), nil
}

// convertValidatedPayload converts a validated payload back to the original format
func (a *AIApplyCommand) convertValidatedPayload(validated *validation.ValidatedConflictPayload) *payload.ConflictPayload {
	result := &payload.ConflictPayload{
		Metadata: payload.PayloadMetadata{
			Timestamp:      validated.Metadata.Timestamp,
			RepoPath:       validated.Metadata.RepoPath,
			TotalFiles:     validated.Metadata.TotalFiles,
			TotalConflicts: validated.Metadata.TotalConflicts,
			Version:        validated.Metadata.Version,
		},
	}

	for _, validatedFile := range validated.Files {
		file := payload.ConflictFilePayload{
			Path:     validatedFile.Path,
			Language: validatedFile.Language,
			Context: payload.FileContext{
				BeforeLines: validatedFile.Context.BeforeLines,
				AfterLines:  validatedFile.Context.AfterLines,
			},
		}

		for _, validatedConflict := range validatedFile.Conflicts {
			conflict := payload.ConflictHunkPayload{
				ID:          validatedConflict.ID,
				StartLine:   validatedConflict.StartLine,
				EndLine:     validatedConflict.EndLine,
				OursLines:   validatedConflict.OursLines,
				TheirsLines: validatedConflict.TheirsLines,
				BaseLines:   validatedConflict.BaseLines,
			}
			file.Conflicts = append(file.Conflicts, conflict)
		}

		result.Files = append(result.Files, file)
	}

	return result
}

// sendToAI sends the conflict payload to Claude Code CLI via ConflictResolver
func (a *AIApplyCommand) sendToAI(conflictPayload *payload.ConflictPayload) (*AIResolveResponse, error) {
	logging.Logger.ConflictResolution("sending_to_ai", zap.Int("files_count", len(conflictPayload.Files)))
	if a.options.Verbose {
		fmt.Printf("Sending conflict resolution request to Claude CLI\n")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(a.options.TimeoutSeconds)*time.Second)
	defer cancel()

	// Use ConflictResolver to process the conflicts
	result, err := a.resolver.ResolveConflicts(ctx, conflictPayload)
	if err != nil {
		return nil, fmt.Errorf("conflict resolution failed: %w", err)
	}

	if !result.Success {
		return &AIResolveResponse{
			Success:      false,
			ErrorMessage: result.ErrorMessage,
		}, nil
	}

	// Convert ConflictResolver result to AIResolveResponse format
	aiResponse := &AIResolveResponse{
		Success:           result.Success,
		Resolutions:       result.Resolutions,
		OverallConfidence: result.OverallConfidence,
		ProcessingTime:    result.ProcessingTime.Seconds(),
		Warnings:          result.Warnings,
	}

	// Add reasoning from resolutions if available
	if len(result.Resolutions) > 0 && result.Resolutions[0].Reasoning != "" {
		var reasonings []string
		for _, resolution := range result.Resolutions {
			if resolution.Reasoning != "" {
				reasonings = append(reasonings, fmt.Sprintf("%s: %s", resolution.FilePath, resolution.Reasoning))
			}
		}
		if len(reasonings) > 0 {
			aiResponse.Reasoning = strings.Join(reasonings, "; ")
		}
	}

	logging.Logger.ConflictResolution("ai_response_received",
		zap.Int("resolved_conflicts", len(aiResponse.Resolutions)),
		zap.Float64("overall_confidence", aiResponse.OverallConfidence),
		zap.Float64("processing_time_seconds", aiResponse.ProcessingTime))
	if a.options.Verbose {
		fmt.Printf("Claude CLI resolved %d conflicts with overall confidence %.2f\n",
			len(aiResponse.Resolutions), aiResponse.OverallConfidence)
	}

	return aiResponse, nil
}

// filterResolutionsByConfidence filters resolutions based on confidence threshold
func (a *AIApplyCommand) filterResolutionsByConfidence(
	resolutions []gitutils.ConflictResolution,
) []gitutils.ConflictResolution {
	var filtered []gitutils.ConflictResolution

	for _, resolution := range resolutions {
		if resolution.Confidence >= a.options.MinConfidence {
			filtered = append(filtered, resolution)
		}
	}

	return filtered
}

// createBackups creates backup files before applying resolutions
func (a *AIApplyCommand) createBackups(conflictPayload *payload.ConflictPayload) error {
	var filesToBackup []string

	for _, file := range conflictPayload.Files {
		filesToBackup = append(filesToBackup, file.Path)
	}

	for _, filePath := range filesToBackup {
		if err := gitutils.CreateBackup(a.options.RepoPath, filePath); err != nil {
			return fmt.Errorf("failed to backup %s: %w", filePath, err)
		}
	}

	logging.Logger.InfoSafe("Backup files created", zap.Int("file_count", len(filesToBackup)))
	if a.options.Verbose {
		fmt.Printf("Created backups for %d files\n", len(filesToBackup))
	}

	return nil
}

// askForConfirmation asks the user for confirmation before applying resolutions
func (a *AIApplyCommand) askForConfirmation(resolutions []gitutils.ConflictResolution) bool {
	fmt.Printf("\nAI has generated %d conflict resolutions.\n", len(resolutions))
	fmt.Println("Preview of resolutions:")

	for i, resolution := range resolutions {
		if i >= 3 { // Show only first 3 resolutions
			fmt.Printf("... and %d more resolutions\n", len(resolutions)-3)
			break
		}

		fmt.Printf("\n%d. %s (lines %d-%d, confidence: %.2f)\n",
			i+1, resolution.FilePath, resolution.StartLine, resolution.EndLine, resolution.Confidence)

		if resolution.Reasoning != "" {
			fmt.Printf("   Reasoning: %s\n", resolution.Reasoning)
		}

		// Show first few lines of resolution
		if len(resolution.ResolvedLines) > 0 {
			fmt.Printf("   Resolution preview:\n")
			for j, line := range resolution.ResolvedLines {
				if j >= 2 {
					fmt.Printf("   ... (%d more lines)\n", len(resolution.ResolvedLines)-2)
					break
				}
				fmt.Printf("   + %s\n", line)
			}
		}
	}

	fmt.Print("\nApply these resolutions? [y/N]: ")
	var response string
	if _, err := fmt.Scanln(&response); err != nil {
		// If input fails, default to no
		response = "n"
	}

	return strings.ToLower(strings.TrimSpace(response)) == "y"
}

// outputResults outputs the AI application results
func (a *AIApplyCommand) outputResults(result *AIApplyResult) error {
	if a.options.OutputFile != "" {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal results: %w", err)
		}

		err = os.WriteFile(a.options.OutputFile, data, 0600)
		if err != nil {
			return fmt.Errorf("failed to write results to file: %w", err)
		}

		logging.Logger.InfoSafe("AI apply results written to file", zap.String("output_file", a.options.OutputFile))
		if a.options.Verbose {
			fmt.Printf("Results written to: %s\n", a.options.OutputFile)
		}
	}

	return nil
}

// ApplyAIResolutions is a convenience function for applying AI resolutions
func ApplyAIResolutions(payloadFile, repoPath string) (*AIApplyResult, error) {
	options := AIApplyOptions{
		PayloadFile: payloadFile,
		RepoPath:    repoPath,
		AutoApply:   false,
		DryRun:      false,
		Verbose:     true,
		BackupFiles: true,
	}

	cmd, err := NewAIApplyCommand(options)
	if err != nil {
		return nil, fmt.Errorf("failed to create AI apply command: %w", err)
	}
	return cmd.Execute()
}

// ApplyAIResolutionsDryRun is a convenience function for dry-run AI resolution
func ApplyAIResolutionsDryRun(payloadFile, repoPath string) (*AIApplyResult, error) {
	options := AIApplyOptions{
		PayloadFile: payloadFile,
		RepoPath:    repoPath,
		DryRun:      true,
		Verbose:     true,
		BackupFiles: false,
	}

	cmd, err := NewAIApplyCommand(options)
	if err != nil {
		return nil, fmt.Errorf("failed to create AI apply command: %w", err)
	}
	return cmd.Execute()
}

// Close cleans up the AI apply command resources
func (a *AIApplyCommand) Close() error {
	if a.resolver != nil {
		return a.resolver.Close()
	}
	return nil
}
