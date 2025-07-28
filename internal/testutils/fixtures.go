// Package testutils provides shared testing utilities and fixtures
package testutils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// TestGoConflictContent returns a Go file with conflict markers
func TestGoConflictContent() string {
	return `package main

import (
<<<<<<< HEAD
	"fmt"
=======
	"fmt"
	"errors"
>>>>>>> feature-branch
)

func main() {
<<<<<<< HEAD
	fmt.Println("Hello, World!")
=======
	if err := greet("World"); err != nil {
		fmt.Printf("Error: %v\n", err)
	}
>>>>>>> feature-branch
}

<<<<<<< HEAD
func greet(name string) {
	fmt.Printf("Hello, %s!\n", name)
}
=======
func greet(name string) error {
	if name == "" {
		return errors.New("name cannot be empty")
	}
	fmt.Printf("Hello, %s!\n", name)
	return nil
}
>>>>>>> feature-branch
`
}

// TestJavaScriptConflictContent returns a JavaScript file with conflict markers
func TestJavaScriptConflictContent() string {
	return `const express = require('express');
const app = express();

<<<<<<< HEAD
app.get('/', (req, res) => {
  res.send('Hello World!');
});
=======
app.get('/', (req, res) => {
  res.json({ message: 'Hello World!' });
});
>>>>>>> feature-branch

const PORT = process.env.PORT || 3000;
app.listen(PORT, () => {
<<<<<<< HEAD
  console.log('Server running on port ' + PORT);
=======
  console.log(` + "`" + `Server running on port ${PORT}` + "`" + `);
>>>>>>> feature-branch
});
`
}

// TestPythonConflictContent returns a Python file with conflict markers
func TestPythonConflictContent() string {
	return `#!/usr/bin/env python3

<<<<<<< HEAD
def greet(name):
    print(f"Hello, {name}!")
=======
def greet(name):
    if not name:
        raise ValueError("Name cannot be empty")
    print(f"Hello, {name}!")
>>>>>>> feature-branch

if __name__ == "__main__":
<<<<<<< HEAD
    greet("World")
=======
    try:
        greet("World")
    except ValueError as e:
        print(f"Error: {e}")
>>>>>>> feature-branch
`
}

// TestClaudeJSONResponse creates a mock Claude JSON response for conflict resolution
func TestClaudeJSONResponse() string {
	return `{
  "resolutions": [
    {
      "file_path": "main.go",
      "start_line": 10,
      "end_line": 15,
      "resolved_lines": [
        "func greet(name string) error {",
        "  if name == \"\" {",
        "    return fmt.Errorf(\"name cannot be empty\")",
        "  }",
        "  fmt.Printf(\"Hello, %s!\\n\", name)",
        "  return nil",
        "}"
      ],
      "confidence": 0.85,
      "reasoning": "Merged function signatures by preserving error handling while maintaining the greeting functionality",
      "go_specific_notes": "Added proper error handling following Go idioms"
    },
    {
      "file_path": "main.go",  
      "start_line": 25,
      "end_line": 30,
      "resolved_lines": [
        "import (",
        "  \"fmt\"",
        "  \"errors\"",
        ")"
      ],
      "confidence": 0.95,
      "reasoning": "Combined import statements into a grouped import block",
      "go_specific_notes": "Followed Go import grouping conventions"
    }
  ]
}`
}

// TestSecurityPaths provides paths for security testing
func TestSecurityPaths() []string {
	return []string{
		"../../../etc/passwd",
		"..\\..\\..\\windows\\system32",
		"/etc/shadow",
		"../../.ssh/id_rsa",
		"file;rm -rf /",
		"file`rm -rf /`",
		"file$(rm -rf /)",
		"file|cat /etc/passwd",
		"file&cat /etc/passwd",
		"normal/file/path.go",
	}
}

// TestMaliciousCommands provides malicious commands for security testing
func TestMaliciousCommands() []string {
	return []string{
		"git status; rm -rf /",
		"git status && rm -rf /",
		"git status | cat /etc/passwd",
		"git status `cat /etc/passwd`",
		"git status $(cat /etc/passwd)",
		"git status & cat /etc/passwd",
	}
}

// TestValidCommands provides valid commands for testing
func TestValidCommands() []string {
	return []string{
		"git status",
		"git diff --name-only",
		"go build ./...",
		"npm test",
		"python -m pytest",
		"cargo build",
		"make test",
	}
}

// TestFileExtensions provides file extensions for testing
func TestFileExtensions() map[string]string {
	return map[string]string{
		"main.go":       "go",
		"app.js":        "javascript",
		"component.tsx": "typescript",
		"script.py":     "python",
		"main.rs":       "rust",
		"App.java":      "java",
		"style.css":     "css",
		"config.json":   "json",
		"docker.yaml":   "yaml",
		"README.md":     "markdown",
		"Makefile":      "makefile",
		"Dockerfile":    "dockerfile",
	}
}

