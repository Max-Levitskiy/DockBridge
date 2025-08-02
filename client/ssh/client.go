package ssh

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
)

// Client defines the interface for SSH operations
type Client interface {
	// Connect establishes an SSH connection to the remote server
	Connect(ctx context.Context) error

	// Close terminates the SSH connection
	Close() error

	// CreateTunnel creates an SSH tunnel from local to remote
	CreateTunnel(ctx context.Context, localAddr, remoteAddr string) (TunnelInterface, error)

	// ExecuteCommand runs a command on the remote server
	ExecuteCommand(ctx context.Context, command string) ([]byte, error)

	// IsConnected returns true if the client has an active connection
	IsConnected() bool
}

// ClientConfig holds the configuration for an SSH client
type ClientConfig struct {
	Host           string
	Port           int
	User           string
	PrivateKeyPath string
	Timeout        time.Duration
}

// DefaultClientConfig returns a default SSH client configuration
func DefaultClientConfig() *ClientConfig {
	return &ClientConfig{
		Port:    22,
		User:    "root",
		Timeout: 30 * time.Second,
	}
}

// clientImpl implements the Client interface
type clientImpl struct {
	config    *ClientConfig
	sshClient *ssh.Client
	connected bool
	tunnels   []*Tunnel
}

// NewClient creates a new SSH client with the given configuration
func NewClient(config *ClientConfig) Client {
	return &clientImpl{
		config:  config,
		tunnels: make([]*Tunnel, 0),
	}
}

// Connect establishes an SSH connection to the remote server
func (c *clientImpl) Connect(ctx context.Context) error {
	if c.connected && c.sshClient != nil {
		return nil
	}

	// Read private key
	key, err := os.ReadFile(c.config.PrivateKeyPath)
	if err != nil {
		return errors.Wrap(err, "failed to read private key")
	}

	// Parse private key
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return errors.Wrap(err, "failed to parse private key")
	}

	// Create SSH client config
	config := &ssh.ClientConfig{
		User: c.config.User,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Note: In production, use ssh.FixedHostKey() or ssh.KnownHosts()
		Timeout:         c.config.Timeout,
	}

	// Connect to SSH server
	addr := fmt.Sprintf("%s:%d", c.config.Host, c.config.Port)

	// Create a context with timeout for the connection
	connectCtx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	// Use a channel to handle the connection with timeout
	type connectResult struct {
		client *ssh.Client
		err    error
	}

	ch := make(chan connectResult, 1)
	go func() {
		client, err := ssh.Dial("tcp", addr, config)
		ch <- connectResult{client, err}
	}()

	// Wait for connection or timeout
	select {
	case <-connectCtx.Done():
		return errors.New("connection timeout")
	case res := <-ch:
		if res.err != nil {
			return errors.Wrap(res.err, "failed to connect to SSH server")
		}
		c.sshClient = res.client
	}

	c.connected = true
	return nil
}

// Close terminates the SSH connection and all tunnels
func (c *clientImpl) Close() error {
	if !c.connected || c.sshClient == nil {
		return nil
	}

	// Close all tunnels
	for _, tunnel := range c.tunnels {
		if err := tunnel.Close(); err != nil {
			// Log error but continue closing other resources
			fmt.Printf("Error closing tunnel: %v\n", err)
		}
	}
	c.tunnels = make([]*Tunnel, 0)

	// Close SSH client
	if err := c.sshClient.Close(); err != nil {
		return errors.Wrap(err, "failed to close SSH client")
	}

	c.connected = false
	c.sshClient = nil
	return nil
}

// CreateTunnel creates an SSH tunnel from local to remote
func (c *clientImpl) CreateTunnel(ctx context.Context, localAddr, remoteAddr string) (TunnelInterface, error) {
	if !c.connected || c.sshClient == nil {
		return nil, errors.New("not connected to SSH server")
	}

	tunnel := NewTunnel(c.sshClient, localAddr, remoteAddr)
	if err := tunnel.Start(ctx); err != nil {
		return nil, err
	}

	c.tunnels = append(c.tunnels, tunnel)
	return tunnel, nil
}

// ExecuteCommand runs a command on the remote server
func (c *clientImpl) ExecuteCommand(ctx context.Context, command string) ([]byte, error) {
	if !c.connected || c.sshClient == nil {
		return nil, errors.New("not connected to SSH server")
	}

	// Create a session
	session, err := c.sshClient.NewSession()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create SSH session")
	}
	defer session.Close()

	// Execute the command with timeout
	type cmdResult struct {
		output []byte
		err    error
	}

	ch := make(chan cmdResult, 1)
	go func() {
		output, err := session.CombinedOutput(command)
		ch <- cmdResult{output, err}
	}()

	// Wait for command completion or timeout
	select {
	case <-ctx.Done():
		return nil, errors.New("command execution timeout")
	case res := <-ch:
		if res.err != nil {
			return res.output, errors.Wrap(res.err, "command execution failed")
		}
		return res.output, nil
	}
}

// IsConnected returns true if the client has an active connection
func (c *clientImpl) IsConnected() bool {
	return c.connected && c.sshClient != nil
}
