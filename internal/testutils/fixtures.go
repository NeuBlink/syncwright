// Package testutils provides shared testing utilities and fixtures
package testutils

import (
	"fmt"
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
