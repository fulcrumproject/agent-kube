// Provides an in-memory Core client for tests and mock mode.
// It uses current job params and completion payloads with an example lifecycle.
package agent

import (
	"fmt"
	"sync"
	"time"
)

// MockFulcrumClient implements FulcrumClient without external infrastructure.
type MockFulcrumClient struct {
	mu          sync.RWMutex
	agentStatus string
	agentInfo   map[string]any
	jobs        []*Job
	jobMap      map[string]*Job
	service     map[string]Service
}

func NewMockFulcrumClient() *MockFulcrumClient {
	return &MockFulcrumClient{
		agentStatus: "Connected",
		jobMap:      make(map[string]*Job),
		agentInfo: map[string]any{
			"id":   "test-agent-id",
			"name": "test-agent",
		},
		service: make(map[string]Service),
	}
}

func (c *MockFulcrumClient) UpdateAgentStatus(status string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.agentStatus = status
	return nil
}

func (c *MockFulcrumClient) GetAgentStatus() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.agentStatus
}

func (c *MockFulcrumClient) GetAgentInfo() (map[string]any, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	info := make(map[string]any, len(c.agentInfo))
	for k, v := range c.agentInfo {
		info[k] = v
	}
	return info, nil
}

func (c *MockFulcrumClient) SetAgentInfo(info map[string]any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.agentInfo = info
}

func (c *MockFulcrumClient) GetPendingJobs() ([]*Job, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var pending []*Job
	for _, job := range c.jobs {
		if job.Status == JobStatusPending {
			copy := *job
			pending = append(pending, &copy)
		}
	}
	return pending, nil
}

func (c *MockFulcrumClient) PullCompletedJobs() []*Job {
	return c.PullJobs(JobStatusCompleted)
}

func (c *MockFulcrumClient) PullFailedJobs() []*Job {
	return c.PullJobs(JobStatusFailed)
}

func (c *MockFulcrumClient) PullJobs(status JobStatus) []*Job {
	c.mu.Lock()
	defer c.mu.Unlock()
	var pulled, remaining []*Job
	for _, job := range c.jobs {
		if job.Status == status {
			pulled = append(pulled, job)
			delete(c.jobMap, job.ID)
		} else {
			remaining = append(remaining, job)
		}
	}
	c.jobs = remaining
	return pulled
}

func (c *MockFulcrumClient) ClaimJob(jobID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	job, ok := c.jobMap[jobID]
	if !ok || job.Status != JobStatusPending {
		return fmt.Errorf("job %s is not pending", jobID)
	}
	job.Status = JobStatusProcessing
	return nil
}

func (c *MockFulcrumClient) CompleteJob(jobID string, response JobResponse) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	job, ok := c.jobMap[jobID]
	if !ok || job.Status != JobStatusProcessing {
		return fmt.Errorf("job %s is not processing", jobID)
	}
	service := c.service[job.ServiceID]
	if response.AgentInstanceData != nil {
		service.AgentInstanceData = response.AgentInstanceData
	}
	if response.AgentInstanceID != nil {
		service.AgentInstanceID = response.AgentInstanceID
	}
	switch job.Action {
	case JobActionServiceCreate:
		service.Status = ServiceCreated
	case JobActionServiceStart:
		service.Status = ServiceStarted
	case JobActionServiceStop:
		service.Status = ServiceStopped
	case JobActionServiceDelete:
		service.Status = ServiceDeleted
	case JobActionServiceUpdate:
		service.Properties = job.Params
	}
	c.service[service.ID] = service
	job.Status = JobStatusCompleted
	return nil
}

func (c *MockFulcrumClient) FailJob(jobID string, message string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	job, ok := c.jobMap[jobID]
	if !ok || job.Status != JobStatusProcessing {
		return fmt.Errorf("job %s is not processing", jobID)
	}
	job.Status = JobStatusFailed
	job.ErrorMessage = message
	return nil
}

func (c *MockFulcrumClient) ReportMetric(metric *MetricEntry) error {
	fmt.Printf("Metric %s %s %s=%v", metric.AgentInstanceID, metric.ResourceID, metric.TypeName, metric.Value)
	return nil
}

func (c *MockFulcrumClient) CreateService(id, name string, instanceID *string, properties *Properties) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.service[id]; ok {
		return fmt.Errorf("service %s already exists", id)
	}
	service := Service{
		ID:              id,
		Name:            name,
		AgentInstanceID: instanceID,
		Properties:      properties,
		Status:          ServiceNew,
		ServiceType: &ServiceType{LifecycleSchema: &LifecycleSchema{
			RunningStates: []ServiceStatus{ServiceStarted},
		}},
	}
	c.service[id] = service
	return c.enqueueServiceJob(service, JobActionServiceCreate, properties)
}

func (c *MockFulcrumClient) StartService(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	service, ok := c.service[id]
	if !ok || (service.Status != ServiceCreated && service.Status != ServiceStopped) {
		return fmt.Errorf("service %s must be created or stopped before starting", id)
	}
	return c.enqueueServiceJob(service, JobActionServiceStart, nil)
}

func (c *MockFulcrumClient) StopService(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	service, ok := c.service[id]
	if !ok || service.Status != ServiceStarted {
		return fmt.Errorf("service %s must be started before stopping", id)
	}
	return c.enqueueServiceJob(service, JobActionServiceStop, nil)
}

func (c *MockFulcrumClient) UpdateService(id string, properties *Properties) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	service, ok := c.service[id]
	if !ok || (service.Status != ServiceStarted && service.Status != ServiceStopped) {
		return fmt.Errorf("service %s must be started or stopped before updating", id)
	}
	return c.enqueueServiceJob(service, JobActionServiceUpdate, properties)
}

func (c *MockFulcrumClient) DeleteService(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	service, ok := c.service[id]
	if !ok || service.Status != ServiceStopped {
		return fmt.Errorf("service %s must be stopped before deletion", id)
	}
	return c.enqueueServiceJob(service, JobActionServiceDelete, nil)
}

func (c *MockFulcrumClient) GetService(id string) (Service, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	service, ok := c.service[id]
	if !ok {
		return Service{}, fmt.Errorf("service %s not found", id)
	}
	return service, nil
}

func (c *MockFulcrumClient) enqueueServiceJob(service Service, action JobAction, params *Properties) error {
	// Core's pending-job query preloads Service but not Service.ServiceType.
	service.ServiceType = nil
	return c.enqueueJob(&Job{
		ID:        fmt.Sprintf("job-%s-%s-%d", service.ID, action, time.Now().UnixNano()),
		ServiceID: service.ID,
		Action:    action,
		Params:    params,
		Status:    JobStatusPending,
		Priority:  1,
		Service:   service,
	})
}

func (c *MockFulcrumClient) EnqueueJob(job *Job) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.enqueueJob(job)
}

func (c *MockFulcrumClient) enqueueJob(job *Job) error {
	if _, ok := c.jobMap[job.ID]; ok {
		return fmt.Errorf("job %s already exists", job.ID)
	}
	c.jobs = append(c.jobs, job)
	c.jobMap[job.ID] = job
	return nil
}

func (c *MockFulcrumClient) GetServices(_ int) (*ServicesResponse, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	services := make([]*Service, 0, len(c.service))
	for _, service := range c.service {
		copy := service
		services = append(services, &copy)
	}
	return &ServicesResponse{
		Items:       services,
		TotalItems:  len(services),
		TotalPages:  1,
		CurrentPage: 1,
	}, nil
}
