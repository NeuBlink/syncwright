package commands

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/NeuBlink/syncwright/internal/gitutils"
	"github.com/NeuBlink/syncwright/internal/payload"
)

// TestBatchCommandCreation tests the creation of a batch command with default options
func TestBatchCommandCreation(t *testing.T) {
	options := BatchOptions{}
	cmd := NewBatchCommand(options)

	// Verify defaults are set correctly
	if cmd.options.BatchSize != 10 {
		t.Errorf("Expected default batch size 10, got %d", cmd.options.BatchSize)
	}
	if cmd.options.Concurrency != 3 {
		t.Errorf("Expected default concurrency 3, got %d", cmd.options.Concurrency)
	}
	if cmd.options.GroupBy != "language" {
		t.Errorf("Expected default group by 'language', got '%s'", cmd.options.GroupBy)
	}
	if cmd.options.MaxTokens != 50000 {
		t.Errorf("Expected default max tokens 50000, got %d", cmd.options.MaxTokens)
	}
	if cmd.options.MinConfidence != 0.7 {
		t.Errorf("Expected default min confidence 0.7, got %f", cmd.options.MinConfidence)
	}
}

// TestBatchCommandWithCustomOptions tests creation with custom options
func TestBatchCommandWithCustomOptions(t *testing.T) {
	options := BatchOptions{
		BatchSize:     20,
		Concurrency:   5,
		GroupBy:       "file",
		MaxTokens:     100000,
		MinConfidence: 0.8,
		Verbose:       true,
	}
	cmd := NewBatchCommand(options)

	// Verify custom options are preserved
	if cmd.options.BatchSize != 20 {
		t.Errorf("Expected batch size 20, got %d", cmd.options.BatchSize)
	}
	if cmd.options.Concurrency != 5 {
		t.Errorf("Expected concurrency 5, got %d", cmd.options.Concurrency)
	}
	if cmd.options.GroupBy != "file" {
		t.Errorf("Expected group by 'file', got '%s'", cmd.options.GroupBy)
	}
	if cmd.options.MaxTokens != 100000 {
		t.Errorf("Expected max tokens 100000, got %d", cmd.options.MaxTokens)
	}
	if cmd.options.MinConfidence != 0.8 {
		t.Errorf("Expected min confidence 0.8, got %f", cmd.options.MinConfidence)
	}
	if !cmd.options.Verbose {
		t.Error("Expected verbose to be true")
	}
}

// TestCountTotalConflicts tests the conflict counting functionality
func TestCountTotalConflicts(t *testing.T) {
	cmd := NewBatchCommand(BatchOptions{})

	// Create test payload
	conflictPayload := &payload.ConflictPayload{
		Files: []payload.ConflictFilePayload{
			{
				Path:     "file1.go",
				Language: "go",
				Conflicts: []payload.ConflictHunkPayload{
					{StartLine: 1, EndLine: 5},
					{StartLine: 10, EndLine: 15},
				},
			},
			{
				Path:     "file2.js",
				Language: "javascript",
				Conflicts: []payload.ConflictHunkPayload{
					{StartLine: 1, EndLine: 3},
				},
			},
		},
	}

	total := cmd.countTotalConflicts(conflictPayload)
	expected := 3 // 2 from file1.go + 1 from file2.js
	if total != expected {
		t.Errorf("Expected total conflicts %d, got %d", expected, total)
	}
}

// TestCreateBatchesByLanguage tests the language-based batching strategy
func TestCreateBatchesByLanguage(t *testing.T) {
	cmd := NewBatchCommand(BatchOptions{BatchSize: 2})

	// Create test payload with multiple languages
	conflictPayload := &payload.ConflictPayload{
		Files: []payload.ConflictFilePayload{
			{Path: "file1.go", Language: "go", Conflicts: []payload.ConflictHunkPayload{{}}},
			{Path: "file2.go", Language: "go", Conflicts: []payload.ConflictHunkPayload{{}}},
			{Path: "file3.go", Language: "go", Conflicts: []payload.ConflictHunkPayload{{}}},
			{Path: "file1.js", Language: "javascript", Conflicts: []payload.ConflictHunkPayload{{}}},
			{Path: "file2.js", Language: "javascript", Conflicts: []payload.ConflictHunkPayload{{}}},
		},
	}

	batches, err := cmd.createBatchesByLanguage(conflictPayload)
	if err != nil {
		t.Fatalf("Failed to create batches: %v", err)
	}

	// Should have 3 batches total: 2 for Go (due to batch size 2) and 1 for JavaScript
	if len(batches) != 3 {
		t.Errorf("Expected 3 batches, got %d", len(batches))
	}

	// Verify batches are properly grouped by language
	goFiles := 0
	jsFiles := 0
	for _, batch := range batches {
		for _, file := range batch.Files {
			if file.Language == "go" {
				goFiles++
			} else if file.Language == "javascript" {
				jsFiles++
			}
		}
	}

	if goFiles != 3 {
		t.Errorf("Expected 3 Go files across batches, got %d", goFiles)
	}
	if jsFiles != 2 {
		t.Errorf("Expected 2 JavaScript files across batches, got %d", jsFiles)
	}
}

