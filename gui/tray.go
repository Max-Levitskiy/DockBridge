package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

// Tray manages the system tray icon and menu for DockBridge.
type Tray struct {
	app                    fyne.App
	controller             *Controller
	state                  *AppState
	menu                   *fyne.Menu
	mainWindowCallback     func()
	settingsWindowCallback func()
}

// NewTray creates a new system tray instance.
func NewTray(app fyne.App, controller *Controller, state *AppState) *Tray {
	return &Tray{
		app:        app,
		controller: controller,
		state:      state,
	}
}

// Setup initializes the system tray with menu items.
func (t *Tray) Setup() {
	// Register a callback to update the tray when status changes
	t.state.RegisterStatusCallback(t.updateTrayStatus)

	// Build the menu
	t.menu = t.buildMenu()

	// Set the system tray menu
	if desk, ok := t.app.(desktop.App); ok {
		desk.SetSystemTrayMenu(t.menu)
	}
}

// buildMenu creates the tray menu structure.
func (t *Tray) buildMenu() *fyne.Menu {
	return fyne.NewMenu("DockBridge",
		fyne.NewMenuItem("Status: "+t.state.GetStatus().String(), nil),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Show Dashboard", func() {
			if t.mainWindowCallback != nil {
				t.mainWindowCallback()
			}
		}),
		fyne.NewMenuItem("Settings", func() {
			if t.settingsWindowCallback != nil {
				t.settingsWindowCallback()
			}
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Quit", func() {
			t.app.Quit()
		}),
	)
}

// updateTrayStatus refreshes the menu when the status changes.
func (t *Tray) updateTrayStatus(status ServerStatus) {
	// All UI updates must run on the main Fyne thread

	fyne.Do(func() {
		t.menu = t.buildMenu()

		// Update the system tray
		if desk, ok := t.app.(desktop.App); ok {
			desk.SetSystemTrayMenu(t.menu)
		}
	})
}

// SetMainWindowCallback sets the callback for opening the main window.
func (t *Tray) SetMainWindowCallback(callback func()) {
	t.mainWindowCallback = callback
}

// SetSettingsWindowCallback sets the callback for opening the settings window.
func (t *Tray) SetSettingsWindowCallback(callback func()) {
	t.settingsWindowCallback = callback
}
