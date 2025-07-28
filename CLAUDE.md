# Syncwright: AI-Powered Git Conflict Resolution

**Syncwright** is a production-ready Go CLI tool that automatically resolves Git merge conflicts using Claude AI. This CLAUDE.md provides context, conventions, and workflow guidance for development.

## Tech Stack & Architecture

**Core Technologies:**
- **Language**: Go 1.23+ with Go modules
- **CLI Framework**: Cobra + Viper for command structure
- **AI Integration**: Claude Code OAuth API for conflict resolution
- **Data Format**: JSON for conflict payloads and responses
- **CI/CD**: GitHub Actions with composite action architecture
- **Distribution**: Multi-platform binaries via GoReleaser

**Workflow Pipeline:**
```
git conflicts → detect → payload → ai-apply → format → validate → commit
```

## Project Structure

```
syncwright/
├── cmd/syncwright/           # CLI entry point and command definitions
├── internal/
│   ├── commands/            # Command implementations (detect, ai-apply, etc.)
│   ├── claude/              # Claude AI client and conflict resolver
│   ├── gitutils/            # Git operations and conflict parsing
│   ├── payload/             # JSON data structures for AI processing
│   ├── format/              # Code formatting integration
│   └── validate/            # Project validation logic
├── .claude/agents/          # Specialized AI sub-agents
├── .github/workflows/       # CI/CD automation
├── action.yml              # GitHub composite action definition
├── scripts/                # Installation and utility scripts
└── docs/                   # Documentation and guides
```

## Core Commands

**CLI Commands:**
- `syncwright detect` - Scan for merge conflicts, output JSON
- `syncwright payload` - Transform conflicts to AI-ready format
- `syncwright ai-apply` - Apply Claude AI resolutions
- `syncwright batch` - Process multiple conflicts concurrently for better performance
- `syncwright format` - Format resolved files
- `syncwright validate` - Project integrity checks
- `syncwright resolve` - Full pipeline automation

**Development Commands:**
- `make build` - Build Go binary
- `make test` - Run test suite with coverage
- `make lint` - Go linting and formatting
- `make ci-local` - Full CI pipeline locally

## Code Style & Conventions

**Go Standards:**
- Follow `gofmt` and `golangci-lint` rules
- Use idiomatic Go patterns and error handling
- Prefer explicit error handling over panics
- Design for testability with interfaces and dependency injection
- Use context.Context for cancellation and timeouts

**JSON Schema Design:**
- Type-safe marshaling with struct tags
- Validate schemas for conflict payloads
- Handle malformed input gracefully
- Sanitize sensitive data before AI processing

**CLI UX Principles:**
- Support both human and machine-readable output
- Provide meaningful exit codes (0=success, >0=error)
- Include progress indicators for long operations
- Offer `--dry-run` for preview functionality
- Follow Unix philosophy: composable, focused commands

## Batch Processing

**Performance Optimization:**
The `syncwright batch` command is designed for large repositories with many conflicts, providing significant performance improvements through:

- **Intelligent Grouping**: Groups conflicts by language, file, or estimated token size for optimal AI processing
- **Concurrent Processing**: Processes multiple batches simultaneously with configurable concurrency limits
- **Streaming Results**: Shows results as batches complete rather than waiting for all processing to finish
- **Performance Metrics**: Detailed timing, throughput, and efficiency statistics

**Grouping Strategies:**
- `language` - Group conflicts by programming language (default, most efficient for AI processing)
- `file` - Create one batch per file (good for isolated file conflicts)
- `size` - Group by estimated token consumption (optimizes API usage)
- `none` - Sequential batching without intelligent grouping

**Usage Examples:**
```bash
# Basic high-performance processing
syncwright batch --batch-size 15 --concurrency 5

# Language-optimized processing with progress tracking
syncwright batch --group-by language --progress --streaming

# Token-optimized processing for large conflicts
syncwright batch --group-by size --max-tokens 40000

# Dry run to preview batch organization
syncwright batch --dry-run --verbose
```

**Performance Considerations:**
- Optimal batch size: 10-20 conflicts per batch (default: 10)
- Recommended concurrency: 3-5 parallel batches (default: 3)
- Token limits: 50,000 tokens per batch (default, adjust based on Claude API limits)
- Memory usage: Scales with batch size and concurrent batches

