// Collects VM metrics for services in lifecycle-defined running states.
// Each metric identifies the service through its agent instance ID.
package agent

import (
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
	// Handle pagination - we'll process all pages
	currentPage := 1
	hasMorePages := true

	for hasMorePages {
		// Get the services - one page at a time
		services, err := m.fulcrumCli.GetServices(currentPage)
		if err != nil {
			return err
		}

		// Process services in current page
		for _, service := range services.Items {
			running, err := service.IsRunning()
			if err != nil {
				return err
			}
			if !running {
				continue
			}
			if service.AgentInstanceData == nil || service.AgentInstanceData.Nodes == nil || service.AgentInstanceID == nil {
				continue
			}

			// Process this service's metrics
			if err := m.processServiceMetrics(service); err != nil {
				return err
			}
		}

		// Check if there are more pages
		hasMorePages = services.HasNext
		currentPage++
	}

	return nil
}

// processServiceMetrics collects and reports metrics for a single service
func (m *MetricsReporter) processServiceMetrics(service *Service) error {
	for name, id := range service.AgentInstanceData.Nodes {
		// Get the node metrics
		info, err := m.proxmoxCli.GetVMInfo(id)
		if err != nil {
			slog.Error("failed to get VM info", "id", id, "error", err)
			continue
		}
		if info.Status != VMStatusRunning {
			continue
		}
		var metrics []MetricEntry
		// Report the metrics
		metrics = append(metrics, MetricEntry{
			AgentInstanceID: *service.AgentInstanceID,
			ResourceID:      name,
			Value:           info.CPU,
			TypeName:        MetricTypeVMCPUUsage,
		})
		metrics = append(metrics, MetricEntry{
			AgentInstanceID: *service.AgentInstanceID,
			ResourceID:      name,
			Value:           float64(info.Memory),
			TypeName:        MetricTypeVMMemoryUsage,
		})
		for _, metric := range metrics {
			if err := m.fulcrumCli.ReportMetric(&metric); err != nil {
				return err

			}
		}
	}

	return nil
}
