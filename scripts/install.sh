#!/bin/bash

# Syncwright Installation Script
# Downloads and installs the appropriate Syncwright binary for the current platform
# Used by GitHub Actions composite action for automated setup

set -euo pipefail

# Configuration
REPO_OWNER="NeuBlink"
REPO_NAME="syncwright"
BINARY_NAME="syncwright"
INSTALL_DIR="${PWD}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions - all output to stderr to avoid contaminating command substitution
log_info() {
    echo -e "${BLUE}[INFO]${NC} $*" >&2
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $*" >&2
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $*" >&2
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $*" >&2
}

# Error handling
cleanup() {
    local exit_code=$?
    if [ -n "${TEMP_DIR:-}" ] && [ -d "$TEMP_DIR" ]; then
        log_info "Cleaning up temporary directory: $TEMP_DIR"
        rm -rf "$TEMP_DIR"
    fi
    if [ $exit_code -ne 0 ]; then
        log_error "Installation failed with exit code $exit_code"
    fi
    exit $exit_code
}

trap cleanup EXIT

# Detect platform and architecture
detect_platform() {
    local os arch
    
    # Detect OS
    case "$(uname -s)" in
        Linux*)     os="linux" ;;
        Darwin*)    os="darwin" ;;
        CYGWIN*|MINGW*|MSYS*) os="windows" ;;
        *)          
            log_error "Unsupported operating system: $(uname -s)"
            return 1
            ;;
    esac
    
    # Detect architecture
    case "$(uname -m)" in
        x86_64|amd64)   arch="amd64" ;;
        aarch64|arm64)  arch="arm64" ;;
        armv7l)         arch="arm" ;;
        armv6l)         arch="armv6" ;;
        i386|i686)      arch="386" ;;
        *)              
            log_error "Unsupported architecture: $(uname -m)"
            return 1
            ;;
    esac
    
    echo "${os}_${arch}"
}

# Determine version from environment or GitHub Action context
get_version() {
    local version
    
    # Try GITHUB_ACTION_REF first (from action context)
    if [ -n "${GITHUB_ACTION_REF:-}" ]; then
        # Extract version from ref (e.g., refs/tags/v1.0.0 -> v1.0.0)
        version=$(echo "$GITHUB_ACTION_REF" | sed 's|^refs/tags/||')
        log_info "Using version from GITHUB_ACTION_REF: $version"
    # Try SYNCWRIGHT_VERSION environment variable
    elif [ -n "${SYNCWRIGHT_VERSION:-}" ] && [ "$SYNCWRIGHT_VERSION" != "latest" ]; then
        version="$SYNCWRIGHT_VERSION"
        log_info "Using version from SYNCWRIGHT_VERSION: $version"
    # Default to latest
    else
        version="latest"
        log_info "Using latest version"
    fi
    
    # Ensure version starts with 'v' if it's not 'latest'
    if [ "$version" != "latest" ] && [ "${version#v}" = "$version" ]; then
        version="v${version}"
    fi
    
    echo "$version"
}

