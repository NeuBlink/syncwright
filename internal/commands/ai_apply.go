package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/NeuBlink/syncwright/internal/gitutils"
	"github.com/NeuBlink/syncwright/internal/payload"
)

// AIApplyOptions contains options for the ai-apply command
type AIApplyOptions struct {
	PayloadFile    string
	RepoPath       string
	APIKey         string
	APIEndpoint    string
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
}

// AIResolveRequest represents the request sent to Claude Code API
type AIResolveRequest struct {
	Payload     *payload.ConflictPayload `json:"payload"`
	Preferences AIPreferences            `json:"preferences"`
	Context     string                   `json:"context"`
}

// AIPreferences contains preferences for AI resolution
type AIPreferences struct {
	MinConfidence    float64 `json:"min_confidence"`
	PreferExplicit   bool    `json:"prefer_explicit"`
	IncludeReasoning bool    `json:"include_reasoning"`
	PreserveBoth     bool    `json:"preserve_both_when_uncertain"`
	MaxResolutions   int     `json:"max_resolutions"`
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
	options AIApplyOptions
	client  *http.Client
}

// NewAIApplyCommand creates a new ai-apply command
func NewAIApplyCommand(options AIApplyOptions) *AIApplyCommand {
	// Set defaults
	if options.APIEndpoint == "" {
		options.APIEndpoint = "https://api.anthropic.com/v1/claude-code/resolve-conflicts"
	}
	if options.MinConfidence == 0 {
		options.MinConfidence = 0.7
	}
	if options.MaxRetries == 0 {
		options.MaxRetries = 3
	}
	if options.TimeoutSeconds == 0 {
		options.TimeoutSeconds = 120
	}
	if options.RepoPath == "" {
		if wd, err := os.Getwd(); err == nil {
			options.RepoPath = wd
		}
	}

	client := &http.Client{
		Timeout: time.Duration(options.TimeoutSeconds) * time.Second,
	}

	return &AIApplyCommand{
		options: options,
		client:  client,
	}
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

	if a.options.Verbose {
		fmt.Printf("Loaded payload with %d conflicted files\n", len(conflictPayload.Files))
	}

	// Validate API key
	if a.options.APIKey == "" {
		a.options.APIKey = os.Getenv("CLAUDE_API_KEY")
		if a.options.APIKey == "" {
			result.ErrorMessage = "API key not provided. Set CLAUDE_API_KEY environment variable or use --api-key flag"
			return nil, fmt.Errorf("missing API key")
		}
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
func (a *AIApplyCommand) getAIResolutions(conflictPayload *payload.ConflictPayload, result *AIApplyResult) (*AIResolveResponse, error) {
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

	if a.options.Verbose {
		fmt.Printf("AI generated %d resolutions with overall confidence %.2f\n",
			len(aiResponse.Resolutions), aiResponse.OverallConfidence)
	}

	return aiResponse, nil
}

// processResolutions filters resolutions by confidence
func (a *AIApplyCommand) processResolutions(aiResponse *AIResolveResponse, result *AIApplyResult) []gitutils.ConflictResolution {
	filteredResolutions := a.filterResolutionsByConfidence(aiResponse.Resolutions)
	result.Resolutions = filteredResolutions
	result.SkippedResolutions = len(aiResponse.Resolutions) - len(filteredResolutions)

	if a.options.Verbose && result.SkippedResolutions > 0 {
		fmt.Printf("Skipped %d resolutions due to low confidence (< %.2f)\n",
			result.SkippedResolutions, a.options.MinConfidence)
	}

	return filteredResolutions
}

// applyResolutionsIfNeeded applies resolutions based on options
func (a *AIApplyCommand) applyResolutionsIfNeeded(filteredResolutions []gitutils.ConflictResolution, result *AIApplyResult) error {
	if a.options.DryRun {
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
func (a *AIApplyCommand) applyResolutionsAutomatically(filteredResolutions []gitutils.ConflictResolution, result *AIApplyResult) error {
	applicationResult, err := gitutils.ApplyResolutions(a.options.RepoPath, filteredResolutions)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to apply resolutions: %v", err)
		return err
	}

	result.ApplicationResult = applicationResult
	result.AppliedResolutions = applicationResult.AppliedCount
	result.FailedResolutions = applicationResult.FailedCount
	result.Success = applicationResult.Success

	if a.options.Verbose {
		fmt.Printf("Applied %d resolutions, %d failed\n",
			result.AppliedResolutions, result.FailedResolutions)
	}

	return nil
}

// applyResolutionsInteractively applies resolutions with user confirmation
func (a *AIApplyCommand) applyResolutionsInteractively(filteredResolutions []gitutils.ConflictResolution, result *AIApplyResult) error {
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
		result.Success = true
		fmt.Println("Resolution application cancelled by user")
	}

	return nil
}

// loadPayload loads the conflict payload from file or stdin
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

	if len(data) == 0 {
		return nil, fmt.Errorf("empty payload data")
	}

	return payload.FromJSON(data)
}

// sendToAI sends the conflict payload to Claude Code API
func (a *AIApplyCommand) sendToAI(conflictPayload *payload.ConflictPayload) (*AIResolveResponse, error) {
	request := AIResolveRequest{
		Payload: conflictPayload,
		Preferences: AIPreferences{
			MinConfidence:    a.options.MinConfidence,
			PreferExplicit:   true,
			IncludeReasoning: true,
			PreserveBoth:     false,
			MaxResolutions:   100,
		},
		Context: "Please resolve these merge conflicts intelligently, preserving the intent of both sides when possible.",
	}

	requestData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt < a.options.MaxRetries; attempt++ {
		if attempt > 0 {
			if a.options.Verbose {
				fmt.Printf("Retrying API request (attempt %d/%d)\n", attempt+1, a.options.MaxRetries)
			}
			time.Sleep(time.Duration(attempt) * time.Second)
		}

		req, err := http.NewRequest("POST", a.options.APIEndpoint, bytes.NewReader(requestData))
		if err != nil {
			lastErr = fmt.Errorf("failed to create request: %w", err)
			continue
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+a.options.APIKey)
		req.Header.Set("User-Agent", "Syncwright/1.0.0")

		if a.options.Verbose {
			fmt.Printf("Sending request to AI API: %s\n", a.options.APIEndpoint)
		}

		resp, err := a.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			continue
		}
		defer resp.Body.Close()

		responseData, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = fmt.Errorf("failed to read response: %w", err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(responseData))
			continue
		}

		var aiResponse AIResolveResponse
		if err := json.Unmarshal(responseData, &aiResponse); err != nil {
			lastErr = fmt.Errorf("failed to unmarshal response: %w", err)
			continue
		}

		return &aiResponse, nil
	}

	return nil, fmt.Errorf("all retry attempts failed, last error: %w", lastErr)
}

