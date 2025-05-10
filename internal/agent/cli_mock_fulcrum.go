package agent

import (
	"fmt"
	"sync"
	"time"
)

// MockFulcrumClient implements FulcrumClient interface for testing
type MockFulcrumClient struct {
	mu            sync.RWMutex
	agentStatus   string
	agentInfo     map[string]any
	jobs          []*Job
	jobMap        map[string]int
	service       map[string]Service
	serviceExtIDs map[string]string
	metrics       []*MetricEntry
}

// NewMockFulcrumClient creates a new in-memory stub Fulcrum client
func NewMockFulcrumClient() *MockFulcrumClient {
	return &MockFulcrumClient{
		agentStatus: "Online",
		jobs:        []*Job{},
		jobMap:      make(map[string]int),
		metrics:     []*MetricEntry{},
		agentInfo: map[string]any{
			"id":   "test-agent-id",
			"name": "test-agent",
			"type": "kubernetes",
		},
		service:       make(map[string]Service),
		serviceExtIDs: make(map[string]string),
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

// GetPendingJobs retrieves pending jobs for this agent
func (c *MockFulcrumClient) GetPendingJobs() ([]*Job, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Create a slice to hold the pending jobs
	var pendingJobs []*Job

	// Iterate over the jobs and find those that are pending
	for _, j := range c.jobs {
		if j.State == JobStatePending {
			pendingJobs = append(pendingJobs, j)
		}
	}

	return pendingJobs, nil
}

// PullCompletedJobs returns all completed jobs
func (c *MockFulcrumClient) PullCompletedJobs() []*Job {
	return c.PullJobs(JobStateCompleted)
}

// PullFailedJobs returns all failed jobs
func (c *MockFulcrumClient) PullFailedJobs() []*Job {
	return c.PullJobs(JobStateFailed)
}

// GetFailedJobs returns jobs by state and removes them from the queue
func (c *MockFulcrumClient) PullJobs(state JobState) []*Job {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Create a slice to hold the jobs
	var jobs []*Job

	// Iterate over the jobs and find those that match the state
	for i, j := range c.jobs {
		if c.jobs[i].State == state {
			jobs = append(jobs, j)
			// Remove job from the queue
			delete(c.jobMap, c.jobs[i].ID)
			c.jobs = append(c.jobs[:i], c.jobs[i+1:]...)
		}
	}

	return jobs
}

// ClaimJob claims a job for processing
func (c *MockFulcrumClient) ClaimJob(jobID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Find the job by ID in our map
	idx, exists := c.jobMap[jobID]
	if !exists {
		return fmt.Errorf("job with ID %s not found", jobID)
	}

	// Get the job from our array
	job := c.jobs[idx]

	// Check if job is already claimed/not pending
	if job.State != JobStatePending {
		return fmt.Errorf("job with ID %s is not in pending state", jobID)
	}

	// Mark as claimed by updating its state
	job.State = JobStateProcessing

	return nil
}

// CompleteJob marks a job as completed with results
func (c *MockFulcrumClient) CompleteJob(jobID string, response JobResponse) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Find the job by ID in our map
	idx, exists := c.jobMap[jobID]
	if !exists {
		return fmt.Errorf("job with ID %s not found", jobID)
	}

	// Get the job
	job := c.jobs[idx]

	// Check if job is in the correct state
	if job.State != JobStateProcessing {
		return fmt.Errorf("job with ID %s is not in processing state", jobID)
	}

	// Get the service from the job (this fixes the key issue)
	service := job.Service

	// Update the service with the response
	service.Resources = response.Resources

	// Update the service state
	service.CurrentState = *service.TargetState
	service.TargetState = nil

	service.CurrentProperties = service.TargetProperties
	service.TargetProperties = nil

	// Store the updated service in the map by its proper ID
	c.service[service.ID] = service

	// Mark job as completed
	job.State = JobStateCompleted

	return nil
}

// FailJob marks a job as failed with an error message
func (c *MockFulcrumClient) FailJob(jobID string, errorMessage string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Find the job by ID in our map
	idx, exists := c.jobMap[jobID]
	if !exists {
		return fmt.Errorf("job with ID %s not found", jobID)
	}

	// Get the job
	job := c.jobs[idx]

	// Check if job is in the correct state
	if job.State != JobStateProcessing {
		return fmt.Errorf("job with ID %s is not in processing state", jobID)
	}

	// Mark job as failed and store error message
	job.State = JobStateFailed
	job.ErrorMessage = errorMessage

	return nil
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

// CreateService creates a new service with the given parameters
func (c *MockFulcrumClient) CreateService(id, name string, externalID *string, targetProperties *Properties) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.service[id]; exists {
		return fmt.Errorf("service with ID %s already exists", id)
	}

	targetState := ServiceCreated
	currentState := ServiceCreating

	// Create the service
	service := Service{
		ID:                id,
		Name:              name,
		ExternalID:        externalID,
		CurrentProperties: nil, // Initially no current properties
		TargetProperties:  targetProperties,
		Resources:         nil,
		CurrentState:      currentState,
		TargetState:       &targetState,
	}

	// Store the service
	c.service[id] = service

	// Store external ID mapping if it exists
	if externalID != nil {
		c.serviceExtIDs[*externalID] = id
	}

	// Create a job for service creation
	job := &Job{
		ID:       fmt.Sprintf("job-%s-create-%d", id, time.Now().UnixNano()),
		Action:   JobActionServiceCreate,
		State:    JobStatePending,
		Priority: 1,
		Service:  service,
	}
	c.EnqueueJob(job)

	return nil
}

// StartService starts a service
func (c *MockFulcrumClient) StartService(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	service, exists := c.service[id]
	if !exists {
		return fmt.Errorf("service with ID %s not found", id)
	}

	if service.CurrentState != ServiceCreated && service.CurrentState != ServiceStopped {
		return fmt.Errorf("service with ID %s must be created or stopped before starting", id)
	}

	// Update state
	service.CurrentState = ServiceStarting
	targetState := ServiceStarted
	service.TargetState = &targetState

	c.service[id] = service

	// Create a job for service start
	job := &Job{
		ID:       fmt.Sprintf("job-%s-start-%d", id, time.Now().UnixNano()),
		Action:   JobActionServiceStart,
		State:    JobStatePending,
		Priority: 1,
		Service:  service,
	}
	c.EnqueueJob(job)

	return nil
}

// StopService stops a service
func (c *MockFulcrumClient) StopService(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	service, exists := c.service[id]
	if !exists {
		return fmt.Errorf("service with ID %s not found", id)
	}

	if service.CurrentState != ServiceStarted {
		return fmt.Errorf("service with ID %s must be started before stopping", id)
	}

	// Update state
	service.CurrentState = ServiceStopping
	targetState := ServiceStopped
	service.TargetState = &targetState

	c.service[id] = service

	// Create a job for service stop
	job := &Job{
		ID:       fmt.Sprintf("job-%s-stop-%d", id, time.Now().UnixNano()),
		Action:   JobActionServiceStop,
		State:    JobStatePending,
		Priority: 1,
		Service:  service,
	}
	c.EnqueueJob(job)

	return nil
}

// UpdateService updates an existing service with new target properties
func (c *MockFulcrumClient) UpdateService(id string, targetProperties *Properties) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	service, exists := c.service[id]
	if !exists {
		return fmt.Errorf("service with ID %s not found", id)
	}

	// Update target properties
	service.TargetProperties = targetProperties

	var jobAction JobAction

	// Update current state based on the operation
	if service.CurrentState == ServiceStarted {
		service.CurrentState = ServiceHotUpdating
		targetState := ServiceStarted
		service.TargetState = &targetState
		jobAction = JobActionServiceHotUpdate
	} else if service.CurrentState == ServiceStopped {
		service.CurrentState = ServiceColdUpdating
		targetState := ServiceStopped
		service.TargetState = &targetState
		jobAction = JobActionServiceColdUpdate
	} else {
		return fmt.Errorf("service with ID %s must be started or stopped before updating", id)
	}

	c.service[id] = service

	// Create a job for service update
	job := &Job{
		ID:       fmt.Sprintf("job-%s-update-%d", id, time.Now().UnixNano()),
		Action:   jobAction,
		State:    JobStatePending,
		Priority: 1,
		Service:  service,
	}
	c.EnqueueJob(job)

	return nil
}

