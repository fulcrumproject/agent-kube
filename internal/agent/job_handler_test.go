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
		// 1. Job to create a cluster service with 1 node (id: node1, size: s1, state: on)
		// 2. Job to update the cluster service adding a node (id: node2, size: s2, state: on)
		// 3. Job to update the cluster service making node2 off
		// 4. Job to update the cluster service making node2 on
		// 5. Job to update the cluster service making node2 off
		// 6. Job to stop the cluster service
		// 7. Job to update the cluster service making node2 on
		// 8. Job to start the cluster service
		// 9. Job to stop the cluster service
		// 10. Job to delete the cluster service
	})

}
