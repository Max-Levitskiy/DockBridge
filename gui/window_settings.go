package gui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/Max-Levitskiy/DockBridge/client/config"
	"gopkg.in/yaml.v3"
)

// SettingsWindow represents the configuration settings dialog.
type SettingsWindow struct {
	app        fyne.App
	controller *Controller
	state      *AppState
	window     fyne.Window

	// Form fields
	apiTokenEntry   *widget.Entry
	serverTypeEntry *widget.Entry
	locationEntry   *widget.Entry
	volumeSizeEntry *widget.Entry
	socketPathEntry *widget.Entry
	saveButton      *widget.Button
	cancelButton    *widget.Button
}

// NewSettingsWindow creates a new settings window instance.
func NewSettingsWindow(app fyne.App, controller *Controller, state *AppState) *SettingsWindow {
	sw := &SettingsWindow{
		app:        app,
		controller: controller,
		state:      state,
	}

	sw.window = app.NewWindow("DockBridge - Settings")
	sw.buildUI()

	return sw
}

// buildUI constructs the settings form.
func (sw *SettingsWindow) buildUI() {
	// Load current configuration
	manager := config.NewManager()
	configPath, _ := config.GetDefaultConfigPath("client")
	_ = manager.Load(configPath) // Ignore error for now, we'll use defaults
	cfg := manager.GetConfig()

	// Create form fields
	sw.apiTokenEntry = widget.NewPasswordEntry()
	sw.apiTokenEntry.SetPlaceHolder("Enter Hetzner API Token")
	sw.apiTokenEntry.SetText(cfg.Hetzner.APIToken)

	sw.serverTypeEntry = widget.NewEntry()
	sw.serverTypeEntry.SetPlaceHolder("e.g., cpx21")
	sw.serverTypeEntry.SetText(cfg.Hetzner.ServerType)

	sw.locationEntry = widget.NewEntry()
	sw.locationEntry.SetPlaceHolder("e.g., fsn1")
	sw.locationEntry.SetText(cfg.Hetzner.Location)

	sw.volumeSizeEntry = widget.NewEntry()
	sw.volumeSizeEntry.SetPlaceHolder("Volume size in GB")
	sw.volumeSizeEntry.SetText(strconv.Itoa(cfg.Hetzner.VolumeSize))

	sw.socketPathEntry = widget.NewEntry()
	sw.socketPathEntry.SetPlaceHolder("Docker socket path")
	sw.socketPathEntry.SetText(cfg.Docker.SocketPath)

	// Create buttons
	sw.saveButton = widget.NewButton("Save", sw.onSave)
	sw.cancelButton = widget.NewButton("Cancel", func() {
		sw.window.Hide()
	})

	// Build form layout
	form := container.NewVBox(
		widget.NewLabel("Hetzner Configuration"),
		widget.NewForm(
			widget.NewFormItem("API Token", sw.apiTokenEntry),
			widget.NewFormItem("Server Type", sw.serverTypeEntry),
			widget.NewFormItem("Location", sw.locationEntry),
			widget.NewFormItem("Volume Size (GB)", sw.volumeSizeEntry),
		),
		widget.NewSeparator(),
		widget.NewLabel("Docker Configuration"),
		widget.NewForm(
			widget.NewFormItem("Socket Path", sw.socketPathEntry),
		),
		widget.NewSeparator(),
		container.NewHBox(
			sw.saveButton,
			sw.cancelButton,
		),
	)

	sw.window.SetContent(container.NewPadded(form))
	sw.window.Resize(fyne.NewSize(500, 400))
}

// onSave handles the save button click.
func (sw *SettingsWindow) onSave() {
	// Parse form values
	volumeSize, err := strconv.Atoi(sw.volumeSizeEntry.Text)
	if err != nil {
		sw.showError("Invalid volume size: must be a number")
		return
	}

	// Load configuration
	manager := config.NewManager()
	configPath, _ := config.GetDefaultConfigPath("client")
	_ = manager.Load(configPath)
	cfg := manager.GetConfig()

	// Update configuration
	cfg.Hetzner.APIToken = sw.apiTokenEntry.Text
	cfg.Hetzner.ServerType = sw.serverTypeEntry.Text
	cfg.Hetzner.Location = sw.locationEntry.Text
	cfg.Hetzner.VolumeSize = volumeSize
	cfg.Docker.SocketPath = sw.socketPathEntry.Text

	// Save configuration to file
	if err := sw.saveConfig(configPath, cfg); err != nil {
		sw.showError(fmt.Sprintf("Failed to save configuration: %v", err))
		return
	}

	// Show success and close
	sw.showInfo("Configuration saved successfully!")
	sw.window.Hide()
}

// saveConfig writes the configuration to a YAML file.
func (sw *SettingsWindow) saveConfig(path string, cfg interface{}) error {
	// Use the same mechanism from client/cli/init.go
	yamlData, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(path, yamlData, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// showError displays an error dialog.
func (sw *SettingsWindow) showError(message string) {
	dialog := widget.NewLabel(message)
	errorWindow := sw.app.NewWindow("Error")
	errorWindow.SetContent(container.NewPadded(
		container.NewVBox(
			dialog,
			widget.NewButton("OK", func() {
				errorWindow.Hide()
			}),
		),
	))
	errorWindow.Show()
}

// showInfo displays an info dialog.
func (sw *SettingsWindow) showInfo(message string) {
	dialog := widget.NewLabel(message)
	infoWindow := sw.app.NewWindow("Success")
	infoWindow.SetContent(container.NewPadded(
		container.NewVBox(
			dialog,
			widget.NewButton("OK", func() {
				infoWindow.Hide()
			}),
		),
	))
	infoWindow.Show()
}

// Show displays the settings window.
func (sw *SettingsWindow) Show() {
	sw.window.Show()
}
