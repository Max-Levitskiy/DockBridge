# DockBridge Technology Stack

## Language & Runtime
- **Go 1.23+** with Go modules for dependency management
- Monorepo structure with client/server components

## Key Dependencies

### Core Libraries
- `github.com/spf13/cobra` - CLI framework for command structure
- `github.com/spf13/viper` - Configuration management (YAML, env vars)
- `github.com/hetznercloud/hcloud-go/v2` - Official Hetzner Cloud API client
- `github.com/docker/docker/client` - Official Docker Go SDK
- `golang.org/x/crypto/ssh` - SSH client implementation

### Testing & Quality
- `github.com/stretchr/testify` - Testing framework with assertions and mocks
- Standard Go testing with TDD approach
- GitHub Actions for CI/CD

## Build System

### Common Commands
```bash
# Build both client and server binaries
make build

# Build individual components
make client    # Builds bin/dockbridge-client
make server    # Builds bin/dockbridge-server

# Development
make test      # Run all tests
make fmt       # Format code
make lint      # Run linting (requires golangci-lint)
make clean     # Clean build artifacts

# Dependencies
make deps      # Install and tidy dependencies
```

### Build Targets
- `bin/dockbridge-client` - Client binary
- `bin/dockbridge-server` - Server binary

## Configuration Management
- YAML-based configuration files in `configs/`
- Environment variable overrides supported
- Default config locations: `~/.dockbridge/configs/`
- Viper handles configuration loading and validation