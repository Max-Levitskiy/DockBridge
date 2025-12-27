package lifecycle

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dockbridge/dockbridge/client/activity"
	"github.com/dockbridge/dockbridge/pkg/logger"
	"github.com/dockbridge/dockbridge/server"
	"github.com/dockbridge/dockbridge/shared/config"
)

// Manager handles server lifecycle based on activity tracking
type Manager struct {
	activityTracker    activity.ActivityTracker
	serverManager      server.ServerManager
	config             *config.ActivityConfig
	logger             logger.LoggerInterface
	ctx                context.Context
	cancel             context.CancelFunc
	shutdownTimer      *time.Timer
	lastServerCheck    time.Time  // Track when we last checked for servers
	hasServers         bool       // Cache whether we have servers to avoid API spam
	shutdownInProgress bool       // Flag to prevent concurrent shutdowns
	mu                 sync.Mutex // Mutex to protect shutdown state
}

// NewManager creates a new lifecycle manager
func NewManager(
	activityTracker activity.ActivityTracker,
	serverManager server.ServerManager,
	config *config.ActivityConfig,
	logger logger.LoggerInterface,
) *Manager {
	return &Manager{
		activityTracker: activityTracker,
		serverManager:   serverManager,
		config:          config,
		logger:          logger,
	}
}

// Start starts the lifecycle manager
func (m *Manager) Start(ctx context.Context) error {
	m.ctx, m.cancel = context.WithCancel(ctx)

	// Start activity tracker
	if err := m.activityTracker.Start(m.ctx); err != nil {
		return fmt.Errorf("failed to start activity tracker: %w", err)
	}

	// Register activity callback
	m.activityTracker.RegisterCallback(m.handleActivityEvent)

	// Start monitoring loop
	go m.monitoringLoop()

	m.logger.Info("Lifecycle manager started with activity-based server management")
	return nil
}

// Stop stops the lifecycle manager
func (m *Manager) Stop() error {
	if m.cancel != nil {
		m.cancel()
	}

	if m.shutdownTimer != nil {
		m.shutdownTimer.Stop()
	}

	if err := m.activityTracker.Stop(); err != nil {
		m.logger.WithFields(map[string]interface{}{
			"error": err.Error(),
		}).Error("Failed to stop activity tracker")
	}

	m.logger.Info("Lifecycle manager stopped")
	return nil
}

// RecordDockerActivity records Docker command activity
func (m *Manager) RecordDockerActivity() error {
	return m.activityTracker.RecordDockerCommand()
}

// RecordConnectionActivity records connection activity
func (m *Manager) RecordConnectionActivity() error {
	return m.activityTracker.RecordConnectionActivity()
}

// handleActivityEvent handles activity events and manages shutdown timers
func (m *Manager) handleActivityEvent(event activity.ActivityEvent) error {
	m.logger.WithFields(map[string]interface{}{
		"activity_type": string(event.Type),
		"timestamp":     event.Timestamp,
	}).Debug("Activity recorded")

	// If we have activity, we definitely have servers running - update cache
	m.hasServers = true
	m.lastServerCheck = time.Now()

	// Cancel any existing shutdown timer since we have activity
	if m.shutdownTimer != nil {
		m.shutdownTimer.Stop()
		m.shutdownTimer = nil
		m.logger.Info("Server shutdown cancelled due to activity")
	}

	// Reset shutdown flag if activity is detected
	m.mu.Lock()
	if m.shutdownInProgress {
		m.logger.Info("Cancelling shutdown in progress due to activity")
		m.shutdownInProgress = false
	}
	m.mu.Unlock()

	return nil
}

// monitoringLoop continuously monitors activity and manages server lifecycle
func (m *Manager) monitoringLoop() {
	ticker := time.NewTicker(5 * time.Second) // Check every 5 seconds for faster response
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			// Only check and schedule shutdown if we might have servers
			// This avoids constant API calls when no servers are running
			m.checkAndScheduleShutdown()
		}
	}
}

