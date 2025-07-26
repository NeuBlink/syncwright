# CI/CD Pipeline Documentation

This document describes the comprehensive CI/CD pipeline setup for Syncwright, including release automation, continuous integration, and GitHub Marketplace publishing.

## Overview

The Syncwright CI/CD pipeline consists of several GitHub Actions workflows that handle:

- **Continuous Integration**: Automated testing, linting, and validation
- **Release Management**: Automated version bumping and GitHub releases
- **Package Distribution**: Multi-platform binary builds via GoReleaser
- **Marketplace Publishing**: GitHub Actions marketplace integration
- **Quality Assurance**: Security scanning and comprehensive testing

## Workflows

### 1. CI Workflow (`.github/workflows/ci.yml`)

**Triggers**: Push to main/develop, pull requests
**Purpose**: Continuous integration and quality assurance

**Jobs**:
- **Changes Detection**: Determines which parts of the codebase changed
- **Lint**: Code formatting and style checks using golangci-lint
- **Test**: Multi-platform testing (Ubuntu, macOS, Windows) with Go 1.21 and 1.22
- **Build**: Binary compilation for all supported platforms
- **CLI Commands Test**: Validation of all CLI commands and subcommands
- **Integration Test**: End-to-end testing of the GitHub Action
- **Security Scan**: Security vulnerability scanning with Gosec
- **Performance Test**: Benchmark testing and performance validation

**Features**:
- Matrix builds across multiple OS and Go versions
- Coverage reporting with Codecov integration
- Artifact uploads for build outputs
- Comprehensive test reporting in GitHub Step Summary

### 2. Release Workflow (`.github/workflows/release.yml`)

**Triggers**: Git tags (`v*`), manual workflow dispatch
**Purpose**: Automated release creation and distribution

**Jobs**:
- **Validate Release**: Version validation and pre-release detection
- **Test**: Full test suite execution before release
- **Validate Action**: GitHub Action structure and syntax validation
- **Build**: Cross-platform binary builds with GoReleaser
- **Release**: GitHub release creation with automated changelogs
- **Publish Action**: GitHub Marketplace publishing
- **Test Action Consumption**: Multi-platform action testing
- **Notify Completion**: Release status summary and notifications

**Features**:
- Semantic version validation and prerelease detection
- Automated changelog generation from git history
- Multi-platform binary builds (Linux, macOS, Windows, multiple architectures)
- GitHub Marketplace publishing with proper metadata
- Major version tag management (v1, v2, etc.)

### 3. Version Bump Workflow (`.github/workflows/version-bump.yml`)

**Triggers**: Manual workflow dispatch
**Purpose**: Automated version management and release triggering

**Inputs**:
- `version_type`: patch, minor, major
- `prerelease`: Boolean flag for prerelease versions
- `prerelease_type`: alpha, beta, rc

**Features**:
- Semantic version calculation and validation
- Git tag creation and pushing
- Prerelease PR creation for tracking
- Automatic release workflow triggering

### 4. Workflow Validation (`.github/workflows/validate-workflows.yml`)

**Triggers**: Changes to workflows, action.yml, or scripts
**Purpose**: Validate GitHub Actions configuration

**Jobs**:
- **YAML Syntax**: Syntax validation for all workflow files
- **Action Structure**: Composite action validation
- **Permissions**: Security review of workflow permissions
- **Secrets Usage**: Audit of secrets and token usage
- **Marketplace Readiness**: GitHub Marketplace requirements check
- **Security Scan**: Security vulnerability detection in workflows

## Configuration Files

### GoReleaser (`.goreleaser.yml`)

Comprehensive configuration for multi-platform builds and distribution:

- **Platforms**: Linux, macOS, Windows (amd64, arm64, 386, arm)
- **Archives**: Platform-specific archive formats (tar.gz, zip)
- **Checksums**: SHA256 verification files
- **Package Managers**: Homebrew, Winget, APT/RPM packages
- **Docker Images**: Container image publishing to GitHub Container Registry
- **Signing**: Cosign integration for artifact signing

### Linting (`.golangci.yml`)

Comprehensive linting configuration:

- **Enabled Linters**: errcheck, gosimple, govet, staticcheck, security checks
- **Code Quality**: Complexity analysis, function length limits
- **Security**: Gosec integration for vulnerability detection
- **Formatting**: Gofmt and goimports validation
- **Performance**: Inefficient code detection

