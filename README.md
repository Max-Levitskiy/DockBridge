<p align="center">
  <h1 align="center">🌉 DockBridge</h1>
  <p align="center"><strong>Run Docker on cheap cloud servers, pay only when you use it</strong></p>
</p>

<p align="center">
  <a href="#why-dockbridge">Why?</a> •
  <a href="#quick-start">Quick Start</a> •
  <a href="#how-it-works">How it Works</a> •
  <a href="#features">Features</a> •
  <a href="#configuration">Configuration</a>
</p>

---

## The Problem

**Docker Desktop on Mac/Windows is slow and resource-hungry.** Running containers locally means:
- 🐌 Slow builds on ARM Macs when targeting x86
- 🔋 Battery drain from VM overhead  
- 💾 Precious SSD space consumed by images
- 🌀 Fans spinning when building large containers

**Cloud dev environments are expensive.** Keeping a cloud VM running 24/7:
- 💸 ~$30-50/month for a decent dev server
- 🔧 Manual setup and maintenance
- 📦 Losing your containers when you forget to save

## The Solution

**DockBridge automatically provisions cloud servers that exist only when you need them.**

```bash
# Point Docker at DockBridge
export DOCKER_HOST=unix:///tmp/dockbridge.sock

# Just use Docker normally - server spins up automatically
docker run hello-world   # Server created in ~60s
docker build .           # Fast x86 builds on x86 hardware

# Walk away - server auto-destroys when idle
# Your images and volumes persist on cheap cloud storage
```

**Typical cost: $0.01-0.05/hour** (only when actually building/running containers)

## Why DockBridge?

| Feature | Local Docker | Cloud VM | DockBridge |
|---------|-------------|----------|------------|
| **Cost** | Free (but slow) | $30-50/month | Pay-per-use |
| **Speed** | Slow on non-native arch | Fast | Fast |
| **State persistence** | ✅ | Manual | ✅ Automatic |
| **Resource usage** | High | None local | None local |
| **Setup** | Easy | Manual | Easy |

## Quick Start

### 1. Install

```bash
# One-liner install (Linux/macOS)
curl -fsSL https://raw.githubusercontent.com/dockbridge/dockbridge/main/install.sh | sh
```

<details>
<summary>Other installation methods</summary>

```bash
# Build from source (requires Go 1.24+)
git clone https://github.com/dockbridge/dockbridge
cd dockbridge
go build -o dockbridge ./cmd/dockbridge
sudo mv dockbridge /usr/local/bin/
```

</details>

### 2. Configure

Create `dockbridge.yaml`:

```yaml
hetzner:
  api_token: "your-hetzner-api-token"  # Get one at console.hetzner.cloud
  server_type: "cpx21"                  # 3 vCPU, 4GB RAM (~$0.01/hr)
  location: "fsn1"                      # Falkenstein, DE (or nbg1, hel1)
  volume_size: 10                       # Persistent storage in GB

docker:
  socket_path: "/tmp/dockbridge.sock"

ssh:
  key_path: "~/.ssh/id_rsa"            # Your SSH key
```

### 3. Use

```bash
# Start DockBridge
./dockbridge start

# In another terminal, point Docker at it
export DOCKER_HOST=unix:///tmp/dockbridge.sock

# Use Docker as usual
docker ps
docker run -it ubuntu bash
docker build -t myapp .
```

## How it Works

```
Your Machine                              Hetzner Cloud
┌──────────────────┐                     ┌──────────────────┐
│                  │                     │                  │
│  docker build .  │                     │   Docker Daemon  │
│        │         │                     │        ▲         │
│        ▼         │                     │        │         │
│  ┌────────────┐  │      SSH Tunnel     │  ┌────────────┐  │
│  │ DockBridge │──┼─────────────────────┼─▶│   Docker   │  │
│  │            │  │    Encrypted        │  │   Socket   │  │
│  └────────────┘  │                     │  └────────────┘  │
│        ▲         │                     │        ▲         │
│        │         │                     │        │         │
│  Unix Socket     │                     │  ┌────────────┐  │
│  /tmp/dockbridge │                     │  │ Persistent │  │
│                  │                     │  │   Volume   │  │
└──────────────────┘                     └──┴────────────┴──┘

1. You run `docker build`
2. DockBridge sees activity → provisions server (if needed)
3. SSH tunnel forwards commands to remote Docker
4. Server auto-destroys after idle timeout
5. Volume persists your images & data
```

