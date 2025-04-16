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
	return nil
}
