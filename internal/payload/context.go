package payload

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Constants for commonly used languages and file types
const (
	LanguageJavaScript = "javascript"
	LanguageTypeScript = "typescript"
	LanguagePython     = "python"
	LanguageGo         = "go"
	LanguageJSON       = "json"
	LanguageJava       = "java"
	FileTypeText       = "text"
)

// DetectLanguage determines the programming language from file extension
func DetectLanguage(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))

	languageMap := map[string]string{
		".go":    LanguageGo,
		".js":    LanguageJavaScript,
		".jsx":   LanguageJavaScript,
		".ts":    LanguageTypeScript,
		".tsx":   LanguageTypeScript,
		".py":    LanguagePython,
		".pyx":   "python",
		".pyi":   "python",
		".java":  LanguageJava,
		".kt":    "kotlin",
		".scala": "scala",
		".c":     "c",
		".h":     "c",
		".cpp":   "cpp",
		".cxx":   "cpp",
		".cc":    "cpp",
		".hpp":   "cpp",
		".cs":    "csharp",
		".fs":    "fsharp",
		".rb":    "ruby",
		".php":   "php",
		".rs":    "rust",
		".swift": "swift",
		".m":     "objective-c",
		".mm":    "objective-c",
		".dart":  "dart",
		".r":     "r",
		".R":     "r",
		".sql":   "sql",
		".sh":    "shell",
		".bash":  "shell",
		".zsh":   "shell",
		".fish":  "shell",
		".ps1":   "powershell",
		".vim":   "vim",
		".lua":   "lua",
		".pl":    "perl",
		".hs":    "haskell",
		".ml":    "ocaml",
		".clj":   "clojure",
		".ex":    "elixir",
		".exs":   "elixir",
		".erl":   "erlang",
		".nim":   "nim",
		".zig":   "zig",
		".v":     "vlang",
		".jl":    "julia",
		".html":  "html",
		".css":   "css",
		".scss":  "scss",
		".sass":  "sass",
		".less":  "less",
		".xml":   "xml",
		".json":  LanguageJSON,
		".yaml":  "yaml",
		".yml":   "yaml",
		".toml":  "toml",
		".ini":   "ini",
		".cfg":   "config",
		".conf":  "config",
		".md":    "markdown",
		".tex":   "latex",
	}

	if lang, exists := languageMap[ext]; exists {
		return lang
	}

	// Check filename for special cases
	fileName := filepath.Base(filePath)
	fileNameMap := map[string]string{
		"Makefile":       "makefile",
		"Dockerfile":     "dockerfile",
		"Vagrantfile":    "ruby",
		"Rakefile":       "ruby",
		"Gemfile":        "ruby",
		"Podfile":        "ruby",
		"CMakeLists.txt": "cmake",
	}

	if lang, exists := fileNameMap[fileName]; exists {
		return lang
	}

	return FileTypeText
}

// DetectFileType determines the general file type
func DetectFileType(filePath string) string {
	language := DetectLanguage(filePath)

	switch language {
	case LanguageGo, LanguageJavaScript, LanguageTypeScript, LanguagePython, LanguageJava, "kotlin", "scala",
		"c", "cpp", "csharp", "fsharp", "ruby", "php", "rust", "swift",
		"objective-c", "dart", "r", "perl", "haskell", "ocaml", "clojure",
		"elixir", "erlang", "nim", "zig", "vlang", "julia":
		return "source"
	case "html", "css", "scss", "sass", "less":
		return "web"
	case LanguageJSON, "yaml", "toml", "ini", "config":
		return "config"
	case "shell", "powershell", "makefile":
		return "script"
	case "sql":
		return "database"
	case "markdown", "latex":
		return "documentation"
	case "xml":
		return "markup"
	default:
		return FileTypeText
	}
}

// extractRepositoryContext extracts repository-wide context
func (pb *PayloadBuilder) extractRepositoryContext(repoPath string) (RepositoryContext, error) {
	context := RepositoryContext{}

	// Extract branch info
	branchInfo, err := pb.extractBranchInfo(repoPath)
	if err == nil {
		context.BranchInfo = branchInfo
	}

	// Extract commit info
	commitInfo, err := pb.extractCommitInfo(repoPath)
	if err == nil {
		context.CommitInfo = commitInfo
	}

	// Extract project info
	projectInfo, err := pb.extractProjectInfo(repoPath)
	if err == nil {
		context.ProjectInfo = projectInfo
	}

	return context, nil
}

