package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/NeuBlink/syncwright/internal/gitutils"
	"github.com/NeuBlink/syncwright/internal/logging"
	"github.com/NeuBlink/syncwright/internal/payload"
	"go.uber.org/zap"
)

// ConflictResolver handles conflict resolution using Claude Code CLI
type ConflictResolver struct {
	client             *ClaudeClient
	repoPath           string
	minConfidence      float64
	maxBatchSize       int
	includeReasoning   bool
	verbose            bool
	enableMultiTurn    bool
	maxTurns           int
	multiTurnThreshold float64
}

// ConflictResolverConfig contains configuration for the conflict resolver
type ConflictResolverConfig struct {
	ClaudeConfig       *Config
	RepoPath           string
	MinConfidence      float64
	MaxBatchSize       int
	IncludeReasoning   bool
	Verbose            bool
	EnableMultiTurn    bool    // Enable multi-turn conversations for low-confidence conflicts
	MaxTurns           int     // Maximum number of conversation turns
	MultiTurnThreshold float64 // Confidence threshold below which to use multi-turn
}

// ResolverResult contains the results of conflict resolution
type ResolverResult struct {
	Success            bool                          `json:"success"`
	ProcessedFiles     int                           `json:"processed_files"`
	ProcessedConflicts int                           `json:"processed_conflicts"`
	Resolutions        []gitutils.ConflictResolution `json:"resolutions"`
	HighConfidence     []gitutils.ConflictResolution `json:"high_confidence"`
	LowConfidence      []gitutils.ConflictResolution `json:"low_confidence"`
	OverallConfidence  float64                       `json:"overall_confidence"`
	ProcessingTime     time.Duration                 `json:"processing_time"`
	ErrorMessage       string                        `json:"error_message,omitempty"`
	Warnings           []string                      `json:"warnings,omitempty"`
}

// NewConflictResolver creates a new conflict resolver
func NewConflictResolver(config *ConflictResolverConfig) (*ConflictResolver, error) {
	if config == nil {
		return nil, fmt.Errorf("configuration cannot be nil")
	}

	if config.ClaudeConfig == nil {
		config.ClaudeConfig = DefaultConfig()
		// Ensure tools are restricted for conflict resolution
		config.ClaudeConfig.AllowedTools = []string{"Read", "Write", "Bash(git*)"}
	}

	if config.RepoPath == "" {
		return nil, fmt.Errorf("repository path cannot be empty")
	}

	if config.MinConfidence <= 0 {
		config.MinConfidence = 0.7
	}

	if config.MaxBatchSize <= 0 {
		config.MaxBatchSize = 10
	}

	// Set multi-turn defaults
	if config.MaxTurns <= 0 {
		config.MaxTurns = 3
	}
	if config.MultiTurnThreshold <= 0 {
		config.MultiTurnThreshold = 0.6
	}

	client, err := NewClaudeClient(config.ClaudeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Claude client: %w", err)
	}

	return &ConflictResolver{
		client:             client,
		repoPath:           config.RepoPath,
		minConfidence:      config.MinConfidence,
		maxBatchSize:       config.MaxBatchSize,
		includeReasoning:   config.IncludeReasoning,
		verbose:            config.Verbose,
		enableMultiTurn:    config.EnableMultiTurn,
		maxTurns:           config.MaxTurns,
		multiTurnThreshold: config.MultiTurnThreshold,
	}, nil
}

// ResolveConflicts resolves merge conflicts using Claude
func (r *ConflictResolver) ResolveConflicts(ctx context.Context, conflictPayload *payload.ConflictPayload) (*ResolverResult, error) {
	startTime := time.Now()

	result := &ResolverResult{
		ProcessedFiles: len(conflictPayload.Files),
		Resolutions:    make([]gitutils.ConflictResolution, 0),
		HighConfidence: make([]gitutils.ConflictResolution, 0),
		LowConfidence:  make([]gitutils.ConflictResolution, 0),
		Warnings:       make([]string, 0),
	}

	// Count total conflicts
	totalConflicts := 0
	for _, file := range conflictPayload.Files {
		totalConflicts += len(file.Conflicts)
	}
	result.ProcessedConflicts = totalConflicts

	logging.Logger.ConflictResolution("conflict_resolution_started",
		zap.Int("total_conflicts", totalConflicts),
		zap.Int("total_files", len(conflictPayload.Files)))
	if r.verbose {
		fmt.Printf("Resolving %d conflicts across %d files\n", totalConflicts, len(conflictPayload.Files))
	}

	// Process files in batches
	batches := r.createBatches(conflictPayload.Files)

	var allResolutions []gitutils.ConflictResolution
	var totalConfidence float64
	resolutionCount := 0

	for i, batch := range batches {
		logging.Logger.ConflictResolution("batch_processing",
			zap.Int("batch_number", i+1),
			zap.Int("total_batches", len(batches)),
			zap.Int("files_in_batch", len(batch)))
		if r.verbose {
			fmt.Printf("Processing batch %d/%d (%d files)\n", i+1, len(batches), len(batch))
		}

		batchResolutions, err := r.processBatch(ctx, batch, conflictPayload.Metadata.RepoPath)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Batch %d failed: %v", i+1, err))
			continue
		}

		allResolutions = append(allResolutions, batchResolutions...)

		// Calculate cumulative confidence
		for _, resolution := range batchResolutions {
			totalConfidence += resolution.Confidence
			resolutionCount++
		}
	}

	// Calculate overall confidence
	if resolutionCount > 0 {
		result.OverallConfidence = totalConfidence / float64(resolutionCount)
	}

	// Filter resolutions by confidence
	for _, resolution := range allResolutions {
		if resolution.Confidence >= r.minConfidence {
			result.HighConfidence = append(result.HighConfidence, resolution)
		} else {
			result.LowConfidence = append(result.LowConfidence, resolution)
		}
	}

	result.Resolutions = allResolutions
	result.ProcessingTime = time.Since(startTime)
	result.Success = len(result.Resolutions) > 0

	if r.verbose {
		fmt.Printf("Resolution complete: %d total, %d high confidence, %d low confidence (%.2f overall)\n",
			len(result.Resolutions), len(result.HighConfidence), len(result.LowConfidence), result.OverallConfidence)
	}

	return result, nil
}

