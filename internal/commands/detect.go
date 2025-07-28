package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/NeuBlink/syncwright/internal/gitutils"
)

const (
	// OutputFormatJSON represents JSON output format
	OutputFormatJSON = "json"
	// OutputFormatText represents text output format
	OutputFormatText = "text"
)

// SimplifiedConflictPayload represents a simplified payload for AI processing
type SimplifiedConflictPayload struct {
	Files []SimplifiedFilePayload `json:"files"`
}

// SimplifiedFilePayload represents a single file's conflict data
type SimplifiedFilePayload struct {
	Path      string                     `json:"path"`
	Language  string                     `json:"language"`
	Conflicts []SimplifiedConflictHunk   `json:"conflicts"`
}

// SimplifiedConflictHunk represents a conflict with minimal context
type SimplifiedConflictHunk struct {
	ID          string   `json:"id"`
	StartLine   int      `json:"start_line"`
	EndLine     int      `json:"end_line"`
	OursLines   []string `json:"ours_lines"`
	TheirsLines []string `json:"theirs_lines"`
	PreContext  []string `json:"pre_context"`
	PostContext []string `json:"post_context"`
}

// detectLanguage determines the programming language from file extension
func detectLanguage(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	
	languageMap := map[string]string{
		".go":    "go",
		".js":    "javascript",
		".jsx":   "javascript",
		".ts":    "typescript",
		".tsx":   "typescript",
		".py":    "python",
		".java":  "java",
		".c":     "c",
		".h":     "c",
		".cpp":   "cpp",
		".hpp":   "cpp",
		".cs":    "csharp",
		".rb":    "ruby",
		".php":   "php",
		".rs":    "rust",
		".swift": "swift",
		".json":  "json",
		".xml":   "xml",
		".yaml":  "yaml",
		".yml":   "yaml",
		".md":    "markdown",
		".txt":   "text",
	}

	if lang, exists := languageMap[ext]; exists {
		return lang
	}
	return "text"
}

// buildSimplifiedPayload creates a simplified conflict payload from conflict report
func (d *DetectCommand) buildSimplifiedPayload(report *gitutils.ConflictReport) (*SimplifiedConflictPayload, error) {
	payload := &SimplifiedConflictPayload{}

	for _, conflictFile := range report.ConflictedFiles {
		// Skip binary or problematic files
		if d.shouldSkipFile(conflictFile.Path) {
			continue
		}

		language := detectLanguage(conflictFile.Path)
		filePayload := SimplifiedFilePayload{
			Path:     conflictFile.Path,
			Language: language,
		}

		// Convert conflict hunks to simplified format
		for i, hunk := range conflictFile.Hunks {
			conflictHunk := SimplifiedConflictHunk{
				ID:          fmt.Sprintf("%s:%d", conflictFile.Path, i),
				StartLine:   hunk.StartLine,
				EndLine:     hunk.EndLine,
				OursLines:   hunk.OursLines,
				TheirsLines: hunk.TheirsLines,
				PreContext:  d.extractPreContext(conflictFile.Context, hunk.StartLine),
				PostContext: d.extractPostContext(conflictFile.Context, hunk.EndLine),
			}
			filePayload.Conflicts = append(filePayload.Conflicts, conflictHunk)
		}

		payload.Files = append(payload.Files, filePayload)
	}

	return payload, nil
}

// shouldSkipFile determines if a file should be excluded from processing
func (d *DetectCommand) shouldSkipFile(filePath string) bool {
	// Skip binary files and common exclusions
	excludePatterns := []string{
		".git/", ".gitignore", "package-lock.json", "yarn.lock",
		".png", ".jpg", ".jpeg", ".gif", ".bmp", ".ico", ".svg",
		".exe", ".dll", ".so", ".dylib", ".o", ".obj",
		".zip", ".tar", ".gz", ".rar", ".7z",
	}

	lowerPath := strings.ToLower(filePath)
	for _, pattern := range excludePatterns {
		if strings.Contains(lowerPath, pattern) {
			return true
		}
	}
	return false
}

// extractPreContext extracts context lines before the conflict
func (d *DetectCommand) extractPreContext(fileContent []string, startLine int) []string {
	maxLines := d.options.MaxContextLines
	start := startLine - maxLines - 1
	if start < 0 {
		start = 0
	}
	end := startLine - 1
	if end <= start || end > len(fileContent) {
		return []string{}
	}
	return fileContent[start:end]
}

// extractPostContext extracts context lines after the conflict
func (d *DetectCommand) extractPostContext(fileContent []string, endLine int) []string {
	maxLines := d.options.MaxContextLines
	start := endLine
	if start >= len(fileContent) {
		return []string{}
	}
	end := start + maxLines
	if end > len(fileContent) {
		end = len(fileContent)
	}
	return fileContent[start:end]
}

