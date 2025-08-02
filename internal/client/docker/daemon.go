package docker

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dockbridge/dockbridge/internal/client/hetzner"
	"github.com/dockbridge/dockbridge/internal/shared/config"
	"github.com/dockbridge/dockbridge/pkg/logger"
	"github.com/pkg/errors"
)

// DockBridgeDaemon represents the main DockBridge daemon that replaces Docker daemon
type DockBridgeDaemon struct {
	config        *DaemonConfig
	server        *http.Server
	listener      net.Listener
	running       bool
	mu            sync.RWMutex
	logger        logger.LoggerInterface
	clientManager DockerClientManager
}

// DaemonConfig holds configuration for the DockBridge daemon
type DaemonConfig struct {
	SocketPath    string
	HetznerClient hetzner.HetznerClient
	SSHConfig     *config.SSHConfig
	HetznerConfig *config.HetznerConfig
	Logger        logger.LoggerInterface
}

// NewDockBridgeDaemon creates a new DockBridge daemon
func NewDockBridgeDaemon() *DockBridgeDaemon {
	return &DockBridgeDaemon{}
}

// Start starts the DockBridge daemon
func (d *DockBridgeDaemon) Start(ctx context.Context, config *DaemonConfig) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.running {
		return errors.New("daemon is already running")
	}

	if config == nil {
		return errors.New("daemon config cannot be nil")
	}

	d.config = config
	d.logger = config.Logger

	// Initialize components
	if err := d.initializeComponents(); err != nil {
		return errors.Wrap(err, "failed to initialize components")
	}

	// Set up the HTTP server that mimics Docker daemon API
	if err := d.setupServer(); err != nil {
		return errors.Wrap(err, "failed to setup HTTP server")
	}

	// Start the server in a goroutine
	go func() {
		d.logger.WithFields(map[string]interface{}{
			"socket_path": d.config.SocketPath,
		}).Info("Starting DockBridge daemon")

		if err := d.server.Serve(d.listener); err != nil && err != http.ErrServerClosed {
			d.logger.WithFields(map[string]interface{}{
				"error": err.Error(),
			}).Error("DockBridge daemon server error")
		}
	}()

	d.running = true
	d.logger.Info("DockBridge daemon started successfully")
	return nil
}

// Stop gracefully shuts down the DockBridge daemon
func (d *DockBridgeDaemon) Stop() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.running {
		return nil
	}

	d.logger.Info("Stopping DockBridge daemon")

	// Close client manager
	if d.clientManager != nil {
		if err := d.clientManager.Close(); err != nil {
			d.logger.WithFields(map[string]interface{}{
				"error": err.Error(),
			}).Error("Failed to close client manager")
		}
	}

	// Shutdown HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := d.server.Shutdown(ctx); err != nil {
		d.logger.WithFields(map[string]interface{}{
			"error": err.Error(),
		}).Error("Failed to shutdown HTTP server gracefully")
		return err
	}

	d.running = false
	d.logger.Info("DockBridge daemon stopped successfully")
	return nil
}

// IsRunning returns true if the daemon is currently running
func (d *DockBridgeDaemon) IsRunning() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.running
}

// initializeComponents sets up the internal components
func (d *DockBridgeDaemon) initializeComponents() error {
	// Create Docker client manager
	d.clientManager = NewDockerClientManager(
		d.config.HetznerClient,
		d.config.SSHConfig,
		d.config.HetznerConfig,
		d.logger,
	)
	return nil
}

