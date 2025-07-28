package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/NeuBlink/syncwright/internal/gitutils"
	"github.com/NeuBlink/syncwright/internal/payload"
)

// BatchOptions contains options for the batch command
type BatchOptions struct {
	RepoPath      string
	OutputFile    string
	BatchSize     int
	Concurrency   int
	GroupBy       string // "language", "file", "size", "none"
	MaxTokens     int
	TimeoutSec    int
	MinConfidence float64
	AutoApply     bool
	DryRun        bool
	Verbose       bool
	Progress      bool
	Streaming     bool
	BackupFiles   bool
	MaxRetries    int
}

// BatchResult represents the result of batch processing
type BatchResult struct {
	Success            bool                    `json:"success"`
	TotalConflicts     int                     `json:"total_conflicts"`
	TotalBatches       int                     `json:"total_batches"`
	ProcessedBatches   int                     `json:"processed_batches"`
	SuccessfulBatches  int                     `json:"successful_batches"`
	FailedBatches      int                     `json:"failed_batches"`
	TotalResolutions   int                     `json:"total_resolutions"`
	AppliedResolutions int                     `json:"applied_resolutions"`
	SkippedResolutions int                     `json:"skipped_resolutions"`
	FailedResolutions  int                     `json:"failed_resolutions"`
	ProcessingTimeMs   int64                   `json:"processing_time_ms"`
	AverageConfidence  float64                 `json:"average_confidence"`
	BatchResults       []BatchItemResult       `json:"batch_results"`
	ErrorMessage       string                  `json:"error_message,omitempty"`
	Warnings           []string                `json:"warnings,omitempty"`
	Performance        BatchPerformanceMetrics `json:"performance"`
}

// BatchItemResult represents the result of processing a single batch
type BatchItemResult struct {
	BatchID           int                           `json:"batch_id"`
	ConflictsInBatch  int                           `json:"conflicts_in_batch"`
	ProcessingTimeMs  int64                         `json:"processing_time_ms"`
	Success           bool                          `json:"success"`
	Resolutions       []gitutils.ConflictResolution `json:"resolutions"`
	AppliedCount      int                           `json:"applied_count"`
	SkippedCount      int                           `json:"skipped_count"`
	FailedCount       int                           `json:"failed_count"`
	AverageConfidence float64                       `json:"average_confidence"`
	ErrorMessage      string                        `json:"error_message,omitempty"`
	Warnings          []string                      `json:"warnings,omitempty"`
}

// BatchPerformanceMetrics tracks performance statistics
type BatchPerformanceMetrics struct {
	DetectionTimeMs    int64   `json:"detection_time_ms"`
	GroupingTimeMs     int64   `json:"grouping_time_ms"`
	AIProcessingTimeMs int64   `json:"ai_processing_time_ms"`
	ApplicationTimeMs  int64   `json:"application_time_ms"`
	TotalTimeMs        int64   `json:"total_time_ms"`
	MemoryUsageMB      float64 `json:"memory_usage_mb"`
	ConcurrentBatches  int     `json:"concurrent_batches"`
	AverageLatencyMs   int64   `json:"average_latency_ms"`
	ThroughputPerSec   float64 `json:"throughput_per_sec"`
}

// ConflictBatch represents a group of conflicts to be processed together
type ConflictBatch struct {
	ID               int                           `json:"id"`
	Files            []payload.ConflictFilePayload `json:"files"`
	TotalConflicts   int                           `json:"total_conflicts"`
	EstimatedTokens  int                           `json:"estimated_tokens"`
	GroupingCriteria string                        `json:"grouping_criteria"`
	Priority         int                           `json:"priority"`
}

