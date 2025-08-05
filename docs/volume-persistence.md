# Enhanced Volume Management for Docker State Persistence

## Overview

DockBridge now includes enhanced volume management that ensures complete Docker state persistence across server recreations. This means all your Docker containers, images, volumes, and data survive when servers are destroyed and recreated.

## Key Features

### 1. Automatic Volume Creation and Management
- Automatically creates persistent volumes for Docker data storage
- Uses ext4 filesystem optimized for Docker workloads
- Handles volume formatting, mounting, and health monitoring

### 2. Docker State Persistence
- Mounts persistent volume at `/var/lib/docker` (Docker's default data directory)
- Preserves all Docker containers, images, volumes, and configuration
- Maintains Docker daemon state across server recreations

### 3. Enhanced Cloud-Init Script
- Robust volume detection and mounting logic
- Automatic backup and restore of existing Docker data
- Health checks and monitoring for volume integrity
- Comprehensive error handling and logging

### 4. Intelligent Server Lifecycle
- Destroys servers while preserving volumes
- Reuses existing volumes when creating new servers
- Automatic volume reattachment and state restoration

## How It Works

### Volume Creation Process

1. **Volume Detection**: Checks for existing Docker data volumes in the specified location
2. **Volume Creation**: Creates new volume if none exists, with proper labeling and formatting
3. **Filesystem Setup**: Formats volume with ext4 filesystem and appropriate labels
4. **Mount Configuration**: Adds volume to `/etc/fstab` for persistent mounting

### Server Provisioning Process

1. **Volume Preparation**: Ensures Docker data volume exists and is available
2. **Cloud-Init Generation**: Creates enhanced cloud-init script with volume mounting logic
3. **Server Creation**: Provisions server with volume attached
4. **Volume Mounting**: Mounts volume at `/var/lib/docker` during server initialization
5. **Docker Configuration**: Configures Docker daemon to use the persistent volume
6. **State Restoration**: Restores any existing Docker data from previous installations

### Volume Mounting Logic

The enhanced cloud-init script includes sophisticated volume mounting logic:

```bash
# Wait for volume device to be available
for i in {1..60}; do
  for device in /dev/sdb /dev/vdb /dev/xvdb; do
    if [ -b "$device" ]; then
      VOLUME_DEVICE="$device"
      break 2
    fi
  done
  sleep 5
done

# Backup existing Docker data
if [ -d "/var/lib/docker" ] && [ "$(ls -A /var/lib/docker)" ]; then
  cp -a /var/lib/docker/* /tmp/docker-backup/
fi

# Format and mount volume
mkfs.ext4 -F -L "docker-data" "$VOLUME_DEVICE"
mount "$VOLUME_DEVICE" /var/lib/docker

# Restore Docker data
if [ -d "/tmp/docker-backup" ]; then
  cp -a /tmp/docker-backup/* /var/lib/docker/
fi
```

## Configuration

### Volume Settings

```yaml
hetzner:
  volume_size: 10  # Volume size in GB (minimum 10GB)
  location: "fsn1" # Hetzner location for volume
```

### Docker Configuration

The Docker daemon is automatically configured to use the persistent volume:

```json
{
  "data-root": "/var/lib/docker",
  "storage-driver": "overlay2",
  "live-restore": true
}
```

## Usage Examples

### Basic Usage

```go
// Create server manager
serverManager := server.NewManager(hetznerClient, config)

// Ensure volume exists
volume, err := serverManager.EnsureVolume(ctx)
if err != nil {
    log.Fatal(err)
}

// Provision server with persistent volume
server, err := serverManager.EnsureServer(ctx)
if err != nil {
    log.Fatal(err)
}

// Server now has Docker state persistence
fmt.Printf("Server %s has persistent Docker data at %s\n", 
    server.Name, server.Metadata["docker_data_dir"])
```

### Volume Persistence Demo

```bash
# Set your Hetzner API token
export HETZNER_API_TOKEN="your-token-here"

# Run the volume persistence demo
go run cmd/demo-volume-persistence/main.go
```

This demo will:
1. Create a persistent volume for Docker data
2. Provision a server with the volume mounted
3. Destroy the server (preserving the volume)
4. Create a new server that reuses the same volume
5. Verify that Docker state is preserved

## Health Monitoring

### Volume Health Checks

The system includes automatic health monitoring:

```bash
# Health check script runs every 5 minutes
/usr/local/bin/docker-volume-health-check

# Checks performed:
# - Volume is properly mounted
# - Volume is writable
# - Sufficient disk space available
# - Docker daemon is using the volume
```

### Monitoring Logs

```bash
# View volume health logs
tail -f /var/log/docker-volume-health.log

# View cloud-init completion status
cat /var/log/cloud-init-complete

# View volume information
cat /etc/dockbridge/volume-info
```

## Troubleshooting

### Common Issues

1. **Volume Not Mounting**
   - Check if volume device is available: `lsblk`
   - Verify volume is attached to server in Hetzner console
   - Check cloud-init logs: `cat /var/log/cloud-init-output.log`

2. **Docker Data Not Persisting**
   - Verify Docker is using correct data directory: `docker info | grep "Docker Root Dir"`
   - Check volume mount: `mountpoint /var/lib/docker`
   - Verify volume UUID in fstab: `cat /etc/fstab`

3. **Volume Health Check Failures**
   - Check disk space: `df -h /var/lib/docker`
   - Verify volume permissions: `ls -la /var/lib/docker`
   - Test volume write access: `touch /var/lib/docker/.test && rm /var/lib/docker/.test`

### Recovery Procedures

1. **Manual Volume Remount**
   ```bash
   # Stop Docker
   systemctl stop docker
   
   # Remount volume
   umount /var/lib/docker
   mount /dev/sdb /var/lib/docker
   
   # Start Docker
   systemctl start docker
   ```

2. **Volume Filesystem Check**
   ```bash
   # Stop Docker and unmount volume
   systemctl stop docker
   umount /var/lib/docker
   
   # Check filesystem
   fsck.ext4 -f /dev/sdb
   
   # Remount and restart Docker
   mount /dev/sdb /var/lib/docker
   systemctl start docker
   ```

## Testing

### Unit Tests

```bash
# Run server manager tests
go test ./internal/server/...

# Run volume persistence tests
go test ./internal/server/ -run TestDockerStatePersistence
```

### Integration Tests

```bash
# Set API token for integration tests
export HETZNER_API_TOKEN="your-token-here"

# Run integration tests (requires real Hetzner resources)
go test ./internal/server/ -run TestDockerStatePersistenceIntegration
```

### Manual Testing

1. **Create Server with Volume**
   ```bash
   dockbridge start
   # Wait for server provisioning
   ```

2. **Create Docker State**
   ```bash
   docker run -d --name test-container nginx
   docker pull ubuntu:latest
   docker volume create test-volume
   ```

3. **Destroy and Recreate Server**
   ```bash
   dockbridge stop
   # Server is destroyed, volume preserved
   
   dockbridge start
   # New server created with same volume
   ```

4. **Verify State Persistence**
   ```bash
   docker ps -a  # Should show test-container
   docker images # Should show ubuntu:latest
   docker volume ls # Should show test-volume
   ```

## Best Practices

### Volume Management
- Use minimum 10GB volume size for Docker workloads
- Monitor disk usage regularly
- Keep volumes in same location as servers for performance
- Use descriptive volume labels for identification

### Server Lifecycle
- Always destroy servers through DockBridge to preserve volumes
- Monitor server costs and set up automatic destruction
- Use activity-based lifecycle management for cost optimization

### Backup and Recovery
- Consider periodic volume snapshots for critical data
- Test volume recovery procedures regularly
- Monitor volume health checks and alerts

## Cost Optimization

### Volume Costs
- Volumes incur ongoing costs even when servers are destroyed
- 10GB volume costs approximately €0.40/month
- Consider volume cleanup for unused volumes

### Server Costs
- Servers are destroyed automatically when idle
- Volumes persist across server recreations
- Only pay for compute when actually using Docker

## Security Considerations

### Volume Security
- Volumes are encrypted at rest by Hetzner
- Access controlled through server SSH keys
- Volume data isolated per customer

### Network Security
- Docker API exposed only on private network
- SSH access required for volume management
- Firewall rules restrict external access

## Future Enhancements

### Planned Features
- Volume snapshots and backups
- Multi-region volume replication
- Volume encryption key management
- Advanced volume monitoring and alerting
- Volume resize capabilities

### Performance Optimizations
- SSD volume support
- Volume caching strategies
- Network-attached storage options
- Volume performance monitoring