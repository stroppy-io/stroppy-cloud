package run

import (
	"github.com/stroppy-io/hatchet-workflow/internal/core/dag"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/agent"
)

type pgBouncerInstallTask struct {
	client agent.Client
	state  *State
}

func (t *pgBouncerInstallTask) Execute(nc *dag.NodeContext) error {
	// PgBouncer is colocated on all DB nodes.
	targets := t.state.DBTargets()
	nc.Log().Info("installing pgbouncer on DB nodes")
	return t.client.SendAll(nc, targets, agent.Command{
		Action: agent.ActionInstallPgBouncer,
		Config: agent.PgBouncerInstallConfig{},
	})
}

type pgBouncerConfigTask struct {
	client agent.Client
	state  *State
}

func (t *pgBouncerConfigTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.DBTargets()
	nc.Log().Info("configuring pgbouncer")
	cfg := agent.PgBouncerConfig{
		ListenPort:      6432,
		PoolMode:        "transaction",
		MaxClientConn:   1000,
		DefaultPoolSize: 25,
		PGHost:          "127.0.0.1",
		PGPort:          5432,
		AuthType:        "trust",
	}
	return t.client.SendAll(nc, targets, agent.Command{
		Action: agent.ActionConfigPgBouncer,
		Config: cfg,
	})
}