## Features

### 🚀 Automatic Server Lifecycle
- **On-demand provisioning**: Servers created when you run Docker commands
- **Auto-shutdown**: Servers destroyed after configurable idle time
- **Instant resume**: Volume persists, so images are still there next time

### 💾 Persistent Docker State
- All images, containers, and volumes survive server destruction
- Uses Hetzner's cheap block storage (~€0.04/GB/month)
- No more "pulling all images again" on restart

### 🔒 Secure by Default
- All traffic encrypted via SSH tunnel
- No exposed ports on cloud server
- Uses your existing SSH keys

### 💰 Cost Optimization
- Pay only for compute time you use
- Automatic idle detection
- Real-time cost tracking with `dockbridge status`

## Commands

```bash
# Start the proxy (required before using Docker)
dockbridge start [--daemon] [--socket /path/to/socket]

# Check current status, server info, and costs
dockbridge status [--json] [--watch]

# Stop and destroy the server
dockbridge stop [--force]
```

## Configuration Reference

| Setting | Description | Default |
|---------|-------------|---------|
| `hetzner.api_token` | Hetzner Cloud API token | *Required* |
| `hetzner.server_type` | Server type (cpx11, cpx21, cpx31, etc.) | `cpx21` |
| `hetzner.location` | Datacenter (fsn1, nbg1, hel1, ash, hil) | `fsn1` |
| `hetzner.volume_size` | Persistent volume size in GB | `10` |
| `docker.socket_path` | Local Unix socket path | `/tmp/dockbridge.sock` |
| `ssh.key_path` | Path to SSH private key | `~/.ssh/id_rsa` |
| `ssh.timeout` | SSH connection timeout | `10s` |

### Hetzner Server Types

| Type | vCPU | RAM | Price/hr |
|------|------|-----|----------|
| cpx11 | 2 | 2GB | ~$0.006 |
| **cpx21** | 3 | 4GB | ~$0.010 |
| cpx31 | 4 | 8GB | ~$0.017 |
| cpx41 | 8 | 16GB | ~$0.033 |
| cpx51 | 16 | 32GB | ~$0.066 |

## FAQ

### Why Hetzner?

Hetzner offers the best price-to-performance ratio for cloud servers in Europe/US:
- **~3x cheaper** than AWS/GCP/Azure for equivalent specs
- Excellent x86 performance for Docker builds
- Simple, developer-friendly API
- Block storage for persistent volumes

### How does state persist when servers are destroyed?

DockBridge creates a persistent block volume that mounts at `/var/lib/docker`. When a server is destroyed, the volume remains. When a new server is provisioned, it reattaches the same volume, restoring all your images, containers, and volumes.

### What about my running containers?

When the server is destroyed, running containers stop. On next use, the server is reprovisioned and you can restart your containers. Use `docker-compose up -d` or similar for easy restart.

### Is it safe for production?

DockBridge is designed for **development use**. The auto-destroy feature means you shouldn't run production workloads. For production, use proper orchestration (Kubernetes, Docker Swarm, etc.).

### Can I use a different cloud provider?

Currently only Hetzner is supported. AWS/GCP/Azure support is planned. PRs welcome!

## Development

```bash
# Clone
git clone https://github.com/dockbridge/dockbridge
cd dockbridge

# Run tests
go test ./...

# Build
go build -o dockbridge ./cmd/dockbridge

# Run with debug logging
DOCKBRIDGE_DEBUG=1 ./dockbridge start
```

## Contributing

Contributions are welcome! Please see our [contributing guidelines](CONTRIBUTING.md).

Key areas for contribution:
- Support for additional cloud providers (AWS, GCP, Azure)
- Windows support
- Improved idle detection
- Cost prediction and alerts

## License

AGPL-3.0 - See [LICENSE](LICENSE.txt)

---

<p align="center">
  <sub>Built with ☕ for developers who want fast Docker builds without the VM overhead</sub>
</p>