// extractBranchInfo extracts information about branches involved in the merge
func (pb *PayloadBuilder) extractBranchInfo(repoPath string) (BranchInfo, error) {
	info := BranchInfo{}

	// Get current branch
	cmd := exec.Command("git", "symbolic-ref", "--short", "HEAD")
	cmd.Dir = repoPath
	if output, err := cmd.Output(); err == nil {
		info.CurrentBranch = strings.TrimSpace(string(output))
	}

	// Get merge head branch name
	cmd = exec.Command("git", "symbolic-ref", "--short", "MERGE_HEAD")
	cmd.Dir = repoPath
	if output, err := cmd.Output(); err == nil {
		info.MergeBranch = strings.TrimSpace(string(output))
	}

	// Get merge base
	cmd = exec.Command("git", "merge-base", "HEAD", "MERGE_HEAD")
	cmd.Dir = repoPath
	if output, err := cmd.Output(); err == nil {
		info.MergeBase = strings.TrimSpace(string(output))
	}

	return info, nil
}

// extractCommitInfo extracts commit information
func (pb *PayloadBuilder) extractCommitInfo(repoPath string) (CommitInfo, error) {
	info := CommitInfo{}

	// Get HEAD commit
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoPath
	if output, err := cmd.Output(); err == nil {
		info.OursCommit = strings.TrimSpace(string(output))
	}

	// Get MERGE_HEAD commit
	cmd = exec.Command("git", "rev-parse", "MERGE_HEAD")
	cmd.Dir = repoPath
	if output, err := cmd.Output(); err == nil {
		info.TheirsCommit = strings.TrimSpace(string(output))
	}

	// Get merge base commit
	if info.OursCommit != "" && info.TheirsCommit != "" {
		cmd = exec.Command("git", "merge-base", info.OursCommit, info.TheirsCommit)
		cmd.Dir = repoPath
		if output, err := cmd.Output(); err == nil {
			info.BaseCommit = strings.TrimSpace(string(output))
		}
	}

	return info, nil
}

// extractProjectInfo extracts project-specific information
func (pb *PayloadBuilder) extractProjectInfo(repoPath string) (ProjectInfo, error) {
	info := ProjectInfo{
		Conventions: make(map[string]string),
	}

	// Detect primary language by counting files
	languageCounts := make(map[string]int)
	err := filepath.Walk(repoPath, func(path string, fileInfo os.FileInfo, err error) error {
		if err != nil || fileInfo.IsDir() {
			return nil
		}

		// Skip hidden and excluded directories
		if strings.Contains(path, "/.") || strings.Contains(path, "/node_modules/") ||
			strings.Contains(path, "/vendor/") || strings.Contains(path, "/.git/") {
			return nil
		}

		lang := DetectLanguage(path)
		if lang != FileTypeText {
			languageCounts[lang]++
		}
		return nil
	})
	// If filepath.Walk fails, continue with empty counts
	if err != nil {
		languageCounts = make(map[string]int)
	}

	// Find most common language
	maxCount := 0
	for lang, count := range languageCounts {
		if count > maxCount {
			maxCount = count
			info.Language = lang
		}
	}

	// Detect build tools and frameworks
	info.BuildTool = pb.detectBuildTool(repoPath)
	info.Framework = pb.detectFramework(repoPath, info.Language)

	// Find config files
	info.ConfigFiles = pb.findConfigFiles(repoPath)

	return info, nil
}

// detectBuildTool detects the build tool used in the project
func (pb *PayloadBuilder) detectBuildTool(repoPath string) string {
	buildFiles := map[string]string{
		"package.json":   "npm",
		"yarn.lock":      "yarn",
		"pnpm-lock.yaml": "pnpm",
		"Cargo.toml":     "cargo",
		"go.mod":         "go",
		"pom.xml":        "maven",
		"build.gradle":   "gradle",
		"Makefile":       "make",
		"CMakeLists.txt": "cmake",
		"setup.py":       "setuptools",
		"pyproject.toml": "poetry",
		"Pipfile":        "pipenv",
		"composer.json":  "composer",
		"Gemfile":        "bundler",
	}

	for file, tool := range buildFiles {
		if _, err := os.Stat(filepath.Join(repoPath, file)); err == nil {
			return tool
		}
	}

	return ""
}

