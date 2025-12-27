package cli

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	rootCmd = &cobra.Command{
		Use:   "dockbridge-server",
		Short: "DockBridge server for handling remote Docker operations",
		Long: `DockBridge server runs on Hetzner Cloud instances to process Docker 
commands forwarded from the client, manage keep-alive monitoring, and handle 
graceful server lifecycle management.`,
	}
)

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is /etc/dockbridge/server.yaml)")
	rootCmd.PersistentFlags().Bool("verbose", false, "enable verbose logging")

	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath("/etc/dockbridge")
		viper.AddConfigPath("./configs")
		viper.SetConfigName("server")
		viper.SetConfigType("yaml")
	}

	viper.AutomaticEnv()

	_ = viper.ReadInConfig() // Ignore errors if config file is not found
}
