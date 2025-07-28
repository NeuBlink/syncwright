package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"runtime"
	"sync"
	"time"

	"github.com/NeuBlink/syncwright/internal/gitutils"
)

// StreamingJSONEncoder provides memory-efficient JSON encoding for large conflict datasets
type StreamingJSONEncoder struct {
	writer       io.Writer
	encoder      *json.Encoder
	monitor      *MemoryMonitor
	config       *MemoryConfig
	writeHeader  bool
	fileCount    int
	currentIndex int
	mu           sync.Mutex
}

// NewStreamingJSONEncoder creates a new streaming JSON encoder
func NewStreamingJSONEncoder(writer io.Writer, monitor *MemoryMonitor, config *MemoryConfig) *StreamingJSONEncoder {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	
	return &StreamingJSONEncoder{
		writer:      writer,
		encoder:     encoder,
		monitor:     monitor,
		config:      config,
		writeHeader: true,
	}
}

// StreamingPayload represents the structure for streaming JSON output
type StreamingPayload struct {
	Success         bool              `json:"success"`
	Summary         DetectSummary     `json:"summary"`
	Files           []SimplifiedFilePayload `json:"files,omitempty"`
	ErrorMessage    string            `json:"error_message,omitempty"`
	MemoryStats     *MemoryStats      `json:"memory_stats,omitempty"`
}

// WriteHeader writes the JSON header with metadata
func (s *StreamingJSONEncoder) WriteHeader(summary DetectSummary) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if !s.writeHeader {
		return nil
	}
	
	header := StreamingPayload{
		Success: true,
		Summary: summary,
		Files:   make([]SimplifiedFilePayload, 0), // Empty array to be populated
	}
	
	// Write opening structure
	fmt.Fprintf(s.writer, "{\n")
	fmt.Fprintf(s.writer, "  \"success\": %t,\n", header.Success)
	fmt.Fprintf(s.writer, "  \"summary\": ")
	
	summaryBytes, err := json.MarshalIndent(summary, "  ", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal summary: %w", err)
	}
	
	s.writer.Write(summaryBytes)
	fmt.Fprintf(s.writer, ",\n")
	fmt.Fprintf(s.writer, "  \"files\": [\n")
	
	s.writeHeader = false
	return nil
}

// WriteFile writes a single file payload to the stream
func (s *StreamingJSONEncoder) WriteFile(filePayload SimplifiedFilePayload) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Add comma if not the first file
	if s.currentIndex > 0 {
		fmt.Fprintf(s.writer, ",\n")
	}
	
	// Marshal and write the file payload with proper indentation
	fileBytes, err := json.MarshalIndent(filePayload, "    ", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal file payload: %w", err)
	}
	
	// Write with proper indentation
	fmt.Fprintf(s.writer, "    ")
	s.writer.Write(fileBytes)
	
	s.currentIndex++
	
	// Check memory pressure after each file
	if s.monitor != nil {
		s.monitor.IncrementProcessedFiles()
		
		if s.currentIndex%10 == 0 { // Check every 10 files
			if underPressure, stats, err := s.monitor.CheckMemoryPressure(); err == nil && underPressure {
				fmt.Printf("Memory pressure detected: %dMB allocated\n", stats.AllocMB)
				runtime.GC() // Force garbage collection
			}
		}
	}
	
	return nil
}

// WriteFooter writes the JSON footer and closes the structure
func (s *StreamingJSONEncoder) WriteFooter(memoryStats *MemoryStats, errorMessage string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Close files array
	fmt.Fprintf(s.writer, "\n  ]")
	
	// Add memory stats if available
	if memoryStats != nil {
		fmt.Fprintf(s.writer, ",\n  \"memory_stats\": ")
		statsBytes, err := json.MarshalIndent(memoryStats, "  ", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal memory stats: %w", err)
		}
		s.writer.Write(statsBytes)
	}
	
	// Add error message if present
	if errorMessage != "" {
		fmt.Fprintf(s.writer, ",\n  \"error_message\": %s", 
			string(mustMarshal(errorMessage)))
	}
	
	// Close main object
	fmt.Fprintf(s.writer, "\n}\n")
	
	return nil
}

// mustMarshal is a helper function that panics on marshal error (for simple types)
func mustMarshal(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal: %v", err))
	}
	return data
}

