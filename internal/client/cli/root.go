package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	verbose bool
	rootCmd = &cobra.Command{
		Use:   "dockbridge",
		Short: "DockBridge client for managing remote Docker operations",
		Long: `DockBridge client enables seamless Docker development workflows by 
transparently proxying Docker commands from your local laptop to remote 
Hetzner Cloud servers with intelligent lifecycle management.

The client automatically provisions servers when needed, manages server
lifecycle based on laptop lock status, and maintains persistent volumes
for your Docker data.`,
		Version: "0.1.0",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Setup logging based on verbose flag
			if verbose {
				fmt.Println("Verbose logging enabled")
			}
			return nil
		},
	}
)

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.dockbridge/client.yaml)")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "enable verbose logging")

	// Version flag
	rootCmd.SetVersionTemplate("DockBridge Client v{{.Version}}\n")

	// Bind flags to viper
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath("$HOME/.dockbridge")
		viper.AddConfigPath("./configs")
		viper.SetConfigName("client")
		viper.SetConfigType("yaml")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		if verbose {
			fmt.Println("Using config file:", viper.ConfigFileUsed())
		}
	}
}
