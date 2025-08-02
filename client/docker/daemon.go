package docker

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"sync"

	"github.com/dockbridge/dockbridge/client/hetzner"
	"github.com/dockbridge/dockbridge/client/ssh"
	"github.com/dockbridge/dockbridge/pkg/logger"
	"github.com/dockbridge/dockbridge/shared/config"
	"github.com/pkg/errors"
)

// DockBridgeDaemon represents the main DockBridge daemon that forwards Docker socket over SSH
type DockBridgeDaemon struct {
	config        *DaemonConfig
	listener      net.Listener
	running       bool
	mu            sync.RWMutex
	logger        logger.LoggerInterface
	clientManager DockerClientManager
	ctx           context.Context
	cancel        context.CancelFunc
}

// DaemonConfig holds configuration for the DockBridge daemon
type DaemonConfig struct {
	SocketPath    string
	HetznerClient hetzner.HetznerClient
	SSHConfig     *config.SSHConfig
	HetznerConfig *config.HetznerConfig
	Logger        logger.LoggerInterface
}

// NewDockBridgeDaemon creates a new DockBridge daemon
func NewDockBridgeDaemon() *DockBridgeDaemon {
	return &DockBridgeDaemon{}
}

// Start starts the DockBridge daemon with direct socket forwarding
func (d *DockBridgeDaemon) Start(ctx context.Context, config *DaemonConfig) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.running {
		return errors.New("daemon is already running")
	}

	if config == nil {
		return errors.New("daemon config cannot be nil")
	}

	d.config = config
	d.logger = config.Logger

	// Store context for graceful shutdown
	d.ctx, d.cancel = context.WithCancel(ctx)

	// Initialize components
	if err := d.initializeComponents(); err != nil {
		return errors.Wrap(err, "failed to initialize components")
	}

	// Set up the Unix socket listener
	if err := d.setupListener(); err != nil {
		return errors.Wrap(err, "failed to setup listener")
	}

	// Start accepting connections in a goroutine
	go func() {
		d.logger.WithFields(map[string]interface{}{
			"socket_path": d.config.SocketPath,
		}).Info("Starting DockBridge daemon with direct socket forwarding")

		d.acceptConnections()
	}()

	d.running = true
	d.logger.Info("DockBridge daemon started successfully")
	return nil
}

// Stop gracefully shuts down the DockBridge daemon
func (d *DockBridgeDaemon) Stop() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.running {
		return nil
	}

	d.logger.Info("Stopping DockBridge daemon")

	// Cancel context to signal shutdown
	if d.cancel != nil {
		d.cancel()
	}

	// Close listener
	if d.listener != nil {
		d.listener.Close()
	}

	// Close client manager
	if d.clientManager != nil {
		if err := d.clientManager.Close(); err != nil {
			d.logger.WithFields(map[string]interface{}{
				"error": err.Error(),
			}).Error("Failed to close client manager")
		}
	}

	// Clean up socket file
	if err := os.RemoveAll(d.config.SocketPath); err != nil {
		d.logger.WithFields(map[string]interface{}{
			"error":       err.Error(),
			"socket_path": d.config.SocketPath,
		}).Warn("Failed to remove socket file")
	}

	d.running = false
	d.logger.Info("DockBridge daemon stopped successfully")
	return nil
}

// IsRunning returns true if the daemon is currently running
func (d *DockBridgeDaemon) IsRunning() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.running
}

// initializeComponents sets up the internal components
func (d *DockBridgeDaemon) initializeComponents() error {
	// Create Docker client manager
	d.clientManager = NewDockerClientManager(
		d.config.HetznerClient,
		d.config.SSHConfig,
		d.config.HetznerConfig,
		d.logger,
	)
	return nil
}

// setupListener configures the Unix socket listener
func (d *DockBridgeDaemon) setupListener() error {
	// Remove existing socket file if it exists
	if err := os.RemoveAll(d.config.SocketPath); err != nil {
		return errors.Wrap(err, "failed to remove existing socket file")
	}

	// Create listener for Unix socket
	var err error
	if strings.HasPrefix(d.config.SocketPath, "/") {
		// Unix socket
		d.listener, err = net.Listen("unix", d.config.SocketPath)
		if err != nil {
			return errors.Wrap(err, "failed to create listener")
		}

		// Set proper permissions for the socket
		if err := d.setSocketPermissions(d.config.SocketPath); err != nil {
			d.logger.WithFields(map[string]interface{}{
				"socket_path": d.config.SocketPath,
				"error":       err.Error(),
			}).Warn("Failed to set socket permissions")
		}
	} else {
		return errors.New("only Unix socket paths are supported for daemon mode")
	}

	return nil
}

// acceptConnections accepts and handles incoming connections
func (d *DockBridgeDaemon) acceptConnections() {
	for {
		conn, err := d.listener.Accept()
		if err != nil {
			select {
			case <-d.ctx.Done():
				return // Graceful shutdown
			default:
				d.logger.WithFields(map[string]interface{}{
					"error": err.Error(),
				}).Error("Failed to accept connection")
				continue
			}
		}

		// Handle connection in goroutine
		go d.handleConnection(conn)
	}
}

