package payload

import (
	"path/filepath"
	"regexp"
	"strings"
)

// SensitiveFileFilter filters out files that may contain sensitive information
type SensitiveFileFilter struct {
	patterns []string
	regexes  []*regexp.Regexp
}

// NewSensitiveFileFilter creates a new sensitive file filter
func NewSensitiveFileFilter() *SensitiveFileFilter {
	patterns := []string{
		// Environment files
		".env",
		".env.*",
		"environment",
		"*.env",
		
		// Key files
		"*.key",
		"*.pem",
		"*.crt",
		"*.cert",
		"*.p12",
		"*.pfx",
		"*.jks",
		"*.keystore",
		"id_rsa",
		"id_dsa",
		"id_ecdsa",
		"id_ed25519",
		
		// Secret files
		"secrets.*",
		"*secret*",
		"*password*",
		"*credentials*",
		"*token*",
		
		// Configuration with sensitive data
		"*.vault",
		"vault.*",
		"*vault*",
		"ansible-vault",
		
		// Cloud provider files
		".aws/credentials",
		".azure/credentials",
		"gcloud/credentials",
		"service-account.json",
		"*service-account*.json",
		
		// Database connection files
		"*database.yml",
		"*database.yaml",
		"database_url",
		"*connection_string*",
		
		// API keys and tokens
		"*api_key*",
		"*access_token*",
		"*refresh_token*",
		"*bearer_token*",
	}
	
	filter := &SensitiveFileFilter{
		patterns: patterns,
		regexes:  make([]*regexp.Regexp, 0, len(patterns)),
	}
	
	// Compile patterns to regexes
	for _, pattern := range patterns {
		// Convert glob pattern to regex
		regex := globToRegex(pattern)
		compiled, err := regexp.Compile("(?i)" + regex) // Case insensitive
		if err == nil {
			filter.regexes = append(filter.regexes, compiled)
		}
	}
	
	return filter
}

// ShouldExclude returns true if the file should be excluded
func (f *SensitiveFileFilter) ShouldExclude(filePath string) bool {
	fileName := filepath.Base(filePath)
	lowerPath := strings.ToLower(filePath)
	lowerFileName := strings.ToLower(fileName)
	
	// Check against regex patterns
	for _, regex := range f.regexes {
		if regex.MatchString(lowerFileName) || regex.MatchString(lowerPath) {
			return true
		}
	}
	
	return false
}

// GetReason returns the reason for exclusion
func (f *SensitiveFileFilter) GetReason() string {
	return "contains sensitive information"
}

// BinaryFileFilter filters out binary files
type BinaryFileFilter struct {
	binaryExtensions map[string]bool
}

// NewBinaryFileFilter creates a new binary file filter
func NewBinaryFileFilter() *BinaryFileFilter {
	extensions := map[string]bool{
		// Images
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".bmp": true,
		".tiff": true, ".webp": true, ".svg": true, ".ico": true, ".tga": true,
		
		// Videos
		".mp4": true, ".avi": true, ".mov": true, ".wmv": true, ".flv": true,
		".webm": true, ".mkv": true, ".m4v": true, ".3gp": true,
		
		// Audio
		".mp3": true, ".wav": true, ".flac": true, ".aac": true, ".ogg": true,
		".wma": true, ".m4a": true, ".opus": true,
		
		// Archives
		".zip": true, ".tar": true, ".gz": true, ".bz2": true, ".7z": true,
		".rar": true, ".xz": true, ".lz4": true, ".zst": true,
		
		// Executables
		".exe": true, ".dll": true, ".so": true, ".dylib": true, ".app": true,
		".deb": true, ".rpm": true, ".msi": true, ".dmg": true, ".pkg": true,
		
		// Documents
		".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true,
		".ppt": true, ".pptx": true, ".odt": true, ".ods": true, ".odp": true,
		
		// Fonts
		".ttf": true, ".otf": true, ".woff": true, ".woff2": true, ".eot": true,
		
		// Databases
		".db": true, ".sqlite": true, ".sqlite3": true, ".mdb": true,
		
		// Other binary formats
		".bin": true, ".dat": true, ".pyc": true, ".pyo": true, ".class": true,
		".jar": true, ".war": true, ".o": true, ".obj": true, ".lib": true,
		".a": true, ".pdb": true, ".iso": true, ".img": true,
	}
	
	return &BinaryFileFilter{
		binaryExtensions: extensions,
	}
}

// ShouldExclude returns true if the file is binary
func (f *BinaryFileFilter) ShouldExclude(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	return f.binaryExtensions[ext]
}

// GetReason returns the reason for exclusion
func (f *BinaryFileFilter) GetReason() string {
	return "binary file"
}

// GeneratedFileFilter filters out generated files
type GeneratedFileFilter struct {
	patterns []string
	regexes  []*regexp.Regexp
}

