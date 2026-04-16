package run

import (
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/core/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

type pgInstallTask struct {
	client   agent.Client
	state    *State
	version  string
	topology *types.PostgresTopology
	pkg      *types.Package
}

func (t *pgInstallTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.DBTargets()
	nc.Log().Info("installing postgres on targets")
	return t.client.SendAll(nc, targets, agent.Command{
		Action: agent.ActionInstallPostgres,
		Config: agent.PostgresInstallConfig{
			Version: t.version,
			DataDir: "/var/lib/postgresql/data",
			Package: t.pkg,
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

	masterHost := targets[0].InternalHost
	if masterHost == "" {
		masterHost = targets[0].Host
	}

	for i, target := range targets {
		role := "replica"
		if i == 0 {
			role = "master"
		}
		opts := t.topology.MasterOptions
		if role == "replica" {
			opts = t.topology.ReplicaOptions
		}
		cfg := agent.PostgresClusterConfig{
			Version:      t.version,
			Role:         role,
			MasterHost:   masterHost,
			Patroni:      t.topology.Patroni,
			SyncReplicas: t.topology.SyncReplicas,
			Options:      opts,
		}
		if err := t.client.Send(nc, target, agent.Command{Action: agent.ActionConfigPostgres, Config: cfg}); err != nil {
			return err
		}
	}
	// Store effective config.
	m := t.topology.Master
	ec := map[string]string{
		"kind":    "postgres",
		"version": t.version,
		"master":  fmt.Sprintf("%d× %d vCPU / %d MB / %d GB", m.Count, m.CPUs, m.MemoryMB, m.DiskGB),
	}
	if len(t.topology.Replicas) > 0 {
		r := t.topology.Replicas[0]
		ec["replicas"] = fmt.Sprintf("%d× %d vCPU / %d MB", r.Count, r.CPUs, r.MemoryMB)
	}
	if t.topology.Patroni {
		ec["ha"] = "patroni + etcd"
	}
	if t.topology.PgBouncer {
		ec["pooler"] = "pgbouncer"
	}
	for k, v := range t.topology.MasterOptions {
		ec[k] = v
	}
	t.state.SetEffectiveConfig("database", ec)

	// DB endpoint is set by machinesTask with the container name (for container-to-container).
	return nil
}
