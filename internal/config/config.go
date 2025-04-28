package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
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
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		AgentToken:           "", // Must be provided
		FulcrumAPIURL:        "http://localhost:3000",
		JobPollInterval:      5 * time.Second,
		MetricReportInterval: 30 * time.Second,
	}
}

// LoadFromFile loads configuration from a JSON file
func LoadFromFile(filepath string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return cfg, nil
}

// LoadFromEnv overrides configuration with environment variables
func (c *Config) LoadFromEnv() error {
	return LoadEnvToStruct(c, "FULCRUM_AGENT_", "env")
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
