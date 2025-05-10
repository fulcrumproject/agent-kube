# Kubernetes Fulcrum Agent

A Kubernetes clusters lifecycle management agent for the Fulcrum Core platform.

## Overview

This agent manages the complete lifecycle of Kubernetes clusters provisioned on Proxmox virtual machines using Kamaji for tenant control planes. The agent communicates with the Fulcrum Core API to receive jobs, manages VM infrastructure via Proxmox, and configures Kubernetes tenant clusters using Kamaji.

## Installation

### Prerequisites

- Go 1.20 or higher
- Access to a running Fulcrum Core API
- Agent type and provider registered in Fulcrum Core

### Building

```bash
# Build the agent
go build -o fulcrum-kube-agent
```

## Configuration

The agent can be configured using a combination of a configuration file and environment variables.

### Configuration File

Create a JSON configuration file (e.g., `config.json`):

```json
{
  "fulcrumApiToken": "YOUR_AGENT_TOKEN",
  "fulcrumApiUrl": "http://localhost:3000",
  "jobPollInterval": "5s",
  "metricReportInterval": "30s",
  "proxmoxApiUrl": "https://proxmox.example.com:8006/api2/json",
  "proxmoxApiToken": "YOUR_PROXMOX_TOKEN",
  "proxmoxTemplate": 100,
  "proxmoxHost": "pve",
  "proxmoxStorage": "local-lvm",
  "kubeApiUrl": "https://kubernetes.example.com",
  "kubeApiToken": "YOUR_KUBERNETES_TOKEN",
  "skipTlsVerify": false
}
```

### Default Configuration Values

The agent uses the following default values if not specified:

| Parameter              | Default Value           | Description                      |
| ---------------------- | ----------------------- | -------------------------------- |
| `fulcrumApiToken`      | (empty)                 | Must be provided                 |
| `fulcrumApiUrl`        | "http://localhost:3000" | URL of the Fulcrum Core API      |
| `jobPollInterval`      | 5s                      | How often to poll for jobs       |
| `metricReportInterval` | 30s                     | How often to report metrics      |
| `skipTlsVerify`        | false                   | Whether to skip TLS verification |

### Environment Variables

Configuration can be provided or overridden using environment variables. The agent automatically prepends `FULCRUM_AGENT_` to the field names:

#### Fulcrum Connection
- `FULCRUM_AGENT_API_URL`: URL of the Fulcrum Core API
- `FULCRUM_AGENT_API_TOKEN`: Secret token of the agent

#### Agent Behavior
- `FULCRUM_AGENT_JOB_POLL_INTERVAL`: How often to poll for jobs
- `FULCRUM_AGENT_METRIC_REPORT_INTERVAL`: How often to report metrics

#### Proxmox Configuration
- `FULCRUM_AGENT_PROXMOX_API_URL`: Proxmox API URL
- `FULCRUM_AGENT_PROXMOX_API_SECRET`: Proxmox API token
- `FULCRUM_AGENT_PROXMOX_TEMPLATE`: VM template ID
- `FULCRUM_AGENT_PROXMOX_HOST`: Proxmox host
- `FULCRUM_AGENT_PROXMOX_STORAGE`: Proxmox storage

#### Kubernetes Configuration
- `FULCRUM_AGENT_KUBE_API_URL`: Kubernetes API URL
- `FULCRUM_AGENT_KUBE_API_SECRET`: Kubernetes API token

#### Security
- `FULCRUM_AGENT_SKIP_TLS_VERIFY`: Skip TLS certificate validation

## Usage

### Running the Agent

```bash
# Run with default configuration (requires FULCRUM_AGENT_API_TOKEN to be set)
./fulcrum-kube-agent

# Run with a configuration file
./fulcrum-kube-agent -config config.json

# Run with mock clients for development/testing
./fulcrum-kube-agent -mock
```

### Stopping the Agent

The agent handles SIGINT and SIGTERM signals for graceful shutdown. Simply press `Ctrl+C` to stop it cleanly.

## Metrics Generated

The agent generates the following metrics for running services:

| Metric            | Description                                 |
| ----------------- | ------------------------------------------- |
| `VM CPU Usage`    | CPU utilization percentage for each VM node |
| `VM Memory Usage` | Memory utilization for each VM node         |

## Job Processing

The agent processes the following job types from Fulcrum Core:

| Job Type            | Description                                                                  |
| ------------------- | ---------------------------------------------------------------------------- |
| `ServiceCreate`     | Creates a new Kubernetes tenant cluster with worker nodes                    |
| `ServiceColdUpdate` | Updates cluster configuration (adding/removing nodes) without live migration |
| `ServiceHotUpdate`  | Updates cluster configuration while maintaining service availability         |
| `ServiceStart`      | Starts the cluster and its associated VMs                                    |
| `ServiceStop`       | Stops the cluster and its associated VMs                                     |
| `ServiceDelete`     | Deletes the entire cluster and cleans up resources                           |

## Development

### Hot Reloading

The project includes a configuration file for Air (`.air.toml`), which provides hot reloading during development:

```bash
# Install Air
go install github.com/cosmtrek/air@latest

# Run with Air for development
air
```

This will automatically rebuild and restart the agent when source files change.

### Testing

The agent includes comprehensive unit tests:

```bash
# Run all tests
go test ./...

# Run specific tests
go test -v ./agent
```
