// Package docker provides Docker API handling for the DockBridge server.
// This handler forwards Docker API requests from the local Docker socket.
package docker

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"sync"
	"time"

	"github.com/dockbridge/dockbridge/pkg/logger"
)

// HandlerConfig holds configuration for the Docker API handler.
type HandlerConfig struct {
	// DockerSocketPath is the path to the Docker daemon socket.
	DockerSocketPath string `json:"docker_socket_path" yaml:"docker_socket_path"`

	// ListenPort is the port to listen on for API requests.
	ListenPort int `json:"listen_port" yaml:"listen_port"`

	// ReadTimeout is the maximum duration for reading the entire request.
	ReadTimeout time.Duration `json:"read_timeout" yaml:"read_timeout"`

	// WriteTimeout is the maximum duration before timing out writes of the response.
	WriteTimeout time.Duration `json:"write_timeout" yaml:"write_timeout"`
}

// DefaultHandlerConfig returns the default Docker handler configuration.
func DefaultHandlerConfig() *HandlerConfig {
	return &HandlerConfig{
		DockerSocketPath: "/var/run/docker.sock",
		ListenPort:       2376,
		ReadTimeout:      60 * time.Second,
		WriteTimeout:     60 * time.Second,
	}
}

// Handler handles Docker API requests by proxying them to the local Docker daemon.
type Handler struct {
	config  *HandlerConfig
	logger  logger.LoggerInterface
	proxy   *httputil.ReverseProxy
	server  *http.Server
	running bool
	mu      sync.RWMutex

	// Stats
	requestCount int64
	errorCount   int64
}

// NewHandler creates a new Docker API handler.
func NewHandler(config *HandlerConfig, log logger.LoggerInterface) *Handler {
	if config == nil {
		config = DefaultHandlerConfig()
	}
	if log == nil {
		log = logger.NewDefault()
	}

	h := &Handler{
		config: config,
		logger: log,
	}

	// Create reverse proxy for Docker socket
	h.proxy = &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = "localhost"
		},
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.Dial("unix", config.DockerSocketPath)
			},
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			h.logger.Error("Proxy error", "error", err, "path", r.URL.Path)
			h.mu.Lock()
			h.errorCount++
			h.mu.Unlock()
			http.Error(w, "Docker daemon unavailable", http.StatusBadGateway)
		},
	}

	return h
}

// Start begins serving Docker API requests.
func (h *Handler) Start(ctx context.Context) error {
	h.mu.Lock()
	if h.running {
		h.mu.Unlock()
		return nil
	}
	h.running = true
	h.mu.Unlock()

	mux := http.NewServeMux()

	// Proxy all Docker API requests
	mux.HandleFunc("/", h.handleRequest)

	// Add health endpoint
	mux.HandleFunc("/_health", h.handleHealth)
	mux.HandleFunc("/_stats", h.handleStats)

	h.server = &http.Server{
		Addr:         ":" + string(rune(h.config.ListenPort)),
		Handler:      mux,
		ReadTimeout:  h.config.ReadTimeout,
		WriteTimeout: h.config.WriteTimeout,
	}

	// Start in a goroutine
	go func() {
		h.logger.Info("Docker API handler starting", "port", h.config.ListenPort)
		if err := h.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			h.logger.Error("Docker API handler error", "error", err)
		}
	}()

	return nil
}

// Stop gracefully stops the Docker API handler.
func (h *Handler) Stop() error {
	h.mu.Lock()
	if !h.running {
		h.mu.Unlock()
		return nil
	}
	h.running = false
	h.mu.Unlock()

	if h.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return h.server.Shutdown(ctx)
	}

	return nil
}

// handleRequest proxies Docker API requests to the local daemon.
func (h *Handler) handleRequest(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	h.requestCount++
	h.mu.Unlock()

	h.logger.Debug("Docker API request",
		"method", r.Method,
		"path", r.URL.Path,
		"remote_addr", r.RemoteAddr,
	)

	h.proxy.ServeHTTP(w, r)
}

// handleHealth returns a simple health check.
func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	// Check if Docker socket is accessible
	conn, err := net.DialTimeout("unix", h.config.DockerSocketPath, 2*time.Second)
	if err != nil {
		http.Error(w, "Docker daemon unavailable", http.StatusServiceUnavailable)
		return
	}
	conn.Close()

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"healthy","docker":"connected"}`))
}

// handleStats returns handler statistics.
func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	stats := struct {
		RequestCount int64  `json:"request_count"`
		ErrorCount   int64  `json:"error_count"`
		Running      bool   `json:"running"`
		SocketPath   string `json:"socket_path"`
	}{
		RequestCount: h.requestCount,
		ErrorCount:   h.errorCount,
		Running:      h.running,
		SocketPath:   h.config.DockerSocketPath,
	}
	h.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	io.WriteString(w, `{"request_count":`+string(rune(stats.RequestCount))+
		`,"error_count":`+string(rune(stats.ErrorCount))+
		`,"running":`+boolToString(stats.Running)+
		`,"socket_path":"`+stats.SocketPath+`"}`)
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// IsRunning returns whether the handler is running.
func (h *Handler) IsRunning() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.running
}

// GetStats returns the current handler statistics.
func (h *Handler) GetStats() (requests, errors int64) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.requestCount, h.errorCount
}
