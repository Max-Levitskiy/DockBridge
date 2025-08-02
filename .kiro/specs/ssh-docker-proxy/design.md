# Design Document

## Overview

The SSH Docker Proxy implements a transparent HTTP proxy that forwards Docker API requests from local clients to remote Docker daemons over SSH connections. The design follows a pure byte-relay approach, avoiding Docker-specific parsing to ensure 100% compatibility with all Docker features including streaming operations, interactive sessions, and large file transfers.

## Architecture

### High-Level Architecture

```
                 (local machine)
+-------------------+           +-------------------------------+
| any docker client |  ------>  |     proxy (this project)     |
+-------------------+           |  - listens on /tmp/docker.sock|
                                |  - dials remote over SSH      |
                                +-------------------------------+
                                                     ||
                                                     ||  SSH TCP forward
                                                     \/
                                    (remote host)
                                +--------------------+
                                |  dockerd -H 2375   |
                                +--------------------+
```

### Core Design Principles

1. **Pure HTTP Proxy**: Forward raw bytes without Docker-specific interpretation
2. **SSH Transport**: Use SSH for secure, authenticated connections to remote hosts
3. **Per-Connection SSH Streams**: Create fresh SSH streams for each Docker client connection
4. **Minimal Dependencies**: Use Docker SDK only for health checks and version negotiation
5. **Library + CLI**: Provide both programmatic API and command-line interface

## Components and Interfaces

### 1. Configuration Management

```go
// Config represents the proxy configuration
type Config struct {
    LocalSocket  string // Local Unix socket path (e.g., /tmp/docker.sock)
    SSHUser      string // SSH username
    SSHHost      string // SSH hostname with optional port
    SSHKeyPath   string // Path to SSH private key file
    RemoteSocket string // Remote Docker socket path (default: /var/run/docker.sock)
    Timeout      time.Duration // SSH connection timeout
}

// Validate ensures configuration is complete and valid
func (c *Config) Validate() error
```

### 2. SSH Connection Manager

```go
// SSHDialer manages SSH connections and provides connection factory
type SSHDialer struct {
    config *Config
    sshConfig *ssh.ClientConfig
}

// NewSSHDialer creates a new SSH dialer with the given configuration
func NewSSHDialer(config *Config) (*SSHDialer, error)

// Dial establishes a new SSH connection to the remote Docker socket
func (d *SSHDialer) Dial() (net.Conn, error)

// HealthCheck verifies the remote Docker daemon is accessible
func (d *SSHDialer) HealthCheck(ctx context.Context) error
```

### 3. Proxy Server

```go
// Proxy represents the main proxy server
type Proxy struct {
    config    *Config
    dialer    *SSHDialer
    listener  net.Listener
    logger    Logger
    ctx       context.Context
    cancel    context.CancelFunc
}

// NewProxy creates a new proxy instance
func NewProxy(config *Config, logger Logger) (*Proxy, error)

// Start begins listening for connections and serving requests
func (p *Proxy) Start(ctx context.Context) error

// Stop gracefully shuts down the proxy
func (p *Proxy) Stop() error
```

### 4. Connection Handler

```go
// handleConnection processes a single client connection
func (p *Proxy) handleConnection(localConn net.Conn)

// relayTraffic performs bidirectional byte copying between connections
func relayTraffic(local, remote net.Conn, logger Logger)
```

### 5. CLI Interface

```go
// CLI provides command-line interface for the proxy
type CLI struct {
    config *Config
    logger Logger
}

// NewCLI creates a new CLI instance
func NewCLI() *CLI

// Execute runs the CLI with the given arguments
func (c *CLI) Execute(args []string) error
```

## Data Models

### Configuration Structure

The proxy uses a simple configuration model with validation:

```go
type Config struct {
    // Local socket configuration
    LocalSocket string `yaml:"local_socket" validate:"required"`
    
    // SSH connection parameters
    SSHUser    string `yaml:"ssh_user" validate:"required"`
    SSHHost    string `yaml:"ssh_host" validate:"required"`
    SSHKeyPath string `yaml:"ssh_key_path" validate:"required,file"`
    
    // Remote Docker configuration
    RemoteSocket string `yaml:"remote_socket"` // defaults to /var/run/docker.sock
    
    // Connection settings
    Timeout time.Duration `yaml:"timeout"` // defaults to 10s
}
```

### SSH Client Configuration

```go
type sshClientConfig struct {
    User            string
    Auth            []ssh.AuthMethod
    HostKeyCallback ssh.HostKeyCallback
    Timeout         time.Duration
}
```