// setupServer configures the HTTP server that mimics Docker daemon
func (d *DockBridgeDaemon) setupServer() error {
	// Create listener for Unix socket
	var err error
	if strings.HasPrefix(d.config.SocketPath, "/") {
		// Unix socket
		d.listener, err = net.Listen("unix", d.config.SocketPath)
		if err != nil {
			return errors.Wrap(err, "failed to create listener")
		}

		// Set proper permissions for the socket
		if err := d.setSocketPermissions(d.config.SocketPath); err != nil {
			d.logger.WithFields(map[string]interface{}{
				"socket_path": d.config.SocketPath,
				"error":       err.Error(),
			}).Warn("Failed to set socket permissions")
		}
	} else {
		return errors.New("only Unix socket paths are supported for daemon mode")
	}

	// Create HTTP server with Docker API handler
	mux := http.NewServeMux()
	mux.HandleFunc("/", d.handleDockerAPIRequest)

	d.server = &http.Server{
		Handler: mux,
	}

	return nil
}

// handleDockerAPIRequest handles all Docker API requests using Docker Go client
func (d *DockBridgeDaemon) handleDockerAPIRequest(w http.ResponseWriter, r *http.Request) {
	d.logger.WithFields(map[string]interface{}{
		"method": r.Method,
		"path":   r.URL.Path,
		"remote": r.RemoteAddr,
	}).Info("🐳 Docker API request via Go client")

	// Get Docker client from manager
	dockerClient, err := d.clientManager.GetClient(r.Context())
	if err != nil {
		d.logger.WithFields(map[string]interface{}{
			"error": err.Error(),
		}).Error("Failed to get Docker client")

		// Provide user-friendly error messages
		if strings.Contains(err.Error(), "failed to ensure connection") {
			http.Error(w, "🚀 Provisioning remote server, please wait...", http.StatusServiceUnavailable)
		} else {
			http.Error(w, fmt.Sprintf("Failed to connect to remote Docker: %v", err), http.StatusServiceUnavailable)
		}
		return
	}

	// Create a reverse proxy that forwards requests to the Docker client's HTTP client
	// This allows us to use the Docker client's connection while maintaining the HTTP API
	transport := dockerClient.HTTPClient().Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	// Create target URL pointing to the Docker daemon
	targetURL := *r.URL
	targetURL.Scheme = "http"
	targetURL.Host = "localhost:2376" // This will be overridden by our custom transport

	// Create proxy request
	proxyReq, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL.String(), r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create proxy request: %v", err), http.StatusInternalServerError)
		return
	}

	// Copy headers
	for name, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(name, value)
		}
	}

	// Special handling for container attach operations
	if d.isAttachRequest(r) {
		d.handleAttachRequest(w, proxyReq, transport)
		return
	}

	// Execute request using Docker client's transport
	resp, err := transport.RoundTrip(proxyReq)
	if err != nil {
		d.logger.WithFields(map[string]interface{}{
			"error": err.Error(),
		}).Error("Failed to execute request through Docker client")
		http.Error(w, fmt.Sprintf("Failed to execute request: %v", err), http.StatusBadGateway)
		return
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

	// Copy response body with streaming support
	d.copyResponseWithStreaming(w, resp, r)
}

// setSocketPermissions sets the correct permissions for the Docker socket
func (d *DockBridgeDaemon) setSocketPermissions(socketPath string) error {
	// Set socket permissions to 666 (rw-rw-rw-)
	if err := os.Chmod(socketPath, 0666); err != nil {
		return errors.Wrap(err, "failed to set socket permissions")
	}

	// Try to get the appropriate group for Docker socket
	var targetGroup *user.Group
	var err error

	// On macOS, try daemon group first, then docker group on Linux
	groupNames := []string{"daemon", "docker"}

	for _, groupName := range groupNames {
		targetGroup, err = user.LookupGroup(groupName)
		if err == nil {
			break
		}
	}

	if err != nil {
		// If neither group exists, try to create docker group (Linux)
		cmd := exec.Command("groupadd", "-f", "docker")
		if err := cmd.Run(); err != nil {
			return nil // Don't fail, socket is already 666
		}

		targetGroup, err = user.LookupGroup("docker")
		if err != nil {
			return nil // Don't fail, socket is already 666
		}
	}

	// Convert group ID to integer
	gid, err := strconv.Atoi(targetGroup.Gid)
	if err != nil {
		return errors.Wrap(err, "failed to parse group ID")
	}

	// Change group ownership of the socket
	if err := os.Chown(socketPath, -1, gid); err != nil {
		return nil // Don't fail, socket is already 666
	}

	d.logger.WithFields(map[string]interface{}{
		"socket_path": socketPath,
		"group":       targetGroup.Name,
		"gid":         gid,
		"permissions": "666",
	}).Info("Set socket permissions for group access")

	return nil
}

