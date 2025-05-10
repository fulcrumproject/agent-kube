package fulcrum

import (
	"encoding/json"
	"fmt"
	"net/http"

	"fulcrumproject.org/kube-agent/internal/agent"
	"fulcrumproject.org/kube-agent/internal/httpcli"
)

// HTTPFulcrumClient implements FulcrumClient interface using HTTP
type HTTPFulcrumClient struct {
	httpClient *httpcli.Client
}

// NewFulcrumClient creates a new Fulcrum API client
func NewFulcrumClient(baseURL string, token string, options ...httpcli.ClientOption) *HTTPFulcrumClient {
	return &HTTPFulcrumClient{
		httpClient: httpcli.NewHTTPClient(baseURL, token, options...),
	}
}

// UpdateAgentStatus updates the agent's status in Fulcrum Core
func (c *HTTPFulcrumClient) UpdateAgentStatus(status string) error {
	reqBody, err := json.Marshal(map[string]any{
		"state": status,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal status update request: %w", err)
	}

	resp, err := c.httpClient.Put("/api/v1/agents/me/status", reqBody)
	if err != nil {
		return fmt.Errorf("failed to update agent status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to update agent status, status: %d", resp.StatusCode)
	}

	return nil
}

// GetAgentInfo retrieves the agent's information from Fulcrum Core
func (c *HTTPFulcrumClient) GetAgentInfo() (map[string]any, error) {
	resp, err := c.httpClient.Get("/api/v1/agents/me")
	if err != nil {
		return nil, fmt.Errorf("failed to get agent info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get agent info, status: %d", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode agent info response: %w", err)
	}

	return result, nil
}

// GetPendingJobs retrieves pending jobs for this agent
func (c *HTTPFulcrumClient) GetPendingJobs() ([]*agent.Job, error) {
	resp, err := c.httpClient.Get("/api/v1/jobs/pending")
	if err != nil {
		return nil, fmt.Errorf("failed to get pending jobs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get pending jobs, status: %d", resp.StatusCode)
	}

	var jobs []*agent.Job

	if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
		return nil, fmt.Errorf("failed to decode jobs response: %w", err)
	}

	return jobs, nil
}

// ClaimJob claims a job for processing
func (c *HTTPFulcrumClient) ClaimJob(jobID string) error {
	resp, err := c.httpClient.Post(fmt.Sprintf("/api/v1/jobs/%s/claim", jobID), nil)
	if err != nil {
		return fmt.Errorf("failed to claim job: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to claim job, status: %d", resp.StatusCode)
	}

	return nil
}

// CompleteJob marks a job as completed with results
func (c *HTTPFulcrumClient) CompleteJob(jobID string, response agent.JobResponse) error {
	reqBody, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal job completion request: %w", err)
	}

	resp, err := c.httpClient.Post(fmt.Sprintf("/api/v1/jobs/%s/complete", jobID), reqBody)
	if err != nil {
		return fmt.Errorf("failed to complete job: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to complete job, status: %d", resp.StatusCode)
	}

	return nil
}

// FailJob marks a job as failed with an error message
func (c *HTTPFulcrumClient) FailJob(jobID string, errorMessage string) error {
	reqBody, err := json.Marshal(map[string]any{
		"errorMessage": errorMessage,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal job failure request: %w", err)
	}

	resp, err := c.httpClient.Post(fmt.Sprintf("/api/v1/jobs/%s/fail", jobID), reqBody)
	if err != nil {
		return fmt.Errorf("failed to mark job as failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to mark job as failed, status: %d", resp.StatusCode)
	}

	return nil
}

// ReportMetrics sends collected metrics to Fulcrum Core
func (c *HTTPFulcrumClient) ReportMetric(metric *agent.MetricEntry) error {
	reqBody, err := json.Marshal(metric)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics request: %w", err)
	}

	resp, err := c.httpClient.Post("/api/v1/metric-entries", reqBody)
	if err != nil {
		return fmt.Errorf("failed to report metrics: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to report metrics, status: %d", resp.StatusCode)
	}

	return nil
}

// GetServices retrieves all services from Fulcrum Core
func (c *HTTPFulcrumClient) GetServices() ([]*agent.Service, error) {
	resp, err := c.httpClient.Get("/api/v1/services")
	if err != nil {
		return nil, fmt.Errorf("failed to get services: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get services, status: %d", resp.StatusCode)
	}

	var services []*agent.Service
	if err := json.NewDecoder(resp.Body).Decode(&services); err != nil {
		return nil, fmt.Errorf("failed to decode services response: %w", err)
	}

	return services, nil
}
