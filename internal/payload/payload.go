// Package payload provides simplified functionality for generating AI-ready payloads from conflict data
package payload

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/NeuBlink/syncwright/internal/gitutils"
)

// ConflictPayload represents the simplified JSON payload sent to AI for resolution
type ConflictPayload struct {
	Metadata PayloadMetadata       `json:"metadata"`
	Files    []ConflictFilePayload `json:"files"`
}

// PayloadMetadata contains basic metadata about the conflict resolution request
type PayloadMetadata struct {
	Timestamp      time.Time `json:"timestamp"`
	RepoPath       string    `json:"repo_path"`
	TotalFiles     int       `json:"total_files"`
	TotalConflicts int       `json:"total_conflicts"`
	Version        string    `json:"version"`
}

// ConflictFilePayload represents a single file's conflict data for AI processing
type ConflictFilePayload struct {
	Path      string                `json:"path"`
	Language  string                `json:"language"`
	Conflicts []ConflictHunkPayload `json:"conflicts"`
	Context   FileContext           `json:"context,omitempty"`
}

// ConflictHunkPayload represents a conflict hunk with essential data
type ConflictHunkPayload struct {
	ID          string   `json:"id,omitempty"` // For compatibility with existing code
	StartLine   int      `json:"start_line"`
	EndLine     int      `json:"end_line"`
	OursLines   []string `json:"ours_lines"`
	TheirsLines []string `json:"theirs_lines"`
	BaseLines   []string `json:"base_lines,omitempty"` // For compatibility with diff3 style conflicts
}

// FileContext provides minimal context for better AI understanding (compatibility)
type FileContext struct {
	BeforeLines []string `json:"before_lines,omitempty"`
	AfterLines  []string `json:"after_lines,omitempty"`
}

// Simple file exclusion patterns
var excludePatterns = []string{
	"node_modules/",
	".git/",
	"vendor/",
	"target/",
	"build/",
	"dist/",
	".DS_Store",
	"*.min.js",
	"*.min.css",
	"package-lock.json",
	"yarn.lock",
	"Cargo.lock",
	"go.sum",
}

// Binary file extensions to exclude
var binaryExtensions = []string{
	".jpg", ".jpeg", ".png", ".gif", ".bmp", ".svg", ".ico",
	".mp3", ".mp4", ".avi", ".mov", ".wav", ".pdf",
	".zip", ".tar", ".gz", ".exe", ".dll", ".so", ".dylib",
	".jar", ".war", ".ear", ".class",
}

// BuildSimplePayload creates a simplified conflict payload from a conflict report
func BuildSimplePayload(report *gitutils.ConflictReport) (*ConflictPayload, error) {
	if report == nil {
		return nil, fmt.Errorf("conflict report cannot be nil")
	}

	payload := &ConflictPayload{
		Metadata: PayloadMetadata{
			Timestamp:      time.Now(),
			RepoPath:       report.RepoPath,
			TotalFiles:     len(report.ConflictedFiles),
			TotalConflicts: report.TotalConflicts,
			Version:        "1.0.0",
		},
	}

	// Process each conflicted file
	for _, conflictFile := range report.ConflictedFiles {
		// Apply simple exclusion filters
		if shouldExcludeFile(conflictFile.Path) {
			continue
		}

		filePayload := ConflictFilePayload{
			Path:     conflictFile.Path,
			Language: detectSimpleLanguage(conflictFile.Path),
			Context:  FileContext{}, // Empty context for simplicity
		}

		// Convert conflict hunks
		for i, hunk := range conflictFile.Hunks {
			hunkPayload := ConflictHunkPayload{
				ID:          fmt.Sprintf("%s:%d", conflictFile.Path, i), // For compatibility
				StartLine:   hunk.StartLine,
				EndLine:     hunk.EndLine,
				OursLines:   hunk.OursLines,
				TheirsLines: hunk.TheirsLines,
				BaseLines:   hunk.BaseLines, // Preserve BaseLines for compatibility
			}
			filePayload.Conflicts = append(filePayload.Conflicts, hunkPayload)
		}

		payload.Files = append(payload.Files, filePayload)
	}

	return payload, nil
}

// shouldExcludeFile determines if a file should be excluded from processing
func shouldExcludeFile(filePath string) bool {
	// Check exclude patterns
	for _, pattern := range excludePatterns {
		if strings.Contains(filePath, pattern) {
			return true
		}
	}

	// Check binary extensions
	ext := strings.ToLower(filepath.Ext(filePath))
	for _, binExt := range binaryExtensions {
		if ext == binExt {
			return true
		}
	}

	return false
}

// detectSimpleLanguage performs basic language detection based on file extension
func detectSimpleLanguage(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".go":
		return "go"
	case ".js", ".mjs":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".py":
		return "python"
	case ".java":
		return "java"
	case ".c":
		return "c"
	case ".cpp", ".cc", ".cxx":
		return "cpp"
	case ".h", ".hpp":
		return "header"
	case ".rs":
		return "rust"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".swift":
		return "swift"
	case ".kt", ".kts":
		return "kotlin"
	case ".scala":
		return "scala"
	case ".sh", ".bash":
		return "shell"
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".xml":
		return "xml"
	case ".html", ".htm":
		return "html"
	case ".css":
		return "css"
	case ".scss", ".sass":
		return "scss"
	case ".md", ".markdown":
		return "markdown"
	case ".sql":
		return "sql"
	case ".dockerfile":
		return "dockerfile"
	case ".makefile":
		return "makefile"
	default:
		if strings.HasSuffix(strings.ToLower(filePath), "dockerfile") {
			return "dockerfile"
		}
		if strings.HasSuffix(strings.ToLower(filePath), "makefile") {
			return "makefile"
		}
		return "text"
	}
}

// ToJSON converts the payload to JSON
func (p *ConflictPayload) ToJSON() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// FromJSON creates a payload from JSON
func FromJSON(data []byte) (*ConflictPayload, error) {
	var payload ConflictPayload
	err := json.Unmarshal(data, &payload)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal payload: %w", err)
	}
	return &payload, nil
}
