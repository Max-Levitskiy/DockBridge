package proxy

import (
	"context"
	"log"
	"time"

	"ssh-docker-proxy/internal/config"
	"ssh-docker-proxy/internal/proxy"
)

// Logger interface for custom logging implementations
type Logger interface {
	Printf(format string, v ...interface{})
}

// ProxyConfig represents the public configuration for the proxy
type ProxyConfig struct {
	LocalSocket  string
	SSHUser      string
	SSHHost      string
	SSHKeyPath   string
	RemoteSocket string
	Timeout      int // timeout in seconds
}

// Proxy represents the public proxy interface
type Proxy struct {
	internal *proxy.Proxy
}

// NewProxy creates a new proxy instance with the given configuration
func NewProxy(cfg *ProxyConfig, logger Logger) (*Proxy, error) {
	// Convert public config to internal config
	internalConfig := &config.Config{
		LocalSocket:  cfg.LocalSocket,
		SSHUser:      cfg.SSHUser,
		SSHHost:      cfg.SSHHost,
		SSHKeyPath:   cfg.SSHKeyPath,
		RemoteSocket: cfg.RemoteSocket,
	}

	// Set default timeout if not specified
	if cfg.Timeout > 0 {
		internalConfig.Timeout = time.Duration(cfg.Timeout) * time.Second
	}

	// Validate configuration
	if err := internalConfig.Validate(); err != nil {
		return nil, err
	}

	// Convert logger to standard logger if needed
	var stdLogger *log.Logger
	if logger != nil {
		stdLogger = log.New(&loggerWriter{logger}, "", 0)
	} else {
		stdLogger = log.New(log.Writer(), "[ssh-docker-proxy] ", log.LstdFlags)
	}

	// Create internal proxy
	internalProxy, err := proxy.NewProxy(internalConfig, stdLogger)
	if err != nil {
		return nil, err
	}

	return &Proxy{
		internal: internalProxy,
	}, nil
}

// Start begins listening for connections and serving requests
func (p *Proxy) Start(ctx context.Context) error {
	return p.internal.Start(ctx)
}

// Stop gracefully shuts down the proxy
func (p *Proxy) Stop() error {
	return p.internal.Stop()
}

// loggerWriter adapts the Logger interface to io.Writer
type loggerWriter struct {
	logger Logger
}

func (w *loggerWriter) Write(p []byte) (n int, err error) {
	w.logger.Printf("%s", string(p))
	return len(p), nil
}
