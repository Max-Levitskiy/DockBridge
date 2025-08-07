# CLI Commands - Do's and Don'ts

## Do's:
- `make build` - Simple, reliable build command (used 5+ times)
- `go test ./...` - Run all tests efficiently (used 3+ times)
- `go test ./internal/client/hetzner -v` - Run specific package tests with verbose output (used 2+ times)
- `go run demo-docker-optimization.go` - Demo the Docker image optimization (used 1 time)
- `unset DOCKER_HOST` - Reset Docker host before Docker Compose operations
- Use environment variables for test configuration (SSH_HOST, SSH_USER, SSH_KEY)
- Test SSH connection separately before testing proxy: `ssh -i key user@host 'docker ps'`
- Handle `~` path expansion in file paths using `os.UserHomeDir()` and `filepath.Join()`
- Use unit tests + simple manual instructions instead of complex test infrastructure
- Always add default ports to network addresses (e.g., SSH host without port should default to :22)
- Use `unix://` prefix for Docker Unix sockets: `export DOCKER_HOST=unix:///path/to/socket`
- Use `GetByNameAndArchitecture` instead of deprecated `GetByName` for Hetzner images
- Use `WaitFor` instead of deprecated `WatchProgress` for Hetzner actions

## Don'ts:
- Don't rely on Docker-in-Docker for SSH forwarding tests (Alpine SSH server limitations) - failed 2 times
- Don't use complex Docker Compose setups for simple proxy testing - failed 1 time
- Don't assume SSH Unix socket forwarding works in all environments - failed 1 time
- Don't create overly complex test infrastructure when simple approaches work better - failed 1 time
- Don't over-engineer test setups when unit tests + manual instructions suffice - failed 1 time
- Don't delete existing test files, even if they're placeholder stubs - they show intended test structure - failed 1 time
- Don't recreate accidentally deleted test files with proper placeholder implementations and clear TODOs - failed 1 time
- Don't use deprecated Hetzner Cloud API methods - causes compilation errors - failed 1 time
- Don't declare variables without using them - causes compilation errors - failed 2 times
## Port Forwarding Infrastructure (Tasks 1-2)

### Do:
- `go test -v ./internal/client/portforward` - Run all port forwarding tests (manager, proxy, resolver)
- `go test -v ./internal/client/portforward -run TestLocalProxyServer` - Test local TCP proxy server
- `go test -v ./internal/client/portforward -run TestPortConflictResolver` - Test port conflict resolution
- `go test ./internal/client/config` - Test configuration with port forwarding settings
- `go test ./internal/shared/config` - Test shared configuration types

### Don't:
- Don't try to test actual SSH tunnel integration without proper mock setup - requires complex mocking
#
# Container Monitoring and Port Forwarding

### Do:
- `go test ./internal/client/monitor -v` - Run container monitor tests
- `go test ./internal/client/portforward -v` - Run port forwarding tests  
- `go test ./internal/client/portforward -v -run Integration` - Run integration tests for container monitoring and port forwarding
- Use `monitor.NewContainerMonitor(dockerClient, logger)` to create container monitor with minimal Docker API interface
- Use `portforward.NewPortForwardManager(config, logger)` to create port forward manager
- Register port forward manager as container event handler: `monitor.RegisterContainerEventHandler(pfManager)`
- Set polling interval for testing: `monitor.SetPollingInterval(100 * time.Millisecond)`
- Use `monitor.ContainerInfo` and `monitor.PortMapping` types for container information

## Port Forwarding CLI Commands (Task 5)

### Do:
- `dockbridge ports list` or `dockbridge ports ls` - List all active port forwards with status
- `dockbridge ports add <container_id> <local_port> <remote_port>` - Add manual port forward
- `dockbridge ports remove <container_id> <local_port>` - Remove manual port forward
- `dockbridge ports enable` - Enable automatic port forwarding for new containers
- `dockbridge ports disable` - Disable automatic port forwarding for new containers
- `go test -v ./internal/client/cli -run TestPorts` - Run all port forwarding CLI tests
- Use `--config` flag to specify custom configuration file for port commands
- Handle container ID length safely when creating forward IDs (use prefix if longer than 12 chars)

### Don't:
- Don't use full Docker client interface in tests - use minimal `ContainerAPIClient` interface instead
- Don't hardcode container ID slicing to 12 characters without length check (causes slice bounds errors) - 1 time
- Don't create import cycles between monitor and portforward packages in tests - 1 time
- Don't use `.Once()` mock expectations with polling-based monitoring (causes unexpected call panics) - 1 time

## Docker Client Manager Integration

### Do:
- `go test ./internal/client/docker -v` - Run Docker client manager tests including port forwarding integration
- Use `NewDockerClientManagerWithPortForwarding()` constructor for port forwarding support
- Call `StartPortForwarding(ctx)` after establishing Docker connection to initialize monitoring
- Call `StopPortForwarding()` before closing to clean up resources
- Use `RegisterContainerEventHandler()` to register custom event handlers
- Use `InterceptDockerResponse()` to modify Docker API responses for port conflicts
- Check `portForwardConfig.Enabled` before initializing port forwarding components
- Handle both increment and fail strategies in port conflict resolution

### Don't:
- Don't call `StartPortForwarding()` without a valid Docker connection (causes connection errors) - 1 time
- Don't forget to call `StopPortForwarding()` in the Close() method (causes resource leaks) - 1 time
- Don't assume port forwarding is always enabled - check configuration first
- Don't mock incomplete interfaces in tests - implement all required methods for HetznerClient interface
## I
nterface Compatibility Fixes (2025-01-08)

### Do:
- Fix SSH import paths from `github.com/dockbridge/dockbridge/client/ssh` to `github.com/dockbridge/dockbridge/internal/client/ssh`
- Ensure PortForwardManager interface properly implements ContainerEventHandler with correct method signatures
- Use unit tests to verify interface compatibility: `go test ./internal/client/portforward/ -v -run TestPortForwardManagerImplementsContainerEventHandler`
- Test compilation after interface fixes: `go build ./internal/client/docker/`

### Don't:
- Don't assume interface compatibility without testing - create explicit tests to verify
- Don't mix import paths between `client/` and `internal/client/` packages