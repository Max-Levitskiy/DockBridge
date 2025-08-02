# SSH Docker Proxy Makefile

.PHONY: build test clean deps fmt lint help

# Build configuration
BINARY_NAME=ssh-docker-proxy
BUILD_DIR=bin
CMD_DIR=cmd/ssh-docker-proxy

# Go configuration
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOCLEAN=$(GOCMD) clean
GOGET=$(GOCMD) get
GOFMT=$(GOCMD) fmt
GOMOD=$(GOCMD) mod

# Default target
all: build

## Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)/main.go

## Run all tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./internal/...
	$(GOTEST) -v ./pkg/...

## Run unit tests only
test-unit:
	@echo "Running unit tests..."
	$(GOTEST) -v ./internal/... ./pkg/...

## Run integration tests
test-integration:
	@echo "Running integration tests..."
	$(GOTEST) -v ./test/integration/...

## Run BDD tests with Godog
test-bdd:
	@echo "Running BDD tests..."
	$(GOTEST) -v ./test/features/...

## Run all tests including integration
test-all: test test-integration test-bdd

## Install dependencies
deps:
	@echo "Installing dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

## Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) ./...

## Run linter (requires golangci-lint)
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

## Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)

## Install the binary
install: build
	@echo "Installing $(BINARY_NAME)..."
	cp $(BUILD_DIR)/$(BINARY_NAME) $(GOPATH)/bin/

## Show help
help:
	@echo "Available targets:"
	@echo "  build           Build the binary"
	@echo "  test            Run unit tests"
	@echo "  test-unit       Run unit tests only"
	@echo "  test-integration Run integration tests"
	@echo "  test-bdd        Run BDD tests with Godog"
	@echo "  test-all        Run all tests"
	@echo "  deps            Install dependencies"
	@echo "  fmt             Format code"
	@echo "  lint            Run linter"
	@echo "  clean           Clean build artifacts"
	@echo "  install         Install binary to GOPATH/bin"
	@echo "  help            Show this help message"