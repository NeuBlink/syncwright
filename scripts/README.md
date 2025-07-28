# Syncwright Installation Scripts

This directory contains scripts for installing and testing the Syncwright binary across different platforms.

## Files

### `install.sh`
The main installation script used by the GitHub Actions composite action to download and install the appropriate Syncwright binary for the current platform.

**Key Features:**
- Cross-platform support (Linux, macOS, Windows)
- Multi-architecture support (amd64, arm64, arm, 386)
- SHA256 checksum verification for security
- Retry logic for reliable downloads
- Graceful fallback to `go install` if binary download fails
- Comprehensive error handling and logging
- GitHub Actions integration via environment variables

**Usage:**
```bash
# Basic installation (installs to current directory)
./scripts/install.sh

# Show help
./scripts/install.sh --help

# Install specific version
SYNCWRIGHT_VERSION=v1.0.0 ./scripts/install.sh

# Install to custom directory
INSTALL_DIR=/usr/local/bin ./scripts/install.sh
```

**Environment Variables:**
- `GITHUB_ACTION_REF`: GitHub Action reference (e.g., `refs/tags/v1.0.0`)
- `SYNCWRIGHT_VERSION`: Specific version to install (default: `latest`)
- `INSTALL_DIR`: Installation directory (default: current directory)

### `test-install.sh`
Test script that validates the installation script functionality without requiring actual binary downloads.

**Usage:**
```bash
./scripts/test-install.sh
```

## GitHub Actions Integration

The `install.sh` script is designed to work seamlessly with the GitHub Actions composite action. It:

1. Detects the platform and architecture automatically
2. Uses `GITHUB_ACTION_REF` to determine the version when running in GitHub Actions
3. Downloads the appropriate binary from GitHub releases
4. Verifies integrity using SHA256 checksums
5. Installs the binary to the workspace for use in subsequent steps
6. Falls back to Go toolchain installation if binary download fails

## Security Features

- **Checksum Verification**: All downloads are verified against SHA256 checksums
- **Secure Downloads**: Uses HTTPS with proper SSL/TLS verification
- **Input Validation**: Validates platform and architecture detection
- **Retry Logic**: Implements exponential backoff for reliable downloads
- **Clean Fallbacks**: Graceful degradation when preferred methods fail

## Error Handling

The script includes comprehensive error handling:
- Clear error messages with color-coded output
- Automatic cleanup of temporary files
- Graceful handling of missing dependencies
- Detailed logging for debugging
- Exit codes that indicate success/failure status

## Platform Support

| Platform | Architecture | Status |
|----------|--------------|--------|
| Linux    | amd64        | ✅ Supported |
| Linux    | arm64        | ✅ Supported |
| Linux    | arm          | ✅ Supported |
| Linux    | 386          | ✅ Supported |
| macOS    | amd64        | ✅ Supported |
| macOS    | arm64        | ✅ Supported |
| Windows  | amd64        | ✅ Supported |
| Windows  | arm64        | ✅ Supported |
| Windows  | 386          | ✅ Supported |

## Dependencies

**Required:**
- `curl` or `wget` (for downloading)
- `tar` (for extraction)

**Optional but Recommended:**
- `sha256sum` or `shasum` (for checksum verification)
- `go` (for fallback installation)

## Troubleshooting

### Common Issues

1. **Download Failures**: Check network connectivity and GitHub releases availability
2. **Checksum Mismatches**: Verify the release hasn't been tampered with
3. **Permission Errors**: Ensure write permissions in the installation directory
4. **Missing Dependencies**: Install required tools (curl/wget, tar)

### Debug Mode

Enable verbose logging by examining the script output - all operations include detailed status information.

### Manual Fallback

If the script fails entirely, you can manually install using:
```bash
go install github.com/NeuBlink/syncwright/cmd/syncwright@latest
```

This requires the Go toolchain to be installed on your system.