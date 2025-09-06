# Dev PostgreSQL MCP Server Makefile

# Build variables
BINARY_NAME=dev-postgres-mcp
BUILD_DIR=bin
DIST_DIR=dist
CMD_DIR=./cmd/dev-postgres-mcp

# Go variables
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build flags
LDFLAGS=-ldflags="-s -w"

# Default target
.PHONY: all
all: clean deps test build

# Install dependencies
.PHONY: deps
deps:
	$(GOMOD) download
	$(GOMOD) verify

# Clean build artifacts
.PHONY: clean
clean:
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -rf $(DIST_DIR)

# Run tests
.PHONY: test
test:
	$(GOTEST) -v -race -coverprofile=coverage.out ./...

# Run integration tests
.PHONY: test-integration
test-integration:
	$(GOTEST) -v -tags=integration ./test/integration/...

# Run linting
.PHONY: lint
lint:
	golangci-lint run

# Build for current platform
.PHONY: build
build:
	mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)

# Build for all platforms
.PHONY: build-all
build-all: clean
	mkdir -p $(DIST_DIR)
	
	# Linux amd64
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_DIR)
	
	# Linux arm64
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 $(CMD_DIR)
	
	# macOS amd64
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_DIR)
	
	# macOS arm64
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 $(CMD_DIR)
	
	# Windows amd64
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe $(CMD_DIR)

# Install binary to GOPATH/bin
.PHONY: install
install:
	$(GOCMD) install $(CMD_DIR)

# Run the application
.PHONY: run
run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

# Run MCP server
.PHONY: serve
serve: build
	./$(BUILD_DIR)/$(BINARY_NAME) mcp serve

# Format code
.PHONY: fmt
fmt:
	$(GOCMD) fmt ./...

# Generate code coverage report
.PHONY: coverage
coverage: test
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Check for security vulnerabilities
.PHONY: security
security:
	gosec ./...

# Run all checks (test, lint, security)
.PHONY: check
check: test lint security

# Development setup
.PHONY: dev-setup
dev-setup:
	$(GOGET) github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GOGET) github.com/securecodewarrior/gosec/v2/cmd/gosec@latest

# Help target
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  all          - Clean, install deps, test, and build"
	@echo "  deps         - Download and verify dependencies"
	@echo "  clean        - Clean build artifacts"
	@echo "  test         - Run unit tests"
	@echo "  test-integration - Run integration tests"
	@echo "  lint         - Run linting"
	@echo "  build        - Build for current platform"
	@echo "  build-all    - Build for all platforms"
	@echo "  install      - Install binary to GOPATH/bin"
	@echo "  run          - Build and run the application"
	@echo "  serve        - Build and run MCP server"
	@echo "  fmt          - Format code"
	@echo "  coverage     - Generate coverage report"
	@echo "  security     - Check for security vulnerabilities"
	@echo "  check        - Run all checks (test, lint, security)"
	@echo "  dev-setup    - Install development tools"
	@echo "  help         - Show this help message"
