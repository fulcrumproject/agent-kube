package agent

import (
	"context"
	"fmt"
	"log"
	"time"
)

// JobHandler processes jobs from the Fulcrum Core job queue
type JobHandler struct {
	templateID int
	fulcrumCli FulcrumClient
	proxmoxCli ProxmoxClient
	kamajiCli  KamajiClient
	sshCli     SSHClient
}

// JobResponse represents the response for a job
type JobResponse struct {
	Resources  *Resources `json:"resources"`
	ExternalID *string    `json:"externalId"`
}

// NewJobHandler creates a new job handler
func NewJobHandler(
	fulcrumCli FulcrumClient,
	proxmoxCli ProxmoxClient,
	templateID int,
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
		if complErr := h.fulcrumCli.CompleteJob(job.ID, *resp); complErr != nil {
			log.Printf("Failed to mark job %s as completed: %v", job.ID, complErr)
			return complErr
		}
		log.Printf("Job %s completed successfully", job.ID)
	}

	return nil
}

// processJob processes a job based on its type
func (h *JobHandler) processJob(job *Job) (*JobResponse, error) {
	switch job.Action {
	case JobActionServiceCreate:
		return h.handleServiceCreate(job)
	case JobActionServiceColdUpdate:
		return h.handleServiceUpdate(job, false)
	case JobActionServiceHotUpdate:
		return h.handleServiceUpdate(job, true)
	case JobActionServiceStart:
		return h.handleServiceStart(job)
	case JobActionServiceStop:
		return h.handleServiceStop(job)
	case JobActionServiceDelete:
		return h.handleServiceDelete(job)
	default:
		return nil, fmt.Errorf("unknown job type: %s", job.Action)
	}
}

// handleServiceCreate creates a new cluster service
func (h *JobHandler) handleServiceCreate(job *Job) (*JobResponse, error) {
	ctx := context.Background()

	// Create response object
	resp := &JobResponse{
		Resources: &Resources{
			Nodes: make(map[string]int, 0),
		},
	}

	tenantName := job.Service.Name
	log.Printf("Creating tenant control plane: %s", tenantName)

	// Create tenant control plane
	err := h.kamajiCli.CreateTenantControlPlane(ctx, tenantName, "v1.30.2", 1)
	if err != nil {
		return nil, fmt.Errorf("failed to create tenant control plane: %w", err)
	}

	// Wait for tenant control plane to be ready
	err = h.kamajiCli.WaitForTenantControlPlaneReady(ctx, tenantName)
	if err != nil {
		return nil, fmt.Errorf("tenant control plane failed to initialize: %w", err)
	}

	// TODO should be a better way to check if the tenant is ready
	time.Sleep(30 * time.Second)

	// Get tenant client
	tenantClient, err := h.kamajiCli.GetTenantClient(ctx, tenantName)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant client: %w", err)
	}

	// Apply Calico networking
	err = tenantClient.CreateCalicoResources(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to apply Calico resources: %w", err)
	}

	// Get kubeconfig
	kubeConfig, err := h.kamajiCli.GetTenantKubeConfig(ctx, tenantName)
	if err != nil {
		return nil, fmt.Errorf("failed to get Kubernetes config: %w", err)
	}

	// Store kubeconfig and endpoint in response
	resp.Resources.ClusterIP = kubeConfig.Endpoint
	resp.Resources.KubeConfig = kubeConfig.Config

	// Create nodes if specified in the job
	if job.Service.TargetProperties != nil && job.Service.TargetProperties.Nodes != nil {
		for _, node := range job.Service.TargetProperties.Nodes {
			vmID, err := h.createVM(ctx, tenantName, node)
			if err != nil {
				return nil, fmt.Errorf("failed to create node %s: %w", node.ID, err)
			}
			resp.Resources.Nodes[node.ID] = vmID
		}
	}

	// Set external ID
	externalID := fmt.Sprintf("cluster-%s", tenantName)
	resp.ExternalID = &externalID

	return resp, nil
}