// TestCreateBatchesByFile tests the file-based batching strategy
func TestCreateBatchesByFile(t *testing.T) {
	cmd := NewBatchCommand(BatchOptions{})

	// Create test payload
	conflictPayload := &payload.ConflictPayload{
		Files: []payload.ConflictFilePayload{
			{Path: "file1.go", Conflicts: []payload.ConflictHunkPayload{{}, {}}},     // 2 conflicts
			{Path: "file2.go", Conflicts: []payload.ConflictHunkPayload{{}}},         // 1 conflict
			{Path: "file3.py", Conflicts: []payload.ConflictHunkPayload{{}, {}, {}}}, // 3 conflicts
		},
	}

	batches, err := cmd.createBatchesByFile(conflictPayload)
	if err != nil {
		t.Fatalf("Failed to create batches: %v", err)
	}

	// Should have 3 batches (one per file)
	if len(batches) != 3 {
		t.Errorf("Expected 3 batches, got %d", len(batches))
	}

	// Verify each batch has exactly one file
	for i, batch := range batches {
		if len(batch.Files) != 1 {
			t.Errorf("Batch %d should have 1 file, got %d", i, len(batch.Files))
		}
	}

	// Verify batches are sorted by priority (conflicts count) in descending order
	if batches[0].Priority < batches[1].Priority || batches[1].Priority < batches[2].Priority {
		t.Error("Batches should be sorted by priority in descending order")
	}
}

// TestCreateBatchesBySize tests the size-based batching strategy
func TestCreateBatchesBySize(t *testing.T) {
	cmd := NewBatchCommand(BatchOptions{MaxTokens: 50}) // Very small token limit for testing

	// Create test payload with varying sizes - use longer content to exceed token limit
	conflictPayload := &payload.ConflictPayload{
		Files: []payload.ConflictFilePayload{
			{
				Path: "small1.go",
				Conflicts: []payload.ConflictHunkPayload{
					{OursLines: []string{"a short line"}, TheirsLines: []string{"another short line"}},
				},
			},
			{
				Path: "large1.go",
				Conflicts: []payload.ConflictHunkPayload{
					{
						OursLines:   []string{"this is a very long line that contains a lot of text and should consume many tokens when processed by the AI model"},
						TheirsLines: []string{"this is also a very long line that contains a lot of text and should consume many tokens when processed by the AI model"},
					},
				},
			},
			{
				Path: "large2.go",
				Conflicts: []payload.ConflictHunkPayload{
					{
						OursLines:   []string{"another very long line with substantial content that will definitely push the token count over our small limit"},
						TheirsLines: []string{"yet another very long line with substantial content that will definitely push the token count over our small limit"},
					},
				},
			},
		},
	}

	batches, err := cmd.createBatchesBySize(conflictPayload)
	if err != nil {
		t.Fatalf("Failed to create batches: %v", err)
	}

	// Should have multiple batches due to token limit
	if len(batches) == 0 {
		t.Error("Expected at least 1 batch to be created")
	}

	// Verify all files are included in batches
	totalFiles := 0
	for _, batch := range batches {
		totalFiles += len(batch.Files)
	}
	if totalFiles != 3 {
		t.Errorf("Expected all 3 files to be included in batches, got %d", totalFiles)
	}
}

// TestCreateBatchesSequential tests the sequential batching strategy
func TestCreateBatchesSequential(t *testing.T) {
	cmd := NewBatchCommand(BatchOptions{BatchSize: 2})

	// Create test payload with 5 files
	conflictPayload := &payload.ConflictPayload{
		Files: []payload.ConflictFilePayload{
			{Path: "file1.go", Conflicts: []payload.ConflictHunkPayload{{}}},
			{Path: "file2.go", Conflicts: []payload.ConflictHunkPayload{{}}},
			{Path: "file3.go", Conflicts: []payload.ConflictHunkPayload{{}}},
			{Path: "file4.go", Conflicts: []payload.ConflictHunkPayload{{}}},
			{Path: "file5.go", Conflicts: []payload.ConflictHunkPayload{{}}},
		},
	}

	batches, err := cmd.createBatchesSequential(conflictPayload)
	if err != nil {
		t.Fatalf("Failed to create batches: %v", err)
	}

	// Should have 3 batches: [2, 2, 1] files
	if len(batches) != 3 {
		t.Errorf("Expected 3 batches, got %d", len(batches))
	}

	// Verify batch sizes
	expectedSizes := []int{2, 2, 1}
	for i, batch := range batches {
		if len(batch.Files) != expectedSizes[i] {
			t.Errorf("Batch %d should have %d files, got %d", i, expectedSizes[i], len(batch.Files))
		}
	}
}

