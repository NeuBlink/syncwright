.PHONY: build clean test fmt vet install

# Build the binary
build:
	go build -o bin/syncwright ./cmd/syncwright

# Install the binary to GOPATH/bin
install:
	go install ./cmd/syncwright

# Clean build artifacts
clean:
	rm -rf bin/

# Run tests
test:
	go test ./...

# Format code
fmt:
	go fmt ./...

# Run go vet
vet:
	go vet ./...

# Run all checks
check: fmt vet test

# Initialize go modules
mod:
	go mod tidy
	go mod download

# Show help
help:
	@echo "Available targets:"
	@echo "  build    - Build the syncwright binary"
	@echo "  install  - Install syncwright to GOPATH/bin"
	@echo "  clean    - Remove build artifacts"
	@echo "  test     - Run tests"
	@echo "  fmt      - Format code"
	@echo "  vet      - Run go vet"
	@echo "  check    - Run fmt, vet, and test"
	@echo "  mod      - Tidy and download modules"
	@echo "  help     - Show this help"

# Create bin directory
bin:
	mkdir -p bin