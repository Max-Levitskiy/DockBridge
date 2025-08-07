package portforward

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dockbridge/dockbridge/internal/client/ssh"
	"github.com/dockbridge/dockbridge/pkg/logger"
	"github.com/pkg/errors"
)

// LocalProxyServer defines the interface for local TCP proxy servers
type LocalProxyServer interface {
	Start(ctx context.Context, localPort int, remoteAddr string) error
	Stop() error
	GetStats() *ProxyStats
	IsRunning() bool
}

// ProxyStats contains statistics about the proxy server
type ProxyStats struct {
	LocalPort         int           `json:"local_port"`
	RemoteAddr        string        `json:"remote_addr"`
	ActiveConnections int32         `json:"active_connections"`
	TotalConnections  int64         `json:"total_connections"`
	BytesTransferred  int64         `json:"bytes_transferred"`
	LastActivity      time.Time     `json:"last_activity"`
	Uptime            time.Duration `json:"uptime"`
}

// localProxyServerImpl implements LocalProxyServer
type localProxyServerImpl struct {
	sshClient ssh.Client
	logger    logger.LoggerInterface

	// Configuration
	localPort  int
	remoteAddr string

	// Network components
	listener net.Listener

	// State management
	running   bool
	startTime time.Time
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup

	// Statistics (using atomic operations for thread safety)
	activeConnections int32
	totalConnections  int64
	bytesTransferred  int64
	lastActivity      int64 // Unix timestamp
}

// NewLocalProxyServer creates a new local proxy server
func NewLocalProxyServer(sshClient ssh.Client, logger logger.LoggerInterface) LocalProxyServer {
	return &localProxyServerImpl{
		sshClient: sshClient,
		logger:    logger,
	}
}

// Start starts the proxy server on the specified local port, forwarding to remoteAddr
func (lps *localProxyServerImpl) Start(ctx context.Context, localPort int, remoteAddr string) error {
	lps.mu.Lock()
	defer lps.mu.Unlock()

	if lps.running {
		return fmt.Errorf("proxy server is already running")
	}

	// Validate SSH client connection
	if !lps.sshClient.IsConnected() {
		return fmt.Errorf("SSH client is not connected")
	}

	// Store configuration
	lps.localPort = localPort
	lps.remoteAddr = remoteAddr

	// Create local listener
	listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", localPort))
	if err != nil {
		return errors.Wrapf(err, "failed to listen on port %d", localPort)
	}
	lps.listener = listener

	// Update local port with the actual assigned port (important when using port 0)
	if tcpAddr, ok := listener.Addr().(*net.TCPAddr); ok {
		lps.localPort = tcpAddr.Port
	}

	// Initialize state
	lps.ctx, lps.cancel = context.WithCancel(ctx)
	lps.running = true
	lps.startTime = time.Now()
	atomic.StoreInt64(&lps.lastActivity, time.Now().Unix())

	// Start accepting connections
	lps.wg.Add(1)
	go func() {
		defer lps.wg.Done()
		lps.acceptConnections()
	}()

	lps.logger.WithFields(map[string]interface{}{
		"local_port":  localPort,
		"remote_addr": remoteAddr,
	}).Info("Local proxy server started")

	return nil
}

// Stop stops the proxy server and closes all connections
func (lps *localProxyServerImpl) Stop() error {
	lps.mu.Lock()
	defer lps.mu.Unlock()

	if !lps.running {
		return nil
	}

	// Signal all goroutines to stop
	lps.cancel()

	// Close the listener to stop accepting new connections
	if lps.listener != nil {
		if err := lps.listener.Close(); err != nil {
			lps.logger.WithFields(map[string]interface{}{
				"error": err.Error(),
			}).Error("Error closing listener")
		}
	}

	// Wait for all connection handlers to finish
	lps.wg.Wait()

	lps.running = false

	lps.logger.WithFields(map[string]interface{}{
		"local_port":        lps.localPort,
		"remote_addr":       lps.remoteAddr,
		"total_connections": atomic.LoadInt64(&lps.totalConnections),
		"bytes_transferred": atomic.LoadInt64(&lps.bytesTransferred),
		"uptime":            time.Since(lps.startTime),
	}).Info("Local proxy server stopped")

	return nil
}

