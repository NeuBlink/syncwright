# Syncwright

**AI-powered Git merge conflict resolution tool**

Syncwright is a production-ready CLI tool and GitHub Action that automatically detects, analyzes, and resolves Git merge conflicts using AI assistance. It provides a safe, intelligent approach to conflict resolution while maintaining code quality and developer control.

## Key Features

- **Intelligent Conflict Detection** - Automatically scans repositories and identifies merge conflicts with detailed context
- **AI-Powered Resolution** - Uses Claude AI to provide context-aware conflict resolutions with confidence scoring
- **Multi-Language Support** - Handles Go, JavaScript/TypeScript, Python, Java, C/C++, and more
- **Safety First** - Automatic backups, confidence thresholds, and validation ensure safe resolutions
- **GitHub Actions Integration** - Seamless CI/CD workflow integration with composite action
- **Security Conscious** - Automatically filters sensitive files and credentials from AI processing
- **Comprehensive Validation** - Built-in syntax checking and project validation tools

## Quick Start

### As GitHub Action (Recommended)

Add Syncwright to your workflow to automatically resolve merge conflicts in pull requests:

```yaml
name: Auto-resolve conflicts
on:
  pull_request:
    types: [opened, synchronize]

jobs:
  resolve-conflicts:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      
      - name: Resolve merge conflicts
        uses: neublink/syncwright@v1
        with:
          claude_code_oauth_token: ${{ secrets.CLAUDE_CODE_OAUTH_TOKEN }}
          run_validation: true
          max_tokens: 10000
```

### As CLI Tool

Install and use Syncwright directly:

```bash
# Install via script (recommended)
curl -fsSL https://raw.githubusercontent.com/neublink/syncwright/main/scripts/install.sh | bash

# Or install via Go
go install github.com/neublink/syncwright/cmd/syncwright@latest

# Detect conflicts
syncwright detect --verbose

# Generate AI payload
syncwright payload --in conflicts.json --out payload.json

# Apply AI resolutions
export CLAUDE_CODE_OAUTH_TOKEN="your-token"
syncwright ai-apply --in payload.json --verbose

# Format resolved files
syncwright format --recent

# Validate changes
syncwright validate --comprehensive
```

## Installation Options

### GitHub Action (Composite)

Use Syncwright as a reusable GitHub Action:

```yaml
- uses: neublink/syncwright@v1
  with:
    claude_code_oauth_token: ${{ secrets.CLAUDE_CODE_OAUTH_TOKEN }}
    merge_failed: true  # Set to true when merge conflicts detected
    pr_number: ${{ github.event.number }}
    base_branch: ${{ github.base_ref }}
    head_branch: ${{ github.head_ref }}
```

### Binary Installation

**Automated Script (Linux/macOS/Windows)**
```bash
curl -fsSL https://raw.githubusercontent.com/neublink/syncwright/main/scripts/install.sh | bash
```

