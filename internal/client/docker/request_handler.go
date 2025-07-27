package docker

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dockbridge/dockbridge/pkg/logger"
	"github.com/pkg/errors"
)

// RequestHandler handles HTTP request processing and forwarding
type RequestHandler struct {
	connectionManager *ConnectionManager
	logger            logger.LoggerInterface
	// Circuit breaker fields
	connectionFailures int
	lastFailureTime    time.Time
}

// NewRequestHandler creates a new request handler
func NewRequestHandler(connectionManager *ConnectionManager, logger logger.LoggerInterface) *RequestHandler {
	return &RequestHandler{
		connectionManager: connectionManager,
		logger:            logger,
	}
}

// HandleDockerRequest processes incoming Docker API requests
func (rh *RequestHandler) HandleDockerRequest(w http.ResponseWriter, r *http.Request) {
	rh.logger.WithFields(map[string]interface{}{
		"method": r.Method,
		"path":   r.URL.Path,
		"remote": r.RemoteAddr,
	}).Info("Handling Docker request")

	// Check circuit breaker - if we've had too many consecutive failures recently, fail fast
	if rh.connectionFailures >= 3 && time.Since(rh.lastFailureTime) < 30*time.Second {
		rh.logger.WithFields(map[string]interface{}{
			"failures":          rh.connectionFailures,
			"last_failure_time": rh.lastFailureTime,
		}).Error("Circuit breaker open - too many connection failures")
		http.Error(w, "Service temporarily unavailable due to connection issues. Please check your SSH key and server configuration.", http.StatusServiceUnavailable)
		return
	}

	// Ensure we have a connection to a remote server
	if err := rh.connectionManager.EnsureRemoteConnection(r.Context()); err != nil {
		// Get current provisioning state for better error messages
		state, lastError, retryCount := rh.connectionManager.GetProvisioningState()

		// Don't increment failure counter for provisioning-in-progress states
		if state != StateProvisioning {
			rh.connectionFailures++
			rh.lastFailureTime = time.Now()
		}

		rh.logger.WithFields(map[string]interface{}{
			"error":              err.Error(),
			"failures":           rh.connectionFailures,
			"provisioning_state": state,
			"retry_count":        retryCount,
		}).Error("Failed to ensure remote connection")

		// Provide contextual error messages based on state and error type
		if state == StateProvisioning {
			// For Docker ping requests, provide a more appropriate response
			if r.URL.Path == "/_ping" {
				http.Error(w, "🚀 Server provisioning in progress...", http.StatusServiceUnavailable)
			} else {
				http.Error(w, "🚀 Provisioning Hetzner server for Docker operations. This may take 2-5 minutes on first use. Please wait...", http.StatusAccepted)
			}
		} else if strings.Contains(err.Error(), "waiting for retry backoff") {
			http.Error(w, fmt.Sprintf("⏳ Retrying server connection (attempt %d). Please wait a moment before trying again.", retryCount), http.StatusServiceUnavailable)
		} else if strings.Contains(err.Error(), "SSH private key not found") {
			http.Error(w, fmt.Sprintf("🔑 SSH key configuration error: %v", err), http.StatusServiceUnavailable)
		} else if strings.Contains(err.Error(), "context canceled") || strings.Contains(err.Error(), "timeout") {
			http.Error(w, "⏰ Server provisioning is taking longer than expected. Please try again in a few minutes.", http.StatusServiceUnavailable)
		} else if strings.Contains(err.Error(), "failed to provision server") {
			http.Error(w, fmt.Sprintf("☁️ Failed to provision Hetzner server: %v. Please check your API token and try again.", lastError), http.StatusServiceUnavailable)
		} else {
			http.Error(w, fmt.Sprintf("🔌 Failed to connect to remote server: %v", err), http.StatusServiceUnavailable)
		}
		return
	}

	// Reset failure counter on successful connection
	rh.connectionFailures = 0

	// Forward the request
	if err := rh.forwardRequest(w, r); err != nil {
		rh.logger.WithFields(map[string]interface{}{
			"method": r.Method,
			"path":   r.URL.Path,
			"error":  err.Error(),
		}).Error("Failed to forward request")
		http.Error(w, fmt.Sprintf("Failed to forward request: %v", err), http.StatusBadGateway)
		return
	}
}

