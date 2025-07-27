package docker

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"strconv"
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
	config            *ProxyConfig
	server            *http.Server
	listener          net.Listener
	running           bool
	mu                sync.RWMutex
	logger            logger.LoggerInterface
	serverManager     *ServerManager
	connectionManager *ConnectionManager
	requestHandler    *RequestHandler
}

// NewDockerProxy creates a new Docker proxy instance
func NewDockerProxy() DockerProxy {
	return &proxyImpl{}
}

// Start initializes and starts the Docker proxy server
func (p *proxyImpl) Start(ctx context.Context, config *ProxyConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return errors.New("proxy is already running")
	}

	if config == nil {
		return errors.New("proxy config cannot be nil")
	}

	if config.Logger == nil {
		return errors.New("logger cannot be nil")
	}

	if config.HetznerClient == nil {
		return errors.New("hetzner client cannot be nil")
	}

	if config.SSHConfig == nil {
		return errors.New("SSH config cannot be nil")
	}

	if config.HetznerConfig == nil {
		return errors.New("Hetzner config cannot be nil")
	}

	p.config = config
	p.logger = config.Logger

	// Initialize components
	p.initializeComponents()

	// Set up the HTTP server
	if err := p.setupServer(); err != nil {
		return errors.Wrap(err, "failed to setup HTTP server")
	}

	// Start the server in a goroutine
	go func() {
		p.logger.WithFields(map[string]interface{}{
			"socket_path": p.config.SocketPath,
			"proxy_port":  p.config.ProxyPort,
		}).Info("Starting Docker proxy server")

		if err := p.server.Serve(p.listener); err != nil && err != http.ErrServerClosed {
			p.logger.WithFields(map[string]interface{}{
				"error": err.Error(),
			}).Error("Docker proxy server error")
		}
	}()

	p.running = true
	p.logger.Info("Docker proxy started successfully")
	return nil
}

// Stop gracefully shuts down the Docker proxy
func (p *proxyImpl) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return nil
	}

	p.logger.Info("Stopping Docker proxy server")

	// Close connection manager
	if p.connectionManager != nil {
		if err := p.connectionManager.Close(); err != nil {
			p.logger.WithFields(map[string]interface{}{
				"error": err.Error(),
			}).Error("Failed to close connection manager")
		}
	}

	// Shutdown HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := p.server.Shutdown(ctx); err != nil {
		p.logger.WithFields(map[string]interface{}{
			"error": err.Error(),
		}).Error("Failed to shutdown HTTP server gracefully")
		return err
	}

	p.running = false
	p.logger.Info("Docker proxy stopped successfully")
	return nil
}

// IsRunning returns true if the proxy is currently running
func (p *proxyImpl) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

// ForwardRequest forwards a single HTTP request (for testing/external use)
func (p *proxyImpl) ForwardRequest(req *http.Request) (*http.Response, error) {
	if !p.running {
		return nil, errors.New("proxy is not running")
	}

	return p.requestHandler.ForwardRequest(req)
}

// initializeComponents sets up the internal components
func (p *proxyImpl) initializeComponents() {
	// Create SSH config for components
	sshConfig := &ssh.ClientConfig{
		Host:           "", // Will be set when server is selected
		Port:           p.config.SSHConfig.Port,
		User:           "root",
		PrivateKeyPath: p.config.SSHConfig.KeyPath,
		Timeout:        p.config.SSHConfig.Timeout,
	}

	// Create Hetzner config for server manager
	hetznerConfig := &hetzner.Config{
		APIToken:   "", // Not needed for server manager
		ServerType: p.config.HetznerConfig.ServerType,
		Location:   p.config.HetznerConfig.Location,
		VolumeSize: p.config.HetznerConfig.VolumeSize,
	}

	// Initialize components
	p.serverManager = NewServerManager(p.config.HetznerClient, sshConfig, hetznerConfig, p.logger)
	p.connectionManager = NewConnectionManager(p.serverManager, p.config.SSHConfig, p.logger)
	p.connectionManager.Initialize()
	p.requestHandler = NewRequestHandler(p.connectionManager, p.logger)
}

// setupServer configures the HTTP server and reverse proxy
func (p *proxyImpl) setupServer() error {
	// Create listener for Unix socket or TCP port
	var err error
	if strings.HasPrefix(p.config.SocketPath, "/") {
		// Unix socket
		p.listener, err = net.Listen("unix", p.config.SocketPath)
		if err != nil {
			return errors.Wrap(err, "failed to create listener")
		}

		// Set proper permissions for the socket so docker group can access it
		if err := p.setSocketPermissions(p.config.SocketPath); err != nil {
			p.logger.WithFields(map[string]interface{}{
				"socket_path": p.config.SocketPath,
				"error":       err.Error(),
			}).Warn("Failed to set socket permissions, users may need sudo")
		}
	} else {
		// TCP port
		addr := fmt.Sprintf(":%d", p.config.ProxyPort)
		p.listener, err = net.Listen("tcp", addr)
		if err != nil {
			return errors.Wrap(err, "failed to create listener")
		}
	}

	// Create HTTP server with custom handler
	mux := http.NewServeMux()
	mux.HandleFunc("/", p.requestHandler.HandleDockerRequest)

	p.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return nil
}

// setSocketPermissions sets the correct permissions and group ownership for the Docker socket
func (p *proxyImpl) setSocketPermissions(socketPath string) error {
	// Set socket permissions to 666 (rw-rw-rw-) for broader access
	if err := os.Chmod(socketPath, 0666); err != nil {
		return errors.Wrap(err, "failed to set socket permissions")
	}

	// Try to get the appropriate group for Docker socket
	var targetGroup *user.Group
	var err error

	// On macOS, try daemon group first, then docker group on Linux
	groupNames := []string{"daemon", "docker"}

	for _, groupName := range groupNames {
		targetGroup, err = user.LookupGroup(groupName)
		if err == nil {
			break
		}
	}

	if err != nil {
		// If neither group exists, try to create docker group (Linux)
		p.logger.Debug("Neither daemon nor docker group found, attempting to create docker group")

		cmd := exec.Command("groupadd", "-f", "docker")
		if err := cmd.Run(); err != nil {
			p.logger.WithFields(map[string]interface{}{
				"error": err.Error(),
			}).Debug("Failed to create docker group, using world-readable permissions")
			return nil // Don't fail, socket is already 666
		}

		// Try to lookup docker group again
		targetGroup, err = user.LookupGroup("docker")
		if err != nil {
			p.logger.WithFields(map[string]interface{}{
				"error": err.Error(),
			}).Debug("Failed to lookup docker group after creation, using world-readable permissions")
			return nil // Don't fail, socket is already 666
		}
	}

	// Convert group ID to integer
	gid, err := strconv.Atoi(targetGroup.Gid)
	if err != nil {
		return errors.Wrap(err, "failed to parse group ID")
	}

	// Change group ownership of the socket
	if err := os.Chown(socketPath, -1, gid); err != nil {
		p.logger.WithFields(map[string]interface{}{
			"error": err.Error(),
			"group": targetGroup.Name,
		}).Debug("Failed to change socket group ownership, but socket is world-readable")
		return nil // Don't fail, socket is already 666
	}

	p.logger.WithFields(map[string]interface{}{
		"socket_path": socketPath,
		"group":       targetGroup.Name,
		"gid":         gid,
		"permissions": "666",
	}).Info("Set socket permissions for group access")

	return nil
}