// BatchCommand implements the batch subcommand
type BatchCommand struct {
	options BatchOptions
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewBatchCommand creates a new batch command
func NewBatchCommand(options BatchOptions) *BatchCommand {
	// Set defaults
	if options.BatchSize == 0 {
		options.BatchSize = 10
	}
	if options.Concurrency == 0 {
		options.Concurrency = 3
	}
	if options.GroupBy == "" {
		options.GroupBy = "language"
	}
	if options.MaxTokens == 0 {
		options.MaxTokens = 50000
	}
	if options.TimeoutSec == 0 {
		options.TimeoutSec = 300
	}
	if options.MinConfidence == 0 {
		options.MinConfidence = 0.7
	}
	if options.MaxRetries == 0 {
		options.MaxRetries = 3
	}
	if options.RepoPath == "" {
		if wd, err := os.Getwd(); err == nil {
			options.RepoPath = wd
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(options.TimeoutSec)*time.Second)

	return &BatchCommand{
		options: options,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Execute runs the batch command
func (b *BatchCommand) Execute() (*BatchResult, error) {
	defer b.cancel()

	startTime := time.Now()
	result := &BatchResult{
		Performance: BatchPerformanceMetrics{
			ConcurrentBatches: b.options.Concurrency,
		},
	}

	if b.options.Verbose {
		fmt.Printf("🚀 Starting batch conflict resolution...\n")
		fmt.Printf("   Batch size: %d conflicts per batch\n", b.options.BatchSize)
		fmt.Printf("   Concurrency: %d parallel batches\n", b.options.Concurrency)
		fmt.Printf("   Grouping strategy: %s\n", b.options.GroupBy)
	}

	// Step 1: Detect conflicts
	if b.options.Verbose {
		fmt.Printf("🔍 Step 1: Detecting conflicts...\n")
	}
	detectStart := time.Now()

	conflictPayload, err := b.detectConflicts(result)
	if err != nil {
		return result, err
	}

	result.Performance.DetectionTimeMs = time.Since(detectStart).Milliseconds()
	result.TotalConflicts = b.countTotalConflicts(conflictPayload)

	if result.TotalConflicts == 0 {
		result.Success = true
		if b.options.Verbose {
			fmt.Printf("✅ No conflicts detected\n")
		}
		return result, nil
	}

	if b.options.Verbose {
		fmt.Printf("📋 Found %d conflicts across %d files\n", result.TotalConflicts, len(conflictPayload.Files))
	}

	// Step 2: Group conflicts into batches
	if b.options.Verbose {
		fmt.Printf("📦 Step 2: Grouping conflicts into batches...\n")
	}
	groupingStart := time.Now()

	batches, err := b.createBatches(conflictPayload, result)
	if err != nil {
		return result, err
	}

	result.Performance.GroupingTimeMs = time.Since(groupingStart).Milliseconds()
	result.TotalBatches = len(batches)

	if b.options.Verbose {
		fmt.Printf("📦 Created %d batches for processing\n", len(batches))
	}

	// Step 3: Process batches concurrently
	if b.options.Verbose {
		fmt.Printf("🤖 Step 3: Processing batches with AI...\n")
	}
	aiStart := time.Now()

	err = b.processBatchesConcurrently(batches, result)
	if err != nil {
		return result, err
	}

	result.Performance.AIProcessingTimeMs = time.Since(aiStart).Milliseconds()

	// Step 4: Apply results if not dry run
	if !b.options.DryRun {
		if b.options.Verbose {
			fmt.Printf("✅ Step 4: Applying resolutions...\n")
		}
		applicationStart := time.Now()

		err = b.applyBatchResults(result)
		if err != nil {
			return result, err
		}

		result.Performance.ApplicationTimeMs = time.Since(applicationStart).Milliseconds()
	}

	// Finalize results
	result.Performance.TotalTimeMs = time.Since(startTime).Milliseconds()
	result.Success = result.FailedBatches == 0
	b.calculatePerformanceMetrics(result)

	if b.options.Verbose {
		b.printSummary(result)
	}

	// Output results
	if err := b.outputResults(result); err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to output results: %v", err)
		return result, err
	}

	return result, nil
}

// detectConflicts detects conflicts in the repository
func (b *BatchCommand) detectConflicts(result *BatchResult) (*payload.ConflictPayload, error) {
	// Reuse existing detect functionality
	detectResult, err := DetectConflicts(b.options.RepoPath)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Conflict detection failed: %v", err)
		return nil, err
	}

	if detectResult.ConflictPayload == nil {
		result.ErrorMessage = "No conflict payload generated"
		return nil, fmt.Errorf("no conflict payload generated")
	}

	// Convert to proper payload format
	conflictPayload, err := payload.BuildSimplePayload(detectResult.ConflictReport)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to build payload: %v", err)
		return nil, err
	}

	return conflictPayload, nil
}

// countTotalConflicts counts the total number of conflicts
func (b *BatchCommand) countTotalConflicts(conflictPayload *payload.ConflictPayload) int {
	total := 0
	for _, file := range conflictPayload.Files {
		total += len(file.Conflicts)
	}
	return total
}

// createBatches groups conflicts into processing batches
func (b *BatchCommand) createBatches(conflictPayload *payload.ConflictPayload, result *BatchResult) ([]ConflictBatch, error) {
	switch b.options.GroupBy {
	case "language":
		return b.createBatchesByLanguage(conflictPayload)
	case "file":
		return b.createBatchesByFile(conflictPayload)
	case "size":
		return b.createBatchesBySize(conflictPayload)
	case "none":
		return b.createBatchesSequential(conflictPayload)
	default:
		return nil, fmt.Errorf("unsupported grouping strategy: %s", b.options.GroupBy)
	}
}

// createBatchesByLanguage groups conflicts by programming language
func (b *BatchCommand) createBatchesByLanguage(conflictPayload *payload.ConflictPayload) ([]ConflictBatch, error) {
	languageGroups := make(map[string][]payload.ConflictFilePayload)

	// Group files by language
	for _, file := range conflictPayload.Files {
		lang := file.Language
		if lang == "" {
			lang = "unknown"
		}
		languageGroups[lang] = append(languageGroups[lang], file)
	}

	var batches []ConflictBatch
	batchID := 0

	// Create batches within each language group
	for language, files := range languageGroups {
		batches = append(batches, b.createBatchesFromFiles(files, &batchID, fmt.Sprintf("language:%s", language))...)
	}

	// Sort batches by priority (more conflicts = higher priority)
	sort.Slice(batches, func(i, j int) bool {
		return batches[i].TotalConflicts > batches[j].TotalConflicts
	})

	return batches, nil
}

// createBatchesByFile creates one batch per file
func (b *BatchCommand) createBatchesByFile(conflictPayload *payload.ConflictPayload) ([]ConflictBatch, error) {
	var batches []ConflictBatch

	for i, file := range conflictPayload.Files {
		batch := ConflictBatch{
			ID:               i,
			Files:            []payload.ConflictFilePayload{file},
			TotalConflicts:   len(file.Conflicts),
			EstimatedTokens:  b.estimateTokens([]payload.ConflictFilePayload{file}),
			GroupingCriteria: fmt.Sprintf("file:%s", file.Path),
			Priority:         len(file.Conflicts), // Files with more conflicts get higher priority
		}
		batches = append(batches, batch)
	}

	// Sort by priority
	sort.Slice(batches, func(i, j int) bool {
		return batches[i].Priority > batches[j].Priority
	})

	return batches, nil
}

// createBatchesBySize creates batches based on estimated token size
func (b *BatchCommand) createBatchesBySize(conflictPayload *payload.ConflictPayload) ([]ConflictBatch, error) {
	var batches []ConflictBatch
	batchID := 0
	currentBatch := ConflictBatch{
		ID:               batchID,
		GroupingCriteria: "size",
	}
	currentTokens := 0

	for _, file := range conflictPayload.Files {
		fileTokens := b.estimateTokens([]payload.ConflictFilePayload{file})

		// If adding this file would exceed token limit, start new batch
		if currentTokens+fileTokens > b.options.MaxTokens && len(currentBatch.Files) > 0 {
			currentBatch.TotalConflicts = b.countConflictsInFiles(currentBatch.Files)
			currentBatch.EstimatedTokens = currentTokens
			currentBatch.Priority = currentBatch.TotalConflicts
			batches = append(batches, currentBatch)

			batchID++
			currentBatch = ConflictBatch{
				ID:               batchID,
				GroupingCriteria: "size",
			}
			currentTokens = 0
		}

		currentBatch.Files = append(currentBatch.Files, file)
		currentTokens += fileTokens

		// If a single file exceeds the token limit, force it into its own batch
		if fileTokens > b.options.MaxTokens {
			currentBatch.TotalConflicts = b.countConflictsInFiles(currentBatch.Files)
			currentBatch.EstimatedTokens = currentTokens
			currentBatch.Priority = currentBatch.TotalConflicts
			batches = append(batches, currentBatch)

			batchID++
			currentBatch = ConflictBatch{
				ID:               batchID,
				GroupingCriteria: "size",
			}
			currentTokens = 0
		}
	}

	// Add the last batch if it has files
	if len(currentBatch.Files) > 0 {
		currentBatch.TotalConflicts = b.countConflictsInFiles(currentBatch.Files)
		currentBatch.EstimatedTokens = currentTokens
		currentBatch.Priority = currentBatch.TotalConflicts
		batches = append(batches, currentBatch)
	}

	return batches, nil
}

// createBatchesSequential creates batches by splitting files sequentially
func (b *BatchCommand) createBatchesSequential(conflictPayload *payload.ConflictPayload) ([]ConflictBatch, error) {
	var batches []ConflictBatch
	batchID := 0

	for i := 0; i < len(conflictPayload.Files); i += b.options.BatchSize {
		end := i + b.options.BatchSize
		if end > len(conflictPayload.Files) {
			end = len(conflictPayload.Files)
		}

		files := conflictPayload.Files[i:end]
		batch := ConflictBatch{
			ID:               batchID,
			Files:            files,
			TotalConflicts:   b.countConflictsInFiles(files),
			EstimatedTokens:  b.estimateTokens(files),
			GroupingCriteria: "sequential",
			Priority:         len(files),
		}
		batches = append(batches, batch)
		batchID++
	}

	return batches, nil
}

// createBatchesFromFiles creates batches from a set of files
func (b *BatchCommand) createBatchesFromFiles(files []payload.ConflictFilePayload, batchID *int, criteria string) []ConflictBatch {
	var batches []ConflictBatch

	for i := 0; i < len(files); i += b.options.BatchSize {
		end := i + b.options.BatchSize
		if end > len(files) {
			end = len(files)
		}

		batchFiles := files[i:end]
		batch := ConflictBatch{
			ID:               *batchID,
			Files:            batchFiles,
			TotalConflicts:   b.countConflictsInFiles(batchFiles),
			EstimatedTokens:  b.estimateTokens(batchFiles),
			GroupingCriteria: criteria,
			Priority:         b.countConflictsInFiles(batchFiles),
		}
		batches = append(batches, batch)
		(*batchID)++
	}

	return batches
}

// countConflictsInFiles counts conflicts in a slice of files
func (b *BatchCommand) countConflictsInFiles(files []payload.ConflictFilePayload) int {
	total := 0
	for _, file := range files {
		total += len(file.Conflicts)
	}
	return total
}

// estimateTokens provides a rough estimate of tokens for a set of files
func (b *BatchCommand) estimateTokens(files []payload.ConflictFilePayload) int {
	tokens := 0
	for _, file := range files {
		// Rough estimation: ~4 characters per token
		tokens += len(file.Path) / 4
		for _, conflict := range file.Conflicts {
			for _, line := range conflict.OursLines {
				tokens += len(line) / 4
			}
			for _, line := range conflict.TheirsLines {
				tokens += len(line) / 4
			}
		}
	}
	return tokens
}

// processBatchesConcurrently processes batches with controlled concurrency
func (b *BatchCommand) processBatchesConcurrently(batches []ConflictBatch, result *BatchResult) error {
	// Create a semaphore to control concurrency
	sem := make(chan struct{}, b.options.Concurrency)

	// Channel to collect results
	resultsChan := make(chan BatchItemResult, len(batches))

	// WaitGroup to wait for all goroutines
	var wg sync.WaitGroup

	// Progress reporter
	var progress *ProgressReporter
	if b.options.Progress {
		progress = NewProgressReporter(len(batches), b.options.Verbose)
	}

	// Process batches concurrently
	for _, batch := range batches {
		wg.Add(1)
		go func(batch ConflictBatch) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			// Process the batch
			batchResult := b.processSingleBatch(batch)

			// Send result
			resultsChan <- batchResult

			// Update progress
			if progress != nil {
				progress.Update(len(result.BatchResults)+1,
					fmt.Sprintf("Batch %d/%d completed", len(result.BatchResults)+1, len(batches)))
			}
		}(batch)
	}

	// Close results channel when all goroutines complete
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
	for batchResult := range resultsChan {
		result.BatchResults = append(result.BatchResults, batchResult)
		result.ProcessedBatches++

		if batchResult.Success {
			result.SuccessfulBatches++
			result.TotalResolutions += len(batchResult.Resolutions)
			result.AppliedResolutions += batchResult.AppliedCount
			result.SkippedResolutions += batchResult.SkippedCount
			result.FailedResolutions += batchResult.FailedCount
		} else {
			result.FailedBatches++
			if batchResult.ErrorMessage != "" {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("Batch %d failed: %s", batchResult.BatchID, batchResult.ErrorMessage))
			}
		}

		// Stream results if requested
		if b.options.Streaming && b.options.Verbose {
			b.printBatchResult(batchResult)
		}
	}

	if progress != nil {
		progress.Complete("All batches processed")
	}

	return nil
}

