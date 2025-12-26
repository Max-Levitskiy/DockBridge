# Comprehensive Implementation Plan for Hetzner Docker Client-Server System

Based on your requirements, I've created a detailed implementation plan for a sophisticated Go-based system that automatically provisions Hetzner Cloud servers for Docker containers with laptop lock detection and keep-alive mechanisms.

## System Architecture Overview

The system consists of two main components running in a **monorepo structure**:
- **Client**: Runs on your laptop, manages Docker socket proxying, screen lock detection, and keep-alive messaging
- **Server**: Runs on Hetzner Cloud instances, receives Docker commands via HTTP and manages server lifecycle

## Technology Stack & Libraries

### Core Libraries

**Docker Integration:**
- `github.com/docker/docker/client` - Official Docker Go SDK for Docker API interactions[1][2]
- `github.com/docker/go-connections/sockets` - Docker socket connections[1]

**Hetzner Cloud Management:**
- `github.com/hetznercloud/hcloud-go/v2/hcloud` - Official Hetzner Cloud Go library for server provisioning, volume management, and API operations[3][4]

**SSH & Networking:**
- `golang.org/x/crypto/ssh` - Official SSH client implementation[5]
- `github.com/melbahja/goph` - Higher-level SSH client wrapper with key management[6]

**CLI Framework:**
- `github.com/spf13/cobra` - Modern CLI framework used by Kubernetes, Hugo, GitHub CLI[7][8]
- `github.com/spf13/viper` - Configuration management with YAML, environment variables, and flags support[9][10]

**Screen Lock Detection:**
- `github.com/IamFaizanKhalid/lock` - Cross-platform screen lock detection (Windows, macOS, Linux)[11]
- Custom implementations for D-Bus monitoring on Linux[12][13]

**Testing Framework:**
- `github.com/stretchr/testify` - Comprehensive testing toolkit with assertions, mocks, and suites[14][15][16]

**HTTP & Keep-Alive:**
- Go's built-in `net/http` with keep-alive support[17][18]
- Custom HTTP proxy implementation for Docker socket forwarding[19][20]

## Detailed Project Structure

```
hetzner-docker-manager/
├── cmd/
│   ├── client/
│   │   └── main.go                    # Client CLI entry point
│   └── server/
│       └── main.go                    # Server HTTP entry point
├── internal/
│   ├── client/
│   │   ├── config/
│   │   │   ├── config.go              # Configuration management
│   │   │   └── config_test.go
│   │   ├── docker/
│   │   │   ├── proxy.go               # Docker socket proxy
│   │   │   ├── client.go              # Docker API wrapper
│   │   │   └── proxy_test.go
│   │   ├── hetzner/
│   │   │   ├── client.go              # Hetzner Cloud API client
│   │   │   ├── provisioner.go         # Server provisioning logic
│   │   │   └── provisioner_test.go
│   │   ├── keepalive/
│   │   │   ├── client.go              # Keep-alive sender
│   │   │   └── client_test.go
│   │   ├── lockdetection/
│   │   │   ├── detector.go            # Screen lock detection
│   │   │   ├── detector_linux.go      # Linux-specific implementation
│   │   │   ├── detector_windows.go    # Windows-specific implementation
│   │   │   ├── detector_darwin.go     # macOS-specific implementation
│   │   │   └── detector_test.go
│   │   └── ssh/
│   │       ├── client.go              # SSH client wrapper
│   │       └── client_test.go
│   ├── server/
│   │   ├── docker/
│   │   │   ├── handler.go             # Docker command handler
│   │   │   └── handler_test.go
│   │   ├── keepalive/
│   │   │   ├── monitor.go             # Keep-alive monitor
│   │   │   └── monitor_test.go
│   │   └── lifecycle/
│   │       ├── manager.go             # Server lifecycle management
│   │       └── manager_test.go
│   ├── shared/
│   │   ├── types/
│   │   │   └── types.go               # Shared data structures
│   │   └── utils/
│   │       ├── http.go                # HTTP utilities
│   │       └── crypto.go              # Cryptographic utilities
│   └── pkg/
│       ├── logger/
│       │   └── logger.go              # Structured logging
│       └── errors/
│           └── errors.go              # Custom error types
├── configs/
│   ├── client.yaml                    # Default client configuration
│   └── server.yaml                    # Default server configuration
├── scripts/
│   ├── install-server.sh              # Server installation script
│   └── build-release.sh               # Build script for releases
├── .github/
│   └── workflows/
│       ├── test.yml                   # CI/CD testing pipeline
│       ├── release.yml                # Release automation
│       └── security.yml               # Security scanning
├── docs/
│   ├── README.md
│   ├── INSTALLATION.md
│   └── CONFIGURATION.md
├── go.mod
├── go.sum
├── Makefile                           # Development shortcuts
└── LICENSE
```

## Implementation Components

### 1. Client Architecture

**Configuration Management** using Viper[10]:
```yaml
# configs/client.yaml
hetzner:
  api_token: ""  # Set via environment or CLI
  server_type: "cpx21"
  location: "fsn1"
  volume_size: 10  # GB
  
docker:
  socket_path: "/var/run/docker.sock"
  proxy_port: 2376
  
keepalive:
  interval: "30s"
  timeout: "5m"
  
ssh:
  private_key_path: "~/.ssh/hetzner_docker"
  public_key_path: "~/.ssh/hetzner_docker.pub"
```

**Docker Socket Proxy** implementing HTTP forwarding[19][20]:
- Intercepts Docker API calls
- Forwards to remote Hetzner server via HTTPS
- Maintains connection pooling and keep-alive

**Screen Lock Detection** with cross-platform support[11]:
- Linux: D-Bus monitoring for screen saver events[12]
- Windows: Win32 API desktop switching detection[21][22]
- macOS: Core Graphics session state monitoring

