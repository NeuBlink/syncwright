package payload

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/NeuBlink/syncwright/internal/gitutils"
)

// ConflictPayload represents the JSON payload sent to AI for resolution
type ConflictPayload struct {
	Metadata    PayloadMetadata       `json:"metadata"`
	Files       []ConflictFilePayload `json:"files"`
	Context     RepositoryContext     `json:"context"`
	Preferences ResolutionPreferences `json:"preferences"`
}

// PayloadMetadata contains metadata about the conflict resolution request
type PayloadMetadata struct {
	Timestamp      time.Time `json:"timestamp"`
	RepoPath       string    `json:"repo_path"`
	TotalFiles     int       `json:"total_files"`
	TotalConflicts int       `json:"total_conflicts"`
	PayloadHash    string    `json:"payload_hash"`
	Version        string    `json:"version"`
}

// ConflictFilePayload represents a single file's conflict data for AI processing
type ConflictFilePayload struct {
	Path      string                `json:"path"`
	Language  string                `json:"language"`
	FileType  string                `json:"file_type"`
	Conflicts []ConflictHunkPayload `json:"conflicts"`
	Context   FileContext           `json:"context"`
	Metadata  FileMetadata          `json:"metadata"`
}

// ConflictHunkPayload represents a conflict hunk with minimal context
type ConflictHunkPayload struct {
	ID           string   `json:"id"`
	StartLine    int      `json:"start_line"`
	EndLine      int      `json:"end_line"`
	OursLines    []string `json:"ours_lines"`
	TheirsLines  []string `json:"theirs_lines"`
	BaseLines    []string `json:"base_lines,omitempty"`
	PreContext   []string `json:"pre_context"`
	PostContext  []string `json:"post_context"`
	ConflictType string   `json:"conflict_type"`
}

// FileContext provides surrounding context for better AI understanding
type FileContext struct {
	BeforeLines []string `json:"before_lines,omitempty"`
	AfterLines  []string `json:"after_lines,omitempty"`
	Imports     []string `json:"imports,omitempty"`
	Functions   []string `json:"functions,omitempty"`
	Classes     []string `json:"classes,omitempty"`
}

// FileMetadata contains file-specific metadata
type FileMetadata struct {
	Size        int64  `json:"size"`
	LineCount   int    `json:"line_count"`
	Encoding    string `json:"encoding"`
	LineEndings string `json:"line_endings"`
	HasTests    bool   `json:"has_tests"`
	IsGenerated bool   `json:"is_generated"`
}

// RepositoryContext provides repository-wide context
type RepositoryContext struct {
	BranchInfo   BranchInfo  `json:"branch_info"`
	CommitInfo   CommitInfo  `json:"commit_info"`
	ProjectInfo  ProjectInfo `json:"project_info"`
	Dependencies []string    `json:"dependencies,omitempty"`
	BuildSystem  string      `json:"build_system,omitempty"`
}

// BranchInfo contains information about the branches being merged
type BranchInfo struct {
	CurrentBranch string `json:"current_branch"`
	MergeBranch   string `json:"merge_branch"`
	BaseBranch    string `json:"base_branch,omitempty"`
	MergeBase     string `json:"merge_base,omitempty"`
}

// CommitInfo contains commit-related information
type CommitInfo struct {
	OursCommit   string `json:"ours_commit"`
	TheirsCommit string `json:"theirs_commit"`
	BaseCommit   string `json:"base_commit,omitempty"`
	MergeMessage string `json:"merge_message,omitempty"`
}

// ProjectInfo contains project-specific information
type ProjectInfo struct {
	Language    string            `json:"language"`
	Framework   string            `json:"framework,omitempty"`
	BuildTool   string            `json:"build_tool,omitempty"`
	ConfigFiles []string          `json:"config_files,omitempty"`
	Conventions map[string]string `json:"conventions,omitempty"`
}

// ResolutionPreferences contains preferences for how conflicts should be resolved
type ResolutionPreferences struct {
	PreferOurs        bool     `json:"prefer_ours"`
	PreferTheirs      bool     `json:"prefer_theirs"`
	PreserveBoth      bool     `json:"preserve_both"`
	ExcludeGenerated  bool     `json:"exclude_generated"`
	ExcludeLockfiles  bool     `json:"exclude_lockfiles"`
	MaxContextLines   int      `json:"max_context_lines"`
	IncludeComments   bool     `json:"include_comments"`
	IncludeTests      bool     `json:"include_tests"`
	SensitivePatterns []string `json:"sensitive_patterns,omitempty"`
}

