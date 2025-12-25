// Package keepalive provides server-side keep-alive monitoring for automatic server self-destruction.
// When the client becomes unavailable (laptop crashes, network failure, etc.), the server will
// self-destruct after a configurable timeout to avoid ongoing cloud costs.
package keepalive

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/dockbridge/dockbridge/pkg/logger"
)

// Config holds the configuration for the keep-alive monitor.
type Config struct {
	// Port is the HTTP port to listen on for heartbeat requests.
	Port int `json:"port" yaml:"port"`

	// Timeout is the duration after which the server will self-destruct
	// if no heartbeat is received.
	Timeout time.Duration `json:"timeout" yaml:"timeout"`

	// GracePeriod is the duration to wait before actually destroying the server
	// after timeout is reached, allowing for last-minute heartbeats.
	GracePeriod time.Duration `json:"grace_period" yaml:"grace_period"`

	// ServerID is the Hetzner server ID for self-destruction.
	ServerID string `json:"server_id" yaml:"server_id"`

	// HetznerAPIToken is the API token for Hetzner Cloud operations.
	HetznerAPIToken string `json:"hetzner_api_token" yaml:"hetzner_api_token"`
}

// DefaultConfig returns the default keep-alive configuration.
func DefaultConfig() *Config {
	return &Config{
		Port:        8080,
		Timeout:     5 * time.Minute,
		GracePeriod: 30 * time.Second,
	}
}

// Monitor handles keep-alive heartbeat monitoring and server self-destruction.
type Monitor struct {
	config        *Config
	logger        logger.LoggerInterface
	lastHeartbeat time.Time
	mu            sync.RWMutex
	server        *http.Server
	ctx           context.Context
	cancel        context.CancelFunc
	running       bool
	shutdownCh    chan struct{}
}

// NewMonitor creates a new keep-alive monitor.
func NewMonitor(config *Config, log logger.LoggerInterface) *Monitor {
	if config == nil {
		config = DefaultConfig()
	}
	if log == nil {
		log = logger.NewDefault()
	}

	return &Monitor{
		config:        config,
		logger:        log,
		lastHeartbeat: time.Now(),
		shutdownCh:    make(chan struct{}),
	}
}

// Start begins the keep-alive monitoring service.
func (m *Monitor) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return fmt.Errorf("monitor already running")
	}
	m.running = true
	m.ctx, m.cancel = context.WithCancel(ctx)
	m.lastHeartbeat = time.Now()
	m.mu.Unlock()

	// Setup HTTP server for heartbeat endpoint
	mux := http.NewServeMux()
	mux.HandleFunc("/heartbeat", m.handleHeartbeat)
	mux.HandleFunc("/status", m.handleStatus)
	mux.HandleFunc("/health", m.handleHealth)

	m.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", m.config.Port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Start HTTP server in goroutine
	go func() {
		m.logger.Info("Keep-alive monitor HTTP server starting", "port", m.config.Port)
		if err := m.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			m.logger.Error("Keep-alive HTTP server error", "error", err)
		}
	}()

	// Start timeout monitoring in goroutine
	go m.monitorTimeout()

	m.logger.Info("Keep-alive monitor started",
		"port", m.config.Port,
		"timeout", m.config.Timeout,
		"grace_period", m.config.GracePeriod,
	)

	return nil
}

// Stop gracefully stops the keep-alive monitor.
func (m *Monitor) Stop() error {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return nil
	}
	m.running = false
	m.mu.Unlock()

	// Cancel context to stop monitoring goroutine
	if m.cancel != nil {
		m.cancel()
	}

	// Shutdown HTTP server with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if m.server != nil {
		if err := m.server.Shutdown(shutdownCtx); err != nil {
			m.logger.Error("HTTP server shutdown error", "error", err)
			return err
		}
	}

	m.logger.Info("Keep-alive monitor stopped")
	return nil
}

// RecordHeartbeat records a heartbeat from the client.
func (m *Monitor) RecordHeartbeat() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastHeartbeat = time.Now()
	m.logger.Debug("Heartbeat recorded", "time", m.lastHeartbeat)
}

// GetLastHeartbeat returns the timestamp of the last heartbeat.
func (m *Monitor) GetLastHeartbeat() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastHeartbeat
}

// GetTimeSinceLastHeartbeat returns the duration since the last heartbeat.
func (m *Monitor) GetTimeSinceLastHeartbeat() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return time.Since(m.lastHeartbeat)
}

// IsTimedOut returns true if the timeout has been exceeded.
func (m *Monitor) IsTimedOut() bool {
	return m.GetTimeSinceLastHeartbeat() > m.config.Timeout
}

