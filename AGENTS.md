# DockBridge - AI Agent Context

## Project Overview
**DockBridge** is a CLI tool that enables users to run Docker on cheap Hetzner Cloud servers seamlessly. It acts as a transparent proxy, automatically provisioning cloud servers when Docker commands are run and destroying them when idle, while preserving state (images/volumes) via persistent block storage.

**Problem Solved**:
- Eliminates Docker Desktop resource overhead on Mac/Windows.
- Provides fast x86 builds on ARM Macs.
- Reduces cloud costs by only paying for compute when needed (servers destroy on idle).

## Tech Stack
- **Language**: Go (v1.24+)
- **Cloud Provider**: Hetzner Cloud (via `hetznercloud/hcloud-go`)
- **Docker Interaction**: Docker SDK (`docker/docker`)
- **CLI Framework**: Cobra (`spf13/cobra`)
- **Configuration**: Viper (`spf13/viper`)
- **SSH**: `golang.org/x/crypto/ssh` for secure tunneling
- **Task Runner**: Task (Taskfile)

## Architecture
1.  **Local Client**:
    - Listens on a local Unix socket (default: `/tmp/dockbridge.sock`).
    - Intercepts Docker API requests.
    - Manages server lifecycle (Create, Start, Stop, Destroy).
    - Establishes SSH tunnels to the remote server.
2.  **Remote Server**:
    - Hetzner Cloud VPS (default: cpx21).
    - Runs standard Docker Daemon.
    - Mounts a persistent block volume at `/var/lib/docker` to persist state across server destructions.
    - Cloud-init is used for initial server setup.
3.  **Communication**:
    - All traffic is tunnelled securely over SSH.

## Development Workflow & Standards
**Task Runner**: We use `task` (Taskfile.yaml) for all common operations.

### Common Commands
- **Build**: `task build` (output: `bin/dockbridge`)
- **Test**: `task test` (runs `go test ./...`)
- **Lint**: `task lint` (runs `golangci-lint`)
- **Security Check**: `task security` (runs `gosec` and `govulncheck`)
- **All Checks**: `task check` (runs fmt, lint, security, and test) - *Run this before committing!*
- **Format**: `task fmt`

### Coding Style & Conventions
- **Formatting**: Standard `go fmt`.
- **Linting**: Strict `golangci-lint` rules defined in `.golangci.yml`.
    - Enabled linters: `govet`, `staticcheck`, `unused`, `ineffassign`, `modernize`.
- **Security**:
    - `gosec` for static analysis.
    - `govulncheck` for dependency vulnerability scanning.
- **Error Handling**: Use `github.com/pkg/errors` for wrapping errors when context is needed, or standard modern Go error wrapping.
- **Testing**: Use `github.com/stretchr/testify` for assertions.

### Key Directories
- `cmd/dockbridge`: Main entry point.
- `client`: core logic for the local CLI.
- `server`: logic running on the remote server (if any custom agents are deployed).
- `ssh-docker-proxy`: Separate module for handling the SSH/Docker socket proxying logic.
- `pkg`: Reusable Go packages.

## Migration Note
The project recently moved from an `internal/` based structure to a root-level `client/`, `server/`, `pkg/` structure. Code should be placed in these root directories, not `internal`.
