package agent

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

type Clients struct {
	Fulcrum FulcrumClient
	Proxmox ProxmoxClient
	Kamaji  KamajiClient
	SSH     SSHClient
}

func (c *Clients) Close() {
	if c.SSH != nil {
		c.SSH.Close()
	}
}

// Agent is the main agent implementation
type Agent struct {
	fulcrumCli      FulcrumClient
	metricsReporter *MetricsReporter
	metricInterval  time.Duration
	pollInterval    time.Duration
	jobHandler      *JobHandler
	stopCh          chan struct{}
	wg              sync.WaitGroup
	startTime       time.Time
	connected       bool
	agentID         string
}

// New creates a new agent
func New(cli *Clients, templateID int, pollInterval, metricInterval time.Duration) (*Agent, error) {
	jobHandler := NewJobHandler(
		cli.Fulcrum,
		cli.Proxmox,
		templateID,
		cli.Kamaji,
		cli.SSH,
	)
	metricsReporter := NewMetricsReporter(
		cli.Fulcrum,
		cli.Proxmox,
	)

	return &Agent{
		fulcrumCli:      cli.Fulcrum,
		metricsReporter: metricsReporter,
		metricInterval:  metricInterval,
		pollInterval:    pollInterval,
		jobHandler:      jobHandler,
		stopCh:          make(chan struct{}),
		connected:       false,
	}, nil
}

// Start starts the agent
func (a *Agent) Start(ctx context.Context) error {
	a.startTime = time.Now()

	// Get agent information to verify the token is valid
	agentInfo, err := a.fulcrumCli.GetAgentInfo()
	if err != nil {
		return fmt.Errorf("failed to get agent information: %w", err)
	}

	// Extract agent ID from the response
	id, ok := agentInfo["id"].(string)
	if !ok {
		return fmt.Errorf("invalid agent information received")
	}
	a.agentID = id

	log.Printf("Agent authenticated with ID: %s", id)

	// Update agent status to Connected
	if err := a.fulcrumCli.UpdateAgentStatus("Connected"); err != nil {
		return fmt.Errorf("failed to update agent status: %w", err)
	}
	a.connected = true

	log.Printf("Agent status updated to Connected")

	// Start a simple background heartbeat to keep the agent alive
	a.wg.Add(1)
	go a.heartbeat(ctx)

	// Start metrics reporting background task
	a.wg.Add(1)
	go a.reportMetrics(ctx)

	// Start job polling background task
	a.wg.Add(1)
	go a.pollJobs(ctx)

	return nil
}

// heartbeat periodically updates the agent status to maintain the connection
func (a *Agent) heartbeat(ctx context.Context) {
	defer a.wg.Done()

	ticker := time.NewTicker(60 * time.Second) // Update status every minute
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := a.fulcrumCli.UpdateAgentStatus("Connected"); err != nil {
				log.Printf("Failed to update agent status: %v", err)
			} else {
				log.Printf("Heartbeat: Agent status updated")
			}
		case <-a.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// reportMetrics periodically reports collected metrics
func (a *Agent) reportMetrics(ctx context.Context) {
	defer a.wg.Done()

	ticker := time.NewTicker(a.metricInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := a.metricsReporter.Report()
			if err != nil {
				log.Printf("Error reporting metrics: %v", err)
			}
		case <-a.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// pollJobs periodically polls for pending jobs and processes them
func (a *Agent) pollJobs(ctx context.Context) {
	defer a.wg.Done()

	ticker := time.NewTicker(a.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := a.jobHandler.PollAndProcessJobs(); err != nil {
				log.Printf("Error polling jobs: %v", err)
			}
		case <-a.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// Shutdown stops the agent and releases resources
func (a *Agent) Shutdown(ctx context.Context) error {
	// Close the stop channel to signal all goroutines to stop
	close(a.stopCh)

	// Wait for all goroutines to complete with a timeout
	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All goroutines exited successfully
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(30 * time.Second):
		return fmt.Errorf("timeout waiting for goroutines to exit")
	}

	// Update agent status to Disconnected
	if a.connected {
		if err := a.fulcrumCli.UpdateAgentStatus("Disconnected"); err != nil {
			return fmt.Errorf("failed to update agent status on shutdown: %w", err)
		}
		a.connected = false
		log.Println("Agent status updated to Disconnected")
	}

	log.Println("Agent shut down successfully")
	return nil
}

// GetAgentID returns the agent's ID
func (a *Agent) GetAgentID() string {
	return a.agentID
}
