package gitutils

import (
	"fmt"
	"os"
	"strings"
)

// ConflictResolution represents a resolved merge conflict
type ConflictResolution struct {
	FilePath      string   `json:"file_path"`
	StartLine     int      `json:"start_line"`
	EndLine       int      `json:"end_line"`
	ResolvedLines []string `json:"resolved_lines"`
	Confidence    float64  `json:"confidence"`
	Reasoning     string   `json:"reasoning,omitempty"`
}

// ValidateResolution validates that a conflict resolution is well-formed
func ValidateResolution(resolution ConflictResolution) error {
	if resolution.FilePath == "" {
		return fmt.Errorf("file path cannot be empty")
	}

	if resolution.StartLine <= 0 {
		return fmt.Errorf("start line must be positive, got %d", resolution.StartLine)
	}

	if resolution.EndLine < resolution.StartLine {
		return fmt.Errorf("end line (%d) must be >= start line (%d)", resolution.EndLine, resolution.StartLine)
	}

	if len(resolution.ResolvedLines) == 0 {
		return fmt.Errorf("resolved lines cannot be empty")
	}

	if resolution.Confidence < 0.0 || resolution.Confidence > 1.0 {
		return fmt.Errorf("confidence must be between 0.0 and 1.0, got %f", resolution.Confidence)
	}

	// Check for remaining conflict markers
	for i, line := range resolution.ResolvedLines {
		if containsConflictMarker(line) {
			return fmt.Errorf("resolved line %d contains conflict marker: %s", i+1, line)
		}
	}

	return nil
}

// containsConflictMarker checks if a line contains git conflict markers
func containsConflictMarker(line string) bool {
	trimmed := strings.TrimSpace(line)
	conflictMarkers := []string{
		"<<<<<<<",
		"=======",
		">>>>>>>",
		"|||||||",
	}

	for _, marker := range conflictMarkers {
		if strings.HasPrefix(trimmed, marker) {
			return true
		}
	}

	return false
}

// ApplyResolution applies a conflict resolution to the original file content
func ApplyResolution(originalContent string, resolution ConflictResolution) (string, error) {
	if err := ValidateResolution(resolution); err != nil {
		return "", fmt.Errorf("invalid resolution: %w", err)
	}

	lines := strings.Split(originalContent, "\n")
	if resolution.EndLine > len(lines) {
		return "", fmt.Errorf("end line %d exceeds file length %d", resolution.EndLine, len(lines))
	}

	// Replace the lines from StartLine to EndLine with resolved lines
	// Note: Line numbers are 1-based, but slice indices are 0-based
	startIdx := resolution.StartLine - 1
	endIdx := resolution.EndLine

	var newLines []string
	newLines = append(newLines, lines[:startIdx]...)
	newLines = append(newLines, resolution.ResolvedLines...)
	if endIdx < len(lines) {
		newLines = append(newLines, lines[endIdx:]...)
	}

	return strings.Join(newLines, "\n"), nil
}

// ApplyMultipleResolutions applies multiple conflict resolutions to file content
func ApplyMultipleResolutions(originalContent string, resolutions []ConflictResolution) (string, error) {
	if len(resolutions) == 0 {
		return originalContent, nil
	}

	// Sort resolutions by line number (highest first) to avoid line number shifts
	sortedResolutions := make([]ConflictResolution, len(resolutions))
	copy(sortedResolutions, resolutions)

	// Simple bubble sort by StartLine (descending)
	for i := 0; i < len(sortedResolutions)-1; i++ {
		for j := 0; j < len(sortedResolutions)-1-i; j++ {
			if sortedResolutions[j].StartLine < sortedResolutions[j+1].StartLine {
				sortedResolutions[j], sortedResolutions[j+1] = sortedResolutions[j+1], sortedResolutions[j]
			}
		}
	}

	// Apply each resolution
	result := originalContent
	for _, resolution := range sortedResolutions {
		var err error
		result, err = ApplyResolution(result, resolution)
		if err != nil {
			return "", fmt.Errorf("failed to apply resolution for %s:%d-%d: %w",
				resolution.FilePath, resolution.StartLine, resolution.EndLine, err)
		}
	}

	return result, nil
}

