package run

import (
	"github.com/stroppy-io/hatchet-workflow/internal/core/dag"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/agent"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/types"
)

type monitorInstallTask struct {
	client agent.Client
	state  *State
}

func (t *monitorInstallTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.MonitorTargets()
	nc.Log().Info("installing monitoring stack")
	return t.client.SendAll(nc, targets, agent.Command{
		Action: agent.ActionInstallMonitor,
		Config: agent.MonitorInstallConfig{},
	})
}

type monitorConfigTask struct {
	client  agent.Client
	state   *State
	monitor types.MonitorConfig
}

func (t *monitorConfigTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.MonitorTargets()
	nc.Log().Info("configuring monitoring")
	cfg := agent.MonitorSetupConfig{
		MetricsEndpoint: t.monitor.MetricsEndpoint,
		LogsEndpoint:    t.monitor.LogsEndpoint,
		ScrapeTargets:   t.state.AllTargetHosts(),
	}
	return t.client.SendAll(nc, targets, agent.Command{
		Action: agent.ActionConfigMonitor,
		Config: cfg,
	})
}
