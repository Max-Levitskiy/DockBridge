package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestConfig_Validate(t *testing.T) {
	// Create a temporary SSH key file for testing
	tmpFile, err := os.CreateTemp("", "test_ssh_key")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errType string
	}{
		{
			name: "valid configuration",
			config: Config{
				LocalSocket:  "/tmp/test.sock",
				SSHUser:      "testuser",
				SSHHost:      "testhost",
				SSHKeyPath:   tmpFile.Name(),
				RemoteSocket: "/var/run/docker.sock",
				Timeout:      10 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "missing SSH user",
			config: Config{
				LocalSocket:  "/tmp/test.sock",
				SSHHost:      "testhost",
				SSHKeyPath:   tmpFile.Name(),
				RemoteSocket: "/var/run/docker.sock",
			},
			wantErr: true,
			errType: ErrorCategoryConfig,
		},
		{
			name: "missing SSH host",
			config: Config{
				LocalSocket:  "/tmp/test.sock",
				SSHUser:      "testuser",
				SSHKeyPath:   tmpFile.Name(),
				RemoteSocket: "/var/run/docker.sock",
			},
			wantErr: true,
			errType: ErrorCategoryConfig,
		},
		{
			name: "missing SSH key path",
			config: Config{
				LocalSocket:  "/tmp/test.sock",
				SSHUser:      "testuser",
				SSHHost:      "testhost",
				RemoteSocket: "/var/run/docker.sock",
			},
			wantErr: true,
			errType: ErrorCategoryConfig,
		},
		{
			name: "non-existent SSH key file",
			config: Config{
				LocalSocket:  "/tmp/test.sock",
				SSHUser:      "testuser",
				SSHHost:      "testhost",
				SSHKeyPath:   "/non/existent/key",
				RemoteSocket: "/var/run/docker.sock",
			},
			wantErr: true,
			errType: ErrorCategoryConfig,
		},
		{
			name: "missing local socket",
			config: Config{
				SSHUser:      "testuser",
				SSHHost:      "testhost",
				SSHKeyPath:   tmpFile.Name(),
				RemoteSocket: "/var/run/docker.sock",
			},
			wantErr: true,
			errType: ErrorCategoryConfig,
		},
		{
			name: "default remote socket",
			config: Config{
				LocalSocket: "/tmp/test.sock",
				SSHUser:     "testuser",
				SSHHost:     "testhost",
				SSHKeyPath:  tmpFile.Name(),
				// RemoteSocket not set - should default
			},
			wantErr: false,
		},
		{
			name: "default timeout",
			config: Config{
				LocalSocket:  "/tmp/test.sock",
				SSHUser:      "testuser",
				SSHHost:      "testhost",
				SSHKeyPath:   tmpFile.Name(),
				RemoteSocket: "/var/run/docker.sock",
				// Timeout not set - should default
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.wantErr {
				if err == nil {
					t.Errorf("Config.Validate() expected error, got nil")
					return
				}

				if proxyErr, ok := err.(*ProxyError); ok {
					if proxyErr.Category != tt.errType {
						t.Errorf("Config.Validate() error category = %v, want %v", proxyErr.Category, tt.errType)
					}
				} else {
					t.Errorf("Config.Validate() expected ProxyError, got %T", err)
				}
			} else {
				if err != nil {
					t.Errorf("Config.Validate() unexpected error: %v", err)
				}

				// Check defaults were set
				if tt.config.RemoteSocket == "" && tt.config.RemoteSocket != "/var/run/docker.sock" {
					t.Errorf("Config.Validate() did not set default RemoteSocket")
				}
				if tt.config.Timeout == 0 && tt.config.Timeout != 10*time.Second {
					t.Errorf("Config.Validate() did not set default Timeout")
				}
			}
		})
	}
}

