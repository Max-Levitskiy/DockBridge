# DockBridge

DockBridge automatically provisions Hetzner Cloud servers for Docker development with intelligent lifecycle management to minimize costs. It uses the ssh-docker-proxy library to transparently forward Docker commands to remote servers.

## Features

- 🚀 **Automatic Server Provisioning**: Servers are created on-demand when Docker commands are executed
- 🔌 **Transparent Docker Forwarding**: Uses ssh-docker-proxy for seamless Docker command forwarding
- 💰 **Cost Optimization**: Intelligent server lifecycle management to minimize cloud costs
- 🔒 **Security**: All traffic encrypted via SSH tunnels
- 📊 **Status Monitoring**: Real-time status and cost tracking
- ⚡ **Fast Setup**: Simple configuration and automatic SSH key management

## Quick Start

### 1. Install Dependencies

Ensure you have Go 1.23+ installed.

### 2. Configuration

Create a `dockbridge.yaml` configuration file:

```yaml
# Hetzner Cloud settings
hetzner:
  api_token: "your-hetzner-api-token-here"
  server_type: "cpx21"
  location: "fsn1"
  volume_size: 10

# Docker settings
docker:
  socket_path: "/tmp/dockbridge.sock"

# SSH settings
ssh:
  user: "ubuntu"
  key_path: "~/.ssh/id_rsa"
  timeout: "10s"
```

### 3. Start DockBridge

```bash
# Build the binary
go build -o dockbridge cmd/dockbridge/main.go

# Start DockBridge
./dockbridge start
```

### 4. Use Docker

In another terminal:

```bash
export DOCKER_HOST=unix:///tmp/dockbridge.sock
docker run hello-world
```

## Commands

### Start DockBridge
```bash
dockbridge start [flags]
```

Flags:
- `-s, --socket`: Local Docker socket path (overrides config)
- `-d, --daemon`: Run in daemon mode
- `-c, --config`: Configuration file path

### Check Status
```bash
dockbridge status [flags]
```

Flags:
- `-j, --json`: Output in JSON format
- `-w, --watch`: Watch for status changes

### Stop DockBridge
```bash
dockbridge stop [flags]
```

Flags:
- `-f, --force`: Force stop without graceful shutdown

## Architecture

```
Local Machine                    Hetzner Cloud
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
│ DockBridge      │
│                 │
│ • Server Mgmt   │
│ • SSH Proxy     │
│ • Lifecycle     │
└─────────────────┘
```

## Configuration Options

| Setting | Description | Default |
|---------|-------------|---------|
| `hetzner.api_token` | Hetzner Cloud API token | Required |
| `hetzner.server_type` | Server type (cpx11, cpx21, etc.) | `cpx21` |
| `hetzner.location` | Server location | `fsn1` |
| `hetzner.volume_size` | Persistent volume size (GB) | `10` |
| `docker.socket_path` | Local Docker socket path | `/tmp/dockbridge.sock` |
| `ssh.user` | SSH username | `ubuntu` |
| `ssh.key_path` | SSH private key path | `~/.ssh/id_rsa` |
| `ssh.timeout` | SSH connection timeout | `10s` |

## Development Status

### ✅ Completed
- [x] Simplified project structure
- [x] Server Manager (Hetzner integration)
- [x] Proxy Manager (ssh-docker-proxy integration)
- [x] Connection lifecycle hooks
- [x] Basic CLI interface (start/stop/status)
- [x] Configuration management

### 🚧 In Progress
- [ ] Lock detection (cross-platform)
- [ ] Keep-alive system
- [ ] Lifecycle manager
- [ ] Comprehensive logging
- [ ] Error handling and recovery
- [ ] Cost management features

### 📋 Planned
- [ ] Performance optimizations
- [ ] Docker workflow compatibility testing
- [ ] Comprehensive testing suite
- [ ] Documentation and examples

## Contributing

This project follows a spec-driven development approach. See `.kiro/specs/dockbridge/` for detailed requirements, design, and implementation tasks.

## License

[Add your license here]