## Security Requirements

**Critical Security Rules:**
- ❌ **NEVER** commit Claude Code OAuth tokens (`CLAUDE_CODE_OAUTH_TOKEN`)
- ❌ **NEVER** hardcode API keys or credentials in source code
- ❌ **NEVER** commit files with unresolved conflict markers
- ✅ **ALWAYS** use environment variables for sensitive configuration
- ✅ **ALWAYS** validate and sanitize JSON payloads before AI processing
- ✅ **ALWAYS** run security validation before commits

**Sensitive File Patterns:**
- Exclude: `*.key`, `*.pem`, `.env*`, `secrets.yaml`
- Watch for: API tokens, database URLs, private keys
- Validate: Complete conflict marker removal

## Conventional Commits

**Required Format:** `type(scope): subject`

**Types:** `feat|fix|perf|refactor|docs|test|build|ci|chore|revert`

**Syncwright Scopes:**
- `conflict` - Detection, parsing, analysis
- `resolve` - Resolution algorithms, strategies  
- `ai` - Claude API integration, prompts
- `payload` - JSON structures, processing
- `validate` - Validation logic, integrity
- `cli` - Command interface, UX
- `action` - GitHub Actions, workflows
- `core` - System architecture, utilities

**Examples:**
```
feat(resolve): add confidence scoring for AI resolutions
fix(payload): handle malformed JSON in conflict data
perf(cli): optimize git operations for large repositories
```

## Testing Strategy

**Test Coverage Requirements:**
- Unit tests: `internal/` packages (>85% coverage)
- Integration tests: Full CLI workflows  
- E2E tests: GitHub Actions composite action
- Security tests: Token detection, payload validation

**Testing Patterns:**
- Mock external dependencies (Claude API, git operations)
- Use table-driven tests for multiple scenarios
- Test error conditions and edge cases
- Validate JSON schema compliance

## Specialized AI Agents

The `.claude/agents/` directory contains specialized sub-agents:

**Agent Usage Guide:**
- `repo-context-guardian` - Project planning and coordination
- `go-cli-specialist` - Go development and CLI architecture  
- `ai-conflict-resolver` - Claude AI integration and conflict resolution
- `github-actions-developer` - CI/CD workflows and composite actions
- `pre-commit-security-gate` - Security validation and quality checks
- `commit-pr-guardian` - Repository hygiene and commit standards

**Agent Coordination:**
Start complex tasks with `repo-context-guardian` for orchestration, then use specialized agents for implementation.

## Development Workflow

**Feature Development:**
1. Use `repo-context-guardian` to plan architecture
2. Implement with `go-cli-specialist` or `ai-conflict-resolver`
3. Validate security with `pre-commit-security-gate`
4. Generate commit with `commit-pr-guardian`
5. Update CI/CD with `github-actions-developer`

**Quality Gates:**
1. All code must pass `make ci-local`
2. Security validation required before commits
3. Conventional commit messages enforced
4. PR templates and reviews required

## Environment Variables

**Required:**
- `CLAUDE_CODE_OAUTH_TOKEN` - Claude AI API access (development/testing)

**Optional:**
- `SYNCWRIGHT_DEBUG=true` - Enable debug logging
- `SYNCWRIGHT_MAX_TOKENS=-1` - AI token limits
- `GITHUB_TOKEN` - GitHub API access for workflows

## File Organization Rules

**Core Principles:**
- Group related functionality in `internal/` packages
- Keep CLI commands thin, logic in `internal/`
- Separate concerns: parsing, processing, AI integration
- Use interfaces for testability and mocking

**Do Not Touch:**
- `.github/workflows/` without CI/CD expertise
- `action.yml` composite action without testing
- Security-related validation logic without review
- Claude AI prompt templates without conflict resolution knowledge

## Performance Considerations

**Optimization Targets:**
- CLI startup time: <500ms for simple commands
- Memory usage: Efficient for large repositories
- API efficiency: Batch AI requests, cache responses
- Concurrent processing: Multiple files/conflicts simultaneously

**Monitoring:**
- Track Claude API usage and costs
- Monitor conflict resolution success rates
- Measure CLI performance in CI/CD environments

---

**Quick Start:** Use `/agents` to explore specialized sub-agents, or start with `repo-context-guardian` for task coordination.