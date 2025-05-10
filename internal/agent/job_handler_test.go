package agent

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJobHandler(t *testing.T) {
	// Create stub clients and initialize the JobHandler
	fulcrumCli := NewMockFulcrumClient()
	proxmoxCli := NewMockProxmoxClient("test-node")

	// Add template VM with ID 100 that will be used for cloning
	proxmoxCli.AddVM(100, "template-vm", VMStateStopped, 2, 2048)

	kamajiCli := NewMockKamajiClient()
	sshCli := NewMockSSHClient()
	jobHandler := NewJobHandler(fulcrumCli, proxmoxCli, kamajiCli, sshCli)

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
		serviceID := "test-service-1"
		serviceName := "test-cluster"

		// Job 1: Create a cluster service with 1 node (id: node1, size: s1, state: on)
		node := Node{ID: "node1", Size: NodeSizeS1, State: NodeStateOn}
		targetProps := &Properties{Nodes: []Node{node}}

		// Create service and process job
		err := fulcrumCli.CreateService(serviceID, serviceName, nil, targetProps)
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
		require.Equal(t, ServiceCreated, service.CurrentState)

		// Verify node properties
		require.NotNil(t, service.CurrentProperties)
		require.Len(t, service.CurrentProperties.Nodes, 1)
		serviceNode := service.CurrentProperties.Nodes[0]
		require.Equal(t, "node1", serviceNode.ID)
		require.Equal(t, NodeSizeS1, serviceNode.Size)
		require.Equal(t, NodeStateOn, serviceNode.State)

		// Verify VM was created properly
		require.NotNil(t, service.Resources)
		vmID1, exists := service.Resources.Nodes["node1"]
		require.True(t, exists)

		vm, exists := proxmoxCli.GetVM(vmID1)
		require.True(t, exists)
		require.Equal(t, fmt.Sprintf("%s-node-%s", serviceName, "node1"), vm.Name)
		require.Equal(t, VMStateStopped, vm.State)

		expectedCores, expectedMemory := NodeSizeS1.Attrs()
		require.Equal(t, expectedCores, vm.Cores)
		require.Equal(t, expectedMemory, vm.Memory)

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
		require.Equal(t, ServiceStarted, service.CurrentState)

		// Verify VM is now running
		vm, exists = proxmoxCli.GetVM(vmID1)
		require.True(t, exists)
		require.Equal(t, VMStateRunning, vm.State)

		// Job 3: Update the cluster service adding a node (id: node2, size: s2, state: on)
		node1 := service.CurrentProperties.Nodes[0]                      // Keep existing node1
		node2 := Node{ID: "node2", Size: NodeSizeS2, State: NodeStateOn} // Add new node2
		updatedProps := &Properties{Nodes: []Node{node1, node2}}

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
		require.Equal(t, ServiceStarted, service.CurrentState)

		// Verify service now has two nodes
		require.Len(t, service.CurrentProperties.Nodes, 2)

		// Verify node2 was created
		vmID2, exists := service.Resources.Nodes["node2"]
		require.True(t, exists)

		vm2, exists := proxmoxCli.GetVM(vmID2)
		require.True(t, exists)
		require.Equal(t, fmt.Sprintf("%s-node-%s", serviceName, "node2"), vm2.Name)
		require.Equal(t, VMStateRunning, vm2.State)

		// Verify node2 has correct configuration
		expectedCores2, expectedMemory2 := NodeSizeS2.Attrs()
		require.Equal(t, expectedCores2, vm2.Cores)
		require.Equal(t, expectedMemory2, vm2.Memory)

		// Job 4: Update the cluster service making node2 off
		nodeList := service.CurrentProperties.Nodes
		updatedNodes := make([]Node, len(nodeList))
		copy(updatedNodes, nodeList)

		// Update node2 state to off
		for i := range updatedNodes {
			if updatedNodes[i].ID == "node2" {
				updatedNodes[i].State = NodeStateOff
				break
			}
		}
		updatedProps = &Properties{Nodes: updatedNodes}

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
		require.Equal(t, ServiceStarted, service.CurrentState)

		// Verify node2 is now off
		vm2, exists = proxmoxCli.GetVM(vmID2)
		require.True(t, exists)
		require.Equal(t, VMStateStopped, vm2.State)

		// Verify node1 is still on
		vm, exists = proxmoxCli.GetVM(vmID1)
		require.True(t, exists)
		require.Equal(t, VMStateRunning, vm.State)

		// Job 5: Update the cluster service making node2 on
		nodeList = service.CurrentProperties.Nodes
		updatedNodes = make([]Node, len(nodeList))
		copy(updatedNodes, nodeList)

		// Update node2 state to on
		for i := range updatedNodes {
			if updatedNodes[i].ID == "node2" {
				updatedNodes[i].State = NodeStateOn
				break
			}
		}
		updatedProps = &Properties{Nodes: updatedNodes}

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
		require.Equal(t, ServiceStarted, service.CurrentState)

		// Verify node2 is now on again
		vm2, exists = proxmoxCli.GetVM(vmID2)
		require.True(t, exists)
		require.Equal(t, VMStateRunning, vm2.State)

		// Job 6: Update the cluster service making node2 off again
		nodeList = service.CurrentProperties.Nodes
		updatedNodes = make([]Node, len(nodeList))
		copy(updatedNodes, nodeList)

		// Update node2 state to off
		for i := range updatedNodes {
			if updatedNodes[i].ID == "node2" {
				updatedNodes[i].State = NodeStateOff
				break
			}
		}
		updatedProps = &Properties{Nodes: updatedNodes}

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
		require.Equal(t, ServiceStarted, service.CurrentState)

		// Verify node2 is off again
		vm2, exists = proxmoxCli.GetVM(vmID2)
		require.True(t, exists)
		require.Equal(t, VMStateStopped, vm2.State)

		// Verify node1 is still on
		vm, exists = proxmoxCli.GetVM(vmID1)
		require.True(t, exists)
		require.Equal(t, VMStateRunning, vm.State)

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
		require.Equal(t, ServiceStopped, service.CurrentState)

		// Verify both nodes are now stopped
		vm, exists = proxmoxCli.GetVM(vmID1)
		require.True(t, exists)
		require.Equal(t, VMStateStopped, vm.State)

		vm2, exists = proxmoxCli.GetVM(vmID2)
		require.True(t, exists)
		require.Equal(t, VMStateStopped, vm2.State)

		// Job 8: Update the cluster service removing node2
		nodeList = service.CurrentProperties.Nodes
		updatedNodes = []Node{}

		// Keep only node1, removing node2
		for _, n := range nodeList {
			if n.ID == "node1" {
				updatedNodes = append(updatedNodes, n)
				break
			}
		}
		updatedProps = &Properties{Nodes: updatedNodes}

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
		require.Equal(t, ServiceStopped, service.CurrentState)

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
		require.Equal(t, ServiceStarted, service.CurrentState)

		// Verify node1 is now running again
		vm, exists = proxmoxCli.GetVM(vmID1)
		require.True(t, exists)
		require.Equal(t, VMStateRunning, vm.State)

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
		require.Equal(t, ServiceStopped, service.CurrentState)

		// Verify node1 is now stopped
		vm, exists = proxmoxCli.GetVM(vmID1)
		require.True(t, exists)
		require.Equal(t, VMStateStopped, vm.State)

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
		require.Equal(t, ServiceDeleted, service.CurrentState)
	})

}
