package run

import (
	"fmt"

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
	client          agent.Client
	state           *State
	monitor         types.MonitorConfig
	runID           string
	dbKind          types.DatabaseKind
	monitoringURL   string
	monitoringToken string
	accountID       int32
}

func (t *monitorConfigTask) Execute(nc *dag.NodeContext) error {
	allTargets := t.state.AllTargets()
	nc.Log().Info("configuring monitoring on all machines", zap.Int("count", len(allTargets)))

	// VictoriaMetrics remote_write endpoint -- derived from monitoringURL + accountID.
	metricsEndpoint := t.monitor.MetricsEndpoint
	if metricsEndpoint == "" && t.monitoringURL != "" {
		metricsEndpoint = fmt.Sprintf("%s/insert/%d/prometheus/api/v1/write", t.monitoringURL, t.accountID)
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
		BearerToken:     t.monitoringToken,
	}
	return t.client.SendAll(nc, allTargets, agent.Command{
		Action: agent.ActionConfigMonitor,
		Config: cfg,
	})
}
