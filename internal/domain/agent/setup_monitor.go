package agent

import (
	"github.com/stroppy-io/hatchet-workflow/internal/core/dag"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/types"
)

// MonitorInstallConfig is the agent payload for monitoring stack installation.
type MonitorInstallConfig struct{}

// MonitorSetupConfig is the agent payload for monitoring configuration.
type MonitorSetupConfig struct {
	MetricsEndpoint string   `json:"metrics_endpoint"`
	LogsEndpoint    string   `json:"logs_endpoint"`
	ScrapeTargets   []string `json:"scrape_targets"` // host:port of all monitored nodes
}

// --- DAG tasks ---

type monitorInstallTask struct {
	client  Client
	targets []Target
}

func (t *monitorInstallTask) Execute(nc *dag.NodeContext) error {
	return t.client.SendAll(nc, t.targets, Command{
		Action: ActionInstallMonitor,
		Config: MonitorInstallConfig{},
	})
}

type monitorConfigTask struct {
	client        Client
	targets       []Target
	monitor       types.MonitorConfig
	scrapeTargets []string
}

func (t *monitorConfigTask) Execute(nc *dag.NodeContext) error {
	cfg := MonitorSetupConfig{
		MetricsEndpoint: t.monitor.MetricsEndpoint,
		LogsEndpoint:    t.monitor.LogsEndpoint,
		ScrapeTargets:   t.scrapeTargets,
	}
	return t.client.SendAll(nc, t.targets, Command{
		Action: ActionConfigMonitor,
		Config: cfg,
	})
}
