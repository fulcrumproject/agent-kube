package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"fulcrumproject.org/kube-agent/internal/agent"
)

// HTTPProxmoxClient implements the agent.ProxmoxClient interface
type HTTPProxmoxClient struct {
	httpClient  *Client
	hostName    string // Proxmox node name (e.g., "pve")
	storageType string // Default storage type (e.g., "local-lvm")
}

// NewProxmoxClient creates a new Proxmox API client
func NewProxmoxClient(baseURL string, token string, hostName string, storageType string, options ...ClientOption) *HTTPProxmoxClient {
	// Add PVE auth type to the provided options
	allOptions := append([]ClientOption{
		WithAuthType(AuthTypePVE), // Use PVE auth type for Proxmox
	}, options...)

	client := &HTTPProxmoxClient{
		httpClient:  NewHTTPClient(baseURL, token, allOptions...),
		hostName:    hostName,
		storageType: storageType,
	}

	return client
}

// CloneVM creates a new VM by cloning from a template
func (c *HTTPProxmoxClient) CloneVM(templateID int, newVMID int, name string) (*agent.TaskResponse, error) {
	form := url.Values{}
	form.Add("newid", strconv.Itoa(newVMID))
	form.Add("full", "1")
	form.Add("storage", c.storageType)
	form.Add("name", name)

	endpoint := fmt.Sprintf("/api2/json/nodes/%s/qemu/%d/clone", c.hostName, templateID)

	return c.post(endpoint, form)
}

// ConfigureVM configures a VM (CPU, memory, cloud-init)
func (c *HTTPProxmoxClient) ConfigureVM(vmID int, cores int, memory int, cloudInitConfig string) (*agent.TaskResponse, error) {
	form := url.Values{}
	form.Add("cores", strconv.Itoa(cores))
	form.Add("memory", strconv.Itoa(memory))

	// Add cloud-init configuration if provided
	if cloudInitConfig != "" {
		form.Add("cicustom", cloudInitConfig)
	}

	endpoint := fmt.Sprintf("/api2/json/nodes/%s/qemu/%d/config", c.hostName, vmID)

	return c.post(endpoint, form)
}

// StartVM starts a virtual machine
func (c *HTTPProxmoxClient) StartVM(vmID int) (*agent.TaskResponse, error) {
	endpoint := fmt.Sprintf("/api2/json/nodes/%s/qemu/%d/status/start", c.hostName, vmID)

	return c.post(endpoint, url.Values{})
}

// StopVM stops a virtual machine
func (c *HTTPProxmoxClient) StopVM(vmID int) (*agent.TaskResponse, error) {
	endpoint := fmt.Sprintf("/api2/json/nodes/%s/qemu/%d/status/stop", c.hostName, vmID)

	return c.post(endpoint, url.Values{})
}

// DeleteVM deletes a virtual machine
func (c *HTTPProxmoxClient) DeleteVM(vmID int) (*agent.TaskResponse, error) {
	endpoint := fmt.Sprintf("/api2/json/nodes/%s/qemu/%d", c.hostName, vmID)

	resp, err := c.httpClient.Delete(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	return taskResponse(resp)
}

// GetTaskStatus retrieves the current status of a task
func (c *HTTPProxmoxClient) GetTaskStatus(taskID string) (*agent.TaskStatus, error) {
	// Parse the UPID to extract components needed for the API call
	taskResp, err := parseUPID(taskID)
	if err != nil {
		return nil, fmt.Errorf("invalid task ID: %w", err)
	}

	// Construct the endpoint based on extracted node name
	nodeName := taskResp.NodeName
	if nodeName == "" {
		// If we couldn't parse the node name from the UPID, use the client's default host
		nodeName = c.hostName
	}

	endpoint := fmt.Sprintf("/api2/json/nodes/%s/tasks/%s/status", nodeName, taskID)

	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get task status, status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var statusResp struct {
		Data agent.TaskStatus `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&statusResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &statusResp.Data, nil
}

// WaitForTask waits for a task to complete and returns the task's status
func (c *HTTPProxmoxClient) WaitForTask(taskID string, timeout time.Duration) (*agent.TaskStatus, error) {
	// Use a ticker to poll the task status until complete or timeout
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	deadline := time.Now().Add(timeout)

	for {
		select {
		case <-ticker.C:
			status, err := c.GetTaskStatus(taskID)
			if err != nil {
				return nil, err
			}

			// Check if the task is complete
			if status.Status == "stopped" {
				return status, nil
			}

			// Check for timeout
			if time.Now().After(deadline) {
				return nil, fmt.Errorf("timeout waiting for task %s to complete", taskID)
			}
		}
	}
}

func (c *HTTPProxmoxClient) post(endpoint string, form url.Values) (*agent.TaskResponse, error) {
	resp, err := c.httpClient.PostForm(endpoint, form)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	return taskResponse(resp)
}

func taskResponse(resp *http.Response) (*agent.TaskResponse, error) {
	// Check response
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to clone VM, status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var taskResp struct {
		Data string `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&taskResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Extract task ID from response
	taskID := taskResp.Data

	// Parse the UPID and return populated response
	response, err := parseUPID(taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse task ID: %w", err)
	}
	return response, nil

}

// parseUPID parses the UPID string and returns a populated TaskResponse or an error if the UPID is invalid
// UPID format: UPID:<node_name>:<pid_in_hex>:<pstart_in_hex>:<starttime_in_hex>:<type>:<id (optional)>:<user>@<realm>:
func parseUPID(upid string) (*agent.TaskResponse, error) {
	response := &agent.TaskResponse{
		TaskID: upid,
	}

	// Validate UPID format
	if len(upid) <= 5 || upid[:5] != "UPID:" {
		return nil, fmt.Errorf("invalid UPID format: must start with 'UPID:'")
	}

	parts := strings.Split(upid, ":")

	// UPID must have at least 6 components (minimum format without ID)
	if len(parts) < 6 {
		return nil, fmt.Errorf("invalid UPID format: insufficient components, got %d, expected at least 6", len(parts))
	}

	i := 1
	response.NodeName = parts[i]
	i++
	response.PID = parts[i]
	i++
	response.PStart = parts[i]
	i++
	response.StartTime = parts[i]
	i++
	response.Type = parts[i]
	i++

	// Determine if ID field is present
	if len(parts) > 8 {
		response.ID = parts[i]
		i++
	}

	response.User = parts[i]

	return response, nil
}