// createBatches splits files into batches for processing
func (r *ConflictResolver) createBatches(files []payload.ConflictFilePayload) [][]payload.ConflictFilePayload {
	var batches [][]payload.ConflictFilePayload

	for i := 0; i < len(files); i += r.maxBatchSize {
		end := i + r.maxBatchSize
		if end > len(files) {
			end = len(files)
		}
		batches = append(batches, files[i:end])
	}

	return batches
}

// processBatch processes a batch of files for conflict resolution
func (r *ConflictResolver) processBatch(ctx context.Context, files []payload.ConflictFilePayload, repoPath string) ([]gitutils.ConflictResolution, error) {
	// Build the prompt for Claude
	prompt := r.buildConflictResolutionPrompt(files, repoPath)

	// Create context data
	contextData := map[string]interface{}{
		"repo_path":      repoPath,
		"conflict_count": r.countConflictsInBatch(files),
		"files":          r.getFilePathsFromBatch(files),
		"batch_size":     len(files),
	}

	// Execute the command
	response, err := r.client.ExecuteConflictResolution(ctx, prompt, contextData)
	if err != nil {
		return nil, fmt.Errorf("Claude execution failed: %w", err)
	}

	if !response.Success {
		return nil, fmt.Errorf("Claude reported failure: %s", response.ErrorMessage)
	}

	// Parse resolutions from Claude's response
	resolutions, err := r.parseResolutionsFromResponse(response, files)
	if err != nil {
		return nil, fmt.Errorf("failed to parse resolutions: %w", err)
	}

	// Apply Go-specific confidence validation and adjustment
	resolutions = r.adjustConfidenceWithGoValidation(resolutions, files)

	// Apply multi-turn conversation for low-confidence resolutions
	if r.enableMultiTurn {
		resolutions, err = r.applyMultiTurnRefinement(ctx, resolutions, files, repoPath)
		if err != nil {
			return nil, fmt.Errorf("multi-turn refinement failed: %w", err)
		}
	}

	return resolutions, nil
}

// buildConflictResolutionPrompt builds a prompt for Claude to resolve conflicts
func (r *ConflictResolver) buildConflictResolutionPrompt(files []payload.ConflictFilePayload, repoPath string) string {
	var prompt strings.Builder

	// Enhanced prompt with Go-specific guidance
	prompt.WriteString("I need help resolving merge conflicts in a Go codebase. ")
	prompt.WriteString("As an expert Go developer and AI conflict resolution specialist, ")
	prompt.WriteString("please analyze these conflicts with deep understanding of Go semantics, ")
	prompt.WriteString("function signatures, import management, and idiomatic patterns.\n\n")

	prompt.WriteString("**CONFLICT RESOLUTION EXPERTISE REQUIRED:**\n")
	prompt.WriteString("- Go function signatures and method receivers\n")
	prompt.WriteString("- Import statement management and aliasing\n")
	prompt.WriteString("- Package structure and visibility rules\n")
	prompt.WriteString("- Interface satisfaction and type compatibility\n")
	prompt.WriteString("- Error handling patterns and idiomatic Go code\n")
	prompt.WriteString("- Struct definitions and field ordering\n")
	prompt.WriteString("- Goroutine and channel usage patterns\n\n")

	if r.includeReasoning {
		prompt.WriteString("Provide detailed reasoning explaining your resolution decisions. ")
	}

	prompt.WriteString("For each conflict, provide:\n")
	prompt.WriteString("1. The file path\n")
	prompt.WriteString("2. Start and end line numbers\n")
	prompt.WriteString("3. The resolved content (without conflict markers)\n")
	prompt.WriteString("4. A confidence score (0.0 to 1.0) based on:\n")
	prompt.WriteString("   - Semantic correctness and Go idioms\n")
	prompt.WriteString("   - Function signature compatibility\n")
	prompt.WriteString("   - Import statement consistency\n")
	prompt.WriteString("   - Type safety and interface satisfaction\n")

	if r.includeReasoning {
		prompt.WriteString("5. Detailed reasoning including:\n")
		prompt.WriteString("   - Why this resolution preserves Go semantics\n")
		prompt.WriteString("   - How it handles function signatures and types\n")
		prompt.WriteString("   - Import management decisions\n")
		prompt.WriteString("   - Any potential compatibility concerns\n")
	}

	prompt.WriteString("\n**CONFIDENCE SCORING GUIDELINES:**\n")
	prompt.WriteString("- 0.9-1.0: Confident resolution, clear semantic intent, perfect Go idioms\n")
	prompt.WriteString("- 0.7-0.9: Good resolution, minor ambiguity, mostly idiomatic\n")
	prompt.WriteString("- 0.5-0.7: Reasonable resolution, some uncertainty, basic correctness\n")
	prompt.WriteString("- 0.3-0.5: Uncertain resolution, significant ambiguity, may need review\n")
	prompt.WriteString("- 0.0-0.3: Low confidence, complex conflict, recommend manual review\n\n")

	prompt.WriteString("Here are the conflicts to resolve:\n\n")

	// Add conflict details
	for _, file := range files {
		prompt.WriteString(fmt.Sprintf("File: %s\n", file.Path))
		prompt.WriteString("Conflicts:\n")

		for i, conflict := range file.Conflicts {
			prompt.WriteString(fmt.Sprintf("\nConflict %d (lines %d-%d):\n", i+1, conflict.StartLine, conflict.EndLine))

			// Show "ours" section
			prompt.WriteString("<<<<<<< HEAD\n")
			for _, line := range conflict.OursLines {
				prompt.WriteString(line + "\n")
			}

			// Show base section if available
			if len(conflict.BaseLines) > 0 {
				prompt.WriteString("||||||| base\n")
				for _, line := range conflict.BaseLines {
					prompt.WriteString(line + "\n")
				}
			}

			// Show separator
			prompt.WriteString("=======\n")

			// Show "theirs" section
			for _, line := range conflict.TheirsLines {
				prompt.WriteString(line + "\n")
			}
			prompt.WriteString(">>>>>>> branch\n")
		}

		// Add enhanced Go-specific context
		goContext := r.extractGoContext(file, repoPath)
		if goContext != "" {
			prompt.WriteString("\n**GO-SPECIFIC CONTEXT:**\n")
			prompt.WriteString(goContext)
			prompt.WriteString("\n")
		}

		// Add basic file context if available
		if len(file.Context.BeforeLines) > 0 || len(file.Context.AfterLines) > 0 {
			prompt.WriteString("\nSurrounding context:\n")
			if len(file.Context.BeforeLines) > 0 {
				prompt.WriteString("Before conflict:\n")
				for i, line := range file.Context.BeforeLines {
					prompt.WriteString(fmt.Sprintf("%d: %s\n", i+1, line))
				}
			}
			if len(file.Context.AfterLines) > 0 {
				prompt.WriteString("After conflict:\n")
				for i, line := range file.Context.AfterLines {
					prompt.WriteString(fmt.Sprintf("%d: %s\n", i+1, line))
				}
			}
		}

		prompt.WriteString("\n---\n\n")
	}

	prompt.WriteString("**RESPONSE FORMAT:**\n")
	prompt.WriteString("Provide resolutions in JSON format. Ensure all resolved Go code is syntactically correct and follows Go idioms:\n\n")
	prompt.WriteString("{\n")
	prompt.WriteString("  \"resolutions\": [\n")
	prompt.WriteString("    {\n")
	prompt.WriteString("      \"file_path\": \"path/to/file.go\",\n")
	prompt.WriteString("      \"start_line\": 10,\n")
	prompt.WriteString("      \"end_line\": 15,\n")
	prompt.WriteString("      \"resolved_lines\": [\"// Resolved Go code here\", \"func example() error {\", \"  return nil\", \"}\"],\n")
	prompt.WriteString("      \"confidence\": 0.85")

	if r.includeReasoning {
		prompt.WriteString(",\n      \"reasoning\": \"Merged function signatures by preserving both parameter types and ensuring interface compatibility. Maintained idiomatic error handling pattern.\"")
	}

	prompt.WriteString(",\n      \"go_specific_notes\": \"Additional Go-specific observations about imports, types, or semantics\"")
	prompt.WriteString("\n    }\n")
	prompt.WriteString("  ]\n")
	prompt.WriteString("}")

	return prompt.String()
}

