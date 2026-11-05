// Defines the Core API payloads used by the Kubernetes agent.
// Service states are defined by each service type's lifecycle schema.
package agent

import (
	"fmt"
	"slices"
)

// ServiceStatus is a state name from a service type's lifecycle.
type ServiceStatus string

const (
	// These names belong to the example lifecycle, not to Core's API contract.
	ServiceNew     ServiceStatus = "New"
	ServiceCreated ServiceStatus = "Created"
	ServiceStarted ServiceStatus = "Started"
	ServiceStopped ServiceStatus = "Stopped"
	ServiceDeleted ServiceStatus = "Deleted"
)

// JobAction represents the type of job
type JobAction string

const (
	JobActionServiceCreate JobAction = "create"
	JobActionServiceStart  JobAction = "start"
	JobActionServiceStop   JobAction = "stop"
	JobActionServiceUpdate JobAction = "update"
	JobActionServiceDelete JobAction = "delete"
)

// JobStatus represents the status of a job
type JobStatus string

const (
	JobStatusPending    JobStatus = "Pending"
	JobStatusProcessing JobStatus = "Processing"
	JobStatusCompleted  JobStatus = "Completed"
	JobStatusFailed     JobStatus = "Failed"
)

// NodeStatus
type NodeStatus string

const (
	NodeStatusOn  NodeStatus = "On"
	NodeStatusOff NodeStatus = "Off"
)

// NodeSize
type NodeSize string

const (
	NodeSizeS1 NodeSize = "s1"
	NodeSizeS2 NodeSize = "s2"
	NodeSizeS4 NodeSize = "s4"
)

func (size NodeSize) Attrs() (cores int, memory int) {
	switch size {
	case NodeSizeS1:
		return 2, 2048
	case NodeSizeS2:
		return 4, 4096
	case NodeSizeS4:
		return 8, 8192
	default:
		return 2, 2048
	}
}

type Node struct {
	ID     string     `json:"id"`
	Size   NodeSize   `json:"size"`
	Status NodeStatus `json:"status"`
}

// AgentInstanceData holds the cluster resources managed by this agent.
type AgentInstanceData struct {
	ClusterIP  string         `json:"clusterIp,omitempty"`
	KubeConfig string         `json:"kubeConfig,omitempty"`
	Nodes      map[string]int `json:"nodes,omitempty"`
}

// Properties represents the properties of a service
type Properties struct {
	Nodes []Node `json:"nodes"`
}
type Service struct {
	ID                string             `json:"id"`
	Name              string             `json:"name"`
	AgentInstanceID   *string            `json:"agentInstanceId,omitempty"`
	Properties        *Properties        `json:"properties,omitempty"`
	AgentInstanceData *AgentInstanceData `json:"agentInstanceData,omitempty"`
	Status            ServiceStatus      `json:"status"`
	ServiceType       *ServiceType       `json:"serviceType,omitempty"`
}

type ServiceType struct {
	LifecycleSchema *LifecycleSchema `json:"lifecycleSchema"`
}

type LifecycleSchema struct {
	RunningStates []ServiceStatus `json:"runningStates"`
}

// IsRunning uses the service type's schema, never a hardcoded state name.
func (s Service) IsRunning() (bool, error) {
	if s.ServiceType == nil || s.ServiceType.LifecycleSchema == nil {
		return false, fmt.Errorf("service %s has no lifecycle schema", s.ID)
	}
	return slices.Contains(s.ServiceType.LifecycleSchema.RunningStates, s.Status), nil
}

// Job represents a job from the Fulcrum Core job queue
type Job struct {
	ID           string      `json:"id"`
	ServiceID    string      `json:"serviceId"`
	Params       *Properties `json:"params,omitempty"`
	Action       JobAction   `json:"action"`
	Status       JobStatus   `json:"status"`
	Priority     int         `json:"priority"`
	Service      Service     `json:"service"`
	ErrorMessage string      `json:"errorMessage"`
}

type MetricType string

const (
	MetricTypeVMCPUUsage          MetricType = "vm.cpu.usage"
	MetricTypeVMMemoryUsage       MetricType = "vm.memory.usage"
	MetricTypeVMDiskUsage         MetricType = "vm.disk.usage"
	MetricTypeVMNetworkThroughput MetricType = "vm.network.throughput"
)

// MetricEntry represents a single metric measurement
type MetricEntry struct {
	AgentInstanceID string     `json:"agentInstanceId"`
	ResourceID      string     `json:"resourceId"`
	Value           float64    `json:"value"`
	TypeName        MetricType `json:"typeName"`
}

// ServicesResponse represents the paginated response for services
type ServicesResponse struct {
	Items       []*Service `json:"items"`
	TotalItems  int        `json:"totalItems"`
	TotalPages  int        `json:"totalPages"`
	CurrentPage int        `json:"currentPage"`
	HasNext     bool       `json:"hasNext"`
	HasPrev     bool       `json:"hasPrev"`
}

// FulcrumClient defines the interface for communication with the Fulcrum Core API
type FulcrumClient interface {
	UpdateAgentStatus(status string) error
	GetAgentInfo() (map[string]any, error)
	GetServices(page int) (*ServicesResponse, error)
	GetService(id string) (Service, error)
	GetPendingJobs() ([]*Job, error)
	ClaimJob(jobID string) error
	CompleteJob(jobID string, response JobResponse) error
	FailJob(jobID string, errorMessage string) error
	ReportMetric(metrics *MetricEntry) error
}