// copyResponseWithStreaming copies the response body with support for streaming
func (d *DockBridgeDaemon) copyResponseWithStreaming(w http.ResponseWriter, resp *http.Response, req *http.Request) {
	// Check if this is a streaming response
	isStreaming := d.isStreamingResponse(resp, req)

	d.logger.WithFields(map[string]interface{}{
		"content_type":      resp.Header.Get("Content-Type"),
		"transfer_encoding": resp.Header.Get("Transfer-Encoding"),
		"content_length":    resp.Header.Get("Content-Length"),
		"path":              req.URL.Path,
		"is_streaming":      isStreaming,
	}).Info("Copying response body")

	if isStreaming {
		d.copyStreamingResponse(w, resp)
	} else {
		d.copyRegularResponse(w, resp)
	}
}

// isStreamingResponse determines if the response should be streamed
func (d *DockBridgeDaemon) isStreamingResponse(resp *http.Response, req *http.Request) bool {
	// Check for streaming content types
	contentType := resp.Header.Get("Content-Type")
	streamingTypes := []string{
		"application/vnd.docker.raw-stream",
		"application/vnd.docker.multiplexed-stream",
		"text/plain",
	}

	for _, streamType := range streamingTypes {
		if strings.Contains(contentType, streamType) {
			d.logger.WithFields(map[string]interface{}{
				"matched_content_type": streamType,
				"path":                 req.URL.Path,
			}).Info("Detected streaming response by content type")
			return true
		}
	}

	// Check for chunked transfer encoding
	if resp.Header.Get("Transfer-Encoding") == "chunked" {
		d.logger.WithFields(map[string]interface{}{
			"path": req.URL.Path,
		}).Info("Detected streaming response by chunked encoding")
		return true
	}

	// Check for specific Docker API endpoints that should be streamed
	if req != nil {
		path := req.URL.Path

		// Container attach endpoints (docker run, docker exec)
		if strings.Contains(path, "/attach") {
			d.logger.WithFields(map[string]interface{}{
				"path": path,
			}).Info("Detected container attach endpoint - enabling streaming")
			return true
		}

		// Container logs endpoints
		if strings.Contains(path, "/logs") {
			d.logger.WithFields(map[string]interface{}{
				"path": path,
			}).Info("Detected container logs endpoint - enabling streaming")
			return true
		}

		// Container wait endpoints (docker run waits for container to finish)
		if strings.Contains(path, "/wait") {
			d.logger.WithFields(map[string]interface{}{
				"path": path,
			}).Info("Detected container wait endpoint - enabling streaming")
			return true
		}

		// Image pull/push endpoints
		if (strings.Contains(path, "/images/") && (strings.Contains(path, "/push") || strings.Contains(path, "/pull"))) ||
			strings.Contains(path, "/pull") || strings.Contains(path, "/push") {
			d.logger.WithFields(map[string]interface{}{
				"path": path,
			}).Info("Detected image push/pull endpoint - enabling streaming")
			return true
		}

		// Build endpoints
		if strings.Contains(path, "/build") {
			d.logger.WithFields(map[string]interface{}{
				"path": path,
			}).Info("Detected build endpoint - enabling streaming")
			return true
		}

		// Container create with attach (docker run)
		if strings.Contains(path, "/containers/create") && req.URL.RawQuery != "" {
			query := req.URL.Query()
			if query.Get("attach") != "" || query.Get("stream") != "" {
				d.logger.WithFields(map[string]interface{}{
					"path":  path,
					"query": req.URL.RawQuery,
				}).Info("Detected container create with attach - enabling streaming")
				return true
			}
		}
	}

	// If Content-Length is not set and it's a potentially streaming endpoint, treat as streaming
	if resp.Header.Get("Content-Length") == "" && resp.ContentLength == -1 {
		if req != nil {
			path := req.URL.Path
			if strings.Contains(path, "/attach") || strings.Contains(path, "/logs") ||
				strings.Contains(path, "/wait") || strings.Contains(path, "/pull") ||
				strings.Contains(path, "/push") || strings.Contains(path, "/build") {
				d.logger.WithFields(map[string]interface{}{
					"path": path,
				}).Info("No content-length and streaming endpoint - enabling streaming")
				return true
			}
		}
	}

	return false
}

