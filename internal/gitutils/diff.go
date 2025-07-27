package gitutils

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// DiffHunk represents a single hunk from git diff output
type DiffHunk struct {
	OldStart int      `json:"old_start"`
	OldLines int      `json:"old_lines"`
	NewStart int      `json:"new_start"`
	NewLines int      `json:"new_lines"`
	Lines    []string `json:"lines"`
	Header   string   `json:"header"`
}

// DiffFile represents a file in git diff output
type DiffFile struct {
	OldPath string     `json:"old_path"`
	NewPath string     `json:"new_path"`
	Hunks   []DiffHunk `json:"hunks"`
}

// GetConflictDiff retrieves the git diff for conflicted files
func GetConflictDiff(repoPath string, filePaths []string) ([]DiffFile, error) {
	if len(filePaths) == 0 {
		return nil, nil
	}

	args := append([]string{"diff", "--no-index", "--no-prefix"}, filePaths...)
	cmd := exec.Command("git", args...)
	cmd.Dir = repoPath

	output, err := cmd.Output()
	if err != nil {
		// git diff returns non-zero exit code when there are differences
		// This is expected, so we continue processing the output
	}

	return parseDiffOutput(string(output))
}

// GetMergeBaseDiff gets the diff between merge base and current state
func GetMergeBaseDiff(repoPath, filePath string) (*DiffFile, error) {
	// Get merge base
	cmd := exec.Command("git", "merge-base", "HEAD", "MERGE_HEAD")
	cmd.Dir = repoPath

	baseOutput, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get merge base: %w", err)
	}

	base := strings.TrimSpace(string(baseOutput))

	// Get diff from base to working tree
	cmd = exec.Command("git", "diff", "--no-prefix", base, "--", filePath)
	cmd.Dir = repoPath

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get diff: %w", err)
	}

	files, err := parseDiffOutput(string(output))
	if err != nil {
		return nil, err
	}

	if len(files) > 0 {
		return &files[0], nil
	}

	return nil, nil
}

// parseDiffOutput parses the output of git diff command
func parseDiffOutput(output string) ([]DiffFile, error) {
	lines := strings.Split(output, "\n")
	var files []DiffFile
	var currentFile *DiffFile
	var currentHunk *DiffHunk

	parser := &diffParser{
		fileHeaderRegex: regexp.MustCompile(`^diff --git a/(.+) b/(.+)$`),
		oldFileRegex:    regexp.MustCompile(`^--- (.+)$`),
		newFileRegex:    regexp.MustCompile(`^\+\+\+ (.+)$`),
		hunkHeaderRegex: regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@(.*)$`),
	}

	for _, line := range lines {
		var err error
		currentFile, currentHunk, err = parser.parseLine(line, currentFile, currentHunk, &files)
		if err != nil {
			return nil, err
		}
	}

	// Save last hunk and file
	finalizeCurrentItems(currentFile, currentHunk, &files)

	return files, nil
}

// diffParser contains regex patterns and logic for parsing diff output
type diffParser struct {
	fileHeaderRegex *regexp.Regexp
	oldFileRegex    *regexp.Regexp
	newFileRegex    *regexp.Regexp
	hunkHeaderRegex *regexp.Regexp
}

// parseLine processes a single line of diff output
func (p *diffParser) parseLine(line string, currentFile *DiffFile, currentHunk *DiffHunk, files *[]DiffFile) (*DiffFile, *DiffHunk, error) {
	// Check for file header
	if matches := p.fileHeaderRegex.FindStringSubmatch(line); matches != nil {
		return p.handleFileHeader(matches, currentFile, files), nil, nil
	}

	// Check for old file path
	if matches := p.oldFileRegex.FindStringSubmatch(line); matches != nil {
		p.handleOldFilePath(matches, currentFile)
		return currentFile, currentHunk, nil
	}

	// Check for new file path
	if matches := p.newFileRegex.FindStringSubmatch(line); matches != nil {
		p.handleNewFilePath(matches, currentFile)
		return currentFile, currentHunk, nil
	}

	// Check for hunk header
	if matches := p.hunkHeaderRegex.FindStringSubmatch(line); matches != nil {
		newHunk, err := p.handleHunkHeader(matches, currentFile, currentHunk)
		return currentFile, newHunk, err
	}

	// Add line to current hunk
	if currentHunk != nil {
		currentHunk.Lines = append(currentHunk.Lines, line)
	}

	return currentFile, currentHunk, nil
}

// handleFileHeader processes a file header line
func (p *diffParser) handleFileHeader(matches []string, currentFile *DiffFile, files *[]DiffFile) *DiffFile {
	// Save previous file if exists
	if currentFile != nil {
		*files = append(*files, *currentFile)
	}

	// Start new file
	return &DiffFile{
		OldPath: matches[1],
		NewPath: matches[2],
	}
}

// handleOldFilePath processes an old file path line
func (p *diffParser) handleOldFilePath(matches []string, currentFile *DiffFile) {
	if currentFile != nil {
		currentFile.OldPath = matches[1]
	}
}

// handleNewFilePath processes a new file path line
func (p *diffParser) handleNewFilePath(matches []string, currentFile *DiffFile) {
	if currentFile != nil {
		currentFile.NewPath = matches[1]
	}
}

// handleHunkHeader processes a hunk header line
func (p *diffParser) handleHunkHeader(matches []string, currentFile *DiffFile, currentHunk *DiffHunk) (*DiffHunk, error) {
	// Save previous hunk if exists
	if currentHunk != nil && currentFile != nil {
		currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
	}

	// Parse hunk header numbers
	oldStart, oldLines, newStart, newLines, err := parseHunkNumbers(matches)
	if err != nil {
		return nil, err
	}

	return &DiffHunk{
		OldStart: oldStart,
		OldLines: oldLines,
		NewStart: newStart,
		NewLines: newLines,
		Header:   strings.TrimSpace(matches[5]),
	}, nil
}

// parseHunkNumbers extracts and validates numbers from hunk header matches
func parseHunkNumbers(matches []string) (oldStart, oldLines, newStart, newLines int, err error) {
	oldStart, err = strconv.Atoi(matches[1])
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("invalid old start line number: %w", err)
	}

	oldLines = 1
	if matches[2] != "" {
		if ol, parseErr := strconv.Atoi(matches[2]); parseErr == nil {
			oldLines = ol
		}
	}

	newStart, err = strconv.Atoi(matches[3])
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("invalid new start line number: %w", err)
	}

	newLines = 1
	if matches[4] != "" {
		if nl, parseErr := strconv.Atoi(matches[4]); parseErr == nil {
			newLines = nl
		}
	}

	return oldStart, oldLines, newStart, newLines, nil
}

// finalizeCurrentItems saves the last hunk and file
func finalizeCurrentItems(currentFile *DiffFile, currentHunk *DiffHunk, files *[]DiffFile) {
	if currentHunk != nil && currentFile != nil {
		currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
	}
	if currentFile != nil {
		*files = append(*files, *currentFile)
	}
}

// GetFileAtRevision retrieves file content at a specific git revision
func GetFileAtRevision(repoPath, filePath, revision string) ([]string, error) {
	// Validate revision parameter to prevent command injection
	if matched, _ := regexp.MatchString(`^[a-zA-Z0-9_\-]+$`, revision); !matched {
		return nil, fmt.Errorf("invalid revision format: %s", revision)
	}

	// Validate and sanitize file path to prevent command injection
	cleanPath := filepath.Clean(filePath)
	if strings.Contains(cleanPath, "..") || strings.HasPrefix(cleanPath, "/") || strings.Contains(cleanPath, ";") || strings.Contains(cleanPath, "&") || strings.Contains(cleanPath, "|") || strings.Contains(cleanPath, "`") || strings.Contains(cleanPath, "$") {
		return nil, fmt.Errorf("invalid file path: %s", filePath)
	}

	// Additional validation to ensure path contains only safe characters
	if matched, _ := regexp.MatchString(`^[a-zA-Z0-9._/\-]+$`, cleanPath); !matched {
		return nil, fmt.Errorf("file path contains unsafe characters: %s", filePath)
	}

	cmd := exec.Command("git", "show", revision+":"+cleanPath)
	cmd.Dir = repoPath

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get file at revision %s: %w", revision, err)
	}

	return strings.Split(string(output), "\n"), nil
}

