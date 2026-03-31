package run

import (
	"github.com/stroppy-io/stroppy-cloud/internal/core/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

type mysqlInstallTask struct {
	client   agent.Client
	state    *State
	version  string
	topology *types.MySQLTopology
	packages *types.PackageSet
}

func (t *mysqlInstallTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.DBTargets()
	nc.Log().Info("installing mysql on targets")
	return t.client.SendAll(nc, targets, agent.Command{
		Action: agent.ActionInstallMySQL,
		Config: agent.MySQLInstallConfig{
			Version:  t.version,
			DataDir:  "/var/lib/mysql",
			Packages: t.packages,
		},
	})
}

type mysqlConfigTask struct {
	client   agent.Client
	state    *State
	topology *types.MySQLTopology
}

func (t *mysqlConfigTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.DBTargets()
	nc.Log().Info("configuring mysql cluster")
	for i, target := range targets {
		role := "replica"
		if i == 0 {
			role = "primary"
		}
		cfg := agent.MySQLClusterConfig{
			Role:        role,
			PrimaryHost: targets[0].Host,
			GroupRepl:   t.topology.GroupRepl,
			Options:     t.topology.Options,
		}
		if err := t.client.Send(nc, target, agent.Command{Action: agent.ActionConfigMySQL, Config: cfg}); err != nil {
			return err
		}
	}
	return nil
}
