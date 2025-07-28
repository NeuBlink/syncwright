package claude

import (
	"context"
	"fmt"
	"log"

	"github.com/NeuBlink/syncwright/internal/gitutils"
	"github.com/NeuBlink/syncwright/internal/payload"
)

// ExampleEnhancedGoConflictResolution demonstrates the enhanced Go-specific conflict resolution
func ExampleEnhancedGoConflictResolution(repoPath string, conflictFiles []string) error {
	// Configure the enhanced resolver with Go-specific optimizations
	config := &ConflictResolverConfig{
		ClaudeConfig: &Config{
			CLIPath:              "claude",
			MaxTurns:             5, // Allow more turns for complex Go conflicts
			TimeoutSeconds:       180, // Longer timeout for detailed analysis
			AllowedTools:         []string{"Read", "Write", "Bash(git*)"}, 
			OutputFormat:         "json",
			PrintMode:            true,
			Verbose:              true,
			WorkingDirectory:     repoPath,
		},
		RepoPath:            repoPath,
		MinConfidence:       0.7, // Higher threshold for Go code
		MaxBatchSize:        5,   // Smaller batches for more focused analysis
		IncludeReasoning:    true, // Essential for understanding Go semantics
		Verbose:             true,
		EnableMultiTurn:     true, // Enable multi-turn for complex Go conflicts
		MaxTurns:            3,    // Up to 3 refinement rounds
		MultiTurnThreshold:  0.6,  // Refine anything below 60% confidence
	}

	// Create the enhanced resolver
	resolver, err := NewConflictResolver(config)
	if err != nil {
		return fmt.Errorf("failed to create enhanced resolver: %w", err)
	}
	defer resolver.Close()

	// Verify Claude CLI is available
	if !resolver.IsAvailable() {
		return fmt.Errorf("Claude Code CLI is not available - please ensure it's installed and configured")
	}

	// Get conflict report
	report, err := gitutils.GetConflictReport(repoPath)
	if err != nil {
		return fmt.Errorf("failed to get conflict report: %w", err)
	}

	// Build enhanced payload with Go-specific context
	conflictPayload, err := payload.BuildSimplePayload(report)
	if err != nil {
		return fmt.Errorf("failed to build conflict payload: %w", err)
	}

	// Resolve conflicts with enhanced Go analysis
	ctx := context.Background()
	result, err := resolver.ResolveConflicts(ctx, conflictPayload)
	if err != nil {
		return fmt.Errorf("conflict resolution failed: %w", err)
	}

	// Display results with Go-specific insights
	fmt.Printf("\n=== ENHANCED GO CONFLICT RESOLUTION RESULTS ===\n")
	fmt.Printf("Total conflicts processed: %d\n", result.ProcessedConflicts)
	fmt.Printf("High confidence resolutions: %d\n", len(result.HighConfidence))
	fmt.Printf("Low confidence resolutions: %d\n", len(result.LowConfidence))
	fmt.Printf("Overall confidence: %.2f\n", result.OverallConfidence)
	fmt.Printf("Processing time: %v\n", result.ProcessingTime)

	// Detailed analysis of high-confidence resolutions
	if len(result.HighConfidence) > 0 {
		fmt.Printf("\n=== HIGH CONFIDENCE GO RESOLUTIONS ===\n")
		for _, resolution := range result.HighConfidence {
			fmt.Printf("\nFile: %s (lines %d-%d)\n", resolution.FilePath, resolution.StartLine, resolution.EndLine)
			fmt.Printf("Confidence: %.2f\n", resolution.Confidence)
			if resolution.Reasoning != "" {
				fmt.Printf("Reasoning: %s\n", resolution.Reasoning)
			}
			fmt.Printf("Resolved content:\n")
			for i, line := range resolution.ResolvedLines {
				fmt.Printf("  %d: %s\n", resolution.StartLine+i, line)
			}
		}
	}

	// Analysis of low-confidence resolutions that need manual review
	if len(result.LowConfidence) > 0 {
		fmt.Printf("\n=== LOW CONFIDENCE RESOLUTIONS (Manual Review Recommended) ===\n")
		for _, resolution := range result.LowConfidence {
			fmt.Printf("\nFile: %s (lines %d-%d) - Confidence: %.2f\n", 
				resolution.FilePath, resolution.StartLine, resolution.EndLine, resolution.Confidence)
			if resolution.Reasoning != "" {
				fmt.Printf("AI Analysis: %s\n", resolution.Reasoning)
			}
			fmt.Printf("Suggested resolution (requires review):\n")
			for i, line := range resolution.ResolvedLines {
				fmt.Printf("  %d: %s\n", resolution.StartLine+i, line)
			}
			
			// Provide Go-specific guidance for manual review
			fmt.Printf("Manual review checklist:\n")
			fmt.Printf("- Verify function signatures are correct and compatible\n")
			fmt.Printf("- Check import statements for consistency\n")
			fmt.Printf("- Ensure error handling follows Go idioms\n")
			fmt.Printf("- Validate type compatibility and interface satisfaction\n")
			fmt.Printf("- Test that the code compiles and passes existing tests\n")
		}
	}

	// Warnings and recommendations
	if len(result.Warnings) > 0 {
		fmt.Printf("\n=== WARNINGS ===\n")
		for _, warning := range result.Warnings {
			fmt.Printf("- %s\n", warning)
		}
	}

	// Success/failure summary
	if result.Success {
		fmt.Printf("\n✅ Enhanced Go conflict resolution completed successfully!\n")
		if len(result.LowConfidence) > 0 {
			fmt.Printf("⚠️  Manual review recommended for %d low-confidence resolutions\n", len(result.LowConfidence))
		}
	} else {
		fmt.Printf("\n❌ Conflict resolution encountered issues\n")
		if result.ErrorMessage != "" {
			fmt.Printf("Error: %s\n", result.ErrorMessage)
		}
	}

	return nil
}

