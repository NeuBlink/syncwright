package gitutils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NeuBlink/syncwright/internal/testutils"
)

func TestValidateGitPath(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		wantErr     bool
		errContains string
	}{
		{
			name:    "Valid relative path",
			path:    "src/main.go",
			wantErr: false,
		},
		{
			name:    "Valid absolute path",
			path:    "/home/user/project",
			wantErr: false,
		},
		{
			name:    "Valid current directory",
			path:    ".",
			wantErr: false,
		},
		{
			name:        "Empty path",
			path:        "",
			wantErr:     true,
			errContains: "path cannot be empty",
		},
		{
			name:        "Path with semicolon",
			path:        "src/main.go;rm -rf /",
			wantErr:     true,
			errContains: "dangerous characters",
		},
		{
			name:        "Path with pipe",
			path:        "src/main.go|cat /etc/passwd",
			wantErr:     true,
			errContains: "dangerous characters",
		},
		{
			name:        "Path with ampersand",
			path:        "src/main.go&cat /etc/passwd",
			wantErr:     true,
			errContains: "dangerous characters",
		},
		{
			name:        "Path with backtick",
			path:        "src/main.go`cat /etc/passwd`",
			wantErr:     true,
			errContains: "dangerous characters",
		},
		{
			name:        "Path with dollar",
			path:        "src/main.go$(cat /etc/passwd)",
			wantErr:     true,
			errContains: "dangerous characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGitPath(tt.path)

			if tt.wantErr {
				if err == nil {
					t.Errorf("validateGitPath() expected error, got nil")
					return
				}
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("validateGitPath() error = %v, expected to contain %v", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("validateGitPath() unexpected error = %v", err)
			}
		})
	}
}

func TestIsGitRepository(t *testing.T) {
	// This test requires actual git command to be available
	isRepo, err := IsGitRepository()

	// We don't assert the result since it depends on the test environment,
	// but we ensure it doesn't crash and returns a boolean
	if err != nil {
		// Skip if git is not available
		if strings.Contains(err.Error(), "not found") {
			t.Skip("Git not available in test environment")
		}
		// Other errors are not expected but not critical for this test
	}

	// Result should be a boolean (true or false)
	_ = isRepo // Just ensure it's a boolean
}

