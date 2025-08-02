package proxy

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"

	"ssh-docker-proxy/internal/config"
	"ssh-docker-proxy/internal/ssh"
)

// Proxy represents the main proxy server
type Proxy struct {
	config   *config.Config
	dialer   *ssh.SSHDialer
	listener net.Listener
	logger   *log.Logger
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewProxy creates a new proxy instance
func NewProxy(cfg *config.Config, logger *log.Logger) (*Proxy, error) {
	// Create SSH dialer
	dialer, err := ssh.NewSSHDialer(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH dialer: %w", err)
	}

	return &Proxy{
		config: cfg,
		dialer: dialer,
		logger: logger,
	}, nil
}

// Start begins listening for connections and serving requests
func (p *Proxy) Start(ctx context.Context) error {
	// Store context for graceful shutdown
	p.ctx, p.cancel = context.WithCancel(ctx)

	// Perform health check first
	p.logger.Printf("Performing health check...")
	if err := p.dialer.HealthCheck(p.ctx); err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	p.logger.Printf("Health check passed - remote Docker daemon is accessible")

	// Remove existing socket file if it exists
	if err := os.RemoveAll(p.config.LocalSocket); err != nil {
		return &config.ProxyError{
			Category: config.ErrorCategoryRuntime,
			Message:  fmt.Sprintf("failed to remove existing socket file: %s", p.config.LocalSocket),
			Cause:    err,
		}
	}

	// Create Unix domain socket listener
	listener, err := net.Listen("unix", p.config.LocalSocket)
	if err != nil {
		return &config.ProxyError{
			Category: config.ErrorCategoryRuntime,
			Message:  fmt.Sprintf("failed to create Unix socket listener: %s", p.config.LocalSocket),
			Cause:    err,
		}
	}
	p.listener = listener

	p.logger.Printf("Proxy started successfully, listening on %s", p.config.LocalSocket)

	// Handle graceful shutdown
	go func() {
		<-p.ctx.Done()
		p.logger.Printf("Shutting down proxy...")
		p.Stop()
	}()

	// Accept and handle connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-p.ctx.Done():
				return nil // Graceful shutdown
			default:
				p.logger.Printf("Failed to accept connection: %v", err)
				continue
			}
		}

		// Handle connection in goroutine
		go p.handleConnection(conn)
	}
}

// Stop gracefully shuts down the proxy
func (p *Proxy) Stop() error {
	// Cancel context to signal shutdown
	if p.cancel != nil {
		p.cancel()
	}

	if p.listener != nil {
		p.listener.Close()
	}

	// Clean up socket file
	if err := os.RemoveAll(p.config.LocalSocket); err != nil {
		p.logger.Printf("Warning: failed to remove socket file: %v", err)
	}

	return nil
}

// handleConnection processes a single client connection with proper lifecycle management
func (p *Proxy) handleConnection(localConn net.Conn) {
	// Generate connection ID for logging
	connID := fmt.Sprintf("%p", localConn)

	defer func() {
		localConn.Close()
		p.logger.Printf("[%s] Connection cleanup completed", connID)
	}()

	p.logger.Printf("[%s] New connection established from client %s", connID, localConn.RemoteAddr())

	// Create fresh SSH connection for this client (per-connection SSH stream for isolation)
	remoteConn, err := p.dialer.Dial()
	if err != nil {
		p.logger.Printf("[%s] Failed to establish SSH connection: %v", connID, err)
		return
	}
	defer func() {
		remoteConn.Close()
		p.logger.Printf("[%s] SSH connection closed", connID)
	}()

	p.logger.Printf("[%s] SSH connection established to remote Docker daemon", connID)

	// Relay traffic bidirectionally using pure byte copying
	relayTraffic(localConn, remoteConn, p.logger)

	p.logger.Printf("[%s] Connection terminated", connID)
}

// relayTraffic performs bidirectional byte copying between connections
func relayTraffic(local, remote net.Conn, logger *log.Logger) {
	done := make(chan struct{}, 2)
	connID := fmt.Sprintf("%p", local)

	// Copy from local to remote
	go func() {
		defer func() { done <- struct{}{} }()
		bytes, err := io.Copy(remote, local)
		if err != nil && err != io.EOF {
			logger.Printf("[%s] Local->Remote copy ended with error after %d bytes: %v", connID, bytes, err)
		} else {
			logger.Printf("[%s] Local->Remote copy completed, %d bytes transferred", connID, bytes)
		}
	}()

	// Copy from remote to local
	go func() {
		defer func() { done <- struct{}{} }()
		bytes, err := io.Copy(local, remote)
		if err != nil && err != io.EOF {
			logger.Printf("[%s] Remote->Local copy ended with error after %d bytes: %v", connID, bytes, err)
		} else {
			logger.Printf("[%s] Remote->Local copy completed, %d bytes transferred", connID, bytes)
		}
	}()

	// Wait for either direction to complete
	<-done
	logger.Printf("[%s] Traffic relay completed", connID)
}
