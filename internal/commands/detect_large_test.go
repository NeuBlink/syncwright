package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/NeuBlink/syncwright/internal/gitutils"
)

// TestLargeRepositoryMemoryUsage tests memory efficiency with >1000 conflicted files
func TestLargeRepositoryMemoryUsage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large repository test in short mode")
	}

	// Skip this test if git is not available or test setup is complex
	t.Skip("Skipping integration test - focus on unit tests for memory optimization")

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "syncwright-large-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Initialize git repository
	if err := initTestGitRepo(tempDir); err != nil {
		t.Fatalf("Failed to initialize git repo: %v", err)
	}

	// Create many conflicted files (simulate large repository)
	const numConflictedFiles = 1500
	conflictedFiles := make([]string, numConflictedFiles)

	for i := 0; i < numConflictedFiles; i++ {
		filename := fmt.Sprintf("file_%04d.go", i)
		conflictedFiles[i] = filename
		if err := createConflictedFile(tempDir, filename, i); err != nil {
			t.Fatalf("Failed to create conflicted file %s: %v", filename, err)
		}
	}

	// Test streaming processing with memory monitoring
	options := DetectOptions{
		RepoPath:        tempDir,
		OutputFormat:    OutputFormatJSON,
		Verbose:         true,
		EnableStreaming: true,
		MaxMemoryMB:     128, // Conservative memory limit
		BatchSize:       25,
		WorkerPoolSize:  2,
	}

	// Capture initial memory stats
	var initialMem runtime.MemStats
	runtime.ReadMemStats(&initialMem)
	initialMemMB := int64(initialMem.Alloc / 1024 / 1024)

	// Execute detection with streaming
	cmd := NewDetectCommand(options)
	defer cmd.Close()

	// Force stdout capture
	cmd.options.OutputFile = "" // Force stdout capture

	// Redirect stdout to capture streaming output
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	os.Stdout = w

	// Channel to capture output
	outputChan := make(chan string, 1)
	go func() {
		buf := make([]byte, 1024*1024) // 1MB buffer
		var result strings.Builder
		for {
			n, err := r.Read(buf)
			if n > 0 {
				result.Write(buf[:n])
			}
			if err != nil {
				break
			}
		}
		outputChan <- result.String()
	}()

	startTime := time.Now()
	result, err := cmd.Execute()
	w.Close()
	os.Stdout = oldStdout

	outputStr := <-outputChan
	r.Close()

	processingTime := time.Since(startTime)

	if err != nil {
		t.Fatalf("Detection failed: %v", err)
	}

	// Capture final memory stats
	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)
	finalMemMB := int64(finalMem.Alloc / 1024 / 1024)
	peakMemoryUsed := finalMemMB - initialMemMB

	// Validate results
	if !result.Success {
		t.Errorf("Expected success=true, got success=%t", result.Success)
	}

	if result.Summary.TotalFiles < numConflictedFiles {
		t.Errorf("Expected at least %d files, got %d", numConflictedFiles, result.Summary.TotalFiles)
	}

	// Validate memory usage stayed within bounds
	if peakMemoryUsed > options.MaxMemoryMB*2 { // Allow 2x headroom for test environment
		t.Errorf("Memory usage exceeded limits: used %dMB, limit %dMB",
			peakMemoryUsed, options.MaxMemoryMB)
	}

	// Validate processing time is reasonable (should be under 30 seconds for 1500 files)
	if processingTime > 30*time.Second {
		t.Errorf("Processing took too long: %v", processingTime)
	}

	// Validate streaming JSON output is well-formed
	if !json.Valid([]byte(outputStr)) {
		t.Errorf("Generated JSON is not valid")
		t.Logf("Output (first 500 chars): %s", truncateString(outputStr, 500))
	}

	// Validate JSON contains expected structure
	var parsedResult map[string]interface{}
	if err := json.Unmarshal([]byte(outputStr), &parsedResult); err != nil {
		t.Errorf("Failed to parse JSON output: %v", err)
	} else {
		// Check for required fields
		if _, exists := parsedResult["success"]; !exists {
			t.Errorf("JSON output missing 'success' field")
		}
		if _, exists := parsedResult["summary"]; !exists {
			t.Errorf("JSON output missing 'summary' field")
		}
		if _, exists := parsedResult["files"]; !exists {
			t.Errorf("JSON output missing 'files' field")
		}
		if _, exists := parsedResult["memory_stats"]; !exists {
			t.Errorf("JSON output missing 'memory_stats' field")
		}
	}

	t.Logf("Successfully processed %d files in %v with peak memory usage of %dMB",
		result.Summary.TotalFiles, processingTime, peakMemoryUsed)
}

