// Exercises current Core job semantics and their failure paths.
// Infrastructure clients are in-memory mocks; no live resources are touched.
package agent

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func newTestHandler(t *testing.T) (*JobHandler, *MockFulcrumClient, *MockProxmoxClient, *MockKamajiClient) {
	t.Helper()
	core := NewMockFulcrumClient()
	proxmox := NewMockProxmoxClient("test-node")
	proxmox.AddVM(100, "template", VMStatusStopped, 2, 2048)
	kamaji := NewMockKamajiClient()
	return NewJobHandler(core, proxmox, 100, "test", kamaji, NewMockSSHClient()), core, proxmox, kamaji
}

func createTestCluster(t *testing.T, handler *JobHandler, core *MockFulcrumClient) Service {
	t.Helper()
	require.NoError(t, core.CreateService("service-1", "cluster", nil, &Properties{Nodes: []Node{
		{ID: "node1", Size: NodeSizeS1, Status: NodeStatusOn},
	}}))
	require.NoError(t, handler.PollAndProcessJobs())
	require.Len(t, core.PullCompletedJobs(), 1)
	service, err := core.GetService("service-1")
	require.NoError(t, err)
	require.NotNil(t, service.AgentInstanceID)
	require.Equal(t, "cluster-cluster", *service.AgentInstanceID)
	return service
}

func TestUpdateUsesParamsAndLifecycleRunningStates(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  ServiceStatus
		running []ServiceStatus
		wantVM  VMStatus
	}{
		{"custom running state", "Active", []ServiceStatus{"Active"}, VMStatusRunning},
		{"Started is not necessarily running", "Started", []ServiceStatus{"Active"}, VMStatusStopped},
		{"empty running states", "Idle", nil, VMStatusStopped},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler, core, proxmox, _ := newTestHandler(t)
			service := createTestCluster(t, handler, core)
			service.Status = tc.status
			service.ServiceType.LifecycleSchema.RunningStates = tc.running
			core.service[service.ID] = service
			params := &Properties{Nodes: append(append([]Node{}, service.Properties.Nodes...), Node{
				ID: "node2", Size: NodeSizeS2, Status: NodeStatusOn,
			})}
			job := &Job{ID: "update-1", ServiceID: service.ID, Action: JobActionServiceUpdate,
				Status: JobStatusPending, Service: service, Params: params}
			job.Service.ServiceType = nil // As returned by Core's pending-job query.
			require.NoError(t, core.EnqueueJob(job))
			require.NoError(t, handler.PollAndProcessJobs())
			require.Empty(t, core.PullFailedJobs())
			require.Len(t, core.PullCompletedJobs(), 1)
			updated, err := core.GetService(service.ID)
			require.NoError(t, err)
			require.Equal(t, tc.status, updated.Status)
			require.Len(t, updated.Properties.Nodes, 2)
			require.Len(t, service.Properties.Nodes, 1, "applied snapshot must not be mutated")
			require.Len(t, service.AgentInstanceData.Nodes, 1)
			vm, ok := proxmox.GetVM(updated.AgentInstanceData.Nodes["node2"])
			require.True(t, ok)
			require.Equal(t, tc.wantVM, vm.Status)
		})
	}
}

func TestUpdateEmptyClusterInitializesNodeMap(t *testing.T) {
	handler, core, _, _ := newTestHandler(t)
	require.NoError(t, core.CreateService("empty", "empty-cluster", nil, &Properties{Nodes: []Node{}}))
	require.NoError(t, handler.PollAndProcessJobs())
	require.Len(t, core.PullCompletedJobs(), 1)
	service, err := core.GetService("empty")
	require.NoError(t, err)
	// Empty maps are omitted from JSON and therefore decode as nil.
	service.AgentInstanceData.Nodes = nil
	params := &Properties{Nodes: []Node{{ID: "node1", Size: NodeSizeS1, Status: NodeStatusOn}}}
	require.NoError(t, core.EnqueueJob(&Job{ID: "add-node", ServiceID: service.ID,
		Action: JobActionServiceUpdate, Status: JobStatusPending, Service: service, Params: params}))
	require.NoError(t, handler.PollAndProcessJobs())
	require.Empty(t, core.PullFailedJobs())
	require.Len(t, core.PullCompletedJobs(), 1)
	updated, err := core.GetService(service.ID)
	require.NoError(t, err)
	require.Contains(t, updated.AgentInstanceData.Nodes, "node1")
}

