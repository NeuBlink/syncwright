#!/bin/bash

# Version management script for Syncwright
# Provides utilities for version checking and validation

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $*"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $*"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $*"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $*"
}

# Get current version from git tags
get_current_version() {
    if git describe --tags --abbrev=0 >/dev/null 2>&1; then
        git describe --tags --abbrev=0
    else
        echo "v0.0.0"
    fi
}

# Validate version format
validate_version() {
    local version="$1"
    
    if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9-]+)?(\+[a-zA-Z0-9-]+)?$ ]]; then
        log_error "Invalid version format: $version"
        log_error "Expected format: vX.Y.Z[-prerelease][+build]"
        return 1
    fi
    
    return 0
}

# Compare two versions
compare_versions() {
    local version1="$1"
    local version2="$2"
    
    # Remove 'v' prefix and any prerelease/build metadata
    local clean1=$(echo "$version1" | sed 's/^v//' | sed 's/[-+].*//')
    local clean2=$(echo "$version2" | sed 's/^v//' | sed 's/[-+].*//')
    
    # Use sort -V for version comparison
    if [[ "$clean1" == "$clean2" ]]; then
        echo "equal"
    elif [[ "$(printf '%s\n' "$clean1" "$clean2" | sort -V | head -n1)" == "$clean1" ]]; then
        echo "less"
    else
        echo "greater"
    fi
}

# Check if version exists in git tags
version_exists() {
    local version="$1"
    git tag -l | grep -q "^${version}$"
}

# Get next version based on type
get_next_version() {
    local current_version="$1"
    local bump_type="$2"
    local prerelease="${3:-false}"
    local prerelease_type="${4:-alpha}"
    
    # Remove 'v' prefix and any existing prerelease/build metadata
    local clean_version=$(echo "$current_version" | sed 's/^v//' | sed 's/[-+].*//')
    
    # Split version into parts
    IFS='.' read -r major minor patch <<< "$clean_version"
    
    # Bump version based on type
    case "$bump_type" in
        "major")
            major=$((major + 1))
            minor=0
            patch=0
            ;;
        "minor")
            minor=$((minor + 1))
            patch=0
            ;;
        "patch")
            patch=$((patch + 1))
            ;;
        *)
            log_error "Invalid bump type: $bump_type"
            log_error "Valid types: major, minor, patch"
            return 1
            ;;
    esac
    
    # Construct new version
    local new_version="v${major}.${minor}.${patch}"
    
    # Add prerelease suffix if requested
    if [[ "$prerelease" == "true" ]]; then
        new_version="${new_version}-${prerelease_type}.1"
    fi
    
    echo "$new_version"
}

# Show version information
show_version_info() {
    local current_version
    current_version=$(get_current_version)
    
    echo "=== Syncwright Version Information ==="
    echo "Current version: $current_version"
    echo "Project root: $PROJECT_ROOT"
    echo "Git repository: $(git remote get-url origin 2>/dev/null || echo "Not available")"
    echo "Git branch: $(git branch --show-current 2>/dev/null || echo "Not available")"
    echo "Git commit: $(git rev-parse --short HEAD 2>/dev/null || echo "Not available")"
    echo "======================================"
    
    # Show recent tags
    if git tag >/dev/null 2>&1; then
        echo ""
        echo "Recent tags:"
        git tag --sort=-version:refname | head -5 | sed 's/^/  /'
    fi
}

# List all versions
list_versions() {
    echo "All versions (most recent first):"
    if git tag >/dev/null 2>&1; then
        git tag --sort=-version:refname | sed 's/^/  /'
    else
        echo "  No versions found"
    fi
}

# Check for version consistency
check_version_consistency() {
    local current_version
    current_version=$(get_current_version)
    
    log_info "Checking version consistency..."
    
    # Check if current version is valid
    if ! validate_version "$current_version"; then
        log_error "Current version is invalid"
        return 1
    fi
    
    # Check for any version-related files that might need updating
    local files_to_check=(
        "go.mod"
        "action.yml"
        "README.md"
        ".goreleaser.yml"
    )
    
    for file in "${files_to_check[@]}"; do
        if [[ -f "$PROJECT_ROOT/$file" ]]; then
            log_info "Checking $file for version references..."
            # This is a placeholder - you can add specific version checks here
        fi
    done
    
    log_success "Version consistency check completed"
}

# Show next possible versions
show_next_versions() {
    local current_version
    current_version=$(get_current_version)
    
    echo "Current version: $current_version"
    echo ""
    echo "Next possible versions:"
    echo "  Patch:  $(get_next_version "$current_version" "patch")"
    echo "  Minor:  $(get_next_version "$current_version" "minor")"
    echo "  Major:  $(get_next_version "$current_version" "major")"
    echo ""
    echo "Prerelease versions:"
    echo "  Alpha:  $(get_next_version "$current_version" "patch" "true" "alpha")"
    echo "  Beta:   $(get_next_version "$current_version" "patch" "true" "beta")"
    echo "  RC:     $(get_next_version "$current_version" "patch" "true" "rc")"
}

# Main function
main() {
    case "${1:-help}" in
        "current"|"get")
            get_current_version
            ;;
        "info"|"show")
            show_version_info
            ;;
        "list"|"all")
            list_versions
            ;;
        "validate")
            local version="${2:-$(get_current_version)}"
            if validate_version "$version"; then
                log_success "Version $version is valid"
            else
                exit 1
            fi
            ;;
        "compare")
            if [[ $# -lt 3 ]]; then
                log_error "Usage: $0 compare <version1> <version2>"
                exit 1
            fi
            result=$(compare_versions "$2" "$3")
            echo "$2 is $result than $3"
            ;;
        "next")
            local bump_type="${2:-patch}"
            local prerelease="${3:-false}"
            local prerelease_type="${4:-alpha}"
            get_next_version "$(get_current_version)" "$bump_type" "$prerelease" "$prerelease_type"
            ;;
        "next-all"|"possibilities")
            show_next_versions
            ;;
        "exists")
            if [[ $# -lt 2 ]]; then
                log_error "Usage: $0 exists <version>"
                exit 1
            fi
            if version_exists "$2"; then
                log_info "Version $2 exists"
                exit 0
            else
                log_info "Version $2 does not exist"
                exit 1
            fi
            ;;
        "check"|"consistency")
            check_version_consistency
            ;;
        "help"|"--help"|"-h")
            cat << EOF
Syncwright Version Management Script

Usage: $0 <command> [options]

Commands:
  current, get              Get current version
  info, show               Show detailed version information
  list, all                List all versions
  validate [version]       Validate version format (current if not specified)
  compare <v1> <v2>        Compare two versions
  next <type> [prerelease] [type]  Get next version (patch/minor/major)
  next-all, possibilities  Show all possible next versions
  exists <version>         Check if version exists in git tags
  check, consistency       Check version consistency across files
  help                     Show this help message

Examples:
  $0 current               # Show current version
  $0 next minor            # Show next minor version
  $0 next patch true alpha # Show next alpha prerelease
  $0 validate v1.2.3       # Validate specific version
  $0 compare v1.0.0 v1.1.0 # Compare two versions

EOF
            ;;
        *)
            log_error "Unknown command: $1"
            log_info "Run '$0 help' for usage information"
            exit 1
            ;;
    esac
}

# Change to project root
cd "$PROJECT_ROOT"

# Run main function with all arguments
main "$@"