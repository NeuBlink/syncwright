#!/bin/bash

# Example Syncwright workflow
# This script demonstrates the complete pipeline for merge conflict resolution

set -e

echo "Syncwright Example Workflow"
echo "=========================="

# Step 1: Detect conflicts
echo "1. Detecting merge conflicts..."
./bin/syncwright detect --out conflicts.json
echo "   Output saved to conflicts.json"

# Step 2: Generate AI payload
echo "2. Generating AI-ready payload..."
./bin/syncwright payload --in conflicts.json --out payload.json
echo "   Output saved to payload.json"

# Step 3: Apply AI resolutions (simulated)
echo "3. Applying AI-generated resolutions..."
./bin/syncwright ai-apply --in payload.json --out ai_apply.json
echo "   Output saved to ai_apply.json"

# Step 4: Format resolved files
echo "4. Formatting resolved files..."
./bin/syncwright format --out format.json
echo "   Output saved to format.json"

# Step 5: Validate resolved files
echo "5. Validating resolved files..."
./bin/syncwright validate --out validate.json
echo "   Output saved to validate.json"

# Step 6: Commit changes (dry run for now)
echo "6. Ready to commit changes..."
echo "   Run: ./bin/syncwright commit"

echo ""
echo "Workflow complete! Check the generated JSON files for results."
echo ""
echo "Generated files:"
echo "  - conflicts.json  (conflict detection results)"
echo "  - payload.json    (AI-ready payloads)"
echo "  - ai_apply.json   (AI resolution results)"
echo "  - format.json     (formatting results)"
echo "  - validate.json   (validation results)"