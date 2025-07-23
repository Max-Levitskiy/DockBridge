# Hetzner Cloud API Client

This package provides a comprehensive Go client for managing Hetzner Cloud resources, specifically designed for the DockBridge project. It includes server provisioning, volume management, SSH key handling, and lifecycle management with proper cleanup procedures.

## Features

- **Server Provisioning**: Create servers with Docker CE pre-installed via cloud-init
- **Volume Management**: Create, attach, detach, and manage persistent volumes
- **SSH Key Management**: Upload and manage SSH keys for server access
- **Lifecycle Management**: Provision servers with volumes and handle proper cleanup
- **Cloud-Init Integration**: Generate cloud-init scripts for automated server setup
- **Comprehensive Testing**: Unit tests with mocked Hetzner API responses

## Quick Start

### Basic Usage

```go
package main

import (
    "context"
    "log"
    "os"
    
    "github.com/dockbridge/dockbridge/internal/client/hetzner"
)

func main() {
    // Create client configuration
    config := &hetzner.Config{
        APIToken:   os.Getenv("HETZNER_API_TOKEN"),
        ServerType: "cpx21",
        Location:   "fsn1",
        VolumeSize: 10,
    }

    // Create Hetzner client
    client, err := hetzner.NewClient(config)
    if err != nil {
        log.Fatal(err)
    }

    ctx := context.Background()

    // List existing servers
    servers, err := client.ListServers(ctx)
    if err != nil {
        log.Fatal(err)
    }

    for _, server := range servers {
        log.Printf("Server: %s (ID: %d, Status: %s)", 
            server.Name, server.ID, server.Status)
    }
}
```

### Server Provisioning with Lifecycle Management

```go
// Create lifecycle manager
lifecycleManager := hetzner.NewLifecycleManager(client)

// Configure server provisioning
provisionConfig := &hetzner.ServerProvisionConfig{
    ServerName:    "dockbridge-dev",
    ServerType:    "cpx21",
    Location:      "fsn1",
    VolumeSize:    10,
    SSHPublicKey:  "ssh-rsa AAAAB3NzaC1yc2E...",
    KeepAlivePort: 8080,
    DockerAPIPort: 2376,
}

// Provision server with volume
serverWithVolume, err := lifecycleManager.ProvisionServerWithVolume(ctx, provisionConfig)
if err != nil {
    log.Fatal(err)
}

log.Printf("Server provisioned: %s (IP: %s)", 
    serverWithVolume.Server.Name, 
    serverWithVolume.Server.IPAddress)
```

### Cloud-Init Script Generation

```go
// Configure cloud-init
cloudInitConfig := &hetzner.CloudInitConfig{
    DockerVersion: "latest",
    SSHPublicKey:  "ssh-rsa AAAAB3NzaC1yc2E...",
    VolumeMount:   "/mnt/docker-data",
    KeepAlivePort: 8080,
    DockerAPIPort: 2376,
    Packages: []string{"htop", "vim", "curl"},
}

// Generate script
script := hetzner.GenerateCloudInitScript(cloudInitConfig)
```

## API Reference

### Client Interface

```go
type HetznerClient interface {
    ProvisionServer(ctx context.Context, config *ServerConfig) (*Server, error)
    DestroyServer(ctx context.Context, serverID string) error
    CreateVolume(ctx context.Context, size int, location string) (*Volume, error)
    AttachVolume(ctx context.Context, serverID, volumeID string) error
    DetachVolume(ctx context.Context, volumeID string) error
    ManageSSHKeys(ctx context.Context, publicKey string) (*SSHKey, error)
    GetServer(ctx context.Context, serverID string) (*Server, error)
    ListServers(ctx context.Context) ([]*Server, error)
    GetVolume(ctx context.Context, volumeID string) (*Volume, error)
    ListVolumes(ctx context.Context) ([]*Volume, error)
}
```

### Data Structures

#### Server
```go
type Server struct {
    ID        int64     `json:"id"`
    Name      string    `json:"name"`
    Status    string    `json:"status"`
    IPAddress string    `json:"ip_address"`
    VolumeID  string    `json:"volume_id"`
    CreatedAt time.Time `json:"created_at"`
}
```

#### Volume
```go
type Volume struct {
    ID       int64  `json:"id"`
    Name     string `json:"name"`
    Size     int    `json:"size"`
    Location string `json:"location"`
    Status   string `json:"status"`
}
```

#### SSH Key
```go
type SSHKey struct {
    ID          int64  `json:"id"`
    Name        string `json:"name"`
    Fingerprint string `json:"fingerprint"`
    PublicKey   string `json:"public_key"`
}
```

## Configuration

### Environment Variables

- `HETZNER_API_TOKEN`: Your Hetzner Cloud API token (required)

### Client Configuration

```go
type Config struct {
    APIToken   string // Hetzner Cloud API token
    ServerType string // Default server type (e.g., "cpx21")
    Location   string // Default location (e.g., "fsn1")
    VolumeSize int    // Default volume size in GB
}
```

### Server Provisioning Configuration

```go
type ServerProvisionConfig struct {
    ServerName    string // Name for the server
    ServerType    string // Server type (cpx21, cpx31, etc.)
    Location      string // Location (fsn1, nbg1, hel1, ash, hil)
    VolumeSize    int    // Volume size in GB
    VolumeMount   string // Mount point for volume
    SSHPublicKey  string // SSH public key for access
    KeepAlivePort int    // Port for keep-alive service
    DockerAPIPort int    // Port for Docker API
}
```

## Cloud-Init Features

The generated cloud-init script includes:

- **Docker CE Installation**: Latest Docker Community Edition
- **Volume Management**: Automatic volume formatting and mounting
- **Docker Configuration**: Optimized daemon settings for cloud usage
- **SSH Access**: Public key configuration
- **Firewall Setup**: UFW configuration for required ports
- **Log Rotation**: Docker log management
- **DockBridge Server**: Placeholder for server component
- **System Optimization**: Performance and security settings

## Testing

Run the test suite:

```bash
go test ./internal/client/hetzner/... -v
```

The tests include:
- Unit tests for all client methods
- Data structure conversion tests
- Cloud-init script generation tests
- Lifecycle management tests
- Error handling tests

## Error Handling

The client implements comprehensive error handling with:
- Wrapped errors with context
- Retry logic for transient failures
- Proper resource cleanup on failures
- Detailed error messages

## Security Considerations

- API tokens are handled securely
- SSH keys use RSA 4096-bit encryption
- All communications use TLS
- Firewall rules are automatically configured
- Volume encryption is supported

## Performance Optimizations

- Connection pooling for HTTP requests
- Concurrent operations support
- Efficient resource management
- Minimal API calls through caching

## Dependencies

- `github.com/hetznercloud/hcloud-go/v2`: Official Hetzner Cloud Go client
- `github.com/pkg/errors`: Enhanced error handling
- `github.com/stretchr/testify`: Testing framework

## Examples

See `example_usage.go` for comprehensive usage examples including:
- Basic server operations
- Volume management
- SSH key handling
- Cloud-init script generation
- Lifecycle management

## Requirements Mapping

This implementation addresses the following DockBridge requirements:

- **1.3**: Server provisioning with Docker CE
- **4.1**: Hetzner Cloud integration
- **4.2**: Volume creation and management
- **4.3**: SSH key management
- **6.1**: Cloud-init script generation
- **6.2**: Server lifecycle management
- **6.3**: Resource cleanup procedures
- **6.4**: Comprehensive error handling