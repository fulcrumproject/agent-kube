package agent

import (
	"fmt"
	"sync"
)

// MockFulcrumClient implements FulcrumClient interface for testing
type MockFulcrumClient struct {
	agentStatus   string
	pendingJobs   []*Job
	claimedJobs   map[string]bool
	completedJobs map[string]any
	failedJobs    map[string]string // jobID -> error message
	metrics       []*MetricEntry
	agentInfo     map[string]any
	mu            sync.RWMutex
}

// NewMockFulcrumClient creates a new in-memory stub Fulcrum client
func NewMockFulcrumClient() *MockFulcrumClient {
	return &MockFulcrumClient{
		agentStatus:   "Online",
		pendingJobs:   []*Job{},
		claimedJobs:   make(map[string]bool),
		completedJobs: make(map[string]any),
		failedJobs:    make(map[string]string),
		metrics:       []*MetricEntry{},
		agentInfo: map[string]any{
			"id":   "test-agent-id",
			"name": "test-agent",
			"type": "kubernetes",
		},
	}
}

// UpdateAgentStatus updates the agent's status
func (c *MockFulcrumClient) UpdateAgentStatus(status string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.agentStatus = status
	return nil
}

// GetAgentStatus returns the current agent status (for test verification)
func (c *MockFulcrumClient) GetAgentStatus() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.agentStatus
}

// GetAgentInfo retrieves the agent's information
func (c *MockFulcrumClient) GetAgentInfo() (map[string]any, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Return a deep copy to avoid test interference
	infoCopy := make(map[string]any, len(c.agentInfo))
	for k, v := range c.agentInfo {
		infoCopy[k] = v
	}

	return infoCopy, nil
}

// SetAgentInfo sets the agent information that will be returned by GetAgentInfo
func (c *MockFulcrumClient) SetAgentInfo(info map[string]any) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.agentInfo = info
}

// AddPendingJob adds a job to the pending jobs list
func (c *MockFulcrumClient) AddPendingJob(job *Job) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Ensure job has the pending state
	job.State = JobStatePending

	// Add to pending jobs
	c.pendingJobs = append(c.pendingJobs, job)
}

// GetPendingJobs retrieves pending jobs for this agent
func (c *MockFulcrumClient) GetPendingJobs() ([]*Job, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Return a copy of the pending jobs to avoid test interference
	jobsCopy := make([]*Job, len(c.pendingJobs))
	copy(jobsCopy, c.pendingJobs)

	return jobsCopy, nil
}

// ClaimJob claims a job for processing
func (c *MockFulcrumClient) ClaimJob(jobID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Find the job
	var foundJob *Job
	for _, job := range c.pendingJobs {
		if job.ID == jobID {
			foundJob = job
			break
		}
	}

	if foundJob == nil {
		return fmt.Errorf("job with ID %s not found", jobID)
	}

	// Check if job is already claimed
	if _, exists := c.claimedJobs[jobID]; exists {
		return fmt.Errorf("job with ID %s is already claimed", jobID)
	}

	// Mark as claimed
	c.claimedJobs[jobID] = true
	foundJob.State = JobStateProcessing

	// Remove from pending jobs
	for i, job := range c.pendingJobs {
		if job.ID == jobID {
			c.pendingJobs = append(c.pendingJobs[:i], c.pendingJobs[i+1:]...)
			break
		}
	}

	return nil
}

// CompleteJob marks a job as completed with results
func (c *MockFulcrumClient) CompleteJob(jobID string, resources any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if job was claimed
	if _, exists := c.claimedJobs[jobID]; !exists {
		return fmt.Errorf("job with ID %s was not claimed", jobID)
	}

	// Move from claimed to completed
	delete(c.claimedJobs, jobID)
	c.completedJobs[jobID] = resources

	return nil
}

// GetCompletedJobs returns all completed jobs (for test verification)
func (c *MockFulcrumClient) GetCompletedJobs() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Return a copy to avoid test interference
	completedCopy := make(map[string]any, len(c.completedJobs))
	for k, v := range c.completedJobs {
		completedCopy[k] = v
	}

	return completedCopy
}

// FailJob marks a job as failed with an error message
func (c *MockFulcrumClient) FailJob(jobID string, errorMessage string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if job was claimed
	if _, exists := c.claimedJobs[jobID]; !exists {
		return fmt.Errorf("job with ID %s was not claimed", jobID)
	}

	// Move from claimed to failed
	delete(c.claimedJobs, jobID)
	c.failedJobs[jobID] = errorMessage

	return nil
}

// GetFailedJobs returns all failed jobs (for test verification)
func (c *MockFulcrumClient) GetFailedJobs() map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Return a copy to avoid test interference
	failedCopy := make(map[string]string, len(c.failedJobs))
	for k, v := range c.failedJobs {
		failedCopy[k] = v
	}

	return failedCopy
}

// ReportMetric reports a metric
func (c *MockFulcrumClient) ReportMetric(metric *MetricEntry) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.metrics = append(c.metrics, metric)
	return nil
}

// GetReportedMetrics returns all reported metrics (for test verification)
func (c *MockFulcrumClient) GetReportedMetrics() []*MetricEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Return a copy to avoid test interference
	metricsCopy := make([]*MetricEntry, len(c.metrics))
	copy(metricsCopy, c.metrics)

	return metricsCopy
}

// CreateTestJob is a helper to create a job for testing
func CreateTestJob(jobID string, action JobAction, serviceID string, serviceName string) *Job {
	job := &Job{
		ID:       jobID,
		Action:   action,
		State:    JobStatePending,
		Priority: 1,
	}

	job.Service.ID = serviceID
	job.Service.Name = serviceName

	return job
}

// CreateTestJobWithNodes is a helper to create a job with node info for testing
func CreateTestJobWithNodes(
	jobID string,
	action JobAction,
	serviceID string,
	serviceName string,
	currentNodes []Node,
	targetNodes []Node,
) *Job {
	job := CreateTestJob(jobID, action, serviceID, serviceName)

	// Set current nodes if provided
	if len(currentNodes) > 0 {
		job.Service.CurrentProperties = &struct {
			Nodes []Node `json:"nodes"`
		}{
			Nodes: currentNodes,
		}
	}

	// Set target nodes if provided
	if len(targetNodes) > 0 {
		job.Service.TargetProperties = &struct {
			Nodes []Node `json:"nodes"`
		}{
			Nodes: targetNodes,
		}
	}

	return job
}

// CreateTestNode is a helper to create a node for testing
func CreateTestNode(id string, size NodeSize, state NodeState) Node {
	return Node{
		ID:    id,
		Size:  size,
		State: state,
	}
}
