# Syncwright Testing Guide

This document provides comprehensive testing guidelines for Syncwright development and integration.

## Testing Philosophy

Syncwright prioritizes safety and reliability in automated conflict resolution. Our testing approach ensures:

- **Non-destructive operations**: All conflict resolutions preserve original content through backups
- **Confidence-based decisions**: AI suggestions include confidence scores for safety thresholds
- **Comprehensive validation**: Multi-stage validation before, during, and after resolution
- **Real-world scenarios**: Testing with authentic development conflicts, not artificial cases

## Test Categories

### 1. Unit Testing
- Individual command functionality (detect, ai-apply, validate, format)
- Payload builder and conflict analysis
- Git utilities and repository operations
- Configuration parsing and validation

### 2. Integration Testing  
- End-to-end conflict resolution workflows
- GitHub Actions integration and triggers
- CLI tool integration with git operations
- Multi-repository and workspace scenarios

### 3. Performance Testing
- Large repository handling (>10k files)
- Complex conflict scenarios (50+ conflicts)
- Memory usage and optimization
- Timeout and retry mechanism validation

### 4. Security Testing
- Sensitive data filtering and exclusion
- OAuth token handling and scoping
- Repository access permissions
- Input validation and sanitization

## PR Testing Scenarios

For comprehensive production readiness, test these realistic scenarios:

### Scenario A: Simple Feature Addition
- Single file modifications
- No overlapping changes
- Clean merge expected
- Validates basic workflow operation

### Scenario B: Conflicting Feature Development
- Multiple developers modifying shared files
- Overlapping struct/interface changes  
- Configuration file conflicts
- Tests AI-powered conflict resolution

### Scenario C: Base Branch Updates
- Main branch changes requiring PR updates
- Dependency updates affecting multiple files
- Breaking API changes requiring adaptation
- Tests adaptive conflict resolution

## Automation Integration

### GitHub Actions Configuration
The reusable workflow (`syncwright-reusable.yml`) provides:
- Automatic conflict detection on PR events
- AI-powered resolution with confidence scoring
- Status reporting through PR comments and labels
- Concurrency control and cancellation

### Consumer Workflow Setup
Minimal consumer configuration:
```yaml
uses: neublink/syncwright@v1
with:
  claude_code_oauth_token: ${{ secrets.CLAUDE_CODE_OAUTH_TOKEN }}
```

## Quality Assurance

### Pre-merge Validation
- Syntax and compilation checks
- Test suite execution  
- Security scan completion
- Documentation updates

### Post-resolution Verification
- Conflict marker removal
- Code functionality preservation
- Test suite still passing
- No introduction of security vulnerabilities

## Troubleshooting

### Common Issues
- **Timeout errors**: Increase timeout_seconds parameter
- **API rate limits**: Implement retry delays and backoff
- **Large conflicts**: Use max_tokens parameter tuning
- **Permission errors**: Verify repository access and token scopes

### Debug Mode
Enable detailed logging with `debug_mode: true` for comprehensive operation visibility.

## Metrics and Monitoring

Track these key metrics for production deployment:
- Conflict resolution success rate
- Average resolution time  
- Manual intervention frequency
- Developer satisfaction scores

## Contributing to Tests

When adding new test scenarios:
1. Use realistic development conflicts
2. Include both positive and negative test cases
3. Document expected outcomes and failure modes
4. Validate against security and safety requirements

For more information, see SECURITY.md and USAGE.md documentation.