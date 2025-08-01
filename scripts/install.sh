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
        version="${GITHUB_ACTION_REF#refs/tags/}"
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

# Download file with retry logic and faster failure for CI
download_file() {
    local url="$1"
    local output="$2"
    local max_retries=3
    local retry_delay=1
    
    # Reduce retries and timeouts in CI environment
    if [ -n "${CI:-}" ] || [ -n "${GITHUB_ACTIONS:-}" ]; then
        max_retries=2
        retry_delay=1
    fi
    
    for ((i=1; i<=max_retries; i++)); do
        log_info "Downloading $url (attempt $i/$max_retries)"
        
        if command -v curl >/dev/null 2>&1; then
            # Shorter timeouts for faster failure in CI
            if curl -fsSL --connect-timeout 5 --max-time 30 "$url" -o "$output" 2>/dev/null; then
                return 0
            fi
        elif command -v wget >/dev/null 2>&1; then
            if wget -q --timeout=5 --tries=1 "$url" -O "$output" 2>/dev/null; then
                return 0
            fi
        fi
        
        if [ $i -lt $max_retries ]; then
            log_warning "Download failed, retrying in ${retry_delay}s..."
            sleep $retry_delay
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

# Check if releases exist before attempting download
check_releases_exist() {
    local version="$1"
    local base_url="https://github.com/${REPO_OWNER}/${REPO_NAME}/releases"
    
    # In CI environments, do a quick check first
    if [ -n "${CI:-}" ] || [ -n "${GITHUB_ACTIONS:-}" ]; then
        local check_url
        if [ "$version" = "latest" ]; then
            check_url="${base_url}/latest"
        else
            check_url="${base_url}/tag/${version}"
        fi
        
        # Quick HEAD request to check if releases exist
        if command -v curl >/dev/null 2>&1; then
            if ! curl -fsSL --connect-timeout 3 --max-time 10 -I "$check_url" >/dev/null 2>&1; then
                log_warning "No releases found at $check_url - skipping binary download"
                return 1
            fi
        fi
    fi
    
    return 0
}

# Download and install binary
install_binary() {
    local platform="$1"
    local version="$2"
    
    # Quick check if releases exist before trying to download
    if ! check_releases_exist "$version"; then
        log_warning "No releases available, skipping binary installation"
        return 1
    fi
    
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
        log_warning "Failed to download binary archive from GitHub releases"
        log_info "This may indicate that no releases are available yet for this repository"
        log_info "Release URL attempted: $download_url"
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

# Fallback: build from source if available locally
install_via_source_build() {
    log_warning "Attempting fallback installation by building from source"
    
    if ! command -v go >/dev/null 2>&1; then
        log_error "Go toolchain not available for source build"
        return 1
    fi
    
    # Check if we're running from within the source repository
    local source_dir=""
    
    # Try to find source in common locations
    local possible_sources=(
        "${GITHUB_WORKSPACE:-}"
        "${GITHUB_ACTION_PATH:-}"
        "$(pwd)"
        "/github/workspace"
    )
    
    for dir in "${possible_sources[@]}"; do
        if [ -n "$dir" ] && [ -f "$dir/go.mod" ] && [ -f "$dir/cmd/${BINARY_NAME}/main.go" ]; then
            source_dir="$dir"
            log_info "Found source repository at: $source_dir"
            break
        fi
    done
    
    if [ -z "$source_dir" ]; then
        log_warning "Source repository not found in expected locations"
        log_info "Attempting go install with module path resolution..."
        
        # Try go install but handle module path conflicts gracefully
        local go_package="github.com/${REPO_OWNER}/${REPO_NAME}/cmd/${BINARY_NAME}@latest"
        log_info "Attempting: go install $go_package"
        
        # Create temporary directory for isolated build
        local temp_dir
        temp_dir=$(mktemp -d)
        local original_dir
        original_dir=$(pwd)
        
        cd "$temp_dir"
        
        # Initialize a temporary module to avoid conflicts
        go mod init temp-install 2>/dev/null || true
        
        # Try to install, but capture and handle errors gracefully
        if GOBIN="${INSTALL_DIR}" go install "$go_package" 2>/dev/null; then
            cd "$original_dir"
            rm -rf "$temp_dir"
            log_success "Installed via go install (with module path resolution)"
            return 0
        else
            cd "$original_dir"
            rm -rf "$temp_dir"
            log_warning "go install failed due to module path conflicts"
            return 1
        fi
    fi
    
    # Build from local source
    local original_dir
    original_dir=$(pwd)
    cd "$source_dir"
    
    log_info "Building from source in: $source_dir"
    
    # Ensure we have a clean build environment
    if ! go mod tidy; then
        log_warning "go mod tidy failed, proceeding anyway"
    fi
    
    # Build the binary
    local target_path="${INSTALL_DIR}/${BINARY_NAME}"
    if go build -o "$target_path" "./cmd/${BINARY_NAME}"; then
        cd "$original_dir"
        log_success "Built and installed from source"
        return 0
    else
        cd "$original_dir"
        log_error "Failed to build from source"
        return 1
    fi
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
    
    # Try source build fallback if binary installation failed
    log_info "Attempting fallback installation method"
    if install_via_source_build; then
        if verify_installation; then
            log_success "Syncwright installed successfully via source build"
            return 0
        else
            log_error "Source build installation verification failed"
        fi
    else
        log_error "Source build fallback installation failed"
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
5. Fall back to building from source if releases are unavailable

Requirements:
- curl or wget (for downloading)
- tar (for extraction)
- sha256sum or shasum (for verification, optional but recommended)
- go (for source build fallback, optional)

EOF
    exit 0
fi

# Run main installation
main "$@"