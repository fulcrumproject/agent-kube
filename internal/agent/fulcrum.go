package agent

// ServiceStatus represents the possible statuss of a service
type ServiceStatus string

const (
	ServiceCreating     ServiceStatus = "Creating"
	ServiceCreated      ServiceStatus = "Created"
	ServiceStarting     ServiceStatus = "Starting"
	ServiceStarted      ServiceStatus = "Started"
	ServiceStopping     ServiceStatus = "Stopping"
	ServiceStopped      ServiceStatus = "Stopped"
	ServiceHotUpdating  ServiceStatus = "HotUpdating"
	ServiceColdUpdating ServiceStatus = "ColdUpdating"
	ServiceDeleting     ServiceStatus = "Deleting"
	ServiceDeleted      ServiceStatus = "Deleted"
)

// JobAction represents the type of job
type JobAction string

const (
	JobActionServiceCreate     JobAction = "ServiceCreate"
	JobActionServiceStart      JobAction = "ServiceStart"
	JobActionServiceStop       JobAction = "ServiceStop"
	JobActionServiceHotUpdate  JobAction = "ServiceHotUpdate"
	JobActionServiceColdUpdate JobAction = "ServiceColdUpdate"
	JobActionServiceDelete     JobAction = "ServiceDelete"
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

// Resources represents the resources in a job response
type Resources struct {
	ClusterIP  string         `json:"clusterIp,omitempty"`
	KubeConfig string         `json:"kubeConfig,omitempty"`
	Nodes      map[string]int `json:"nodes,omitempty"`
}

// Properties represents the properties of a service
type Properties struct {
	Nodes []Node `json:"nodes"`
}
type Service struct {
	ID                string         `json:"id"`
	Name              string         `json:"name"`
	ExternalID        *string        `json:"externalId"`
	CurrentProperties *Properties    `json:"currentProperties"`
	TargetProperties  *Properties    `json:"targetProperties"`
	Resources         *Resources     `json:"resources"`
	CurrentStatus     ServiceStatus  `json:"currentStatus"`
	TargetStatus      *ServiceStatus `json:"targetStatus"`
}

// Job represents a job from the Fulcrum Core job queue
type Job struct {
	ID           string    `json:"id"`
	Action       JobAction `json:"action"`
	Status       JobStatus `json:"status"`
	Priority     int       `json:"priority"`
	Service      Service   `json:"service"`
	ErrorMessage string    `json:"errorMessage"`
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
	ExternalID string     `json:"externalId"`
	ResourceID string     `json:"resourceId"`
	Value      float64    `json:"value"`
	TypeName   MetricType `json:"typeName"`
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
	GetPendingJobs() ([]*Job, error)
	ClaimJob(jobID string) error
	CompleteJob(jobID string, response JobResponse) error
	FailJob(jobID string, errorMessage string) error
	ReportMetric(metrics *MetricEntry) error
}
