// Package testutils provides shared testing utilities and mock implementations
package testutils

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MockClaudeClient provides a mock implementation of Claude client for testing
type MockClaudeClient struct {
	IsAvailableFunc               func() bool
	ExecuteCommandFunc            func(ctx context.Context, command interface{}) (interface{}, error)
	ExecuteWithRetryFunc          func(ctx context.Context, command interface{}, maxRetries int) (interface{}, error)
	ExecuteConflictResolutionFunc func(ctx context.Context, prompt string, contextData map[string]interface{}) (interface{}, error)
	StartSessionFunc              func(ctx context.Context) (string, error)
	EndSessionFunc                func(ctx context.Context) error
	GetSessionIDFunc              func() string
	CloseFunc                     func() error

	// State tracking for tests
	CommandsExecuted []interface{}
	SessionsStarted  []string
	SessionsEnded    []string
	RetryAttempts    map[string]int
}

// NewMockClaudeClient creates a new mock Claude client with default behaviors
func NewMockClaudeClient() *MockClaudeClient {
	return &MockClaudeClient{
		CommandsExecuted: make([]interface{}, 0),
		SessionsStarted:  make([]string, 0),
		SessionsEnded:    make([]string, 0),
		RetryAttempts:    make(map[string]int),

		// Default implementations
		IsAvailableFunc: func() bool {
			return true
		},

		ExecuteCommandFunc: func(ctx context.Context, command interface{}) (interface{}, error) {
			return map[string]interface{}{
				"Success": true,
				"Content": `{"resolutions": [{"file_path": "test.go", "start_line": 1, "end_line": 5, "resolved_lines": ["resolved code"], "confidence": 0.8}]}`,
			}, nil
		},

		ExecuteConflictResolutionFunc: func(ctx context.Context, prompt string, contextData map[string]interface{}) (interface{}, error) {
			return map[string]interface{}{
				"Success": true,
				"Content": TestClaudeJSONResponse(),
			}, nil
		},

		StartSessionFunc: func(ctx context.Context) (string, error) {
			return fmt.Sprintf("session-%d", time.Now().Unix()), nil
		},

		EndSessionFunc: func(ctx context.Context) error {
			return nil
		},

		GetSessionIDFunc: func() string {
			return "mock-session-id"
		},

		CloseFunc: func() error {
			return nil
		},
	}
}

// IsAvailable implements the Claude client interface
func (m *MockClaudeClient) IsAvailable() bool {
	if m.IsAvailableFunc != nil {
		return m.IsAvailableFunc()
	}
	return true
}

// ExecuteCommand implements the Claude client interface
func (m *MockClaudeClient) ExecuteCommand(ctx context.Context, command interface{}) (interface{}, error) {
	m.CommandsExecuted = append(m.CommandsExecuted, command)

	if m.ExecuteCommandFunc != nil {
		return m.ExecuteCommandFunc(ctx, command)
	}

	return map[string]interface{}{
		"Success": true,
		"Content": "Mock response",
	}, nil
}

// ExecuteWithRetry implements the Claude client interface
func (m *MockClaudeClient) ExecuteWithRetry(ctx context.Context, command interface{}, maxRetries int) (interface{}, error) {
	key := fmt.Sprintf("%v", command)
	m.RetryAttempts[key] = maxRetries

	if m.ExecuteWithRetryFunc != nil {
		return m.ExecuteWithRetryFunc(ctx, command, maxRetries)
	}

	return m.ExecuteCommand(ctx, command)
}

// ExecuteConflictResolution implements the Claude client interface
func (m *MockClaudeClient) ExecuteConflictResolution(ctx context.Context, prompt string, contextData map[string]interface{}) (interface{}, error) {
	if m.ExecuteConflictResolutionFunc != nil {
		return m.ExecuteConflictResolutionFunc(ctx, prompt, contextData)
	}

	return map[string]interface{}{
		"Success": true,
		"Content": TestClaudeJSONResponse(),
	}, nil
}

// StartSession implements the Claude client interface
func (m *MockClaudeClient) StartSession(ctx context.Context) (string, error) {
	sessionID := fmt.Sprintf("mock-session-%d", len(m.SessionsStarted))
	m.SessionsStarted = append(m.SessionsStarted, sessionID)

	if m.StartSessionFunc != nil {
		return m.StartSessionFunc(ctx)
	}

	return sessionID, nil
}

// EndSession implements the Claude client interface
func (m *MockClaudeClient) EndSession(ctx context.Context) error {
	m.SessionsEnded = append(m.SessionsEnded, "ended")

	if m.EndSessionFunc != nil {
		return m.EndSessionFunc(ctx)
	}

	return nil
}

// GetSessionID implements the Claude client interface
func (m *MockClaudeClient) GetSessionID() string {
	if m.GetSessionIDFunc != nil {
		return m.GetSessionIDFunc()
	}
	return "mock-session-id"
}

