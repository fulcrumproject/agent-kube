# Kubernetes Fulcrum Agent

A Kubernetes clusters lifecycle management agent for the Fulcrum Core platform.

## Overview

This agent manages the complete lifecycle of Kubernetes clusters provisioned on Proxmox virtual machines using Kamaji for tenant control planes. The agent communicates with the Fulcrum Core API to receive jobs, manages VM infrastructure via Proxmox, and configures Kubernetes tenant clusters using Kamaji.

## Installation

### Prerequisites

- Go 1.24.1 or higher
- Access to a running Fulcrum Core API
- Agent type and provider registered in Fulcrum Core

### Building

```bash
# Build the agent
go build -o fulcrum-kube-agent ./cmd/fulcrum-kube-agent
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
| `create` | Creates a tenant control plane and stopped worker VMs |
| `update` | Adds/removes nodes; applies node On/Off changes only when the service is running |
| `start`  | Starts worker VMs whose configured status is On |
| `stop`   | Stops worker VMs whose configured status is On |
| `delete` | Deletes worker VMs and the tenant control plane |

### Core API contract

Aligned with Fulcrum Core `3212fc7` (no legacy API compatibility):

- `job.params` contains desired properties for `create` and `update`; `service.properties` contains the applied configuration.
- `service.status` is a lifecycle state. Updates and metrics use `serviceType.lifecycleSchema.runningStates`, not a fixed `Started` label. Update jobs fetch the service separately because pending jobs do not include its service type.
- Job completion sends `agentInstanceId` and `agentInstanceData` (cluster endpoint, kubeconfig and node-to-VM mapping). Metrics use `agentInstanceId` too.
- Core owns state transitions and applies update params after completion. Unknown actions or missing job/instance data fail rather than being silently completed.
- Core errors retain HTTP status, message and field-level validation details. Service listing sends the requested page as a query parameter.
- Local configuration remains authoritative for this agent; Core-managed configuration and vault resolution are not implemented by this change.

Existing limitations: changing a node's size is unsupported; infrastructure operations are not transactional or automatically rolled back. Do not blindly retry partially provisioned clusters.

### Manual REST scenario

[`test/rest/agent_kube.http`](test/rest/agent_kube.http) is a versioned VS Code REST Client scenario. It creates its own service type/lifecycle, agent type, participants and agent instead of depending on fixed IDs. Export `FULCRUM_TEST_ADMIN_TOKEN` to the REST Client process; never commit credentials or response exports.

Use a disposable Core instance. Ensure Resource metric types `vm.cpu.usage` and `vm.memory.usage` exist, run setup, then configure the agent with the generated agent token. Wait for each asynchronous job to complete before the next request. `POST /services/{id}/delete` requests deletion; there is no generic `/retry` endpoint. The `.DS_Store` files in `test/` remain ignored.

**Warning:** without `-mock`, this scenario creates and deletes real infrastructure. `-mock` mocks Proxmox/Kamaji/SSH only: it still connects to Core.

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
# Run unit and local HTTP contract tests (never contact real infrastructure)
INTEGRATION_TEST=false go test ./...
INTEGRATION_TEST=false go test -race ./...
go vet ./...

# Run the cluster lifecycle tests with mock infrastructure
go test -v ./internal/agent

# Live infrastructure tests are opt-in; use only disposable resources.
# INTEGRATION_TEST=true go test ./...
```
