# DockBridge Project Structure Analysis

## Current Structure Overview

The DockBridge project currently has a mixed structure that deviates from Go best practices in several ways:

### Directory Structure
```
DockBridge/
├── cmd/
│   ├── client/main.go          # Client entry point
│   └── server/main.go          # Server entry point
├── internal/
│   ├── client/                 # Client-specific code
│   │   ├── cli/               # CLI commands
│   │   ├── config/            # Configuration management
│   │   ├── docker/            # Custom Docker daemon implementation
│   │   ├── hetzner/           # Hetzner Cloud integration
│   │   ├── keepalive/         # Keep-alive mechanism (placeholder)
│   │   ├── lockdetection/     # Lock detection (placeholder)
│   │   └── ssh/               # SSH client wrapper
│   ├── server/                # Server-specific code
│   │   ├── cli/               # Server CLI
│   │   ├── config/            # Server configuration
│   │   └── docker/            # Server Docker handling (placeholder)
│   ├── shared/                # Shared components
│   │   └── config/            # Shared configuration types
│   └── config/                # Global configuration utilities
├── pkg/                       # Public packages
│   ├── errors/                # Error handling utilities
│   └── logger/                # Logging utilities
├── ssh-docker-proxy/          # Separate SSH proxy library (Go module)
└── configs/                   # Configuration files
```

## Issues Identified

### 1. Package Naming and Organization Issues

#### Generic Package Names
- `internal/client/` - Too generic, doesn't follow Go conventions
- `internal/shared/` - Generic name, unclear purpose
- `internal/config/` - Conflicts with client/config and server/config

#### Package Structure Problems
- Mixed concerns in single packages
- Unclear separation between client and server functionality
- Duplicate configuration handling across packages

### 2. Docker Implementation Issues

#### Custom Docker Daemon Implementation
- `internal/client/docker/daemon.go` contains a complete custom Docker daemon implementation
- Duplicates functionality that exists in `ssh-docker-proxy` library
- Complex HTTP proxy logic that reinvents the wheel
- Difficult to maintain and test

#### Key Problems in Current Implementation:
- **Complex HTTP Proxying**: Manual HTTP request/response handling
- **Streaming Logic**: Custom streaming implementation for Docker attach/logs
- **Connection Management**: Manual connection lifecycle management
- **Error Handling**: Complex error handling for various Docker API scenarios

### 3. SSH Proxy Integration Issues

#### Separate Module Structure
- `ssh-docker-proxy/` exists as a separate Go module within the project
- Not properly integrated as a library dependency
- Causes import path confusion and dependency management issues

#### Integration Problems:
- DockBridge doesn't use ssh-docker-proxy as intended
- Custom Docker daemon implementation bypasses the proven ssh-docker-proxy library
- No connection lifecycle hooks for server provisioning

### 4. Configuration Management Issues

#### Scattered Configuration
- Configuration types spread across multiple packages
- `internal/shared/config/types.go` - Shared types
- `internal/client/config/config.go` - Client configuration management
- `internal/server/config/config.go` - Server configuration management
- `internal/config/logger.go` - Global configuration utilities

#### Configuration Problems:
- No unified configuration approach
- Duplicate configuration validation logic
- Missing SSH proxy configuration integration

### 5. Dependency Management Issues

#### Module Structure
- Main module: `github.com/dockbridge/dockbridge`
- Sub-module: `ssh-docker-proxy` (separate go.mod)
- Creates import path confusion
- Complicates dependency management

## Code That Needs to be Moved/Refactored

### 1. Configuration System
**Current Location**: `internal/client/config/`, `internal/shared/config/`, `internal/config/`
**Target Location**: `client/config/`, `shared/config/`
**Issues**: Scattered across multiple packages, duplicate logic

### 2. Hetzner Client
**Current Location**: `internal/client/hetzner/`
**Target Location**: `client/hetzner/`
**Issues**: Generic internal path, should follow Go conventions

### 3. SSH Client
**Current Location**: `internal/client/ssh/`
**Target Location**: `client/ssh/` (or remove if using ssh-docker-proxy)
**Issues**: May be redundant with ssh-docker-proxy integration

### 4. Docker Daemon Implementation
**Current Location**: `internal/client/docker/daemon.go`
**Target Action**: **REMOVE** - Replace with ssh-docker-proxy integration
**Issues**: Complex custom implementation that duplicates ssh-docker-proxy functionality