// Close implements the Claude client interface
func (m *MockClaudeClient) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

// MockGitOperations provides mock implementations for git operations
type MockGitOperations struct {
	ConflictFiles    []map[string]interface{}
	ConflictHunks    map[string][]map[string]interface{}
	IsInMergeState   bool
	ValidationErrors map[string]error
	FileContents     map[string]string
}

// NewMockGitOperations creates a new mock git operations helper
func NewMockGitOperations() *MockGitOperations {
	return &MockGitOperations{
		ConflictFiles:    make([]map[string]interface{}, 0),
		ConflictHunks:    make(map[string][]map[string]interface{}),
		IsInMergeState:   false,
		ValidationErrors: make(map[string]error),
		FileContents:     make(map[string]string),
	}
}

// AddConflictFile adds a mock conflict file for testing
func (m *MockGitOperations) AddConflictFile(filePath, status string, hunks ...map[string]interface{}) {
	m.ConflictFiles = append(m.ConflictFiles, map[string]interface{}{
		"FilePath": filePath,
		"Status":   status,
	})
	m.ConflictHunks[filePath] = hunks
}

// SetFileContent sets mock file content for testing
func (m *MockGitOperations) SetFileContent(filePath, content string) {
	m.FileContents[filePath] = content
}

// CreateTempGitRepo creates a temporary git repository for testing
func CreateTempGitRepo() (string, func(), error) {
	tempDir, err := os.MkdirTemp("", "syncwright-test-*")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	// Create basic git structure
	gitDir := filepath.Join(tempDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to create .git directory: %w", err)
	}

	// Create minimal git config
	configContent := `[core]
	repositoryformatversion = 0
	filemode = true
	bare = false
	logallrefupdates = true
`
	configFile := filepath.Join(gitDir, "config")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to create git config: %w", err)
	}

	// Create HEAD file
	headFile := filepath.Join(gitDir, "HEAD")
	if err := os.WriteFile(headFile, []byte("ref: refs/heads/main\n"), 0644); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to create HEAD file: %w", err)
	}

	// Create refs structure
	refsDir := filepath.Join(gitDir, "refs", "heads")
	if err := os.MkdirAll(refsDir, 0755); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to create refs directory: %w", err)
	}

	// Create objects dir
	objectsDir := filepath.Join(gitDir, "objects")
	if err := os.MkdirAll(objectsDir, 0755); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to create objects directory: %w", err)
	}

	return tempDir, cleanup, nil
}

// ConflictData represents conflict data for testing
type ConflictData struct {
	OursLines   []string
	TheirsLines []string
	BaseLines   []string
	BeforeLines []string
	AfterLines  []string
}

// CreateConflictFile creates a file with conflict markers for testing
func CreateConflictFile(repoPath, filePath string, conflicts ...ConflictData) error {
	fullPath := filepath.Join(repoPath, filePath)

	// Ensure directory exists
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	var content strings.Builder

	for i, conflict := range conflicts {
		if i > 0 {
			content.WriteString("\n")
		}

		// Add some context before conflict
		for _, line := range conflict.BeforeLines {
			content.WriteString(line + "\n")
		}

		// Add conflict markers
		content.WriteString("<<<<<<< HEAD\n")
		for _, line := range conflict.OursLines {
			content.WriteString(line + "\n")
		}

		if len(conflict.BaseLines) > 0 {
			content.WriteString("||||||| base\n")
			for _, line := range conflict.BaseLines {
				content.WriteString(line + "\n")
			}
		}

		content.WriteString("=======\n")
		for _, line := range conflict.TheirsLines {
			content.WriteString(line + "\n")
		}
		content.WriteString(">>>>>>> branch\n")

		// Add some context after conflict
		for _, line := range conflict.AfterLines {
			content.WriteString(line + "\n")
		}
	}

	return os.WriteFile(fullPath, []byte(content.String()), 0644)
}

// CreateGoConflictFile creates a Go file with realistic conflicts
func CreateGoConflictFile(repoPath, filePath string) error {
	conflicts := []ConflictData{
		{
			BeforeLines: []string{
				"package main",
				"",
				"import (",
				"\t\"fmt\"",
				")",
				"",
			},
			OursLines: []string{
				"func greet(name string) {",
				"\tfmt.Printf(\"Hello, %s!\\n\", name)",
				"}",
			},
			TheirsLines: []string{
				"func greet(name string) error {",
				"\tif name == \"\" {",
				"\t\treturn fmt.Errorf(\"name cannot be empty\")",
				"\t}",
				"\tfmt.Printf(\"Hello, %s!\\n\", name)",
				"\treturn nil",
				"}",
			},
			AfterLines: []string{
				"",
				"func main() {",
				"\tgreet(\"World\")",
				"}",
			},
		},
	}

	return CreateConflictFile(repoPath, filePath, conflicts...)
}

