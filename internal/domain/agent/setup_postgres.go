package agent

import (
	"github.com/stroppy-io/hatchet-workflow/internal/core/dag"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/types"
)

// PostgresInstallConfig is the agent payload for postgres installation.
type PostgresInstallConfig struct {
	Version string `json:"version"`
	DataDir string `json:"data_dir"`
}

// PostgresClusterConfig is the agent payload for postgres cluster setup.
type PostgresClusterConfig struct {
	Version      string            `json:"version"`
	Role         string            `json:"role"` // "master" or "replica"
	MasterHost   string            `json:"master_host,omitempty"`
	Patroni      bool              `json:"patroni"`
	SyncReplicas int               `json:"sync_replicas"`
	Options      map[string]string `json:"options,omitempty"`
}

// --- DAG tasks ---

type postgresInstallTask struct {
	client   Client
	targets  []Target
	topology *types.PostgresTopology
	version  string
}

func (t *postgresInstallTask) Execute(nc *dag.NodeContext) error {
	cfg := PostgresInstallConfig{
		Version: t.version,
		DataDir: "/var/lib/postgresql/data",
	}
	return t.client.SendAll(nc, t.targets, Command{
		Action: ActionInstallPostgres,
		Config: cfg,
	})
}

type postgresConfigTask struct {
	client   Client
	targets  []Target
	topology *types.PostgresTopology
}

func (t *postgresConfigTask) Execute(nc *dag.NodeContext) error {
	for i, target := range t.targets {
		role := "replica"
		if i == 0 {
			role = "master"
		}
		cfg := PostgresClusterConfig{
			Role:         role,
			MasterHost:   t.targets[0].Host,
			Patroni:      t.topology.Patroni,
			SyncReplicas: t.topology.SyncReplicas,
			Options:      t.topology.Options,
		}
		if err := t.client.Send(nc, target, Command{Action: ActionConfigPostgres, Config: cfg}); err != nil {
			return err
		}
	}
	return nil
}
