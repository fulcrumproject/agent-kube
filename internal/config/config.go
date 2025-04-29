package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/joho/godotenv"
)

// Config holds the configuration for the agent
type Config struct {
	// Agent authentication
	AgentToken string `json:"agentToken" env:"TOKEN"` // Authentication token for the agent

	// Fulcrum Core API connection
	FulcrumAPIURL string `json:"fulcrumApiUrl" env:"API_URL"`

	// Polling intervals
	JobPollInterval      time.Duration `json:"jobPollInterval" env:"JOB_POLL_INTERVAL"`           // How often to poll for jobs
	MetricReportInterval time.Duration `json:"metricReportInterval" env:"METRIC_REPORT_INTERVAL"` // How often to report metrics

	// Proxmox
	ProxmoxAPIURL   string `json:"proxmoxApiUrl" env:"PROXMOX_API_URL"`
	ProxmoxAPIToken string `json:"proxmoxApiToken" env:"PROXMOX_API_SECRET"`
	ProxmoxTemplate int    `json:"proxmoxTemplate" env:"PROXMOX_TEMPLATE"`
	ProxmoxHost     string `json:"proxmoxHost" env:"PROXMOX_HOST"`
	ProxmoxStorage  string `json:"proxmoxStorage" env:"PROXMOX_STORAGE"`

	// Kubernetes
	KubeAPIURL   string `json:"kubeApiUrl" env:"KUBE_API_URL"`
	KubeAPIToken string `json:"kubeApiToken" env:"KUBE_API_SECRET"`

	// Client HTTP
	SkipTLSVerify bool `json:"skipTlsVerify" env:"SKIP_TLS_VERIFY"` // Skip TLS certificate validation
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.AgentToken == "" {
		return fmt.Errorf("agent token is required")
	}
	if c.FulcrumAPIURL == "" {
		return fmt.Errorf("the Fulcrum API URL is required")
	}

	// Validate Proxmox configuration - all properties are mandatory
	if c.ProxmoxAPIURL == "" {
		return fmt.Errorf("Proxmox API URL is required")
	}
	if c.ProxmoxAPIToken == "" {
		return fmt.Errorf("Proxmox API token is required")
	}
	if c.ProxmoxTemplate <= 0 {
		return fmt.Errorf("Proxmox template ID must be greater than 0")
	}
	if c.ProxmoxHost == "" {
		return fmt.Errorf("Proxmox host is required")
	}
	if c.ProxmoxStorage == "" {
		return fmt.Errorf("Proxmox storage is required")
	}

	// Validate Kubernetes configuration - all properties are mandatory
	if c.KubeAPIURL == "" {
		return fmt.Errorf("Kubernetes API URL is required")
	}
	if c.KubeAPIToken == "" {
		return fmt.Errorf("Kubernetes API token is required")
	}

	return nil
}

// ConfigBuilder implements a builder pattern for creating Config instances
type ConfigBuilder struct {
	config *Config
	err    error
}

// Default returns a ConfigBuilder with default configuration
func Builder() *ConfigBuilder {
	return &ConfigBuilder{
		config: &Config{
			AgentToken:           "", // Must be provided
			FulcrumAPIURL:        "http://localhost:3000",
			SkipTLSVerify:        false, // By default, verify TLS certificates
			JobPollInterval:      5 * time.Second,
			MetricReportInterval: 30 * time.Second,
		},
	}
}

// LoadFile loads configuration from a JSON file
func (b *ConfigBuilder) LoadFile(filepath *string) *ConfigBuilder {
	if b.err != nil {
		return b
	}

	if filepath == nil || *filepath == "" {
		return b
	}

	data, err := os.ReadFile(*filepath)
	if err != nil {
		b.err = fmt.Errorf("failed to read config file: %w", err)
		return b
	}

	if err := json.Unmarshal(data, b.config); err != nil {
		b.err = fmt.Errorf("failed to parse config file: %w", err)
		return b
	}

	return b
}

// WithEnv overrides configuration from environment variables
func (b *ConfigBuilder) WithEnv(dirpath string) *ConfigBuilder {
	if b.err != nil {
		return b
	}

	_ = godotenv.Load(path.Join(dirpath, ".env.local"))
	_ = godotenv.Load(path.Join(dirpath, ".env"))

	if err := LoadEnvToStruct(b.config, "FULCRUM_AGENT_", "env"); err != nil {
		b.err = fmt.Errorf("failed to override configuration from environment: %w", err)
	}

	return b
}

// Build validates and returns the final Config
func (b *ConfigBuilder) Build() (*Config, error) {
	if b.err != nil {
		return nil, b.err
	}

	if err := b.config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return b.config, nil
}
