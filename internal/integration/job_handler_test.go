package integration

import (
	"testing"
	"time"

	"fulcrumproject.org/kube-agent/internal/agent"
	"fulcrumproject.org/kube-agent/internal/config"
	"fulcrumproject.org/kube-agent/internal/httpcli"
	"fulcrumproject.org/kube-agent/internal/kamaji"
	"fulcrumproject.org/kube-agent/internal/proxmox"
	"fulcrumproject.org/kube-agent/internal/ssh"
	"fulcrumproject.org/kube-agent/internal/testhelp"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestJobHandler(t *testing.T) {
	// Skip if not an integration test
	testhelp.SkipIfNotIntegrationTest(t)

	// Load configuration from .env file
	cfg, err := config.Builder().WithEnv().Build()
	require.NoError(t, err, "Failed to load configuration")
	require.NotNil(t, cfg, "Configuration should not be nil")

	// Create stub clients and initialize the JobHandler
	fulcrumCli := agent.NewMockFulcrumClient()

	// Validate Kamaji configuration
	require.NotEmpty(t, cfg.KubeAPIURL, "KubeAPIURL should not be empty")
	require.NotEmpty(t, cfg.KubeAPIToken, "KubeAPIToken should not be empty")

	// Validate Proxmox configuration
	require.NotEmpty(t, cfg.ProxmoxAPIURL, "ProxmoxAPIURL should not be empty")
	require.NotEmpty(t, cfg.ProxmoxAPIToken, "ProxmoxAPIToken should not be empty")
	require.NotEmpty(t, cfg.ProxmoxHost, "ProxmoxHost should not be empty")
	require.NotEmpty(t, cfg.ProxmoxStorage, "ProxmoxStorage should not be empty")
	require.NotEmpty(t, cfg.ProxmoxTemplate, "ProxmoxTemplate should not be empty")
	require.NotEmpty(t, cfg.ProxmoxCIHost, "ProxmoxCIHost should not be empty")
	require.NotEmpty(t, cfg.ProxmoxCIUser, "ProxmoxCIUser should not be empty")
	require.NotEmpty(t, cfg.ProxmoxCIPath, "ProxmoxCIPath should not be empty")
	require.NotEmpty(t, cfg.ProxmoxCIPKPath, "ProxmoxCIPKPath should not be empty")

	// Create Kamaji client
	kamajiCli, err := kamaji.NewClient(cfg.KubeAPIURL, cfg.KubeAPIToken)
	require.NoError(t, err, "Failed to create Kamaji client")
	require.NotNil(t, kamajiCli, "Kamaji client should not be nil")

	// Create Proxmox client
	httpCli := httpcli.NewHTTPClient(
		cfg.ProxmoxAPIURL,
		cfg.ProxmoxAPIToken,
		httpcli.WithAuthType(httpcli.AuthTypePVE),
		httpcli.WithSkipTLSVerify(true), // Skip TLS verification for test
	)
	require.NotNil(t, httpCli)

	proxmoxCli := proxmox.NewProxmoxClient(cfg.ProxmoxHost, cfg.ProxmoxStorage, httpCli)
	require.NotNil(t, proxmoxCli, "Proxmox client should not be nil")

	// SCP configuration for cloud-init
	scpOpts := ssh.Options{
		Host:           cfg.ProxmoxCIHost,
		Username:       cfg.ProxmoxCIUser,
		PrivateKeyPath: cfg.ProxmoxCIPKPath,
		Timeout:        30 * time.Second,
	}

	sshCli, err := ssh.NewClient(scpOpts)
	require.NoError(t, err, "Failed to create SSH client")
	require.NotNil(t, sshCli, "SSH client should not be nil")

	jobHandler := agent.NewJobHandler(fulcrumCli, proxmoxCli, cfg.ProxmoxTemplate, kamajiCli, sshCli)

	// This test verifies the complete lifecycle of a Kubernetes cluster service by:
	// 1. Creating a cluster with one node (node1)
	// 2. Starting the cluster service
	// 3. Updating the cluster by adding a second node (node2)
	// 4. Turning node2 off while keeping node1 running
	// 5. Turning node2 back on
	// 6. Turning node2 off again
	// 7. Stopping the entire cluster service
	// 8. Removing node2 from the cluster
	// 9. Starting the cluster service again
	// 10. Stopping the cluster service again
	// 11. Deleting the cluster service
	t.Run("Full lifecycle", func(t *testing.T) {
		serviceID := uuid.New().String()
		serviceName := "test-cluster" + "-" + serviceID

		// Job 1: Create a cluster service with 1 node (id: node1, size: s1, state: on)
		node := agent.Node{ID: "node1", Size: agent.NodeSizeS1, State: agent.NodeStateOn}
		targetProps := &agent.Properties{Nodes: []agent.Node{node}}

		// Create service and process job
		err := fulcrumCli.CreateService(serviceID, serviceName, &serviceID, targetProps)
		require.NoError(t, err)
		err = jobHandler.PollAndProcessJobs()
		require.NoError(t, err)

		// Verify job completion
		completedJobs := fulcrumCli.PullCompletedJobs()
		require.Len(t, completedJobs, 1)
		require.Empty(t, fulcrumCli.PullFailedJobs())

		// Verify service state
		service, err := fulcrumCli.GetService(serviceID)
		require.NoError(t, err)
		require.Equal(t, agent.ServiceCreated, service.CurrentState)

		// Verify node properties
		require.NotNil(t, service.CurrentProperties)
		require.Len(t, service.CurrentProperties.Nodes, 1)
		serviceNode := service.CurrentProperties.Nodes[0]
		require.Equal(t, "node1", serviceNode.ID)
		require.Equal(t, agent.NodeSizeS1, serviceNode.Size)
		require.Equal(t, agent.NodeStateOn, serviceNode.State)

		// Verify VM was created properly
		require.NotNil(t, service.Resources)
		vmID1, exists := service.Resources.Nodes["node1"]
		require.True(t, exists)

		vm, err := proxmoxCli.GetVMInfo(vmID1)
		require.NoError(t, err)
		require.NotNil(t, vm)
		require.Equal(t, agent.VMStateStopped, vm.State)

		expectedCores, expectedMemory := agent.NodeSizeS1.Attrs()
		require.Equal(t, expectedCores, vm.CPUCount)
		require.Equal(t, expectedMemory, vm.MaxMemory)

		// Job 2: Start the cluster service
		err = fulcrumCli.StartService(serviceID)
		require.NoError(t, err)
		err = jobHandler.PollAndProcessJobs()
		require.NoError(t, err)

		// Verify job completion
		completedJobs = fulcrumCli.PullCompletedJobs()
		require.Len(t, completedJobs, 1)
		require.Empty(t, fulcrumCli.PullFailedJobs())

		// Verify service state
		service, err = fulcrumCli.GetService(serviceID)
		require.NoError(t, err)
		require.Equal(t, agent.ServiceStarted, service.CurrentState)

		// Verify VM is now running
		vm, err = proxmoxCli.GetVMInfo(vmID1)
		require.NoError(t, err)
		require.NotNil(t, vm)
		require.Equal(t, agent.VMStateRunning, vm.State)

		// Job 3: Update the cluster service adding a node (id: node2, size: s2, state: on)
		node1 := service.CurrentProperties.Nodes[0]                                        // Keep existing node1
		node2 := agent.Node{ID: "node2", Size: agent.NodeSizeS2, State: agent.NodeStateOn} // Add new node2
		updatedProps := &agent.Properties{Nodes: []agent.Node{node1, node2}}

		// Update the service
		err = fulcrumCli.UpdateService(serviceID, updatedProps)
		require.NoError(t, err)
		err = jobHandler.PollAndProcessJobs()
		require.NoError(t, err)

		// Verify job completion
		completedJobs = fulcrumCli.PullCompletedJobs()
		require.Len(t, completedJobs, 1)
		require.Empty(t, fulcrumCli.PullFailedJobs())

		// Verify service state
		service, err = fulcrumCli.GetService(serviceID)
		require.NoError(t, err)
		require.Equal(t, agent.ServiceStarted, service.CurrentState)

		// Verify service now has two nodes
		require.Len(t, service.CurrentProperties.Nodes, 2)

		// Verify node2 was created
		vmID2, exists := service.Resources.Nodes["node2"]
		require.True(t, exists)

		vm2, err := proxmoxCli.GetVMInfo(vmID2)
		require.NoError(t, err)
		require.NotNil(t, vm2)
		require.Equal(t, agent.VMStateRunning, vm2.State)

		// Verify node2 has correct configuration
		expectedCores2, expectedMemory2 := agent.NodeSizeS2.Attrs()
		require.Equal(t, expectedCores2, vm2.CPUCount)
		require.Equal(t, expectedMemory2, vm2.MaxMemory)

		// Job 4: Update the cluster service making node2 off
		nodeList := service.CurrentProperties.Nodes
		updatedNodes := make([]agent.Node, len(nodeList))
		copy(updatedNodes, nodeList)

		// Update node2 state to off
		for i := range updatedNodes {
			if updatedNodes[i].ID == "node2" {
				updatedNodes[i].State = agent.NodeStateOff
				break
			}
		}
		updatedProps = &agent.Properties{Nodes: updatedNodes}

		// Update the service
		err = fulcrumCli.UpdateService(serviceID, updatedProps)
		require.NoError(t, err)
		err = jobHandler.PollAndProcessJobs()
		require.NoError(t, err)

		// Verify job completion
		completedJobs = fulcrumCli.PullCompletedJobs()
		require.Len(t, completedJobs, 1)
		require.Empty(t, fulcrumCli.PullFailedJobs())

		// Verify service state
		service, err = fulcrumCli.GetService(serviceID)
		require.NoError(t, err)
		require.Equal(t, agent.ServiceStarted, service.CurrentState)

		// Verify node2 is now off
		vm2, err = proxmoxCli.GetVMInfo(vmID2)
		require.NoError(t, err)
		require.NotNil(t, vm2)
		require.Equal(t, agent.VMStateStopped, vm2.State)

		// Verify node1 is still on
		vm, err = proxmoxCli.GetVMInfo(vmID1)
		require.NoError(t, err)
		require.NotNil(t, vm)
		require.Equal(t, agent.VMStateRunning, vm.State)

		// Job 5: Update the cluster service making node2 on
		nodeList = service.CurrentProperties.Nodes
		updatedNodes = make([]agent.Node, len(nodeList))
		copy(updatedNodes, nodeList)

		// Update node2 state to on
		for i := range updatedNodes {
			if updatedNodes[i].ID == "node2" {
				updatedNodes[i].State = agent.NodeStateOn
				break
			}
		}
		updatedProps = &agent.Properties{Nodes: updatedNodes}

		// Update the service
		err = fulcrumCli.UpdateService(serviceID, updatedProps)
		require.NoError(t, err)
		err = jobHandler.PollAndProcessJobs()
		require.NoError(t, err)

		// Verify job completion
		completedJobs = fulcrumCli.PullCompletedJobs()
		require.Len(t, completedJobs, 1)
		require.Empty(t, fulcrumCli.PullFailedJobs())

		// Verify service state
		service, err = fulcrumCli.GetService(serviceID)
		require.NoError(t, err)
		require.Equal(t, agent.ServiceStarted, service.CurrentState)

		// Verify node2 is now on again
		vm2, err = proxmoxCli.GetVMInfo(vmID2)
		require.NoError(t, err)
		require.NotNil(t, vm2)
		require.Equal(t, agent.VMStateRunning, vm2.State)

		// Job 6: Update the cluster service making node2 off again
		nodeList = service.CurrentProperties.Nodes
		updatedNodes = make([]agent.Node, len(nodeList))
		copy(updatedNodes, nodeList)

		// Update node2 state to off
		for i := range updatedNodes {
			if updatedNodes[i].ID == "node2" {
				updatedNodes[i].State = agent.NodeStateOff
				break
			}
		}
		updatedProps = &agent.Properties{Nodes: updatedNodes}

		// Update the service
		err = fulcrumCli.UpdateService(serviceID, updatedProps)
		require.NoError(t, err)
		err = jobHandler.PollAndProcessJobs()
		require.NoError(t, err)

		// Verify job completion
		completedJobs = fulcrumCli.PullCompletedJobs()
		require.Len(t, completedJobs, 1)
		require.Empty(t, fulcrumCli.PullFailedJobs())

		// Verify service state
		service, err = fulcrumCli.GetService(serviceID)
		require.NoError(t, err)
		require.Equal(t, agent.ServiceStarted, service.CurrentState)

		// Verify node2 is off again
		vm2, err = proxmoxCli.GetVMInfo(vmID2)
		require.NoError(t, err)
		require.NotNil(t, vm2)
		require.Equal(t, agent.VMStateStopped, vm2.State)

		// Verify node1 is still on
		vm, err = proxmoxCli.GetVMInfo(vmID1)
		require.NoError(t, err)
		require.NotNil(t, vm)
		require.Equal(t, agent.VMStateRunning, vm.State)

		// Job 7: Stop the cluster service
		err = fulcrumCli.StopService(serviceID)
		require.NoError(t, err)
		err = jobHandler.PollAndProcessJobs()
		require.NoError(t, err)

		// Verify job completion
		completedJobs = fulcrumCli.PullCompletedJobs()
		require.Len(t, completedJobs, 1)
		require.Empty(t, fulcrumCli.PullFailedJobs())

		// Verify service state
		service, err = fulcrumCli.GetService(serviceID)
		require.NoError(t, err)
		require.Equal(t, agent.ServiceStopped, service.CurrentState)

		// Verify both nodes are now stopped
		vm, err = proxmoxCli.GetVMInfo(vmID1)
		require.NoError(t, err)
		require.NotNil(t, vm)
		require.Equal(t, agent.VMStateStopped, vm.State)

		vm2, err = proxmoxCli.GetVMInfo(vmID2)
		require.NoError(t, err)
		require.NotNil(t, vm2)
		require.Equal(t, agent.VMStateStopped, vm2.State)

		// Job 8: Update the cluster service removing node2
		nodeList = service.CurrentProperties.Nodes
		updatedNodes = []agent.Node{}

		// Keep only node1, removing node2
		for _, n := range nodeList {
			if n.ID == "node1" {
				updatedNodes = append(updatedNodes, n)
				break
			}
		}
		updatedProps = &agent.Properties{Nodes: updatedNodes}

		// Update the service
		err = fulcrumCli.UpdateService(serviceID, updatedProps)
		require.NoError(t, err)
		err = jobHandler.PollAndProcessJobs()
		require.NoError(t, err)

		// Verify job completion
		completedJobs = fulcrumCli.PullCompletedJobs()
		require.Len(t, completedJobs, 1)
		require.Empty(t, fulcrumCli.PullFailedJobs())

		// Verify service state
		service, err = fulcrumCli.GetService(serviceID)
		require.NoError(t, err)
		require.Equal(t, agent.ServiceStopped, service.CurrentState)

		// Verify node2 is removed from properties
		require.Len(t, service.CurrentProperties.Nodes, 1)
		require.Equal(t, "node1", service.CurrentProperties.Nodes[0].ID)

		// Job 9: Start the cluster service again
		err = fulcrumCli.StartService(serviceID)
		require.NoError(t, err)
		err = jobHandler.PollAndProcessJobs()
		require.NoError(t, err)

		// Verify job completion
		completedJobs = fulcrumCli.PullCompletedJobs()
		require.Len(t, completedJobs, 1)
		require.Empty(t, fulcrumCli.PullFailedJobs())

		// Verify service state
		service, err = fulcrumCli.GetService(serviceID)
		require.NoError(t, err)
		require.Equal(t, agent.ServiceStarted, service.CurrentState)

		// Verify node1 is now running again
		vm, err = proxmoxCli.GetVMInfo(vmID1)
		require.NoError(t, err)
		require.NotNil(t, vm)
		require.Equal(t, agent.VMStateRunning, vm.State)

		// Job 10: Stop the cluster service again
		err = fulcrumCli.StopService(serviceID)
		require.NoError(t, err)
		err = jobHandler.PollAndProcessJobs()
		require.NoError(t, err)

		// Verify job completion
		completedJobs = fulcrumCli.PullCompletedJobs()
		require.Len(t, completedJobs, 1)
		require.Empty(t, fulcrumCli.PullFailedJobs())

		// Verify service state
		service, err = fulcrumCli.GetService(serviceID)
		require.NoError(t, err)
		require.Equal(t, agent.ServiceStopped, service.CurrentState)

		// Verify node1 is now stopped
		vm, err = proxmoxCli.GetVMInfo(vmID1)
		require.NoError(t, err)
		require.NotNil(t, vm)
		require.Equal(t, agent.VMStateStopped, vm.State)

		// Job 11: Delete the cluster service
		err = fulcrumCli.DeleteService(serviceID)
		require.NoError(t, err)
		err = jobHandler.PollAndProcessJobs()
		require.NoError(t, err)

		// Verify job completion
		completedJobs = fulcrumCli.PullCompletedJobs()
		require.Len(t, completedJobs, 1)
		require.Empty(t, fulcrumCli.PullFailedJobs())

		// Verify service is in deleted state
		service, err = fulcrumCli.GetService(serviceID)
		require.NoError(t, err)
		require.Equal(t, agent.ServiceDeleted, service.CurrentState)
	})

}
