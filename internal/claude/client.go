package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// ClaudeClient represents a client for interacting with Claude Code CLI
type ClaudeClient struct {
	config       *Config
	sessionID    string
	isAvailable  bool
	lastChecked  time.Time
	checkTimeout time.Duration
}

// Config contains configuration for the Claude Code CLI client
type Config struct {
	// CLIPath is the path to the Claude Code CLI executable
	CLIPath string

	// MaxTurns limits the number of conversation turns
	MaxTurns int

	// TimeoutSeconds is the timeout for individual CLI calls
	TimeoutSeconds int

	// AllowedTools restricts which tools Claude can use
	AllowedTools []string

	// OutputFormat specifies the output format (json, text)
	OutputFormat string

	// WorkingDirectory sets the working directory for Claude CLI
	WorkingDirectory string

	// PrintMode enables non-interactive mode (-p flag)
	PrintMode bool

	// Verbose enables verbose logging
	Verbose bool
}

// ClaudeCommand represents a command to send to Claude
type ClaudeCommand struct {
	Prompt    string            `json:"prompt"`
	Context   string            `json:"context,omitempty"`
	Files     []string          `json:"files,omitempty"`
	Options   map[string]string `json:"options,omitempty"`
	SessionID string            `json:"session_id,omitempty"`
}

