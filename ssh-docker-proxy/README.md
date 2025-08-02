# SSH Docker Proxy

A Go-based proxy that forwards Docker API requests to remote Docker daemons over SSH connections.

## Features

- **Pure HTTP Proxy**: Forwards raw bytes without Docker-specific parsing
- **SSH Transport**: Secure, authenticated connections to remote hosts  
- **Per-Connection Isolation**: Fresh SSH streams for each Docker client
- **Streaming Support**: Full compatibility with `docker logs -f`, `docker exec -it`, etc.
- **Concurrent Connections**: Multiple Docker operations simultaneously
- **Graceful Shutdown**: Proper cleanup and resource management

## Quick Start

### 1. Build the Proxy

```bash
make build
```

### 2. Test with Your Remote Server

Set up environment variables for your remote server:

```bash
export SSH_HOST=your-server.com:22
export SSH_USER=your-username  
export SSH_KEY=~/.ssh/id_rsa
./test-local.sh
```

### 3. Start the Proxy

```bash
./bin/ssh-docker-proxy \
  -ssh-user=your-username \
  -ssh-host=your-server.com:22 \
  -ssh-key=~/.ssh/id_rsa \
  -local-socket=/tmp/docker-proxy.sock
```

### 4. Use Docker Commands

In another terminal:

```bash
export DOCKER_HOST=unix:///tmp/docker-proxy.sock
docker version
docker ps
docker run --rm alpine:latest echo "Hello from remote Docker!"
```

## Configuration

### Command Line Flags

```bash
./bin/ssh-docker-proxy --help
```

### Configuration File

Create `config.yaml`:

```yaml
local_socket: "/tmp/docker-proxy.sock"
ssh_user: "your-username"
ssh_host: "your-server.com:22"
ssh_key_path: "~/.ssh/id_rsa"
remote_socket: "/var/run/docker.sock"
timeout: 10s
```

Then run:

```bash
./bin/ssh-docker-proxy -config=config.yaml
```

## Testing

### Unit Tests

```bash
go test ./...
```

### Integration Test

```bash
./test-local.sh
```

## Supported Docker Operations

- ✅ **Basic Commands**: `docker ps`, `docker images`, `docker version`
- ✅ **Container Management**: `docker run`, `docker stop`, `docker rm`
- ✅ **Image Operations**: `docker pull`, `docker push`, `docker build`
- ✅ **Streaming**: `docker logs -f`, `docker exec -it`, `docker attach`
- ✅ **Large Transfers**: Multi-GB build contexts and image layers

## Requirements

- Go 1.23+
- SSH access to remote server with Docker installed
- SSH key-based authentication configured

## Architecture

```
Local Docker Client → Unix Socket → SSH Docker Proxy → SSH → Remote Docker Daemon
```

The proxy creates a Unix socket that Docker clients connect to, then forwards all traffic over SSH to the remote Docker daemon using pure byte relay.

## Troubleshooting

### Common Issues

1. **"Connection refused"**: Check if proxy is running and socket path is correct
2. **"SSH connection failed"**: Verify SSH credentials and host accessibility  
3. **"Docker daemon unreachable"**: Ensure Docker is running on remote host
4. **Permission denied**: Check SSH key permissions (should be 600)

### Debug Mode

Add verbose logging by checking proxy output for connection details.

### Test SSH Connection

```bash
ssh -i ~/.ssh/id_rsa username@hostname 'docker ps'
```

## Performance

- **Latency**: Near-zero overhead, limited only by SSH connection latency
- **Throughput**: Achieves full SSH connection bandwidth
- **Memory**: Constant ~32KB buffer usage per connection
- **Concurrency**: No degradation with multiple simultaneous operations

## Security

- Uses SSH for encrypted, authenticated connections
- No Docker credentials stored locally
- Supports SSH key-based authentication
- Per-connection isolation prevents cross-talk

## License

MIT License - see LICENSE file for details.