// CreateJavaScriptConflictFile creates a JavaScript file with realistic conflicts
func CreateJavaScriptConflictFile(repoPath, filePath string) error {
	conflicts := []ConflictData{
		{
			BeforeLines: []string{
				"const express = require('express');",
				"const app = express();",
				"",
			},
			OursLines: []string{
				"app.get('/', (req, res) => {",
				"  res.send('Hello World!');",
				"});",
			},
			TheirsLines: []string{
				"app.get('/', (req, res) => {",
				"  res.json({ message: 'Hello World!' });",
				"});",
			},
			AfterLines: []string{
				"",
				"const PORT = process.env.PORT || 3000;",
				"app.listen(PORT, () => {",
				"  console.log(`Server running on port ${PORT}`);",
				"});",
			},
		},
	}

	return CreateConflictFile(repoPath, filePath, conflicts...)
}

// TestingT is an interface that matches the testing.T interface for our helpers
type TestingT interface {
	Fatalf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Helper()
}

// AssertNoError is a test helper that fails the test if err is not nil
func AssertNoError(t TestingT, err error, msgAndArgs ...interface{}) {
	if err != nil {
		if len(msgAndArgs) > 0 {
			t.Fatalf("Expected no error, but got: %v. Message: %v", err, msgAndArgs[0])
		} else {
			t.Fatalf("Expected no error, but got: %v", err)
		}
	}
}

// AssertError is a test helper that fails the test if err is nil
func AssertError(t TestingT, err error, msgAndArgs ...interface{}) {
	if err == nil {
		if len(msgAndArgs) > 0 {
			t.Fatalf("Expected an error, but got nil. Message: %v", msgAndArgs[0])
		} else {
			t.Fatalf("Expected an error, but got nil")
		}
	}
}

// AssertEqual is a test helper that fails the test if expected != actual
func AssertEqual(t TestingT, expected, actual interface{}, msgAndArgs ...interface{}) {
	if expected != actual {
		if len(msgAndArgs) > 0 {
			t.Fatalf("Expected %v, but got %v. Message: %v", expected, actual, msgAndArgs[0])
		} else {
			t.Fatalf("Expected %v, but got %v", expected, actual)
		}
	}
}

// AssertTrue is a test helper that fails the test if condition is false
func AssertTrue(t TestingT, condition bool, msgAndArgs ...interface{}) {
	if !condition {
		if len(msgAndArgs) > 0 {
			t.Fatalf("Expected condition to be true. Message: %v", msgAndArgs[0])
		} else {
			t.Fatalf("Expected condition to be true")
		}
	}
}

// AssertFalse is a test helper that fails the test if condition is true
func AssertFalse(t TestingT, condition bool, msgAndArgs ...interface{}) {
	if condition {
		if len(msgAndArgs) > 0 {
			t.Fatalf("Expected condition to be false. Message: %v", msgAndArgs[0])
		} else {
			t.Fatalf("Expected condition to be false")
		}
	}
}

// AssertContains is a test helper that fails the test if haystack doesn't contain needle
func AssertContains(t TestingT, haystack, needle string, msgAndArgs ...interface{}) {
	if !strings.Contains(haystack, needle) {
		if len(msgAndArgs) > 0 {
			t.Fatalf("Expected '%s' to contain '%s'. Message: %v", haystack, needle, msgAndArgs[0])
		} else {
			t.Fatalf("Expected '%s' to contain '%s'", haystack, needle)
		}
	}
}

// SecurityTestData provides test data for security validation tests
type SecurityTestData struct {
	MaliciousPaths    []string
	SafePaths         []string
	CommandInjections []string
	SafeCommands      []string
}

// GetSecurityTestData returns common security test data
func GetSecurityTestData() *SecurityTestData {
	return &SecurityTestData{
		MaliciousPaths: []string{
			"../../../etc/passwd",
			"..\\..\\..\\windows\\system32\\config\\sam",
			"/etc/shadow",
			"../../.ssh/id_rsa",
			"file;rm -rf /",
			"file`rm -rf /`",
			"file$(rm -rf /)",
			"file|cat /etc/passwd",
			"file&cat /etc/passwd",
		},
		SafePaths: []string{
			"src/main.go",
			"internal/gitutils/conflict.go",
			"cmd/syncwright/main.go",
			"README.md",
			"go.mod",
		},
		CommandInjections: []string{
			"git status; rm -rf /",
			"git status && rm -rf /",
			"git status | cat /etc/passwd",
			"git status `cat /etc/passwd`",
			"git status $(cat /etc/passwd)",
			"git status & cat /etc/passwd",
		},
		SafeCommands: []string{
			"git status",
			"git diff --name-only",
			"go build ./...",
			"npm test",
			"python -m pytest",
		},
	}
}