// detectFramework detects the framework used based on language and files
func (pb *PayloadBuilder) detectFramework(repoPath, language string) string {
	switch language {
	case LanguageJavaScript, LanguageTypeScript:
		return pb.detectJSFramework(repoPath)
	case LanguagePython:
		return pb.detectPythonFramework(repoPath)
	case LanguageGo:
		return pb.detectGoFramework(repoPath)
	case LanguageJava:
		return pb.detectJavaFramework(repoPath)
	default:
		return ""
	}
}

// detectJSFramework detects JavaScript/TypeScript frameworks
func (pb *PayloadBuilder) detectJSFramework(repoPath string) string {
	packageJSONPath := filepath.Join(repoPath, "package.json")
	if content, err := os.ReadFile(packageJSONPath); err == nil {
		contentStr := string(content)
		if strings.Contains(contentStr, "\"react\"") {
			if strings.Contains(contentStr, "\"next\"") {
				return "next.js"
			}
			return "react"
		}
		if strings.Contains(contentStr, "\"vue\"") {
			if strings.Contains(contentStr, "\"nuxt\"") {
				return "nuxt.js"
			}
			return "vue"
		}
		if strings.Contains(contentStr, "\"angular\"") {
			return "angular"
		}
		if strings.Contains(contentStr, "\"express\"") {
			return "express"
		}
		if strings.Contains(contentStr, "\"svelte\"") {
			return "svelte"
		}
	}
	return ""
}

// detectPythonFramework detects Python frameworks
func (pb *PayloadBuilder) detectPythonFramework(repoPath string) string {
	// Check requirements files
	reqFiles := []string{"requirements.txt", "pyproject.toml", "Pipfile"}
	for _, reqFile := range reqFiles {
		if content, err := os.ReadFile(filepath.Join(repoPath, reqFile)); err == nil {
			contentStr := strings.ToLower(string(content))
			if strings.Contains(contentStr, "django") {
				return "django"
			}
			if strings.Contains(contentStr, "flask") {
				return "flask"
			}
			if strings.Contains(contentStr, "fastapi") {
				return "fastapi"
			}
			if strings.Contains(contentStr, "tornado") {
				return "tornado"
			}
		}
	}
	return ""
}

// detectGoFramework detects Go frameworks
func (pb *PayloadBuilder) detectGoFramework(repoPath string) string {
	if content, err := os.ReadFile(filepath.Join(repoPath, "go.mod")); err == nil {
		contentStr := string(content)
		if strings.Contains(contentStr, "github.com/gin-gonic/gin") {
			return "gin"
		}
		if strings.Contains(contentStr, "github.com/gorilla/mux") {
			return "gorilla"
		}
		if strings.Contains(contentStr, "github.com/labstack/echo") {
			return "echo"
		}
		if strings.Contains(contentStr, "github.com/gofiber/fiber") {
			return "fiber"
		}
	}
	return ""
}

// detectJavaFramework detects Java frameworks
func (pb *PayloadBuilder) detectJavaFramework(repoPath string) string {
	pomPath := filepath.Join(repoPath, "pom.xml")
	if content, err := os.ReadFile(pomPath); err == nil {
		contentStr := string(content)
		if strings.Contains(contentStr, "spring-boot") {
			return "spring-boot"
		}
		if strings.Contains(contentStr, "springframework") {
			return "spring"
		}
		if strings.Contains(contentStr, "micronaut") {
			return "micronaut"
		}
		if strings.Contains(contentStr, "quarkus") {
			return "quarkus"
		}
	}
	return ""
}