## Version Management

### Semantic Versioning

Syncwright follows semantic versioning (SemVer):
- **Major**: Breaking changes
- **Minor**: New features (backward compatible)
- **Patch**: Bug fixes (backward compatible)
- **Prerelease**: alpha, beta, rc suffixes

### Version Script (`scripts/version.sh`)

Utility script for version management:

```bash
# Show current version
./scripts/version.sh current

# Show version information
./scripts/version.sh info

# Calculate next versions
./scripts/version.sh next patch
./scripts/version.sh next minor

# Validate version format
./scripts/version.sh validate v1.2.3
```

### Makefile Integration

Enhanced Makefile with CI/CD targets:

```bash
# Version management
make version           # Current version
make version-info      # Detailed information
make version-next      # Next possible versions

# Local CI pipeline
make ci-local          # Full local CI run
make check             # Quality checks
make test-coverage     # Test with coverage

# Workflow validation
make validate-workflows
make validate-action
```

## Release Process

### Automated Release (Recommended)

1. **Trigger Version Bump**:
   - Go to GitHub Actions → "Version Bump" workflow
   - Select version type (patch/minor/major)
   - Choose prerelease options if needed
   - Run workflow

2. **Automatic Steps**:
   - Version calculation and validation
   - Git tag creation and pushing
   - Release workflow triggering
   - Binary builds and GitHub release creation
   - GitHub Marketplace publishing

### Manual Release

1. **Create and Push Tag**:
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```

2. **Monitor Workflows**:
   - Release workflow triggers automatically
   - Monitor job progress in GitHub Actions
   - Verify release creation and marketplace publishing

## GitHub Marketplace

### Publishing Configuration

The action is automatically published to GitHub Marketplace when:
- A stable release (non-prerelease) is created
- `action.yml` contains proper marketplace metadata
- All validation checks pass

### Marketplace Metadata

Key metadata in `action.yml`:
- **Name**: Clear, descriptive action name
- **Description**: Detailed functionality description
- **Author**: Author information
- **Branding**: Icon and color for marketplace display

### Major Version Tags

Major version tags (v1, v2) are automatically maintained:
- Updated on each stable release
- Allow consumers to use `@v1` for latest v1.x.x
- Provide stability and easy updates

## Security Considerations

### Permissions

Workflows use minimal required permissions:
- **contents: write**: For release creation and tag pushing
- **packages: write**: For container image publishing
- **pull-requests: write**: For PR comments and labels

### Secrets Management

Required secrets:
- **GITHUB_TOKEN**: Automatically provided (releases, marketplace)
- **CLAUDE_CODE_OAUTH_TOKEN**: Optional for AI-powered testing

### Security Scanning

Multiple security layers:
- **Gosec**: Go security vulnerability scanning
- **Workflow Analysis**: GitHub Actions security review
- **Dependency Scanning**: Automated dependency vulnerability checks
- **SARIF Integration**: Security findings uploaded to GitHub Security tab

## Monitoring and Debugging

### Workflow Status

Monitor workflow execution:
- GitHub Actions tab shows all workflow runs
- Step-by-step execution logs
- Artifact downloads for build outputs
- Test results and coverage reports

### Common Issues

1. **Version Validation Failures**:
   - Ensure semantic version format (vX.Y.Z)
   - Check for duplicate tags
   - Validate git repository state

2. **Build Failures**:
   - Check Go module dependencies
   - Verify cross-compilation compatibility
   - Review build logs for specific errors

3. **Marketplace Publishing Issues**:
   - Validate action.yml metadata
   - Ensure stable release (not prerelease)
   - Check marketplace guidelines compliance

### Debug Resources

- **Workflow Logs**: Detailed execution information
- **Step Summary**: High-level status and results
- **Artifacts**: Download build outputs and reports
- **Local Testing**: Use `make ci-local` for local validation

## Local Development

### Setup

```bash
# Install development tools
make dev-setup

# Run local CI pipeline
make ci-local

# Validate workflows
make validate-workflows
make validate-action
```

### Testing

```bash
# Run tests with coverage
make test-coverage

# Run benchmarks
make bench

# Security scanning
make security-scan
```

This CI/CD pipeline provides a robust, automated workflow for maintaining code quality, creating releases, and distributing Syncwright across multiple platforms while ensuring security and reliability.