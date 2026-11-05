// Verifies paginated metrics collection against Core's current service model.
// Custom lifecycle states and stopped VMs must not be mistaken for Started.
package agent

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type metricsCore struct {
	FulcrumClient
	pages     []*ServicesResponse
	requested []int
	metrics   []MetricEntry
	reportErr error
}

func (c *metricsCore) GetServices(page int) (*ServicesResponse, error) {
	c.requested = append(c.requested, page)
	if page < 1 || page > len(c.pages) {
		return nil, errors.New("unexpected page")
	}
	return c.pages[page-1], nil
}

func (c *metricsCore) ReportMetric(metric *MetricEntry) error {
	c.metrics = append(c.metrics, *metric)
	return c.reportErr
}

func TestMetricsUseLifecycleAndAllPages(t *testing.T) {
	instanceID := "cluster-1"
	service := &Service{
		ID: "service-1", Status: "Active", AgentInstanceID: &instanceID,
		AgentInstanceData: &AgentInstanceData{Nodes: map[string]int{"on": 1001, "off": 1002}},
		ServiceType: &ServiceType{LifecycleSchema: &LifecycleSchema{
			RunningStates: []ServiceStatus{"Active"},
		}},
	}
	notRunning := *service
	notRunning.Status = ServiceStarted
	core := &metricsCore{pages: []*ServicesResponse{
		{Items: []*Service{&notRunning}, HasNext: true},
		{Items: []*Service{service}, HasNext: false},
	}}
	proxmox := NewMockProxmoxClient("test-node")
	proxmox.AddVM(1001, "on", VMStatusRunning, 2, 2048)
	proxmox.AddVM(1002, "off", VMStatusStopped, 2, 2048)
	require.NoError(t, NewMetricsReporter(core, proxmox).Report())
	require.Equal(t, []int{1, 2}, core.requested)
	require.Len(t, core.metrics, 2)
	for _, metric := range core.metrics {
		require.Equal(t, instanceID, metric.AgentInstanceID)
		require.Equal(t, "on", metric.ResourceID)
	}
	require.Equal(t, MetricTypeVMCPUUsage, core.metrics[0].TypeName)
	require.Equal(t, MetricTypeVMMemoryUsage, core.metrics[1].TypeName)
}

func TestMetricsEmptyPage(t *testing.T) {
	core := &metricsCore{pages: []*ServicesResponse{{Items: []*Service{}}}}
	require.NoError(t, NewMetricsReporter(core, nil).Report())
	require.Empty(t, core.metrics)
	require.Equal(t, []int{1}, core.requested)
}

func TestMetricsMissingLifecycle(t *testing.T) {
	core := &metricsCore{pages: []*ServicesResponse{{Items: []*Service{{ID: "service-1"}}}}}
	require.ErrorContains(t, NewMetricsReporter(core, nil).Report(), "no lifecycle schema")
}

func TestMetricsPropagateReportFailure(t *testing.T) {
	instanceID := "cluster-1"
	core := &metricsCore{reportErr: errors.New("metrics rejected"), pages: []*ServicesResponse{
		{Items: []*Service{{
			ID: "service-1", Status: "Active", AgentInstanceID: &instanceID,
			AgentInstanceData: &AgentInstanceData{Nodes: map[string]int{"node1": 1001}},
			ServiceType:       &ServiceType{LifecycleSchema: &LifecycleSchema{RunningStates: []ServiceStatus{"Active"}}},
		}}},
	}}
	proxmox := NewMockProxmoxClient("test-node")
	proxmox.AddVM(1001, "node1", VMStatusRunning, 2, 2048)
	require.ErrorContains(t, NewMetricsReporter(core, proxmox).Report(), "metrics rejected")
}