// DetectOptions contains options for the detect command
type DetectOptions struct {
	RepoPath        string
	OutputFormat    string // "json", "text"
	OutputFile      string
	MaxContextLines int
	Verbose         bool
	// Memory optimization options
	MaxMemoryMB     int64
	EnableStreaming bool
	BatchSize       int
	WorkerPoolSize  int
}

// DetectResult represents the result of conflict detection
type DetectResult struct {
	Success         bool                        `json:"success"`
	ConflictReport  *gitutils.ConflictReport    `json:"conflict_report,omitempty"`
	ConflictPayload *SimplifiedConflictPayload  `json:"conflict_payload,omitempty"`
	ErrorMessage    string                      `json:"error_message,omitempty"`
	Summary         DetectSummary               `json:"summary"`
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
	options        DetectOptions
	memoryMonitor  *MemoryMonitor
	memoryConfig   *MemoryConfig
	fileProcessor  *FileProcessor
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
	if options.RepoPath == "" {
		if wd, err := os.Getwd(); err == nil {
			options.RepoPath = wd
		}
	}
	
	// Set memory optimization defaults
	if options.MaxMemoryMB == 0 {
		options.MaxMemoryMB = 512 // Default 512MB
	}
	if options.BatchSize == 0 {
		options.BatchSize = 50 // Default batch size
	}
	if options.WorkerPoolSize == 0 {
		options.WorkerPoolSize = 4 // Default worker pool size
	}

	// Initialize memory monitoring components
	memoryMonitor := NewMemoryMonitor(options.MaxMemoryMB)
	memoryConfig := &MemoryConfig{
		MaxMemoryMB:      options.MaxMemoryMB,
		BatchSize:        options.BatchSize,
		WorkerPoolSize:   options.WorkerPoolSize,
		EnableStreaming:  options.EnableStreaming,
		ForceGCInterval:  30 * time.Second,
		ProgressInterval: 5 * time.Second,
	}
	fileProcessor := NewFileProcessor(memoryMonitor, memoryConfig)

	return &DetectCommand{
		options:       options,
		memoryMonitor: memoryMonitor,
		memoryConfig:  memoryConfig,
		fileProcessor: fileProcessor,
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

	// Optimize memory configuration for large repositories with enhanced scaling
	if result.Summary.TotalFiles > 500 {
		d.memoryConfig.OptimizeForLargeRepo(result.Summary.TotalFiles)
		if d.options.Verbose {
			fmt.Printf("Large repository detected (%d files), optimizing memory usage\n", result.Summary.TotalFiles)
			fmt.Printf("Using batch size: %d, workers: %d, memory limit: %dMB\n",
				d.memoryConfig.BatchSize, d.memoryConfig.WorkerPoolSize, d.memoryConfig.MaxMemoryMB)
		}
		
		// Enable streaming by default for large repositories
		if !d.options.EnableStreaming && d.options.OutputFormat == OutputFormatJSON {
			d.options.EnableStreaming = true
			if d.options.Verbose {
				fmt.Println("Automatically enabling streaming for large repository")
			}
		}
		
		// Reduce context lines for memory efficiency in very large repos
		if result.Summary.TotalFiles > 1000 && d.options.MaxContextLines > 3 {
			originalContextLines := d.options.MaxContextLines
			d.options.MaxContextLines = 3
			if d.options.Verbose {
				fmt.Printf("Reducing context lines from %d to %d for memory efficiency\n", 
					originalContextLines, d.options.MaxContextLines)
			}
		}
	}

	// Use streaming processing for better memory efficiency
	if d.options.EnableStreaming && d.options.OutputFormat == OutputFormatJSON {
		if err := d.executeStreamingProcessing(conflictReport, result); err != nil {
			result.ErrorMessage = fmt.Sprintf("Streaming processing failed: %v", err)
			return result, err
		}
	} else {
		// Fall back to traditional processing for non-JSON output or when streaming is disabled
		conflictPayload, err := d.buildSimplifiedPayload(conflictReport)
		if err != nil {
			result.ErrorMessage = fmt.Sprintf("Failed to build conflict payload: %v", err)
			return result, err
		}

		result.ConflictPayload = conflictPayload
		result.Summary.ProcessableFiles = len(conflictPayload.Files)
		result.Summary.ExcludedFiles = result.Summary.TotalFiles - result.Summary.ProcessableFiles
	}

	if d.options.Verbose {
		fmt.Printf("Processable files: %d, Excluded files: %d\n",
			result.Summary.ProcessableFiles, result.Summary.ExcludedFiles)
	}

	result.Success = true

	if d.options.Verbose {
		fmt.Printf("Conflict detection completed successfully\n")
	}

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
func (d *DetectCommand) addVerboseConflictDetails(output []string, conflicts []SimplifiedConflictHunk) []string {
	for i, conflict := range conflicts {
		output = append(output, fmt.Sprintf("      Conflict %d: lines %d-%d",
			i+1, conflict.StartLine, conflict.EndLine))
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

// executeStreamingProcessing handles memory-efficient streaming JSON processing
func (d *DetectCommand) executeStreamingProcessing(conflictReport *gitutils.ConflictReport, result *DetectResult) error {
	// Prepare output writer
	var writer io.Writer
	if d.options.OutputFile != "" {
		file, err := os.Create(d.options.OutputFile)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer file.Close()
		writer = file
	} else {
		writer = os.Stdout
	}

	// Initialize streaming encoder
	encoder := NewStreamingJSONEncoder(writer, d.memoryMonitor, d.memoryConfig)

	// Write JSON header
	if err := encoder.WriteHeader(result.Summary); err != nil {
		return fmt.Errorf("failed to write JSON header: %w", err)
	}

	// Process files using streaming approach
	processedCount := 0
	skippedCount := 0
	
	err := d.fileProcessor.ProcessFilesStreaming(
		conflictReport.ConflictedFiles,
		d,
		func(processResult ProcessResult) {
			if processResult.Error != nil {
				if d.options.Verbose {
					fmt.Printf("Error processing file: %v\n", processResult.Error)
				}
				return
			}
			
			if processResult.FilePayload == nil {
				// File was skipped
				skippedCount++
				return
			}
			
			// Write file to stream
			if err := encoder.WriteFile(*processResult.FilePayload); err != nil {
				if d.options.Verbose {
					fmt.Printf("Error writing file to stream: %v\n", err)
				}
				return
			}
			
			processedCount++
			
			// Progress reporting for large repositories
			if d.options.Verbose && (processedCount%100 == 0 || processedCount+skippedCount == len(conflictReport.ConflictedFiles)) {
				stats := d.memoryMonitor.GetMemoryStats()
				fmt.Printf("Progress: %d/%d files processed, %dMB memory used\n",
					processedCount+skippedCount, len(conflictReport.ConflictedFiles), stats.AllocMB)
			}
		},
	)

	if err != nil {
		return fmt.Errorf("file processing failed: %w", err)
	}

	// Update result summary
	result.Summary.ProcessableFiles = processedCount
	result.Summary.ExcludedFiles = skippedCount

	// Get final memory stats
	finalStats := d.memoryMonitor.GetMemoryStats()
	
	// Write JSON footer
	errorMessage := ""
	if result.ErrorMessage != "" {
		errorMessage = result.ErrorMessage
	}
	
	if err := encoder.WriteFooter(finalStats, errorMessage); err != nil {
		return fmt.Errorf("failed to write JSON footer: %w", err)
	}

	if d.options.Verbose {
		LogMemoryStats(finalStats, true)
		fmt.Printf("Streaming processing completed: %d files processed, %d skipped\n",
			processedCount, skippedCount)
	}

	return nil
}

// DetectConflicts is a convenience function for simple conflict detection
func DetectConflicts(repoPath string) (*DetectResult, error) {
	options := DetectOptions{
		RepoPath:        repoPath,
		OutputFormat:    OutputFormatJSON,
		Verbose:         false,
		EnableStreaming: true, // Enable streaming by default
	}

	cmd := NewDetectCommand(options)
	defer cmd.Close()
	return cmd.Execute()
}

// DetectConflictsVerbose is a convenience function for verbose conflict detection
func DetectConflictsVerbose(repoPath string, outputFile string) (*DetectResult, error) {
	options := DetectOptions{
		RepoPath:        repoPath,
		OutputFormat:    OutputFormatJSON,
		OutputFile:      outputFile,
		Verbose:         true,
		EnableStreaming: true,
		MaxMemoryMB:     256, // Lower memory limit for verbose mode
	}

	cmd := NewDetectCommand(options)
	defer cmd.Close()
	return cmd.Execute()
}

// DetectConflictsText is a convenience function for text format output
func DetectConflictsText(repoPath string) (*DetectResult, error) {
	options := DetectOptions{
		RepoPath:        repoPath,
		OutputFormat:    OutputFormatText,
		Verbose:         true,
		EnableStreaming: false, // Disable streaming for text output
	}

	cmd := NewDetectCommand(options)
	defer cmd.Close() // Ensure resources are cleaned up
	return cmd.Execute()
}

// DetectConflictsLargeRepo is optimized for repositories with >1000 conflicted files
func DetectConflictsLargeRepo(repoPath string, outputFile string) (*DetectResult, error) {
	options := DetectOptions{
		RepoPath:        repoPath,
		OutputFormat:    OutputFormatJSON,
		OutputFile:      outputFile,
		Verbose:         true,
		EnableStreaming: true,
		MaxMemoryMB:     128,  // Very conservative memory limit
		BatchSize:       10,   // Small batches
		WorkerPoolSize:  1,    // Single worker to minimize memory usage
	}

	cmd := NewDetectCommand(options)
	defer cmd.Close()
	return cmd.Execute()
}

// Close cleans up resources used by the DetectCommand
func (d *DetectCommand) Close() {
	if d.fileProcessor != nil {
		d.fileProcessor.Close()
	}
}

