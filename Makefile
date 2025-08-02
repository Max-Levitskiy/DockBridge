.PHONY: build test clean dockbridge

# Build the main dockbridge binary
build: dockbridge

# Build dockbridge binary
dockbridge:
	go build -o bin/dockbridge ./cmd/dockbridge

# Legacy targets for backward compatibility
client: dockbridge
server: dockbridge

# Run tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -rf bin/

# Install dependencies
deps:
	go mod tidy
	go mod download

# Run linting (requires golangci-lint)
lint:
	golangci-lint run

# Format code
fmt:
	go fmt ./...