// copyStreamingResponse handles streaming responses with immediate flushing
func (d *DockBridgeDaemon) copyStreamingResponse(w http.ResponseWriter, resp *http.Response) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		d.logger.Error("Response writer does not support flushing, falling back to regular copy")
		d.copyRegularResponse(w, resp)
		return
	}

	d.logger.WithFields(map[string]interface{}{
		"status_code":       resp.StatusCode,
		"content_type":      resp.Header.Get("Content-Type"),
		"transfer_encoding": resp.Header.Get("Transfer-Encoding"),
	}).Info("Starting streaming response copy")

	// Handle HTTP 101 Switching Protocols (WebSocket upgrade for attach)
	if resp.StatusCode == 101 {
		d.logger.Info("Handling HTTP 101 Switching Protocols for container attach")
		d.copyRawStream(w, resp, flusher)
		return
	}

	// Use smaller buffer for more responsive streaming
	buffer := make([]byte, 4*1024) // 4KB buffer for very responsive output
	totalBytes := 0
	chunkCount := 0

	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			totalBytes += n
			chunkCount++

			// Write the data
			written, writeErr := w.Write(buffer[:n])
			if writeErr != nil {
				d.logger.WithFields(map[string]interface{}{
					"error":          writeErr.Error(),
					"bytes_streamed": totalBytes,
					"chunk_count":    chunkCount,
				}).Error("Failed to write streaming response")
				return
			}

			// Flush immediately for real-time output
			flusher.Flush()

			// Log progress for first few chunks and periodically
			if chunkCount <= 5 || chunkCount%50 == 0 {
				d.logger.WithFields(map[string]interface{}{
					"chunk_size":    n,
					"bytes_written": written,
					"total_bytes":   totalBytes,
					"chunk_count":   chunkCount,
				}).Debug("Streamed data chunk")
			}
		}

		if err == io.EOF {
			d.logger.WithFields(map[string]interface{}{
				"total_bytes_streamed": totalBytes,
				"total_chunks":         chunkCount,
			}).Info("Streaming response completed successfully")
			break
		}

		if err != nil {
			d.logger.WithFields(map[string]interface{}{
				"error":                err.Error(),
				"total_bytes_streamed": totalBytes,
				"total_chunks":         chunkCount,
			}).Error("Error reading streaming response")
			return
		}
	}
}