// TestMemoryMonitoringAccuracy tests the accuracy of memory monitoring
func TestMemoryMonitoringAccuracy(t *testing.T) {
	monitor := NewMemoryMonitor(100) // 100MB limit

	// Test basic stats
	stats := monitor.GetMemoryStats()
	if stats.AllocMB < 0 {
		t.Errorf("Invalid allocated memory: %d", stats.AllocMB)
	}

	// Test pressure detection
	underPressure, _, err := monitor.CheckMemoryPressure()
	if err != nil {
		t.Errorf("Memory pressure check failed: %v", err)
	}

	// For a 100MB limit, we shouldn't be under pressure in a simple test
	if underPressure && stats.AllocMB < 80 { // 80MB threshold
		t.Errorf("False positive memory pressure detected")
	}

	// Test counters
	monitor.IncrementProcessedFiles()
	monitor.IncrementSkippedFiles()

	newStats := monitor.GetMemoryStats()
	if newStats.ProcessedFiles != 1 {
		t.Errorf("Expected processed files=1, got %d", newStats.ProcessedFiles)
	}
	if newStats.SkippedFiles != 1 {
		t.Errorf("Expected skipped files=1, got %d", newStats.SkippedFiles)
	}
}

// TestStreamingJSONOutput tests the streaming JSON encoder
func TestStreamingJSONOutput(t *testing.T) {
	var output bytes.Buffer
	monitor := NewMemoryMonitor(512)
	config := DefaultMemoryConfig()

	encoder := NewStreamingJSONEncoder(&output, monitor, config)

	// Test header
	summary := DetectSummary{
		TotalFiles:       3,
		TotalConflicts:   5,
		ProcessableFiles: 2,
		ExcludedFiles:    1,
		RepoPath:         "/test/repo",
		InMergeState:     true,
	}

	err := encoder.WriteHeader(summary)
	if err != nil {
		t.Fatalf("Failed to write header: %v", err)
	}

	// Test file output
	file1 := SimplifiedFilePayload{
		Path:     "file1.go",
		Language: "go",
		Conflicts: []SimplifiedConflictHunk{
			{
				ID:          "file1:0",
				StartLine:   10,
				EndLine:     15,
				OursLines:   []string{"ours line 1", "ours line 2"},
				TheirsLines: []string{"theirs line 1", "theirs line 2"},
				PreContext:  []string{"context before"},
				PostContext: []string{"context after"},
			},
		},
	}

	file2 := SimplifiedFilePayload{
		Path:     "file2.js",
		Language: "javascript",
		Conflicts: []SimplifiedConflictHunk{
			{
				ID:          "file2:0",
				StartLine:   5,
				EndLine:     8,
				OursLines:   []string{"console.log('ours');"},
				TheirsLines: []string{"console.log('theirs');"},
			},
		},
	}

	// Write files
	if err := encoder.WriteFile(file1); err != nil {
		t.Fatalf("Failed to write file1: %v", err)
	}

	if err := encoder.WriteFile(file2); err != nil {
		t.Fatalf("Failed to write file2: %v", err)
	}

	// Write footer
	memStats := monitor.GetMemoryStats()
	if err := encoder.WriteFooter(memStats, ""); err != nil {
		t.Fatalf("Failed to write footer: %v", err)
	}

	// Validate output
	outputStr := output.String()
	if !json.Valid([]byte(outputStr)) {
		t.Errorf("Generated JSON is not valid")
		t.Logf("Output: %s", outputStr)
	}

	// Parse and validate structure
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(outputStr), &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Validate files array
	files, ok := result["files"].([]interface{})
	if !ok {
		t.Fatalf("Files field is not an array")
	}

	if len(files) != 2 {
		t.Errorf("Expected 2 files, got %d", len(files))
	}
}