// handleConnection processes a single client connection with direct socket forwarding
func (d *DockBridgeDaemon) handleConnection(localConn net.Conn) {
	// Generate connection ID for logging
	connID := fmt.Sprintf("%p", localConn)

	defer func() {
		localConn.Close()
		d.logger.WithFields(map[string]interface{}{
			"conn_id": connID,
		}).Debug("Connection cleanup completed")
	}()

	d.logger.WithFields(map[string]interface{}{
		"conn_id": connID,
		"remote":  localConn.RemoteAddr(),
	}).Info("🐳 New Docker connection established")

	// Ensure we have a connection to remote server
	if err := d.clientManager.EnsureConnection(d.ctx); err != nil {
		d.logger.WithFields(map[string]interface{}{
			"conn_id": connID,
			"error":   err.Error(),
		}).Error("Failed to ensure connection to remote server")
		return
	}

	// Get SSH tunnel from client manager
	tunnel, err := d.getTunnelFromClientManager()
	if err != nil {
		d.logger.WithFields(map[string]interface{}{
			"conn_id": connID,
			"error":   err.Error(),
		}).Error("Failed to get SSH tunnel")
		return
	}

	// Create connection to remote Docker daemon via SSH tunnel
	remoteConn, err := net.Dial("tcp", tunnel.LocalAddr())
	if err != nil {
		d.logger.WithFields(map[string]interface{}{
			"conn_id":     connID,
			"tunnel_addr": tunnel.LocalAddr(),
			"error":       err.Error(),
		}).Error("Failed to connect to remote Docker daemon via SSH tunnel")
		return
	}
	defer func() {
		remoteConn.Close()
		d.logger.WithFields(map[string]interface{}{
			"conn_id": connID,
		}).Debug("Remote connection closed")
	}()

	d.logger.WithFields(map[string]interface{}{
		"conn_id":     connID,
		"tunnel_addr": tunnel.LocalAddr(),
	}).Info("Connected to remote Docker daemon via SSH tunnel")

	// Relay traffic bidirectionally using pure byte copying
	d.relayTraffic(localConn, remoteConn, connID)

	d.logger.WithFields(map[string]interface{}{
		"conn_id": connID,
	}).Info("Docker connection terminated")
}

// relayTraffic performs bidirectional byte copying between connections
func (d *DockBridgeDaemon) relayTraffic(local, remote net.Conn, connID string) {
	done := make(chan struct{}, 2)

	// Copy from local to remote
	go func() {
		defer func() { done <- struct{}{} }()
		bytes, err := io.Copy(remote, local)
		if err != nil && err != io.EOF {
			d.logger.WithFields(map[string]interface{}{
				"conn_id": connID,
				"bytes":   bytes,
				"error":   err.Error(),
			}).Debug("Local->Remote copy ended with error")
		} else {
			d.logger.WithFields(map[string]interface{}{
				"conn_id": connID,
				"bytes":   bytes,
			}).Debug("Local->Remote copy completed")
		}
	}()

	// Copy from remote to local
	go func() {
		defer func() { done <- struct{}{} }()
		bytes, err := io.Copy(local, remote)
		if err != nil && err != io.EOF {
			d.logger.WithFields(map[string]interface{}{
				"conn_id": connID,
				"bytes":   bytes,
				"error":   err.Error(),
			}).Debug("Remote->Local copy ended with error")
		} else {
			d.logger.WithFields(map[string]interface{}{
				"conn_id": connID,
				"bytes":   bytes,
			}).Debug("Remote->Local copy completed")
		}
	}()

	// Wait for either direction to complete
	<-done
	d.logger.WithFields(map[string]interface{}{
		"conn_id": connID,
	}).Debug("Traffic relay completed")
}

// getTunnelFromClientManager extracts the SSH tunnel from the client manager
func (d *DockBridgeDaemon) getTunnelFromClientManager() (TunnelInterface, error) {
	tunnel := d.clientManager.GetTunnel()
	if tunnel == nil {
		return nil, errors.New("no SSH tunnel available")
	}
	return tunnel, nil
}

// Use the SSH tunnel interface from the ssh package
type TunnelInterface = ssh.TunnelInterface

// setSocketPermissions sets the correct permissions for the Docker socket
func (d *DockBridgeDaemon) setSocketPermissions(socketPath string) error {
	// Set socket permissions to 666 (rw-rw-rw-)
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
		cmd := exec.Command("groupadd", "-f", "docker")
		if err := cmd.Run(); err != nil {
			return nil // Don't fail, socket is already 666
		}

		targetGroup, err = user.LookupGroup("docker")
		if err != nil {
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
		return nil // Don't fail, socket is already 666
	}

	d.logger.WithFields(map[string]interface{}{
		"socket_path": socketPath,
		"group":       targetGroup.Name,
		"gid":         gid,
		"permissions": "666",
	}).Info("Set socket permissions for group access")

	return nil
}
