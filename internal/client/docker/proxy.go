package docker

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/dockbridge/dockbridge/internal/client/hetzner"
	"github.com/dockbridge/dockbridge/internal/client/ssh"
	"github.com/dockbridge/dockbridge/internal/shared/config"
	"github.com/dockbridge/dockbridge/pkg/logger"
	"github.com/pkg/errors"
)

// DockerProxy defines the interface for Docker API proxying
type DockerProxy interface {
	Start(ctx context.Context, config *ProxyConfig) error
	Stop() error
	ForwardRequest(req *http.Request) (*http.Response, error)
	IsRunning() bool
}

// ProxyConfig holds configuration for the Docker proxy
type ProxyConfig struct {
	SocketPath    string
	ProxyPort     int
	HetznerClient hetzner.HetznerClient
	SSHConfig     *config.SSHConfig
	HetznerConfig *config.HetznerConfig
	Logger        logger.LoggerInterface
}

// proxyImpl implements the DockerProxy interface
type proxyImpl struct {
	config       *ProxyConfig
	server       *http.Server
	listener     net.Listener
	sshClient    ssh.Client
	tunnel       ssh.TunnelInterface
	reverseProxy *httputil.ReverseProxy
	running      bool
	mu           sync.RWMutex
	connPool     *connectionPool
	logger       logger.LoggerInterface
}

// connectionPool manages HTTP connections for performance optimization
type connectionPool struct {
	transport *http.Transport
	client    *http.Client
}

// NewDockerProxy creates a new Docker proxy instance
func NewDockerProxy() DockerProxy {
	return &proxyImpl{
		connPool: &connectionPool{
			transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
				DisableKeepAlives:   false,
			},
		},
	}
}

// Start initializes and starts the Docker proxy server
func (p *proxyImpl) Start(ctx context.Context, config *ProxyConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return errors.New("proxy is already running")
	}

	p.config = config
	p.logger = config.Logger

	// Initialize connection pool
	p.connPool.client = &http.Client{
		Transport: p.connPool.transport,
		Timeout:   30 * time.Second,
	}

	// Set up the HTTP server
	if err := p.setupServer(ctx); err != nil {
		return errors.Wrap(err, "failed to setup HTTP server")
	}

	// Start the server in a goroutine
	go func() {
		p.logger.Info("Starting Docker proxy server", map[string]interface{}{
			"socket_path": p.config.SocketPath,
			"proxy_port":  p.config.ProxyPort,
		})

		if err := p.server.Serve(p.listener); err != nil && err != http.ErrServerClosed {
			p.logger.Error("Docker proxy server error", err, nil)
		}
	}()

	p.running = true
	p.logger.Info("Docker proxy started successfully", nil)
	return nil
}

// Stop gracefully shuts down the Docker proxy
func (p *proxyImpl) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return nil
	}

	p.logger.Info("Stopping Docker proxy server", nil)

	// Close SSH tunnel if exists
	if p.tunnel != nil {
		if err := p.tunnel.Close(); err != nil {
			p.logger.Error("Failed to close SSH tunnel", err, nil)
		}
	}

	// Close SSH client if exists
	if p.sshClient != nil {
		if err := p.sshClient.Close(); err != nil {
			p.logger.Error("Failed to close SSH client", err, nil)
		}
	}

	// Shutdown HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := p.server.Shutdown(ctx); err != nil {
		p.logger.Error("Failed to shutdown HTTP server gracefully", err, nil)
		return err
	}

	// Close connection pool
	p.connPool.transport.CloseIdleConnections()

	p.running = false
	p.logger.Info("Docker proxy stopped successfully", nil)
	return nil
}

// IsRunning returns true if the proxy is currently running
func (p *proxyImpl) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