// countConflictsInBatch counts total conflicts in a batch
func (r *ConflictResolver) countConflictsInBatch(files []payload.ConflictFilePayload) int {
	total := 0
	for _, file := range files {
		total += len(file.Conflicts)
	}
	return total
}

// getFilePathsFromBatch extracts file paths from a batch
func (r *ConflictResolver) getFilePathsFromBatch(files []payload.ConflictFilePayload) []string {
	paths := make([]string, len(files))
	for i, file := range files {
		paths[i] = file.Path
	}
	return paths
}

// parseResolutionsFromResponse parses conflict resolutions from Claude's response
func (r *ConflictResolver) parseResolutionsFromResponse(response *ClaudeResponse, files []payload.ConflictFilePayload) ([]gitutils.ConflictResolution, error) {
	// First try to parse as JSON
	if resolutions, err := r.parseJSONResolutions(response.Content); err == nil {
		return resolutions, nil
	}

	// If JSON parsing fails, try to extract from text
	return r.parseTextResolutions(response.Content, files)
}

// parseJSONResolutions parses resolutions from JSON format with Go-specific enhancements
func (r *ConflictResolver) parseJSONResolutions(content string) ([]gitutils.ConflictResolution, error) {
	// Look for JSON in the content
	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}")

	if jsonStart == -1 || jsonEnd == -1 || jsonStart >= jsonEnd {
		return nil, fmt.Errorf("no valid JSON found in response")
	}

	jsonContent := content[jsonStart : jsonEnd+1]

	// Enhanced response structure to handle Go-specific fields
	var response struct {
		Resolutions []struct {
			FilePath        string   `json:"file_path"`
			StartLine       int      `json:"start_line"`
			EndLine         int      `json:"end_line"`
			ResolvedLines   []string `json:"resolved_lines"`
			Confidence      float64  `json:"confidence"`
			Reasoning       string   `json:"reasoning,omitempty"`
			GoSpecificNotes string   `json:"go_specific_notes,omitempty"`
		} `json:"resolutions"`
	}

	if err := json.Unmarshal([]byte(jsonContent), &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Convert to standard resolution format and apply semantic validation
	var resolutions []gitutils.ConflictResolution
	for _, res := range response.Resolutions {
		resolution := gitutils.ConflictResolution{
			FilePath:      res.FilePath,
			StartLine:     res.StartLine,
			EndLine:       res.EndLine,
			ResolvedLines: res.ResolvedLines,
			Confidence:    res.Confidence,
			Reasoning:     res.Reasoning,
		}

		// Append Go-specific notes to reasoning if available
		if res.GoSpecificNotes != "" {
			if resolution.Reasoning != "" {
				resolution.Reasoning += " [Go-specific: " + res.GoSpecificNotes + "]"
			} else {
				resolution.Reasoning = "Go-specific: " + res.GoSpecificNotes
			}
		}

		// Apply semantic validation if this is a Go file
		if r.isGoFile(resolution.FilePath) {
			if err := r.validateSemanticCorrectness(resolution); err != nil {
				if r.verbose {
					fmt.Printf("Semantic validation warning for %s: %v\n", resolution.FilePath, err)
				}
				// Reduce confidence for semantic issues
				resolution.Confidence *= 0.8
			}
		}

		resolutions = append(resolutions, resolution)
	}

	return resolutions, nil
}

