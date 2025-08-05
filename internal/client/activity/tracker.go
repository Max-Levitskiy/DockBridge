package activity

import (
	"context"
	"sync"
	"time"

	"github.com/dockbridge/dockbridge/internal/shared/config"
)

// ActivityType represents the type of activity
type ActivityType string

const (
	ActivityTypeDockerCommand    ActivityType = "docker_command"
	ActivityTypeConnectionActive ActivityType = "connection_active"
)

// ActivityEvent represents an activity event
type ActivityEvent struct {
	Type      ActivityType `json:"type"`
	Timestamp time.Time    `json:"timestamp"`
}

// ActivityCallback is called when activity events occur
type ActivityCallback func(event ActivityEvent) error

// ActivityTracker interface for tracking Docker commands and connections
type ActivityTracker interface {
	Start(ctx context.Context) error
	Stop() error
	RecordDockerCommand() error
	RecordConnectionActivity() error
	GetLastActivity() time.Time
	GetLastConnection() time.Time
	GetTimeUntilShutdown() (time.Duration, string)
	RegisterCallback(callback ActivityCallback)
}

// Tracker implements the ActivityTracker interface
type Tracker struct {
	config    *config.ActivityConfig
	mu        sync.RWMutex
	lastCmd   time.Time
	lastConn  time.Time
	callbacks []ActivityCallback
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewTracker creates a new activity tracker
func NewTracker(config *config.ActivityConfig) *Tracker {
	return &Tracker{
		config:    config,
		callbacks: make([]ActivityCallback, 0),
	}
}

// Start starts the activity tracker
func (t *Tracker) Start(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.ctx, t.cancel = context.WithCancel(ctx)

	// Initialize timestamps
	now := time.Now()
	t.lastCmd = now
	t.lastConn = now

	return nil
}

// Stop stops the activity tracker
func (t *Tracker) Stop() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.cancel != nil {
		t.cancel()
	}

	return nil
}

// RecordDockerCommand records a Docker command execution
func (t *Tracker) RecordDockerCommand() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	t.lastCmd = now
	t.lastConn = now // Docker commands also count as connection activity

	event := ActivityEvent{
		Type:      ActivityTypeDockerCommand,
		Timestamp: now,
	}

	// Notify callbacks
	for _, callback := range t.callbacks {
		go func(cb ActivityCallback) {
			_ = cb(event)
		}(callback)
	}

	return nil
}

// RecordConnectionActivity records connection activity
func (t *Tracker) RecordConnectionActivity() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	t.lastConn = now

	event := ActivityEvent{
		Type:      ActivityTypeConnectionActive,
		Timestamp: now,
	}

	// Notify callbacks
	for _, callback := range t.callbacks {
		go func(cb ActivityCallback) {
			_ = cb(event)
		}(callback)
	}

	return nil
}

// GetLastActivity returns the timestamp of the last Docker command
func (t *Tracker) GetLastActivity() time.Time {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.lastCmd
}

// GetLastConnection returns the timestamp of the last connection activity
func (t *Tracker) GetLastConnection() time.Time {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.lastConn
}

// GetTimeUntilShutdown calculates time until shutdown and the reason
func (t *Tracker) GetTimeUntilShutdown() (time.Duration, string) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	now := time.Now()

	// Check idle timeout (no Docker commands)
	timeSinceLastCmd := now.Sub(t.lastCmd)
	if timeSinceLastCmd >= t.config.IdleTimeout {
		return 0, "idle_timeout"
	}

	// Check connection timeout (no connection activity)
	timeSinceLastConn := now.Sub(t.lastConn)
	if timeSinceLastConn >= t.config.ConnectionTimeout {
		return 0, "connection_timeout"
	}

	// Calculate time until next timeout
	timeUntilIdleTimeout := t.config.IdleTimeout - timeSinceLastCmd
	timeUntilConnTimeout := t.config.ConnectionTimeout - timeSinceLastConn

	// Return the shorter timeout
	if timeUntilIdleTimeout < timeUntilConnTimeout {
		return timeUntilIdleTimeout, "approaching_idle_timeout"
	}

	return timeUntilConnTimeout, "approaching_connection_timeout"
}

// RegisterCallback registers a callback for activity events
func (t *Tracker) RegisterCallback(callback ActivityCallback) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.callbacks = append(t.callbacks, callback)
}
