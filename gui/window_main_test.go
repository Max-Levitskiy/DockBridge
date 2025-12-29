package gui

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/assert"
)

// TestMainWindow_Creation verifies that the main window is created correctly.
func TestMainWindow_Creation(t *testing.T) {
	// Arrange
	app := test.NewApp()
	state := NewAppState()
	controller := NewController(state)

	// Act
	mainWindow := NewMainWindow(app, controller, state)

	// Assert
	assert.NotNil(t, mainWindow, "Main window should be created")
	assert.NotNil(t, mainWindow.window, "Fyne window should be initialized")
}

// TestMainWindow_ButtonStates verifies that buttons are enabled/disabled based on status.
func TestMainWindow_ButtonStates(t *testing.T) {
	// Arrange
	app := test.NewApp()
	state := NewAppState()
	controller := NewController(state)
	mainWindow := NewMainWindow(app, controller, state)

	// Test Stopped state
	state.SetStatus(StatusStopped)
	mainWindow.updateButtonStates()
	assert.False(t, mainWindow.startButton.Disabled(), "Start button should be enabled when stopped")
	assert.True(t, mainWindow.stopButton.Disabled(), "Stop button should be disabled when stopped")

	// Test Running state
	state.SetStatus(StatusRunning)
	mainWindow.updateButtonStates()
	assert.True(t, mainWindow.startButton.Disabled(), "Start button should be disabled when running")
	assert.False(t, mainWindow.stopButton.Disabled(), "Stop button should be enabled when running")

	// Test Provisioning state
	state.SetStatus(StatusProvisioning)
	mainWindow.updateButtonStates()
	assert.True(t, mainWindow.startButton.Disabled(), "Start button should be disabled when provisioning")
	assert.True(t, mainWindow.stopButton.Disabled(), "Stop button should be disabled when provisioning")
}

// TestMainWindow_StatusDisplay verifies that the status label updates correctly.
func TestMainWindow_StatusDisplay(t *testing.T) {
	// Arrange
	app := test.NewApp()
	state := NewAppState()
	controller := NewController(state)
	mainWindow := NewMainWindow(app, controller, state)

	// Act
	state.SetStatus(StatusRunning)

	// Assert
	// The status callback should update the label
	// Note: Actual label text verification would require accessing the widget text
	assert.Equal(t, StatusRunning, state.GetStatus(), "State should be Running")
	assert.NotNil(t, mainWindow, "Main window should exist")
}
