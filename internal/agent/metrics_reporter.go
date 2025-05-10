package agent

import (
	"fmt"
	"log/slog"
)

type MetricsReporter struct {
	fulcrumCli FulcrumClient
	proxmoxCli ProxmoxClient
}

func NewMetricsReporter(fulcrumCli FulcrumClient, proxmoxCli ProxmoxClient) *MetricsReporter {
	return &MetricsReporter{
		fulcrumCli: fulcrumCli,
		proxmoxCli: proxmoxCli,
	}
}

func (m *MetricsReporter) Report() error {
	// Get the services
	services, err := m.fulcrumCli.GetServices()
	if err != nil {
		return err
	}
	// Iterate from resource nodes
	for _, service := range services {
		if service.Resources == nil || service.Resources.Nodes == nil || service.ExternalID == nil {
			continue
		}
		for name, id := range service.Resources.Nodes {
			// Get the node metrics
			info, err := m.proxmoxCli.GetVMInfo(id)
			if err != nil {
				slog.Error("failed to get VM info", "id", id, "error", err)
				continue
			}
			if info.State != VMStateRunning {
				continue
			}
			var metrics []MetricEntry
			// Report the metrics
			metrics = append(metrics, MetricEntry{
				ExternalID: *service.ExternalID,
				ResourceID: fmt.Sprintf("%s-%s", service.ID, name),
				Value:      info.CPU,
				TypeName:   MetricTypeVMCPUUsage,
			})
			metrics = append(metrics, MetricEntry{
				ExternalID: *service.ExternalID,
				ResourceID: fmt.Sprintf("%s-%s", service.ID, name),
				Value:      float64(info.Memory),
				TypeName:   MetricTypeVMMemoryUsage,
			})
			for _, metric := range metrics {
				if err := m.fulcrumCli.ReportMetric(&metric); err != nil {
					return err

				}
			}
		}
	}
	return nil
}
