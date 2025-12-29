package gui

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/assert"
)

// TestSettingsWindow_Creation verifies that the settings window is created correctly.
func TestSettingsWindow_Creation(t *testing.T) {
	// Arrange
	app := test.NewApp()
	state := NewAppState()
	controller := NewController(state)

	// Act
	settingsWindow := NewSettingsWindow(app, controller, state)

	// Assert
	assert.NotNil(t, settingsWindow, "Settings window should be created")
	assert.NotNil(t, settingsWindow.window, "Fyne window should be initialized")
}

// TestSettingsWindow_FormFields verifies that all required form fields are present.
func TestSettingsWindow_FormFields(t *testing.T) {
	// Arrange
	app := test.NewApp()
	state := NewAppState()
	controller := NewController(state)
	settingsWindow := NewSettingsWindow(app, controller, state)

	// Act & Assert
	assert.NotNil(t, settingsWindow.apiTokenEntry, "API Token field should exist")
	assert.NotNil(t, settingsWindow.serverTypeEntry, "Server Type field should exist")
	assert.NotNil(t, settingsWindow.locationEntry, "Location field should exist")
	assert.NotNil(t, settingsWindow.volumeSizeEntry, "Volume Size field should exist")
	assert.NotNil(t, settingsWindow.socketPathEntry, "Socket Path field should exist")
}

// TestSettingsWindow_SaveValidation verifies that save validates the form.
func TestSettingsWindow_SaveValidation(t *testing.T) {
	// Arrange
	app := test.NewApp()
	state := NewAppState()
	controller := NewController(state)
	settingsWindow := NewSettingsWindow(app, controller, state)

	// Act - Try to save with empty required fields
	// The actual validation will be tested when we implement persistence

	// Assert
	// For now, just verify that the save button exists
	assert.NotNil(t, settingsWindow.saveButton, "Save button should exist")
}