// processSingleBatch processes a single batch of conflicts
func (b *BatchCommand) processSingleBatch(batch ConflictBatch) BatchItemResult {
	startTime := time.Now()

	result := BatchItemResult{
		BatchID:          batch.ID,
		ConflictsInBatch: batch.TotalConflicts,
		Success:          false,
	}

	// Create payload for this batch
	batchPayload := &payload.ConflictPayload{
		Metadata: payload.PayloadMetadata{
			Timestamp:      time.Now(),
			RepoPath:       b.options.RepoPath,
			TotalFiles:     len(batch.Files),
			TotalConflicts: batch.TotalConflicts,
			Version:        "1.0.0",
		},
		Files: batch.Files,
	}

	// Create AI apply options for this batch
	aiOptions := AIApplyOptions{
		RepoPath:       b.options.RepoPath,
		DryRun:         b.options.DryRun,
		Verbose:        false, // Suppress individual batch verbosity
		AutoApply:      b.options.AutoApply,
		MinConfidence:  b.options.MinConfidence,
		BackupFiles:    b.options.BackupFiles,
		MaxRetries:     b.options.MaxRetries,
		TimeoutSeconds: b.options.TimeoutSec,
	}

	// Create temporary payload file
	tmpFile, err := os.CreateTemp("", fmt.Sprintf("syncwright-batch-%d-*.json", batch.ID))
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to create temp file: %v", err)
		result.ProcessingTimeMs = time.Since(startTime).Milliseconds()
		return result
	}
	defer func() {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
	}()

	// Write payload to temp file
	payloadData, err := json.Marshal(batchPayload)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to marshal payload: %v", err)
		result.ProcessingTimeMs = time.Since(startTime).Milliseconds()
		return result
	}

	if _, err := tmpFile.Write(payloadData); err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to write payload: %v", err)
		result.ProcessingTimeMs = time.Since(startTime).Milliseconds()
		return result
	}
	tmpFile.Close()

	aiOptions.PayloadFile = tmpFile.Name()

	// Process with AI
	aiCmd, err := NewAIApplyCommand(aiOptions)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to create AI command: %v", err)
		result.ProcessingTimeMs = time.Since(startTime).Milliseconds()
		return result
	}
	defer aiCmd.Close()

	aiResult, err := aiCmd.Execute()
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("AI processing failed: %v", err)
		result.ProcessingTimeMs = time.Since(startTime).Milliseconds()
		return result
	}

	// Populate results
	result.Success = aiResult.Success
	result.Resolutions = aiResult.Resolutions
	result.AppliedCount = aiResult.AppliedResolutions
	result.SkippedCount = aiResult.SkippedResolutions
	result.FailedCount = aiResult.FailedResolutions
	result.ProcessingTimeMs = time.Since(startTime).Milliseconds()

	if aiResult.ErrorMessage != "" {
		result.ErrorMessage = aiResult.ErrorMessage
	}

	// Calculate average confidence
	if len(result.Resolutions) > 0 {
		totalConfidence := 0.0
		for _, res := range result.Resolutions {
			totalConfidence += res.Confidence
		}
		result.AverageConfidence = totalConfidence / float64(len(result.Resolutions))
	}

	return result
}

