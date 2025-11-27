# Makefile for doublezero-version-sync

# Variables
BINARY_NAME := doublezero-version-sync
BUILD_DIR := bin
LDFLAGS := -ldflags="-s -w"

# Build targets
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64

# Default target
.PHONY: all
all: build

# Local development build
.PHONY: build
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go mod tidy
	@CGO_ENABLED=0 go build -mod=mod $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/doublezero-version-sync

# Cross-platform build for all platforms
.PHONY: build-all
build-all:
	@echo "Building $(BINARY_NAME) for all platforms..."
	@echo "Debug: Current directory: $$(pwd)"
	@echo "Debug: Contents of cmd/:"
	@ls -la cmd/ || echo "cmd/ directory not found"
	@echo "Debug: Contents of cmd/doublezero-version-sync/:"
	@ls -la cmd/doublezero-version-sync/ || echo "cmd/doublezero-version-sync/ directory not found"
	@mkdir -p $(BUILD_DIR)
	@go mod tidy
	@VERSION=$$(cat cmd/version.txt | tr -d '\n'); \
	for platform in $(PLATFORMS); do \
		OS=$$(echo $$platform | cut -d'/' -f1); \
		ARCH=$$(echo $$platform | cut -d'/' -f2); \
		OUTPUT_NAME=$(BINARY_NAME)-$$VERSION-$$OS-$$ARCH; \
		echo "Building for $$OS/$$ARCH..."; \
		CGO_ENABLED=0 GOOS=$$OS GOARCH=$$ARCH go build -mod=mod $(LDFLAGS) -o $(BUILD_DIR)/$$OUTPUT_NAME ./cmd/doublezero-version-sync; \
	done
	@echo "Compressing binaries..."
	@cd $(BUILD_DIR) && \
	for binary in $(BINARY_NAME)-*; do \
		if [ -f "$$binary" ] && [[ ! "$$binary" == *.sha256 ]]; then \
			echo "Compressing $$binary..."; \
			gzip "$$binary"; \
		fi; \
	done
	@echo "Generating checksums..."
	@cd $(BUILD_DIR) && \
	for binary in $(BINARY_NAME)-*.gz; do \
		if [ -f "$$binary" ]; then \
			echo "Generating checksum for $$binary..."; \
			sha256sum "$$binary" > "$$binary.sha256"; \
		fi; \
	done
	@echo "Build complete. Compressed binaries and checksums are in $(BUILD_DIR)/"

# Docker build (linux-amd64)
.PHONY: build-docker
build-docker:
	@echo "Building $(BINARY_NAME) for Docker..."
	@mkdir -p $(BUILD_DIR)
	@go mod tidy
	@VERSION=$$(cat cmd/version.txt | tr -d '\n'); \
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=mod $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-$$VERSION-linux-amd64 ./cmd/doublezero-version-sync

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	@go test -mod=mod -v ./...

# Mock validator server
.PHONY: mock-validator
mock-validator:
	@echo "Starting mock validator server..."
	@docker compose up -d mock-validator
	@echo "Mock validator is running on http://localhost:8899"

.PHONY: mock-validator-stop
mock-validator-stop:
	@echo "Stopping mock validator server..."
	@docker compose stop mock-validator

.PHONY: mock-validator-logs
mock-validator-logs:
	@docker compose logs -f mock-validator

# Local development
.PHONY: dev
dev:
	@echo "Running in development mode..."
	@go run ./cmd/doublezero-version-sync run --config config.yml

# Show help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build              - Build the binary locally"
	@echo "  build-all          - Build binaries for all platforms (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64)"
	@echo "  build-docker       - Build for Docker (linux-amd64)"
	@echo "  clean              - Clean build artifacts"
	@echo "  test               - Run tests"
	@echo "  dev                - Run in local development mode"
	@echo "  mock-validator     - Start the mock validator server in Docker"
	@echo "  mock-validator-stop - Stop the mock validator server"
	@echo "  mock-validator-logs - Show mock validator server logs"
	@echo "  help               - Show this help"

