# SSH Docker Proxy

A lightweight, transparent proxy that forwards Docker API requests from local clients to remote Docker daemons over SSH connections. This enables seamless use of local Docker CLI tools against remote Docker instances while maintaining full compatibility with all Docker features.

## Features

- **Transparent Proxying**: Pure byte-level forwarding without Docker-specific parsing
- **SSH Security**: All traffic encrypted via SSH tunnels
- **Full Docker Compatibility**: Supports all Docker commands including streaming operations
- **Concurrent Connections**: Handle multiple Docker clients simultaneously
- **Library + CLI**: Use as standalone tool or integrate into other applications
- **Health Checking**: Verify remote Docker daemon accessibility before starting

## Quick Start

### CLI Usage

```bash
# Using command-line flags
ssh-docker-proxy \
  -ssh-user=ubuntu \
  -ssh-host=192.168.1.100 \
  -ssh-key=~/.ssh/id_rsa \
  -local-socket=/tmp/docker.sock

# Using configuration file
ssh-docker-proxy -config=config.yaml
```

Then in another terminal:
```bash
export DOCKER_HOST=unix:///tmp/docker.sock
docker ps
```

### Configuration File

Create `ssh-docker-proxy.yaml`:

```yaml
local_socket: "/tmp/docker.sock"
ssh_user: "ubuntu"
ssh_host: "your-server.example.com:22"
ssh_key_path: "~/.ssh/id_rsa"
remote_socket: "/var/run/docker.sock"
timeout: "10s"
```

### Library Usage

```go
package main

import (
    "context"
    ssh_docker_proxy "ssh-docker-proxy"
)

func main() {
    config := &ssh_docker_proxy.ProxyConfig{
        LocalSocket:  "/tmp/docker.sock",
        SSHUser:      "ubuntu",
        SSHHost:      "your-server.example.com",
        SSHKeyPath:   "~/.ssh/id_rsa",
        RemoteSocket: "/var/run/docker.sock",
        Timeout:      "10s",
    }

    proxy, err := ssh_docker_proxy.NewProxy(config, nil)
    if err != nil {
        panic(err)
    }

    ctx := context.Background()
    if err := proxy.Start(ctx); err != nil {
        panic(err)
    }
}
```

## Installation

```bash
go install ssh-docker-proxy/cmd/ssh-docker-proxy@latest
```

Or build from source:

```bash
git clone <repository>
cd ssh-docker-proxy
go build -o ssh-docker-proxy cmd/ssh-docker-proxy/main.go
```

## Configuration Options

| Flag | Config File | Description | Default |
|------|-------------|-------------|---------|
| `-local-socket` | `local_socket` | Local Unix socket path | Required |
| `-ssh-user` | `ssh_user` | SSH username | Required |
| `-ssh-host` | `ssh_host` | SSH hostname with optional port | Required |
| `-ssh-key` | `ssh_key_path` | Path to SSH private key file | Required |
| `-remote-socket` | `remote_socket` | Remote Docker socket path | `/var/run/docker.sock` |
| `-timeout` | `timeout` | SSH connection timeout | `10s` |
| `-config` | N/A | Path to configuration file | Auto-detected |

## Supported Docker Operations

- ✅ Container operations (`run`, `exec`, `logs`, `attach`)
- ✅ Image operations (`pull`, `push`, `build`)
- ✅ Network operations
- ✅ Volume operations
- ✅ System operations (`info`, `version`)
- ✅ Interactive sessions (`docker exec -it`)
- ✅ Streaming operations (`docker logs -f`)
- ✅ Large file transfers (`docker build` with large contexts)

## Architecture

```
Local Machine                    Remote Host
┌─────────────────┐             ┌──────────────────┐
│ Docker Client   │             │ Docker Daemon    │
│                 │             │                  │
│ docker ps   ────┼─────────────┼──► /var/run/     │
│                 │   SSH       │    docker.sock   │
│                 │   Tunnel    │                  │
└─────────────────┘             └──────────────────┘
        │
        ▼
┌─────────────────┐
│ SSH Docker      │
│ Proxy           │
│                 │
│ /tmp/docker.sock│
└─────────────────┘
```

## Error Handling

The proxy categorizes errors for better troubleshooting:

- **CONFIG**: Configuration validation errors
- **SSH**: SSH connection and authentication errors  
- **DOCKER**: Docker daemon connectivity errors
- **RUNTIME**: Proxy runtime errors

## Testing

Run unit tests:
```bash
go test ./...
```

Manual testing with a real SSH server:
```bash
# Terminal 1: Start proxy
ssh-docker-proxy -ssh-user=ubuntu -ssh-host=your-server -ssh-key=~/.ssh/id_rsa -local-socket=/tmp/test.sock

# Terminal 2: Test Docker commands
export DOCKER_HOST=unix:///tmp/test.sock
docker version
docker run hello-world
```

## Troubleshooting

### SSH Connection Issues
- Verify SSH key permissions: `chmod 600 ~/.ssh/id_rsa`
- Test SSH connection: `ssh -i ~/.ssh/id_rsa user@host`
- Check SSH server configuration allows key authentication

### Docker Daemon Issues
- Verify Docker daemon is running on remote host
- Check Docker socket permissions on remote host
- Test Docker access: `ssh user@host 'docker ps'`

### Permission Issues
- Ensure local socket directory is writable
- Check Docker group membership on remote host

## License

[Add your license here]