# Kubernetes Fulcrum Agent

A Kubernetes clusters lifecycle management agent for the Fulcrum Core platform

## Overview

TBD

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
  "agentToken": "YOUR_AGENT_TOKEN",
  "fulcrumApiUrl": "http://localhost:3000",
  "jobPollInterval": "5s",
  "metricReportInterval": "1m"
}
```

### Default Configuration Values

The agent uses the following default values if not specified:

| Parameter              | Default Value           | Description                 |
| ---------------------- | ----------------------- | --------------------------- |
| `agentToken`           | (empty)                 | Must be provided            |
| `fulcrumApiUrl`        | "http://localhost:3000" | URL of the Fulcrum Core API |
| `jobPollInterval`      | 5s                      | How often to poll for jobs  |
| `metricReportInterval` | 1m                      | How often to report metrics |

### Environment Variables

Configuration can also be provided or overridden using environment variables. The agent automatically prepends `TESTAGENT_` to the field names defined in the Config struct:

- `FULCRUM_AGENT_TOKEN`: Secret token of the agent
- `FULCRUM_AGENT_API_URL`: URL of the Fulcrum Core API
- `FULCRUM_AGENT_JOB_POLL_INTERVAL`: How often to poll for jobs
- `FULCRUM_AGENT_METRIC_REPORT_INTERVAL`: How often to report metrics

## Usage

### Running the Agent

```bash
# Run with default configuration (requires TESTAGENT_AGENT_TOKEN to be set)
./fulcrum-kube-agent

# Run with a configuration file
./fulcrum-kube-agent -config config.json
```

### Stopping the Agent

The agent handles SIGINT and SIGTERM signals for graceful shutdown. Simply press `Ctrl+C` to stop it cleanly.

## Metrics Generated

The agent generates the following metrics:

TBD

## Job Processing

The agent can process the following job types from Fulcrum Core:

TBD

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
