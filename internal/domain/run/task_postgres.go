package run

import (
	"github.com/stroppy-io/hatchet-workflow/internal/core/dag"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/agent"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/types"
)

type pgInstallTask struct {
	client   agent.Client
	state    *State
	version  string
	topology *types.PostgresTopology
}

func (t *pgInstallTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.DBTargets()
	nc.Log().Info("installing postgres on targets")
	return t.client.SendAll(nc, targets, agent.Command{
		Action: agent.ActionInstallPostgres,
		Config: agent.PostgresInstallConfig{
			Version: t.version,
			DataDir: "/var/lib/postgresql/data",
		},
	})
}

type pgConfigTask struct {
	client   agent.Client
	state    *State
	version  string
	topology *types.PostgresTopology
}

func (t *pgConfigTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.DBTargets()
	nc.Log().Info("configuring postgres cluster")
	for i, target := range targets {
		role := "replica"
		if i == 0 {
			role = "master"
		}
		masterHost := targets[0].InternalHost
		if masterHost == "" {
			masterHost = targets[0].Host
		}
		cfg := agent.PostgresClusterConfig{
			Version:      t.version,
			Role:         role,
			MasterHost:   masterHost,
			Patroni:      t.topology.Patroni,
			SyncReplicas: t.topology.SyncReplicas,
			Options:      t.topology.Options,
		}
		if err := t.client.Send(nc, target, agent.Command{Action: agent.ActionConfigPostgres, Config: cfg}); err != nil {
			return err
		}
	}
	// DB endpoint is set by machinesTask with the container name (for container-to-container).
	return nil
}
