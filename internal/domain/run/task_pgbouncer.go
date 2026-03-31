package run

import (
	"github.com/stroppy-io/stroppy-cloud/internal/core/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
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
	client   agent.Client
	state    *State
	topology *types.PostgresTopology
}

func (t *pgBouncerConfigTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.DBTargets()
	nc.Log().Info("configuring pgbouncer")

	// Default auth_type is "trust" for testing; topology Options can override it.
	authType := "trust"
	if t.topology != nil {
		if v, ok := t.topology.Options["pgbouncer_auth_type"]; ok && v != "" {
			authType = v
		}
	}

	cfg := agent.PgBouncerConfig{
		ListenPort:      6432,
		PoolMode:        "transaction",
		MaxClientConn:   1000,
		DefaultPoolSize: 25,
		PGHost:          "127.0.0.1",
		PGPort:          5432,
		AuthType:        authType,
	}
	return t.client.SendAll(nc, targets, agent.Command{
		Action: agent.ActionConfigPgBouncer,
		Config: cfg,
	})
}
