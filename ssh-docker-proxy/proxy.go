// Package ssh-docker-proxy provides a library for creating SSH-based Docker proxies
package ssh_docker_proxy

import (
	"context"
	"fmt"
	"log"
	"time"

	"ssh-docker-proxy/internal/config"
	"ssh-docker-proxy/internal/proxy"
)

// ProxyConfig represents the configuration for the SSH Docker proxy
type ProxyConfig struct {
	LocalSocket  string // Local Unix socket path (e.g., /tmp/docker.sock)
	SSHUser      string // SSH username
	SSHHost      string // SSH hostname with optional port
	SSHKeyPath   string // Path to SSH private key file
	RemoteSocket string // Remote Docker socket path (default: /var/run/docker.sock)
	Timeout      string // SSH connection timeout (e.g., "10s")
}

// Proxy represents a running SSH Docker proxy instance
type Proxy struct {
	proxy  *proxy.Proxy
	logger *log.Logger
}

// Logger interface allows custom logging implementations
type Logger interface {
	Printf(format string, v ...interface{})
}

// NewProxy creates a new SSH Docker proxy instance
func NewProxy(cfg *ProxyConfig, logger Logger) (*Proxy, error) {
	// Convert public config to internal config
	internalConfig := &config.Config{
		LocalSocket:  cfg.LocalSocket,
		SSHUser:      cfg.SSHUser,
		SSHHost:      cfg.SSHHost,
		SSHKeyPath:   cfg.SSHKeyPath,
		RemoteSocket: cfg.RemoteSocket,
	}

	// Set default remote socket if not specified
	if internalConfig.RemoteSocket == "" {
		internalConfig.RemoteSocket = "/var/run/docker.sock"
	}

	// Parse timeout if provided
	if cfg.Timeout != "" {
		timeout, err := time.ParseDuration(cfg.Timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout format: %s", err)
		}
		internalConfig.Timeout = timeout
	}

	// Validate configuration
	if err := internalConfig.Validate(); err != nil {
		return nil, err
	}

	// Create logger wrapper
	var internalLogger *log.Logger
	if logger != nil {
		internalLogger = log.New(&loggerWriter{logger: logger}, "", 0)
	} else {
		internalLogger = log.New(log.Writer(), "[ssh-docker-proxy] ", log.LstdFlags)
	}

	// Create internal proxy
	internalProxy, err := proxy.NewProxy(internalConfig, internalLogger)
	if err != nil {
		return nil, err
	}

	return &Proxy{
		proxy:  internalProxy,
		logger: internalLogger,
	}, nil
}

// Start begins listening for connections and serving requests
func (p *Proxy) Start(ctx context.Context) error {
	return p.proxy.Start(ctx)
}

// Stop gracefully shuts down the proxy
func (p *Proxy) Stop() error {
	return p.proxy.Stop()
}

// loggerWriter wraps a Logger interface to implement io.Writer
type loggerWriter struct {
	logger Logger
}

func (w *loggerWriter) Write(p []byte) (n int, err error) {
	w.logger.Printf("%s", string(p))
	return len(p), nil
}
