package cli

import (
	"context"
	"fmt"
	"log"

	"ssh-docker-proxy/internal/config"
	"ssh-docker-proxy/internal/proxy"
)

// CLI provides command-line interface for the proxy
type CLI struct {
	config *config.Config
	logger *log.Logger
}

// NewCLI creates a new CLI instance
func NewCLI() *CLI {
	return &CLI{
		logger: log.New(log.Writer(), "[ssh-docker-proxy] ", log.LstdFlags),
	}
}

// Execute runs the CLI with the given arguments
func (c *CLI) Execute(ctx context.Context, args []string) error {
	// Check for help flag first
	for _, arg := range args {
		if arg == "-help" || arg == "--help" || arg == "-h" {
			c.showHelp()
			return nil
		}
	}

	// Parse config file flag to get config path
	var configPath string
	for i, arg := range args {
		if arg == "-config" && i+1 < len(args) {
			configPath = args[i+1]
			break
		}
		// Also handle -config=value format
		if len(arg) > 8 && arg[:8] == "-config=" {
			configPath = arg[8:]
			break
		}
	}

	// Find config file if not specified
	if configPath == "" {
		configPath = config.FindConfigFile()
	}

	// Load configuration with precedence: flags > file > defaults
	var err error
	c.config, err = config.LoadConfig(configPath, args)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Validate configuration
	if err := c.config.Validate(); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	// Create and start proxy
	p, err := proxy.NewProxy(c.config, c.logger)
	if err != nil {
		return fmt.Errorf("failed to create proxy: %w", err)
	}

	c.logger.Printf("Starting SSH Docker proxy...")
	c.logger.Printf("Local socket: %s", c.config.LocalSocket)
	c.logger.Printf("SSH target: %s@%s", c.config.SSHUser, c.config.SSHHost)
	c.logger.Printf("Remote socket: %s", c.config.RemoteSocket)

	return p.Start(ctx)
}

func (c *CLI) showHelp() {
	fmt.Println("SSH Docker Proxy - Forward Docker commands over SSH")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  ssh-docker-proxy [options]")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -config string")
	fmt.Println("        Path to configuration file (optional)")
	fmt.Println("  -local-socket string")
	fmt.Println("        Local Unix socket path (required)")
	fmt.Println("  -ssh-user string")
	fmt.Println("        SSH username (required)")
	fmt.Println("  -ssh-host string")
	fmt.Println("        SSH hostname with optional port (required)")
	fmt.Println("  -ssh-key string")
	fmt.Println("        Path to SSH private key file (required)")
	fmt.Println("  -remote-socket string")
	fmt.Println("        Remote Docker socket path (default \"/var/run/docker.sock\")")
	fmt.Println("  -timeout duration")
	fmt.Println("        SSH connection timeout (default 10s)")
	fmt.Println("  -help")
	fmt.Println("        Show help message")
	fmt.Println()
	fmt.Println("Configuration File:")
	fmt.Println("  The proxy looks for configuration files in the following order:")
	fmt.Println("  - ssh-docker-proxy.yaml (current directory)")
	fmt.Println("  - ~/.ssh-docker-proxy.yaml (home directory)")
	fmt.Println("  - ~/.config/ssh-docker-proxy/config.yaml (XDG config directory)")
	fmt.Println()
	fmt.Println("  Command-line flags override configuration file values.")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  # Using command-line flags")
	fmt.Println("  ssh-docker-proxy -ssh-user=ubuntu -ssh-host=192.168.1.100 -ssh-key=~/.ssh/id_rsa -local-socket=/tmp/docker.sock")
	fmt.Println()
	fmt.Println("  # Using configuration file")
	fmt.Println("  ssh-docker-proxy -config=my-config.yaml")
	fmt.Println()
	fmt.Println("  # Mixed: config file with flag overrides")
	fmt.Println("  ssh-docker-proxy -config=my-config.yaml -ssh-host=different-host")
}
