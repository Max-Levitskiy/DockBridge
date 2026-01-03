//go:build !linux || !arm64

package cli

import (
	"github.com/Max-Levitskiy/DockBridge/gui"
	"github.com/spf13/cobra"
)

var guiCmd = &cobra.Command{
	Use:   "gui",
	Short: "Launch the DockBridge graphical user interface",
	Long: `Launch the DockBridge graphical user interface.

This starts the DockBridge GUI as a system tray application, providing
a visual interface to manage your cloud development environment.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create and run the Fyne application
		app := gui.NewFyneApp()
		app.Run()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(guiCmd)
}
