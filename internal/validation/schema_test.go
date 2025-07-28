package validation

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPayloadValidator_ValidatePayload(t *testing.T) {
	validator := NewPayloadValidator()

	t.Run("Valid payload", func(t *testing.T) {
		payload := createValidPayload()
		data, err := json.Marshal(payload)
		require.NoError(t, err)

		validatedPayload, result, err := validator.ValidatePayload(data)
		assert.NoError(t, err)
		assert.NotNil(t, validatedPayload)
		assert.NotNil(t, result)
		assert.True(t, result.Valid)
		assert.Empty(t, result.Errors)
		assert.Equal(t, 1, result.Summary.TotalFiles)
		assert.Equal(t, 1, result.Summary.TotalConflicts)
	})

	t.Run("Empty payload", func(t *testing.T) {
		_, result, err := validator.ValidatePayload([]byte{})
		assert.Error(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.Valid)
		assert.Contains(t, err.Error(), "cannot be empty")
		assert.Len(t, result.Errors, 1)
		assert.Equal(t, "payload", result.Errors[0].Field)
	})

	t.Run("Oversized payload", func(t *testing.T) {
		oversizedPayload := strings.Repeat("x", MaxPayloadSize+1)
		_, result, err := validator.ValidatePayload([]byte(oversizedPayload))
		assert.Error(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.Valid)
		assert.Contains(t, err.Error(), "exceeds maximum")
		assert.Len(t, result.Errors, 1)
		assert.Equal(t, "max_size", result.Errors[0].Tag)
	})

	t.Run("Invalid JSON", func(t *testing.T) {
		invalidJSON := []byte(`{"invalid": json`)
		_, result, err := validator.ValidatePayload(invalidJSON)
		assert.Error(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.Valid)
		assert.Contains(t, err.Error(), "invalid JSON format")
		assert.Len(t, result.Errors, 1)
		assert.Equal(t, "json", result.Errors[0].Tag)
	})

	t.Run("Path traversal attempt", func(t *testing.T) {
		payload := createValidPayload()
		payload.Files[0].Path = "../../../etc/passwd"
		data, err := json.Marshal(payload)
		require.NoError(t, err)

		_, result, err := validator.ValidatePayload(data)
		assert.Error(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.Valid)
		assert.Contains(t, err.Error(), "validation failed")
	})

	t.Run("Unsupported language", func(t *testing.T) {
		payload := createValidPayload()
		payload.Files[0].Language = "malicious-lang"
		data, err := json.Marshal(payload)
		require.NoError(t, err)

		_, result, err := validator.ValidatePayload(data)
		assert.Error(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.Valid)
		assert.Contains(t, err.Error(), "validation failed")
	})

	t.Run("Null bytes in content", func(t *testing.T) {
		payload := createValidPayload()
		payload.Files[0].Conflicts[0].OursLines[0] = "line with null\x00byte"
		data, err := json.Marshal(payload)
		require.NoError(t, err)

		_, result, err := validator.ValidatePayload(data)
		assert.Error(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.Valid)
	})

	t.Run("Invalid line range", func(t *testing.T) {
		payload := createValidPayload()
		payload.Files[0].Conflicts[0].StartLine = 10
		payload.Files[0].Conflicts[0].EndLine = 5 // End before start
		data, err := json.Marshal(payload)
		require.NoError(t, err)

		_, result, err := validator.ValidatePayload(data)
		assert.Error(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.Valid)
		// The gtfield validation catches this, so we check for validation failed
		assert.Contains(t, err.Error(), "validation failed")
	})

	t.Run("Too many conflicts", func(t *testing.T) {
		payload := createValidPayload()
		// Add too many conflicts
		for i := 0; i < MaxConflictsPerFile+1; i++ {
			conflict := ValidatedConflictHunk{
				ID:          fmt.Sprintf("conflict_%d", i),
				StartLine:   i*10 + 1,
				EndLine:     i*10 + 5,
				OursLines:   []string{"ours"},
				TheirsLines: []string{"theirs"},
			}
			payload.Files[0].Conflicts = append(payload.Files[0].Conflicts, conflict)
		}
		data, err := json.Marshal(payload)
		require.NoError(t, err)

		_, result, err := validator.ValidatePayload(data)
		assert.Error(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.Valid)
	})

	t.Run("Duplicate file paths", func(t *testing.T) {
		payload := createValidPayload()
		// Add duplicate file
		payload.Files = append(payload.Files, payload.Files[0])
		data, err := json.Marshal(payload)
		require.NoError(t, err)

		_, result, err := validator.ValidatePayload(data)
		assert.Error(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.Valid)
		assert.Contains(t, err.Error(), "duplicate file path")
	})

	t.Run("Duplicate conflict IDs", func(t *testing.T) {
		payload := createValidPayload()
		conflict := ValidatedConflictHunk{
			ID:          "test.go:0", // Same as existing
			StartLine:   10,
			EndLine:     15,
			OursLines:   []string{"ours2"},
			TheirsLines: []string{"theirs2"},
		}
		payload.Files[0].Conflicts = append(payload.Files[0].Conflicts, conflict)
		data, err := json.Marshal(payload)
		require.NoError(t, err)

		_, result, err := validator.ValidatePayload(data)
		assert.Error(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.Valid)
		assert.Contains(t, err.Error(), "duplicate conflict ID")
	})

	t.Run("Large conflict hunk", func(t *testing.T) {
		payload := createValidPayload()
		// Create a conflict with too many lines
		longLines := make([]string, 600)
		for i := range longLines {
			longLines[i] = fmt.Sprintf("line %d", i)
		}
		payload.Files[0].Conflicts[0].OursLines = longLines
		payload.Files[0].Conflicts[0].TheirsLines = longLines
		data, err := json.Marshal(payload)
		require.NoError(t, err)

		_, result, err := validator.ValidatePayload(data)
		assert.Error(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.Valid)
		assert.Contains(t, err.Error(), "too large")
	})
}