// parseTextResolutions extracts resolutions from text format
func (r *ConflictResolver) parseTextResolutions(content string, files []payload.ConflictFilePayload) ([]gitutils.ConflictResolution, error) {
	var resolutions []gitutils.ConflictResolution

	// Use regex patterns to extract resolution information
	lines := strings.Split(content, "\n")

	var currentResolution *gitutils.ConflictResolution
	var collectingLines bool

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Look for file path indicators
		if fileMatch := regexp.MustCompile(`(?i)file:?\s*(.+)`).FindStringSubmatch(line); fileMatch != nil {
			if currentResolution != nil {
				resolutions = append(resolutions, *currentResolution)
			}
			currentResolution = &gitutils.ConflictResolution{
				FilePath:   strings.TrimSpace(fileMatch[1]),
				Confidence: 0.5, // Default confidence
			}
			collectingLines = false
		}

		// Look for line range indicators
		if currentResolution != nil && !collectingLines {
			if rangeMatch := regexp.MustCompile(`(?i)lines?\s*(\d+)-(\d+)`).FindStringSubmatch(line); rangeMatch != nil {
				if start, err := strconv.Atoi(rangeMatch[1]); err == nil {
					currentResolution.StartLine = start
				}
				if end, err := strconv.Atoi(rangeMatch[2]); err == nil {
					currentResolution.EndLine = end
				}
			}
		}

		// Look for confidence indicators
		if currentResolution != nil {
			if confMatch := regexp.MustCompile(`(?i)confidence:?\s*([0-9.]+)`).FindStringSubmatch(line); confMatch != nil {
				if conf, err := strconv.ParseFloat(confMatch[1], 64); err == nil {
					currentResolution.Confidence = conf
				}
			}
		}

		// Look for reasoning
		if currentResolution != nil && r.includeReasoning {
			if reasonMatch := regexp.MustCompile(`(?i)reasoning:?\s*(.+)`).FindStringSubmatch(line); reasonMatch != nil {
				currentResolution.Reasoning = strings.TrimSpace(reasonMatch[1])
			}
		}

		// Look for resolution content
		if currentResolution != nil && !collectingLines {
			if strings.Contains(strings.ToLower(line), "resolution") || strings.Contains(strings.ToLower(line), "resolved") {
				collectingLines = true
				continue
			}
		}

		// Collect resolution lines
		if currentResolution != nil && collectingLines && line != "" {
			// Skip obvious non-content lines
			if !strings.Contains(line, "---") && !strings.Contains(line, "===") &&
				!strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "//") {
				currentResolution.ResolvedLines = append(currentResolution.ResolvedLines, line)
			}
		}
	}

	// Add the last resolution
	if currentResolution != nil {
		resolutions = append(resolutions, *currentResolution)
	}

	// Validate and clean up resolutions
	var validResolutions []gitutils.ConflictResolution
	for _, resolution := range resolutions {
		if err := gitutils.ValidateResolution(resolution); err == nil && len(resolution.ResolvedLines) > 0 {
			validResolutions = append(validResolutions, resolution)
		}
	}

	if len(validResolutions) == 0 {
		return nil, fmt.Errorf("no valid resolutions found in response")
	}

	return validResolutions, nil
}

// ResolveConflictsByFiles resolves conflicts for specific files
func (r *ConflictResolver) ResolveConflictsByFiles(ctx context.Context, filePaths []string) (*ResolverResult, error) {
	// Build conflict payload from file paths
	conflictPayload, err := r.buildPayloadFromFiles(filePaths)
	if err != nil {
		return nil, fmt.Errorf("failed to build payload from files: %w", err)
	}

	return r.ResolveConflicts(ctx, conflictPayload)
}

// buildPayloadFromFiles builds a conflict payload from file paths
func (r *ConflictResolver) buildPayloadFromFiles(filePaths []string) (*payload.ConflictPayload, error) {
	var files []payload.ConflictFilePayload

	for _, filePath := range filePaths {
		// Parse conflicts from the file
		hunks, err := gitutils.ParseConflictHunks(filePath, r.repoPath)
		if err != nil {
			return nil, fmt.Errorf("failed to parse conflicts in %s: %w", filePath, err)
		}

		// Convert hunks to conflict payloads
		var conflicts []payload.ConflictHunkPayload
		for i, hunk := range hunks {
			conflicts = append(conflicts, payload.ConflictHunkPayload{
				ID:          fmt.Sprintf("%s:%d", filePath, i),
				StartLine:   hunk.StartLine,
				EndLine:     hunk.EndLine,
				OursLines:   hunk.OursLines,
				TheirsLines: hunk.TheirsLines,
				BaseLines:   hunk.BaseLines,
			})
		}

		// Get file context
		contextLines, err := gitutils.ExtractFileContext(filePath, r.repoPath, 10)
		if err != nil {
			contextLines = nil // Continue without context
		}

		// Build file context
		fileContext := payload.FileContext{}
		if len(contextLines) > 10 {
			fileContext.BeforeLines = contextLines[:5]
			fileContext.AfterLines = contextLines[len(contextLines)-5:]
		} else {
			fileContext.BeforeLines = contextLines
		}

		files = append(files, payload.ConflictFilePayload{
			Path:      filePath,
			Conflicts: conflicts,
			Context:   fileContext,
		})
	}

	return &payload.ConflictPayload{
		Metadata: payload.PayloadMetadata{
			RepoPath: r.repoPath,
		},
		Files: files,
	}, nil
}

// extractGoContext extracts Go-specific context for better conflict resolution
func (r *ConflictResolver) extractGoContext(file payload.ConflictFilePayload, repoPath string) string {
	if file.Language != "go" {
		return ""
	}

	var context strings.Builder

	// Extract package information
	if packageInfo := r.extractPackageInfo(file, repoPath); packageInfo != "" {
		context.WriteString("Package: " + packageInfo + "\n")
	}

	// Extract import statements
	if imports := r.extractImportStatements(file, repoPath); len(imports) > 0 {
		context.WriteString("Imports: " + strings.Join(imports, ", ") + "\n")
	}

	// Extract function signatures in conflict regions
	if functions := r.extractFunctionSignatures(file); len(functions) > 0 {
		context.WriteString("Function signatures involved: " + strings.Join(functions, "; ") + "\n")
	}

	// Extract struct/interface definitions
	if types := r.extractTypeDefinitions(file); len(types) > 0 {
		context.WriteString("Type definitions: " + strings.Join(types, "; ") + "\n")
	}

	return context.String()
}

