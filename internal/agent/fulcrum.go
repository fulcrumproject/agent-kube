package agent

// ServiceState represents the possible states of a service
type ServiceState string

const (
	ServiceCreating     ServiceState = "Creating"
	ServiceCreated      ServiceState = "Created"
	ServiceStarting     ServiceState = "Starting"
	ServiceStarted      ServiceState = "Started"
	ServiceStopping     ServiceState = "Stopping"
	ServiceStopped      ServiceState = "Stopped"
	ServiceHotUpdating  ServiceState = "HotUpdating"
	ServiceColdUpdating ServiceState = "ColdUpdating"
	ServiceDeleting     ServiceState = "Deleting"
	ServiceDeleted      ServiceState = "Deleted"
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

// JobState represents the state of a job
type JobState string

const (
	JobStatePending    JobState = "Pending"
	JobStateProcessing JobState = "Processing"
	JobStateCompleted  JobState = "Completed"
	JobStateFailed     JobState = "Failed"
)

// NodeState
type NodeState string

const (
	NodeStateOn  NodeState = "On"
	NodeStateOff NodeState = "Off"
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
	ID    string    `json:"id"`
	Size  NodeSize  `json:"size"`
	State NodeState `json:"state"`
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
	ID                string        `json:"id"`
	Name              string        `json:"name"`
	ExternalID        *string       `json:"externalId"`
	CurrentProperties *Properties   `json:"currentProperties"`
	TargetProperties  *Properties   `json:"targetProperties"`
	Resources         *Resources    `json:"resources"`
	CurrentState      ServiceState  `json:"currentState"`
	TargetState       *ServiceState `json:"targetState"`
}

// Job represents a job from the Fulcrum Core job queue
type Job struct {
	ID           string    `json:"id"`
	Action       JobAction `json:"action"`
	State        JobState  `json:"state"`
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

// FulcrumClient defines the interface for communication with the Fulcrum Core API
type FulcrumClient interface {
	UpdateAgentStatus(status string) error
	GetAgentInfo() (map[string]any, error)
	GetServices() ([]*Service, error)
	GetPendingJobs() ([]*Job, error)
	ClaimJob(jobID string) error
	CompleteJob(jobID string, response JobResponse) error
	FailJob(jobID string, errorMessage string) error
	ReportMetric(metrics *MetricEntry) error
}