// NewGeneratedFileFilter creates a new generated file filter
func NewGeneratedFileFilter() *GeneratedFileFilter {
	patterns := []string{
		// Build outputs
		"**/build/**",
		"**/dist/**",
		"**/target/**",
		"**/out/**",
		"**/bin/**",
		"**/obj/**",
		"**/.build/**",
		
		// Generated directories
		"**/generated/**",
		"**/gen/**",
		"**/__pycache__/**",
		"**/node_modules/**",
		"**/vendor/**",
		"**/.git/**",
		"**/.svn/**",
		"**/.hg/**",
		
		// Cache directories
		"**/.cache/**",
		"**/.tmp/**",
		"**/tmp/**",
		"**/temp/**",
		
		// IDE files
		"**/.vscode/**",
		"**/.idea/**",
		"**/*.iml",
		"**/.project",
		"**/.classpath",
		"**/.settings/**",
		
		// Generated source files (common patterns)
		"*.generated.*",
		"*_generated.*",
		"*.gen.*",
		"*_gen.*",
		"*.pb.go", // Protocol buffer generated Go files
		"*.pb.h",  // Protocol buffer generated C++ files
		"*.pb.cc",
		"*_pb2.py", // Protocol buffer generated Python files
		
		// Web build artifacts
		"**/*.min.js",
		"**/*.min.css",
		"**/bundle.js",
		"**/bundle.css",
		"**/webpack-assets.json",
		
		// Mobile artifacts
		"**/Pods/**",
		"**/DerivedData/**",
		"**/*.xcworkspace/**",
		"**/*.xcodeproj/**",
		
		// Documentation generated
		"**/docs/_build/**",
		"**/site/**", // mkdocs
		"**/_site/**", // Jekyll
	}
	
	filter := &GeneratedFileFilter{
		patterns: patterns,
		regexes:  make([]*regexp.Regexp, 0, len(patterns)),
	}
	
	// Compile patterns to regexes
	for _, pattern := range patterns {
		regex := globToRegex(pattern)
		compiled, err := regexp.Compile(regex)
		if err == nil {
			filter.regexes = append(filter.regexes, compiled)
		}
	}
	
	return filter
}

// ShouldExclude returns true if the file is generated
func (f *GeneratedFileFilter) ShouldExclude(filePath string) bool {
	lowerPath := strings.ToLower(filePath)
	
	for _, regex := range f.regexes {
		if regex.MatchString(lowerPath) {
			return true
		}
	}
	
	return false
}

// GetReason returns the reason for exclusion
func (f *GeneratedFileFilter) GetReason() string {
	return "generated file"
}

// LockfileFilter filters out lockfiles and dependency files
type LockfileFilter struct {
	lockfiles map[string]bool
	patterns  []string
	regexes   []*regexp.Regexp
}

// NewLockfileFilter creates a new lockfile filter
func NewLockfileFilter() *LockfileFilter {
	lockfiles := map[string]bool{
		// JavaScript/Node.js
		"package-lock.json": true,
		"yarn.lock":         true,
		"pnpm-lock.yaml":    true,
		"npm-shrinkwrap.json": true,
		
		// Python
		"Pipfile.lock":    true,
		"poetry.lock":     true,
		"requirements.txt": false, // Sometimes manually managed
		
		// Go
		"go.sum": true,
		"go.mod": false, // Sometimes manually managed
		
		// Rust
		"Cargo.lock": true,
		
		// Ruby
		"Gemfile.lock": true,
		
		// PHP
		"composer.lock": true,
		
		// .NET
		"packages.lock.json": true,
		"project.assets.json": true,
		
		// Java
		"gradle.lockfile": true,
		
		// Swift
		"Package.resolved": true,
	}
	
	patterns := []string{
		"**/node_modules/**",
		"**/vendor/**",
		"**/target/dependency-reduced-pom.xml",
		"**/.gradle/dependency-cache/**",
	}
	
	filter := &LockfileFilter{
		lockfiles: lockfiles,
		patterns:  patterns,
		regexes:   make([]*regexp.Regexp, 0, len(patterns)),
	}
	
	// Compile patterns
	for _, pattern := range patterns {
		regex := globToRegex(pattern)
		compiled, err := regexp.Compile(regex)
		if err == nil {
			filter.regexes = append(filter.regexes, compiled)
		}
	}
	
	return filter
}

// ShouldExclude returns true if the file is a lockfile
func (f *LockfileFilter) ShouldExclude(filePath string) bool {
	fileName := filepath.Base(filePath)
	
	// Check exact lockfile names
	if shouldExclude, exists := f.lockfiles[fileName]; exists && shouldExclude {
		return true
	}
	
	// Check patterns
	lowerPath := strings.ToLower(filePath)
	for _, regex := range f.regexes {
		if regex.MatchString(lowerPath) {
			return true
		}
	}
	
	return false
}

// GetReason returns the reason for exclusion
func (f *LockfileFilter) GetReason() string {
	return "lockfile or dependency file"
}

// globToRegex converts a glob pattern to a regex pattern
func globToRegex(glob string) string {
	// Escape regex special characters except * and ?
	regex := regexp.QuoteMeta(glob)
	
	// Replace escaped glob characters with regex equivalents
	regex = strings.ReplaceAll(regex, "\\*\\*", ".*")  // ** matches any path
	regex = strings.ReplaceAll(regex, "\\*", "[^/]*")  // * matches any file/dir name
	regex = strings.ReplaceAll(regex, "\\?", "[^/]")   // ? matches any single character
	
	// Anchor the regex
	regex = "^" + regex + "$"
	
	return regex
}

// DefaultSensitivePatterns returns default patterns for sensitive content
func DefaultSensitivePatterns() []string {
	return []string{
		"password",
		"passwd",
		"secret",
		"api_key",
		"apikey",
		"access_token",
		"auth_token",
		"bearer_token",
		"refresh_token",
		"private_key",
		"public_key",
		"certificate",
		"credential",
		"database_url",
		"connection_string",
		"redis_url",
		"mongodb_uri",
		"aws_access_key",
		"aws_secret",
		"gcp_key",
		"azure_key",
		"stripe_key",
		"paypal_key",
		"github_token",
		"gitlab_token",
		"bitbucket_token",
		"docker_password",
		"smtp_password",
		"mail_password",
		"jwt_secret",
		"session_secret",
		"encryption_key",
		"signing_key",
		"webhook_secret",
		"oauth_secret",
		"client_secret",
		"app_secret",
	}
}