// TestEstimateTokens tests the token estimation functionality
func TestEstimateTokens(t *testing.T) {
	cmd := NewBatchCommand(BatchOptions{})

	files := []payload.ConflictFilePayload{
		{
			Path: "test.go",
			Conflicts: []payload.ConflictHunkPayload{
				{
					OursLines:   []string{"line1", "line2"},
					TheirsLines: []string{"alternative1", "alternative2"},
				},
			},
		},
	}

	tokens := cmd.estimateTokens(files)

	// Should be greater than 0
	if tokens <= 0 {
		t.Errorf("Expected token estimate > 0, got %d", tokens)
	}

	// Test with empty files
	emptyFiles := []payload.ConflictFilePayload{}
	emptyTokens := cmd.estimateTokens(emptyFiles)
	if emptyTokens != 0 {
		t.Errorf("Expected 0 tokens for empty files, got %d", emptyTokens)
	}
}

// TestCalculatePerformanceMetrics tests performance metrics calculation
func TestCalculatePerformanceMetrics(t *testing.T) {
	cmd := NewBatchCommand(BatchOptions{})

	result := &BatchResult{
		TotalConflicts: 10,
		BatchResults: []BatchItemResult{
			{ProcessingTimeMs: 1000, Resolutions: []gitutils.ConflictResolution{{Confidence: 0.8}, {Confidence: 0.9}}},
			{ProcessingTimeMs: 2000, Resolutions: []gitutils.ConflictResolution{{Confidence: 0.7}}},
		},
		Performance: BatchPerformanceMetrics{TotalTimeMs: 5000},
	}

	cmd.calculatePerformanceMetrics(result)

	// Check average latency calculation
	expectedAvgLatency := int64(1500) // (1000 + 2000) / 2
	if result.Performance.AverageLatencyMs != expectedAvgLatency {
		t.Errorf("Expected average latency %d, got %d", expectedAvgLatency, result.Performance.AverageLatencyMs)
	}

	// Check throughput calculation
	expectedThroughput := float64(10) / (float64(5000) / 1000.0) // 10 conflicts / 5 seconds = 2.0
	if result.Performance.ThroughputPerSec != expectedThroughput {
		t.Errorf("Expected throughput %.2f, got %.2f", expectedThroughput, result.Performance.ThroughputPerSec)
	}

	// Check average confidence calculation
	expectedAvgConfidence := (0.8 + 0.9 + 0.7) / 3.0 // Average of all resolution confidences
	if result.AverageConfidence != expectedAvgConfidence {
		t.Errorf("Expected average confidence %.2f, got %.2f", expectedAvgConfidence, result.AverageConfidence)
	}
}

// BenchmarkBatchCreation benchmarks batch creation performance
func BenchmarkBatchCreation(b *testing.B) {
	cmd := NewBatchCommand(BatchOptions{BatchSize: 10})

	// Create a large test payload
	files := make([]payload.ConflictFilePayload, 1000)
	for i := 0; i < 1000; i++ {
		files[i] = payload.ConflictFilePayload{
			Path:     fmt.Sprintf("file%d.go", i),
			Language: "go",
			Conflicts: []payload.ConflictHunkPayload{
				{StartLine: 1, EndLine: 5, OursLines: []string{"line1"}, TheirsLines: []string{"line2"}},
			},
		}
	}

	conflictPayload := &payload.ConflictPayload{Files: files}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := cmd.createBatchesByLanguage(conflictPayload)
		if err != nil {
			b.Fatalf("Failed to create batches: %v", err)
		}
	}
}

// TestBatchResultSerialization tests JSON serialization of batch results
func TestBatchResultSerialization(t *testing.T) {
	result := &BatchResult{
		Success:            true,
		TotalConflicts:     5,
		TotalBatches:       2,
		ProcessedBatches:   2,
		SuccessfulBatches:  2,
		AppliedResolutions: 4,
		Performance: BatchPerformanceMetrics{
			TotalTimeMs:      3000,
			ThroughputPerSec: 1.67,
		},
	}

	// Test that the result can be marshaled to JSON
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal batch result: %v", err)
	}

	// Test that it can be unmarshaled back
	var unmarshaled BatchResult
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal batch result: %v", err)
	}

	// Verify key fields are preserved
	if unmarshaled.Success != result.Success {
		t.Error("Success field not preserved during serialization")
	}
	if unmarshaled.TotalConflicts != result.TotalConflicts {
		t.Error("TotalConflicts field not preserved during serialization")
	}
	if unmarshaled.Performance.TotalTimeMs != result.Performance.TotalTimeMs {
		t.Error("Performance metrics not preserved during serialization")
	}
}