// PayloadBuilder builds conflict payloads for AI processing
type PayloadBuilder struct {
	preferences       ResolutionPreferences
	filters           []FileFilter
	contextExtractors map[string]ContextExtractor
}

// FileFilter represents a filter for excluding files
type FileFilter interface {
	ShouldExclude(filePath string) bool
	GetReason() string
}

// ContextExtractor extracts language-specific context
type ContextExtractor interface {
	ExtractContext(filePath string, content []string) FileContext
	GetLanguage() string
}

// NewPayloadBuilder creates a new payload builder with default configuration
func NewPayloadBuilder() *PayloadBuilder {
	builder := &PayloadBuilder{
		preferences: ResolutionPreferences{
			ExcludeGenerated:  true,
			ExcludeLockfiles:  true,
			MaxContextLines:   5,
			IncludeComments:   true,
			IncludeTests:      true,
			SensitivePatterns: DefaultSensitivePatterns(),
		},
		filters: []FileFilter{
			NewSensitiveFileFilter(),
			NewBinaryFileFilter(),
			NewGeneratedFileFilter(),
			NewLockfileFilter(),
		},
		contextExtractors: make(map[string]ContextExtractor),
	}

	// Register default context extractors
	builder.contextExtractors["go"] = NewGoContextExtractor()
	builder.contextExtractors["javascript"] = NewJavaScriptContextExtractor()
	builder.contextExtractors["typescript"] = NewTypeScriptContextExtractor()
	builder.contextExtractors["python"] = NewPythonContextExtractor()

	return builder
}

// BuildPayload creates a conflict payload from a conflict report
func (pb *PayloadBuilder) BuildPayload(report *gitutils.ConflictReport) (*ConflictPayload, error) {
	payload := &ConflictPayload{
		Metadata: PayloadMetadata{
			Timestamp:      time.Now(),
			RepoPath:       report.RepoPath,
			TotalFiles:     len(report.ConflictedFiles),
			TotalConflicts: report.TotalConflicts,
			Version:        "1.0.0",
		},
		Preferences: pb.preferences,
	}

	// Extract repository context
	repoContext, err := pb.extractRepositoryContext(report.RepoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to extract repository context: %w", err)
	}
	payload.Context = repoContext

	// Process each conflicted file
	for _, conflictFile := range report.ConflictedFiles {
		// Apply filters
		if pb.shouldExcludeFile(conflictFile.Path) {
			continue
		}

		filePayload, err := pb.buildFilePayload(conflictFile, report.RepoPath)
		if err != nil {
			return nil, fmt.Errorf("failed to build payload for file %s: %w", conflictFile.Path, err)
		}

		if filePayload != nil {
			payload.Files = append(payload.Files, *filePayload)
		}
	}

	// Generate payload hash
	payload.Metadata.PayloadHash = pb.generatePayloadHash(payload)

	return payload, nil
}

// buildFilePayload creates a file payload from a conflict file
func (pb *PayloadBuilder) buildFilePayload(conflictFile gitutils.ConflictFile, repoPath string) (*ConflictFilePayload, error) {
	language := DetectLanguage(conflictFile.Path)
	fileType := DetectFileType(conflictFile.Path)

	filePayload := &ConflictFilePayload{
		Path:     conflictFile.Path,
		Language: language,
		FileType: fileType,
	}

	// Extract file metadata
	metadata, err := pb.extractFileMetadata(conflictFile.Path, repoPath, conflictFile.Context)
	if err != nil {
		return nil, fmt.Errorf("failed to extract metadata: %w", err)
	}
	filePayload.Metadata = metadata

	// Build conflict hunks
	for i, hunk := range conflictFile.Hunks {
		hunkPayload := ConflictHunkPayload{
			ID:           fmt.Sprintf("%s:%d", conflictFile.Path, i),
			StartLine:    hunk.StartLine,
			EndLine:      hunk.EndLine,
			OursLines:    pb.sanitizeLines(hunk.OursLines),
			TheirsLines:  pb.sanitizeLines(hunk.TheirsLines),
			BaseLines:    pb.sanitizeLines(hunk.BaseLines),
			ConflictType: pb.classifyConflict(hunk),
		}

		// Extract context around the conflict
		preContext, postContext := pb.extractConflictContext(conflictFile.Context, hunk)
		hunkPayload.PreContext = pb.sanitizeLines(preContext)
		hunkPayload.PostContext = pb.sanitizeLines(postContext)

		filePayload.Conflicts = append(filePayload.Conflicts, hunkPayload)
	}

	// Extract file context using language-specific extractor
	if extractor, exists := pb.contextExtractors[language]; exists {
		filePayload.Context = extractor.ExtractContext(conflictFile.Path, conflictFile.Context)
	}

	return filePayload, nil
}

