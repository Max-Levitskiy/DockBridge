// DockBridge Server - Runs on Hetzner Cloud servers to handle keep-alive monitoring
// and self-destruction when the client becomes unavailable.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dockbridge/dockbridge/pkg/logger"
	"github.com/dockbridge/dockbridge/server/keepalive"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile  string
	verbose  bool
	serverID string
)

var rootCmd = &cobra.Command{
	Use:   "dockbridge-server",
	Short: "DockBridge server daemon for keep-alive monitoring",
	Long: `DockBridge server runs on Hetzner Cloud instances and monitors for
keep-alive heartbeats from the client. If no heartbeat is received within
the configured timeout, the server self-destructs to avoid ongoing costs.

The server exposes HTTP endpoints for:
  - /heartbeat (POST/PUT) - Record a heartbeat from the client
  - /status (GET) - Get current monitor status
  - /health (GET) - Simple health check
`,
	Run: runServer,
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "enable verbose logging")
	rootCmd.PersistentFlags().StringVar(&serverID, "server-id", "", "Hetzner server ID for self-destruction")

	// Server flags
	rootCmd.Flags().Int("port", 8080, "HTTP port for keep-alive server")
	rootCmd.Flags().Duration("timeout", 5*time.Minute, "timeout before self-destruction")
	rootCmd.Flags().Duration("grace-period", 30*time.Second, "grace period before destruction")

	// Bind flags to viper
	viper.BindPFlag("port", rootCmd.Flags().Lookup("port"))
	viper.BindPFlag("timeout", rootCmd.Flags().Lookup("timeout"))
	viper.BindPFlag("grace_period", rootCmd.Flags().Lookup("grace-period"))
	viper.BindPFlag("server_id", rootCmd.PersistentFlags().Lookup("server-id"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath("/etc/dockbridge")
		viper.AddConfigPath("$HOME/.dockbridge")
		viper.AddConfigPath("./configs")
		viper.SetConfigName("server")
		viper.SetConfigType("yaml")
	}

	// Read environment variables
	viper.AutomaticEnv()
	viper.SetEnvPrefix("DOCKBRIDGE")

	// Try to read config file
	if err := viper.ReadInConfig(); err == nil {
		if verbose {
			fmt.Println("Using config file:", viper.ConfigFileUsed())
		}
	}

	// Get server ID from environment if not provided
	if serverID == "" {
		serverID = os.Getenv("HETZNER_SERVER_ID")
	}
}

func runServer(cmd *cobra.Command, args []string) {
	// Setup logger
	log := logger.NewDefault()
	if verbose {
		log.Info("Verbose logging enabled")
	}

	log.Info("Starting DockBridge server",
		"server_id", serverID,
		"port", viper.GetInt("port"),
		"timeout", viper.GetDuration("timeout"),
	)

	// Create keep-alive config
	config := &keepalive.Config{
		Port:            viper.GetInt("port"),
		Timeout:         viper.GetDuration("timeout"),
		GracePeriod:     viper.GetDuration("grace_period"),
		ServerID:        serverID,
		HetznerAPIToken: os.Getenv("HETZNER_API_TOKEN"),
	}

	if config.ServerID == "" {
		log.Warn("Server ID not configured - self-destruction via API will not work")
	}
	if config.HetznerAPIToken == "" {
		log.Warn("Hetzner API token not configured - self-destruction via API will not work")
	}

	// Create and start monitor
	monitor := keepalive.NewMonitor(config, log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := monitor.Start(ctx); err != nil {
		log.Error("Failed to start keep-alive monitor", "error", err)
		os.Exit(1)
	}

	log.Info("DockBridge server running",
		"port", config.Port,
		"timeout", config.Timeout,
		"grace_period", config.GracePeriod,
	)

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Info("Received shutdown signal", "signal", sig)
	case <-ctx.Done():
		log.Info("Context cancelled")
	}

	// Graceful shutdown
	log.Info("Shutting down DockBridge server...")
	if err := monitor.Stop(); err != nil {
		log.Error("Error during shutdown", "error", err)
	}

	log.Info("DockBridge server stopped")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