// extractPackageInfo extracts package declaration from the file
func (r *ConflictResolver) extractPackageInfo(file payload.ConflictFilePayload, repoPath string) string {
	// Read first few lines to find package declaration
	filePath := filepath.Join(repoPath, file.Path)
	content, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines[:min(10, len(lines))] {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "package ") {
			return strings.TrimPrefix(line, "package ")
		}
	}
	return ""
}

// extractImportStatements extracts import statements relevant to conflicts
func (r *ConflictResolver) extractImportStatements(file payload.ConflictFilePayload, repoPath string) []string {
	filePath := filepath.Join(repoPath, file.Path)
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}

	var imports []string
	lines := strings.Split(string(content), "\n")
	inImportBlock := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Single import
		if strings.HasPrefix(line, "import \"") {
			imports = append(imports, line)
		}

		// Import block start
		if strings.HasPrefix(line, "import (") {
			inImportBlock = true
			continue
		}

		// Import block end
		if inImportBlock && strings.HasPrefix(line, ")") {
			inImportBlock = false
			continue
		}

		// Import block content
		if inImportBlock && line != "" && !strings.HasPrefix(line, "//") {
			imports = append(imports, "\t"+line)
		}
	}

	return imports
}

// extractFunctionSignatures extracts function signatures from conflict regions
func (r *ConflictResolver) extractFunctionSignatures(file payload.ConflictFilePayload) []string {
	var signatures []string
	funcRegex := regexp.MustCompile(`^\s*func\s+(\w*\s*)?\w+\s*\([^)]*\).*$`)

	for _, conflict := range file.Conflicts {
		// Check both sides of the conflict
		allLines := append(conflict.OursLines, conflict.TheirsLines...)
		for _, line := range allLines {
			if funcRegex.MatchString(line) {
				signatures = append(signatures, strings.TrimSpace(line))
			}
		}
	}

	return signatures
}

