# DockBridge Project Structure

## Directory Organization

### Entry Points
- `cmd/client/` - Client CLI application entry point
- `cmd/server/` - Server HTTP application entry point

### Internal Packages
- `internal/client/` - Client-specific implementation
  - `cli/` - Cobra CLI commands and handlers
  - `config/` - Client configuration management
  - `docker/` - Docker socket proxy and client wrapper
  - `hetzner/` - Hetzner Cloud API client and provisioning
  - `keepalive/` - Keep-alive client implementation
  - `lockdetection/` - Screen lock detection (cross-platform)
  - `ssh/` - SSH client wrapper
- `internal/server/` - Server-specific implementation
  - `cli/` - Server CLI commands
  - `config/` - Server configuration
  - `docker/` - Docker command handler
- `internal/shared/` - Shared components between client/server
  - `config/` - Shared configuration types
- `internal/config/` - Global configuration utilities

### Configuration & Assets
- `configs/` - Default configuration files (YAML)
- `bin/` - Built binaries (gitignored)

### Development
- `Makefile` - Build automation and common tasks
- `.github/workflows/` - CI/CD pipelines

## Code Organization Principles

### Package Structure
- Use `internal/` for private packages not intended for external use
- Separate client and server concerns clearly
- Share common types and utilities in `internal/shared/`
- Keep CLI logic separate from business logic

### Configuration Pattern
- YAML files for default configuration
- Environment variable overrides
- Viper for configuration loading and validation
- Type-safe configuration structs

### Testing Strategy
- Co-locate tests with implementation (`*_test.go`)
- Use testify for assertions and test suites
- Mock external dependencies (Hetzner API, Docker API)
- Follow TDD methodology