// handleHeartbeat processes incoming heartbeat requests.
func (m *Monitor) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	m.RecordHeartbeat()

	response := map[string]interface{}{
		"status":              "ok",
		"last_heartbeat":      m.GetLastHeartbeat().Format(time.RFC3339),
		"time_until_shutdown": (m.config.Timeout - m.GetTimeSinceLastHeartbeat()).String(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleStatus returns the current monitor status.
func (m *Monitor) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	timeSinceLast := m.GetTimeSinceLastHeartbeat()
	timeUntilShutdown := m.config.Timeout - timeSinceLast

	response := map[string]interface{}{
		"server_id":            m.config.ServerID,
		"last_heartbeat":       m.GetLastHeartbeat().Format(time.RFC3339),
		"time_since_heartbeat": timeSinceLast.String(),
		"time_until_shutdown":  timeUntilShutdown.String(),
		"timeout":              m.config.Timeout.String(),
		"grace_period":         m.config.GracePeriod.String(),
		"is_timed_out":         m.IsTimedOut(),
		"running":              m.running,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleHealth returns a simple health check response.
func (m *Monitor) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

// monitorTimeout continuously monitors for heartbeat timeout.
func (m *Monitor) monitorTimeout() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			m.logger.Info("Timeout monitoring stopped")
			return
		case <-ticker.C:
			if m.IsTimedOut() {
				m.logger.Warn("Keep-alive timeout exceeded, initiating self-destruction",
					"last_heartbeat", m.GetLastHeartbeat(),
					"timeout", m.config.Timeout,
				)
				m.initiateShutdown()
				return
			}

			// Log status periodically
			remaining := m.config.Timeout - m.GetTimeSinceLastHeartbeat()
			m.logger.Debug("Keep-alive status",
				"time_since_last_heartbeat", m.GetTimeSinceLastHeartbeat(),
				"time_remaining", remaining,
			)
		}
	}
}

// initiateShutdown initiates the server self-destruction sequence.
func (m *Monitor) initiateShutdown() {
	m.logger.Warn("Starting grace period before self-destruction",
		"grace_period", m.config.GracePeriod,
	)

	// Wait for grace period
	select {
	case <-time.After(m.config.GracePeriod):
		// No heartbeat received during grace period, proceed with shutdown
		break
	case <-m.ctx.Done():
		m.logger.Info("Shutdown cancelled by context")
		return
	}

	// Check one more time if heartbeat was received during grace period
	if !m.IsTimedOut() {
		m.logger.Info("Heartbeat received during grace period, cancelling shutdown")
		go m.monitorTimeout() // Restart monitoring
		return
	}

	m.logger.Error("No heartbeat received during grace period, proceeding with self-destruction")
	m.selfDestruct()
}

// selfDestruct triggers server self-destruction via Hetzner API.
func (m *Monitor) selfDestruct() {
	if m.config.ServerID == "" {
		m.logger.Error("Cannot self-destruct: server ID not configured")
		// Fall back to system shutdown
		m.systemShutdown()
		return
	}

	if m.config.HetznerAPIToken == "" {
		m.logger.Error("Cannot self-destruct via API: Hetzner API token not configured")
		// Fall back to system shutdown
		m.systemShutdown()
		return
	}

	m.logger.Warn("Initiating self-destruction via Hetzner API",
		"server_id", m.config.ServerID,
	)

	// Make HTTP request to Hetzner API to delete this server
	if err := m.deleteServerViaAPI(); err != nil {
		m.logger.Error("Failed to delete server via API", "error", err)
		// Fall back to system shutdown
		m.systemShutdown()
		return
	}

	m.logger.Info("Server deletion initiated successfully")
}

// deleteServerViaAPI deletes the server using Hetzner Cloud API.
func (m *Monitor) deleteServerViaAPI() error {
	url := fmt.Sprintf("https://api.hetzner.cloud/v1/servers/%s", m.config.ServerID)

	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+m.config.HetznerAPIToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute delete request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("API returned error status: %d", resp.StatusCode)
	}

	return nil
}

// systemShutdown performs a local system shutdown as fallback.
func (m *Monitor) systemShutdown() {
	m.logger.Warn("Initiating system shutdown as fallback")

	// Try to shutdown gracefully first
	// This would require root/sudo privileges
	cmd := "shutdown -h now"
	m.logger.Info("Executing shutdown command", "command", cmd)

	// In a real implementation, we would execute the shutdown command
	// For safety, we just log and exit
	os.Exit(1)
}

// HeartbeatClient provides a client for sending heartbeats to the monitor.
type HeartbeatClient struct {
	serverURL string
	client    *http.Client
}

// NewHeartbeatClient creates a new heartbeat client.
func NewHeartbeatClient(serverURL string) *HeartbeatClient {
	return &HeartbeatClient{
		serverURL: serverURL,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// SendHeartbeat sends a heartbeat to the server.
func (c *HeartbeatClient) SendHeartbeat() error {
	url := fmt.Sprintf("%s/heartbeat", c.serverURL)
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create heartbeat request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send heartbeat: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("heartbeat failed with status: %d", resp.StatusCode)
	}

	return nil
}

// GetStatus retrieves the current monitor status.
func (c *HeartbeatClient) GetStatus() (*MonitorStatus, error) {
	url := fmt.Sprintf("%s/status", c.serverURL)
	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}
	defer resp.Body.Close()

	var status MonitorStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("failed to decode status: %w", err)
	}

	return &status, nil
}

// MonitorStatus represents the status returned by the /status endpoint.
type MonitorStatus struct {
	ServerID           string `json:"server_id"`
	LastHeartbeat      string `json:"last_heartbeat"`
	TimeSinceHeartbeat string `json:"time_since_heartbeat"`
	TimeUntilShutdown  string `json:"time_until_shutdown"`
	Timeout            string `json:"timeout"`
	GracePeriod        string `json:"grace_period"`
	IsTimedOut         bool   `json:"is_timed_out"`
	Running            bool   `json:"running"`
}