**Keep-Alive Client** with HTTP persistence[17]:
- Sends periodic heartbeat messages
- Handles connection failures gracefully
- Triggers server shutdown on timeout

### 2. Server Architecture

**Docker Command Handler**:
- Receives proxied Docker API calls
- Executes commands on local Docker daemon
- Returns responses maintaining API compatibility

**Keep-Alive Monitor**:
- Tracks client heartbeat messages
- Implements configurable timeout logic
- Initiates graceful shutdown sequence

**Lifecycle Manager**:
- Handles server self-destruction via Hetzner API
- Manages volume detachment before termination
- Implements cleanup procedures

### 3. Hetzner Integration

**Server Provisioning** using hcloud-go[3][4]:
```go
// Provision server with Docker CE image
server, _, err := client.Server.Create(ctx, hcloud.ServerCreateOpts{
    Name:       "docker-" + uuid.New().String()[:8],
    ServerType: &hcloud.ServerType{Name: config.ServerType},
    Image:      &hcloud.Image{Name: "docker-ce"},  // Pre-installed Docker
    Location:   &hcloud.Location{Name: config.Location},
    SSHKeys:    []*hcloud.SSHKey{sshKey},
    UserData:   cloudInitScript,
    Volumes:    []*hcloud.Volume{volume},
})
```

**Volume Management**:
- Create persistent volumes for data storage
- Automatic attachment/detachment
- Volume expansion support[previous conversation]

**SSH Key Management**:
- Automatic SSH key generation and upload
- Secure key storage and rotation
- Integration with Hetzner SSH key API

## Testing Strategy (TDD Approach)

### Test Structure using Testify[14][15]:

```go
// Example test suite structure
type ClientTestSuite struct {
    suite.Suite
    mockHetznerClient *mocks.HetznerClient
    mockDockerClient  *mocks.DockerClient
    client           *client.Client
}

func (suite *ClientTestSuite) SetupTest() {
    suite.mockHetznerClient = new(mocks.HetznerClient)
    suite.mockDockerClient = new(mocks.DockerClient)
    suite.client = client.New(suite.mockHetznerClient, suite.mockDockerClient)
}

func (suite *ClientTestSuite) TestServerProvisioning() {
    // TDD: Red -> Green -> Refactor cycle
    suite.mockHetznerClient.On("CreateServer", mock.Anything).Return(server, nil)
    
    result, err := suite.client.ProvisionServer(ctx, config)
    
    suite.NoError(err)
    suite.Equal(expected, result)
    suite.mockHetznerClient.AssertExpectations(suite.T())
}
```

### CI/CD Pipeline using GitHub Actions[23][24]:

```yaml
# .github/workflows/test.yml
name: Test and Build
on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v3
        with:
          go-version: '1.21'
      - run: go test -v -race -coverprofile=coverage.out ./...
      - run: go vet ./...
      - uses: golangci/golangci-lint-action@v3
```

## CLI Commands Structure using Cobra[7][8]:

```bash
# Initialize client configuration
hetzner-docker init --api-key=

# Start client daemon
hetzner-docker client start

# Configure server settings
hetzner-docker config set server-type cpx31
hetzner-docker config set location nbg1

# Manual server operations
hetzner-docker server create
hetzner-docker server destroy
hetzner-docker server status

# View logs and monitoring
hetzner-docker logs client
hetzner-docker logs server
```

## Implementation Instructions for AI Agent

### Phase 1: Core Infrastructure
1. **Setup monorepo structure** with Go modules and proper package organization
2. **Implement configuration management** using Viper with YAML support and environment variable overrides
3. **Create CLI framework** with Cobra, including all subcommands and flag definitions
4. **Setup testing framework** with Testify suites and mock generation

### Phase 2: Client Components
1. **Implement Docker socket proxy** with HTTP forwarding and connection pooling
2. **Create Hetzner API client** with server provisioning, volume management, and SSH key handling
3. **Develop screen lock detection** with platform-specific implementations
4. **Build keep-alive client** with retry logic and graceful error handling

### Phase 3: Server Components
1. **Create Docker command handler** with API compatibility and error forwarding
2. **Implement keep-alive monitor** with timeout detection and cleanup procedures
3. **Build lifecycle manager** with self-destruction capabilities via Hetzner API

### Phase 4: Integration & Testing
1. **Write comprehensive unit tests** following TDD methodology with high coverage
2. **Implement integration tests** with real Hetzner API calls using test credentials
3. **Setup GitHub Actions CI/CD** with automated testing, building, and release creation
4. **Create installation scripts** and documentation

### Phase 5: Release & Documentation
1. **Implement GitHub Actions release workflow** with binary building for multiple platforms
2. **Create comprehensive documentation** with setup guides, configuration examples, and troubleshooting
3. **Add security scanning** and dependency management
4. **Implement proper logging** and error reporting throughout the system

## Key Features to Implement

### Authentication & Security:
- SSH key generation and management
- Hetzner API token validation
- Secure communication channels
- Proper secret handling in configuration

### Docker Integration:
- Full Docker API compatibility
- Socket forwarding with minimal latency
- Container lifecycle management
- Volume mounting support

### Automation Features:
- Automatic server provisioning on first Docker command
- Graceful shutdown on laptop lock detection
- Keep-alive monitoring with configurable timeouts
- Volume persistence across server recreations

### Monitoring & Logging:
- Structured logging throughout the system
- Performance metrics collection
- Error reporting and recovery
- Health check endpoints

This comprehensive implementation plan provides a robust foundation for building a production-ready system that meets all your requirements while following Go best practices and utilizing proven libraries from the ecosystem.