// copyRawStream handles raw streaming for HTTP 101 responses (container attach)
func (d *DockBridgeDaemon) copyRawStream(w http.ResponseWriter, resp *http.Response, flusher http.Flusher) {
	d.logger.Info("Copying raw stream for container attach")

	// For HTTP 101, we need to copy the raw connection data
	buffer := make([]byte, 1024) // Smaller buffer for interactive responses
	totalBytes := 0

	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			totalBytes += n

			if _, writeErr := w.Write(buffer[:n]); writeErr != nil {
				d.logger.WithFields(map[string]interface{}{
					"error":        writeErr.Error(),
					"bytes_copied": totalBytes,
				}).Error("Failed to write raw stream")
				return
			}

			// Flush immediately for interactive responses
			flusher.Flush()

			// Log first chunk for debugging
			if totalBytes <= n {
				preview := string(buffer[:min(n, 50)])
				d.logger.WithFields(map[string]interface{}{
					"first_chunk_size": n,
					"preview":          preview,
				}).Debug("First chunk of raw stream")
			}
		}

		if err == io.EOF {
			d.logger.WithFields(map[string]interface{}{
				"total_bytes": totalBytes,
			}).Info("Raw stream completed")
			break
		}

		if err != nil {
			d.logger.WithFields(map[string]interface{}{
				"error":       err.Error(),
				"total_bytes": totalBytes,
			}).Error("Error reading raw stream")
			return
		}
	}
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// copyRegularResponse handles regular responses
func (d *DockBridgeDaemon) copyRegularResponse(w http.ResponseWriter, resp *http.Response) {
	written, err := io.Copy(w, resp.Body)
	if err != nil {
		d.logger.WithFields(map[string]interface{}{
			"error":         err.Error(),
			"bytes_written": written,
		}).Error("Failed to copy response body")
	} else {
		d.logger.WithFields(map[string]interface{}{
			"bytes_written": written,
		}).Debug("Response body copied successfully")
	}
}

// isAttachRequest checks if this is a container attach request
func (d *DockBridgeDaemon) isAttachRequest(r *http.Request) bool {
	path := r.URL.Path

	// Check for attach endpoints
	if strings.Contains(path, "/attach") {
		return true
	}

	// Check for exec endpoints with attach
	if strings.Contains(path, "/exec/") && strings.Contains(path, "/start") {
		return true
	}

	return false
}

// handleAttachRequest handles container attach requests with proper TTY support
func (d *DockBridgeDaemon) handleAttachRequest(w http.ResponseWriter, proxyReq *http.Request, transport http.RoundTripper) {
	d.logger.WithFields(map[string]interface{}{
		"path":   proxyReq.URL.Path,
		"method": proxyReq.Method,
		"query":  proxyReq.URL.RawQuery,
	}).Info("Handling container attach request with TTY support")

	// Execute the attach request
	resp, err := transport.RoundTrip(proxyReq)
	if err != nil {
		d.logger.WithFields(map[string]interface{}{
			"error": err.Error(),
		}).Error("Failed to execute attach request")
		http.Error(w, fmt.Sprintf("Failed to execute attach request: %v", err), http.StatusBadGateway)
		return
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

	d.logger.WithFields(map[string]interface{}{
		"status_code":       resp.StatusCode,
		"content_type":      resp.Header.Get("Content-Type"),
		"transfer_encoding": resp.Header.Get("Transfer-Encoding"),
		"upgrade":           resp.Header.Get("Upgrade"),
		"connection":        resp.Header.Get("Connection"),
	}).Info("Attach response headers set, starting stream copy")

	// Handle the attach stream with special care for TTY
	d.handleAttachStream(w, resp, proxyReq)
}

// handleAttachStream handles the streaming for container attach operations
func (d *DockBridgeDaemon) handleAttachStream(w http.ResponseWriter, resp *http.Response, req *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		d.logger.Error("Response writer does not support flushing for attach stream")
		d.copyRegularResponse(w, resp)
		return
	}

	// Check if this is a TTY attach (no multiplexing)
	query := req.URL.Query()
	isTTY := query.Get("tty") == "true" || query.Get("tty") == "1"

	d.logger.WithFields(map[string]interface{}{
		"is_tty":       isTTY,
		"status_code":  resp.StatusCode,
		"content_type": resp.Header.Get("Content-Type"),
	}).Info("Starting attach stream copy")

	if resp.StatusCode == 101 {
		// HTTP 101 Switching Protocols - raw stream
		d.copyAttachRawStream(w, resp, flusher, isTTY)
	} else if isTTY {
		// TTY mode - raw stream without multiplexing
		d.copyAttachTTYStream(w, resp, flusher)
	} else {
		// Non-TTY mode - may be multiplexed stream
		d.copyAttachMultiplexedStream(w, resp, flusher)
	}
}