// ClaudeResponse represents the response from Claude Code CLI
type ClaudeResponse struct {
	Success      bool                   `json:"success"`
	Content      string                 `json:"content"`
	Actions      []ClaudeAction         `json:"actions,omitempty"`
	SessionID    string                 `json:"session_id,omitempty"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// ClaudeAction represents an action performed by Claude
type ClaudeAction struct {
	Type      string                 `json:"type"`      // "read", "write", "bash"
	Target    string                 `json:"target"`    // file path or command
	Content   string                 `json:"content"`   // content for write actions
	Result    string                 `json:"result"`    // result of the action
	Success   bool                   `json:"success"`   // whether the action succeeded
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// NewClaudeClient creates a new Claude Code CLI client
func NewClaudeClient(config *Config) (*ClaudeClient, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Validate configuration
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	client := &ClaudeClient{
		config:       config,
		checkTimeout: 30 * time.Second,
	}

	// Check if Claude CLI is available
	if err := client.checkAvailability(); err != nil {
		return nil, fmt.Errorf("Claude Code CLI not available: %w", err)
	}

	return client, nil
}

// DefaultConfig returns a default configuration for Claude Code CLI with optimized settings for large repositories
func DefaultConfig() *Config {
	return &Config{
		CLIPath:          "claude", // Assume it's in PATH
		MaxTurns:         10, // Increased for complex conflicts and batching
		TimeoutSeconds:   300, // Extended timeout for large repository processing
		AllowedTools:     []string{"Read", "Write", "Edit", "MultiEdit", "Bash", "Grep", "Glob", "LS"}, // Enhanced tools for efficient conflict resolution
		OutputFormat:     "json",
		PrintMode:        true, // Non-interactive mode for automation
		Verbose:          false,
		WorkingDirectory: "",
	}
}

// validateConfig validates the client configuration
func validateConfig(config *Config) error {
	if config.CLIPath == "" {
		return fmt.Errorf("CLI path cannot be empty")
	}

	if config.MaxTurns <= 0 {
		return fmt.Errorf("max turns must be positive")
	}

	if config.TimeoutSeconds <= 0 {
		return fmt.Errorf("timeout must be positive")
	}

	validFormats := []string{"json", "text"}
	formatValid := false
	for _, format := range validFormats {
		if config.OutputFormat == format {
			formatValid = true
			break
		}
	}
	if !formatValid {
		return fmt.Errorf("invalid output format: %s", config.OutputFormat)
	}

	return nil
}

// checkAvailability checks if Claude Code CLI is available and functional
func (c *ClaudeClient) checkAvailability() error {
	// Skip check if recently checked
	if time.Since(c.lastChecked) < c.checkTimeout && c.isAvailable {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.config.CLIPath, "--version")
	output, err := cmd.Output()
	if err != nil {
		c.isAvailable = false
		return fmt.Errorf("failed to execute claude --version: %w", err)
	}

	// Validate that this is actually Claude Code CLI
	outputStr := string(output)
	if !strings.Contains(strings.ToLower(outputStr), "claude") {
		c.isAvailable = false
		return fmt.Errorf("unexpected version output: %s", outputStr)
	}

	c.isAvailable = true
	c.lastChecked = time.Now()

	if c.config.Verbose {
		fmt.Printf("Claude Code CLI version: %s", strings.TrimSpace(outputStr))
	}

	return nil
}

// IsAvailable returns whether Claude Code CLI is available
func (c *ClaudeClient) IsAvailable() bool {
	if err := c.checkAvailability(); err != nil {
		return false
	}
	return c.isAvailable
}

// ExecuteCommand executes a command using Claude Code CLI
func (c *ClaudeClient) ExecuteCommand(ctx context.Context, command *ClaudeCommand) (*ClaudeResponse, error) {
	if !c.IsAvailable() {
		return nil, fmt.Errorf("Claude Code CLI is not available")
	}

	// Build command arguments
	args := c.buildCommandArgs(command)

	// Create the command with context for timeout handling
	cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(c.config.TimeoutSeconds)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, c.config.CLIPath, args...)

	// Set working directory if specified
	if c.config.WorkingDirectory != "" {
		cmd.Dir = c.config.WorkingDirectory
	}

	// Set up stdin for the prompt
	cmd.Stdin = strings.NewReader(command.Prompt)

	if c.config.Verbose {
		fmt.Printf("Executing Claude CLI: %s %s\n", c.config.CLIPath, strings.Join(args, " "))
	}

	// Execute the command
	output, err := cmd.Output()
	if err != nil {
		return &ClaudeResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Claude CLI execution failed: %v", err),
		}, err
	}

	// Parse the response
	response, err := c.parseResponse(output)
	if err != nil {
		return &ClaudeResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to parse Claude response: %v", err),
		}, err
	}

	// Update session ID if provided
	if response.SessionID != "" {
		c.sessionID = response.SessionID
	}

	return response, nil
}

// buildCommandArgs builds command line arguments for Claude CLI
func (c *ClaudeClient) buildCommandArgs(command *ClaudeCommand) []string {
	var args []string

	// Add print mode flag for non-interactive operation
	if c.config.PrintMode {
		args = append(args, "-p")
	}

	// Add output format
	if c.config.OutputFormat != "" {
		args = append(args, "--output-format", c.config.OutputFormat)
	}

	// Add max turns
	if c.config.MaxTurns > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", c.config.MaxTurns))
	}

	// Add allowed tools
	if len(c.config.AllowedTools) > 0 {
		args = append(args, "--allowed-tools", strings.Join(c.config.AllowedTools, ","))
	}

	// Add session ID if we have one
	if c.sessionID != "" {
		args = append(args, "--session-id", c.sessionID)
	} else if command.SessionID != "" {
		args = append(args, "--session-id", command.SessionID)
	}

	// Add context if provided
	if command.Context != "" {
		args = append(args, "--context", command.Context)
	}

	// Add files if provided
	for _, file := range command.Files {
		args = append(args, "--file", file)
	}

	// Add any additional options
	for key, value := range command.Options {
		args = append(args, "--"+key, value)
	}

	return args
}

// parseResponse parses the output from Claude Code CLI
func (c *ClaudeClient) parseResponse(output []byte) (*ClaudeResponse, error) {
	if c.config.OutputFormat == "json" {
		return c.parseJSONResponse(output)
	}
	return c.parseTextResponse(output)
}

// parseJSONResponse parses JSON output from Claude
func (c *ClaudeClient) parseJSONResponse(output []byte) (*ClaudeResponse, error) {
	var response ClaudeResponse
	if err := json.Unmarshal(output, &response); err != nil {
		// If JSON parsing fails, treat as text response
		return &ClaudeResponse{
			Success: true,
			Content: string(output),
		}, nil
	}
	return &response, nil
}

// parseTextResponse parses text output from Claude
func (c *ClaudeClient) parseTextResponse(output []byte) (*ClaudeResponse, error) {
	content := string(output)
	
	// Extract actions from text output using patterns
	actions := c.extractActionsFromText(content)
	
	return &ClaudeResponse{
		Success: true,
		Content: content,
		Actions: actions,
	}, nil
}

// extractActionsFromText extracts actions from Claude's text output
func (c *ClaudeClient) extractActionsFromText(content string) []ClaudeAction {
	var actions []ClaudeAction
	
	// Look for common patterns that indicate Claude performed actions
	scanner := bufio.NewScanner(strings.NewReader(content))
	
	for scanner.Scan() {
		line := scanner.Text()
		
		// Pattern for file reads: "Reading file: path/to/file"
		if readMatch := regexp.MustCompile(`(?i)reading\s+file:?\s+(.+)`).FindStringSubmatch(line); readMatch != nil {
			actions = append(actions, ClaudeAction{
				Type:      "read",
				Target:    strings.TrimSpace(readMatch[1]),
				Success:   true,
				Timestamp: time.Now(),
			})
		}
		
		// Pattern for file writes: "Writing to file: path/to/file"
		if writeMatch := regexp.MustCompile(`(?i)writing\s+to\s+file:?\s+(.+)`).FindStringSubmatch(line); writeMatch != nil {
			actions = append(actions, ClaudeAction{
				Type:      "write",
				Target:    strings.TrimSpace(writeMatch[1]),
				Success:   true,
				Timestamp: time.Now(),
			})
		}
		
		// Pattern for bash commands: "Executing: command"
		if bashMatch := regexp.MustCompile(`(?i)executing:?\s+(.+)`).FindStringSubmatch(line); bashMatch != nil {
			actions = append(actions, ClaudeAction{
				Type:      "bash",
				Target:    strings.TrimSpace(bashMatch[1]),
				Success:   true,
				Timestamp: time.Now(),
			})
		}
	}
	
	return actions
}

// ExecuteConflictResolution executes a conflict resolution command
func (c *ClaudeClient) ExecuteConflictResolution(ctx context.Context, prompt string, contextData map[string]interface{}) (*ClaudeResponse, error) {
	// Build context string from data
	contextStr := c.buildContextString(contextData)
	
	command := &ClaudeCommand{
		Prompt:  prompt,
		Context: contextStr,
		Options: map[string]string{
			"task-type": "conflict-resolution",
		},
	}
	
	return c.ExecuteCommand(ctx, command)
}

// buildContextString builds a context string from provided data
func (c *ClaudeClient) buildContextString(contextData map[string]interface{}) string {
	if len(contextData) == 0 {
		return ""
	}
	
	var parts []string
	
	// Add repository information
	if repoPath, ok := contextData["repo_path"].(string); ok && repoPath != "" {
		parts = append(parts, fmt.Sprintf("Repository: %s", repoPath))
	}
	
	// Add conflict information
	if conflictCount, ok := contextData["conflict_count"].(int); ok && conflictCount > 0 {
		parts = append(parts, fmt.Sprintf("Total conflicts: %d", conflictCount))
	}
	
	// Add file information
	if files, ok := contextData["files"].([]string); ok && len(files) > 0 {
		parts = append(parts, fmt.Sprintf("Affected files: %s", strings.Join(files, ", ")))
	}
	
	return strings.Join(parts, "\n")
}

// StartSession starts a new Claude session
func (c *ClaudeClient) StartSession(ctx context.Context) (string, error) {
	// For now, we'll generate a simple session ID
	// In a more sophisticated implementation, this could initialize a persistent session
	sessionID := fmt.Sprintf("syncwright-%d", time.Now().Unix())
	c.sessionID = sessionID
	return sessionID, nil
}

// EndSession ends the current Claude session
func (c *ClaudeClient) EndSession(ctx context.Context) error {
	c.sessionID = ""
	return nil
}

// GetSessionID returns the current session ID
func (c *ClaudeClient) GetSessionID() string {
	return c.sessionID
}

// ExecuteWithRetry executes a command with enhanced retry logic and intelligent backoff
func (c *ClaudeClient) ExecuteWithRetry(ctx context.Context, command *ClaudeCommand, maxRetries int) (*ClaudeResponse, error) {
	var lastErr error
	
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			if c.config.Verbose {
				fmt.Printf("Retrying Claude CLI command (attempt %d/%d)\n", attempt+1, maxRetries+1)
			}
			
			// Intelligent backoff based on error type
			backoff := c.calculateBackoff(attempt, lastErr)
			
			if c.config.Verbose {
				fmt.Printf("Waiting %v before retry...\n", backoff)
			}
			
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}
		
		response, err := c.ExecuteCommand(ctx, command)
		if err == nil && response.Success {
			return response, nil
		}
		
		lastErr = err
		
		// Check if we should retry based on the error
		if !c.shouldRetry(err) {
			break
		}
	}
	
	return nil, fmt.Errorf("command failed after %d attempts: %w", maxRetries+1, lastErr)
}

// calculateBackoff calculates intelligent backoff duration based on error type and attempt number
func (c *ClaudeClient) calculateBackoff(attempt int, err error) time.Duration {
	baseDelay := time.Second
	
	if err != nil {
		errStr := strings.ToLower(err.Error())
		
		// Longer backoff for rate limiting
		if strings.Contains(errStr, "rate limit") || 
		   strings.Contains(errStr, "too many requests") ||
		   strings.Contains(errStr, "429") ||
		   strings.Contains(errStr, "throttled") {
			baseDelay = 5 * time.Second
		}
		
		// Moderate backoff for server errors
		if strings.Contains(errStr, "500") || 
		   strings.Contains(errStr, "502") || 
		   strings.Contains(errStr, "503") || 
		   strings.Contains(errStr, "504") {
			baseDelay = 2 * time.Second
		}
	}
	
	// Exponential backoff with jitter
	exponential := time.Duration(1<<uint(attempt)) * baseDelay
	
	// Add random jitter (±25%) to prevent thundering herd
	jitter := time.Duration(float64(exponential) * (0.75 + 0.5*rand.Float64()))
	
	// Cap maximum backoff at 30 seconds
	maxBackoff := 30 * time.Second
	if jitter > maxBackoff {
		jitter = maxBackoff
	}
	
	return jitter
}

// shouldRetry determines if an error is retryable, with enhanced rate limiting detection
func (c *ClaudeClient) shouldRetry(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := strings.ToLower(err.Error())
	
	// Retry on timeout errors
	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded") {
		return true
	}
	
	// Retry on temporary network issues
	if strings.Contains(errStr, "connection") || strings.Contains(errStr, "network") {
		return true
	}
	
	// Retry on rate limiting (common API rate limit indicators)
	if strings.Contains(errStr, "rate limit") || 
	   strings.Contains(errStr, "too many requests") ||
	   strings.Contains(errStr, "429") ||
	   strings.Contains(errStr, "quota exceeded") ||
	   strings.Contains(errStr, "throttled") {
		return true
	}
	
	// Retry on temporary server errors
	if strings.Contains(errStr, "500") || 
	   strings.Contains(errStr, "502") || 
	   strings.Contains(errStr, "503") || 
	   strings.Contains(errStr, "504") ||
	   strings.Contains(errStr, "internal server error") ||
	   strings.Contains(errStr, "service unavailable") {
		return true
	}
	
	// Don't retry on validation or configuration errors
	if strings.Contains(errStr, "invalid") || 
	   strings.Contains(errStr, "not found") ||
	   strings.Contains(errStr, "unauthorized") ||
	   strings.Contains(errStr, "forbidden") ||
	   strings.Contains(errStr, "400") ||
	   strings.Contains(errStr, "401") ||
	   strings.Contains(errStr, "403") {
		return false
	}
	
	return false
}

// ValidateClaudeCodeCLI validates that Claude Code CLI is available and functional
func ValidateClaudeCodeCLI(cliPath string) error {
	if cliPath == "" {
		cliPath = "claude"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, cliPath, "--version")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("Claude Code CLI not found at '%s': %w", cliPath, err)
	}

	outputStr := string(output)
	if !strings.Contains(strings.ToLower(outputStr), "claude") {
		return fmt.Errorf("invalid Claude CLI response: %s", outputStr)
	}

	return nil
}

// GetClaudeVersion returns the version of Claude Code CLI
func GetClaudeVersion(cliPath string) (string, error) {
	if cliPath == "" {
		cliPath = "claude"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, cliPath, "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get Claude version: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// TestClaudeCodeCLI performs a comprehensive test of Claude Code CLI functionality
func TestClaudeCodeCLI(cliPath string) error {
	if err := ValidateClaudeCodeCLI(cliPath); err != nil {
		return err
	}

	// Test basic command execution
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if cliPath == "" {
		cliPath = "claude"
	}

	// Test help command
	cmd := exec.CommandContext(ctx, cliPath, "--help")
	_, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("Claude CLI help command failed: %w", err)
	}

	// Test print mode (non-interactive)
	cmd = exec.CommandContext(ctx, cliPath, "-p", "Hello, this is a test")
	cmd.Stdin = strings.NewReader("Hello, this is a test")
	_, err = cmd.Output()
	if err != nil {
		return fmt.Errorf("Claude CLI print mode test failed: %w", err)
	}

	return nil
}

// Close cleans up the client
func (c *ClaudeClient) Close() error {
	return c.EndSession(context.Background())
}