// setupServer configures the HTTP server and reverse proxy
func (p *proxyImpl) setupServer(ctx context.Context) error {
	// Create listener for Unix socket or TCP port
	var err error
	if strings.HasPrefix(p.config.SocketPath, "/") {
		// Unix socket
		p.listener, err = net.Listen("unix", p.config.SocketPath)
	} else {
		// TCP port
		addr := fmt.Sprintf(":%d", p.config.ProxyPort)
		p.listener, err = net.Listen("tcp", addr)
	}

	if err != nil {
		return errors.Wrap(err, "failed to create listener")
	}

	// Create HTTP server with custom handler
	mux := http.NewServeMux()
	mux.HandleFunc("/", p.handleDockerRequest)

	p.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return nil
}

// handleDockerRequest processes incoming Docker API requests
func (p *proxyImpl) handleDockerRequest(w http.ResponseWriter, r *http.Request) {
	p.logger.Debug("Handling Docker request", map[string]interface{}{
		"method": r.Method,
		"path":   r.URL.Path,
		"remote": r.RemoteAddr,
	})

	// Ensure we have a connection to a remote server
	if err := p.ensureRemoteConnection(r.Context()); err != nil {
		p.logger.Error("Failed to ensure remote connection", err, nil)
		http.Error(w, fmt.Sprintf("Failed to connect to remote server: %v", err), http.StatusServiceUnavailable)
		return
	}

	// Forward the request
	if err := p.forwardRequest(w, r); err != nil {
		p.logger.Error("Failed to forward request", err, map[string]interface{}{
			"method": r.Method,
			"path":   r.URL.Path,
		})
		http.Error(w, fmt.Sprintf("Failed to forward request: %v", err), http.StatusBadGateway)
		return
	}
}

// ensureRemoteConnection ensures we have an active connection to a remote server
func (p *proxyImpl) ensureRemoteConnection(ctx context.Context) error {
	// Check if we already have an active SSH connection
	if p.sshClient != nil && p.sshClient.IsConnected() && p.tunnel != nil {
		return nil
	}

	p.logger.Info("Establishing connection to remote server", nil)

	// Get or provision a server
	server, err := p.getOrProvisionServer(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get or provision server")
	}

	// Create SSH client
	sshConfig := &ssh.ClientConfig{
		Host:           server.IPAddress,
		Port:           p.config.SSHConfig.Port,
		User:           "root",
		PrivateKeyPath: p.config.SSHConfig.KeyPath,
		Timeout:        p.config.SSHConfig.Timeout,
	}

	p.sshClient = ssh.NewClient(sshConfig)

	// Connect to SSH server
	if err := p.sshClient.Connect(ctx); err != nil {
		return errors.Wrap(err, "failed to connect to SSH server")
	}

	// Create SSH tunnel for Docker API (port 2376 on remote)
	localAddr := "127.0.0.1:0" // Use random available port
	remoteAddr := "127.0.0.1:2376"

	p.tunnel, err = p.sshClient.CreateTunnel(ctx, localAddr, remoteAddr)
	if err != nil {
		return errors.Wrap(err, "failed to create SSH tunnel")
	}

	p.logger.Info("SSH tunnel established", map[string]interface{}{
		"local_addr":  p.tunnel.LocalAddr(),
		"remote_addr": remoteAddr,
		"server_ip":   server.IPAddress,
	})

	return nil
}

// getOrProvisionServer gets an existing server or provisions a new one
func (p *proxyImpl) getOrProvisionServer(ctx context.Context) (*hetzner.Server, error) {
	// First, try to find an existing DockBridge server
	servers, err := p.config.HetznerClient.ListServers(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list servers")
	}

	// Look for a running DockBridge server
	for _, server := range servers {
		if strings.HasPrefix(server.Name, "dockbridge-") && server.Status == "running" {
			p.logger.Info("Found existing DockBridge server", map[string]interface{}{
				"server_id": server.ID,
				"server_ip": server.IPAddress,
			})
			return server, nil
		}
	}

	// No existing server found, provision a new one
	p.logger.Info("No existing server found, provisioning new server", nil)
	return p.provisionNewServer(ctx)
}

