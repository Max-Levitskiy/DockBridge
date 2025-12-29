package gui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// MainWindow represents the main dashboard window for DockBridge.
type MainWindow struct {
	app        fyne.App
	controller *Controller
	state      *AppState
	window     fyne.Window

	// UI elements
	statusLabel   *widget.Label
	ipLabel       *widget.Label
	errorLabel    *widget.Label
	startButton   *widget.Button
	stopButton    *widget.Button
	connectButton *widget.Button
}

// NewMainWindow creates a new main window instance.
func NewMainWindow(app fyne.App, controller *Controller, state *AppState) *MainWindow {
	mw := &MainWindow{
		app:        app,
		controller: controller,
		state:      state,
	}

	mw.window = app.NewWindow("DockBridge - Dashboard")
	mw.buildUI()

	// Register status callback
	mw.state.RegisterStatusCallback(mw.onStatusChange)

	return mw
}

// buildUI constructs the main dashboard interface.
func (mw *MainWindow) buildUI() {
	// Status display
	mw.statusLabel = widget.NewLabel("Status: " + mw.state.GetStatus().String())
	mw.ipLabel = widget.NewLabel("Server IP: " + mw.state.GetServerIP())
	mw.errorLabel = widget.NewLabel("")

	// Action buttons
	mw.startButton = widget.NewButton("Start Server", mw.onStartServer)
	mw.stopButton = widget.NewButton("Stop Server", mw.onStopServer)
	mw.connectButton = widget.NewButton("Connect to Docker", mw.onConnect)

	// Initial button states
	mw.updateButtonStates()

	// Layout
	statusBox := container.NewVBox(
		widget.NewLabelWithStyle("DockBridge Dashboard", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		mw.statusLabel,
		mw.ipLabel,
		mw.errorLabel,
	)

	buttonBox := container.NewHBox(
		mw.startButton,
		mw.stopButton,
		mw.connectButton,
	)

	content := container.NewBorder(
		statusBox,
		buttonBox,
		nil,
		nil,
		container.NewCenter(widget.NewLabel("Use the buttons below to control your DockBridge server.")),
	)

	mw.window.SetContent(container.NewPadded(content))
	mw.window.Resize(fyne.NewSize(500, 300))
}

// onStartServer handles the Start Server button click.
func (mw *MainWindow) onStartServer() {
	if err := mw.controller.StartServer(); err != nil {
		mw.state.SetLastError(err)
		mw.errorLabel.SetText(fmt.Sprintf("Error: %v", err))
	} else {
		mw.errorLabel.SetText("")
	}
	mw.updateButtonStates()
}

// onStopServer handles the Stop Server button click.
func (mw *MainWindow) onStopServer() {
	if err := mw.controller.StopServer(); err != nil {
		mw.state.SetLastError(err)
		mw.errorLabel.SetText(fmt.Sprintf("Error: %v", err))
	} else {
		mw.errorLabel.SetText("")
	}
	mw.updateButtonStates()
}

// onConnect handles the Connect to Docker button click.
func (mw *MainWindow) onConnect() {
	// TODO: Implement Docker connection logic
	mw.errorLabel.SetText("Docker connection not yet implemented")
}

// onStatusChange is called when the application status changes.
func (mw *MainWindow) onStatusChange(status ServerStatus) {
	// All UI updates must run on the main Fyne thread
	fyne.Do(func() {
		mw.statusLabel.SetText("Status: " + status.String())
		mw.ipLabel.SetText("Server IP: " + mw.state.GetServerIP())

		if err := mw.state.GetLastError(); err != nil {
			mw.errorLabel.SetText(fmt.Sprintf("Error: %v", err))
		} else {
			mw.errorLabel.SetText("")
		}

		mw.updateButtonStates()
	})
}

// updateButtonStates updates button enabled/disabled states based on current status.
func (mw *MainWindow) updateButtonStates() {
	status := mw.state.GetStatus()

	switch status {
	case StatusStopped:
		mw.startButton.Enable()
		mw.stopButton.Disable()
		mw.connectButton.Disable()
	case StatusProvisioning:
		mw.startButton.Disable()
		mw.stopButton.Disable()
		mw.connectButton.Disable()
	case StatusRunning:
		mw.startButton.Disable()
		mw.stopButton.Enable()
		mw.connectButton.Enable()
	case StatusStopping:
		mw.startButton.Disable()
		mw.stopButton.Disable()
		mw.connectButton.Disable()
	case StatusError:
		mw.startButton.Enable()
		mw.stopButton.Disable()
		mw.connectButton.Disable()
	}
}

// Show displays the main window.
func (mw *MainWindow) Show() {
	mw.window.Show()
}
