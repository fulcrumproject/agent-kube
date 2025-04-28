package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
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
func (c *HTTPProxmoxClient) CloneVM(templateID int, newVMID int, name string) (*agent.VMCloneResponse, error) {
	log.Printf("Cloning VM template %d to new VM %d with name %s", templateID, newVMID, name)

	// Prepare form parameters
	form := url.Values{}
	form.Add("newid", strconv.Itoa(newVMID))
	form.Add("full", "1")
	form.Add("storage", c.storageType)
	form.Add("name", name)

	// Construct endpoint URL
	endpoint := fmt.Sprintf("/api2/json/nodes/%s/qemu/%d/clone", c.hostName, templateID)

	// Execute request using the base client's PostForm method
	resp, err := c.httpClient.PostForm(endpoint, form)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to clone VM, status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var cloneResp struct {
		Data string `json:"data"` // Proxmox returns task ID as a string
	}

	if err := json.NewDecoder(resp.Body).Decode(&cloneResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Extract task ID from response
	taskID := cloneResp.Data

	return &agent.VMCloneResponse{
		TaskID: taskID,
		VMID:   newVMID,
	}, nil
}

// ConfigureVM configures a VM (CPU, memory, cloud-init)
func (c *HTTPProxmoxClient) ConfigureVM(vmID int, cores int, memory int, cloudInitConfig string) error {
	log.Printf("Configuring VM %d: cores=%d, memory=%d", vmID, cores, memory)

	// Prepare form parameters
	form := url.Values{}
	form.Add("cores", strconv.Itoa(cores))
	form.Add("memory", strconv.Itoa(memory))

	// Add cloud-init configuration if provided
	if cloudInitConfig != "" {
		form.Add("cicustom", cloudInitConfig)
	}

	// Construct endpoint URL
	endpoint := fmt.Sprintf("/api2/json/nodes/%s/qemu/%d/config", c.hostName, vmID)

	// Execute request using the base client's PostForm method
	resp, err := c.httpClient.PostForm(endpoint, form)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to configure VM, status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// StartVM starts a virtual machine
func (c *HTTPProxmoxClient) StartVM(vmID int) error {
	log.Printf("Starting VM %d", vmID)

	// Construct endpoint URL
	endpoint := fmt.Sprintf("/api2/json/nodes/%s/qemu/%d/status/start", c.hostName, vmID)

	// Send empty form data for POST request
	form := url.Values{}
	resp, err := c.httpClient.PostForm(endpoint, form)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()
	// Check response
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to start VM, status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// StopVM stops a virtual machine
func (c *HTTPProxmoxClient) StopVM(vmID int) error {
	log.Printf("Stopping VM %d", vmID)

	// Construct endpoint URL
	endpoint := fmt.Sprintf("/api2/json/nodes/%s/qemu/%d/status/stop", c.hostName, vmID)

	// Send empty form data for POST request
	form := url.Values{}
	resp, err := c.httpClient.PostForm(endpoint, form)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()
	// Check response
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to stop VM, status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// DeleteVM deletes a virtual machine
func (c *HTTPProxmoxClient) DeleteVM(vmID int) error {
	log.Printf("Deleting VM %d", vmID)

	// Construct endpoint URL
	endpoint := fmt.Sprintf("/api2/json/nodes/%s/qemu/%d", c.hostName, vmID)

	// Use the Delete method from the base client
	resp, err := c.httpClient.Delete(endpoint)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()
	// Check response
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete VM, status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// GetVMInfo gets information about a VM
func (c *HTTPProxmoxClient) GetVMInfo(vmID int) (*agent.VMInfo, error) {
	log.Printf("Getting info for VM %d", vmID)

	// Construct endpoint URL
	endpoint := fmt.Sprintf("/api2/json/nodes/%s/qemu/%d/config", c.hostName, vmID)

	// Use the Get method from the base client
	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()
	// Check response
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get VM info, status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var configResp struct {
		Data struct {
			Name   string `json:"name"`
			Memory int    `json:"memory"`
			Cores  int    `json:"cores"`
			// Other fields not directly mapped to VMInfo
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&configResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Now get the status to complete the VM info
	status, err := c.GetVMStatus(vmID)
	if err != nil {
		return nil, fmt.Errorf("failed to get VM status: %w", err)
	}

	return &agent.VMInfo{
		ID:     vmID,
		Name:   configResp.Data.Name,
		Status: status.Status,
		CPU:    float64(configResp.Data.Cores),
		Memory: configResp.Data.Memory,
		Node:   c.hostName,
	}, nil
}

// GetVMNetworkInterfaces gets network interface information from a VM
func (c *HTTPProxmoxClient) GetVMNetworkInterfaces(vmID int) ([]agent.VMNetworkInterface, error) {
	log.Printf("Getting network interfaces for VM %d", vmID)

	// Construct endpoint URL
	endpoint := fmt.Sprintf("/api2/json/nodes/%s/qemu/%d/agent/network-get-interfaces", c.hostName, vmID)

	// Use the Get method from the base client
	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()
	// Check response
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get VM network interfaces, status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var interfacesResp struct {
		Data struct {
			Result []struct {
				Name         string `json:"name"`
				HardwareAddr string `json:"hardware-address"`
				IPAddresses  []struct {
					IPAddressType string `json:"ip-address-type"`
					IPAddress     string `json:"ip-address"`
				} `json:"ip-addresses"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&interfacesResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to agent.VMNetworkInterface structures
	var interfaces []agent.VMNetworkInterface
	for _, iface := range interfacesResp.Data.Result {
		var ipAddresses []string
		for _, ip := range iface.IPAddresses {
			if ip.IPAddressType == "ipv4" {
				ipAddresses = append(ipAddresses, ip.IPAddress)
			}
		}

		interfaces = append(interfaces, agent.VMNetworkInterface{
			Name:        iface.Name,
			MACAddress:  iface.HardwareAddr,
			IPAddresses: ipAddresses,
		})
	}

	return interfaces, nil
}

// GetVMStatus gets the current status of a VM
func (c *HTTPProxmoxClient) GetVMStatus(vmID int) (*agent.VMStatus, error) {
	log.Printf("Getting status for VM %d", vmID)

	// Construct endpoint URL
	endpoint := fmt.Sprintf("/api2/json/nodes/%s/qemu/%d/status/current", c.hostName, vmID)

	// Use the Get method from the base client
	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()
	// Check response
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get VM status, status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var statusResp struct {
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&statusResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &agent.VMStatus{
		Status: statusResp.Data.Status,
		VMID:   vmID,
		Node:   c.hostName,
	}, nil
}

// WaitForVMStatus waits for a VM to reach a specific status
func (c *HTTPProxmoxClient) WaitForVMStatus(vmID int, targetStatus string, timeoutSec int) error {
	log.Printf("Waiting for VM %d to reach status %s (timeout: %d seconds)", vmID, targetStatus, timeoutSec)

	// Calculate the deadline
	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)

	// Check the status periodically
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		select {
		case <-ticker.C:
			status, err := c.GetVMStatus(vmID)
			if err != nil {
				log.Printf("Error getting VM status: %v", err)
				continue
			}

			log.Printf("VM %d status: %s", vmID, status.Status)

			if status.Status == targetStatus {
				log.Printf("VM %d has reached target status: %s", vmID, targetStatus)
				return nil
			}
		}
	}

	return fmt.Errorf("timed out waiting for VM %d to reach status %s", vmID, targetStatus)
}

// CreateCloudInitSnippet creates a cloud-init snippet file on the Proxmox host
func (c *HTTPProxmoxClient) CreateCloudInitSnippet(vmName string, config agent.CloudInitConfig) (string, error) {
	// This would typically use SSH to create the file on the host
	// For this implementation, we'll return a formatted cloud-init custom string

	// Format SSH keys as a single string with newlines
	sshKeysStr := strings.Join(config.SSHKeys, "\n  - ")
	if sshKeysStr != "" {
		sshKeysStr = "  - " + sshKeysStr
	}

	// Format join command if join config is provided
	runcmds := "runcmd:"
	if config.JoinConfig != nil {
		joinCmd := fmt.Sprintf("  - curl -sfL https://goyaki.clastix.io | sudo JOIN_URL=%s JOIN_TOKEN=%s JOIN_TOKEN_CACERT_HASH=%s JOIN_ASCP=1 KUBERNETES_VERSION=%s bash -s join",
			config.JoinConfig.JoinURL,
			config.JoinConfig.JoinToken,
			config.JoinConfig.JoinTokenHash,
			config.JoinConfig.KubernetesVersion)
		runcmds += "\n" + joinCmd
	}

	// Return the formatted cloud-init snippet path
	snippetName := fmt.Sprintf("kube-agent-user-ci-%s.yml", vmName)
	return fmt.Sprintf("user=local:snippets/%s", snippetName), nil
}
