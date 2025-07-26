package gitutils

import (
	"fmt"
	"os"
	"strings"
)

// ConflictResolution represents a resolved conflict hunk
type ConflictResolution struct {
	FilePath    string   `json:"file_path"`
	StartLine   int      `json:"start_line"`
	EndLine     int      `json:"end_line"`
	ResolvedLines []string `json:"resolved_lines"`
	Confidence  float64  `json:"confidence"`
	Reasoning   string   `json:"reasoning,omitempty"`
}

// ResolutionResult represents the result of applying resolutions
type ResolutionResult struct {
	Success       bool     `json:"success"`
	AppliedCount  int      `json:"applied_count"`
	FailedCount   int      `json:"failed_count"`
	Errors        []string `json:"errors,omitempty"`
	ModifiedFiles []string `json:"modified_files"`
}

// ApplyResolutions applies conflict resolutions to files
func ApplyResolutions(repoPath string, resolutions []ConflictResolution) (*ResolutionResult, error) {
	result := &ResolutionResult{
		Success:      true,
		ModifiedFiles: make([]string, 0),
	}
	
	// Group resolutions by file
	fileResolutions := make(map[string][]ConflictResolution)
	for _, resolution := range resolutions {
		fileResolutions[resolution.FilePath] = append(fileResolutions[resolution.FilePath], resolution)
	}
	
	// Apply resolutions file by file
	for filePath, fileRes := range fileResolutions {
		err := applyFileResolutions(repoPath, filePath, fileRes)
		if err != nil {
			result.Success = false
			result.FailedCount += len(fileRes)
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to apply resolutions to %s: %v", filePath, err))
		} else {
			result.AppliedCount += len(fileRes)
			result.ModifiedFiles = append(result.ModifiedFiles, filePath)
		}
	}
	
	return result, nil
}

