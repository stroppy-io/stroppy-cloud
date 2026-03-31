package run

import (
	"go.uber.org/zap"

	"github.com/stroppy-io/stroppy-cloud/internal/core/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

type monitorInstallTask struct {
	client agent.Client
	state  *State
	dbKind types.DatabaseKind
}

func (t *monitorInstallTask) Execute(nc *dag.NodeContext) error {
	allTargets := t.state.AllTargets()
	nc.Log().Info("installing monitoring exporters on all machines", zap.Int("count", len(allTargets)))
	return t.client.SendAll(nc, allTargets, agent.Command{
		Action: agent.ActionInstallMonitor,
		Config: agent.MonitorInstallConfig{
			DatabaseKind: string(t.dbKind),
		},
	})
}

type monitorConfigTask struct {
	client   agent.Client
	state    *State
	monitor  types.MonitorConfig
	runID    string
	settings *types.ServerSettings
	dbKind   types.DatabaseKind
}

func (t *monitorConfigTask) Execute(nc *dag.NodeContext) error {
	allTargets := t.state.AllTargets()
	nc.Log().Info("configuring monitoring on all machines", zap.Int("count", len(allTargets)))

	// VictoriaMetrics remote_write endpoint -- accessible from agent containers.
	metricsEndpoint := t.monitor.MetricsEndpoint
	if metricsEndpoint == "" && t.settings != nil && t.settings.Monitoring.VictoriaMetricsURL != "" {
		metricsEndpoint = t.settings.Monitoring.VictoriaMetricsURL + "/api/v1/write"
	}
	if metricsEndpoint == "" {
		// Default: host's VictoriaMetrics via docker bridge.
		metricsEndpoint = "http://172.17.0.1:8428/api/v1/write"
	}

	// Scrape targets -- use InternalHost (container names) for container-to-container scraping.
	var scrapeHosts []string
	for _, tgt := range allTargets {
		host := tgt.InternalHost
		if host == "" {
			host = tgt.Host
		}
		scrapeHosts = append(scrapeHosts, host)
	}

	cfg := agent.MonitorSetupConfig{
		MetricsEndpoint: metricsEndpoint,
		LogsEndpoint:    t.monitor.LogsEndpoint,
		ScrapeTargets:   scrapeHosts,
		RunID:           t.runID,
		DatabaseKind:    string(t.dbKind),
	}
	return t.client.SendAll(nc, allTargets, agent.Command{
		Action: agent.ActionConfigMonitor,
		Config: cfg,
	})
}
