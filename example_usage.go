package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"syncwright/internal/commands"
	"syncwright/internal/gitutils"
	"syncwright/internal/payload"
)

// This file demonstrates how to use the Syncwright conflict resolution system

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	command := os.Args[1]

	switch command {
	case "detect":
		runDetectExample()
	case "ai-apply":
		runAIApplyExample()
	case "manual-test":
		runManualTest()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
	}
}

func printUsage() {
	fmt.Println("Syncwright Example Usage")
	fmt.Println("Usage: go run example_usage.go <command>")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  detect      - Detect conflicts in current repository")
	fmt.Println("  ai-apply    - Apply AI-suggested conflict resolutions")
	fmt.Println("  manual-test - Run manual test of git utilities")
}

// runDetectExample demonstrates conflict detection
func runDetectExample() {
	fmt.Println("=== Conflict Detection Example ===")

	// Get current working directory as repo path
	repoPath, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}

	// Run conflict detection
	result, err := commands.DetectConflictsVerbose(repoPath, "")
	if err != nil {
		log.Fatalf("Conflict detection failed: %v", err)
	}

	// Print results
	fmt.Printf("Detection successful: %t\n", result.Success)
	fmt.Printf("Total files: %d\n", result.Summary.TotalFiles)
	fmt.Printf("Total conflicts: %d\n", result.Summary.TotalConflicts)
	fmt.Printf("In merge state: %t\n", result.Summary.InMergeState)

	if result.ConflictPayload != nil {
		fmt.Printf("Processable files: %d\n", len(result.ConflictPayload.Files))

		for _, file := range result.ConflictPayload.Files {
			fmt.Printf("  File: %s (%s) - %d conflicts\n",
				file.Path, file.Language, len(file.Conflicts))
		}
	}

	if result.ErrorMessage != "" {
		fmt.Printf("Error: %s\n", result.ErrorMessage)
	}
}

// runAIApplyExample demonstrates AI conflict resolution
func runAIApplyExample() {
	fmt.Println("=== AI Resolution Example ===")

	// This is a dry-run example since we don't have a real API endpoint
	repoPath, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}

	// First, detect conflicts
	detectResult, err := commands.DetectConflicts(repoPath)
	if err != nil {
		log.Fatalf("Conflict detection failed: %v", err)
	}

	if !detectResult.Summary.InMergeState {
		fmt.Println("No conflicts detected - repository not in merge state")
		return
	}

	// Create a temporary payload file
	payloadFile := "/tmp/syncwright_payload.json"
	payloadData, err := detectResult.ConflictPayload.ToJSON()
	if err != nil {
		log.Fatalf("Failed to serialize payload: %v", err)
	}

	err = os.WriteFile(payloadFile, payloadData, 0644)
	if err != nil {
		log.Fatalf("Failed to write payload file: %v", err)
	}

	fmt.Printf("Payload written to: %s\n", payloadFile)

	// Run dry-run AI application
	result, err := commands.ApplyAIResolutionsDryRun(payloadFile, repoPath, "test-api-key")
	if err != nil {
		// Expected to fail since we don't have a real API endpoint
		fmt.Printf("AI application failed (expected): %v\n", err)
	} else {
		fmt.Printf("AI application successful: %t\n", result.Success)
		fmt.Printf("Processed files: %d\n", result.ProcessedFiles)
		fmt.Printf("Generated resolutions: %d\n", len(result.Resolutions))
	}

	// Clean up
	os.Remove(payloadFile)
}

// runManualTest demonstrates manual testing of git utilities
func runManualTest() {
	fmt.Println("=== Manual Git Utilities Test ===")

	repoPath, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}

	// Test 1: Check if in merge state
	inMerge, err := gitutils.IsInMergeState(repoPath)
	if err != nil {
		fmt.Printf("Failed to check merge state: %v\n", err)
	} else {
		fmt.Printf("In merge state: %t\n", inMerge)
	}

	// Test 2: Detect conflicts
	conflicts, err := gitutils.DetectConflicts(repoPath)
	if err != nil {
		fmt.Printf("Failed to detect conflicts: %v\n", err)
	} else {
		fmt.Printf("Found %d conflicted files\n", len(conflicts))
		for _, conflict := range conflicts {
			fmt.Printf("  %s (status: %s)\n", conflict.FilePath, conflict.Status)
		}
	}

	// Test 3: Test payload builder
	if len(conflicts) > 0 {
		report := &gitutils.ConflictReport{
			RepoPath:       repoPath,
			TotalConflicts: len(conflicts),
		}

		for _, conflict := range conflicts {
			hunks, err := gitutils.ParseConflictHunks(conflict.FilePath, repoPath)
			if err != nil {
				fmt.Printf("Failed to parse hunks for %s: %v\n", conflict.FilePath, err)
				continue
			}

			context, err := gitutils.ExtractFileContext(conflict.FilePath, repoPath, 5)
			if err != nil {
				fmt.Printf("Failed to extract context for %s: %v\n", conflict.FilePath, err)
				context = nil
			}

			conflictFile := gitutils.ConflictFile{
				Path:    conflict.FilePath,
				Hunks:   hunks,
				Context: context,
			}

			report.ConflictedFiles = append(report.ConflictedFiles, conflictFile)
		}

		// Test payload builder
		builder := payload.NewPayloadBuilder()
		payload, err := builder.BuildPayload(report)
		if err != nil {
			fmt.Printf("Failed to build payload: %v\n", err)
		} else {
			fmt.Printf("Built payload with %d files\n", len(payload.Files))

			// Print payload summary
			jsonData, err := json.MarshalIndent(payload.Metadata, "", "  ")
			if err == nil {
				fmt.Printf("Payload metadata:\n%s\n", string(jsonData))
			}
		}
	}

	// Test 4: Test file type detection
	testFiles := []string{
		"main.go",
		"package.json",
		"app.js",
		"style.css",
		"README.md",
		".env",
		"Dockerfile",
	}

	fmt.Println("\nFile type detection test:")
	for _, file := range testFiles {
		language := payload.DetectLanguage(file)
		fileType := payload.DetectFileType(file)
		fmt.Printf("  %s: language=%s, type=%s\n", file, language, fileType)
	}

	// Test 5: Test filters
	fmt.Println("\nFilter test:")
	sensitiveFilter := payload.NewSensitiveFileFilter()
	binaryFilter := payload.NewBinaryFileFilter()
	lockfileFilter := payload.NewLockfileFilter()

	testPaths := []string{
		".env",
		"package-lock.json",
		"image.png",
		"main.go",
		"secrets.txt",
		"node_modules/package/index.js",
	}

	for _, path := range testPaths {
		fmt.Printf("  %s:\n", path)
		fmt.Printf("    Sensitive: %t\n", sensitiveFilter.ShouldExclude(path))
		fmt.Printf("    Binary: %t\n", binaryFilter.ShouldExclude(path))
		fmt.Printf("    Lockfile: %t\n", lockfileFilter.ShouldExclude(path))
	}
}
