package agent

import (
	"errors"
	"fmt"
	"log"
	"time"
)

// JobHandler processes jobs from the Fulcrum Core job queue
type JobHandler struct {
	fulcrumCli FulcrumClient
	proxmoxCli ProxmoxClient
	kamajiCli  KamajiClient
	sshCli     SSHClient
}

// JobResources represents the resources in a job response
type JobResources struct {
	TS time.Time `json:"ts"`
}

// JobResponse represents the response for a job
type JobResponse struct {
	Resources  JobResources `json:"resources"`
	ExternalID *string      `json:"externalId"`
}

// NewJobHandler creates a new job handler
func NewJobHandler(
	fulcrumCli FulcrumClient,
	proxmoxCli ProxmoxClient,
	kamajiCli KamajiClient,
	sshCli SSHClient,
) *JobHandler {
	return &JobHandler{
		fulcrumCli: fulcrumCli,
		proxmoxCli: proxmoxCli,
		kamajiCli:  kamajiCli,
		sshCli:     sshCli,
	}
}

// PollAndProcessJobs polls for pending jobs and processes them
func (h *JobHandler) PollAndProcessJobs() error {
	// Get pending jobs
	jobs, err := h.fulcrumCli.GetPendingJobs()
	if err != nil {
		return fmt.Errorf("failed to get pending jobs: %w", err)
	}

	if len(jobs) == 0 {
		log.Printf("Pending jobs not found")
		return nil
	}
	// First
	job := jobs[0]
	// Increment processed count
	// Claim the job
	if err := h.fulcrumCli.ClaimJob(job.ID); err != nil {
		log.Printf("Failed to claim job %s: %v", job.ID, err)
		return err
	}
	log.Printf("Processing job %s of type %s", job.ID, job.Action)
	// Process the job
	resp, err := h.processJob(job)
	if err != nil {
		// Mark job as failed
		log.Printf("Job %s failed: %v", job.ID, err)

		if failErr := h.fulcrumCli.FailJob(job.ID, err.Error()); failErr != nil {
			log.Printf("Failed to mark job %s as failed: %v", job.ID, failErr)
			return failErr
		}
	} else {
		// Job succeeded
		if complErr := h.fulcrumCli.CompleteJob(job.ID, resp); complErr != nil {
			log.Printf("Failed to mark job %s as completed: %v", job.ID, complErr)
			return complErr
		}
		log.Printf("Job %s completed successfully", job.ID)
	}

	return nil
}

// processJob processes a job based on its type
func (h *JobHandler) processJob(job *Job) (any, error) {
	switch job.Action {
	case JobActionServiceCreate:
		return nil, errors.New("not supported")
	case JobActionServiceColdUpdate, JobActionServiceHotUpdate:
		return nil, errors.New("not supported")
	case JobActionServiceStart:
		return nil, errors.New("not supported")
	case JobActionServiceStop:
		return nil, errors.New("not supported")
	case JobActionServiceDelete:
		return nil, errors.New("not supported")
	default:
		return nil, fmt.Errorf("unknown job type: %s", job.Action)
	}
}
