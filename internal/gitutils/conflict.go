package gitutils

import (
	"bufio"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// ConflictStatus represents the status of a conflicted file
type ConflictStatus struct {
	FilePath string
	Status   string // "UU" for both modified, "AA" for both added, etc.
}

// ConflictHunk represents a single conflict region in a file
type ConflictHunk struct {
	StartLine   int      `json:"start_line"`
	EndLine     int      `json:"end_line"`
	OursLines   []string `json:"ours_lines"`
	TheirsLines []string `json:"theirs_lines"`
	BaseLines   []string `json:"base_lines,omitempty"` // For diff3 style conflicts
}

// ConflictFile represents a file with merge conflicts
type ConflictFile struct {
	Path    string         `json:"path"`
	Hunks   []ConflictHunk `json:"hunks"`
	Context []string       `json:"context,omitempty"` // Surrounding lines for AI context
}

// ConflictReport represents the overall conflict detection report
type ConflictReport struct {
	ConflictedFiles []ConflictFile `json:"conflicted_files"`
	TotalConflicts  int            `json:"total_conflicts"`
	RepoPath        string         `json:"repo_path"`
}

// DetectConflicts identifies files with merge conflicts from git status
func DetectConflicts(repoPath string) ([]ConflictStatus, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoPath

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run git status: %w", err)
	}

	var conflicts []ConflictStatus
	scanner := bufio.NewScanner(strings.NewReader(string(output)))

	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 3 {
			continue
		}

		// Check for conflict markers in git status
		// UU = both modified, AA = both added, DD = both deleted
		// AU = added by us, UA = added by them, DU = deleted by us, UD = deleted by them
		status := line[:2]
		if isConflictStatus(status) {
			filePath := strings.TrimSpace(line[3:])
			conflicts = append(conflicts, ConflictStatus{
				FilePath: filePath,
				Status:   status,
			})
		}
	}

	return conflicts, scanner.Err()
}

// isConflictStatus checks if the git status indicates a merge conflict
func isConflictStatus(status string) bool {
	conflictStatuses := []string{"UU", "AA", "DD", "AU", "UA", "DU", "UD"}
	for _, cs := range conflictStatuses {
		if status == cs {
			return true
		}
	}
	return false
}

// ParseConflictHunks extracts conflict hunks from a file's content
func ParseConflictHunks(filePath, repoPath string) ([]ConflictHunk, error) {
	cmd := exec.Command("git", "show", ":"+filePath)
	cmd.Dir = repoPath

	// If the file doesn't exist in git, read from filesystem
	content, err := cmd.Output()
	if err != nil {
		// Try reading from filesystem
		cmd = exec.Command("cat", filePath)
		cmd.Dir = repoPath
		content, err = cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
		}
	}

	return parseConflictMarkers(string(content))
}

// parseConflictMarkers parses conflict markers from file content
func parseConflictMarkers(content string) ([]ConflictHunk, error) {
	lines := strings.Split(content, "\n")
	var hunks []ConflictHunk

	// Regex patterns for conflict markers
	startMarker := regexp.MustCompile(`^<{7}\s*(.*)$`) // <<<<<<< HEAD or <<<<<<< branch
	middleMarker := regexp.MustCompile(`^={7}$`)       // =======
	baseMarker := regexp.MustCompile(`^\|{7}\s*(.*)$`) // ||||||| base (diff3 style)
	endMarker := regexp.MustCompile(`^>{7}\s*(.*)$`)   // >>>>>>> branch

	i := 0
	for i < len(lines) {
		line := lines[i]

		// Look for start of conflict
		if startMarker.MatchString(line) {
			hunk := ConflictHunk{
				StartLine: i + 1, // 1-based line numbers
			}

			i++ // Move past start marker

			// Collect "ours" lines until we hit middle or base marker
			for i < len(lines) && !middleMarker.MatchString(lines[i]) && !baseMarker.MatchString(lines[i]) {
				hunk.OursLines = append(hunk.OursLines, lines[i])
				i++
			}

			// Check if we have a base section (diff3 style)
			if i < len(lines) && baseMarker.MatchString(lines[i]) {
				i++ // Move past base marker
				// Collect base lines until middle marker
				for i < len(lines) && !middleMarker.MatchString(lines[i]) {
					hunk.BaseLines = append(hunk.BaseLines, lines[i])
					i++
				}
			}

			// Should be at middle marker now
			if i < len(lines) && middleMarker.MatchString(lines[i]) {
				i++ // Move past middle marker

				// Collect "theirs" lines until end marker
				for i < len(lines) && !endMarker.MatchString(lines[i]) {
					hunk.TheirsLines = append(hunk.TheirsLines, lines[i])
					i++
				}

				// Should be at end marker
				if i < len(lines) && endMarker.MatchString(lines[i]) {
					hunk.EndLine = i + 1 // 1-based line numbers
					hunks = append(hunks, hunk)
				}
			}
		}
		i++
	}

	return hunks, nil
}

// ExtractFileContext extracts surrounding context lines for AI processing
func ExtractFileContext(filePath, repoPath string, contextLines int) ([]string, error) {
	cmd := exec.Command("cat", filePath)
	cmd.Dir = repoPath

	content, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	lines := strings.Split(string(content), "\n")

	// For now, return all lines as context
	// In a more sophisticated implementation, we could extract only
	// relevant context around conflict hunks
	return lines, nil
}

// IsInMergeState checks if the repository is currently in a merge state
func IsInMergeState(repoPath string) (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain=v1")
	cmd.Dir = repoPath

	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check git status: %w", err)
	}

	// Check for any conflict indicators
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if len(line) >= 2 {
			status := line[:2]
			if isConflictStatus(status) {
				return true, nil
			}
		}
	}

	return false, nil
}

// GetConflictReport generates a comprehensive conflict report
func GetConflictReport(repoPath string) (*ConflictReport, error) {
	conflicts, err := DetectConflicts(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to detect conflicts: %w", err)
	}

	report := &ConflictReport{
		RepoPath:       repoPath,
		TotalConflicts: len(conflicts),
	}

	for _, conflict := range conflicts {
		hunks, err := ParseConflictHunks(conflict.FilePath, repoPath)
		if err != nil {
			// Log error but continue with other files
			continue
		}

		context, err := ExtractFileContext(conflict.FilePath, repoPath, 5)
		if err != nil {
			// Continue without context if we can't read the file
			context = nil
		}

		conflictFile := ConflictFile{
			Path:    conflict.FilePath,
			Hunks:   hunks,
			Context: context,
		}

		report.ConflictedFiles = append(report.ConflictedFiles, conflictFile)
	}

	return report, nil
}