// TestBinaryFiles provides binary file extensions for exclusion testing
func TestBinaryFiles() []string {
	return []string{
		"image.jpg",
		"document.pdf",
		"archive.zip",
		"executable.exe",
		"library.dll",
		"music.mp3",
		"video.mp4",
		"font.ttf",
	}
}

// TestExcludedPaths provides paths that should be excluded from processing
func TestExcludedPaths() []string {
	return []string{
		"node_modules/package/index.js",
		".git/config",
		"vendor/github.com/lib/file.go",
		"target/debug/binary",
		"build/output.js",
		"dist/bundle.min.js",
		".DS_Store",
		"package-lock.json",
		"go.sum",
	}
}

// GenerateTestFiles creates a map of test files with their content
func GenerateTestFiles(count int) map[string]string {
	files := make(map[string]string)

	for i := 0; i < count; i++ {
		filename := fmt.Sprintf("test_file_%d.go", i)
		content := fmt.Sprintf(`package main

import "fmt"

func test%d() {
	fmt.Println("This is test function %d")
}

func main() {
	test%d()
}
`, i, i, i)
		files[filename] = content
	}

	return files
}

// GenerateConflictFiles creates a map of files with conflict markers
func GenerateConflictFiles(count int) map[string]string {
	files := make(map[string]string)

	for i := 0; i < count; i++ {
		filename := fmt.Sprintf("conflict_%d.go", i)
		content := fmt.Sprintf(`package main

import "fmt"

<<<<<<< HEAD
func method%d() {
	fmt.Println("Original implementation %d")
}
=======
func method%d() error {
	fmt.Printf("New implementation %d with error handling\n")
	return nil
}
>>>>>>> feature-branch

func main() {
	method%d()
}
`, i, i, i, i, i)
		files[filename] = content
	}

	return files
}

// GetCurrentTime returns current timestamp for testing
func GetCurrentTime() time.Time {
	return time.Now()
}

// CreateTestError creates a test error with a given message
func CreateTestError(message string) error {
	return fmt.Errorf("test error: %s", message)
}

// SetupTestGitRepository initializes a Git repository in the given directory with proper configuration
func SetupTestGitRepository(repoPath string) error {
	// Initialize git repository
	if err := runGitCommand(repoPath, "init"); err != nil {
		return fmt.Errorf("failed to initialize git repo: %w", err)
	}

	// Configure git user (required for commits)
	if err := runGitCommand(repoPath, "config", "user.name", "Test User"); err != nil {
		return fmt.Errorf("failed to configure git user.name: %w", err)
	}

	if err := runGitCommand(repoPath, "config", "user.email", "test@example.com"); err != nil {
		return fmt.Errorf("failed to configure git user.email: %w", err)
	}

	// Set default branch to main
	if err := runGitCommand(repoPath, "config", "init.defaultBranch", "main"); err != nil {
		// This might fail on older git versions, continue anyway
	}

	return nil
}

