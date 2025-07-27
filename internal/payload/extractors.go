package payload

import (
	"regexp"
	"strings"
)

// GoContextExtractor extracts Go-specific context
type GoContextExtractor struct{}

// NewGoContextExtractor creates a new Go context extractor
func NewGoContextExtractor() *GoContextExtractor {
	return &GoContextExtractor{}
}

// ExtractContext extracts Go-specific context from file content
func (g *GoContextExtractor) ExtractContext(filePath string, content []string) FileContext {
	context := FileContext{}

	var imports []string
	var functions []string
	var classes []string // Go doesn't have classes, but we'll use this for types/structs

	regexes := g.initializeRegexes()
	inImportBlock := false

	for i, line := range content {
		trimmed := strings.TrimSpace(line)

		// Handle import blocks
		if g.handleImportBlock(trimmed, &inImportBlock, &imports) {
			continue
		}

		// Process single-line patterns
		g.processSingleLinePatterns(line, regexes, &imports, &functions, &classes)

		// Add significant lines as context
		if i < 10 || strings.Contains(trimmed, "package ") {
			context.BeforeLines = append(context.BeforeLines, line)
		}
	}

	context.Imports = imports
	context.Functions = functions
	context.Classes = classes

	return context
}

// initializeRegexes creates all the regex patterns used for parsing
func (g *GoContextExtractor) initializeRegexes() map[string]*regexp.Regexp {
	return map[string]*regexp.Regexp{
		"import": regexp.MustCompile(`^\s*import\s+(?:"([^"]+)"|(\w+)\s+"([^"]+)")`),
		"func":   regexp.MustCompile(`^\s*func\s+(?:\([^)]*\)\s+)?(\w+)\s*\(`),
		"type":   regexp.MustCompile(`^\s*type\s+(\w+)\s+(?:struct|interface)`),
	}
}

// handleImportBlock processes import block lines
func (g *GoContextExtractor) handleImportBlock(trimmed string, inImportBlock *bool, imports *[]string) bool {
	if strings.HasPrefix(trimmed, "import (") {
		*inImportBlock = true
		return true
	}

	if *inImportBlock {
		if trimmed == ")" {
			*inImportBlock = false
			return true
		}

		if strings.Contains(trimmed, `"`) {
			g.extractImportPath(trimmed, imports)
		}
		return true
	}

	return false
}

// extractImportPath extracts import path from import line
func (g *GoContextExtractor) extractImportPath(trimmed string, imports *[]string) {
	start := strings.Index(trimmed, `"`)
	end := strings.LastIndex(trimmed, `"`)
	if start != -1 && end != -1 && start < end {
		importPath := trimmed[start+1 : end]
		*imports = append(*imports, importPath)
	}
}

// processSingleLinePatterns processes single-line patterns for imports, functions, and types
func (g *GoContextExtractor) processSingleLinePatterns(line string,
	regexes map[string]*regexp.Regexp, imports, functions, classes *[]string) {
	// Single-line imports
	if matches := regexes["import"].FindStringSubmatch(line); matches != nil {
		if matches[1] != "" {
			*imports = append(*imports, matches[1])
		} else if matches[3] != "" {
			*imports = append(*imports, matches[3])
		}
	}

	// Functions
	if matches := regexes["func"].FindStringSubmatch(line); matches != nil {
		*functions = append(*functions, matches[1])
	}

	// Types/Structs
	if matches := regexes["type"].FindStringSubmatch(line); matches != nil {
		*classes = append(*classes, matches[1])
	}
}

// GetLanguage returns the language name
func (g *GoContextExtractor) GetLanguage() string {
	return "go"
}

// JavaScriptContextExtractor extracts JavaScript-specific context
type JavaScriptContextExtractor struct{}

// NewJavaScriptContextExtractor creates a new JavaScript context extractor
func NewJavaScriptContextExtractor() *JavaScriptContextExtractor {
	return &JavaScriptContextExtractor{}
}