// checkAndScheduleShutdown checks if shutdown should be scheduled based on activity
func (m *Manager) checkAndScheduleShutdown() {
	// First, check if we have any running servers - don't waste API calls if none exist
	hasRunningServers, err := m.hasRunningServers()
	if err != nil {
		// If we can't check server status, log error but don't spam
		m.logger.WithFields(map[string]interface{}{
			"error": err.Error(),
		}).Debug("Failed to check server status")
		return
	}

	if !hasRunningServers {
		// No servers running, no need to check timeouts or spam logs
		return
	}

	timeUntilShutdown, reason := m.activityTracker.GetTimeUntilShutdown()

	// Only log when approaching shutdown to avoid spam
	if timeUntilShutdown <= m.config.GracePeriod*2 {
		m.logger.WithFields(map[string]interface{}{
			"time_until_shutdown": timeUntilShutdown,
			"reason":              reason,
		}).Debug("Activity check - approaching shutdown")
	}

	if timeUntilShutdown <= 0 {
		// Check if shutdown is already in progress
		m.mu.Lock()
		if m.shutdownInProgress {
			m.mu.Unlock()
			m.logger.Debug("Shutdown already in progress, skipping duplicate shutdown")
			return
		}
		m.shutdownInProgress = true
		m.mu.Unlock()

		// Time to shutdown
		m.logger.WithFields(map[string]interface{}{
			"reason": reason,
		}).Info("🚨 Activity timeout reached, shutting down server immediately")

		go m.shutdownServer(reason)
		return
	}

	// Check if we need to schedule a shutdown timer - be more aggressive
	gracePeriodThreshold := m.config.GracePeriod * 2 // Start warning earlier
	if m.shutdownTimer == nil && timeUntilShutdown <= gracePeriodThreshold {
		m.logger.WithFields(map[string]interface{}{
			"time_until_shutdown": timeUntilShutdown,
			"reason":              reason,
			"grace_period":        m.config.GracePeriod,
		}).Info("⏰ Scheduling server shutdown due to inactivity")

		m.shutdownTimer = time.AfterFunc(timeUntilShutdown, func() {
			// Check if shutdown is already in progress
			m.mu.Lock()
			if m.shutdownInProgress {
				m.mu.Unlock()
				m.logger.Debug("Shutdown already in progress, skipping timer-based shutdown")
				return
			}
			m.shutdownInProgress = true
			m.mu.Unlock()

			m.logger.WithFields(map[string]interface{}{
				"reason": reason,
			}).Info("⏱️ Grace period expired, shutting down server now")
			m.shutdownServer(reason)
		})
	}
}

// shutdownServer shuts down the server
func (m *Manager) shutdownServer(reason string) {
	// Ensure we reset the shutdown flag when done
	defer func() {
		m.mu.Lock()
		m.shutdownInProgress = false
		m.mu.Unlock()
	}()

	m.logger.WithFields(map[string]interface{}{
		"reason": reason,
	}).Info("Starting server shutdown process")

	// We already know there are running servers from hasRunningServers check
	// Skip redundant status check to avoid API spam

	// List servers to find the one to shutdown
	servers, err := m.serverManager.ListServers(m.ctx)
	if err != nil {
		m.logger.WithFields(map[string]interface{}{
			"error": err.Error(),
		}).Error("Failed to list servers for shutdown")
		return
	}

	// Find the most recent running server (no verbose logging)
	var serverToShutdown *server.ServerInfo
	for _, srv := range servers {
		if srv.Status == server.StatusRunning {
			if serverToShutdown == nil || srv.CreatedAt.After(serverToShutdown.CreatedAt) {
				serverToShutdown = srv
			}
		}
	}

	if serverToShutdown == nil {
		m.logger.Info("No running server found to shutdown (may have been destroyed already)")
		return
	}

	m.logger.WithFields(map[string]interface{}{
		"server_id":   serverToShutdown.ID,
		"server_name": serverToShutdown.Name,
		"reason":      reason,
	}).Info("Shutting down server due to inactivity")

	// Destroy the server (this preserves the volume)
	m.logger.WithFields(map[string]interface{}{
		"server_id":   serverToShutdown.ID,
		"server_name": serverToShutdown.Name,
	}).Info("💥 DESTROYING SERVER due to inactivity")

	if err := m.serverManager.DestroyServer(m.ctx, serverToShutdown.ID); err != nil {
		// Check if the error is "server not found" - this means it was already destroyed
		if isServerNotFoundError(err) {
			m.logger.WithFields(map[string]interface{}{
				"server_id": serverToShutdown.ID,
			}).Info("✅ Server already destroyed (not found), shutdown successful")
		} else {
			m.logger.WithFields(map[string]interface{}{
				"server_id": serverToShutdown.ID,
				"error":     err.Error(),
			}).Error("❌ FAILED to destroy server")
			return
		}
	} else {
		m.logger.WithFields(map[string]interface{}{
			"server_id":   serverToShutdown.ID,
			"server_name": serverToShutdown.Name,
		}).Info("✅ Server destroyed successfully, volume preserved for future use")
	}

	// Reset shutdown timer and update cache
	m.shutdownTimer = nil
	m.hasServers = false // We just destroyed the server
	m.lastServerCheck = time.Now()
}

// hasRunningServers checks if there are any running servers with caching to avoid API spam
func (m *Manager) hasRunningServers() (bool, error) {
	now := time.Now()

	// Only check server status every 30 seconds to avoid API spam
	if now.Sub(m.lastServerCheck) < 30*time.Second {
		return m.hasServers, nil
	}

	// Use a quick server status check instead of listing all servers
	status, err := m.serverManager.GetServerStatus(m.ctx)
	if err != nil {
		return false, err
	}

	// Update cache
	m.lastServerCheck = now
	m.hasServers = *status == server.StatusRunning

	return m.hasServers, nil
}

// isServerNotFoundError checks if the error indicates the server was not found
func isServerNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "server not found") ||
		strings.Contains(errStr, "not found") ||
		strings.Contains(errStr, "not_found")
}
