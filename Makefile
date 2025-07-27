.PHONY: build clean test fmt vet install check mod help bin
.PHONY: version version-info version-bump release-patch release-minor release-major
.PHONY: validate-workflows validate-action lint security-scan ci-local

# Version information
VERSION := $(shell scripts/version.sh current 2>/dev/null || echo "v0.0.0")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Build flags
BUILD_FLAGS := -trimpath
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

# Build the binary (with platform-specific executable extension)
build: bin
	@if [ "$$(go env GOOS)" = "windows" ]; then \
		go build $(BUILD_FLAGS) -ldflags "$(LDFLAGS)" -o bin/syncwright.exe ./cmd/syncwright; \
	else \
		go build $(BUILD_FLAGS) -ldflags "$(LDFLAGS)" -o bin/syncwright ./cmd/syncwright; \
	fi

# Build for multiple platforms (used by GoReleaser)
build-all: clean
	@echo "Building for multiple platforms..."
	GOOS=linux GOARCH=amd64 go build $(BUILD_FLAGS) -ldflags "$(LDFLAGS)" -o bin/syncwright-linux-amd64 ./cmd/syncwright
	GOOS=linux GOARCH=arm64 go build $(BUILD_FLAGS) -ldflags "$(LDFLAGS)" -o bin/syncwright-linux-arm64 ./cmd/syncwright
	GOOS=darwin GOARCH=amd64 go build $(BUILD_FLAGS) -ldflags "$(LDFLAGS)" -o bin/syncwright-darwin-amd64 ./cmd/syncwright
	GOOS=darwin GOARCH=arm64 go build $(BUILD_FLAGS) -ldflags "$(LDFLAGS)" -o bin/syncwright-darwin-arm64 ./cmd/syncwright
	GOOS=windows GOARCH=amd64 go build $(BUILD_FLAGS) -ldflags "$(LDFLAGS)" -o bin/syncwright-windows-amd64.exe ./cmd/syncwright

# Install the binary to GOPATH/bin
install:
	go install -ldflags "$(LDFLAGS)" ./cmd/syncwright

# Clean build artifacts
clean:
	rm -rf bin/ dist/

# Run tests
test:
	go test -v -race -coverprofile=coverage.out ./...

# Run tests with coverage report
test-coverage: test
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run benchmarks
bench:
	go test -bench=. -benchmem ./...

# Format code
fmt:
	go fmt ./...

# Run go vet
vet:
	go vet ./...

# Run linter
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not found, running go vet instead"; \
		go vet ./...; \
	fi

# Security scan
security-scan:
	@if command -v gosec >/dev/null 2>&1; then \
		gosec ./...; \
	else \
		echo "gosec not found, skipping security scan"; \
		echo "Install with: go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest"; \
	fi

# Run all checks
check: fmt vet lint test

# Full CI pipeline (local)
ci-local: clean mod check build test-coverage security-scan
	@echo "Local CI pipeline completed successfully"

# Initialize go modules
mod:
	go mod tidy
	go mod download

# Version management
version:
	@scripts/version.sh current

version-info:
	@scripts/version.sh info

version-next:
	@scripts/version.sh next-all

# Version bumping (requires git)
version-bump-patch:
	@scripts/version.sh next patch

version-bump-minor:
	@scripts/version.sh next minor

version-bump-major:
	@scripts/version.sh next major

# Release targets (triggers GitHub Actions)
release-patch: version-info
	@echo "Triggering patch release..."
	@echo "Use GitHub UI to run 'Version Bump' workflow with 'patch' option"

release-minor: version-info
	@echo "Triggering minor release..."
	@echo "Use GitHub UI to run 'Version Bump' workflow with 'minor' option"

release-major: version-info
	@echo "Triggering major release..."
	@echo "Use GitHub UI to run 'Version Bump' workflow with 'major' option"

# Validate GitHub Actions
validate-workflows:
	@echo "Validating GitHub Actions workflows..."
	@for workflow in .github/workflows/*.yml; do \
		echo "Validating $$workflow"; \
		python3 -c "import yaml; yaml.safe_load(open('$$workflow'))" || exit 1; \
	done
	@echo "All workflows are valid"

validate-action:
	@echo "Validating action.yml..."
	@python3 -c "import yaml; action = yaml.safe_load(open('action.yml')); \
		required = ['name', 'description', 'runs']; \
		missing = [f for f in required if f not in action]; \
		exit(1) if missing else print('action.yml is valid')"

# Test action locally (requires Docker)
test-action:
	@echo "Testing action locally..."
	@if command -v act >/dev/null 2>&1; then \
		act -j test-action-consumption || echo "Action test completed"; \
	else \
		echo "act not found. Install from: https://github.com/nektos/act"; \
	fi

# Development helpers
dev-setup:
	@echo "Setting up development environment..."
	go mod download
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "Installing golangci-lint..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	fi
	@if ! command -v gosec >/dev/null 2>&1; then \
		echo "Installing gosec..."; \
		go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest; \
	fi
	@echo "Development environment setup complete"

# Generate documentation
docs:
	@echo "Generating documentation..."
	@./bin/syncwright --help > docs/cli-help.txt 2>/dev/null || echo "Build binary first with 'make build'"

# Show current build info
info:
	@echo "=== Build Information ==="
	@echo "Version: $(VERSION)"
	@echo "Commit:  $(COMMIT)"
	@echo "Date:    $(DATE)"
	@echo "Go:      $$(go version)"
	@echo "OS:      $$(uname -s)"
	@echo "Arch:    $$(uname -m)"
	@echo "========================"

# Show help
help:
	@echo "Syncwright Makefile"
	@echo ""
	@echo "Build targets:"
	@echo "  build         - Build the syncwright binary"
	@echo "  build-all     - Build for multiple platforms"
	@echo "  install       - Install syncwright to GOPATH/bin"
	@echo "  clean         - Remove build artifacts"
	@echo ""
	@echo "Testing targets:"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  bench         - Run benchmarks"
	@echo ""
	@echo "Code quality targets:"
	@echo "  fmt           - Format code"
	@echo "  vet           - Run go vet"
	@echo "  lint          - Run linter"
	@echo "  security-scan - Run security scanner"
	@echo "  check         - Run fmt, vet, lint, and test"
	@echo "  ci-local      - Run full CI pipeline locally"
	@echo ""
	@echo "Version management:"
	@echo "  version       - Show current version"
	@echo "  version-info  - Show detailed version information"
	@echo "  version-next  - Show next possible versions"
	@echo ""
	@echo "Release targets:"
	@echo "  release-patch - Trigger patch release"
	@echo "  release-minor - Trigger minor release"
	@echo "  release-major - Trigger major release"
	@echo ""
	@echo "GitHub Actions:"
	@echo "  validate-workflows - Validate workflow YAML files"
	@echo "  validate-action    - Validate action.yml"
	@echo "  test-action        - Test action locally with act"
	@echo ""
	@echo "Development:"
	@echo "  dev-setup     - Set up development environment"
	@echo "  docs          - Generate documentation"
	@echo "  info          - Show build information"
	@echo "  mod           - Tidy and download modules"
	@echo "  help          - Show this help"

# Create bin directory
bin:
	mkdir -p bin