// ExtractContext extracts JavaScript-specific context
func (j *JavaScriptContextExtractor) ExtractContext(filePath string, content []string) FileContext {
	context := FileContext{}

	var imports []string
	var functions []string
	var classes []string

	// Regex patterns for JavaScript/ES6+
	importRegex := regexp.MustCompile(`^\s*import\s+.*from\s+['"]([^'"]+)['"]`)
	requireRegex := regexp.MustCompile(`require\s*\(\s*['"]([^'"]+)['"]\s*\)`)
	funcRegex := regexp.MustCompile(`^\s*(?:export\s+)?(?:async\s+)?function\s+(\w+)\s*\(`)
	arrowFuncRegex := regexp.MustCompile(`^\s*(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s+)?\(.*\)\s*=>`)
	classRegex := regexp.MustCompile(`^\s*(?:export\s+)?class\s+(\w+)`)

	for i, line := range content {
		// ES6 imports
		if matches := importRegex.FindStringSubmatch(line); matches != nil {
			imports = append(imports, matches[1])
		}

		// CommonJS requires
		if matches := requireRegex.FindStringSubmatch(line); matches != nil {
			imports = append(imports, matches[1])
		}

		// Function declarations
		if matches := funcRegex.FindStringSubmatch(line); matches != nil {
			functions = append(functions, matches[1])
		}

		// Arrow functions
		if matches := arrowFuncRegex.FindStringSubmatch(line); matches != nil {
			functions = append(functions, matches[1])
		}

		// Classes
		if matches := classRegex.FindStringSubmatch(line); matches != nil {
			classes = append(classes, matches[1])
		}

		// Add initial lines as context
		if i < 10 {
			context.BeforeLines = append(context.BeforeLines, line)
		}
	}

	context.Imports = imports
	context.Functions = functions
	context.Classes = classes

	return context
}

// GetLanguage returns the language name
func (j *JavaScriptContextExtractor) GetLanguage() string {
	return "javascript"
}

// TypeScriptContextExtractor extracts TypeScript-specific context
type TypeScriptContextExtractor struct{}

// NewTypeScriptContextExtractor creates a new TypeScript context extractor
func NewTypeScriptContextExtractor() *TypeScriptContextExtractor {
	return &TypeScriptContextExtractor{}
}

// ExtractContext extracts TypeScript-specific context
func (t *TypeScriptContextExtractor) ExtractContext(filePath string, content []string) FileContext {
	// Use JavaScript extractor as base
	jsExtractor := NewJavaScriptContextExtractor()
	context := jsExtractor.ExtractContext(filePath, content)

	// Add TypeScript-specific patterns
	var additionalClasses []string

	interfaceRegex := regexp.MustCompile(`^\s*(?:export\s+)?interface\s+(\w+)`)
	typeRegex := regexp.MustCompile(`^\s*(?:export\s+)?type\s+(\w+)\s*=`)
	enumRegex := regexp.MustCompile(`^\s*(?:export\s+)?enum\s+(\w+)`)

	for _, line := range content {
		// Interfaces
		if matches := interfaceRegex.FindStringSubmatch(line); matches != nil {
			additionalClasses = append(additionalClasses, matches[1])
		}

		// Type aliases
		if matches := typeRegex.FindStringSubmatch(line); matches != nil {
			additionalClasses = append(additionalClasses, matches[1])
		}

		// Enums
		if matches := enumRegex.FindStringSubmatch(line); matches != nil {
			additionalClasses = append(additionalClasses, matches[1])
		}
	}

	context.Classes = append(context.Classes, additionalClasses...)

	return context
}

// GetLanguage returns the language name
func (t *TypeScriptContextExtractor) GetLanguage() string {
	return "typescript"
}

// PythonContextExtractor extracts Python-specific context
type PythonContextExtractor struct{}

