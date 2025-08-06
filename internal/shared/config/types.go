package config

import "time"

// ClientConfig represents the complete client configuration
type ClientConfig struct {
	Hetzner     HetznerConfig     `yaml:"hetzner" mapstructure:"hetzner"`
	Docker      DockerConfig      `yaml:"docker" mapstructure:"docker"`
	Activity    ActivityConfig    `yaml:"activity" mapstructure:"activity"`
	KeepAlive   KeepAliveConfig   `yaml:"keepalive" mapstructure:"keepalive"`
	SSH         SSHConfig         `yaml:"ssh" mapstructure:"ssh"`
	Logging     LoggingConfig     `yaml:"logging" mapstructure:"logging"`
	PortForward PortForwardConfig `yaml:"port_forward" mapstructure:"port_forward"`
}

// ServerConfig represents the complete server configuration
type ServerConfig struct {
	Docker    DockerConfig    `yaml:"docker" mapstructure:"docker"`
	KeepAlive KeepAliveConfig `yaml:"keepalive" mapstructure:"keepalive"`
	Logging   LoggingConfig   `yaml:"logging" mapstructure:"logging"`
}

// HetznerConfig contains Hetzner Cloud API configuration
type HetznerConfig struct {
	APIToken        string   `yaml:"api_token" mapstructure:"api_token" env:"HETZNER_API_TOKEN"`
	ServerType      string   `yaml:"server_type" mapstructure:"server_type" default:"cpx21"`
	Location        string   `yaml:"location" mapstructure:"location" default:"fsn1"`
	VolumeSize      int      `yaml:"volume_size" mapstructure:"volume_size" default:"10"`
	PreferredImages []string `yaml:"preferred_images" mapstructure:"preferred_images" default:"[\"docker-ce\", \"ubuntu-22.04\"]"`
}

// DockerConfig contains Docker-related configuration
type DockerConfig struct {
	SocketPath string `yaml:"socket_path" mapstructure:"socket_path" default:"/var/run/docker.sock"`
	ProxyPort  int    `yaml:"proxy_port" mapstructure:"proxy_port" default:"2376"`
}

// KeepAliveConfig contains keep-alive mechanism configuration
type KeepAliveConfig struct {
	Interval      time.Duration `yaml:"interval" mapstructure:"interval" default:"30s"`
	Timeout       time.Duration `yaml:"timeout" mapstructure:"timeout" default:"5m"`
	RetryInterval time.Duration `yaml:"retry_interval" mapstructure:"retry_interval" default:"5s"`
	MaxRetries    int           `yaml:"max_retries" mapstructure:"max_retries" default:"3"`
}

// SSHConfig contains SSH connection configuration
type SSHConfig struct {
	KeyPath   string        `yaml:"key_path" mapstructure:"key_path" default:"~/.dockbridge/ssh/id_rsa"`
	Port      int           `yaml:"port" mapstructure:"port" default:"22"`
	Timeout   time.Duration `yaml:"timeout" mapstructure:"timeout" default:"30s"`
	KeepAlive time.Duration `yaml:"keep_alive" mapstructure:"keep_alive" default:"30s"`
}

// ActivityConfig contains activity tracking and timeout configuration
type ActivityConfig struct {
	IdleTimeout       time.Duration `yaml:"idle_timeout" mapstructure:"idle_timeout" default:"5m"`
	ConnectionTimeout time.Duration `yaml:"connection_timeout" mapstructure:"connection_timeout" default:"30m"`
	GracePeriod       time.Duration `yaml:"grace_period" mapstructure:"grace_period" default:"30s"`
}

// LoggingConfig contains logging configuration
type LoggingConfig struct {
	Level  string `yaml:"level" mapstructure:"level" default:"info"`
	Format string `yaml:"format" mapstructure:"format" default:"json"`
	Output string `yaml:"output" mapstructure:"output" default:"stdout"`
}

// PortForwardConfig contains port forwarding configuration
type PortForwardConfig struct {
	Enabled          bool             `yaml:"enabled" mapstructure:"enabled" default:"true"`
	ConflictStrategy ConflictStrategy `yaml:"conflict_strategy" mapstructure:"conflict_strategy" default:"increment"`
	MonitorInterval  time.Duration    `yaml:"monitor_interval" mapstructure:"monitor_interval" default:"30s"`
}

// ConflictStrategy defines how to handle port conflicts
type ConflictStrategy string

const (
	ConflictStrategyIncrement ConflictStrategy = "increment" // Find next available port
	ConflictStrategyFail      ConflictStrategy = "fail"      // Return Docker error
)
