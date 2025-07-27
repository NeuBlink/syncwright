// Package gitutils provides Git repository operations for Syncwright
package gitutils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// IsGitRepository checks if the current directory is a Git repository
func IsGitRepository() (bool, error) {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	err := cmd.Run()
	return err == nil, nil
}

// IsGitRepositoryPath checks if the specified path is a Git repository
func IsGitRepositoryPath(repoPath string) bool {
	// Check if .git directory or file exists
	gitPath := filepath.Join(repoPath, ".git")
	if _, err := os.Stat(gitPath); err == nil {
		return true
	}
	return false
}

// GetConflictedFiles returns a list of files with merge conflicts
func GetConflictedFiles() ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only", "--diff-filter=U")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get conflicted files: %w", err)
	}

	files := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(files) == 1 && files[0] == "" {
		return []string{}, nil
	}

	return files, nil
}

// CommitChanges creates a commit with the provided message
func CommitChanges(message string) error {
	// Add all changes
	addCmd := exec.Command("git", "add", ".")
	if err := addCmd.Run(); err != nil {
		return fmt.Errorf("failed to add changes: %w", err)
	}

	// Create commit
	commitCmd := exec.Command("git", "commit", "-m", message)
	if err := commitCmd.Run(); err != nil {
		return fmt.Errorf("failed to commit changes: %w", err)
	}

	return nil
}

// GetRecentlyModifiedFiles returns files modified within the specified number of days
func GetRecentlyModifiedFiles(repoPath string, days int) ([]string, error) {
	// Use git log to find files modified in the last N days
	since := time.Now().AddDate(0, 0, -days).Format("2006-01-02")

	// Validate the since parameter format to prevent command injection
	// Expected format: YYYY-MM-DD
	if matched, _ := regexp.MatchString(`^\d{4}-\d{2}-\d{2}$`, since); !matched {
		return nil, fmt.Errorf("invalid date format: %s", since)
	}

	cmd := exec.Command("git", "log", "--name-only", "--pretty=format:", "--since="+since)
	cmd.Dir = repoPath

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get recently modified files: %w", err)
	}

	// Parse the output and deduplicate files
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	fileMap := make(map[string]bool)
	var files []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !fileMap[line] {
			// Check if file still exists
			fullPath := filepath.Join(repoPath, line)
			if _, err := os.Stat(fullPath); err == nil {
				files = append(files, line)
				fileMap[line] = true
			}
		}
	}

	return files, nil
}

// GetAllTrackedFiles returns all files tracked by git
func GetAllTrackedFiles(repoPath string) ([]string, error) {
	cmd := exec.Command("git", "ls-files")
	cmd.Dir = repoPath

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get tracked files: %w", err)
	}

	files := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(files) == 1 && files[0] == "" {
		return []string{}, nil
	}

	// Filter out files that don't exist
	var existingFiles []string
	for _, file := range files {
		fullPath := filepath.Join(repoPath, file)
		if _, err := os.Stat(fullPath); err == nil {
			existingFiles = append(existingFiles, file)
		}
	}

	return existingFiles, nil
}
