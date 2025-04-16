package agent

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

// Job represents a job from the Fulcrum Core job queue
type Job struct {
	ID       string    `json:"id"`
	Action   JobAction `json:"action"`
	State    JobState  `json:"state"`
	Priority int       `json:"priority"`
	Service  struct {
		ID                string  `json:"id"`
		Name              string  `json:"name"`
		ExternalID        *string `json:"externalId"`
		CurrentProperties *struct {
			CPU    int `json:"cpu"`
			Memory int `json:"memory"`
		} `json:"currentProperties"`
		TargetProperties *struct {
			CPU    int `json:"cpu"`
			Memory int `json:"memory"`
		} `json:"targetProperties"`
	} `json:"service"`
}

// MetricEntry represents a single metric measurement
type MetricEntry struct {
	ExternalID string  `json:"externalId"`
	ResourceID string  `json:"resourceId"`
	Value      float64 `json:"value"`
	TypeName   string  `json:"typeName"`
}

// FulcrumClient defines the interface for communication with the Fulcrum Core API
type FulcrumClient interface {
	UpdateAgentStatus(status string) error
	GetAgentInfo() (map[string]any, error)
	GetPendingJobs() ([]*Job, error)
	ClaimJob(jobID string) error
	CompleteJob(jobID string, resources any) error
	FailJob(jobID string, errorMessage string) error
	ReportMetric(metrics *MetricEntry) error
}