// GetConflictVersions retrieves all versions of a conflicted file
func GetConflictVersions(repoPath, filePath string) (ours, theirs, base []string, err error) {
	// Get our version (HEAD)
	ours, err = GetFileAtRevision(repoPath, filePath, "HEAD")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get our version: %w", err)
	}

	// Get their version (MERGE_HEAD)
	theirs, err = GetFileAtRevision(repoPath, filePath, "MERGE_HEAD")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get their version: %w", err)
	}

	// Get base version (merge base)
	cmd := exec.Command("git", "merge-base", "HEAD", "MERGE_HEAD")
	cmd.Dir = repoPath

	baseOutput, err := cmd.Output()
	if err != nil {
		// Base might not exist for some conflicts, that's ok
		return ours, theirs, nil, nil
	}

	baseRevision := strings.TrimSpace(string(baseOutput))
	base, err = GetFileAtRevision(repoPath, filePath, baseRevision)
	if err != nil {
		// Base might not exist for this file, that's ok
		return ours, theirs, nil, nil
	}

	return ours, theirs, base, nil
}

// IsTextFile checks if a file is likely to be a text file (not binary)
func IsTextFile(filePath string) bool {
	// Simple heuristic - check file extension
	textExtensions := map[string]bool{
		".go":         true,
		".js":         true,
		".ts":         true,
		".py":         true,
		".java":       true,
		".c":          true,
		".cpp":        true,
		".h":          true,
		".hpp":        true,
		".css":        true,
		".html":       true,
		".xml":        true,
		".json":       true,
		".yaml":       true,
		".yml":        true,
		".toml":       true,
		".md":         true,
		".txt":        true,
		".sh":         true,
		".bat":        true,
		".ps1":        true,
		".sql":        true,
		".php":        true,
		".rb":         true,
		".rs":         true,
		".kt":         true,
		".swift":      true,
		".dart":       true,
		".scala":      true,
		".clj":        true,
		".hs":         true,
		".ml":         true,
		".fs":         true,
		".r":          true,
		".m":          true,
		".pl":         true,
		".lua":        true,
		".vim":        true,
		".cfg":        true,
		".conf":       true,
		".ini":        true,
		".properties": true,
	}

	// Get file extension
	lastDot := strings.LastIndex(filePath, ".")
	if lastDot == -1 {
		// No extension - check for common text files without extensions
		fileName := filePath
		if lastSlash := strings.LastIndex(filePath, "/"); lastSlash != -1 {
			fileName = filePath[lastSlash+1:]
		}

		textFiles := map[string]bool{
			"Makefile":       true,
			"Dockerfile":     true,
			"README":         true,
			"LICENSE":        true,
			"CHANGELOG":      true,
			"CONTRIBUTING":   true,
			"AUTHORS":        true,
			".gitignore":     true,
			".gitattributes": true,
			".editorconfig":  true,
		}

		return textFiles[fileName]
	}

	ext := filePath[lastDot:]
	return textExtensions[ext]
}
