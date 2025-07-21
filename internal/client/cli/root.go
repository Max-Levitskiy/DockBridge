package cli

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	rootCmd = &cobra.Command{
		Use:   "dockbridge-client",
		Short: "DockBridge client for managing remote Docker operations",
		Long: `DockBridge client enables seamless Docker development workflows by 
transparently proxying Docker commands from your local laptop to remote 
Hetzner Cloud servers with intelligent lifecycle management.`,
	}
)

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.dockbridge/client.yaml)")
	rootCmd.PersistentFlags().Bool("verbose", false, "enable verbose logging")

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
		// Config file found and successfully parsed
	}
}
