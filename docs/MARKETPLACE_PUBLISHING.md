# GitHub Marketplace Publishing Guide

This document provides comprehensive instructions for publishing the Syncwright GitHub Action to the GitHub Marketplace, including automated workflows and manual processes.

## Table of Contents

- [Overview](#overview)
- [Automated Publishing](#automated-publishing)
- [Manual Publishing](#manual-publishing)
- [Version Management](#version-management)
- [Marketplace Requirements](#marketplace-requirements)
- [Troubleshooting](#troubleshooting)
- [Best Practices](#best-practices)

## Overview

Syncwright uses an automated publishing workflow that triggers on version tags to streamline the marketplace publication process. The workflow handles validation, versioning, release creation, and marketplace submission automatically.

### Key Features

- **Automated Publishing**: Triggered by git tags (v1.0.0, v2.1.3, etc.)
- **Version Management**: Automatic major version tag updates (v1, v2, etc.)
- **Validation Suite**: Comprehensive action validation before publishing
- **Release Notes**: Automatic generation of detailed release notes
- **Prerelease Support**: Handles beta/alpha versions appropriately

## Automated Publishing

### Triggering a Release

The automated publishing process is triggered by creating and pushing a version tag:

```bash
# Create a new release version
git tag -a v1.2.3 -m "Release version 1.2.3"
git push origin v1.2.3
```

### Tag Format Requirements

Tags must follow semantic versioning format:

- **Stable releases**: `v1.0.0`, `v2.1.3`, `v3.0.0`
- **Prereleases**: `v1.0.0-beta.1`, `v2.0.0-alpha.2`, `v1.5.0-rc.1`

### Workflow Steps

The automated workflow (`/.github/workflows/publish-marketplace.yml`) performs:

1. **Validation Phase**:
   - Tag format validation
   - action.yml syntax checking
   - Go build testing
   - Script validation
   - Local action testing

2. **Publishing Phase**:
   - Major version tag updates (for stable releases)
   - Release notes generation
   - GitHub release creation
   - Marketplace publication status

3. **Notification Phase**:
   - Success/failure notifications
   - Summary reporting

### Workflow Outputs

The workflow provides detailed outputs including:

- Version information
- Validation results
- Release URLs
- Marketplace publication status

## Manual Publishing

### Prerequisites

Before manual publishing, ensure you have:

1. **Repository permissions**: Admin access to the repository
2. **Clean working tree**: No uncommitted changes
3. **Valid action.yml**: Properly formatted and complete
4. **Working binaries**: Go build succeeds
5. **Documentation**: Up-to-date README and usage docs

### Manual Release Process

If automatic publishing fails or manual intervention is needed:

#### 1. Validate the Action

```bash
# Validate action.yml syntax
python3 -c "
import yaml
with open('action.yml', 'r') as f:
    yaml.safe_load(f)
print('action.yml is valid')
"

# Test Go build
go build -v ./cmd/syncwright
./syncwright --version

# Validate scripts
bash -n scripts/install.sh
```

#### 2. Create Release Manually

```bash
# Create and push tag
git tag -a v1.2.3 -m "Release version 1.2.3"
git push origin v1.2.3

# Create major version tag
git tag -fa v1 -m "Update v1 to v1.2.3"
git push origin v1 --force
```

#### 3. Create GitHub Release

Use the GitHub web interface or CLI:

```bash
# Using GitHub CLI
gh release create v1.2.3 \
  --title "Syncwright v1.2.3" \
  --notes-file release-notes.md \
  --latest
```

#### 4. Manual Workflow Trigger

You can also trigger the automated workflow manually:

```bash
# Using GitHub CLI
gh workflow run publish-marketplace.yml \
  --field tag=v1.2.3 \
  --field force_publish=false
```

## Version Management

### Semantic Versioning

Syncwright follows [semantic versioning](https://semver.org/):

- **MAJOR** (v1.0.0 → v2.0.0): Breaking changes
- **MINOR** (v1.0.0 → v1.1.0): New features, backward compatible
- **PATCH** (v1.0.0 → v1.0.1): Bug fixes, backward compatible

### Major Version Tags

For user convenience, major version tags (v1, v2, etc.) are automatically updated:

- Users can reference `NeuBlink/syncwright@v1` for latest v1.x.x
- Provides stability while allowing automatic updates
- Only updated for stable releases (not prereleases)

### Prerelease Versions

Prerelease versions are supported:

- **Alpha**: `v1.0.0-alpha.1` (early development)
- **Beta**: `v1.0.0-beta.1` (feature complete, testing)
- **Release Candidate**: `v1.0.0-rc.1` (final testing)

Prereleases:
- Do not update major version tags
- Are marked as prereleases in GitHub
- Are not promoted as "latest" releases

## Marketplace Requirements

### Essential Requirements

The action must meet GitHub Marketplace requirements:

#### action.yml Requirements

- ✅ Valid YAML syntax
- ✅ Required fields: `name`, `description`, `runs`
- ✅ Clear, descriptive name
- ✅ Comprehensive description (under 125 characters for marketplace)
- ✅ Proper branding with icon and color

#### Repository Requirements

- ✅ Public repository
- ✅ README.md with usage instructions
- ✅ LICENSE file
- ✅ Functional action code
- ✅ No broken links

#### Quality Standards

- ✅ Comprehensive input/output documentation
- ✅ Working examples in README
- ✅ Proper error handling
- ✅ Security best practices

### Branding Guidelines

Current branding configuration:

```yaml
branding:
  icon: 'git-merge'    # Represents merge conflict resolution
  color: 'blue'        # Professional, trustworthy color
```

### Description Optimization

The description is optimized for marketplace discovery:

```yaml
description: 'AI-powered Git merge conflict resolution tool that automatically detects, analyzes, and resolves merge conflicts using advanced language models for seamless CI/CD integration'
```

Key phrases for SEO:
- AI-powered
- Git merge conflict resolution
- Automatic detection and analysis
- Advanced language models
- CI/CD integration

## Troubleshooting

### Common Issues

#### Validation Failures

**Problem**: action.yml validation fails
```
Error: Missing required field: description
```

**Solution**: Ensure all required fields are present:
```yaml
name: 'Syncwright'
description: 'Your description here'
runs:
  using: 'composite'
  steps: [...]
```

#### Tag Format Errors

**Problem**: Invalid tag format
```
Error: Invalid tag format. Expected format: v1.2.3 or v1.2.3-beta.1
```

**Solution**: Use proper semantic versioning:
```bash
# Correct formats
git tag v1.0.0
git tag v1.0.0-beta.1

# Incorrect formats
git tag 1.0.0        # Missing 'v' prefix
git tag v1.0         # Incomplete version
```

#### Build Failures

**Problem**: Go build fails during validation
```
go: module not found
```

**Solution**: Ensure dependencies are properly declared:
```bash
go mod tidy
go mod verify
```

#### Permission Issues

**Problem**: Cannot push major version tags
```
Permission denied (push)
```

**Solution**: Ensure the workflow has proper permissions:
```yaml
permissions:
  contents: write
  packages: write
```

#### Marketplace Rejection

**Problem**: Action rejected from marketplace

**Common causes**:
- Broken links in README
- Missing or invalid LICENSE
- Non-functional action
- Incomplete documentation

**Solution**: Review marketplace guidelines and fix identified issues.

### Debugging Workflows

#### Enable Debug Logging

Add debug output to workflows:

```yaml
- name: Debug Information
  run: |
    echo "Event: ${{ github.event_name }}"
    echo "Ref: ${{ github.ref }}"
    echo "SHA: ${{ github.sha }}"
    env
```

#### Local Testing

Test action components locally:

```bash
# Test action steps manually
export INPUT_RUN_VALIDATION=true
export INPUT_MAX_TOKENS=-1
export INPUT_TIMEOUT_SECONDS=300
export INPUT_MAX_RETRIES=3
export INPUT_DEBUG_MODE=false

# Run installation script
./scripts/install.sh

# Test binary
./syncwright --version
```

#### Workflow Re-runs

Re-run failed workflows with debugging:

```bash
# Re-run workflow with debug logging
gh workflow run publish-marketplace.yml \
  --field tag=v1.2.3 \
  --field force_publish=true
```

## Best Practices

### Pre-Release Checklist

Before creating a release tag:

- [ ] All tests pass
- [ ] Documentation is updated
- [ ] CHANGELOG.md is updated
- [ ] Version numbers are consistent
- [ ] action.yml is valid
- [ ] Scripts are executable and functional
- [ ] Binary builds successfully
- [ ] Examples in README work
- [ ] Timeout and retry mechanisms tested
- [ ] Debug mode functionality validated
- [ ] New input parameters documented

### Release Preparation

1. **Update Documentation**:
   - Update version references
   - Refresh examples
   - Update feature lists

2. **Test Thoroughly**:
   - Test on multiple platforms
   - Validate with real merge conflicts
   - Check CI/CD integration

3. **Version Consistency**:
   - Update version in relevant files
   - Ensure go.mod is current
   - Check action.yml references

### Marketplace Optimization

#### SEO Optimization

- Use relevant keywords in description
- Include popular terms: AI, automation, CI/CD
- Keep description under 125 characters for marketplace display

#### User Experience

- Provide clear examples
- Document all inputs/outputs
- Include troubleshooting guides
- Maintain responsive support

#### Quality Maintenance

- Regular dependency updates
- Security vulnerability monitoring
- Performance optimization
- User feedback incorporation

### Security Considerations

#### Token Management

- Never commit tokens to repository
- Use repository secrets for sensitive data
- Implement least-privilege access
- Regular token rotation

#### Workflow Security

- Pin action versions
- Validate inputs
- Limit workflow permissions
- Monitor for security issues

### Monitoring and Maintenance

#### Post-Release Monitoring

- Monitor marketplace listing
- Track usage analytics
- Watch for user issues
- Monitor security alerts

#### Regular Maintenance

- Dependency updates
- Security patches
- Documentation updates
- Feature enhancements

## Support

For issues with marketplace publishing:

1. **Check workflow logs**: Review failed workflow runs
2. **Validate locally**: Test action components manually
3. **Review requirements**: Ensure marketplace compliance
4. **Community support**: GitHub Community Forum
5. **GitHub Support**: For marketplace-specific issues

## Resources

- [GitHub Actions Documentation](https://docs.github.com/actions)
- [GitHub Marketplace Guidelines](https://docs.github.com/marketplace)
- [Semantic Versioning](https://semver.org/)
- [YAML Specification](https://yaml.org/spec/)
- [GitHub CLI Documentation](https://cli.github.com/manual/)