package ssh

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
)

// sshDialer is an interface for the Dial method of an SSH client
type sshDialer interface {
	Dial(network, addr string) (net.Conn, error)
}

// TunnelInterface defines the interface for SSH tunnels
type TunnelInterface interface {
	Start(ctx context.Context) error
	Close() error
	LocalAddr() string
	RemoteAddr() string
	IsActive() bool
}

// Tunnel represents an SSH tunnel from a local address to a remote address
type Tunnel struct {
	sshClient  *ssh.Client
	localAddr  string
	remoteAddr string
	listener   net.Listener
	active     bool
	mu         sync.Mutex
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	dialer     sshDialer // Interface for dialing, used for testing
}

// NewTunnel creates a new SSH tunnel
func NewTunnel(sshClient *ssh.Client, localAddr, remoteAddr string) *Tunnel {
	ctx, cancel := context.WithCancel(context.Background())
	return &Tunnel{
		sshClient:  sshClient,
		localAddr:  localAddr,
		remoteAddr: remoteAddr,
		ctx:        ctx,
		cancel:     cancel,
		dialer:     sshClient, // SSH client implements the Dial method
	}
}

// Start begins listening on the local address and forwarding connections to the remote address
func (t *Tunnel) Start(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.active {
		return nil
	}

	// Start local listener
	listener, err := net.Listen("tcp", t.localAddr)
	if err != nil {
		return errors.Wrapf(err, "failed to listen on %s", t.localAddr)
	}
	t.listener = listener
	t.active = true

	// Start accepting connections in a goroutine
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		t.acceptConnections()
	}()

	return nil
}

// acceptConnections accepts connections from the local listener and forwards them to the remote address
func (t *Tunnel) acceptConnections() {
	for {
		// Check if the tunnel has been closed
		select {
		case <-t.ctx.Done():
			return
		default:
			// Continue accepting connections
		}

		// Accept a connection
		localConn, err := t.listener.Accept()
		if err != nil {
			// Check if the listener was closed
			if errors.Is(err, net.ErrClosed) {
				return
			}
			fmt.Printf("Error accepting connection: %v\n", err)
			continue
		}

		// Handle the connection in a goroutine
		t.wg.Add(1)
		go func() {
			defer t.wg.Done()
			defer localConn.Close()
			t.handleConnection(localConn)
		}()
	}
}

// handleConnection forwards a single connection from local to remote
func (t *Tunnel) handleConnection(localConn net.Conn) {
	// Open a connection to the remote address via the SSH client or dialer
	remoteConn, err := t.dialer.Dial("tcp", t.remoteAddr)
	if err != nil {
		fmt.Printf("Error dialing remote address %s: %v\n", t.remoteAddr, err)
		return
	}
	defer remoteConn.Close()

	// Copy data in both directions
	errCh := make(chan error, 2)
	go func() {
		_, err := io.Copy(remoteConn, localConn)
		errCh <- err
	}()
	go func() {
		_, err := io.Copy(localConn, remoteConn)
		errCh <- err
	}()

	// Wait for either direction to finish or for the tunnel to be closed
	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, io.EOF) {
			fmt.Printf("Error in tunnel connection: %v\n", err)
		}
	case <-t.ctx.Done():
		// Tunnel is being closed
	}
}

// Close stops the tunnel and closes all connections
func (t *Tunnel) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.active {
		return nil
	}

	// Signal all goroutines to stop
	t.cancel()

	// Close the listener to stop accepting new connections
	if t.listener != nil {
		if err := t.listener.Close(); err != nil {
			return errors.Wrap(err, "failed to close listener")
		}
	}

	// Wait for all goroutines to finish
	t.wg.Wait()

	t.active = false
	return nil
}

// IsActive returns true if the tunnel is active
func (t *Tunnel) IsActive() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.active
}

// LocalAddr returns the local address of the tunnel
func (t *Tunnel) LocalAddr() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.listener != nil {
		return t.listener.Addr().String()
	}
	return t.localAddr
}

// RemoteAddr returns the remote address of the tunnel
func (t *Tunnel) RemoteAddr() string {
	return t.remoteAddr
}

// NewTunnelWithDialer creates a new tunnel with a custom dialer for testing
// This is primarily used for testing purposes
func NewTunnelWithDialer(dialer sshDialer, localAddr, remoteAddr string) *Tunnel {
	ctx, cancel := context.WithCancel(context.Background())
	return &Tunnel{
		localAddr:  localAddr,
		remoteAddr: remoteAddr,
		ctx:        ctx,
		cancel:     cancel,
		dialer:     dialer,
	}
}

// Ensure Tunnel implements TunnelInterface
var _ TunnelInterface = (*Tunnel)(nil)