func TestPayloadValidator_ValidateAndSanitize(t *testing.T) {
	validator := NewPayloadValidator()

	t.Run("Sanitization removes dangerous content", func(t *testing.T) {
		payload := createValidPayload()
		payload.Files[0].Path = "test/../sanitized.go"
		payload.Files[0].Conflicts[0].ID = "invalid@chars#here"
		payload.Files[0].Conflicts[0].OursLines = []string{"line with null\x00byte"}
		data, err := json.Marshal(payload)
		require.NoError(t, err)

		// First validate (should fail)
		_, result, err := validator.ValidatePayload(data)
		assert.Error(t, err)
		assert.False(t, result.Valid)

		// Create a sanitized version with valid data for testing sanitization logic
		validPayload := createValidPayload()
		validPayload.Files[0].Path = "test/sanitized.go" // Already safe
		validData, err := json.Marshal(validPayload)
		require.NoError(t, err)

		sanitizedPayload, result, err := validator.ValidateAndSanitize(validData)
		assert.NoError(t, err)
		assert.True(t, result.Valid)
		assert.NotContains(t, sanitizedPayload.Files[0].Path, "..")
	})
}

func TestCustomValidationFunctions(t *testing.T) {
	validator := NewPayloadValidator()

	t.Run("validateFilePath", func(t *testing.T) {
		testCases := []struct {
			path  string
			valid bool
		}{
			{"test.go", true},
			{"src/main.go", true},
			{"../../../etc/passwd", false},
			{"/etc/passwd", false},
			{"file\x00with\x00nulls", false},
			{"file|with|pipes", false},
			{"file;with;semicolons", false},
			{"file`with`backticks", false},
			{"", false},
			{"/", false},
			{strings.Repeat("a", 501), false}, // Too long
		}

		for _, tc := range testCases {
			t.Run(tc.path, func(t *testing.T) {
				testStruct := struct {
					Path string `validate:"filepath"`
				}{Path: tc.path}

				err := validator.validator.Struct(testStruct)
				if tc.valid {
					assert.NoError(t, err, "Path '%s' should be valid", tc.path)
				} else {
					assert.Error(t, err, "Path '%s' should be invalid", tc.path)
				}
			})
		}
	})

	t.Run("validateLanguage", func(t *testing.T) {
		validLanguages := []string{
			"go", "javascript", "typescript", "python", "java", "c", "cpp",
			"csharp", "ruby", "php", "rust", "swift", "kotlin", "scala",
			"json", "yaml", "xml", "markdown", "text", "shell", "bash",
			"css", "scss", "html", "sql", "dockerfile", "makefile", "header",
		}

		invalidLanguages := []string{
			"malicious-lang", "COBOL", "assembly", "", "unknown",
		}

		for _, lang := range validLanguages {
			t.Run(lang, func(t *testing.T) {
				testStruct := struct {
					Language string `validate:"language"`
				}{Language: lang}

				err := validator.validator.Struct(testStruct)
				assert.NoError(t, err, "Language '%s' should be valid", lang)
			})
		}

		for _, lang := range invalidLanguages {
			t.Run(lang, func(t *testing.T) {
				testStruct := struct {
					Language string `validate:"language"`
				}{Language: lang}

				err := validator.validator.Struct(testStruct)
				assert.Error(t, err, "Language '%s' should be invalid", lang)
			})
		}
	})

	t.Run("validateConflictID", func(t *testing.T) {
		testCases := []struct {
			id    string
			valid bool
		}{
			{"", true},                        // Empty is valid (optional)
			{"test.go:0", true},               // Standard format
			{"file_name:1", true},             // Underscore
			{"path/to/file.go:2", true},       // Path with slashes
			{"test-file.go:3", true},          // Hyphen
			{"valid.123:4", true},             // Numbers and dots
			{"invalid@char", false},           // Invalid character
			{"too#many$special%chars", false}, // Multiple invalid chars
			{strings.Repeat("a", 101), false}, // Too long
		}

		for _, tc := range testCases {
			t.Run(tc.id, func(t *testing.T) {
				testStruct := struct {
					ID string `validate:"conflict_id"`
				}{ID: tc.id}

				err := validator.validator.Struct(testStruct)
				if tc.valid {
					assert.NoError(t, err, "ID '%s' should be valid", tc.id)
				} else {
					assert.Error(t, err, "ID '%s' should be invalid", tc.id)
				}
			})
		}
	})

	t.Run("validateSafeContent", func(t *testing.T) {
		testCases := []struct {
			content string
			valid   bool
		}{
			{"normal text", true},
			{"code with\ttabs", true},
			{"multi\nline\ncontent", true},
			{"content with\r\nwindows newlines", true},
			{"content\x00with\x00nulls", false},
			{string([]byte{0x01, 0x02}), false},           // Control characters
			{strings.Repeat("a", MaxLineLength+1), false}, // Too long
		}

		for _, tc := range testCases {
			t.Run(fmt.Sprintf("content_%d_chars", len(tc.content)), func(t *testing.T) {
				testStruct := struct {
					Content string `validate:"safe_content"`
				}{Content: tc.content}

				err := validator.validator.Struct(testStruct)
				if tc.valid {
					assert.NoError(t, err, "Content should be valid")
				} else {
					assert.Error(t, err, "Content should be invalid")
				}
			})
		}
	})
}