// CalculateResolutionStats calculates statistics for a set of resolutions
func CalculateResolutionStats(resolutions []ConflictResolution) ResolutionStats {
	if len(resolutions) == 0 {
		return ResolutionStats{}
	}

	stats := ResolutionStats{
		TotalResolutions: len(resolutions),
	}

	var totalConfidence float64
	minConfidence := 1.0
	maxConfidence := 0.0

	for _, resolution := range resolutions {
		totalConfidence += resolution.Confidence

		if resolution.Confidence < minConfidence {
			minConfidence = resolution.Confidence
		}

		if resolution.Confidence > maxConfidence {
			maxConfidence = resolution.Confidence
		}

		if resolution.Confidence >= 0.8 {
			stats.HighConfidenceCount++
		} else if resolution.Confidence >= 0.6 {
			stats.MediumConfidenceCount++
		} else {
			stats.LowConfidenceCount++
		}

		if resolution.Reasoning != "" {
			stats.WithReasoningCount++
		}
	}

	stats.AverageConfidence = totalConfidence / float64(len(resolutions))
	stats.MinConfidence = minConfidence
	stats.MaxConfidence = maxConfidence

	return stats
}

// ResolutionStats contains statistics about conflict resolutions
type ResolutionStats struct {
	TotalResolutions      int     `json:"total_resolutions"`
	HighConfidenceCount   int     `json:"high_confidence_count"`   // >= 0.8
	MediumConfidenceCount int     `json:"medium_confidence_count"` // 0.6-0.8
	LowConfidenceCount    int     `json:"low_confidence_count"`    // < 0.6
	WithReasoningCount    int     `json:"with_reasoning_count"`
	AverageConfidence     float64 `json:"average_confidence"`
	MinConfidence         float64 `json:"min_confidence"`
	MaxConfidence         float64 `json:"max_confidence"`
}

// ResolutionResult represents the result of applying resolutions to files
type ResolutionResult struct {
	Success       bool                `json:"success"`
	AppliedCount  int                 `json:"applied_count"`
	FailedCount   int                 `json:"failed_count"`
	ModifiedFiles []string            `json:"modified_files"`
	FailedFiles   []ResolutionFailure `json:"failed_files,omitempty"`
	Errors        []string            `json:"errors,omitempty"`
	Stats         ResolutionStats     `json:"stats"`
}

// ResolutionFailure represents a failed resolution application
type ResolutionFailure struct {
	FilePath     string `json:"file_path"`
	ErrorMessage string `json:"error_message"`
}

// ApplyResolutions applies multiple resolutions to their respective files
func ApplyResolutions(repoPath string, resolutions []ConflictResolution) (*ResolutionResult, error) {
	result := &ResolutionResult{
		ModifiedFiles: make([]string, 0),
		FailedFiles:   make([]ResolutionFailure, 0),
		Errors:        make([]string, 0),
		Stats:         CalculateResolutionStats(resolutions),
	}

	// Group resolutions by file path
	fileResolutions := make(map[string][]ConflictResolution)
	for _, resolution := range resolutions {
		fileResolutions[resolution.FilePath] = append(fileResolutions[resolution.FilePath], resolution)
	}

	// Apply resolutions to each file
	for filePath, fileResolutions := range fileResolutions {
		fullPath := fmt.Sprintf("%s/%s", repoPath, filePath)

		// Read original file content
		content, err := os.ReadFile(fullPath)
		if err != nil {
			result.FailedFiles = append(result.FailedFiles, ResolutionFailure{
				FilePath:     filePath,
				ErrorMessage: fmt.Sprintf("failed to read file: %v", err),
			})
			result.FailedCount++
			continue
		}

		// Apply resolutions to the file
		modifiedContent, err := ApplyMultipleResolutions(string(content), fileResolutions)
		if err != nil {
			result.FailedFiles = append(result.FailedFiles, ResolutionFailure{
				FilePath:     filePath,
				ErrorMessage: fmt.Sprintf("failed to apply resolutions: %v", err),
			})
			result.FailedCount++
			continue
		}

		// Write modified content back to file
		err = os.WriteFile(fullPath, []byte(modifiedContent), 0644)
		if err != nil {
			result.FailedFiles = append(result.FailedFiles, ResolutionFailure{
				FilePath:     filePath,
				ErrorMessage: fmt.Sprintf("failed to write file: %v", err),
			})
			result.FailedCount++
			continue
		}

		result.ModifiedFiles = append(result.ModifiedFiles, filePath)
		result.AppliedCount++
	}

	result.Success = result.FailedCount == 0
	return result, nil
}

// CreateBackup creates a backup of a file before modification
func CreateBackup(repoPath, filePath string) error {
	fullPath := fmt.Sprintf("%s/%s", repoPath, filePath)
	backupPath := fullPath + ".backup"

	// Read original file
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return fmt.Errorf("failed to read original file: %w", err)
	}

	// Write backup file
	err = os.WriteFile(backupPath, content, 0644)
	if err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	return nil
}