// FileProcessor handles concurrent processing of conflict files with memory management
type FileProcessor struct {
	monitor    *MemoryMonitor
	config     *MemoryConfig
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewFileProcessor creates a new file processor with memory monitoring
func NewFileProcessor(monitor *MemoryMonitor, config *MemoryConfig) *FileProcessor {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &FileProcessor{
		monitor: monitor,
		config:  config,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// ProcessResult represents the result of processing a single file
type ProcessResult struct {
	FilePayload *SimplifiedFilePayload
	Error       error
	Index       int
}

// ProcessFilesStreaming processes conflict files using a streaming approach with memory monitoring
func (fp *FileProcessor) ProcessFilesStreaming(
	conflictFiles []gitutils.ConflictFile,
	detectCmd *DetectCommand,
	resultCallback func(ProcessResult),
) error {
	defer fp.cancel()
	
	// Start memory monitoring
	memoryWatchCtx, cancelWatch := context.WithCancel(fp.ctx)
	defer cancelWatch()
	
	go fp.monitor.StartMemoryWatcher(memoryWatchCtx, func(stats *MemoryStats) {
		if detectCmd.options.Verbose {
			fmt.Printf("Memory pressure: %dMB/%dMB (processed: %d files)\n",
				stats.AllocMB, fp.config.MaxMemoryMB, stats.ProcessedFiles)
		}
		fp.monitor.ForceGC()
	})
	
	// Process files in batches to manage memory
	totalFiles := len(conflictFiles)
	batchSize := fp.config.BatchSize
	
	if totalFiles > 1000 {
		fp.config.OptimizeForLargeRepo(totalFiles)
		batchSize = fp.config.BatchSize
	}
	
	for i := 0; i < totalFiles; i += batchSize {
		end := i + batchSize
		if end > totalFiles {
			end = totalFiles
		}
		
		batch := conflictFiles[i:end]
		
		if err := fp.processBatch(batch, i, detectCmd, resultCallback); err != nil {
			return fmt.Errorf("error processing batch starting at %d: %w", i, err)
		}
		
		// Check context cancellation
		select {
		case <-fp.ctx.Done():
			return fp.ctx.Err()
		default:
		}
		
		// Force GC between batches for large repositories
		if totalFiles > 1000 {
			runtime.GC()
			time.Sleep(10 * time.Millisecond) // Brief pause to allow GC
		}
	}
	
	return nil
}

// processBatch processes a batch of files concurrently with memory management
func (fp *FileProcessor) processBatch(
	batch []gitutils.ConflictFile,
	startIndex int,
	detectCmd *DetectCommand,
	resultCallback func(ProcessResult),
) error {
	// Create worker pool
	jobs := make(chan struct {
		file  gitutils.ConflictFile
		index int
	}, len(batch))
	
	results := make(chan ProcessResult, len(batch))
	
	// Start workers
	for w := 0; w < fp.config.WorkerPoolSize; w++ {
		go fp.worker(jobs, results, detectCmd)
	}
	
	// Send jobs
	for i, file := range batch {
		jobs <- struct {
			file  gitutils.ConflictFile
			index int
		}{file, startIndex + i}
	}
	close(jobs)
	
	// Collect results
	for i := 0; i < len(batch); i++ {
		select {
		case result := <-results:
			resultCallback(result)
		case <-fp.ctx.Done():
			return fp.ctx.Err()
		}
	}
	
	return nil
}

// worker processes individual files in the worker pool
func (fp *FileProcessor) worker(
	jobs <-chan struct {
		file  gitutils.ConflictFile
		index int
	},
	results chan<- ProcessResult,
	detectCmd *DetectCommand,
) {
	for job := range jobs {
		result := ProcessResult{Index: job.index}
		
		// Skip files that should be excluded
		if detectCmd.shouldSkipFile(job.file.Path) {
			fp.monitor.IncrementSkippedFiles()
			result.FilePayload = nil // Indicates skipped
			results <- result
			continue
		}
		
		// Process the file
		language := detectLanguage(job.file.Path)
		filePayload := SimplifiedFilePayload{
			Path:     job.file.Path,
			Language: language,
		}
		
		// Convert conflict hunks to simplified format
		for i, hunk := range job.file.Hunks {
			conflictHunk := SimplifiedConflictHunk{
				ID:          fmt.Sprintf("%s:%d", job.file.Path, i),
				StartLine:   hunk.StartLine,
				EndLine:     hunk.EndLine,
				OursLines:   hunk.OursLines,
				TheirsLines: hunk.TheirsLines,
				PreContext:  detectCmd.extractPreContext(job.file.Context, hunk.StartLine),
				PostContext: detectCmd.extractPostContext(job.file.Context, hunk.EndLine),
			}
			filePayload.Conflicts = append(filePayload.Conflicts, conflictHunk)
		}
		
		result.FilePayload = &filePayload
		results <- result
	}
}

// Close cancels the file processor context
func (fp *FileProcessor) Close() {
	fp.cancel()
}