// applyBatchResults applies all successful batch results (placeholder for now)
func (b *BatchCommand) applyBatchResults(result *BatchResult) error {
	// Results are already applied by individual batches in processSingleBatch
	// This method is a placeholder for any post-processing needed
	return nil
}

// calculatePerformanceMetrics calculates performance statistics
func (b *BatchCommand) calculatePerformanceMetrics(result *BatchResult) {
	if len(result.BatchResults) == 0 {
		return
	}

	// Calculate average latency
	totalLatency := int64(0)
	for _, batchResult := range result.BatchResults {
		totalLatency += batchResult.ProcessingTimeMs
	}
	result.Performance.AverageLatencyMs = totalLatency / int64(len(result.BatchResults))

	// Calculate throughput (conflicts per second)
	if result.Performance.TotalTimeMs > 0 {
		result.Performance.ThroughputPerSec = float64(result.TotalConflicts) / (float64(result.Performance.TotalTimeMs) / 1000.0)
	}

	// Calculate average confidence
	totalConfidence := 0.0
	totalResolutions := 0
	for _, batchResult := range result.BatchResults {
		for _, resolution := range batchResult.Resolutions {
			totalConfidence += resolution.Confidence
			totalResolutions++
		}
	}
	if totalResolutions > 0 {
		result.AverageConfidence = totalConfidence / float64(totalResolutions)
	}
}