// NewPythonContextExtractor creates a new Python context extractor
func NewPythonContextExtractor() *PythonContextExtractor {
	return &PythonContextExtractor{}
}

// ExtractContext extracts Python-specific context
func (p *PythonContextExtractor) ExtractContext(filePath string, content []string) FileContext {
	context := FileContext{}

	var imports []string
	var functions []string
	var classes []string

	regexes := p.initializePythonRegexes()

	for i, line := range content {
		p.processImportLine(line, regexes.importRegex, &imports)
		p.processFunctionLine(line, regexes.funcRegex, regexes.asyncFuncRegex, &functions)
		p.processClassLine(line, regexes.classRegex, &classes)

		// Add initial lines as context
		if i < 10 {
			context.BeforeLines = append(context.BeforeLines, line)
		}
	}

	context.Imports = imports
	context.Functions = functions
	context.Classes = classes

	return context
}

// pythonRegexes holds compiled regex patterns for Python parsing
type pythonRegexes struct {
	importRegex    *regexp.Regexp
	funcRegex      *regexp.Regexp
	classRegex     *regexp.Regexp
	asyncFuncRegex *regexp.Regexp
}

// initializePythonRegexes creates regex patterns for Python code parsing
func (p *PythonContextExtractor) initializePythonRegexes() pythonRegexes {
	return pythonRegexes{
		importRegex:    regexp.MustCompile(`^\s*(?:from\s+(\S+)\s+)?import\s+(.+)`),
		funcRegex:      regexp.MustCompile(`^\s*def\s+(\w+)\s*\(`),
		classRegex:     regexp.MustCompile(`^\s*class\s+(\w+)`),
		asyncFuncRegex: regexp.MustCompile(`^\s*async\s+def\s+(\w+)\s*\(`),
	}
}

// processImportLine processes import statements
func (p *PythonContextExtractor) processImportLine(line string, importRegex *regexp.Regexp, imports *[]string) {
	matches := importRegex.FindStringSubmatch(line)
	if matches == nil {
		return
	}

	if matches[1] != "" {
		// from module import something
		*imports = append(*imports, matches[1])
	} else {
		// import something
		p.processImportParts(matches[2], imports)
	}
}

// processImportParts processes comma-separated import parts
func (p *PythonContextExtractor) processImportParts(importStr string, imports *[]string) {
	importParts := strings.Split(importStr, ",")
	for _, part := range importParts {
		part = strings.TrimSpace(part)
		if strings.Contains(part, " as ") {
			part = strings.Split(part, " as ")[0]
		}
		*imports = append(*imports, strings.TrimSpace(part))
	}
}

// processFunctionLine processes function definitions
func (p *PythonContextExtractor) processFunctionLine(line string,
	funcRegex, asyncFuncRegex *regexp.Regexp, functions *[]string) {
	// Regular functions
	if matches := funcRegex.FindStringSubmatch(line); matches != nil {
		*functions = append(*functions, matches[1])
		return
	}

	// Async functions
	if matches := asyncFuncRegex.FindStringSubmatch(line); matches != nil {
		*functions = append(*functions, matches[1])
	}
}

// processClassLine processes class definitions
func (p *PythonContextExtractor) processClassLine(line string, classRegex *regexp.Regexp, classes *[]string) {
	if matches := classRegex.FindStringSubmatch(line); matches != nil {
		*classes = append(*classes, matches[1])
	}
}

// GetLanguage returns the language name
func (p *PythonContextExtractor) GetLanguage() string {
	return "python"
}

// JavaContextExtractor extracts Java-specific context
type JavaContextExtractor struct{}

// NewJavaContextExtractor creates a new Java context extractor
func NewJavaContextExtractor() *JavaContextExtractor {
	return &JavaContextExtractor{}
}