func TestInvalidJobsAreFailed(t *testing.T) {
	for _, tc := range []struct {
		name   string
		action JobAction
		change func(*Job, *MockFulcrumClient)
		want   string
	}{
		{"missing service", JobActionServiceStart, func(j *Job, _ *MockFulcrumClient) { j.Service = Service{} }, "has no service"},
		{"missing create params", JobActionServiceCreate, func(j *Job, _ *MockFulcrumClient) { j.Params = nil }, "has no params"},
		{"missing update params", JobActionServiceUpdate, func(j *Job, _ *MockFulcrumClient) { j.Params = nil }, "has no params"},
		{"missing properties", JobActionServiceStart, func(j *Job, _ *MockFulcrumClient) { j.Service.Properties = nil }, "missing properties"},
		{"missing instance data", JobActionServiceUpdate, func(j *Job, _ *MockFulcrumClient) { j.Service.AgentInstanceData = nil }, "agent instance data"},
		{"missing VM mapping", JobActionServiceStop, func(j *Job, _ *MockFulcrumClient) { j.Service.AgentInstanceData = &AgentInstanceData{} }, "no VM for node"},
		{"missing lifecycle", JobActionServiceUpdate, func(j *Job, c *MockFulcrumClient) {
			s := c.service[j.ServiceID]
			s.ServiceType = nil
			c.service[s.ID] = s
		}, "no lifecycle schema"},
		{"service lookup fails", JobActionServiceUpdate, func(j *Job, c *MockFulcrumClient) { delete(c.service, j.ServiceID) }, "failed to get service lifecycle"},
		{"unsupported size change", JobActionServiceUpdate, func(j *Job, _ *MockFulcrumClient) {
			j.Params = &Properties{Nodes: []Node{{ID: "node1", Size: NodeSizeS4, Status: NodeStatusOn}}}
		}, "changing VM size is not supported"},
		{"unknown action", "rebuild", func(*Job, *MockFulcrumClient) {}, "unknown job type"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler, core, _, _ := newTestHandler(t)
			service := createTestCluster(t, handler, core)
			job := &Job{ID: "invalid-1", ServiceID: service.ID, Action: tc.action,
				Status: JobStatusPending, Service: service, Params: service.Properties}
			tc.change(job, core)
			require.NoError(t, core.EnqueueJob(job))
			require.NoError(t, handler.PollAndProcessJobs())
			require.Empty(t, core.PullCompletedJobs())
			failed := core.PullFailedJobs()
			require.Len(t, failed, 1)
			require.Contains(t, failed[0].ErrorMessage, tc.want)
		})
	}
}

type deleteFailingProxmox struct{ ProxmoxClient }

func (deleteFailingProxmox) DeleteVM(int) (*TaskResponse, error) {
	return nil, errors.New("deletion denied")
}

func TestDeleteDoesNotCompleteAfterNodeFailure(t *testing.T) {
	handler, core, proxmox, kamaji := newTestHandler(t)
	service := createTestCluster(t, handler, core)
	handler.proxmoxCli = deleteFailingProxmox{proxmox}
	require.NoError(t, core.EnqueueJob(&Job{ID: "delete-1", ServiceID: service.ID,
		Action: JobActionServiceDelete, Status: JobStatusPending, Service: service}))
	require.NoError(t, handler.PollAndProcessJobs())
	require.Empty(t, core.PullCompletedJobs())
	failed := core.PullFailedJobs()
	require.Len(t, failed, 1)
	require.Contains(t, failed[0].ErrorMessage, "deletion denied")
	_, exists := kamaji.tenantControlPlanes[service.Name]
	require.True(t, exists, "control plane must not be deleted after a node failure")
}