// copyAttachRawStream handles raw attach streams (HTTP 101)
func (d *DockBridgeDaemon) copyAttachRawStream(w http.ResponseWriter, resp *http.Response, flusher http.Flusher, isTTY bool) {
	d.logger.WithFields(map[string]interface{}{
		"is_tty": isTTY,
	}).Info("Copying raw attach stream (HTTP 101)")

	// Use very small buffer for interactive responses
	buffer := make([]byte, 512)
	totalBytes := 0

	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			totalBytes += n

			if _, writeErr := w.Write(buffer[:n]); writeErr != nil {
				d.logger.WithFields(map[string]interface{}{
					"error":        writeErr.Error(),
					"bytes_copied": totalBytes,
				}).Error("Failed to write raw attach stream")
				return
			}

			// Flush immediately for interactive responses
			flusher.Flush()
		}

		if err == io.EOF {
			d.logger.WithFields(map[string]interface{}{
				"total_bytes": totalBytes,
			}).Info("Raw attach stream completed")
			break
		}

		if err != nil {
			d.logger.WithFields(map[string]interface{}{
				"error":       err.Error(),
				"total_bytes": totalBytes,
			}).Error("Error reading raw attach stream")
			return
		}
	}
}

// copyAttachTTYStream handles TTY attach streams (no multiplexing)
func (d *DockBridgeDaemon) copyAttachTTYStream(w http.ResponseWriter, resp *http.Response, flusher http.Flusher) {
	d.logger.Info("Copying TTY attach stream (no multiplexing)")

	// Use very small buffer for character-by-character TTY output
	buffer := make([]byte, 256)
	totalBytes := 0

	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			totalBytes += n

			if _, writeErr := w.Write(buffer[:n]); writeErr != nil {
				d.logger.WithFields(map[string]interface{}{
					"error":        writeErr.Error(),
					"bytes_copied": totalBytes,
				}).Error("Failed to write TTY attach stream")
				return
			}

			// Flush immediately for TTY responses
			flusher.Flush()
		}

		if err == io.EOF {
			d.logger.WithFields(map[string]interface{}{
				"total_bytes": totalBytes,
			}).Info("TTY attach stream completed")
			break
		}

		if err != nil {
			d.logger.WithFields(map[string]interface{}{
				"error":       err.Error(),
				"total_bytes": totalBytes,
			}).Error("Error reading TTY attach stream")
			return
		}
	}
}

// copyAttachMultiplexedStream handles multiplexed attach streams (stdout/stderr separated)
func (d *DockBridgeDaemon) copyAttachMultiplexedStream(w http.ResponseWriter, resp *http.Response, flusher http.Flusher) {
	d.logger.Info("Copying multiplexed attach stream (stdout/stderr)")

	// For multiplexed streams, we still copy raw bytes but with larger buffer
	// The Docker client will handle demultiplexing on the client side
	buffer := make([]byte, 2048)
	totalBytes := 0

	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			totalBytes += n

			if _, writeErr := w.Write(buffer[:n]); writeErr != nil {
				d.logger.WithFields(map[string]interface{}{
					"error":        writeErr.Error(),
					"bytes_copied": totalBytes,
				}).Error("Failed to write multiplexed attach stream")
				return
			}

			// Flush for real-time output
			flusher.Flush()
		}

		if err == io.EOF {
			d.logger.WithFields(map[string]interface{}{
				"total_bytes": totalBytes,
			}).Info("Multiplexed attach stream completed")
			break
		}

		if err != nil {
			d.logger.WithFields(map[string]interface{}{
				"error":       err.Error(),
				"total_bytes": totalBytes,
			}).Error("Error reading multiplexed attach stream")
			return
		}
	}
}
