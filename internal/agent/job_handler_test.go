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
	proxmoxCli.AddVM(100, "template-vm", "stopped", 2, 2048)

	kamajiCli := NewMockKamajiClient()
	sshCli := NewMockSSHClient()
	jobHandler := NewJobHandler(fulcrumCli, proxmoxCli, kamajiCli, sshCli)

	t.Run("Full lifecycle", func(t *testing.T) {
		serviceID := "test-service-1"
		serviceName := "test-cluster"

		// Job 1: Create a cluster service with 1 node (id: node1, size: s1, state: on)

		// Create service with one node
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
		vmID, exists := service.Resources.Nodes["node1"]
		require.True(t, exists)

		vm, exists := proxmoxCli.GetVM(vmID)
		require.True(t, exists)
		require.Equal(t, fmt.Sprintf("%s-node-%s", serviceName, "node1"), vm.Name)
		require.Equal(t, "stopped", vm.Status)

		expectedCores, expectedMemory := NodeSizeS1.Attrs()
		require.Equal(t, expectedCores, vm.Cores)
		require.Equal(t, expectedMemory, vm.Memory)

		// Job 2: Start the cluster service
		// Job 3: Update the cluster service adding a node (id: node2, size: s2, state: on)
		// Job 4: Update the cluster service making node2 off
		// Job 5: Update the cluster service making node2 on
		// Job 6: Update the cluster service making node2 off again
		// Job 7: Stop the cluster service
		// Job 8: Update the cluster service making node2 on (when cluster is stopped)
		// Job 9: Start the cluster service again
		// Job 10: Stop the cluster service again
		// Job 11: Delete the cluster service
	})

}
