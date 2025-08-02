# Design Document

## Overview

This design outlines the migration of DockBridge to use the ssh-docker-proxy library and refactoring the project structure to follow Go best practices. The migration will be done incrementally with a test-first approach to ensure reliability and maintainability.

## Architecture

### Current Structure Issues
- Generic package names (`internal/client`, `internal/shared`)
- Mixed concerns in single packages
- Custom Docker daemon implementation that duplicates ssh-docker-proxy functionality
- Non-standard Go project layout

### Target Structure (Go Best Practices)

```
DockBridge/
├── cmd/
│   └── dockbridge/           # Main application entry point
├── pkg/                      # Public API packages (if needed for external use)
├── client/                   # DockBridge client functionality
│   ├── config/              # Configuration management
│   ├── hetzner/             # Hetzner Cloud integration
│   ├── lifecycle/           # Server lifecycle management
│   └── proxy/               # SSH proxy integration (using ssh-docker-proxy lib)
├── server/                   # Server-side functionality (if applicable)
├── shared/                   # Shared utilities and types
│   ├── config/              # Shared configuration types
│   └── logging/             # Logging utilities
├── ssh-docker-proxy/         # SSH proxy library (existing)
└── configs/                  # Configuration files
```

## Components and Interfaces

### 1. Connection Lifecycle Hook System

```go
// ConnectionHook represents a hook that can be triggered during connection lifecycle
type ConnectionHook interface {
    OnConnectionFailure(ctx context.Context, err error) error
    OnConnectionSuccess(ctx context.Context) error
    OnConnectionLost(ctx context.Context) error
}

// ProxyManager manages SSH proxy lifecycle with hook support
type ProxyManager struct {
    config   *config.Config
    proxy    *sshproxy.Proxy
    hooks    []ConnectionHook
    logger   *log.Logger
}

// RegisterHook registers a connection lifecycle hook
func (pm *ProxyManager) RegisterHook(hook ConnectionHook)

// StartLazy starts the proxy with lazy connection (doesn't fail if server doesn't exist)
func (pm *ProxyManager) StartLazy(ctx context.Context) (string, error)

// ServerProvisioningHook handles server provisioning on connection failures
type ServerProvisioningHook struct {
    hetzner *hetzner.Client
    config  *config.Config
    logger  *log.Logger
    mu      sync.Mutex
    provisioning bool
}

// OnConnectionFailure provisions server if it doesn't exist
func (sph *ServerProvisioningHook) OnConnectionFailure(ctx context.Context, err error) error
```

### 2. Refactored Client Structure with Hook-Based Architecture

```go
// Client represents the main DockBridge client
type Client struct {
    config           *config.Config
    proxyManager     *ProxyManager
    provisioningHook *ServerProvisioningHook
    lifecycle        *lifecycle.Manager
    logger           *log.Logger
}

// Start sets up the client with lazy proxy connection
func (c *Client) Start(ctx context.Context) error {
    // Register server provisioning hook
    c.proxyManager.RegisterHook(c.provisioningHook)
    
    // Start proxy with lazy connection (won't fail if server doesn't exist)
    return c.proxyManager.StartLazy(ctx)
}

// ConnectionHandler handles concurrent Docker client connections
type ConnectionHandler struct {
    proxySocket string
    mu          sync.RWMutex
}

// HandleConnection processes Docker client connections in goroutines
func (ch *ConnectionHandler) HandleConnection(conn net.Conn)
```

### 3. Configuration Migration

```go
// Config represents DockBridge configuration
type Config struct {
    // Existing DockBridge config fields
    Hetzner HetznerConfig `yaml:"hetzner"`
    
    // SSH proxy configuration (integrated from ssh-docker-proxy)
    SSH SSHConfig `yaml:"ssh"`
    
    // Local proxy settings
    Proxy ProxyConfig `yaml:"proxy"`
}

// SSHConfig contains SSH connection settings
type SSHConfig struct {
    User     string `yaml:"user"`
    KeyPath  string `yaml:"key_path"`
    Timeout  time.Duration `yaml:"timeout"`
}

// ProxyConfig contains local proxy settings
type ProxyConfig struct {
    Socket string `yaml:"socket"`
}
```

## Data Models

### Migration Strategy

1. **Phase 1: Structure Refactoring**
   - Rename packages to follow Go conventions
   - Move code to appropriate packages
   - Update imports and references

2. **Phase 2: SSH Proxy Integration**
   - Add ssh-docker-proxy as dependency
   - Create ProxyManager wrapper
   - Replace Docker daemon implementation

3. **Phase 3: Configuration Migration**
   - Update configuration structure
   - Ensure backward compatibility
   - Add configuration validation

4. **Phase 4: Testing and Cleanup**
   - Add comprehensive tests
   - Remove unused code
   - Update documentation

## Error Handling

### Error Categories

1. **Migration Errors**: Issues during the refactoring process
2. **Integration Errors**: Problems integrating ssh-docker-proxy
3. **Configuration Errors**: Invalid or incompatible configuration
4. **Proxy Errors**: SSH proxy connection or operation failures

### Error Handling Strategy

```go
// MigrationError represents errors during migration
type MigrationError struct {
    Phase   string
    Message string
    Cause   error
}

func (e *MigrationError) Error() string {
    return fmt.Sprintf("[%s] %s: %v", e.Phase, e.Message, e.Cause)
}
```

## Testing Strategy

### Test-First Approach

1. **Unit Tests**: Test each component in isolation
2. **Integration Tests**: Test ssh-docker-proxy integration
3. **End-to-End Tests**: Test complete DockBridge workflow
4. **Regression Tests**: Ensure existing functionality works

### Testing Phases

```go
// Phase 1: Structure Tests
func TestPackageStructure(t *testing.T)
func TestImportPaths(t *testing.T)

// Phase 2: Integration Tests
func TestProxyManagerIntegration(t *testing.T)
func TestSSHProxyLibraryUsage(t *testing.T)

// Phase 3: Configuration Tests
func TestConfigurationMigration(t *testing.T)
func TestBackwardCompatibility(t *testing.T)

// Phase 4: End-to-End Tests
func TestDockBridgeWorkflow(t *testing.T)
func TestExistingFunctionality(t *testing.T)
```

## Implementation Plan

### Incremental Migration Steps

1. **Create new package structure** (with tests)
2. **Move configuration management** (with tests)
3. **Move Hetzner client** (with tests)
4. **Integrate ssh-docker-proxy library** (with tests)
5. **Replace Docker daemon implementation** (with tests)
6. **Update CLI commands** (with tests)
7. **Clean up old code** (with tests)
8. **Update documentation** (with tests)

### Commit Strategy

- Each step should be a separate commit
- Commits should be focused and atomic
- Tests should be included with each change
- Commit messages should be descriptive

### Validation Approach

- Run tests after each change
- Validate functionality at each step
- Use feature flags if needed for gradual rollout
- Maintain backward compatibility during transition

## Benefits

1. **Reliability**: Use proven ssh-docker-proxy library
2. **Maintainability**: Follow Go best practices
3. **Testability**: Comprehensive test coverage
4. **Clarity**: Clear package structure and responsibilities
5. **Reusability**: Better separation of concerns