// extractTypeDefinitions extracts struct/interface definitions from conflicts
func (r *ConflictResolver) extractTypeDefinitions(file payload.ConflictFilePayload) []string {
	var types []string
	typeRegex := regexp.MustCompile(`^\s*type\s+\w+\s+(struct|interface).*$`)

	for _, conflict := range file.Conflicts {
		allLines := append(conflict.OursLines, conflict.TheirsLines...)
		for _, line := range allLines {
			if typeRegex.MatchString(line) {
				types = append(types, strings.TrimSpace(line))
			}
		}
	}

	return types
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// IsAvailable checks if Claude CLI is available
func (r *ConflictResolver) IsAvailable() bool {
	return r.client != nil && r.client.IsAvailable()
}

// applyMultiTurnRefinement uses multi-turn conversations to improve low-confidence resolutions
func (r *ConflictResolver) applyMultiTurnRefinement(ctx context.Context, resolutions []gitutils.ConflictResolution, files []payload.ConflictFilePayload, repoPath string) ([]gitutils.ConflictResolution, error) {
	var refinedResolutions []gitutils.ConflictResolution

	for _, resolution := range resolutions {
		if resolution.Confidence >= r.multiTurnThreshold {
			// High confidence, no refinement needed
			refinedResolutions = append(refinedResolutions, resolution)
			continue
		}

		if r.verbose {
			fmt.Printf("Applying multi-turn refinement for low-confidence resolution (%.2f) in %s\n", resolution.Confidence, resolution.FilePath)
		}

		// Apply multi-turn conversation to improve the resolution
		refinedResolution, err := r.refineResolutionWithMultiTurn(ctx, resolution, files, repoPath)
		if err != nil {
			if r.verbose {
				fmt.Printf("Multi-turn refinement failed for %s: %v\n", resolution.FilePath, err)
			}
			// Fall back to original resolution
			refinedResolutions = append(refinedResolutions, resolution)
		} else {
			refinedResolutions = append(refinedResolutions, refinedResolution)
		}
	}

	return refinedResolutions, nil
}

// refineResolutionWithMultiTurn conducts a multi-turn conversation to improve a resolution
func (r *ConflictResolver) refineResolutionWithMultiTurn(ctx context.Context, resolution gitutils.ConflictResolution, files []payload.ConflictFilePayload, repoPath string) (gitutils.ConflictResolution, error) {
	// Find the file containing this resolution
	var targetFile *payload.ConflictFilePayload
	for i, file := range files {
		if file.Path == resolution.FilePath {
			targetFile = &files[i]
			break
		}
	}

	if targetFile == nil {
		return resolution, fmt.Errorf("could not find file %s for multi-turn refinement", resolution.FilePath)
	}

	// Start a session for multi-turn conversation
	sessionID, err := r.client.StartSession(ctx)
	if err != nil {
		return resolution, fmt.Errorf("failed to start session: %w", err)
	}
	defer r.client.EndSession(ctx)

	currentResolution := resolution

	for turn := 1; turn <= r.maxTurns; turn++ {
		if r.verbose {
			fmt.Printf("Multi-turn refinement: Turn %d/%d for %s\n", turn, r.maxTurns, resolution.FilePath)
		}

		// Build refinement prompt based on current resolution and context
		refinementPrompt := r.buildRefinementPrompt(currentResolution, *targetFile, turn)

		// Execute refinement
		command := &ClaudeCommand{
			Prompt:    refinementPrompt,
			SessionID: sessionID,
			Options: map[string]string{
				"task-type": "conflict-refinement",
				"turn":      fmt.Sprintf("%d", turn),
			},
		}

		response, err := r.client.ExecuteCommand(ctx, command)
		if err != nil {
			return currentResolution, fmt.Errorf("refinement turn %d failed: %w", turn, err)
		}

		if !response.Success {
			return currentResolution, fmt.Errorf("refinement turn %d unsuccessful: %s", turn, response.ErrorMessage)
		}

		// Parse the refined resolution
		refinedResolutions, err := r.parseResolutionsFromResponse(response, []payload.ConflictFilePayload{*targetFile})
		if err != nil || len(refinedResolutions) == 0 {
			if r.verbose {
				fmt.Printf("Failed to parse refinement in turn %d, keeping current resolution\n", turn)
			}
			break
		}

		refinedResolution := refinedResolutions[0]

		// Check if confidence improved significantly
		if refinedResolution.Confidence > currentResolution.Confidence {
			if r.verbose {
				fmt.Printf("Confidence improved from %.2f to %.2f in turn %d\n", currentResolution.Confidence, refinedResolution.Confidence, turn)
			}
			currentResolution = refinedResolution

			// Stop if we've reached good confidence
			if currentResolution.Confidence >= r.minConfidence {
				break
			}
		} else {
			// No improvement, stop refinement
			if r.verbose {
				fmt.Printf("No confidence improvement in turn %d, stopping refinement\n", turn)
			}
			break
		}
	}

	return currentResolution, nil
}

// adjustConfidenceWithGoValidation applies Go-specific validation to adjust confidence scores
func (r *ConflictResolver) adjustConfidenceWithGoValidation(resolutions []gitutils.ConflictResolution, files []payload.ConflictFilePayload) []gitutils.ConflictResolution {
	var adjustedResolutions []gitutils.ConflictResolution

	for _, resolution := range resolutions {
		// Find the corresponding file
		var targetFile *payload.ConflictFilePayload
		for i, file := range files {
			if file.Path == resolution.FilePath && file.Language == "go" {
				targetFile = &files[i]
				break
			}
		}

		// Skip non-Go files or if file not found
		if targetFile == nil {
			adjustedResolutions = append(adjustedResolutions, resolution)
			continue
		}

		// Apply Go-specific validation
		adjustedConfidence := r.validateGoResolution(resolution, *targetFile)

		// Create adjusted resolution
		adjustedResolution := resolution
		adjustedResolution.Confidence = adjustedConfidence

		if r.verbose && adjustedConfidence != resolution.Confidence {
			fmt.Printf("Adjusted confidence for %s from %.2f to %.2f based on Go validation\n",
				resolution.FilePath, resolution.Confidence, adjustedConfidence)
		}

		adjustedResolutions = append(adjustedResolutions, adjustedResolution)
	}

	return adjustedResolutions
}

// validateGoResolution performs Go-specific validation and returns adjusted confidence
func (r *ConflictResolver) validateGoResolution(resolution gitutils.ConflictResolution, file payload.ConflictFilePayload) float64 {
	originalConfidence := resolution.Confidence
	confidenceMultiplier := 1.0

	// Check syntax validity
	if !r.isValidGoSyntax(resolution.ResolvedLines) {
		confidenceMultiplier *= 0.3 // Heavily penalize invalid syntax
		if r.verbose {
			fmt.Printf("Invalid Go syntax detected in resolution for %s\n", resolution.FilePath)
		}
	}

	// Check function signature consistency
	if !r.validateFunctionSignatures(resolution.ResolvedLines, file) {
		confidenceMultiplier *= 0.7 // Moderately penalize inconsistent signatures
		if r.verbose {
			fmt.Printf("Function signature inconsistency detected in %s\n", resolution.FilePath)
		}
	}

	// Check import statement validity
	if !r.validateImportStatements(resolution.ResolvedLines) {
		confidenceMultiplier *= 0.8 // Slightly penalize import issues
		if r.verbose {
			fmt.Printf("Import statement issues detected in %s\n", resolution.FilePath)
		}
	}

	// Check for common Go idioms and patterns
	if r.followsGoIdioms(resolution.ResolvedLines) {
		confidenceMultiplier *= 1.1 // Bonus for following Go idioms
	}

	// Ensure confidence doesn't exceed 1.0
	adjustedConfidence := originalConfidence * confidenceMultiplier
	if adjustedConfidence > 1.0 {
		adjustedConfidence = 1.0
	}
	if adjustedConfidence < 0.0 {
		adjustedConfidence = 0.0
	}

	return adjustedConfidence
}

// isValidGoSyntax performs basic syntax validation on Go code lines
func (r *ConflictResolver) isValidGoSyntax(lines []string) bool {
	codeContent := strings.Join(lines, "\n")

	// Check for basic syntax issues
	braceCount := 0
	parenCount := 0
	bracketCount := 0

	for _, char := range codeContent {
		switch char {
		case '{':
			braceCount++
		case '}':
			braceCount--
		case '(':
			parenCount++
		case ')':
			parenCount--
		case '[':
			bracketCount++
		case ']':
			bracketCount--
		}
	}

	// Check for balanced braces, parentheses, and brackets
	if braceCount != 0 || parenCount != 0 || bracketCount != 0 {
		return false
	}

	// Check for obvious syntax errors
	invalidPatterns := []string{
		"func (",   // Incomplete function declaration
		"if {",     // Empty if condition
		"for {",    // Empty for condition (actually valid in Go, but suspicious)
		";;",       // Double semicolon
		"import (", // Incomplete import block
	}

	for _, pattern := range invalidPatterns {
		if strings.Contains(codeContent, pattern) {
			// Additional validation needed for some patterns
			if pattern == "for {" && strings.Contains(codeContent, "for {") {
				// "for {}" is valid Go (infinite loop), but check context
				continue
			}
			return false
		}
	}

	return true
}

// validateFunctionSignatures checks if function signatures are consistent
func (r *ConflictResolver) validateFunctionSignatures(resolvedLines []string, file payload.ConflictFilePayload) bool {
	funcRegex := regexp.MustCompile(`^\s*func\s+(\w*\s*)?\w+\s*\([^)]*\).*$`)

	// Extract function signatures from resolved content
	resolvedFunctions := make([]string, 0)
	for _, line := range resolvedLines {
		if funcRegex.MatchString(line) {
			resolvedFunctions = append(resolvedFunctions, strings.TrimSpace(line))
		}
	}

	// If no functions in resolution, consider it valid
	if len(resolvedFunctions) == 0 {
		return true
	}

	// Check for duplicate function names (which could indicate merge issues)
	functionNames := make(map[string]int)
	for _, funcSig := range resolvedFunctions {
		// Extract function name
		nameParts := strings.Fields(funcSig)
		if len(nameParts) >= 2 {
			funcName := ""
			for i, part := range nameParts {
				if part == "func" && i+1 < len(nameParts) {
					// Handle method receivers: func (r *Type) method(...)
					if strings.HasPrefix(nameParts[i+1], "(") {
						if i+2 < len(nameParts) {
							funcName = nameParts[i+2]
						}
					} else {
						funcName = nameParts[i+1]
					}
					break
				}
			}

			if funcName != "" {
				// Remove parameter list from name
				if idx := strings.Index(funcName, "("); idx != -1 {
					funcName = funcName[:idx]
				}
				functionNames[funcName]++
			}
		}
	}

	// Check for duplicates (possible merge conflicts)
	for name, count := range functionNames {
		if count > 1 {
			if r.verbose {
				fmt.Printf("Duplicate function name detected: %s (appears %d times)\n", name, count)
			}
			return false
		}
	}

	return true
}

// validateImportStatements checks if import statements are valid
func (r *ConflictResolver) validateImportStatements(lines []string) bool {
	importRegex := regexp.MustCompile(`^\s*import\s+`)
	quotedImportRegex := regexp.MustCompile(`^\s*"[^"]+"\s*$`)
	aliasImportRegex := regexp.MustCompile(`^\s*\w+\s+"[^"]+"\s*$`)

	for _, line := range lines {
		if importRegex.MatchString(line) {
			// Single import statement
			if strings.Contains(line, "import \"") {
				// Extract the import path
				parts := strings.SplitN(line, "import \"", 2)
				if len(parts) == 2 && !strings.HasSuffix(parts[1], "\"") {
					return false // Unclosed import string
				}
			}
		} else if strings.TrimSpace(line) != "" &&
			(quotedImportRegex.MatchString(strings.TrimSpace(line)) ||
				aliasImportRegex.MatchString(strings.TrimSpace(line))) {
			// Import line within import block - basic validation
			continue
		}
	}

	return true
}

// followsGoIdioms checks if the code follows common Go idioms
func (r *ConflictResolver) followsGoIdioms(lines []string) bool {
	codeContent := strings.Join(lines, "\n")

	// Check for proper error handling patterns
	hasProperErrorHandling := strings.Contains(codeContent, "if err != nil") ||
		strings.Contains(codeContent, "return err") ||
		strings.Contains(codeContent, "return nil, err")

	// Check for proper naming conventions
	hasGoodNaming := true
	variableRegex := regexp.MustCompile(`\b[a-z][a-zA-Z0-9]*\b`)
	constantRegex := regexp.MustCompile(`\b[A-Z][A-Z0-9_]*\b`)

	// Bonus points for following Go idioms
	idiomsScore := 0
	if hasProperErrorHandling {
		idiomsScore++
	}
	if hasGoodNaming {
		idiomsScore++
	}
	if strings.Contains(codeContent, "defer ") {
		idiomsScore++ // Proper resource cleanup
	}
	if variableRegex.MatchString(codeContent) || constantRegex.MatchString(codeContent) {
		idiomsScore++ // Proper naming conventions
	}

	return idiomsScore >= 2 // At least 2 positive idiom indicators
}

// buildRefinementPrompt builds a prompt for refining a low-confidence resolution
func (r *ConflictResolver) buildRefinementPrompt(resolution gitutils.ConflictResolution, file payload.ConflictFilePayload, turn int) string {
	var prompt strings.Builder

	prompt.WriteString(fmt.Sprintf("**MULTI-TURN CONFLICT RESOLUTION REFINEMENT - Turn %d**\n\n", turn))

	if turn == 1 {
		prompt.WriteString("I provided a resolution for a Go merge conflict, but the confidence score was low (")
		prompt.WriteString(fmt.Sprintf("%.2f", resolution.Confidence))
		prompt.WriteString("). Please help me improve this resolution by:\n\n")
		prompt.WriteString("1. **Analyzing potential issues** with the current resolution\n")
		prompt.WriteString("2. **Examining Go semantics** more carefully (types, interfaces, imports)\n")
		prompt.WriteString("3. **Considering alternative approaches** that might be more robust\n")
		prompt.WriteString("4. **Providing an improved resolution** with higher confidence\n\n")
	} else {
		prompt.WriteString("Continuing refinement of the Go conflict resolution. Please further analyze and improve based on:\n\n")
		prompt.WriteString("1. **Go language best practices** and idioms\n")
		prompt.WriteString("2. **Type safety** and interface compatibility\n")
		prompt.WriteString("3. **Semantic correctness** and maintainability\n\n")
	}

	prompt.WriteString("**CURRENT RESOLUTION:**\n")
	prompt.WriteString(fmt.Sprintf("File: %s (lines %d-%d)\n", resolution.FilePath, resolution.StartLine, resolution.EndLine))
	prompt.WriteString(fmt.Sprintf("Confidence: %.2f\n", resolution.Confidence))
	if resolution.Reasoning != "" {
		prompt.WriteString(fmt.Sprintf("Previous reasoning: %s\n", resolution.Reasoning))
	}
	prompt.WriteString("Resolved content:\n")
	for _, line := range resolution.ResolvedLines {
		prompt.WriteString("  " + line + "\n")
	}

	prompt.WriteString("\n**ORIGINAL CONFLICT:**\n")
	// Find the specific conflict hunk
	for _, conflict := range file.Conflicts {
		if conflict.StartLine <= resolution.StartLine && conflict.EndLine >= resolution.EndLine {
			prompt.WriteString("<<<<<<< HEAD\n")
			for _, line := range conflict.OursLines {
				prompt.WriteString(line + "\n")
			}
			if len(conflict.BaseLines) > 0 {
				prompt.WriteString("||||||| base\n")
				for _, line := range conflict.BaseLines {
					prompt.WriteString(line + "\n")
				}
			}
			prompt.WriteString("=======\n")
			for _, line := range conflict.TheirsLines {
				prompt.WriteString(line + "\n")
			}
			prompt.WriteString(">>>>>>> branch\n")
			break
		}
	}

	// Add Go-specific context
	goContext := r.extractGoContext(file, r.repoPath)
	if goContext != "" {
		prompt.WriteString("\n**GO CONTEXT:**\n")
		prompt.WriteString(goContext)
	}

	prompt.WriteString("\n**REFINEMENT REQUEST:**\n")
	prompt.WriteString("Please provide an improved resolution with:\n")
	prompt.WriteString("- Higher confidence score (ideally > 0.7)\n")
	prompt.WriteString("- Detailed explanation of improvements made\n")
	prompt.WriteString("- Go-specific validation of the solution\n")
	prompt.WriteString("- Consideration of edge cases and compatibility\n\n")

	prompt.WriteString("Respond in the same JSON format as before.")

	return prompt.String()
}

// isGoFile checks if a file path represents a Go source file
func (r *ConflictResolver) isGoFile(filePath string) bool {
	return strings.HasSuffix(strings.ToLower(filePath), ".go")
}

// validateSemanticCorrectness performs comprehensive semantic validation on Go code
func (r *ConflictResolver) validateSemanticCorrectness(resolution gitutils.ConflictResolution) error {
	codeContent := strings.Join(resolution.ResolvedLines, "\n")

	// Check for common semantic issues
	var issues []string

	// 1. Check for incomplete function definitions
	if r.hasIncompleteFunctions(codeContent) {
		issues = append(issues, "incomplete function definitions detected")
	}

	// 2. Check for unbalanced control structures
	if r.hasUnbalancedControlStructures(codeContent) {
		issues = append(issues, "unbalanced control structures detected")
	}

	// 3. Check for invalid variable declarations
	if r.hasInvalidVariableDeclarations(codeContent) {
		issues = append(issues, "invalid variable declarations detected")
	}

	// 4. Check for improper error handling
	if r.hasImproperErrorHandling(codeContent) {
		issues = append(issues, "improper error handling patterns detected")
	}

	// 5. Check for type consistency issues
	if r.hasTypeConsistencyIssues(codeContent) {
		issues = append(issues, "type consistency issues detected")
	}

	if len(issues) > 0 {
		return fmt.Errorf("semantic validation failed: %s", strings.Join(issues, ", "))
	}

	return nil
}

// hasIncompleteFunctions checks for incomplete function definitions
func (r *ConflictResolver) hasIncompleteFunctions(code string) bool {
	// Look for function declarations without bodies
	funcWithoutBodyRegex := regexp.MustCompile(`func\s+\w+\([^)]*\)[^{]*$`)
	lines := strings.Split(code, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if funcWithoutBodyRegex.MatchString(line) && !strings.Contains(line, "{") {
			return true
		}
	}

	return false
}

// hasUnbalancedControlStructures checks for unbalanced if/for/switch statements
func (r *ConflictResolver) hasUnbalancedControlStructures(code string) bool {
	lines := strings.Split(code, "\n")
	braceDepth := 0
	controlStructures := []string{"if", "for", "switch", "select"}

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Check for control structure keywords
		for _, keyword := range controlStructures {
			if strings.HasPrefix(line, keyword+" ") || strings.HasPrefix(line, keyword+"(") {
				if !strings.Contains(line, "{") {
					// Control structure without opening brace
					return true
				}
			}
		}

		// Track brace depth
		for _, char := range line {
			if char == '{' {
				braceDepth++
			} else if char == '}' {
				braceDepth--
				if braceDepth < 0 {
					return true // More closing braces than opening
				}
			}
		}
	}

	return braceDepth != 0 // Should end with balanced braces
}