// provisionNewServer creates a new Hetzner server with Docker CE
func (p *proxyImpl) provisionNewServer(ctx context.Context) (*hetzner.Server, error) {
	// Generate server name with timestamp
	serverName := fmt.Sprintf("dockbridge-%d", time.Now().Unix())

	// Create cloud-init script for Docker CE installation
	cloudInitScript := `#!/bin/bash
# Update system
apt-get update
apt-get upgrade -y

# Install Docker CE
curl -fsSL https://get.docker.com -o get-docker.sh
sh get-docker.sh

# Enable Docker service
systemctl enable docker
systemctl start docker

# Configure Docker daemon to listen on TCP port 2376 (insecure for internal use)
mkdir -p /etc/systemd/system/docker.service.d
cat > /etc/systemd/system/docker.service.d/override.conf << EOF
[Service]
ExecStart=
ExecStart=/usr/bin/dockerd -H fd:// -H tcp://0.0.0.0:2376
EOF

# Reload systemd and restart Docker
systemctl daemon-reload
systemctl restart docker

# Install additional tools
apt-get install -y htop curl wget git

# Create dockbridge user
useradd -m -s /bin/bash dockbridge
usermod -aG docker dockbridge

echo "DockBridge server setup completed" > /var/log/dockbridge-setup.log
`

	// TODO: Get SSH key ID from SSH key management
	// For now, we'll provision without SSH key and handle it separately
	serverConfig := &hetzner.ServerConfig{
		Name:       serverName,
		ServerType: p.config.HetznerConfig.ServerType,
		Location:   p.config.HetznerConfig.Location,
		UserData:   cloudInitScript,
	}

	server, err := p.config.HetznerClient.ProvisionServer(ctx, serverConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to provision server")
	}

	p.logger.Info("New server provisioned successfully", map[string]interface{}{
		"server_id":   server.ID,
		"server_name": server.Name,
		"server_ip":   server.IPAddress,
	})

	// Wait for server to be fully ready (Docker service to start)
	if err := p.waitForServerReady(ctx, server); err != nil {
		return nil, errors.Wrap(err, "server provisioned but not ready")
	}

	return server, nil
}

// waitForServerReady waits for the server to be fully configured and Docker to be running
func (p *proxyImpl) waitForServerReady(ctx context.Context, server *hetzner.Server) error {
	p.logger.Info("Waiting for server to be ready", map[string]interface{}{
		"server_id": server.ID,
	})

	// Wait up to 5 minutes for server to be ready
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return errors.New("timeout waiting for server to be ready")
		case <-ticker.C:
			// Try to connect and check if Docker is running
			if p.checkServerReady(ctx, server) {
				p.logger.Info("Server is ready", map[string]interface{}{
					"server_id": server.ID,
				})
				return nil
			}
		}
	}
}

// checkServerReady checks if the server is ready by attempting to connect and verify Docker
func (p *proxyImpl) checkServerReady(ctx context.Context, server *hetzner.Server) bool {
	// Create temporary SSH client to check server status
	sshConfig := &ssh.ClientConfig{
		Host:           server.IPAddress,
		Port:           p.config.SSHConfig.Port,
		User:           "root",
		PrivateKeyPath: p.config.SSHConfig.KeyPath,
		Timeout:        10 * time.Second,
	}

	tempSSHClient := ssh.NewClient(sshConfig)
	defer tempSSHClient.Close()

	// Try to connect
	if err := tempSSHClient.Connect(ctx); err != nil {
		p.logger.Debug("Server not ready - SSH connection failed", map[string]interface{}{
			"error": err.Error(),
		})
		return false
	}

	// Check if Docker is running
	output, err := tempSSHClient.ExecuteCommand(ctx, "systemctl is-active docker")
	if err != nil || strings.TrimSpace(string(output)) != "active" {
		p.logger.Debug("Server not ready - Docker not active", map[string]interface{}{
			"output": string(output),
			"error":  err,
		})
		return false
	}

	return true
}

