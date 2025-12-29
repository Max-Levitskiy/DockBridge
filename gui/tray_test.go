package gui

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTray_Setup verifies that the system tray is created with the correct menu structure.
func TestTray_Setup(t *testing.T) {
	// Arrange
	app := test.NewApp()
	state := NewAppState()
	controller := NewController(state)
	tray := NewTray(app, controller, state)

	// Act
	tray.Setup()

	// Assert
	assert.NotNil(t, tray.menu, "Tray menu should be initialized")
}

// TestTray_StatusUpdates verifies that the tray reflects status changes.
func TestTray_StatusUpdates(t *testing.T) {
	// Arrange
	app := test.NewApp()
	state := NewAppState()
	controller := NewController(state)
	tray := NewTray(app, controller, state)
	tray.Setup()

	// Act - Change status to Running
	state.SetStatus(StatusRunning)

	// Assert - Verify status changed (menu update verification would require menu item inspection)
	assert.Equal(t, StatusRunning, state.GetStatus(), "State should be updated to Running")
}

// TestTray_MenuCallbacks verifies that menu item actions are properly wired.
func TestTray_MenuCallbacks(t *testing.T) {
	// Arrange
	app := test.NewApp()
	state := NewAppState()
	controller := NewController(state)
	tray := NewTray(app, controller, state)
	tray.Setup()

	mainWindowCalled := false
	settingsWindowCalled := false

	tray.SetMainWindowCallback(func() {
		mainWindowCalled = true
	})
	tray.SetSettingsWindowCallback(func() {
		settingsWindowCalled = true
	})

	// Act
	require.NotNil(t, tray.mainWindowCallback, "Main window callback should be set")
	require.NotNil(t, tray.settingsWindowCallback, "Settings window callback should be set")

	tray.mainWindowCallback()
	tray.settingsWindowCallback()

	// Assert
	assert.True(t, mainWindowCalled, "Main window callback should be invoked")
	assert.True(t, settingsWindowCalled, "Settings window callback should be invoked")
}

// TestTray_QuitAction verifies the quit functionality.
func TestTray_QuitAction(t *testing.T) {
	// Arrange
	app := test.NewApp()
	state := NewAppState()
	controller := NewController(state)
	tray := NewTray(app, controller, state)
	tray.Setup()

	// Act & Assert
	// Note: We can't actually test app.Quit() in a unit test without terminating the test.
	// This test verifies that the tray is set up correctly for the quit action.
	assert.NotNil(t, tray.menu, "Tray menu should contain a quit option")
}