func TestProxyError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  ProxyError
		want string
	}{
		{
			name: "error without cause",
			err: ProxyError{
				Category: ErrorCategoryConfig,
				Message:  "test message",
			},
			want: "[CONFIG] test message",
		},
		{
			name: "error with cause",
			err: ProxyError{
				Category: ErrorCategorySSH,
				Message:  "test message",
				Cause:    os.ErrNotExist,
			},
			want: "[SSH] test message: file does not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("ProxyError.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.RemoteSocket != "/var/run/docker.sock" {
		t.Errorf("DefaultConfig() RemoteSocket = %v, want %v", config.RemoteSocket, "/var/run/docker.sock")
	}

	if config.Timeout != 10*time.Second {
		t.Errorf("DefaultConfig() Timeout = %v, want %v", config.Timeout, 10*time.Second)
	}

	// Other fields should be empty
	if config.LocalSocket != "" {
		t.Errorf("DefaultConfig() LocalSocket should be empty, got %v", config.LocalSocket)
	}
	if config.SSHUser != "" {
		t.Errorf("DefaultConfig() SSHUser should be empty, got %v", config.SSHUser)
	}
	if config.SSHHost != "" {
		t.Errorf("DefaultConfig() SSHHost should be empty, got %v", config.SSHHost)
	}
	if config.SSHKeyPath != "" {
		t.Errorf("DefaultConfig() SSHKeyPath should be empty, got %v", config.SSHKeyPath)
	}
}

func TestLoadFromFile(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a temporary SSH key file
	keyFile := filepath.Join(tmpDir, "test_key")
	if err := os.WriteFile(keyFile, []byte("test key content"), 0600); err != nil {
		t.Fatalf("Failed to create key file: %v", err)
	}

	tests := []struct {
		name     string
		content  string
		filename string
		want     *Config
		wantErr  bool
		errType  string
	}{
		{
			name: "valid YAML configuration",
			content: `
local_socket: /tmp/test.sock
ssh_user: testuser
ssh_host: testhost:2222
ssh_key_path: ` + keyFile + `
remote_socket: /custom/docker.sock
timeout: 30s
`,
			filename: "valid.yaml",
			want: &Config{
				LocalSocket:  "/tmp/test.sock",
				SSHUser:      "testuser",
				SSHHost:      "testhost:2222",
				SSHKeyPath:   keyFile,
				RemoteSocket: "/custom/docker.sock",
				Timeout:      30 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "partial configuration with defaults",
			content: `
local_socket: /tmp/test.sock
ssh_user: testuser
ssh_host: testhost
ssh_key_path: ` + keyFile + `
`,
			filename: "partial.yaml",
			want: &Config{
				LocalSocket:  "/tmp/test.sock",
				SSHUser:      "testuser",
				SSHHost:      "testhost",
				SSHKeyPath:   keyFile,
				RemoteSocket: "/var/run/docker.sock", // default
				Timeout:      10 * time.Second,       // default
			},
			wantErr: false,
		},
		{
			name:     "non-existent file",
			filename: "nonexistent.yaml",
			wantErr:  true,
			errType:  ErrorCategoryConfig,
		},
		{
			name: "invalid YAML",
			content: `
local_socket: /tmp/test.sock
ssh_user: testuser
invalid_yaml: [unclosed
`,
			filename: "invalid.yaml",
			wantErr:  true,
			errType:  ErrorCategoryConfig,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var configPath string
			if tt.content != "" {
				configPath = filepath.Join(tmpDir, tt.filename)
				if err := os.WriteFile(configPath, []byte(tt.content), 0644); err != nil {
					t.Fatalf("Failed to create config file: %v", err)
				}
			} else {
				configPath = filepath.Join(tmpDir, tt.filename)
			}

			got, err := LoadFromFile(configPath)

			if tt.wantErr {
				if err == nil {
					t.Errorf("LoadFromFile() expected error, got nil")
					return
				}

				if proxyErr, ok := err.(*ProxyError); ok {
					if proxyErr.Category != tt.errType {
						t.Errorf("LoadFromFile() error category = %v, want %v", proxyErr.Category, tt.errType)
					}
				} else {
					t.Errorf("LoadFromFile() expected ProxyError, got %T", err)
				}
				return
			}

			if err != nil {
				t.Errorf("LoadFromFile() unexpected error: %v", err)
				return
			}

			if got.LocalSocket != tt.want.LocalSocket {
				t.Errorf("LoadFromFile() LocalSocket = %v, want %v", got.LocalSocket, tt.want.LocalSocket)
			}
			if got.SSHUser != tt.want.SSHUser {
				t.Errorf("LoadFromFile() SSHUser = %v, want %v", got.SSHUser, tt.want.SSHUser)
			}
			if got.SSHHost != tt.want.SSHHost {
				t.Errorf("LoadFromFile() SSHHost = %v, want %v", got.SSHHost, tt.want.SSHHost)
			}
			if got.SSHKeyPath != tt.want.SSHKeyPath {
				t.Errorf("LoadFromFile() SSHKeyPath = %v, want %v", got.SSHKeyPath, tt.want.SSHKeyPath)
			}
			if got.RemoteSocket != tt.want.RemoteSocket {
				t.Errorf("LoadFromFile() RemoteSocket = %v, want %v", got.RemoteSocket, tt.want.RemoteSocket)
			}
			if got.Timeout != tt.want.Timeout {
				t.Errorf("LoadFromFile() Timeout = %v, want %v", got.Timeout, tt.want.Timeout)
			}
		})
	}
}

func TestLoadFromFlags(t *testing.T) {
	// Create a temporary SSH key file
	tmpFile, err := os.CreateTemp("", "test_ssh_key")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	tests := []struct {
		name    string
		args    []string
		want    *Config
		wantErr bool
		errType string
	}{
		{
			name: "all flags provided",
			args: []string{
				"-local-socket", "/tmp/test.sock",
				"-ssh-user", "testuser",
				"-ssh-host", "testhost:2222",
				"-ssh-key", tmpFile.Name(),
				"-remote-socket", "/custom/docker.sock",
				"-timeout", "30s",
			},
			want: &Config{
				LocalSocket:  "/tmp/test.sock",
				SSHUser:      "testuser",
				SSHHost:      "testhost:2222",
				SSHKeyPath:   tmpFile.Name(),
				RemoteSocket: "/custom/docker.sock",
				Timeout:      30 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "partial flags with defaults",
			args: []string{
				"-local-socket", "/tmp/test.sock",
				"-ssh-user", "testuser",
				"-ssh-host", "testhost",
				"-ssh-key", tmpFile.Name(),
			},
			want: &Config{
				LocalSocket:  "/tmp/test.sock",
				SSHUser:      "testuser",
				SSHHost:      "testhost",
				SSHKeyPath:   tmpFile.Name(),
				RemoteSocket: "/var/run/docker.sock", // default
				Timeout:      10 * time.Second,       // default
			},
			wantErr: false,
		},
		{
			name: "no flags - defaults only",
			args: []string{},
			want: &Config{
				LocalSocket:  "",                     // empty
				SSHUser:      "",                     // empty
				SSHHost:      "",                     // empty
				SSHKeyPath:   "",                     // empty
				RemoteSocket: "/var/run/docker.sock", // default
				Timeout:      10 * time.Second,       // default
			},
			wantErr: false,
		},
		{
			name: "invalid timeout format",
			args: []string{
				"-timeout", "invalid",
			},
			wantErr: true,
			errType: ErrorCategoryConfig,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadFromFlags(tt.args)

			if tt.wantErr {
				if err == nil {
					t.Errorf("LoadFromFlags() expected error, got nil")
					return
				}

				if proxyErr, ok := err.(*ProxyError); ok {
					if proxyErr.Category != tt.errType {
						t.Errorf("LoadFromFlags() error category = %v, want %v", proxyErr.Category, tt.errType)
					}
				} else {
					t.Errorf("LoadFromFlags() expected ProxyError, got %T", err)
				}
				return
			}

			if err != nil {
				t.Errorf("LoadFromFlags() unexpected error: %v", err)
				return
			}

			if got.LocalSocket != tt.want.LocalSocket {
				t.Errorf("LoadFromFlags() LocalSocket = %v, want %v", got.LocalSocket, tt.want.LocalSocket)
			}
			if got.SSHUser != tt.want.SSHUser {
				t.Errorf("LoadFromFlags() SSHUser = %v, want %v", got.SSHUser, tt.want.SSHUser)
			}
			if got.SSHHost != tt.want.SSHHost {
				t.Errorf("LoadFromFlags() SSHHost = %v, want %v", got.SSHHost, tt.want.SSHHost)
			}
			if got.SSHKeyPath != tt.want.SSHKeyPath {
				t.Errorf("LoadFromFlags() SSHKeyPath = %v, want %v", got.SSHKeyPath, tt.want.SSHKeyPath)
			}
			if got.RemoteSocket != tt.want.RemoteSocket {
				t.Errorf("LoadFromFlags() RemoteSocket = %v, want %v", got.RemoteSocket, tt.want.RemoteSocket)
			}
			if got.Timeout != tt.want.Timeout {
				t.Errorf("LoadFromFlags() Timeout = %v, want %v", got.Timeout, tt.want.Timeout)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a temporary SSH key file
	keyFile := filepath.Join(tmpDir, "test_key")
	if err := os.WriteFile(keyFile, []byte("test key content"), 0600); err != nil {
		t.Fatalf("Failed to create key file: %v", err)
	}

	// Create a config file
	configContent := `
local_socket: /tmp/file.sock
ssh_user: fileuser
ssh_host: filehost
ssh_key_path: ` + keyFile + `
remote_socket: /file/docker.sock
timeout: 20s
`
	configFile := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	tests := []struct {
		name       string
		configFile string
		args       []string
		want       *Config
		wantErr    bool
	}{
		{
			name:       "file only",
			configFile: configFile,
			args:       []string{},
			want: &Config{
				LocalSocket:  "/tmp/file.sock",
				SSHUser:      "fileuser",
				SSHHost:      "filehost",
				SSHKeyPath:   keyFile,
				RemoteSocket: "/file/docker.sock",
				Timeout:      20 * time.Second,
			},
			wantErr: false,
		},
		{
			name:       "flags override file",
			configFile: configFile,
			args: []string{
				"-local-socket", "/tmp/flag.sock",
				"-ssh-user", "flaguser",
				"-timeout", "30s",
			},
			want: &Config{
				LocalSocket:  "/tmp/flag.sock",    // overridden by flag
				SSHUser:      "flaguser",          // overridden by flag
				SSHHost:      "filehost",          // from file
				SSHKeyPath:   keyFile,             // from file
				RemoteSocket: "/file/docker.sock", // from file
				Timeout:      30 * time.Second,    // overridden by flag
			},
			wantErr: false,
		},
		{
			name:       "flags only (no config file)",
			configFile: "",
			args: []string{
				"-local-socket", "/tmp/flag.sock",
				"-ssh-user", "flaguser",
				"-ssh-host", "flaghost",
				"-ssh-key", keyFile,
			},
			want: &Config{
				LocalSocket:  "/tmp/flag.sock",
				SSHUser:      "flaguser",
				SSHHost:      "flaghost",
				SSHKeyPath:   keyFile,
				RemoteSocket: "/var/run/docker.sock", // default
				Timeout:      10 * time.Second,       // default
			},
			wantErr: false,
		},
		{
			name:       "defaults only",
			configFile: "",
			args:       []string{},
			want: &Config{
				LocalSocket:  "",                     // empty
				SSHUser:      "",                     // empty
				SSHHost:      "",                     // empty
				SSHKeyPath:   "",                     // empty
				RemoteSocket: "/var/run/docker.sock", // default
				Timeout:      10 * time.Second,       // default
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadConfig(tt.configFile, tt.args)

			if tt.wantErr {
				if err == nil {
					t.Errorf("LoadConfig() expected error, got nil")
					return
				}
				return
			}

			if err != nil {
				t.Errorf("LoadConfig() unexpected error: %v", err)
				return
			}

			if got.LocalSocket != tt.want.LocalSocket {
				t.Errorf("LoadConfig() LocalSocket = %v, want %v", got.LocalSocket, tt.want.LocalSocket)
			}
			if got.SSHUser != tt.want.SSHUser {
				t.Errorf("LoadConfig() SSHUser = %v, want %v", got.SSHUser, tt.want.SSHUser)
			}
			if got.SSHHost != tt.want.SSHHost {
				t.Errorf("LoadConfig() SSHHost = %v, want %v", got.SSHHost, tt.want.SSHHost)
			}
			if got.SSHKeyPath != tt.want.SSHKeyPath {
				t.Errorf("LoadConfig() SSHKeyPath = %v, want %v", got.SSHKeyPath, tt.want.SSHKeyPath)
			}
			if got.RemoteSocket != tt.want.RemoteSocket {
				t.Errorf("LoadConfig() RemoteSocket = %v, want %v", got.RemoteSocket, tt.want.RemoteSocket)
			}
			if got.Timeout != tt.want.Timeout {
				t.Errorf("LoadConfig() Timeout = %v, want %v", got.Timeout, tt.want.Timeout)
			}
		})
	}
}

func TestFindConfigFile(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to temp directory for testing
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(oldWd)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Test with no config files
	result := FindConfigFile()
	if result != "" {
		t.Errorf("FindConfigFile() with no files = %v, want empty string", result)
	}

	// Create a config file
	configFile := "ssh-docker-proxy.yaml"
	if err := os.WriteFile(configFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	// Test with config file present
	result = FindConfigFile()
	if result != configFile {
		t.Errorf("FindConfigFile() = %v, want %v", result, configFile)
	}
}