// findConfigFiles finds configuration files in the repository
func (pb *PayloadBuilder) findConfigFiles(repoPath string) []string {
	var configFiles []string

	configPatterns := []string{
		"*.json", "*.yaml", "*.yml", "*.toml", "*.ini", "*.cfg", "*.conf",
		"Dockerfile", ".dockerignore", "docker-compose.yml",
		".gitignore", ".gitattributes", ".editorconfig",
		"tsconfig.json", "jsconfig.json", "webpack.config.js",
		"babel.config.js", ".babelrc", ".eslintrc*", ".prettierrc*",
		"pytest.ini", "setup.cfg", "tox.ini", ".flake8",
		"Cargo.toml", "rust-toolchain", ".rustfmt.toml",
	}

	for _, pattern := range configPatterns {
		matches, err := filepath.Glob(filepath.Join(repoPath, pattern))
		if err != nil {
			// If glob pattern fails, skip this pattern
			continue
		}
		for _, match := range matches {
			relPath, err := filepath.Rel(repoPath, match)
			if err != nil {
				// If we can't get relative path, use the full path
				relPath = match
			}
			configFiles = append(configFiles, relPath)
		}
	}

	return configFiles
}

// extractFileMetadata extracts metadata for a specific file
func (pb *PayloadBuilder) extractFileMetadata(filePath, repoPath string, content []string) (FileMetadata, error) {
	metadata := FileMetadata{
		Encoding:    "utf-8", // Default assumption
		LineEndings: "lf",    // Default assumption
	}

	fullPath := filepath.Join(repoPath, filePath)

	// Get file size
	if stat, err := os.Stat(fullPath); err == nil {
		metadata.Size = stat.Size()
	}

	// Count lines
	metadata.LineCount = len(content)

	// Detect line endings from actual file content
	if file, err := os.Open(fullPath); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		if scanner.Scan() {
			_ = scanner.Text() // Read first line to check format but don't use it
			// Read raw bytes to check line endings
			if rawContent, err := os.ReadFile(fullPath); err == nil {
				rawStr := string(rawContent)
				if strings.Contains(rawStr, "\r\n") {
					metadata.LineEndings = "crlf"
				} else if strings.Contains(rawStr, "\r") {
					metadata.LineEndings = "cr"
				}
			}
		}
	}

	// Check if file has tests
	metadata.HasTests = pb.hasAssociatedTests(filePath, repoPath)

	// Check if file is generated
	metadata.IsGenerated = pb.isGeneratedFile(filePath, content)

	return metadata, nil
}

// hasAssociatedTests checks if there are test files for this source file
func (pb *PayloadBuilder) hasAssociatedTests(filePath, repoPath string) bool {
	baseName := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	dir := filepath.Dir(filePath)

	testPatterns := []string{
		fmt.Sprintf("%s_test.*", baseName),
		fmt.Sprintf("%s.test.*", baseName),
		fmt.Sprintf("test_%s.*", baseName),
		fmt.Sprintf("%s_spec.*", baseName),
		fmt.Sprintf("%s.spec.*", baseName),
	}

	for _, pattern := range testPatterns {
		matches, err := filepath.Glob(filepath.Join(repoPath, dir, pattern))
		if err != nil {
			// If glob pattern fails, skip this pattern
			continue
		}
		if len(matches) > 0 {
			return true
		}
	}

	return false
}

// isGeneratedFile checks if a file appears to be generated
func (pb *PayloadBuilder) isGeneratedFile(filePath string, content []string) bool {
	// Check filename patterns
	fileName := filepath.Base(filePath)
	generatedPatterns := []regexp.Regexp{
		*regexp.MustCompile(`(?i).*\.generated\..*`),
		*regexp.MustCompile(`(?i).*_generated\..*`),
		*regexp.MustCompile(`(?i).*\.gen\..*`),
		*regexp.MustCompile(`(?i).*_gen\..*`),
		*regexp.MustCompile(`(?i).*\.pb\..*`),
	}

	for _, pattern := range generatedPatterns {
		if pattern.MatchString(fileName) {
			return true
		}
	}

	// Check file content for generation markers
	if len(content) > 0 {
		first10Lines := content
		if len(content) > 10 {
			first10Lines = content[:10]
		}

		for _, line := range first10Lines {
			lower := strings.ToLower(line)
			if strings.Contains(lower, "auto-generated") ||
				strings.Contains(lower, "automatically generated") ||
				strings.Contains(lower, "do not edit") ||
				strings.Contains(lower, "generated by") ||
				strings.Contains(lower, "code generated") {
				return true
			}
		}
	}

	return false
}
