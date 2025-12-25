package docker

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultHandlerConfig(t *testing.T) {
	config := DefaultHandlerConfig()
	assert.Equal(t, "/var/run/docker.sock", config.DockerSocketPath)
	assert.Equal(t, 2376, config.ListenPort)
}

func TestNewHandler(t *testing.T) {
	t.Run("with nil config uses defaults", func(t *testing.T) {
		h := NewHandler(nil, nil)
		require.NotNil(t, h)
		assert.Equal(t, "/var/run/docker.sock", h.config.DockerSocketPath)
		assert.Equal(t, 2376, h.config.ListenPort)
	})

	t.Run("with custom config", func(t *testing.T) {
		config := &HandlerConfig{
			DockerSocketPath: "/custom/docker.sock",
			ListenPort:       9999,
		}
		h := NewHandler(config, nil)
		require.NotNil(t, h)
		assert.Equal(t, "/custom/docker.sock", h.config.DockerSocketPath)
		assert.Equal(t, 9999, h.config.ListenPort)
	})
}

func TestHandler_IsRunning(t *testing.T) {
	h := NewHandler(nil, nil)
	assert.False(t, h.IsRunning())

	h.mu.Lock()
	h.running = true
	h.mu.Unlock()

	assert.True(t, h.IsRunning())
}

func TestHandler_GetStats(t *testing.T) {
	h := NewHandler(nil, nil)

	// Initial stats
	requests, errors := h.GetStats()
	assert.Equal(t, int64(0), requests)
	assert.Equal(t, int64(0), errors)

	// Update stats
	h.mu.Lock()
	h.requestCount = 100
	h.errorCount = 5
	h.mu.Unlock()

	requests, errors = h.GetStats()
	assert.Equal(t, int64(100), requests)
	assert.Equal(t, int64(5), errors)
}

func TestHandler_HandleHealth_NoSocket(t *testing.T) {
	// Test with non-existent socket
	config := &HandlerConfig{
		DockerSocketPath: "/nonexistent/docker.sock",
		ListenPort:       2376,
	}
	h := NewHandler(config, nil)

	req := httptest.NewRequest(http.MethodGet, "/_health", nil)
	rec := httptest.NewRecorder()

	h.handleHealth(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestHandler_HandleHealth_WithSocket(t *testing.T) {
	// Skip if Docker socket doesn't exist
	if _, err := net.DialTimeout("unix", "/var/run/docker.sock", 100); err != nil {
		t.Skip("Docker socket not available")
	}

	config := &HandlerConfig{
		DockerSocketPath: "/var/run/docker.sock",
		ListenPort:       2376,
	}
	h := NewHandler(config, nil)

	req := httptest.NewRequest(http.MethodGet, "/_health", nil)
	rec := httptest.NewRecorder()

	h.handleHealth(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "healthy")
}

func TestHandler_HandleStats(t *testing.T) {
	config := &HandlerConfig{
		DockerSocketPath: "/var/run/docker.sock",
		ListenPort:       2376,
	}
	h := NewHandler(config, nil)
	h.running = true
	h.requestCount = 42
	h.errorCount = 3

	req := httptest.NewRequest(http.MethodGet, "/_stats", nil)
	rec := httptest.NewRecorder()

	h.handleStats(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `"running":true`)
	assert.Contains(t, body, `"socket_path":"/var/run/docker.sock"`)
}

func TestBoolToString(t *testing.T) {
	assert.Equal(t, "true", boolToString(true))
	assert.Equal(t, "false", boolToString(false))
}