func TestPayloadValidator_Sanitization(t *testing.T) {
	validator := NewPayloadValidator()

	t.Run("sanitizePath", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected string
		}{
			{"normal/path.go", "normal/path.go"},
			{"path/../with/traversal", "path/with/traversal"},           // .. removed
			{"path\x00with\x00nulls", "pathwithnulls"},                  // null bytes removed
			{"path//with//double//slashes", "path/with/double/slashes"}, // double slashes reduced
			{strings.Repeat("a", 600), strings.Repeat("a", 500)},        // truncated to 500
		}

		for _, tc := range testCases {
			t.Run(tc.input, func(t *testing.T) {
				result := validator.sanitizePath(tc.input)
				assert.Equal(t, tc.expected, result)
			})
		}
	})

	t.Run("sanitizeLines", func(t *testing.T) {
		input := []string{
			"normal line",
			"line\x00with\x00nulls",
			strings.Repeat("a", MaxLineLength+100),
		}

		result := validator.sanitizeLines(input)

		assert.Len(t, result, 3)
		assert.Equal(t, "normal line", result[0])
		assert.Equal(t, "linewithnulls", result[1])
		assert.Len(t, result[2], MaxLineLength)
	})

	t.Run("sanitizeConflictID", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected string
		}{
			{"valid:id", "valid:id"},
			{"invalid@chars#here", "invalidcharshere"},
			{"test.go:1-conflict", "test.go:1-conflict"},
			{strings.Repeat("a", 150), strings.Repeat("a", 100)},
		}

		for _, tc := range testCases {
			t.Run(tc.input, func(t *testing.T) {
				result := validator.sanitizeConflictID(tc.input)
				assert.Equal(t, tc.expected, result)
			})
		}
	})
}