// forwardRequest forwards the HTTP request through the SSH tunnel
func (rh *RequestHandler) forwardRequest(w http.ResponseWriter, r *http.Request) error {
	tunnel := rh.connectionManager.GetTunnel()
	if tunnel == nil {
		return errors.New("no SSH tunnel available")
	}

	// Create target URL using the tunnel's local address
	targetURL := &url.URL{
		Scheme:   "http",
		Host:     tunnel.LocalAddr(),
		Path:     r.URL.Path,
		RawQuery: r.URL.RawQuery,
	}

	// Create new request
	proxyReq, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL.String(), r.Body)
	if err != nil {
		return errors.Wrap(err, "failed to create proxy request")
	}

	// Copy headers
	for name, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(name, value)
		}
	}

	// Execute request through connection pool
	httpClient := rh.connectionManager.GetHTTPClient()
	resp, err := httpClient.Do(proxyReq)
	if err != nil {
		return errors.Wrap(err, "failed to execute proxy request")
	}
	defer resp.Body.Close()

	// Copy response headers
	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	// Set status code
	w.WriteHeader(resp.StatusCode)

	// Handle streaming responses
	if rh.isStreamingResponse(resp) {
		return rh.handleStreamingResponse(w, resp)
	}

	// Copy response body
	_, err = io.Copy(w, resp.Body)
	return err
}

// ForwardRequest forwards a single HTTP request (for testing/external use)
func (rh *RequestHandler) ForwardRequest(req *http.Request) (*http.Response, error) {
	// Ensure remote connection
	if err := rh.connectionManager.EnsureRemoteConnection(req.Context()); err != nil {
		return nil, errors.Wrap(err, "failed to ensure remote connection")
	}

	tunnel := rh.connectionManager.GetTunnel()
	if tunnel == nil {
		return nil, errors.New("no SSH tunnel available")
	}

	// Create target URL
	targetURL := &url.URL{
		Scheme:   "http",
		Host:     tunnel.LocalAddr(),
		Path:     req.URL.Path,
		RawQuery: req.URL.RawQuery,
	}

	// Create proxy request
	proxyReq, err := http.NewRequestWithContext(req.Context(), req.Method, targetURL.String(), req.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create proxy request")
	}

	// Copy headers
	for name, values := range req.Header {
		for _, value := range values {
			proxyReq.Header.Add(name, value)
		}
	}

	// Execute request
	httpClient := rh.connectionManager.GetHTTPClient()
	return httpClient.Do(proxyReq)
}

// isStreamingResponse determines if the response should be streamed
func (rh *RequestHandler) isStreamingResponse(resp *http.Response) bool {
	contentType := resp.Header.Get("Content-Type")

	// Check for streaming content types
	streamingTypes := []string{
		"application/vnd.docker.raw-stream",
		"application/vnd.docker.multiplexed-stream",
		"text/plain", // Docker logs often use text/plain
	}

	for _, streamType := range streamingTypes {
		if strings.Contains(contentType, streamType) {
			return true
		}
	}

	// Check for chunked transfer encoding
	if resp.Header.Get("Transfer-Encoding") == "chunked" {
		return true
	}

	return false
}

// handleStreamingResponse handles streaming responses like Docker logs, image pulls, etc.
func (rh *RequestHandler) handleStreamingResponse(w http.ResponseWriter, resp *http.Response) error {
	// Ensure we can flush the response
	flusher, ok := w.(http.Flusher)
	if !ok {
		return errors.New("response writer does not support flushing")
	}

	// Buffer for streaming
	buffer := make([]byte, 32*1024) // 32KB buffer

	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			if _, writeErr := w.Write(buffer[:n]); writeErr != nil {
				return errors.Wrap(writeErr, "failed to write streaming response")
			}
			flusher.Flush()
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.Wrap(err, "failed to read streaming response")
		}
	}

	return nil
}