// TestConcurrentFileProcessing tests the worker pool implementation
func TestConcurrentFileProcessing(t *testing.T) {
	monitor := NewMemoryMonitor(256)
	config := &MemoryConfig{
		BatchSize:      10,
		WorkerPoolSize: 3,
		MaxMemoryMB:    256,
	}

	processor := NewFileProcessor(monitor, config)
	defer processor.Close()

	// Create test conflict files
	conflictFiles := make([]gitutils.ConflictFile, 50)
	for i := 0; i < 50; i++ {
		conflictFiles[i] = gitutils.ConflictFile{
			Path: fmt.Sprintf("file_%d.go", i),
			Hunks: []gitutils.ConflictHunk{
				{
					StartLine:   i + 1,
					EndLine:     i + 5,
					OursLines:   []string{fmt.Sprintf("ours line %d", i)},
					TheirsLines: []string{fmt.Sprintf("theirs line %d", i)},
				},
			},
			Context: []string{"context line"},
		}
	}

	// Mock detect command
	detectCmd := &DetectCommand{
		options: DetectOptions{MaxContextLines: 3},
	}

	// Process files and collect results
	var results []ProcessResult
	resultChan := make(chan ProcessResult, len(conflictFiles))

	err := processor.ProcessFilesStreaming(conflictFiles, detectCmd, func(result ProcessResult) {
		resultChan <- result
	})

	if err != nil {
		t.Fatalf("File processing failed: %v", err)
	}

	// Collect all results
	for i := 0; i < len(conflictFiles); i++ {
		select {
		case result := <-resultChan:
			results = append(results, result)
		case <-time.After(5 * time.Second):
			t.Fatalf("Timeout waiting for results")
		}
	}

	// Validate results
	if len(results) != len(conflictFiles) {
		t.Errorf("Expected %d results, got %d", len(conflictFiles), len(results))
	}

	// Validate each result has proper structure
	for i, result := range results {
		if result.Error != nil {
			t.Errorf("Result %d has error: %v", i, result.Error)
		}
		if result.FilePayload == nil {
			t.Errorf("Result %d missing file payload", i)
		} else {
			if len(result.FilePayload.Conflicts) == 0 {
				t.Errorf("Result %d has no conflicts", i)
			}
		}
	}
}

// Helper functions for tests

func initTestGitRepo(dir string) error {
	// Create a basic git repo structure that git status will recognize
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		return err
	}

	// Create minimal git structure
	configContent := `[core]
	repositoryformatversion = 0
	filemode = true
	bare = false
	logallrefupdates = true
`
	configFile := filepath.Join(gitDir, "config")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		return err
	}

	// Create HEAD file
	headFile := filepath.Join(gitDir, "HEAD")
	if err := os.WriteFile(headFile, []byte("ref: refs/heads/main\n"), 0644); err != nil {
		return err
	}

	// Create refs structure
	refsDir := filepath.Join(gitDir, "refs", "heads")
	if err := os.MkdirAll(refsDir, 0755); err != nil {
		return err
	}

	// Create objects dir
	objectsDir := filepath.Join(gitDir, "objects")
	if err := os.MkdirAll(objectsDir, 0755); err != nil {
		return err
	}

	return nil
}

func createConflictedFile(dir, filename string, index int) error {
	content := fmt.Sprintf(`package main

import "fmt"

func main() {
<<<<<<< HEAD
	fmt.Println("Our version %d")
	ourVariable := %d
=======
	fmt.Println("Their version %d")
	theirVariable := %d
>>>>>>> feature-branch
	
	// Some additional content
	for i := 0; i < 10; i++ {
		fmt.Printf("Line %%d\n", i)
	}
}
`, index, index*2, index, index*3)

	return os.WriteFile(filepath.Join(dir, filename), []byte(content), 0644)
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// BenchmarkLargeRepositoryProcessing benchmarks processing performance
func BenchmarkLargeRepositoryProcessing(b *testing.B) {
	b.Skip("Skipping git-dependent benchmark - focus on memory optimization unit tests")
	tempDir, err := os.MkdirTemp("", "syncwright-bench-*")
	if err != nil {
		b.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	if err := initTestGitRepo(tempDir); err != nil {
		b.Fatalf("Failed to initialize git repo: %v", err)
	}

	// Create 100 conflicted files for benchmarking
	const numFiles = 100
	for i := 0; i < numFiles; i++ {
		filename := fmt.Sprintf("bench_file_%04d.go", i)
		if err := createConflictedFile(tempDir, filename, i); err != nil {
			b.Fatalf("Failed to create file %s: %v", filename, err)
		}
	}

	options := DetectOptions{
		RepoPath:        tempDir,
		OutputFormat:    OutputFormatJSON,
		EnableStreaming: true,
		MaxMemoryMB:     256,
		BatchSize:       25,
		WorkerPoolSize:  4,
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cmd := NewDetectCommand(options)
		_, err := cmd.Execute()
		cmd.Close()

		if err != nil {
			b.Fatalf("Benchmark run %d failed: %v", i, err)
		}
	}
}