// ValidateGoResolutionQuality performs additional validation on resolved Go code
func ValidateGoResolutionQuality(resolutions []gitutils.ConflictResolution, repoPath string) error {
	for _, resolution := range resolutions {
		if !isGoFile(resolution.FilePath) {
			continue
		}

		// Check if resolved code follows Go conventions
		if err := validateGoConventions(resolution); err != nil {
			log.Printf("Go convention warning for %s: %v", resolution.FilePath, err)
		}

		// Check for potential runtime issues
		if err := checkForRuntimeIssues(resolution); err != nil {
			log.Printf("Potential runtime issue in %s: %v", resolution.FilePath, err)
		}
	}

	return nil
}

// validateGoConventions checks if the resolution follows Go coding conventions
func validateGoConventions(resolution gitutils.ConflictResolution) error {
	code := fmt.Sprintf("%s", resolution.ResolvedLines)
	
	// Check naming conventions
	if err := checkNamingConventions(code); err != nil {
		return fmt.Errorf("naming convention issue: %w", err)
	}
	
	// Check for proper documentation
	if err := checkDocumentationPatterns(code); err != nil {
		return fmt.Errorf("documentation issue: %w", err)
	}
	
	return nil
}

// checkForRuntimeIssues identifies potential runtime issues in resolved Go code
func checkForRuntimeIssues(resolution gitutils.ConflictResolution) error {
	code := fmt.Sprintf("%s", resolution.ResolvedLines)
	
	// Check for potential nil pointer dereferences
	if hasNilPointerRisk(code) {
		return fmt.Errorf("potential nil pointer dereference detected")
	}
	
	// Check for potential goroutine leaks
	if hasGoroutineLeakRisk(code) {
		return fmt.Errorf("potential goroutine leak detected")
	}
	
	return nil
}

// Helper functions for validation
func isGoFile(filePath string) bool {
	return len(filePath) > 3 && filePath[len(filePath)-3:] == ".go"
}

func checkNamingConventions(code string) error {
	// Implementation would check Go naming conventions
	return nil
}

func checkDocumentationPatterns(code string) error {
	// Implementation would check for proper Go documentation patterns
	return nil
}

func hasNilPointerRisk(code string) bool {
	// Implementation would analyze code for nil pointer risks
	return false
}

func hasGoroutineLeakRisk(code string) bool {
	// Implementation would analyze code for goroutine leak risks
	return false
}