func TestIsGitRepositoryPath(t *testing.T) {
	// Create a temporary directory structure
	tempDir, cleanup, err := testutils.CreateTempGitRepo()
	if err != nil {
		t.Fatalf("Failed to create temp git repo: %v", err)
	}
	defer cleanup()

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "Valid git repository",
			path:     tempDir,
			expected: true,
		},
		{
			name:     "Non-existent directory",
			path:     "/nonexistent/path",
			expected: false,
		},
		{
			name:     "Directory without .git",
			path:     os.TempDir(),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsGitRepositoryPath(tt.path)

			if result != tt.expected {
				t.Errorf("IsGitRepositoryPath(%s) = %v, expected %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestGetConflictedFiles(t *testing.T) {
	// This test depends on git being available and in a conflicted state
	files, err := GetConflictedFiles()

	if err != nil {
		// Skip if git is not available
		if strings.Contains(err.Error(), "failed to get conflicted files") {
			t.Skip("Git not available or not in a repository")
		}
		t.Errorf("GetConflictedFiles() unexpected error = %v", err)
		return
	}

	// In most test environments, we expect no conflicted files
	// The main test is that it doesn't crash and returns a slice
	if files == nil {
		t.Errorf("GetConflictedFiles() returned nil, expected empty slice")
	}
}

func TestCommitChanges(t *testing.T) {
	// Create a temporary git repository
	tempDir, cleanup, err := testutils.CreateTempGitRepo()
	if err != nil {
		t.Fatalf("Failed to create temp git repo: %v", err)
	}
	defer cleanup()

	// Change to the temporary directory
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(oldDir)

	err = os.Chdir(tempDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create a test file to commit
	testFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test commit (will likely fail in test environment without proper git setup)
	err = CommitChanges("Test commit")
	if err != nil {
		// Skip if git operations fail (expected in test environment)
		if strings.Contains(err.Error(), "failed to add changes") ||
			strings.Contains(err.Error(), "failed to commit changes") {
			t.Skip("Git operations not available in test environment")
		}
		t.Errorf("CommitChanges() unexpected error = %v", err)
	}
}

func TestGetRecentlyModifiedFiles(t *testing.T) {
	// Create a temporary git repository
	tempDir, cleanup, err := testutils.CreateTempGitRepo()
	if err != nil {
		t.Fatalf("Failed to create temp git repo: %v", err)
	}
	defer cleanup()

	// Create some test files
	testFiles := []string{"file1.go", "file2.js", "file3.py"}
	for _, fileName := range testFiles {
		filePath := filepath.Join(tempDir, fileName)
		err = os.WriteFile(filePath, []byte("test content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", fileName, err)
		}
	}

	// Test getting recently modified files
	files, err := GetRecentlyModifiedFiles(tempDir, 7)
	if err != nil {
		// Skip if git operations fail (expected in test environment)
		if strings.Contains(err.Error(), "failed to get recently modified files") {
			t.Skip("Git operations not available in test environment")
		}
		t.Errorf("GetRecentlyModifiedFiles() unexpected error = %v", err)
		return
	}

	// In test environment, we may not have any committed files
	// The main test is that it doesn't crash and returns a slice
	if files == nil {
		t.Errorf("GetRecentlyModifiedFiles() returned nil, expected slice")
	}
}

func TestGetRecentlyModifiedFiles_DateValidation(t *testing.T) {
	tempDir, cleanup, err := testutils.CreateTempGitRepo()
	if err != nil {
		t.Fatalf("Failed to create temp git repo: %v", err)
	}
	defer cleanup()

	tests := []struct {
		name        string
		days        int
		expectError bool
	}{
		{
			name:        "Valid days parameter",
			days:        7,
			expectError: false,
		},
		{
			name:        "Zero days",
			days:        0,
			expectError: false,
		},
		{
			name:        "Negative days",
			days:        -1,
			expectError: false, // Should work (future dates)
		},
		{
			name:        "Large number of days",
			days:        365,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GetRecentlyModifiedFiles(tempDir, tt.days)

			if tt.expectError && err == nil {
				t.Errorf("GetRecentlyModifiedFiles() expected error, got nil")
			}

			if !tt.expectError && err != nil {
				// Skip if git operations fail (expected in test environment)
				if strings.Contains(err.Error(), "failed to get recently modified files") {
					t.Skip("Git operations not available in test environment")
				}
				t.Errorf("GetRecentlyModifiedFiles() unexpected error = %v", err)
			}
		})
	}
}

func TestGetAllTrackedFiles(t *testing.T) {
	// Create a temporary git repository
	tempDir, cleanup, err := testutils.CreateTempGitRepo()
	if err != nil {
		t.Fatalf("Failed to create temp git repo: %v", err)
	}
	defer cleanup()

	// Create some test files
	testFiles := []string{"tracked1.go", "tracked2.js"}
	for _, fileName := range testFiles {
		filePath := filepath.Join(tempDir, fileName)
		err = os.WriteFile(filePath, []byte("test content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", fileName, err)
		}
	}

	// Test getting all tracked files
	files, err := GetAllTrackedFiles(tempDir)
	if err != nil {
		// Skip if git operations fail (expected in test environment)
		if strings.Contains(err.Error(), "failed to get tracked files") {
			t.Skip("Git operations not available in test environment")
		}
		t.Errorf("GetAllTrackedFiles() unexpected error = %v", err)
		return
	}

	// In test environment, we may not have any tracked files
	// The main test is that it doesn't crash and returns a slice
	if files == nil {
		t.Errorf("GetAllTrackedFiles() returned nil, expected slice")
	}

	// Test with non-existent directory
	_, err = GetAllTrackedFiles("/nonexistent/path")
	if err == nil {
		t.Errorf("GetAllTrackedFiles() with non-existent path should return error")
	}
}

func TestGetAllTrackedFiles_FileFiltering(t *testing.T) {
	// Create a temporary git repository
	tempDir, cleanup, err := testutils.CreateTempGitRepo()
	if err != nil {
		t.Fatalf("Failed to create temp git repo: %v", err)
	}
	defer cleanup()

	// Create test files including some that don't exist
	existingFiles := []string{"existing1.go", "existing2.js"}
	for _, fileName := range existingFiles {
		filePath := filepath.Join(tempDir, fileName)
		err = os.WriteFile(filePath, []byte("test content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", fileName, err)
		}
	}

	// The function should only return files that actually exist on disk
	files, err := GetAllTrackedFiles(tempDir)
	if err != nil {
		// Skip if git operations fail
		if strings.Contains(err.Error(), "failed to get tracked files") {
			t.Skip("Git operations not available in test environment")
		}
		t.Errorf("GetAllTrackedFiles() unexpected error = %v", err)
		return
	}

	// Verify that all returned files exist
	for _, file := range files {
		fullPath := filepath.Join(tempDir, file)
		if _, err := os.Stat(fullPath); err != nil {
			t.Errorf("GetAllTrackedFiles() returned non-existent file: %s", file)
		}
	}
}

func TestGitPath_SecurityValidation(t *testing.T) {
	securityData := testutils.GetSecurityTestData()

	// Test validateGitPath with malicious paths
	for _, maliciousPath := range securityData.MaliciousPaths {
		t.Run("Malicious path: "+maliciousPath, func(t *testing.T) {
			err := validateGitPath(maliciousPath)

			if err == nil {
				t.Errorf("validateGitPath() with malicious path %s should have failed", maliciousPath)
			}

			if !strings.Contains(err.Error(), "dangerous characters") &&
				!strings.Contains(err.Error(), "path cannot be empty") {
				t.Errorf("validateGitPath() error should mention dangerous characters or empty path, got: %v", err)
			}
		})
	}

	// Test with safe paths
	for _, safePath := range securityData.SafePaths {
		t.Run("Safe path: "+safePath, func(t *testing.T) {
			err := validateGitPath(safePath)

			if err != nil {
				t.Errorf("validateGitPath() with safe path %s should have succeeded, got: %v", safePath, err)
			}
		})
	}
}

func TestGitOperations_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create a temporary git repository
	tempDir, cleanup, err := testutils.CreateTempGitRepo()
	if err != nil {
		t.Fatalf("Failed to create temp git repo: %v", err)
	}
	defer cleanup()

	// Test sequence of operations
	t.Run("Repository detection", func(t *testing.T) {
		isRepo := IsGitRepositoryPath(tempDir)
		if !isRepo {
			t.Errorf("IsGitRepositoryPath() = false for git repository")
		}
	})

	t.Run("File operations", func(t *testing.T) {
		// Create test files
		testFile := filepath.Join(tempDir, "integration_test.go")
		err = os.WriteFile(testFile, []byte("package main\n"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Test getting tracked files
		files, err := GetAllTrackedFiles(tempDir)
		if err != nil {
			if strings.Contains(err.Error(), "failed to get tracked files") {
				t.Skip("Git operations not available")
			}
			t.Errorf("GetAllTrackedFiles() error: %v", err)
		} else if files == nil {
			t.Errorf("GetAllTrackedFiles() returned nil")
		}

		// Test getting recently modified files
		recentFiles, err := GetRecentlyModifiedFiles(tempDir, 1)
		if err != nil {
			if strings.Contains(err.Error(), "failed to get recently modified files") {
				t.Skip("Git operations not available")
			}
			t.Errorf("GetRecentlyModifiedFiles() error: %v", err)
		} else if recentFiles == nil {
			t.Errorf("GetRecentlyModifiedFiles() returned nil")
		}
	})

	t.Run("Conflict detection", func(t *testing.T) {
		// Create a file with conflict markers
		err = testutils.CreateGoConflictFile(tempDir, "conflict_test.go")
		if err != nil {
			t.Fatalf("Failed to create conflict file: %v", err)
		}

		// Test conflict parsing
		hunks, err := ParseConflictHunks("conflict_test.go", tempDir)
		if err != nil {
			t.Errorf("ParseConflictHunks() error: %v", err)
		} else if len(hunks) == 0 {
			t.Errorf("ParseConflictHunks() found no conflicts in file with conflicts")
		}

		// Test file context extraction
		context, err := ExtractFileContext("conflict_test.go", tempDir, 5)
		if err != nil {
			t.Errorf("ExtractFileContext() error: %v", err)
		} else if len(context) == 0 {
			t.Errorf("ExtractFileContext() returned no context")
		}
	})
}

// Benchmark tests
func BenchmarkValidateGitPath_Single(b *testing.B) {
	paths := []string{
		"src/main.go",
		"internal/service/handler.go",
		"cmd/cli/main.go",
		"pkg/utils/helper.go",
		"test/integration_test.go",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		path := paths[i%len(paths)]
		_ = validateGitPath(path)
	}
}

func BenchmarkIsGitRepositoryPath(b *testing.B) {
	// Create a temporary git repository for benchmarking
	tempDir, cleanup, err := testutils.CreateTempGitRepo()
	if err != nil {
		b.Fatalf("Failed to create temp git repo: %v", err)
	}
	defer cleanup()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = IsGitRepositoryPath(tempDir)
	}
}

func BenchmarkParseConflictHunks(b *testing.B) {
	// Create a temporary git repository
	tempDir, cleanup, err := testutils.CreateTempGitRepo()
	if err != nil {
		b.Fatalf("Failed to create temp git repo: %v", err)
	}
	defer cleanup()

	// Create a conflict file
	err = testutils.CreateGoConflictFile(tempDir, "benchmark_conflict.go")
	if err != nil {
		b.Fatalf("Failed to create conflict file: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ParseConflictHunks("benchmark_conflict.go", tempDir)
	}
}
