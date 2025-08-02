# Structure Migration Mapping

## Detailed File-by-File Migration Plan

### 1. Configuration System Migration

#### Current Files:
- `internal/shared/config/types.go` → `shared/config/types.go`
- `internal/client/config/config.go` → `client/config/config.go`
- `internal/client/config/config_test.go` → `client/config/config_test.go`
- `internal/client/config/init.go` → `client/config/init.go`
- `internal/client/config/init_test.go` → `client/config/init_test.go`
- `internal/client/config/load_without_validation.go` → `client/config/load_without_validation.go`
- `internal/server/config/config.go` → `server/config/config.go`
- `internal/server/config/config_test.go` → `server/config/config_test.go`

#### Changes Required:
- Update import paths from `internal/shared/config` to `shared/config`
- Update import paths from `internal/client/config` to `client/config`
- Add SSH proxy configuration integration
- Merge configuration validation logic

### 2. Hetzner Client Migration

#### Current Files:
- `internal/client/hetzner/client.go` → `client/hetzner/client.go`
- `internal/client/hetzner/client_test.go` → `client/hetzner/client_test.go`
- `internal/client/hetzner/cloudinit.go` → `client/hetzner/cloudinit.go`
- `internal/client/hetzner/lifecycle.go` → `client/hetzner/lifecycle.go`
- `internal/client/hetzner/lifecycle_test.go` → `client/hetzner/lifecycle_test.go`
- `internal/client/hetzner/utils.go` → `client/hetzner/utils.go`
- `internal/client/hetzner/example_usage.go` → `client/hetzner/example_usage.go`
- `internal/client/hetzner/README.md` → `client/hetzner/README.md`

#### Changes Required:
- Update import paths throughout the codebase
- Integrate with new proxy manager for on-demand provisioning
- Update configuration references

### 3. SSH Client Migration/Removal

#### Current Files:
- `internal/client/ssh/client.go` → **EVALUATE FOR REMOVAL**
- `internal/client/ssh/client_test.go` → **EVALUATE FOR REMOVAL**
- `internal/client/ssh/keys.go` → `client/ssh/keys.go` (keep key management)
- `internal/client/ssh/keys_test.go` → `client/ssh/keys_test.go`
- `internal/client/ssh/tunnel.go` → **REMOVE** (replaced by ssh-docker-proxy)
- `internal/client/ssh/tunnel_test.go` → **REMOVE**

#### Changes Required:
- Remove tunnel-related code (replaced by ssh-docker-proxy)
- Keep SSH key management utilities
- Update import paths for remaining code

### 4. Docker Daemon Removal

#### Current Files:
- `internal/client/docker/daemon.go` → **REMOVE COMPLETELY**
- `internal/client/docker/client_manager.go` → **REMOVE COMPLETELY**
- `internal/client/docker/client_wrapper.go` → **REMOVE COMPLETELY**
- `internal/client/docker/utils.go` → **EVALUATE** (keep utility functions if needed)
- `internal/client/docker/.gitkeep` → **REMOVE**

#### Replacement:
- Create `client/proxy/manager.go` - ProxyManager using ssh-docker-proxy
- Create `client/proxy/hooks.go` - Connection lifecycle hooks
- Create `client/proxy/server_manager.go` - Integration with Hetzner provisioning

### 5. CLI Commands Migration

#### Current Files:
- `internal/client/cli/root.go` → `client/cli/root.go`
- `internal/client/cli/root_test.go` → `client/cli/root_test.go`
- `internal/client/cli/start.go` → `client/cli/start.go`
- `internal/client/cli/server.go` → `client/cli/server.go`
- `internal/client/cli/server_test.go` → `client/cli/server_test.go`
- `internal/client/cli/config.go` → `client/cli/config.go`
- `internal/client/cli/init.go` → `client/cli/init.go`
- `internal/client/cli/init_test.go` → `client/cli/init_test.go`
- `internal/client/cli/logs.go` → `client/cli/logs.go`
- `internal/server/cli/root.go` → `server/cli/root.go`

#### Changes Required:
- Update all import paths
- Update start command to use ProxyManager instead of Docker daemon
- Update configuration loading paths

### 6. Lifecycle Management Migration

#### Current Files:
- `internal/client/keepalive/.gitkeep` → **REMOVE**
- `internal/client/lockdetection/.gitkeep` → **REMOVE**