// shouldExcludeFile determines if a file should be excluded from processing
func (pb *PayloadBuilder) shouldExcludeFile(filePath string) bool {
	for _, filter := range pb.filters {
		if filter.ShouldExclude(filePath) {
			return true
		}
	}
	return false
}

// sanitizeLines removes sensitive information from lines
func (pb *PayloadBuilder) sanitizeLines(lines []string) []string {
	if lines == nil {
		return nil
	}

	sanitized := make([]string, len(lines))
	for i, line := range lines {
		sanitized[i] = pb.sanitizeLine(line)
	}
	return sanitized
}

// sanitizeLine removes sensitive information from a single line
func (pb *PayloadBuilder) sanitizeLine(line string) string {
	sanitized := line

	// Apply sensitive patterns
	for _, pattern := range pb.preferences.SensitivePatterns {
		if strings.Contains(strings.ToLower(sanitized), strings.ToLower(pattern)) {
			// Replace sensitive content with placeholder
			sanitized = strings.ReplaceAll(sanitized, pattern, "[REDACTED]")
		}
	}

	return sanitized
}

// extractConflictContext extracts surrounding context for a conflict hunk
func (pb *PayloadBuilder) extractConflictContext(fileContent []string, hunk gitutils.ConflictHunk) ([]string, []string) {
	maxLines := pb.preferences.MaxContextLines

	// Extract pre-context
	preStart := max(0, hunk.StartLine-maxLines-1)
	preEnd := max(0, hunk.StartLine-1)
	var preContext []string
	if preEnd > preStart && preStart < len(fileContent) {
		preContext = fileContent[preStart:min(preEnd, len(fileContent))]
	}

	// Extract post-context
	postStart := min(hunk.EndLine, len(fileContent))
	postEnd := min(hunk.EndLine+maxLines, len(fileContent))
	var postContext []string
	if postStart < postEnd && postStart < len(fileContent) {
		postContext = fileContent[postStart:postEnd]
	}

	return preContext, postContext
}

// classifyConflict determines the type of conflict
func (pb *PayloadBuilder) classifyConflict(hunk gitutils.ConflictHunk) string {
	oursEmpty := len(hunk.OursLines) == 0 || (len(hunk.OursLines) == 1 && strings.TrimSpace(hunk.OursLines[0]) == "")
	theirsEmpty := len(hunk.TheirsLines) == 0 || (len(hunk.TheirsLines) == 1 && strings.TrimSpace(hunk.TheirsLines[0]) == "")

	if oursEmpty && theirsEmpty {
		return "deletion"
	} else if oursEmpty {
		return "addition_theirs"
	} else if theirsEmpty {
		return "addition_ours"
	} else {
		// Check for similar content (might be whitespace or formatting conflict)
		if pb.areLinesSemanticallySimilar(hunk.OursLines, hunk.TheirsLines) {
			return "formatting"
		}
		return "modification"
	}
}

// areLinesSemanticallySimilar checks if lines are semantically similar
func (pb *PayloadBuilder) areLinesSemanticallySimilar(ours, theirs []string) bool {
	if len(ours) != len(theirs) {
		return false
	}

	for i := range ours {
		// Remove whitespace and compare
		oursNorm := strings.ReplaceAll(strings.TrimSpace(ours[i]), " ", "")
		theirsNorm := strings.ReplaceAll(strings.TrimSpace(theirs[i]), " ", "")

		if oursNorm != theirsNorm {
			return false
		}
	}

	return true
}

// generatePayloadHash generates a hash for the payload
func (pb *PayloadBuilder) generatePayloadHash(payload *ConflictPayload) string {
	// Create a hash of the essential payload content
	hasher := sha256.New()

	// Include file paths and conflict positions
	for _, file := range payload.Files {
		hasher.Write([]byte(file.Path))
		for _, conflict := range file.Conflicts {
			hasher.Write([]byte(fmt.Sprintf("%d:%d", conflict.StartLine, conflict.EndLine)))
		}
	}

	return fmt.Sprintf("%x", hasher.Sum(nil))[:16]
}

// Helper functions
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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
