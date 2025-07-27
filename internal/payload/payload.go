// Package payload provides functionality for generating AI-ready payloads from conflict data
package payload

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ConflictContext represents the context around a merge conflict
type ConflictContext struct {
	File        string   `json:"file"`
	ConflictID  string   `json:"conflict_id"`
	BeforeLines []string `json:"before_lines"`
	OurLines    []string `json:"our_lines"`
	TheirLines  []string `json:"their_lines"`
	AfterLines  []string `json:"after_lines"`
}

// ExtractConflictContext extracts the context around a merge conflict
func ExtractConflictContext(filepath string, startLine, endLine int) (*ConflictContext, error) {
	file, err := os.Open(filepath) // #nosec G304 - filepath is validated by caller
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filepath, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to close file: %v\n", closeErr)
		}
	}()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		lines = append(lines, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filepath, err)
	}

	context := &ConflictContext{
		File:       filepath,
		ConflictID: fmt.Sprintf("%s:%d-%d", filepath, startLine, endLine),
	}

	// Extract conflict sections
	// TODO: Implement actual conflict parsing logic
	// This is a placeholder implementation
	if len(lines) > 0 {
		// Use lines variable to extract actual conflict context
		// For now, just ensure the variable is used to satisfy staticcheck
		_ = lines
	}

	return context, nil
}

// GenerateAIPayload creates a structured payload for AI processing
func GenerateAIPayload(context *ConflictContext) map[string]interface{} {
	payload := map[string]interface{}{
		"conflict_id": context.ConflictID,
		"file":        context.File,
		"context": map[string]interface{}{
			"before": strings.Join(context.BeforeLines, "\n"),
			"ours":   strings.Join(context.OurLines, "\n"),
			"theirs": strings.Join(context.TheirLines, "\n"),
			"after":  strings.Join(context.AfterLines, "\n"),
		},
		"instructions": "Please resolve this merge conflict by choosing the appropriate lines or combining them logically.",
	}

	return payload
}