**Manual Download**
Download pre-compiled binaries from [GitHub Releases](https://github.com/neublink/syncwright/releases)

**Go Install**
```bash
go install github.com/neublink/syncwright/cmd/syncwright@latest
```

## Configuration

### Environment Variables

| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| `CLAUDE_CODE_OAUTH_TOKEN` | Claude Code OAuth token for AI operations | Yes* | - |
| `SYNCWRIGHT_MAX_TOKENS` | Maximum tokens for AI processing | No | 10000 |
| `SYNCWRIGHT_DEBUG` | Enable debug logging | No | false |
| `SYNCWRIGHT_VERSION` | Specific version to install | No | latest |

*Required only for AI-powered operations

### GitHub Action Inputs

| Input | Description | Required | Default |
|-------|-------------|----------|---------|
| `claude_code_oauth_token` | Claude Code OAuth token | No | - |
| `run_validation` | Run validation checks | No | true |
| `max_tokens` | Maximum tokens for AI processing | No | 10000 |
| `merge_failed` | Whether automatic merge failed | No | false |
| `pr_number` | Pull request number | No | - |
| `base_branch` | Base branch name | No | - |
| `head_branch` | Head branch name | No | - |

### CLI Configuration

```bash
# Set confidence threshold (0.0-1.0)
syncwright ai-apply --confidence-threshold 0.8

# Enable verbose output
syncwright detect --verbose

# Specify output format
syncwright validate --format json --out results.json

# Dry run mode
syncwright ai-apply --dry-run
```

## Workflow Examples

### Standard GitHub Actions Workflow

```yaml
name: Syncwright Conflict Resolution

on:
  pull_request:
    types: [opened, synchronize, reopened]

jobs:
  resolve-conflicts:
    runs-on: ubuntu-latest
    permissions:
      contents: write
      pull-requests: write
    
    steps:
      - name: Checkout code
        uses: actions/checkout@v4
        with:
          fetch-depth: 0
          token: ${{ secrets.GITHUB_TOKEN }}
      
      - name: Attempt merge
        id: merge
        continue-on-error: true
        run: |
          git config user.name "github-actions[bot]"
          git config user.email "github-actions[bot]@users.noreply.github.com"
          
          # Attempt to merge base branch
          if ! git merge origin/${{ github.base_ref }}; then
            echo "merge_failed=true" >> $GITHUB_OUTPUT
          else
            echo "merge_failed=false" >> $GITHUB_OUTPUT
          fi
      
      - name: Resolve conflicts with Syncwright
        if: steps.merge.outputs.merge_failed == 'true'
        uses: neublink/syncwright@v1
        with:
          claude_code_oauth_token: ${{ secrets.CLAUDE_CODE_OAUTH_TOKEN }}
          merge_failed: true
          pr_number: ${{ github.event.number }}
          base_branch: ${{ github.base_ref }}
          head_branch: ${{ github.head_ref }}
          run_validation: true
          max_tokens: 15000
```

### CLI Workflow

```bash
#!/bin/bash
# Complete conflict resolution workflow

set -e

echo "🔍 Detecting conflicts..."
syncwright detect --verbose --out conflicts.json

if [ ! -s conflicts.json ] || [ "$(jq '.conflicts | length' conflicts.json)" -eq 0 ]; then
    echo "✅ No conflicts detected"
    exit 0
fi

echo "📦 Building AI payload..."
syncwright payload --in conflicts.json --out payload.json

echo "🤖 Applying AI resolutions..."
syncwright ai-apply --in payload.json --confidence-threshold 0.7 --verbose

echo "🎨 Formatting resolved files..."
syncwright format --recent --verbose

echo "✅ Validating changes..."
syncwright validate --comprehensive --verbose

echo "🎉 Conflict resolution complete!"
```

## Security Considerations

Syncwright is designed with security as a priority:

- **Sensitive Data Protection** - Automatically excludes files containing API keys, passwords, and credentials
- **Selective Processing** - Only conflict regions are sent to AI, not entire files
- **Backup Creation** - Automatic backups before any modifications
- **Confidence Scoring** - AI resolutions include confidence levels; low-confidence changes can be rejected
- **Local Processing** - Most operations run locally; only conflict context sent to AI service
- **Token Security** - OAuth tokens are handled securely and never logged

See [SECURITY.md](SECURITY.md) for detailed security information.

## Troubleshooting

### Common Issues

**Q: "No conflicts detected" but I see conflict markers**
```bash
# Ensure you're in a Git repository
git status

# Check if files are properly staged
git add .

# Re-run detection
syncwright detect --verbose
```

**Q: "AI resolution failed with authentication error"**
```bash
# Verify token is set
echo $CLAUDE_CODE_OAUTH_TOKEN

# Check token permissions
# Token needs access to Claude Code API
```

**Q: "Binary not found" in GitHub Actions**
```bash
# Ensure installation step completed
./syncwright --version

# Check PATH includes workspace
echo $PATH | grep workspace
```

**Q: "Confidence too low" rejecting all resolutions**
```bash
# Lower confidence threshold
syncwright ai-apply --confidence-threshold 0.5

# Review conflicts manually
syncwright ai-apply --dry-run --verbose
```

### Debug Mode

Enable detailed logging:

```bash
export SYNCWRIGHT_DEBUG=true
syncwright detect --verbose
```

Or in GitHub Actions:
```yaml
env:
  SYNCWRIGHT_DEBUG: true
```

### Getting Help

- Check [USAGE.md](USAGE.md) for detailed examples
- Review [GitHub Issues](https://github.com/neublink/syncwright/issues) for known problems
- Enable verbose mode (`--verbose`) for detailed output
- Use dry-run mode (`--dry-run`) to preview changes

## Contributing

We welcome contributions! Please see our contributing guidelines:

1. **Fork the repository** and create a feature branch
2. **Write tests** for new functionality
3. **Follow Go conventions** and run `go fmt`
4. **Update documentation** as needed
5. **Submit a pull request** with clear description

### Development Setup

```bash
# Clone repository
git clone https://github.com/neublink/syncwright.git
cd syncwright

# Install dependencies
go mod download

# Run tests
make test

# Build binary
make build

# Test locally
./bin/syncwright --version
```

### Code Structure

```
syncwright/
├── cmd/syncwright/     # CLI entry point
├── internal/
│   ├── commands/       # Command implementations
│   ├── gitutils/       # Git operations
│   ├── payload/        # AI payload generation
│   └── validate/       # Validation logic
├── scripts/            # Installation and utilities
└── action.yml          # GitHub Action definition
```

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Support

- **Documentation**: [USAGE.md](USAGE.md), [SECURITY.md](SECURITY.md)
- **Issues**: [GitHub Issues](https://github.com/neublink/syncwright/issues)
- **Discussions**: [GitHub Discussions](https://github.com/neublink/syncwright/discussions)

---

**Syncwright** - Making merge conflicts a thing of the past with AI-powered resolution.