### 5. CLI Commands
**Current Location**: `internal/client/cli/`, `internal/server/cli/`
**Target Location**: `client/cli/`, `server/cli/`
**Issues**: Generic internal paths

### 6. Lifecycle Management
**Current Location**: `internal/client/keepalive/`, `internal/client/lockdetection/`
**Target Location**: `client/lifecycle/`
**Issues**: Separate packages for related functionality, mostly placeholder code

## Mapping from Old Structure to New Structure

### Target Go-Standard Structure
```
DockBridge/
├── cmd/
│   └── dockbridge/            # Single entry point (client-focused)
├── client/                    # DockBridge client functionality
│   ├── config/               # Configuration management
│   ├── hetzner/              # Hetzner Cloud integration
│   ├── lifecycle/            # Server lifecycle management (keepalive + lockdetection)
│   ├── proxy/                # SSH proxy integration (using ssh-docker-proxy lib)
│   └── cli/                  # CLI commands
├── server/                    # Server-side functionality (if needed)
│   ├── config/               # Server configuration
│   └── cli/                  # Server CLI
├── shared/                    # Shared utilities and types
│   ├── config/               # Shared configuration types
│   └── logging/              # Logging utilities
└── configs/                   # Configuration files
```

### Migration Mapping

| Current Location | Target Location | Action |
|------------------|-----------------|---------|
| `internal/client/config/` | `client/config/` | Move + refactor |
| `internal/client/hetzner/` | `client/hetzner/` | Move |
| `internal/client/ssh/` | Remove or integrate | Replace with ssh-docker-proxy |
| `internal/client/docker/daemon.go` | Remove | Replace with proxy integration |
| `internal/client/cli/` | `client/cli/` | Move |
| `internal/client/keepalive/` + `internal/client/lockdetection/` | `client/lifecycle/` | Merge + implement |
| `internal/shared/config/` | `shared/config/` | Move |
| `internal/config/` | `shared/logging/` | Move + rename |
| `internal/server/` | `server/` | Move |
| `pkg/` | Keep as-is | No change needed |

## Dependencies Analysis

### Current Dependencies (go.mod)
- `github.com/spf13/cobra` - CLI framework ✅
- `github.com/spf13/viper` - Configuration management ✅
- `github.com/hetznercloud/hcloud-go/v2` - Hetzner Cloud API ✅
- `github.com/docker/docker` - Docker Go SDK ❌ (should be removed/reduced)
- `golang.org/x/crypto/ssh` - SSH client ❌ (redundant with ssh-docker-proxy)

### Target Dependencies
- Keep: `cobra`, `viper`, `hcloud-go/v2`, `testify`
- Add: `ssh-docker-proxy` as local dependency
- Remove/Reduce: `docker/docker` (only needed for types, not client)
- Remove: Direct `golang.org/x/crypto/ssh` usage

## Integration Strategy

### SSH Proxy Library Integration
1. **Add ssh-docker-proxy as dependency**: Update go.mod to include ssh-docker-proxy as local dependency
2. **Create ProxyManager wrapper**: Wrapper around ssh-docker-proxy with DockBridge-specific logic
3. **Implement connection lifecycle hooks**: Hook system for server provisioning on connection failures
4. **Replace Docker daemon**: Remove custom daemon implementation

### Configuration Integration
1. **Merge configuration types**: Combine SSH proxy settings with DockBridge configuration
2. **Unified configuration loading**: Single configuration system supporting both old and new formats
3. **Backward compatibility**: Ensure existing config files continue to work

## Benefits of Refactoring

### 1. Reliability
- Use proven ssh-docker-proxy library instead of custom implementation
- Reduce code complexity and maintenance burden
- Better error handling and connection management

### 2. Maintainability
- Follow Go best practices for package organization
- Clear separation of concerns
- Easier to understand and modify

### 3. Testability
- Smaller, focused packages are easier to test
- Better mocking and dependency injection
- Comprehensive test coverage

### 4. Extensibility
- Hook system allows for flexible server provisioning strategies
- Modular architecture supports future enhancements
- Clear interfaces for different components

## Next Steps

1. **Create new directory structure** following Go best practices
2. **Move configuration management** to new structure with unified approach
3. **Integrate ssh-docker-proxy library** with connection lifecycle hooks
4. **Replace Docker daemon implementation** with proxy integration
5. **Move and refactor remaining components** (CLI, Hetzner client, etc.)
6. **Update imports and dependencies** throughout the codebase
7. **Add comprehensive tests** for all refactored components
8. **Update documentation** to reflect new structure