// forwardRequest forwards the HTTP request through the SSH tunnel
func (p *proxyImpl) forwardRequest(w http.ResponseWriter, r *http.Request) error {
	if p.tunnel == nil {
		return errors.New("no SSH tunnel available")
	}

	// Create target URL using the tunnel's local address
	targetURL := &url.URL{
		Scheme:   "http",
		Host:     p.tunnel.LocalAddr(),
		Path:     r.URL.Path,
		RawQuery: r.URL.RawQuery,
	}

	// Create new request
	proxyReq, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL.String(), r.Body)
	if err != nil {
		return errors.Wrap(err, "failed to create proxy request")
	}

	// Copy headers
	for name, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(name, value)
		}
	}

	// Execute request through connection pool
	resp, err := p.connPool.client.Do(proxyReq)
	if err != nil {
		return errors.Wrap(err, "failed to execute proxy request")
	}
	defer resp.Body.Close()

	// Copy response headers
	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	// Set status code
	w.WriteHeader(resp.StatusCode)

	// Handle streaming responses
	if p.isStreamingResponse(resp) {
		return p.handleStreamingResponse(w, resp)
	}

	// Copy response body
	_, err = io.Copy(w, resp.Body)
	return err
}

// isStreamingResponse determines if the response should be streamed
func (p *proxyImpl) isStreamingResponse(resp *http.Response) bool {
	contentType := resp.Header.Get("Content-Type")

	// Check for streaming content types
	streamingTypes := []string{
		"application/vnd.docker.raw-stream",
		"application/vnd.docker.multiplexed-stream",
		"text/plain", // Docker logs often use text/plain
	}

	for _, streamType := range streamingTypes {
		if strings.Contains(contentType, streamType) {
			return true
		}
	}

	// Check for chunked transfer encoding
	if resp.Header.Get("Transfer-Encoding") == "chunked" {
		return true
	}

	return false
}

// handleStreamingResponse handles streaming responses like Docker logs, image pulls, etc.
func (p *proxyImpl) handleStreamingResponse(w http.ResponseWriter, resp *http.Response) error {
	// Ensure we can flush the response
	flusher, ok := w.(http.Flusher)
	if !ok {
		return errors.New("response writer does not support flushing")
	}

	// Buffer for streaming
	buffer := make([]byte, 32*1024) // 32KB buffer

	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			if _, writeErr := w.Write(buffer[:n]); writeErr != nil {
				return errors.Wrap(writeErr, "failed to write streaming response")
			}
			flusher.Flush()
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.Wrap(err, "failed to read streaming response")
		}
	}

	return nil
}

// ForwardRequest forwards a single HTTP request (for testing/external use)
func (p *proxyImpl) ForwardRequest(req *http.Request) (*http.Response, error) {
	if !p.running {
		return nil, errors.New("proxy is not running")
	}

	// Ensure remote connection
	if err := p.ensureRemoteConnection(req.Context()); err != nil {
		return nil, errors.Wrap(err, "failed to ensure remote connection")
	}

	if p.tunnel == nil {
		return nil, errors.New("no SSH tunnel available")
	}

	// Create target URL
	targetURL := &url.URL{
		Scheme:   "http",
		Host:     p.tunnel.LocalAddr(),
		Path:     req.URL.Path,
		RawQuery: req.URL.RawQuery,
	}

	// Create proxy request
	proxyReq, err := http.NewRequestWithContext(req.Context(), req.Method, targetURL.String(), req.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create proxy request")
	}

	// Copy headers
	for name, values := range req.Header {
		for _, value := range values {
			proxyReq.Header.Add(name, value)
		}
	}

	// Execute request
	return p.connPool.client.Do(proxyReq)
}
