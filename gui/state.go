package gui

import (
	"sync"
)

// ServerStatus represents the current status of the DockBridge server.
type ServerStatus int

const (
	// StatusStopped indicates the daemon is not running or server is down.
	StatusStopped ServerStatus = iota
	// StatusProvisioning indicates the server is starting.
	StatusProvisioning
	// StatusRunning indicates the server is active and connected.
	StatusRunning
	// StatusStopping indicates the server is shutting down.
	StatusStopping
	// StatusError indicates a connection failure.
	StatusError
)

// String returns a human-readable representation of the status.
func (s ServerStatus) String() string {
	switch s {
	case StatusStopped:
		return "Stopped"
	case StatusProvisioning:
		return "Provisioning"
	case StatusRunning:
		return "Running"
	case StatusStopping:
		return "Stopping"
	case StatusError:
		return "Error"
	default:
		return "Unknown"
	}
}

// AppState holds the ephemeral application state for the GUI.
type AppState struct {
	mu              sync.RWMutex
	status          ServerStatus
	daemonConnected bool
	serverIP        string
	lastError       error
	statusCallbacks []func(ServerStatus)
}

// NewAppState creates a new application state instance.
func NewAppState() *AppState {
	return &AppState{
		status:          StatusStopped,
		daemonConnected: false,
		statusCallbacks: make([]func(ServerStatus), 0),
	}
}

// GetStatus returns the current server status.
func (s *AppState) GetStatus() ServerStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// SetStatus updates the server status and notifies registered callbacks.
func (s *AppState) SetStatus(status ServerStatus) {
	s.mu.Lock()
	s.status = status
	callbacks := s.statusCallbacks
	s.mu.Unlock()

	// Notify all callbacks outside the lock to avoid deadlock
	for _, callback := range callbacks {
		callback(status)
	}
}

// IsDaemonConnected returns whether the GUI is connected to the daemon.
func (s *AppState) IsDaemonConnected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.daemonConnected
}

// SetDaemonConnected updates the daemon connection status.
func (s *AppState) SetDaemonConnected(connected bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.daemonConnected = connected
}

// GetServerIP returns the current server IP address.
func (s *AppState) GetServerIP() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.serverIP
}

// SetServerIP updates the server IP address.
func (s *AppState) SetServerIP(ip string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.serverIP = ip
}

// GetLastError returns the last error that occurred.
func (s *AppState) GetLastError() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastError
}

// SetLastError updates the last error.
func (s *AppState) SetLastError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastError = err
}

// RegisterStatusCallback adds a callback to be notified when status changes.
func (s *AppState) RegisterStatusCallback(callback func(ServerStatus)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statusCallbacks = append(s.statusCallbacks, callback)
}