// SetupGitRepositoryWithConflicts creates a git repository with actual merge conflicts
func SetupGitRepositoryWithConflicts(repoPath string, files map[string]string) error {
	// Initialize repository
	if err := SetupTestGitRepository(repoPath); err != nil {
		return err
	}

	// Create initial files and commit
	for filename, content := range files {
		// Extract the base content (remove conflict markers for initial commit)
		baseContent := extractBaseContent(content)
		fullPath := filepath.Join(repoPath, filename)
		
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}

		if err := os.WriteFile(fullPath, []byte(baseContent), 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", filename, err)
		}
	}

	// Add and commit initial files
	if err := runGitCommand(repoPath, "add", "."); err != nil {
		return fmt.Errorf("failed to add files: %w", err)
	}

	if err := runGitCommand(repoPath, "commit", "-m", "Initial commit"); err != nil {
		return fmt.Errorf("failed to create initial commit: %w", err)
	}

	// Create a feature branch
	if err := runGitCommand(repoPath, "checkout", "-b", "feature-branch"); err != nil {
		return fmt.Errorf("failed to create feature branch: %w", err)
	}

	// Modify files for feature branch
	for filename, content := range files {
		featureContent := extractFeatureContent(content)
		fullPath := filepath.Join(repoPath, filename)
		
		if err := os.WriteFile(fullPath, []byte(featureContent), 0644); err != nil {
			return fmt.Errorf("failed to write feature branch file %s: %w", filename, err)
		}
	}

	// Commit feature branch changes
	if err := runGitCommand(repoPath, "add", "."); err != nil {
		return fmt.Errorf("failed to add feature branch files: %w", err)
	}

	if err := runGitCommand(repoPath, "commit", "-m", "Feature branch changes"); err != nil {
		return fmt.Errorf("failed to commit feature branch: %w", err)
	}

	// Switch back to main and modify files differently
	if err := runGitCommand(repoPath, "checkout", "main"); err != nil {
		return fmt.Errorf("failed to checkout main: %w", err)
	}

	// Modify files for main branch
	for filename, content := range files {
		mainContent := extractMainContent(content)
		fullPath := filepath.Join(repoPath, filename)
		
		if err := os.WriteFile(fullPath, []byte(mainContent), 0644); err != nil {
			return fmt.Errorf("failed to write main branch file %s: %w", filename, err)
		}
	}

	// Commit main branch changes
	if err := runGitCommand(repoPath, "add", "."); err != nil {
		return fmt.Errorf("failed to add main branch files: %w", err)
	}

	if err := runGitCommand(repoPath, "commit", "-m", "Main branch changes"); err != nil {
		return fmt.Errorf("failed to commit main branch: %w", err)
	}

	// Attempt to merge feature-branch, which should create conflicts
	err := runGitCommand(repoPath, "merge", "feature-branch")
	if err != nil {
		// Merge conflicts are expected, so we check if we're in a merge state
		if isInMergeState(repoPath) {
			// Perfect! We have merge conflicts
			// Manually create the conflict markers in files
			for filename, content := range files {
				fullPath := filepath.Join(repoPath, filename)
				if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
					return fmt.Errorf("failed to write conflicted file %s: %w", filename, err)
				}
			}
			return nil
		}
		return fmt.Errorf("failed to create merge conflicts: %w", err)
	}

	return fmt.Errorf("expected merge conflicts but merge succeeded")
}

// runGitCommand executes a git command in the specified directory
func runGitCommand(repoPath string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %v failed: %w (output: %s)", args, err, string(output))
	}
	return nil
}

// isInMergeState checks if the repository is in a merge state
func isInMergeState(repoPath string) bool {
	mergeHeadPath := filepath.Join(repoPath, ".git", "MERGE_HEAD")
	_, err := os.Stat(mergeHeadPath)
	return err == nil
}

// extractBaseContent removes conflict markers and returns the base content
func extractBaseContent(content string) string {
	lines := []string{}
	inConflict := false
	
	for _, line := range splitLines(content) {
		if isConflictStart(line) {
			inConflict = true
			continue
		}
		if isConflictEnd(line) {
			inConflict = false
			continue
		}
		if isConflictMiddle(line) {
			continue
		}
		if !inConflict {
			lines = append(lines, line)
		}
	}
	
	return joinLines(lines)
}

// extractMainContent extracts the "ours" (HEAD) side of conflicts
func extractMainContent(content string) string {
	lines := []string{}
	inConflict := false
	inOurSection := false
	
	for _, line := range splitLines(content) {
		if isConflictStart(line) {
			inConflict = true
			inOurSection = true
			continue
		}
		if isConflictMiddle(line) {
			inOurSection = false
			continue
		}
		if isConflictEnd(line) {
			inConflict = false
			inOurSection = false
			continue
		}
		
		if !inConflict || inOurSection {
			lines = append(lines, line)
		}
	}
	
	return joinLines(lines)
}

// extractFeatureContent extracts the "theirs" (feature-branch) side of conflicts
func extractFeatureContent(content string) string {
	lines := []string{}
	inConflict := false
	inTheirSection := false
	
	for _, line := range splitLines(content) {
		if isConflictStart(line) {
			inConflict = true
			continue
		}
		if isConflictMiddle(line) {
			inTheirSection = true
			continue
		}
		if isConflictEnd(line) {
			inConflict = false
			inTheirSection = false
			continue
		}
		
		if !inConflict || inTheirSection {
			lines = append(lines, line)
		}
	}
	
	return joinLines(lines)
}

// Helper functions for conflict marker detection
func isConflictStart(line string) bool {
	return len(line) >= 7 && line[:7] == "<<<<<<< "
}

func isConflictMiddle(line string) bool {
	return line == "======="
}

func isConflictEnd(line string) bool {
	return len(line) >= 7 && line[:7] == ">>>>>>> "
}

func splitLines(content string) []string {
	if content == "" {
		return []string{}
	}
	lines := []string{}
	current := ""
	
	for _, r := range content {
		if r == '\n' {
			lines = append(lines, current)
			current = ""
		} else {
			current += string(r)
		}
	}
	
	if current != "" {
		lines = append(lines, current)
	}
	
	return lines
}

func joinLines(lines []string) string {
	result := ""
	for i, line := range lines {
		if i > 0 {
			result += "\n"
		}
		result += line
	}
	if len(lines) > 0 {
		result += "\n"
	}
	return result
}

