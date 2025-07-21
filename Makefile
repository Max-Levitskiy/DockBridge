.PHONY: build test clean client server

# Build both client and server binaries
build: client server

# Build client binary
client:
	go build -o bin/dockbridge-client ./cmd/client

# Build server binary  
server:
	go build -o bin/dockbridge-server ./cmd/server

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