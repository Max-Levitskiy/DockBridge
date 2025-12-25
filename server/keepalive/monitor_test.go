package keepalive

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMonitor(t *testing.T) {
	t.Run("with nil config uses defaults", func(t *testing.T) {
		m := NewMonitor(nil, nil)
		require.NotNil(t, m)
		assert.Equal(t, 8080, m.config.Port)
		assert.Equal(t, 5*time.Minute, m.config.Timeout)
		assert.Equal(t, 30*time.Second, m.config.GracePeriod)
	})

	t.Run("with custom config", func(t *testing.T) {
		config := &Config{
			Port:        9090,
			Timeout:     10 * time.Minute,
			GracePeriod: 1 * time.Minute,
			ServerID:    "test-123",
		}
		m := NewMonitor(config, nil)
		require.NotNil(t, m)
		assert.Equal(t, 9090, m.config.Port)
		assert.Equal(t, 10*time.Minute, m.config.Timeout)
		assert.Equal(t, "test-123", m.config.ServerID)
	})
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	assert.Equal(t, 8080, config.Port)
	assert.Equal(t, 5*time.Minute, config.Timeout)
	assert.Equal(t, 30*time.Second, config.GracePeriod)
}

func TestMonitor_RecordHeartbeat(t *testing.T) {
	m := NewMonitor(nil, nil)

	// Initial heartbeat time
	initialTime := m.GetLastHeartbeat()

	// Wait a small amount
	time.Sleep(10 * time.Millisecond)

	// Record new heartbeat
	m.RecordHeartbeat()

	// Verify heartbeat was updated
	newTime := m.GetLastHeartbeat()
	assert.True(t, newTime.After(initialTime), "Heartbeat should be updated to later time")
}

func TestMonitor_GetTimeSinceLastHeartbeat(t *testing.T) {
	m := NewMonitor(nil, nil)
	m.RecordHeartbeat()

	// Immediately after heartbeat, time since should be very small
	duration := m.GetTimeSinceLastHeartbeat()
	assert.Less(t, duration, time.Second, "Time since heartbeat should be less than 1 second")

	// Wait and check again
	time.Sleep(100 * time.Millisecond)
	duration = m.GetTimeSinceLastHeartbeat()
	assert.GreaterOrEqual(t, duration, 100*time.Millisecond, "Time since heartbeat should increase")
}

func TestMonitor_IsTimedOut(t *testing.T) {
	config := &Config{
		Port:    8080,
		Timeout: 100 * time.Millisecond,
	}
	m := NewMonitor(config, nil)
	m.RecordHeartbeat()

	// Should not be timed out immediately
	assert.False(t, m.IsTimedOut(), "Should not be timed out immediately after heartbeat")

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// Should be timed out now
	assert.True(t, m.IsTimedOut(), "Should be timed out after waiting longer than timeout")
}

func TestMonitor_HandleHeartbeat(t *testing.T) {
	m := NewMonitor(nil, nil)

	t.Run("POST request succeeds", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/heartbeat", nil)
		rec := httptest.NewRecorder()

		m.handleHeartbeat(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response map[string]interface{}
		err := json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, "ok", response["status"])
		assert.NotEmpty(t, response["last_heartbeat"])
	})

	t.Run("PUT request succeeds", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/heartbeat", nil)
		rec := httptest.NewRecorder()

		m.handleHeartbeat(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("GET request fails", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/heartbeat", nil)
		rec := httptest.NewRecorder()

		m.handleHeartbeat(rec, req)

		assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	})
}

func TestMonitor_HandleStatus(t *testing.T) {
	config := &Config{
		Port:        8080,
		Timeout:     5 * time.Minute,
		GracePeriod: 30 * time.Second,
		ServerID:    "server-123",
	}
	m := NewMonitor(config, nil)
	m.running = true

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	rec := httptest.NewRecorder()

	m.handleStatus(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response map[string]interface{}
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, "server-123", response["server_id"])
	assert.Equal(t, "5m0s", response["timeout"])
	assert.Equal(t, "30s", response["grace_period"])
	assert.Equal(t, false, response["is_timed_out"])
	assert.Equal(t, true, response["running"])
}

func TestMonitor_HandleHealth(t *testing.T) {
	m := NewMonitor(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	m.handleHealth(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response map[string]string
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "healthy", response["status"])
}

func TestMonitor_StartStop(t *testing.T) {
	config := &Config{
		Port:    18080, // Use high port to avoid conflicts
		Timeout: 5 * time.Minute,
	}
	m := NewMonitor(config, nil)

	ctx := context.Background()

	// Start the monitor
	err := m.Start(ctx)
	require.NoError(t, err)

	// Wait for server to start
	time.Sleep(50 * time.Millisecond)

	// Verify it's running
	assert.True(t, m.running)

	// Stop the monitor
	err = m.Stop()
	require.NoError(t, err)

	// Verify it's stopped
	assert.False(t, m.running)
}

func TestMonitor_StartTwiceFails(t *testing.T) {
	config := &Config{
		Port:    18081,
		Timeout: 5 * time.Minute,
	}
	m := NewMonitor(config, nil)

	ctx := context.Background()

	// Start once
	err := m.Start(ctx)
	require.NoError(t, err)
	defer m.Stop()

	// Start again should fail
	err = m.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")
}

func TestHeartbeatClient_SendHeartbeat(t *testing.T) {
	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/heartbeat" && r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":         "ok",
				"last_heartbeat": time.Now().Format(time.RFC3339),
			})
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	client := NewHeartbeatClient(server.URL)

	err := client.SendHeartbeat()
	assert.NoError(t, err)
}

func TestHeartbeatClient_GetStatus(t *testing.T) {
	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/status" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(MonitorStatus{
				ServerID:           "test-server",
				LastHeartbeat:      time.Now().Format(time.RFC3339),
				TimeSinceHeartbeat: "1s",
				TimeUntilShutdown:  "4m59s",
				Timeout:            "5m0s",
				GracePeriod:        "30s",
				IsTimedOut:         false,
				Running:            true,
			})
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	client := NewHeartbeatClient(server.URL)

	status, err := client.GetStatus()
	require.NoError(t, err)
	assert.Equal(t, "test-server", status.ServerID)
	assert.Equal(t, "5m0s", status.Timeout)
	assert.False(t, status.IsTimedOut)
	assert.True(t, status.Running)
}

func TestMonitor_HeartbeatResetsTimer(t *testing.T) {
	config := &Config{
		Port:    8080,
		Timeout: 200 * time.Millisecond,
	}
	m := NewMonitor(config, nil)
	m.RecordHeartbeat()

	// Wait for half the timeout
	time.Sleep(100 * time.Millisecond)
	assert.False(t, m.IsTimedOut(), "Should not be timed out yet")

	// Record a new heartbeat to reset the timer
	m.RecordHeartbeat()

	// Wait for half the timeout again
	time.Sleep(100 * time.Millisecond)
	assert.False(t, m.IsTimedOut(), "Should not be timed out because timer was reset")

	// Wait for full timeout from last heartbeat
	time.Sleep(150 * time.Millisecond)
	assert.True(t, m.IsTimedOut(), "Should be timed out now")
}