// handleServiceUpdate handles the updates to a service
// It adds or removes nodes based on the difference between current and target properties
// VMs will be started or stopped based on their target state if start is true
func (h *JobHandler) handleServiceUpdate(job *Job, startStop bool) (*JobResponse, error) {
	ctx := context.Background()
	resp := &JobResponse{
		Resources:  job.Service.Resources,
		ExternalID: job.Service.ExternalID,
	}

	// Check if job has target and current properties
	if job.Service.TargetProperties == nil {
		return nil, fmt.Errorf("target properties are nil")
	}
	if job.Service.CurrentProperties == nil {
		return nil, fmt.Errorf("current properties are nil")
	}

	// Get current and target nodes
	currentNodes := job.Service.CurrentProperties.Nodes
	targetNodes := job.Service.TargetProperties.Nodes

	// Create a map for quick lookup of current nodes
	currentNodesMap := make(map[string]Node)
	for _, node := range currentNodes {
		currentNodesMap[node.ID] = node
	}

	// Create a map for quick lookup of target nodes
	targetNodesMap := make(map[string]Node)
	for _, node := range targetNodes {
		targetNodesMap[node.ID] = node
	}

	// Check if there are changes in the node size
	for _, targetNode := range targetNodes {
		currentNode, exists := currentNodesMap[targetNode.ID]
		if exists && targetNode.Size != currentNode.Size {
			return nil, fmt.Errorf("changing VM size is not supported: node %s", targetNode.ID)
		}
	}

	// Get nodes to be added, removed and updated
	var nodesToAdd []Node
	var nodesToRemove []Node
	var nodesToStart []Node
	var nodesToStop []Node
	for _, targetNode := range targetNodes {
		if _, exists := currentNodesMap[targetNode.ID]; !exists {
			nodesToAdd = append(nodesToAdd, targetNode)
		} else {
			if startStop {
				if targetNode.State == NodeStateOn {
					nodesToStart = append(nodesToStart, targetNode)
				} else {
					nodesToStop = append(nodesToStop, targetNode)
				}
			}
		}
	}
	for _, currentNode := range currentNodes {
		if _, exists := targetNodesMap[currentNode.ID]; !exists {
			nodesToRemove = append(nodesToRemove, currentNode)
		}
	}

	tenantName := job.Service.Name

	// Get tenant client to manage worker nodes
	tenantClient, err := h.kamajiCli.GetTenantClient(ctx, tenantName)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant client: %w", err)
	}

	// Add new nodes
	for _, targetNode := range nodesToAdd {
		vmID, err := h.createVM(ctx, tenantName, targetNode)
		if err != nil {
			return nil, fmt.Errorf("failed to create node %s: %w", targetNode.ID, err)
		}
		resp.Resources.Nodes[targetNode.ID] = vmID
		if startStop && targetNode.State == NodeStateOn {
			nodesToStart = append(nodesToStart, targetNode)
		}
	}

	// Remove old nodes
	for _, currentNode := range nodesToRemove {
		if vmID, ok := resp.Resources.Nodes[currentNode.ID]; ok {
			// Delete the VM
			if err := h.deleteVM(vmID); err != nil {
				return nil, fmt.Errorf("failed to delete node %s: %w", currentNode.ID, err)
			}
			// Delete the node from Kubernetes
			if err := tenantClient.DeleteWorkerNode(ctx, currentNode.ID); err != nil {
				return nil, fmt.Errorf("failed to delete worker node %s: %w", currentNode.ID, err)
			}
			// Remove from resources
			delete(resp.Resources.Nodes, currentNode.ID)
		}
	}

	// Start or stop existing nodes
	for _, currentNode := range nodesToStart {
		if vmID, ok := resp.Resources.Nodes[currentNode.ID]; ok {
			// Start the VM
			err := h.startVM(vmID)
			if err != nil {
				return nil, fmt.Errorf("failed to start node %s: %w", currentNode.ID, err)
			}
		}
	}
	for _, currentNode := range nodesToStop {
		if vmID, ok := resp.Resources.Nodes[currentNode.ID]; ok {
			// Stop the VM
			err := h.stopVM(vmID)
			if err != nil {
				return nil, fmt.Errorf("failed to stop node %s: %w", currentNode.ID, err)
			}
		}
	}
	return resp, nil
}

// handleServiceStart starts the cluster service
func (h *JobHandler) handleServiceStart(job *Job) (*JobResponse, error) {
	err := iterateCurrNodes(job, func(node Node, vmID int) error {
		if node.State == NodeStateOn {
			err := h.startVM(vmID)
			if err != nil {
				return fmt.Errorf("failed to start node %s: %w", node.ID, err)
			}
		}
		return nil
	})
	return &JobResponse{
		Resources:  job.Service.Resources,
		ExternalID: job.Service.ExternalID,
	}, err
}

// handleServiceStop stops the cluster service
func (h *JobHandler) handleServiceStop(job *Job) (*JobResponse, error) {
	err := iterateCurrNodes(job, func(node Node, vmID int) error {
		if node.State == NodeStateOn {
			err := h.stopVM(vmID)
			if err != nil {
				return fmt.Errorf("failed to stop node %s: %w", node.ID, err)
			}
		}
		return nil
	})
	return &JobResponse{
		Resources:  job.Service.Resources,
		ExternalID: job.Service.ExternalID,
	}, err
}

// handleServiceDelete deletes the cluster service
func (h *JobHandler) handleServiceDelete(job *Job) (*JobResponse, error) {
	tenantName := job.Service.Name
	ctx := context.Background()

	tenantCli, err := h.kamajiCli.GetTenantClient(ctx, tenantName)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant client: %w", err)
	}

	iterateCurrNodes(job, func(node Node, vmID int) error {
		// Stop and delete the VM
		if err := h.deleteVM(vmID); err != nil {
			return err
		}
		// Delete the node from the tenant control plane
		if err := tenantCli.DeleteWorkerNode(ctx, node.ID); err != nil {
			return err
		}
		return nil
	})

	// Delete tenant control plane
	if err := h.kamajiCli.DeleteTenantControlPlane(ctx, tenantName); err != nil {
		return nil, fmt.Errorf("failed to delete tenant control plane: %w", err)
	}

	return &JobResponse{}, nil
}

