package agent

import (
	"testing"
)

func TestJobHandler(t *testing.T) {
	// Create stub clients and initialize the JobHandler
	fulcrumCli := NewMockFulcrumClient()
	proxmoxCli := NewMockProxmoxClient()
	kamajiCli := NewMockKamajiClient()
	sshCli := NewMockSSHClient()
	jobHandler := NewJobHandler(fulcrumCli, proxmoxCli, kamajiCli, sshCli)

	t.Run("Full lifecycle", func(t *testing.T) {
		// Job to create a cluster service with 1 node (id: node1, size: s1, state: on)
		// Job to start the cluster service
		// Job to update the cluster service adding a node (id: node2, size: s2, state: on)
		// Job to update the cluster service making node2 off
		// Job to update the cluster service making node2 on
		// Job to update the cluster service making node2 off
		// Job to stop the cluster service
		// Job to update the cluster service making node2 on
		// Job to start the cluster service
		// Job to stop the cluster service
		// Job to delete the cluster service
	})

}