// DeleteService deletes a service
func (c *MockFulcrumClient) DeleteService(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	service, exists := c.service[id]
	if !exists {
		return fmt.Errorf("service with ID %s not found", id)
	}

	if service.CurrentState != ServiceStopped {
		return fmt.Errorf("service with ID %s must be stopped before deletion", id)
	}

	// Update state to deleting
	service.CurrentState = ServiceDeleting
	targetState := ServiceDeleted
	service.TargetState = &targetState
	c.service[id] = service

	// Create a job for service deletion
	job := &Job{
		ID:       fmt.Sprintf("job-%s-delete-%d", id, time.Now().UnixNano()),
		Action:   JobActionServiceDelete,
		State:    JobStatePending,
		Priority: 1,
		Service:  service,
	}
	c.EnqueueJob(job)

	return nil
}

// GetService retrieves a service by ID
func (c *MockFulcrumClient) GetService(id string) (Service, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	service, exists := c.service[id]
	if !exists {
		return Service{}, fmt.Errorf("service with ID %s not found", id)
	}

	return service, nil
}

// GetServiceByExternalID retrieves a service by its external ID
func (c *MockFulcrumClient) GetServiceByExternalID(externalID string) (Service, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	serviceID, exists := c.serviceExtIDs[externalID]
	if !exists {
		return Service{}, fmt.Errorf("service with external ID %s not found", externalID)
	}

	service, exists := c.service[serviceID]
	if !exists {
		// This should not happen if the maps are properly maintained
		return Service{}, fmt.Errorf("inconsistent state: service ID %s points to non-existent service", serviceID)
	}

	return service, nil
}

// EnqueueJob adds a job to the queue
func (c *MockFulcrumClient) EnqueueJob(job *Job) error {
	// Check if job already exists
	if _, exists := c.jobMap[job.ID]; exists {
		return fmt.Errorf("job with ID %s already exists", job.ID)
	}
	// Add job to the array and map
	c.jobs = append(c.jobs, job)
	c.jobMap[job.ID] = len(c.jobs) - 1
	return nil
}