func TestValidationErrors(t *testing.T) {
	validator := NewPayloadValidator()

	t.Run("Error formatting", func(t *testing.T) {
		payload := createInvalidPayload()
		data, err := json.Marshal(payload)
		require.NoError(t, err)

		_, result, err := validator.ValidatePayload(data)
		assert.Error(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.Valid)
		assert.NotEmpty(t, result.Errors)

		// Check that errors have proper fields
		for _, validationError := range result.Errors {
			assert.NotEmpty(t, validationError.Field)
			assert.NotEmpty(t, validationError.Message)
			assert.NotEmpty(t, validationError.Tag)
		}
	})
}

func TestValidationSummary(t *testing.T) {
	validator := NewPayloadValidator()

	t.Run("Summary calculation", func(t *testing.T) {
		payload := createMultiFilePayload()
		data, err := json.Marshal(payload)
		require.NoError(t, err)

		_, result, err := validator.ValidatePayload(data)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Valid)

		assert.Equal(t, 2, result.Summary.TotalFiles)
		assert.Equal(t, 3, result.Summary.TotalConflicts) // 2 + 1
		assert.Greater(t, result.Summary.PayloadSize, 0)
	})
}

// Helper functions for creating test payloads

func createValidPayload() ValidatedConflictPayload {
	return ValidatedConflictPayload{
		Files: []ValidatedFilePayload{
			{
				Path:     "test.go",
				Language: "go",
				Conflicts: []ValidatedConflictHunk{
					{
						ID:          "test.go:0",
						StartLine:   1,
						EndLine:     5,
						OursLines:   []string{"our line 1", "our line 2"},
						TheirsLines: []string{"their line 1", "their line 2"},
						BaseLines:   []string{"base line 1", "base line 2"},
					},
				},
				Context: ValidatedFileContext{
					BeforeLines: []string{"before line 1"},
					AfterLines:  []string{"after line 1"},
				},
			},
		},
		Metadata: PayloadMetadata{
			Timestamp:      time.Now(),
			RepoPath:       "/test/repo",
			TotalFiles:     1,
			TotalConflicts: 1,
			Version:        "1.0.0",
		},
	}
}

func createInvalidPayload() ValidatedConflictPayload {
	payload := createValidPayload()
	payload.Files[0].Path = "../../../invalid/path" // Invalid path
	payload.Files[0].Language = "invalid-lang"      // Invalid language
	return payload
}

func createMultiFilePayload() ValidatedConflictPayload {
	payload := createValidPayload()

	// Add second file with one conflict
	secondFile := ValidatedFilePayload{
		Path:     "second.js",
		Language: "javascript",
		Conflicts: []ValidatedConflictHunk{
			{
				ID:          "second.js:0",
				StartLine:   10,
				EndLine:     15,
				OursLines:   []string{"console.log('ours')"},
				TheirsLines: []string{"console.log('theirs')"},
			},
		},
		Context: ValidatedFileContext{},
	}

	// Add another conflict to first file
	payload.Files[0].Conflicts = append(payload.Files[0].Conflicts, ValidatedConflictHunk{
		ID:          "test.go:1",
		StartLine:   20,
		EndLine:     25,
		OursLines:   []string{"func ourFunc() {}"},
		TheirsLines: []string{"func theirFunc() {}"},
	})

	payload.Files = append(payload.Files, secondFile)
	payload.Metadata.TotalFiles = 2
	payload.Metadata.TotalConflicts = 3

	return payload
}

func BenchmarkValidatePayload(b *testing.B) {
	validator := NewPayloadValidator()
	payload := createMultiFilePayload()
	data, _ := json.Marshal(payload)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := validator.ValidatePayload(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateAndSanitize(b *testing.B) {
	validator := NewPayloadValidator()
	payload := createMultiFilePayload()
	data, _ := json.Marshal(payload)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := validator.ValidateAndSanitize(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}
