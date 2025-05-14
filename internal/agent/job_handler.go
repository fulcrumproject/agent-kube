package agent

import (
	"context"
	"fmt"
	"log"
	"time"

	"fulcrumproject.org/kube-agent/internal/cloudinit"
	"k8s.io/apimachinery/pkg/util/wait"
)

// JobHandler processes jobs from the Fulcrum Core job queue
type JobHandler struct {
	templateID int
	ciPath     string
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
	ciPath string,
	kamajiCli KamajiClient,
	sshCli SSHClient,
) *JobHandler {
	return &JobHandler{
		templateID: templateID,
		ciPath:     ciPath,
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

			if err := tenantClient.DeleteWorkerNode(ctx, vmName(job.Service.Name, currentNode.ID)); err != nil {
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
			err := h.startVMAndWaitJoin(vmID, job.Service.Name, currentNode.ID)
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
			err := h.startVMAndWaitJoin(vmID, job.Service.Name, node.ID)
			if err != nil {
				return fmt.Errorf("failed to start node %s: %w", node.ID, err)
			}
			return nil

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
func (h *JobHandler) createVM(ctx context.Context, serviceName string, node Node) (int, error) {
	vmName := vmName(serviceName, node.ID)
	vmID := h.generateVMID(serviceName, node.ID)

	// Get node configuration based on size
	cores, memory := node.Size.Attrs()

	// Create VM by cloning from template
	t, err := h.proxmoxCli.CloneVM(h.templateID, vmID, vmName) // Assume 100 is template ID
	if err != nil {
		return 0, fmt.Errorf("failed to clone VM: %w", err)
	}

	_, err = h.proxmoxCli.WaitForTask(t.TaskID, 10*time.Minute)
	if err != nil {
		return 0, fmt.Errorf("failed to clone VM: %w", err)
	}

	// Generate join token for the node to join the cluster
	tenantClient, err := h.kamajiCli.GetTenantClient(ctx, serviceName)
	if err != nil {
		return 0, fmt.Errorf("failed to get tenant client: %w", err)
	}

	joinToken, err := tenantClient.CreateJoinToken(ctx, serviceName, 24) // 24 hours validity
	if err != nil {
		return 0, fmt.Errorf("failed to create join token: %w", err)
	}

	// Get CA cert hash for the cluster
	caCertHash, err := h.kamajiCli.GetTenantCAHash(ctx, serviceName)
	if err != nil {
		return 0, fmt.Errorf("failed to get CA cert hash: %w", err)
	}

	// Get kubeconfig for the cluster
	kubeConfig, err := h.kamajiCli.GetTenantKubeConfig(ctx, serviceName)
	if err != nil {
		return 0, fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	// Generate cloud-init configuration
	cloudInitParams := cloudinit.CloudInitParams{
		Hostname:       vmName,
		FQDN:           vmName,
		Username:       "ubuntu",
		Password:       "ubuntu",
		SSHKeys:        []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIBeZfPGgiVw7zMpOhs7RQMCL3+jxfA8U1iiGSiYDSXWy kube@testudo"},
		ExpirePassword: false,
		PackageUpgrade: true,
		JoinURL:        kubeConfig.Endpoint,
		JoinToken:      joinToken.FullToken,
		CACertHash:     caCertHash,
		KubeVersion:    "v1.30.2",
	}

	// Generate cloud-init config
	cloudInitContent, err := cloudinit.GenerateCloudInit(cloudinit.CloudInitTempl, cloudInitParams)
	if err != nil {
		return 0, fmt.Errorf("failed to generate cloud-init configuration: %w", err)
	}

	cloudInitFileName := fmt.Sprintf("kube-agent-ci-%s.yml", vmName)
	cloudInitFilePath := fmt.Sprintf("%s/%s", h.ciPath, cloudInitFileName)

	// Upload cloud-init config to Proxmox host via SSH - use appropriate path/filename
	err = h.sshCli.Copy(cloudInitContent, cloudInitFilePath)
	if err != nil {
		return 0, fmt.Errorf("failed to copy cloud-init configuration: %w", err)
	}

	// Configure VM with cloud-init config
	cloudInitConfig := fmt.Sprintf("user=local:snippets/%s", cloudInitFileName)
	t, err = h.proxmoxCli.ConfigureVM(vmID, cores, memory, cloudInitConfig)
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
func (h *JobHandler) startVMAndWaitJoin(vmID int, serviceName, nodeName string) error {
	err := h.startVM(vmID)
	if err != nil {
		return err
	}
	vmName := vmName(serviceName, nodeName)
	err = h.waitJoin(serviceName, vmName)
	if err != nil {
		return err
	}
	return nil
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

func (h *JobHandler) waitJoin(serviceName, nodeName string) error {
	ctx := context.Background()

	tenantClient, err := h.kamajiCli.GetTenantClient(ctx, serviceName)
	if err != nil {
		return fmt.Errorf("failed to get tenant client: %w", err)
	}

	// Wait for node to join
	err = wait.PollUntilContextTimeout(ctx, 10*time.Second, 10*time.Minute, true, func(ctx context.Context) (bool, error) {
		nodeStatus, err := tenantClient.GetNodeStatus(ctx, nodeName)
		if err != nil {
			return false, nil // Node not found, continue polling
		}
		if !nodeStatus.Ready {
			return false, nil // If the node is found but not ready, continue waiting
		}
		// Node is registered and ready
		return true, nil
	})

	if err != nil {
		return fmt.Errorf("node failed to join: %w", err)
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

func vmName(serviceName, nodeID string) string {
	return fmt.Sprintf("%s-node-%s", serviceName, nodeID)
}