// Helper methods

// iterateCurrNodes iterates over the current nodes in the job and applies the provided function
func iterateCurrNodes(job *Job, callback func(node Node, vmID int) error) error {
	var nodes []Node
	if job.Service.CurrentProperties != nil && job.Service.CurrentProperties.Nodes != nil {
		nodes = job.Service.CurrentProperties.Nodes
	}
	vmIDs := make(map[string]int)
	if job.Service.Resources != nil && job.Service.Resources.Nodes != nil {
		vmIDs = job.Service.Resources.Nodes
	}

	for _, node := range nodes {
		if vmID, ok := vmIDs[node.ID]; ok {
			err := callback(node, vmID)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// createVM creates a new node for a service
func (h *JobHandler) createVM(_ context.Context, serviceName string, node Node) (int, error) {
	vmName := fmt.Sprintf("%s-node-%s", serviceName, node.ID)
	vmID := h.generateVMID(serviceName, node.ID)

	// Get node configuration based on size
	cores, memory := node.Size.Attrs()

	// Create VM by cloning from template
	t, err := h.proxmoxCli.CloneVM(h.templateID, vmID, vmName) // Assume 100 is template ID
	if err != nil {
		return 0, fmt.Errorf("failed to clone VM: %w", err)
	}

	_, err = h.proxmoxCli.WaitForTask(t.TaskID, 1*time.Minute)
	if err != nil {
		return 0, fmt.Errorf("failed to clone VM: %w", err)
	}

	// Configure VM
	t, err = h.proxmoxCli.ConfigureVM(vmID, cores, memory, "")
	if err != nil {
		return 0, fmt.Errorf("failed to configure VM: %w", err)
	}

	_, err = h.proxmoxCli.WaitForTask(t.TaskID, 1*time.Minute)
	if err != nil {
		return 0, fmt.Errorf("failed to configure VM: %w", err)
	}

	return vmID, nil
}

// startVM starts a node
func (h *JobHandler) startVM(vmID int) error {
	i, err := h.proxmoxCli.GetVMInfo(vmID)
	if err != nil {
		return fmt.Errorf("failed to get VM info: %w", err)
	}
	if i.State == VMStateRunning {
		return nil
	} else if i.State != VMStateStopped {
		return fmt.Errorf("VM is not stopped")
	}

	t, err := h.proxmoxCli.StartVM(vmID)
	if err != nil {
		return fmt.Errorf("failed to start VM: %w", err)
	}

	_, err = h.proxmoxCli.WaitForTask(t.TaskID, 1*time.Minute)
	if err != nil {
		return fmt.Errorf("failed to start VM: %w", err)
	}

	return nil
}

// stopVM stops a node
func (h *JobHandler) stopVM(vmID int) error {
	i, err := h.proxmoxCli.GetVMInfo(vmID)
	if err != nil {
		return fmt.Errorf("failed to get VM info: %w", err)
	}
	if i.State == VMStateStopped {
		return nil
	} else if i.State != VMStateRunning {
		return fmt.Errorf("VM is not running")
	}

	t, err := h.proxmoxCli.StopVM(vmID)
	if err != nil {
		return fmt.Errorf("failed to stop VM: %w", err)
	}

	_, err = h.proxmoxCli.WaitForTask(t.TaskID, 1*time.Minute)
	if err != nil {
		return fmt.Errorf("failed to stop VM: %w", err)
	}

	return nil
}

// deleteVM deletes a node
func (h *JobHandler) deleteVM(vmID int) error {
	// First try to stop the VM if it's running
	t, err := h.proxmoxCli.StopVM(vmID) // Ignore errors - might already be stopped
	if err == nil {
		_, err = h.proxmoxCli.WaitForTask(t.TaskID, 1*time.Minute)
	}

	// Then delete it
	t, err = h.proxmoxCli.DeleteVM(vmID)
	if err != nil {
		return fmt.Errorf("failed to delete VM: %w", err)
	}

	_, err = h.proxmoxCli.WaitForTask(t.TaskID, 1*time.Minute)
	if err != nil {
		return fmt.Errorf("failed to delete VM: %w", err)
	}

	return nil
}

// generateVMID generates a VM ID based on service name and node ID
// This is a simple implementation - in a real system this would likely use a database
func (h *JobHandler) generateVMID(serviceName string, nodeID string) int {
	// Simple hashing to generate a VM ID in range 1000-9999
	hash := 0
	for _, c := range serviceName + nodeID {
		hash = (hash*31 + int(c)) % 9000
	}
	return hash + 1000
}