// filterResolutionsByConfidence filters resolutions based on confidence threshold
func (a *AIApplyCommand) filterResolutionsByConfidence(resolutions []gitutils.ConflictResolution) []gitutils.ConflictResolution {
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

		if a.options.Verbose {
			fmt.Printf("Results written to: %s\n", a.options.OutputFile)
		}
	}

	return nil
}

// ApplyAIResolutions is a convenience function for applying AI resolutions
func ApplyAIResolutions(payloadFile, repoPath, apiKey string) (*AIApplyResult, error) {
	options := AIApplyOptions{
		PayloadFile: payloadFile,
		RepoPath:    repoPath,
		APIKey:      apiKey,
		AutoApply:   false,
		DryRun:      false,
		Verbose:     true,
		BackupFiles: true,
	}

	cmd := NewAIApplyCommand(options)
	return cmd.Execute()
}

// ApplyAIResolutionsDryRun is a convenience function for dry-run AI resolution
func ApplyAIResolutionsDryRun(payloadFile, repoPath, apiKey string) (*AIApplyResult, error) {
	options := AIApplyOptions{
		PayloadFile: payloadFile,
		RepoPath:    repoPath,
		APIKey:      apiKey,
		DryRun:      true,
		Verbose:     true,
		BackupFiles: false,
	}

	cmd := NewAIApplyCommand(options)
	return cmd.Execute()
}