// hasInvalidVariableDeclarations checks for invalid variable declarations
func (r *ConflictResolver) hasInvalidVariableDeclarations(code string) bool {
	// Check for common variable declaration issues
	invalidPatterns := []string{
		"var = ",   // Missing variable name
		":= var",   // Invalid short declaration
		"var {",    // Invalid syntax
		"const = ", // Missing constant name
	}

	for _, pattern := range invalidPatterns {
		if strings.Contains(code, pattern) {
			return true
		}
	}

	return false
}

// hasImproperErrorHandling checks for improper error handling patterns
func (r *ConflictResolver) hasImproperErrorHandling(code string) bool {
	// Look for functions that return error but don't handle it properly
	lines := strings.Split(code, "\n")

	for i, line := range lines {
		line = strings.TrimSpace(line)

		// Check for function calls that likely return errors
		if strings.Contains(line, "(") && strings.Contains(line, ")") {
			// Look for assignment to err variable
			if strings.Contains(line, "err") {
				// Check if the next few lines handle the error
				errorHandled := false
				for j := i + 1; j < len(lines) && j < i+3; j++ {
					nextLine := strings.TrimSpace(lines[j])
					if strings.Contains(nextLine, "if err != nil") ||
						strings.Contains(nextLine, "return err") ||
						strings.Contains(nextLine, "log.Fatal") ||
						strings.Contains(nextLine, "panic") {
						errorHandled = true
						break
					}
				}

				if !errorHandled && strings.Contains(line, ", err") {
					// Error is assigned but not handled
					return true
				}
			}
		}
	}

	return false
}

// hasTypeConsistencyIssues checks for basic type consistency issues
func (r *ConflictResolver) hasTypeConsistencyIssues(code string) bool {
	// Check for obvious type mismatches
	typeMismatchPatterns := []string{
		"string = int",  // Direct type mismatch
		"int = string",  // Direct type mismatch
		"bool = string", // Direct type mismatch
		"string = bool", // Direct type mismatch
	}

	for _, pattern := range typeMismatchPatterns {
		if strings.Contains(code, pattern) {
			return true
		}
	}

	return false
}

// Close cleans up the resolver
func (r *ConflictResolver) Close() error {
	return r.client.Close()
}