## Error Handling

### Error Categories

1. **Configuration Errors**: Invalid or missing configuration parameters
2. **SSH Connection Errors**: Authentication failures, network issues, host unreachable
3. **Docker Daemon Errors**: Remote daemon unavailable, API version mismatch
4. **Proxy Runtime Errors**: Socket creation failures, connection relay errors

### Error Handling Strategy

```go
// ProxyError represents categorized proxy errors
type ProxyError struct {
    Category string
    Message  string
    Cause    error
}

func (e *ProxyError) Error() string {
    return fmt.Sprintf("[%s] %s: %v", e.Category, e.Message, e.Cause)
}

// Error categories
const (
    ErrorCategoryConfig    = "CONFIG"
    ErrorCategorySSH       = "SSH"
    ErrorCategoryDocker    = "DOCKER"
    ErrorCategoryRuntime   = "RUNTIME"
)
```

### Graceful Degradation

- SSH connection failures result in immediate proxy shutdown with clear error messages
- Individual connection relay failures are logged but don't affect other connections
- Socket cleanup is performed on shutdown regardless of error conditions

## Testing Strategy

### Unit Testing

1. **Configuration Validation**: Test all validation rules and edge cases
2. **SSH Dialer**: Mock SSH connections to test connection establishment and error handling
3. **Proxy Logic**: Test connection handling and traffic relay with mock connections
4. **CLI Interface**: Test command-line parsing and flag validation

### Integration Testing

1. **End-to-End Proxy**: Test with real SSH connections to Docker daemon
2. **Docker Command Compatibility**: Verify common Docker commands work correctly
3. **Streaming Operations**: Test `docker exec -it`, `docker logs -f`, `docker build`
4. **Concurrent Connections**: Test multiple simultaneous Docker operations

### Test Infrastructure

```go
// MockSSHDialer for unit testing
type MockSSHDialer struct {
    dialFunc func() (net.Conn, error)
    healthFunc func(context.Context) error
}

// TestProxy creates a proxy instance for testing
func TestProxy(config *Config, dialer SSHDialer) *Proxy

// Integration test helpers
func SetupTestDockerDaemon() (cleanup func(), endpoint string)
func SetupTestSSHServer() (cleanup func(), host string, keyPath string)
```

### Performance Testing

- Connection establishment latency
- Throughput for large file transfers (docker build contexts)
- Memory usage under concurrent connections
- SSH connection pooling effectiveness

## Implementation Details

### SSH Connection Management

The proxy creates a fresh SSH connection for each Docker client connection to ensure isolation and prevent connection state issues:

```go
func (p *Proxy) handleConnection(localConn net.Conn) {
    defer localConn.Close()
    
    // Create fresh SSH connection for this client
    remoteConn, err := p.dialer.Dial()
    if err != nil {
        p.logger.Error("Failed to establish SSH connection", "error", err)
        return
    }
    defer remoteConn.Close()
    
    // Relay traffic bidirectionally
    relayTraffic(localConn, remoteConn, p.logger)
}
```

### Traffic Relay Implementation

The core proxy logic uses `io.Copy` for efficient byte copying:

```go
func relayTraffic(local, remote net.Conn, logger Logger) {
    done := make(chan struct{}, 2)
    
    // Copy from local to remote
    go func() {
        defer func() { done <- struct{}{} }()
        if _, err := io.Copy(remote, local); err != nil {
            logger.Debug("Local->Remote copy ended", "error", err)
        }
    }()
    
    // Copy from remote to local
    go func() {
        defer func() { done <- struct{}{} }()
        if _, err := io.Copy(local, remote); err != nil {
            logger.Debug("Remote->Local copy ended", "error", err)
        }
    }()
    
    // Wait for either direction to complete
    <-done
}
```

### Health Check Implementation

Optional Docker SDK usage for health verification:

```go
func (d *SSHDialer) HealthCheck(ctx context.Context) error {
    // Create Docker client with custom dialer
    client, err := client.NewClientWithOpts(
        client.WithHost("http://dummy"), // not used
        client.WithAPIVersionNegotiation(),
        client.WithDialContext(func(ctx context.Context, _, _ string) (net.Conn, error) {
            return d.Dial()
        }),
    )
    if err != nil {
        return fmt.Errorf("failed to create Docker client: %w", err)
    }
    defer client.Close()
    
    // Perform ping to verify connectivity
    _, err = client.Ping(ctx)
    return err
}
```