// ExtractContext extracts Java-specific context
func (j *JavaContextExtractor) ExtractContext(filePath string, content []string) FileContext {
	context := FileContext{}

	var imports []string
	var functions []string
	var classes []string

	packageRegex := regexp.MustCompile(`^\s*package\s+([^;]+);`)
	importRegex := regexp.MustCompile(`^\s*import\s+(?:static\s+)?([^;]+);`)
	methodRegex := regexp.MustCompile(`^\s*(?:public|private|protected)?\s*(?:static\s+)?(?:final\s+)?\w+\s+(\w+)\s*\(`)
	classRegex := regexp.MustCompile(`^\s*(?:public\s+)?(?:abstract\s+)?(?:final\s+)?class\s+(\w+)`)
	interfaceRegex := regexp.MustCompile(`^\s*(?:public\s+)?interface\s+(\w+)`)

	for i, line := range content {
		// Package declaration
		if matches := packageRegex.FindStringSubmatch(line); matches != nil {
			context.BeforeLines = append(context.BeforeLines, line)
		}

		// Imports
		if matches := importRegex.FindStringSubmatch(line); matches != nil {
			imports = append(imports, matches[1])
		}

		// Methods
		if matches := methodRegex.FindStringSubmatch(line); matches != nil {
			functions = append(functions, matches[1])
		}

		// Classes
		if matches := classRegex.FindStringSubmatch(line); matches != nil {
			classes = append(classes, matches[1])
		}

		// Interfaces
		if matches := interfaceRegex.FindStringSubmatch(line); matches != nil {
			classes = append(classes, matches[1])
		}

		// Add initial lines as context
		if i < 15 {
			context.BeforeLines = append(context.BeforeLines, line)
		}
	}

	context.Imports = imports
	context.Functions = functions
	context.Classes = classes

	return context
}

// GetLanguage returns the language name
func (j *JavaContextExtractor) GetLanguage() string {
	return "java"
}

// GenericContextExtractor provides basic context extraction for any language
type GenericContextExtractor struct {
	language string
}

// NewGenericContextExtractor creates a new generic context extractor
func NewGenericContextExtractor(language string) *GenericContextExtractor {
	return &GenericContextExtractor{language: language}
}

// ExtractContext extracts basic context from any file
func (g *GenericContextExtractor) ExtractContext(filePath string, content []string) FileContext {
	context := FileContext{}

	// Add first 10 lines as before context
	maxLines := 10
	if len(content) < maxLines {
		maxLines = len(content)
	}

	context.BeforeLines = make([]string, maxLines)
	copy(context.BeforeLines, content[:maxLines])

	// Try to extract some basic patterns that might be functions or imports
	funcPatterns := []*regexp.Regexp{
		regexp.MustCompile(`^\s*def\s+(\w+)`),      // Python
		regexp.MustCompile(`^\s*function\s+(\w+)`), // JavaScript
		regexp.MustCompile(`^\s*func\s+(\w+)`),     // Go
		regexp.MustCompile(`^\s*\w+\s+(\w+)\s*\(`), // C/C++/Java style
	}

	importPatterns := []*regexp.Regexp{
		regexp.MustCompile(`^\s*import\s+(.+)`),
		regexp.MustCompile(`^\s*#include\s+(.+)`),
		regexp.MustCompile(`^\s*require\s*\(\s*(.+)\s*\)`),
		regexp.MustCompile(`^\s*use\s+(.+)`),
	}

	var functions []string
	var imports []string

	for _, line := range content {
		// Try to match functions
		for _, pattern := range funcPatterns {
			if matches := pattern.FindStringSubmatch(line); matches != nil {
				functions = append(functions, matches[1])
				break
			}
		}

		// Try to match imports
		for _, pattern := range importPatterns {
			if matches := pattern.FindStringSubmatch(line); matches != nil {
				imports = append(imports, strings.TrimSpace(matches[1]))
				break
			}
		}
	}

	context.Functions = functions
	context.Imports = imports

	return context
}

// GetLanguage returns the language name
func (g *GenericContextExtractor) GetLanguage() string {
	return g.language
}