// applyFileResolutions applies resolutions to a single file
func applyFileResolutions(repoPath, filePath string, resolutions []ConflictResolution) error {
	fullPath := fmt.Sprintf("%s/%s", repoPath, filePath)
	
	// Read current file content
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	
	lines := strings.Split(string(content), "\n")
	
	// Sort resolutions by start line (descending) to apply from bottom to top
	// This prevents line number shifting issues
	for i := 0; i < len(resolutions)-1; i++ {
		for j := i + 1; j < len(resolutions); j++ {
			if resolutions[i].StartLine < resolutions[j].StartLine {
				resolutions[i], resolutions[j] = resolutions[j], resolutions[i]
			}
		}
	}
	
	// Apply each resolution
	for _, resolution := range resolutions {
		lines, err = applyResolution(lines, resolution)
		if err != nil {
			return fmt.Errorf("failed to apply resolution at lines %d-%d: %w", 
				resolution.StartLine, resolution.EndLine, err)
		}
	}
	
	// Write back to file
	newContent := strings.Join(lines, "\n")
	err = os.WriteFile(fullPath, []byte(newContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	
	return nil
}

// applyResolution applies a single resolution to file lines
func applyResolution(lines []string, resolution ConflictResolution) ([]string, error) {
	// Convert to 0-based indexing
	startIdx := resolution.StartLine - 1
	endIdx := resolution.EndLine - 1
	
	// Validate indices
	if startIdx < 0 || endIdx >= len(lines) || startIdx > endIdx {
		return nil, fmt.Errorf("invalid line range: %d-%d (file has %d lines)", 
			resolution.StartLine, resolution.EndLine, len(lines))
	}
	
	// Verify this is actually a conflict region
	if !isConflictRegion(lines[startIdx:endIdx+1]) {
		return nil, fmt.Errorf("specified region does not contain conflict markers")
	}
	
	// Replace the conflict region with resolved lines
	result := make([]string, 0, len(lines)-((endIdx-startIdx)+1)+len(resolution.ResolvedLines))
	result = append(result, lines[:startIdx]...)
	result = append(result, resolution.ResolvedLines...)
	result = append(result, lines[endIdx+1:]...)
	
	return result, nil
}

// isConflictRegion checks if the given lines contain conflict markers
func isConflictRegion(lines []string) bool {
	hasStart := false
	hasMiddle := false
	hasEnd := false
	
	for _, line := range lines {
		if strings.HasPrefix(line, "<<<<<<<") {
			hasStart = true
		} else if strings.HasPrefix(line, "=======") {
			hasMiddle = true
		} else if strings.HasPrefix(line, ">>>>>>>") {
			hasEnd = true
		}
	}
	
	return hasStart && hasMiddle && hasEnd
}

// ValidateResolution performs basic validation on a resolution
func ValidateResolution(resolution ConflictResolution) error {
	if resolution.FilePath == "" {
		return fmt.Errorf("file path cannot be empty")
	}
	
	if resolution.StartLine <= 0 || resolution.EndLine <= 0 {
		return fmt.Errorf("line numbers must be positive")
	}
	
	if resolution.StartLine > resolution.EndLine {
		return fmt.Errorf("start line cannot be greater than end line")
	}
	
	if resolution.Confidence < 0 || resolution.Confidence > 1 {
		return fmt.Errorf("confidence must be between 0 and 1")
	}
	
	return nil
}

// ExtractConflictContent extracts the content of conflict markers
func ExtractConflictContent(filePath, repoPath string) (map[int]ConflictContent, error) {
	fullPath := fmt.Sprintf("%s/%s", repoPath, filePath)
	
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	
	lines := strings.Split(string(content), "\n")
	conflicts := make(map[int]ConflictContent)
	
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		
		// Look for conflict start marker
		if strings.HasPrefix(line, "<<<<<<<") {
			conflict := ConflictContent{
				StartLine: i + 1, // 1-based
				StartMarker: line,
			}
			
			i++ // Move past start marker
			
			// Collect "ours" lines
			for i < len(lines) && !strings.HasPrefix(lines[i], "=======") && !strings.HasPrefix(lines[i], "|||||||") {
				conflict.OursLines = append(conflict.OursLines, lines[i])
				i++
			}
			
			// Check for base marker (diff3 style)
			if i < len(lines) && strings.HasPrefix(lines[i], "|||||||") {
				conflict.BaseMarker = lines[i]
				i++ // Move past base marker
				
				// Collect base lines
				for i < len(lines) && !strings.HasPrefix(lines[i], "=======") {
					conflict.BaseLines = append(conflict.BaseLines, lines[i])
					i++
				}
			}
			
			// Should be at middle marker
			if i < len(lines) && strings.HasPrefix(lines[i], "=======") {
				conflict.MiddleMarker = lines[i]
				i++ // Move past middle marker
				
				// Collect "theirs" lines
				for i < len(lines) && !strings.HasPrefix(lines[i], ">>>>>>>") {
					conflict.TheirsLines = append(conflict.TheirsLines, lines[i])
					i++
				}
				
				// Should be at end marker
				if i < len(lines) && strings.HasPrefix(lines[i], ">>>>>>>") {
					conflict.EndMarker = lines[i]
					conflict.EndLine = i + 1 // 1-based
					conflicts[conflict.StartLine] = conflict
				}
			}
		}
	}
	
	return conflicts, nil
}

// ConflictContent represents the detailed content of a conflict
type ConflictContent struct {
	StartLine     int      `json:"start_line"`
	EndLine       int      `json:"end_line"`
	StartMarker   string   `json:"start_marker"`
	MiddleMarker  string   `json:"middle_marker"`
	EndMarker     string   `json:"end_marker"`
	BaseMarker    string   `json:"base_marker,omitempty"`
	OursLines     []string `json:"ours_lines"`
	TheirsLines   []string `json:"theirs_lines"`
	BaseLines     []string `json:"base_lines,omitempty"`
}

// CreateBackup creates a backup of the file before applying resolutions
func CreateBackup(repoPath, filePath string) error {
	fullPath := fmt.Sprintf("%s/%s", repoPath, filePath)
	backupPath := fullPath + ".backup"
	
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return fmt.Errorf("failed to read file for backup: %w", err)
	}
	
	err = os.WriteFile(backupPath, content, 0644)
	if err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}
	
	return nil
}

// RestoreBackup restores a file from its backup
func RestoreBackup(repoPath, filePath string) error {
	fullPath := fmt.Sprintf("%s/%s", repoPath, filePath)
	backupPath := fullPath + ".backup"
	
	content, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup file: %w", err)
	}
	
	err = os.WriteFile(fullPath, content, 0644)
	if err != nil {
		return fmt.Errorf("failed to restore from backup: %w", err)
	}
	
	return nil
}

// CleanupBackups removes backup files
func CleanupBackups(repoPath string, filePaths []string) error {
	for _, filePath := range filePaths {
		backupPath := fmt.Sprintf("%s/%s.backup", repoPath, filePath)
		if err := os.Remove(backupPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove backup %s: %w", backupPath, err)
		}
	}
	return nil
}