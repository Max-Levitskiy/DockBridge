package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
)

// FyneApp represents the main DockBridge GUI application.
type FyneApp struct {
	app            fyne.App
	state          *AppState
	controller     *Controller
	tray           *Tray
	mainWindow     *MainWindow
	settingsWindow *SettingsWindow
}

// NewFyneApp creates a new FyneApp instance.
func NewFyneApp() *FyneApp {
	fyneApp := app.NewWithID("com.dockbridge.client")
	fyneApp.SetIcon(ResourceIcon)
	state := NewAppState()
	controller := NewController(state)

	return &FyneApp{
		app:        fyneApp,
		state:      state,
		controller: controller,
	}
}

// Run starts the Fyne application.
func (fa *FyneApp) Run() {
	// Setup system tray
	fa.tray = NewTray(fa.app, fa.controller, fa.state)
	fa.tray.Setup()

	// Create windows (but don't show them yet - tray-first approach)
	fa.mainWindow = NewMainWindow(fa.app, fa.controller, fa.state)
	fa.settingsWindow = NewSettingsWindow(fa.app, fa.controller, fa.state)

	// Wire up tray menu actions
	fa.tray.SetMainWindowCallback(fa.mainWindow.Show)
	fa.tray.SetSettingsWindowCallback(fa.settingsWindow.Show)

	// Auto-start the daemon (like CLI does)
	go func() {
		if err := fa.controller.StartServer(); err != nil {
			fa.state.SetStatus(StatusError)
			fa.state.SetLastError(err)
		}
	}()

	// Start the application (this blocks until quit)
	fa.app.Run()
}