# Check if required tools are available
check_dependencies() {
    local missing_tools=()
    
    # Check for download tools
    if ! command -v curl >/dev/null 2>&1 && ! command -v wget >/dev/null 2>&1; then
        missing_tools+=("curl or wget")
    fi
    
    # Check for verification tools
    if ! command -v sha256sum >/dev/null 2>&1 && ! command -v shasum >/dev/null 2>&1; then
        missing_tools+=("sha256sum or shasum")
    fi
    
    # Check for extraction tools
    if ! command -v tar >/dev/null 2>&1; then
        missing_tools+=("tar")
    fi
    
    if [ ${#missing_tools[@]} -ne 0 ]; then
        log_error "Missing required tools: ${missing_tools[*]}"
        return 1
    fi
    
    return 0
}

# Download file with retry logic
download_file() {
    local url="$1"
    local output="$2"
    local max_retries=3
    local retry_delay=2
    
    for ((i=1; i<=max_retries; i++)); do
        log_info "Downloading $url (attempt $i/$max_retries)"
        
        if command -v curl >/dev/null 2>&1; then
            if curl -fsSL --connect-timeout 10 --max-time 300 "$url" -o "$output"; then
                return 0
            fi
        elif command -v wget >/dev/null 2>&1; then
            if wget -q --timeout=10 --tries=1 "$url" -O "$output"; then
                return 0
            fi
        fi
        
        if [ $i -lt $max_retries ]; then
            log_warning "Download failed, retrying in ${retry_delay}s..."
            sleep $retry_delay
            retry_delay=$((retry_delay * 2))
        fi
    done
    
    log_error "Failed to download $url after $max_retries attempts"
    return 1
}

# Verify file checksum
verify_checksum() {
    local file="$1"
    local checksums_file="$2"
    local filename
    filename=$(basename "$file")
    
    log_info "Verifying checksum for $filename"
    
    # Extract expected checksum for our file
    local expected_checksum
    if ! expected_checksum=$(grep "$filename" "$checksums_file" | cut -d' ' -f1); then
        log_error "Checksum not found for $filename in checksums file"
        return 1
    fi
    
    if [ -z "$expected_checksum" ]; then
        log_error "Empty checksum found for $filename"
        return 1
    fi
    
    # Calculate actual checksum
    local actual_checksum
    if command -v sha256sum >/dev/null 2>&1; then
        actual_checksum=$(sha256sum "$file" | cut -d' ' -f1)
    elif command -v shasum >/dev/null 2>&1; then
        actual_checksum=$(shasum -a 256 "$file" | cut -d' ' -f1)
    else
        log_error "No checksum tool available"
        return 1
    fi
    
    if [ "$expected_checksum" = "$actual_checksum" ]; then
        log_success "Checksum verification passed"
        return 0
    else
        log_error "Checksum verification failed!"
        log_error "Expected: $expected_checksum"
        log_error "Actual:   $actual_checksum"
        return 1
    fi
}

# Download and install binary
install_binary() {
    local platform="$1"
    local version="$2"
    
    # Create temporary directory
    TEMP_DIR=$(mktemp -d)
    log_info "Using temporary directory: $TEMP_DIR"
    
    # Construct download URLs
    local base_url="https://github.com/${REPO_OWNER}/${REPO_NAME}/releases"
    local archive_name="${BINARY_NAME}_${platform}.tar.gz"
    local checksums_name="checksums.txt"
    
    if [ "$version" = "latest" ]; then
        local download_url="${base_url}/latest/download/${archive_name}"
        local checksums_url="${base_url}/latest/download/${checksums_name}"
    else
        local download_url="${base_url}/download/${version}/${archive_name}"
        local checksums_url="${base_url}/download/${version}/${checksums_name}"
    fi
    
    # Download files
    local archive_path="${TEMP_DIR}/${archive_name}"
    local checksums_path="${TEMP_DIR}/${checksums_name}"
    
    if ! download_file "$download_url" "$archive_path"; then
        log_error "Failed to download binary archive"
        return 1
    fi
    
    if ! download_file "$checksums_url" "$checksums_path"; then
        log_warning "Failed to download checksums file, skipping verification"
    else
        # Verify checksum
        if ! verify_checksum "$archive_path" "$checksums_path"; then
            log_error "Checksum verification failed"
            return 1
        fi
    fi
    
    # Extract archive
    log_info "Extracting binary archive"
    if ! tar -xzf "$archive_path" -C "$TEMP_DIR"; then
        log_error "Failed to extract archive"
        return 1
    fi
    
    # Find the binary (handle different archive structures)
    local binary_path
    if [ -f "${TEMP_DIR}/${BINARY_NAME}" ]; then
        binary_path="${TEMP_DIR}/${BINARY_NAME}"
    elif [ -f "${TEMP_DIR}/bin/${BINARY_NAME}" ]; then
        binary_path="${TEMP_DIR}/bin/${BINARY_NAME}"
    else
        # Search for binary in extracted files
        binary_path=$(find "$TEMP_DIR" -name "$BINARY_NAME" -type f | head -1)
        if [ -z "$binary_path" ]; then
            log_error "Binary not found in extracted archive"
            return 1
        fi
    fi
    
    # Install binary
    local target_path="${INSTALL_DIR}/${BINARY_NAME}"
    log_info "Installing binary to $target_path"
    
    if ! cp "$binary_path" "$target_path"; then
        log_error "Failed to copy binary to $target_path"
        return 1
    fi
    
    # Make executable
    if ! chmod +x "$target_path"; then
        log_error "Failed to make binary executable"
        return 1
    fi
    
    log_success "Binary installed successfully"
    return 0
}

# Fallback: install via go install
install_via_go() {
    log_warning "Attempting fallback installation via Go toolchain"
    
    if ! command -v go >/dev/null 2>&1; then
        log_error "Go toolchain not available for fallback installation"
        return 1
    fi
    
    local go_package="github.com/${REPO_OWNER}/${REPO_NAME}/cmd/${BINARY_NAME}@latest"
    log_info "Installing via: go install $go_package"
    
    # Install to temporary location first
    local temp_gopath
    temp_gopath=$(mktemp -d)
    
    if ! GOPATH="$temp_gopath" GOBIN="${INSTALL_DIR}" go install "$go_package"; then
        log_error "Failed to install via go install"
        rm -rf "$temp_gopath"
        return 1
    fi
    
    rm -rf "$temp_gopath"
    log_success "Installed via Go toolchain"
    return 0
}

# Verify installation
verify_installation() {
    local binary_path="${INSTALL_DIR}/${BINARY_NAME}"
    
    if [ ! -f "$binary_path" ]; then
        log_error "Binary not found at $binary_path"
        return 1
    fi
    
    if [ ! -x "$binary_path" ]; then
        log_error "Binary is not executable"
        return 1
    fi
    
    # Test binary execution
    log_info "Testing binary execution"
    if ! "$binary_path" --version >/dev/null 2>&1; then
        log_warning "Binary version check failed, but installation appears successful"
    else
        local version_output
        version_output=$("$binary_path" --version 2>/dev/null || echo "version unavailable")
        log_success "Installation verified: $version_output"
    fi
    
    return 0
}

# Main installation function
main() {
    log_info "Starting Syncwright installation"
    log_info "Install directory: $INSTALL_DIR"
    
    # Check dependencies
    if ! check_dependencies; then
        log_error "Dependency check failed"
        return 1
    fi
    
    # Detect platform
    local platform
    if ! platform=$(detect_platform); then
        log_error "Platform detection failed"
        return 1
    fi
    log_info "Detected platform: $platform"
    
    # Get version
    local version
    version=$(get_version)
    log_info "Target version: $version"
    
    # Try binary installation first
    if install_binary "$platform" "$version"; then
        if verify_installation; then
            log_success "Syncwright installed successfully via binary download"
            return 0
        else
            log_error "Binary installation verification failed"
        fi
    else
        log_warning "Binary installation failed"
    fi
    
    # Try Go fallback if binary installation failed
    log_info "Attempting fallback installation method"
    if install_via_go; then
        if verify_installation; then
            log_success "Syncwright installed successfully via Go toolchain"
            return 0
        else
            log_error "Go installation verification failed"
        fi
    else
        log_error "Go fallback installation failed"
    fi
    
    log_error "All installation methods failed"
    return 1
}

# Handle command line arguments
if [ "${1:-}" = "--help" ] || [ "${1:-}" = "-h" ]; then
    cat << EOF
Syncwright Installation Script

This script downloads and installs the appropriate Syncwright binary for your platform.

Usage: $0 [OPTIONS]

Options:
  -h, --help     Show this help message

Environment Variables:
  GITHUB_ACTION_REF     GitHub Action reference (e.g., refs/tags/v1.0.0)
  SYNCWRIGHT_VERSION    Specific version to install (default: latest)
  INSTALL_DIR          Installation directory (default: current directory)

The script will:
1. Detect your OS and architecture
2. Download the appropriate binary from GitHub releases
3. Verify the download using SHA256 checksums
4. Extract and install the binary
5. Fall back to 'go install' if binary download fails

Requirements:
- curl or wget (for downloading)
- tar (for extraction)
- sha256sum or shasum (for verification, optional but recommended)
- go (for fallback installation, optional)

EOF
    exit 0
fi

# Run main installation
main "$@"