// printBatchResult prints the result of a single batch
func (b *BatchCommand) printBatchResult(batchResult BatchItemResult) {
	status := "✅"
	if !batchResult.Success {
		status = "❌"
	}

	fmt.Printf("%s Batch %d: %d conflicts, %d resolutions (%.1fs)\n",
		status, batchResult.BatchID, batchResult.ConflictsInBatch,
		len(batchResult.Resolutions), float64(batchResult.ProcessingTimeMs)/1000.0)
}

// printSummary prints a summary of the batch processing results
func (b *BatchCommand) printSummary(result *BatchResult) {
	fmt.Printf("\n🎉 Batch processing completed!\n")
	fmt.Printf("   Total conflicts: %d\n", result.TotalConflicts)
	fmt.Printf("   Batches processed: %d/%d\n", result.ProcessedBatches, result.TotalBatches)
	fmt.Printf("   Successful batches: %d\n", result.SuccessfulBatches)
	fmt.Printf("   Failed batches: %d\n", result.FailedBatches)
	fmt.Printf("   Total resolutions: %d\n", result.TotalResolutions)
	fmt.Printf("   Applied resolutions: %d\n", result.AppliedResolutions)
	fmt.Printf("   Skipped resolutions: %d\n", result.SkippedResolutions)
	fmt.Printf("   Failed resolutions: %d\n", result.FailedResolutions)
	fmt.Printf("   Average confidence: %.2f\n", result.AverageConfidence)
	fmt.Printf("   Processing time: %.1fs\n", float64(result.Performance.TotalTimeMs)/1000.0)
	fmt.Printf("   Throughput: %.1f conflicts/sec\n", result.Performance.ThroughputPerSec)
	fmt.Printf("   Average batch latency: %.1fs\n", float64(result.Performance.AverageLatencyMs)/1000.0)

	if len(result.Warnings) > 0 {
		fmt.Printf("\n⚠️  Warnings:\n")
		for _, warning := range result.Warnings {
			fmt.Printf("   - %s\n", warning)
		}
	}
}

// outputResults outputs the batch processing results
func (b *BatchCommand) outputResults(result *BatchResult) error {
	if b.options.OutputFile == "" {
		// Output JSON to stdout if no output file specified
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal results: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	// Write to file
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal results: %w", err)
	}

	err = os.WriteFile(b.options.OutputFile, data, 0600)
	if err != nil {
		return fmt.Errorf("failed to write results to file: %w", err)
	}

	if b.options.Verbose {
		fmt.Printf("📄 Results written to: %s\n", b.options.OutputFile)
	}

	return nil
}

// BatchConflicts is a convenience function for batch conflict resolution
func BatchConflicts(repoPath string, options BatchOptions) (*BatchResult, error) {
	if options.RepoPath == "" {
		options.RepoPath = repoPath
	}

	cmd := NewBatchCommand(options)
	return cmd.Execute()
}
