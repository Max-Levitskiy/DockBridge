package gui

import (
	"context"
	"fmt"
	"sync"

	"github.com/Max-Levitskiy/DockBridge/client/config"
	"github.com/Max-Levitskiy/DockBridge/client/docker"
	"github.com/Max-Levitskiy/DockBridge/client/hetzner"
	"github.com/Max-Levitskiy/DockBridge/pkg/logger"
	"github.com/Max-Levitskiy/DockBridge/server"
	sharedconfig "github.com/Max-Levitskiy/DockBridge/shared/config"
)

// Controller manages the DockBridge daemon lifecycle from the GUI.
type Controller struct {
	state         *AppState
	daemon        *docker.DockBridgeDaemon
	daemonCancel  context.CancelFunc
	daemonContext context.Context
	serverManager *server.Manager
	hetznerClient hetzner.HetznerClient
	config        *sharedconfig.ClientConfig
	log           logger.LoggerInterface
	mu            sync.Mutex
}

// NewController creates a new Controller instance.
func NewController(state *AppState) *Controller {
	return &Controller{
		state:  state,
		daemon: docker.NewDockBridgeDaemon(),
		log:    logger.NewDefault(),
	}
}

// LoadConfig loads the DockBridge configuration.
func (c *Controller) LoadConfig() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	manager := config.NewManager()
	configPath, _ := config.GetDefaultConfigPath("client")

	if err := manager.Load(configPath); err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	c.config = manager.GetConfig()

	// Validate Hetzner API token
	if c.config.Hetzner.APIToken == "" {
		return fmt.Errorf("hetzner API token is required - please configure it in Settings")
	}

	return nil
}

// StartServer starts the DockBridge daemon.
func (c *Controller) StartServer() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.daemon.IsRunning() {
		return fmt.Errorf("daemon is already running")
	}

	// Immediately update UI state and return - do all work in background
	c.state.SetStatus(StatusProvisioning)
	c.state.SetLastError(nil)

	// Start daemon in background - this goroutine does ALL the work
	go func() {
		// Load configuration
		if c.config == nil {
			c.mu.Lock()
			manager := config.NewManager()
			configPath, _ := config.GetDefaultConfigPath("client")

			if err := manager.Load(configPath); err != nil {
				c.mu.Unlock()
				c.log.WithFields(map[string]any{"error": err.Error()}).Error("Failed to load configuration")
				c.state.SetStatus(StatusError)
				c.state.SetLastError(fmt.Errorf("failed to load configuration: %w", err))
				return
			}

			c.config = manager.GetConfig()

			// Validate Hetzner API token
			if c.config.Hetzner.APIToken == "" {
				c.mu.Unlock()
				err := fmt.Errorf("hetzner API token is required - please configure it in Settings")
				c.state.SetStatus(StatusError)
				c.state.SetLastError(err)
				return
			}
			c.mu.Unlock()
		}

		// Create Hetzner client
		c.mu.Lock()
		cfg := c.config
		c.mu.Unlock()

		hetznerConfig := &hetzner.Config{
			APIToken:        cfg.Hetzner.APIToken,
			ServerType:      cfg.Hetzner.ServerType,
			Location:        cfg.Hetzner.Location,
			VolumeSize:      cfg.Hetzner.VolumeSize,
			PreferredImages: cfg.Hetzner.PreferredImages,
		}

		hetznerClient, err := hetzner.NewClient(hetznerConfig)
		if err != nil {
			c.log.WithFields(map[string]any{"error": err.Error()}).Error("Failed to create Hetzner client")
			c.state.SetStatus(StatusError)
			c.state.SetLastError(fmt.Errorf("failed to create Hetzner client: %w", err))
			return
		}

		c.mu.Lock()
		c.hetznerClient = hetznerClient
		c.serverManager = server.NewManager(c.hetznerClient, &cfg.Hetzner)
		c.daemonContext, c.daemonCancel = context.WithCancel(context.Background())
		ctx := c.daemonContext
		c.mu.Unlock()

		// Ensure volume exists
		volume, err := c.serverManager.EnsureVolume(ctx)
		if err != nil {
			c.log.WithFields(map[string]any{"error": err.Error()}).Error("Failed to ensure volume")
			c.state.SetStatus(StatusError)
			c.state.SetLastError(fmt.Errorf("failed to ensure volume: %w", err))
			return
		}

		c.log.WithFields(map[string]any{
			"volume_name": volume.Name,
			"volume_size": volume.Size,
		}).Info("Docker data volume ready")

		// Create daemon configuration
		daemonConfig := &docker.DaemonConfig{
			SocketPath:     cfg.Docker.SocketPath,
			HetznerClient:  hetznerClient,
			SSHConfig:      &cfg.SSH,
			HetznerConfig:  &cfg.Hetzner,
			ActivityConfig: &cfg.Activity,
			Logger:         c.log,
		}

		// Start daemon
		c.log.Info("Starting DockBridge daemon")
		if err := c.daemon.Start(ctx, daemonConfig); err != nil {
			c.log.WithFields(map[string]any{"error": err.Error()}).Error("Daemon failed to start")
			c.state.SetStatus(StatusError)
			c.state.SetLastError(err)
			return
		}

		// Daemon started successfully
		c.log.Info("DockBridge daemon started successfully")
		c.state.SetStatus(StatusRunning)
		c.state.SetLastError(nil)

		// Fetch and update server IP
		go c.updateServerIP()

		// Wait for context cancellation (Stop was called)
		<-ctx.Done()
		c.log.Info("Daemon context cancelled")
	}()

	return nil
}

// StopServer stops the DockBridge daemon.
func (c *Controller) StopServer() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.daemon.IsRunning() {
		return fmt.Errorf("daemon is not running")
	}

	c.state.SetStatus(StatusStopping)
	c.state.SetLastError(nil)

	// Cancel daemon context
	if c.daemonCancel != nil {
		c.daemonCancel()
	}

	// Stop daemon
	if err := c.daemon.Stop(); err != nil {
		c.state.SetStatus(StatusError)
		c.state.SetLastError(err)
		return fmt.Errorf("failed to stop daemon: %w", err)
	}

	c.state.SetStatus(StatusStopped)
	c.state.SetServerIP("")
	return nil
}

// GetServerStatus retrieves the current server status.
func (c *Controller) GetServerStatus() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.serverManager == nil {
		return fmt.Errorf("server manager not initialized")
	}

	ctx := context.Background()
	if c.hetznerClient == nil {
		return fmt.Errorf("hetzner client not initialized")
	}

	// List servers
	servers, err := c.hetznerClient.ListServers(ctx)
	if err != nil {
		return fmt.Errorf("failed to list servers: %w", err)
	}

	// Find DockBridge server
	for _, srv := range servers {
		if len(srv.Name) >= 10 && srv.Name[:10] == "dockbridge" {
			c.state.SetServerIP(srv.IPAddress)
			c.log.WithFields(map[string]any{
				"server_id": srv.ID,
				"server_ip": srv.IPAddress,
			}).Info("Found DockBridge server")
			break
		}
	}

	return nil
}

// IsRunning returns true if the daemon is currently running.
func (c *Controller) IsRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.daemon.IsRunning()
}
