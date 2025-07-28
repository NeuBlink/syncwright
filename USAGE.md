# Syncwright Usage Guide

This guide provides comprehensive examples and use cases for Syncwright, covering both CLI and GitHub Actions usage patterns.

## Table of Contents

- [CLI Usage](#cli-usage)
- [GitHub Actions Usage](#github-actions-usage)
- [Workflow Scenarios](#workflow-scenarios)
- [Advanced Configuration](#advanced-configuration)
- [Best Practices](#best-practices)
- [Troubleshooting Examples](#troubleshooting-examples)

## CLI Usage

### Basic Commands

#### Detect Conflicts

```bash
# Basic conflict detection
syncwright detect

# Save results to file
syncwright detect --out conflicts.json

# Verbose output with progress
syncwright detect --verbose

# Text format output
syncwright detect --format text
```

#### Generate AI Payload

```bash
# From detection results
syncwright payload --in conflicts.json --out payload.json

# Direct from stdin
syncwright detect | syncwright payload --out payload.json

# Verbose payload generation
syncwright payload --in conflicts.json --out payload.json --verbose
```

#### Apply AI Resolutions

```bash
# Basic AI resolution
export CLAUDE_CODE_OAUTH_TOKEN="your-token"
syncwright ai-apply --in payload.json

# With confidence threshold
syncwright ai-apply --in payload.json --confidence-threshold 0.8

# Dry run to preview changes
syncwright ai-apply --in payload.json --dry-run --verbose

# Save results
syncwright ai-apply --in payload.json --out resolutions.json
```

#### Format Files

```bash
# Format recently modified files
syncwright format --recent

# Format specific files
syncwright format main.go utils.py

# Dry run formatting
syncwright format --dry-run --verbose

# Format with specific formatters
syncwright format --prefer-formatter goimports,prettier

# Exclude certain extensions
syncwright format --exclude-ext .min.js,.generated.go
```

#### Validate Project

```bash
# Basic validation
syncwright validate

# Comprehensive validation
syncwright validate --comprehensive

# With custom timeout and retries
syncwright validate --timeout 600 --max-retries 5

# Save validation results with debug output
syncwright validate --out validation.json --verbose --debug
```

### Complete CLI Workflow

```bash
#!/bin/bash
# complete-resolution.sh - Full conflict resolution workflow

set -e

# Configuration
CONFIDENCE_THRESHOLD=0.7
TIMEOUT_SECONDS=600
MAX_RETRIES=5
# MAX_TOKENS=-1  # unlimited by default
OUTPUT_DIR="./syncwright-output"

# Create output directory
mkdir -p "$OUTPUT_DIR"

echo "🔍 Step 1: Detecting conflicts..."
if ! syncwright detect --verbose --out "$OUTPUT_DIR/conflicts.json"; then
    echo "❌ Conflict detection failed"
    exit 1
fi

# Check if conflicts exist
CONFLICT_COUNT=$(jq '.conflicts | length' "$OUTPUT_DIR/conflicts.json" 2>/dev/null || echo "0")
if [ "$CONFLICT_COUNT" -eq 0 ]; then
    echo "✅ No conflicts detected"
    exit 0
fi

echo "📦 Step 2: Building AI payload for $CONFLICT_COUNT conflicts..."
if ! syncwright payload \
    --in "$OUTPUT_DIR/conflicts.json" \
    --out "$OUTPUT_DIR/payload.json" \
    --verbose; then
    echo "❌ Payload generation failed"
    exit 1
fi

echo "🤖 Step 3: Applying AI resolutions..."
if ! syncwright ai-apply \
    --in "$OUTPUT_DIR/payload.json" \
    --out "$OUTPUT_DIR/resolutions.json" \
    --confidence-threshold "$CONFIDENCE_THRESHOLD" \
    --timeout "$TIMEOUT_SECONDS" \
    --max-retries "$MAX_RETRIES" \
    # --max-tokens -1  # unlimited by default \
    --verbose; then
    echo "❌ AI resolution failed"
    exit 1
fi

echo "🎨 Step 4: Formatting resolved files..."
if ! syncwright format --recent --verbose; then
    echo "⚠️ Formatting failed (non-critical)"
fi

echo "✅ Step 5: Validating changes..."
if ! syncwright validate \
    --comprehensive \
    --out "$OUTPUT_DIR/validation.json" \
    --verbose; then
    echo "⚠️ Validation failed (non-critical)"
fi

echo "🎉 Conflict resolution workflow complete!"
echo "📊 Summary:"
echo "  - Conflicts detected: $CONFLICT_COUNT"
echo "  - Results saved to: $OUTPUT_DIR/"

# Display resolution summary
if command -v jq >/dev/null 2>&1; then
    echo "  - Resolutions applied: $(jq '.resolutions | length' "$OUTPUT_DIR/resolutions.json" 2>/dev/null || echo "unknown")"
    echo "  - Overall confidence: $(jq '.overall_confidence // "unknown"' "$OUTPUT_DIR/resolutions.json" 2>/dev/null)"
fi
```

## GitHub Actions Usage

### Basic Integration

```yaml
name: Syncwright Integration
on:
  pull_request:

jobs:
  resolve:
    runs-on: ubuntu-latest
    permissions:
      contents: write
      pull-requests: write
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: NeuBlink/syncwright@v1
        with:
          claude_code_oauth_token: ${{ secrets.CLAUDE_CODE_OAUTH_TOKEN }}
```

### Advanced Workflow

```yaml
name: Advanced Conflict Resolution

on:
  pull_request:
    types: [opened, synchronize, reopened]
  workflow_dispatch:
    inputs:
      force_resolution:
        description: 'Force AI resolution even for low confidence'
        required: false
        default: 'false'

env:
  SYNCWRIGHT_DEBUG: ${{ github.event_name == 'workflow_dispatch' }}

jobs:
  detect-conflicts:
    runs-on: ubuntu-latest
    outputs:
      has-conflicts: ${{ steps.check.outputs.has-conflicts }}
      conflict-count: ${{ steps.check.outputs.conflict-count }}
    
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      
      - name: Setup merge attempt
        run: |
          git config user.name "github-actions[bot]"
          git config user.email "github-actions[bot]@users.noreply.github.com"
      
      - name: Attempt merge
        id: merge
        continue-on-error: true
        run: |
          if git merge origin/${{ github.base_ref }}; then
            echo "merge-successful=true" >> $GITHUB_OUTPUT
          else
            echo "merge-successful=false" >> $GITHUB_OUTPUT
          fi
      
      - name: Check for conflicts
        id: check
        if: steps.merge.outputs.merge-successful == 'false'
        uses: NeuBlink/syncwright@v1
        with:
          run_validation: false
          # Only detect, don't resolve yet

  resolve-conflicts:
    needs: detect-conflicts
    if: needs.detect-conflicts.outputs.has-conflicts == 'true'
    runs-on: ubuntu-latest
    permissions:
      contents: write
      pull-requests: write
    
    strategy:
      matrix:
        confidence: [0.8, 0.6]  # Try high confidence first, then lower
      fail-fast: false
    
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
          token: ${{ secrets.GITHUB_TOKEN }}
      
      - name: Resolve with confidence ${{ matrix.confidence }}
        id: resolve
        uses: NeuBlink/syncwright@v1
        with:
          claude_code_oauth_token: ${{ secrets.CLAUDE_CODE_OAUTH_TOKEN }}
          merge_failed: true
          pr_number: ${{ github.event.number }}
          base_branch: ${{ github.base_ref }}
          head_branch: ${{ github.head_ref }}
          timeout_seconds: 900     # Extended timeout for complex conflicts
          max_retries: 3          # Retry failed operations
          debug_mode: true        # Enable for detailed logging
          # max_tokens: -1        # unlimited by default
          confidence_threshold: ${{ matrix.confidence }}
      
      - name: Post resolution summary
        if: steps.resolve.outputs.conflicts_resolved == 'true'
        uses: actions/github-script@v7
        with:
          script: |
            const comment = `## 🤖 Syncwright Resolution Summary
            
            ✅ Successfully resolved merge conflicts!
            
            - **Files modified**: ${{ steps.resolve.outputs.files_modified }}
            - **Confidence level**: ${{ matrix.confidence }}
            - **Resolution method**: AI-powered analysis
            
            The conflicts have been automatically resolved and committed to this PR.
            Please review the changes before merging.`;
            
            github.rest.issues.createComment({
              issue_number: context.issue.number,
              owner: context.repo.owner,
              repo: context.repo.repo,
              body: comment
            });

  post-resolution-checks:
    needs: resolve-conflicts
    if: always()
    runs-on: ubuntu-latest
    
    steps:
      - uses: actions/checkout@v4
      
      - name: Run comprehensive validation
        uses: NeuBlink/syncwright@v1
        with:
          run_validation: true
          validation_mode: comprehensive
      
      - name: Check for remaining conflicts
        run: |
          if git ls-files -u | grep -q .; then
            echo "❌ Unresolved conflicts detected:"
            git ls-files -u
            exit 1
          else
            echo "✅ All conflicts resolved successfully"
          fi
```

### Matrix Strategy for Multiple Languages

```yaml
name: Multi-Language Conflict Resolution

on:
  pull_request:

jobs:
  resolve-by-language:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        include:
          - language: go
            files: "**/*.go"
            # max_tokens: -1  # unlimited by default
          - language: typescript
            files: "**/*.{ts,tsx,js,jsx}"
            # max_tokens: -1  # unlimited by default
          - language: python
            files: "**/*.py"
            # max_tokens: -1  # unlimited by default
    
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      
      - name: Resolve ${{ matrix.language }} conflicts
        uses: NeuBlink/syncwright@v1
        with:
          claude_code_oauth_token: ${{ secrets.CLAUDE_CODE_OAUTH_TOKEN }}
          timeout_seconds: 900      # Extended timeout for large codebases
          max_retries: 5           # More retries for reliability
          debug_mode: false        # Enable for troubleshooting
          # max_tokens: -1         # unlimited by default
          file_filter: ${{ matrix.files }}
          language_specific: true
```

## Workflow Scenarios

### Scenario 1: Automated PR Conflict Resolution

**Use Case**: Automatically resolve conflicts when PRs are created or updated.

```bash
# In PR workflow
git checkout $PR_BRANCH
git merge $BASE_BRANCH  # This may fail with conflicts

# Run Syncwright if merge fails
if [ $? -ne 0 ]; then
    syncwright detect | syncwright payload | syncwright ai-apply
    git add -A
    git commit -m "Resolve conflicts using Syncwright"
    git push origin $PR_BRANCH
fi
```

### Scenario 2: Pre-merge Validation

**Use Case**: Validate that a PR can be merged cleanly before approval.

```yaml
- name: Pre-merge validation
  run: |
    # Create temporary merge commit
    git checkout -b temp-merge-${{ github.event.number }}
    git merge origin/${{ github.base_ref }}
    
    # If conflicts, try to resolve
    if [ $? -ne 0 ]; then
      syncwright detect --out conflicts.json
      if [ "$(jq '.conflicts | length' conflicts.json)" -gt 0 ]; then
        echo "conflicts-detected=true" >> $GITHUB_OUTPUT
      fi
    fi
```

### Scenario 3: Batch Conflict Resolution

**Use Case**: Resolve conflicts across multiple branches or repositories.

```bash
#!/bin/bash
# batch-resolve.sh

BRANCHES=("feature/auth" "feature/ui" "feature/api")
BASE_BRANCH="main"

for branch in "${BRANCHES[@]}"; do
    echo "Processing branch: $branch"
    
    git checkout "$branch"
    git pull origin "$branch"
    
    # Attempt merge with base
    if ! git merge "origin/$BASE_BRANCH"; then
        echo "Conflicts detected in $branch, resolving..."
        
        # Run Syncwright workflow
        syncwright detect --out "conflicts-$branch.json"
        syncwright payload --in "conflicts-$branch.json" --out "payload-$branch.json"
        syncwright ai-apply --in "payload-$branch.json" --confidence-threshold 0.7
        
        # Format and validate
        syncwright format --recent
        syncwright validate --comprehensive
        
        # Commit and push
        git add -A
        git commit -m "Resolve merge conflicts with $BASE_BRANCH using Syncwright"
        git push origin "$branch"
        
        echo "✅ Resolved conflicts in $branch"
    else
        echo "✅ No conflicts in $branch"
    fi
    
    echo "---"
done
```

### Scenario 4: Release Branch Preparation

**Use Case**: Prepare release branches by resolving conflicts with main.

```bash
#!/bin/bash
# prepare-release.sh

RELEASE_VERSION="v2.1.0"
RELEASE_BRANCH="release/$RELEASE_VERSION"

# Create release branch
git checkout main
git pull origin main
git checkout -b "$RELEASE_BRANCH"

# Cherry-pick or merge features
FEATURE_BRANCHES=("feature/payment" "feature/notifications" "hotfix/security")

for feature in "${FEATURE_BRANCHES[@]}"; do
    echo "Merging $feature into $RELEASE_BRANCH..."
    
    if ! git merge "origin/$feature"; then
        echo "Resolving conflicts for $feature..."
        
        # Use higher confidence for release branches
        syncwright detect --verbose | \
        syncwright payload | \
        syncwright ai-apply --confidence-threshold 0.85 --verbose
        
        # Extra validation for release
        syncwright validate --comprehensive --timeout 600
        syncwright format --verbose
        
        git add -A
        git commit -m "Merge $feature with conflict resolution"
    fi
done

# Final release validation
echo "Running final release validation..."
syncwright validate --comprehensive --out "release-validation.json"

git push origin "$RELEASE_BRANCH"
echo "🚀 Release branch $RELEASE_BRANCH ready"
```

## Advanced Configuration

### Custom Confidence Thresholds

```bash
# Conservative approach (high confidence required)
syncwright ai-apply --confidence-threshold 0.9

# Balanced approach (default)
syncwright ai-apply --confidence-threshold 0.7

# Aggressive approach (lower confidence accepted)
syncwright ai-apply --confidence-threshold 0.5

# Manual review for low confidence
syncwright ai-apply --confidence-threshold 0.8 --manual-review-below 0.6
```

### File Filtering

```bash
# Only process specific file types
syncwright detect --include-ext go,js,py

# Exclude certain patterns
syncwright detect --exclude-pattern "*.generated.*,*_pb.go,*.min.js"

# Custom sensitive file patterns
export SYNCWRIGHT_SENSITIVE_PATTERNS="*.key,*.pem,*secret*,*password*"
syncwright detect
```

### Output Formats

```bash
# JSON output (default)
syncwright detect --format json --out conflicts.json

# Human-readable text
syncwright detect --format text

# Structured markdown report
syncwright validate --format markdown --out validation.md

# CSV format for analysis
syncwright detect --format csv --out conflicts.csv
```

## Best Practices

### 1. Confidence Thresholds

- **Production branches**: Use 0.8+ confidence
- **Feature branches**: Use 0.6+ confidence  
- **Experimental**: Use 0.5+ confidence
- **Always review**: Changes below 0.7 confidence

### 2. Backup Strategy

```bash
# Syncwright automatically creates backups, but you can also:
git stash push -m "Before Syncwright resolution"
syncwright ai-apply --in payload.json
# If something goes wrong:
# git stash pop
```

### 3. Validation Pipeline

```bash
# Recommended validation sequence
syncwright ai-apply --dry-run --verbose  # Preview changes
syncwright ai-apply --confidence-threshold 0.8  # Apply with high confidence
syncwright format --recent  # Format resolved files
syncwright validate --comprehensive  # Validate entire project
```

### 4. Security Best Practices

- Store `CLAUDE_CODE_OAUTH_TOKEN` in secrets, never in code
- Use `.gitignore` to exclude sensitive files from processing
- Review AI resolutions before merging to production
- Monitor for false positives in sensitive code areas

## Troubleshooting Examples

### Debug Workflow Issues

```bash
# Enable debug mode
export SYNCWRIGHT_DEBUG=true

# Verbose output for all commands
syncwright detect --verbose
syncwright payload --verbose --in conflicts.json
syncwright ai-apply --verbose --dry-run --in payload.json

# Check intermediate files
ls -la *.json
jq '.' conflicts.json  # Validate JSON structure
```

### Handle API Rate Limits and Timeouts

```bash
# Reduce token usage if needed (unlimited by default)
syncwright ai-apply --max-tokens 5000

# Extended timeout for large repositories
syncwright ai-apply --timeout 1200

# Increase retry attempts for unreliable networks
syncwright ai-apply --max-retries 10

# Combine timeout and retry settings
syncwright ai-apply --timeout 900 --max-retries 5 --verbose
```

### Handle Timeout Issues

```bash
# For large repositories, increase timeout
export SYNCWRIGHT_TIMEOUT=1800
syncwright resolve --ai --verbose

# Network timeout troubleshooting
syncwright resolve --ai --timeout 600 --max-retries 3 --debug

# Progressive timeout strategy
syncwright ai-apply --timeout 300 || \
syncwright ai-apply --timeout 600 || \
syncwright ai-apply --timeout 1200
```

### Recovery from Failed Resolutions

```bash
# If AI resolution fails, fall back to manual
syncwright detect --out conflicts.json
cat conflicts.json  # Review conflicts manually

# Restore from backup if needed
ls .syncwright-backup-*
cp .syncwright-backup-20240101-120000/main.go main.go

# Re-run with different settings
syncwright ai-apply --confidence-threshold 0.5 --manual-review
```

### Validate GitHub Actions Integration

```bash
# Test action locally with act
act pull_request -s CLAUDE_CODE_OAUTH_TOKEN="your-token"

# Debug action steps
- name: Debug Syncwright
  run: |
    echo "Environment:"
    env | grep SYNCWRIGHT
    echo "Binary location:"
    which syncwright
    echo "Version:"
    ./syncwright --version
```

This comprehensive usage guide covers the most common scenarios and advanced configurations for Syncwright. For additional help, see the main [README.md](README.md) or [open an issue](https://github.com/NeuBlink/syncwright/issues).