// GetStats returns current proxy statistics
func (lps *localProxyServerImpl) GetStats() *ProxyStats {
	lps.mu.RLock()
	defer lps.mu.RUnlock()

	var uptime time.Duration
	if lps.running {
		uptime = time.Since(lps.startTime)
	}

	lastActivityTime := time.Unix(atomic.LoadInt64(&lps.lastActivity), 0)

	return &ProxyStats{
		LocalPort:         lps.localPort,
		RemoteAddr:        lps.remoteAddr,
		ActiveConnections: atomic.LoadInt32(&lps.activeConnections),
		TotalConnections:  atomic.LoadInt64(&lps.totalConnections),
		BytesTransferred:  atomic.LoadInt64(&lps.bytesTransferred),
		LastActivity:      lastActivityTime,
		Uptime:            uptime,
	}
}

// IsRunning returns true if the proxy server is currently running
func (lps *localProxyServerImpl) IsRunning() bool {
	lps.mu.RLock()
	defer lps.mu.RUnlock()
	return lps.running
}

// acceptConnections accepts incoming connections and handles them
func (lps *localProxyServerImpl) acceptConnections() {
	for {
		// Check if we should stop
		select {
		case <-lps.ctx.Done():
			return
		default:
		}

		// Accept a connection
		localConn, err := lps.listener.Accept()
		if err != nil {
			// Check if the listener was closed (expected during shutdown)
			if errors.Is(err, net.ErrClosed) {
				return
			}
			lps.logger.WithFields(map[string]interface{}{
				"error": err.Error(),
			}).Error("Error accepting connection")
			continue
		}

		// Handle the connection in a goroutine
		lps.wg.Add(1)
		go func() {
			defer lps.wg.Done()
			lps.handleConnection(localConn)
		}()
	}
}

// handleConnection handles a single connection by proxying it through SSH
func (lps *localProxyServerImpl) handleConnection(localConn net.Conn) {
	defer localConn.Close()

	// Update statistics
	atomic.AddInt32(&lps.activeConnections, 1)
	atomic.AddInt64(&lps.totalConnections, 1)
	atomic.StoreInt64(&lps.lastActivity, time.Now().Unix())

	defer func() {
		atomic.AddInt32(&lps.activeConnections, -1)
	}()

	lps.logger.WithFields(map[string]interface{}{
		"local_addr":  localConn.LocalAddr().String(),
		"remote_addr": localConn.RemoteAddr().String(),
		"target":      lps.remoteAddr,
	}).Debug("Handling new connection")

	// Create SSH tunnel to remote address
	tunnel, err := lps.sshClient.CreateTunnel(lps.ctx, "localhost:0", lps.remoteAddr)
	if err != nil {
		lps.logger.WithFields(map[string]interface{}{
			"error":       err.Error(),
			"remote_addr": lps.remoteAddr,
		}).Error("Failed to create SSH tunnel")
		return
	}
	defer tunnel.Close()

	// Get the tunnel's local address and connect to it
	tunnelAddr := tunnel.LocalAddr()
	remoteConn, err := net.Dial("tcp", tunnelAddr)
	if err != nil {
		lps.logger.WithFields(map[string]interface{}{
			"error":       err.Error(),
			"tunnel_addr": tunnelAddr,
		}).Error("Failed to connect to SSH tunnel")
		return
	}
	defer remoteConn.Close()

	// Proxy data bidirectionally
	lps.proxyData(localConn, remoteConn)
}

// proxyData copies data bidirectionally between two connections
func (lps *localProxyServerImpl) proxyData(localConn, remoteConn net.Conn) {
	// Use channels to coordinate the two copy operations
	errCh := make(chan error, 2)

	// Copy from local to remote
	go func() {
		bytes, err := io.Copy(remoteConn, localConn)
		atomic.AddInt64(&lps.bytesTransferred, bytes)
		atomic.StoreInt64(&lps.lastActivity, time.Now().Unix())
		errCh <- err
	}()

	// Copy from remote to local
	go func() {
		bytes, err := io.Copy(localConn, remoteConn)
		atomic.AddInt64(&lps.bytesTransferred, bytes)
		atomic.StoreInt64(&lps.lastActivity, time.Now().Unix())
		errCh <- err
	}()

	// Wait for either direction to finish or error
	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, io.EOF) && !isConnectionClosed(err) {
			lps.logger.WithFields(map[string]interface{}{
				"error": err.Error(),
			}).Debug("Connection proxy error")
		}
	case <-lps.ctx.Done():
		// Proxy server is shutting down
	}
}

// isConnectionClosed checks if an error indicates a closed connection
func isConnectionClosed(err error) bool {
	if err == nil {
		return false
	}

	// Check for common connection closed errors
	errStr := err.Error()
	return errStr == "use of closed network connection" ||
		errStr == "connection reset by peer" ||
		errStr == "broken pipe"
}