#### New Files to Create:
- `client/lifecycle/manager.go` - Unified lifecycle management
- `client/lifecycle/keepalive.go` - Keep-alive mechanism
- `client/lifecycle/lockdetection.go` - Lock detection
- `client/lifecycle/manager_test.go` - Tests

### 7. Shared Utilities Migration

#### Current Files:
- `internal/config/logger.go` → `shared/logging/logger.go`
- `internal/config/logger_test.go` → `shared/logging/logger_test.go`

#### Changes Required:
- Rename package from `config` to `logging`
- Update import paths throughout codebase

### 8. Server Components Migration

#### Current Files:
- `internal/server/docker/.gitkeep` → **REMOVE** (placeholder)
- Keep other server files as-is, just move to new location

## Import Path Updates Required

### Files That Import Configuration:
- `internal/client/cli/start.go` - Update config imports
- `internal/client/hetzner/client.go` - Update config imports
- `cmd/client/main.go` - Update CLI import
- `cmd/server/main.go` - Update CLI import

### Files That Import Hetzner Client:
- `internal/client/cli/start.go` - Update hetzner import
- `internal/client/docker/daemon.go` - **REMOVE** (file being deleted)

### Files That Import SSH Client:
- `internal/client/docker/daemon.go` - **REMOVE** (file being deleted)
- Any other files using SSH tunneling - Replace with proxy integration

## New Files to Create

### 1. Proxy Integration
```
client/proxy/
├── manager.go          # ProxyManager wrapper around ssh-docker-proxy
├── manager_test.go     # Tests for ProxyManager
├── hooks.go            # Connection lifecycle hooks interface
├── hooks_test.go       # Tests for hooks
├── server_manager.go   # Integration with Hetzner provisioning
└── server_manager_test.go # Tests for server manager
```

### 2. Lifecycle Management
```
client/lifecycle/
├── manager.go          # Unified lifecycle management
├── manager_test.go     # Tests for lifecycle manager
├── keepalive.go        # Keep-alive mechanism implementation
├── keepalive_test.go   # Tests for keep-alive
├── lockdetection.go    # Lock detection implementation
└── lockdetection_test.go # Tests for lock detection
```

### 3. Updated Main Entry Point
```
cmd/dockbridge/
└── main.go             # Single entry point (client-focused)
```

## go.mod Updates Required

### Current Dependencies to Remove/Reduce:
- Reduce `github.com/docker/docker` usage (keep only for types)
- Remove direct `golang.org/x/crypto/ssh` usage

### New Dependencies to Add:
- Add `./ssh-docker-proxy` as local dependency
- Ensure all existing dependencies are properly used

### Module Structure:
- Keep main module as `github.com/dockbridge/dockbridge`
- Integrate `ssh-docker-proxy` as local dependency, not separate module

## Testing Strategy

### Test Files to Move:
- All `*_test.go` files should move with their corresponding source files
- Update import paths in all test files
- Ensure test coverage is maintained

### New Tests to Create:
- `client/proxy/manager_test.go` - Test ProxyManager integration
- `client/proxy/hooks_test.go` - Test connection lifecycle hooks
- `client/lifecycle/manager_test.go` - Test unified lifecycle management
- Integration tests for ssh-docker-proxy integration

## Configuration File Updates

### Current Config Files:
- `configs/client.yaml` - May need SSH proxy settings
- `configs/server.yaml` - Keep as-is
- `configs/logger.yaml` - Keep as-is

### New Configuration Sections:
Add SSH proxy configuration to client.yaml:
```yaml
proxy:
  socket: "/tmp/dockbridge.sock"
ssh:
  user: "root"
  key_path: "~/.dockbridge/ssh/id_rsa"
  timeout: "30s"
```

## Validation Steps

### After Each Migration Step:
1. **Build Test**: `make build` should succeed
2. **Import Test**: All import paths should resolve correctly
3. **Unit Tests**: `go test ./...` should pass
4. **Integration Test**: Basic functionality should work

### Final Validation:
1. **Complete Build**: Both client and server should build successfully
2. **Configuration Loading**: All configuration files should load correctly
3. **SSH Proxy Integration**: ssh-docker-proxy should be properly integrated
4. **Backward Compatibility**: Existing workflows should continue to work