package agent

import (
	"github.com/stroppy-io/hatchet-workflow/internal/core/dag"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/types"
)

// MySQLInstallConfig is the agent payload for MySQL installation.
type MySQLInstallConfig struct {
	Version string `json:"version"`
	DataDir string `json:"data_dir"`
}

// MySQLClusterConfig is the agent payload for MySQL cluster setup.
type MySQLClusterConfig struct {
	Role        string            `json:"role"` // "primary" or "replica"
	PrimaryHost string            `json:"primary_host,omitempty"`
	GroupRepl   bool              `json:"group_replication"`
	Options     map[string]string `json:"options,omitempty"`
}

// --- DAG tasks ---

type mysqlInstallTask struct {
	client   Client
	targets  []Target
	topology *types.MySQLTopology
	version  string
}

func (t *mysqlInstallTask) Execute(nc *dag.NodeContext) error {
	cfg := MySQLInstallConfig{
		Version: t.version,
		DataDir: "/var/lib/mysql",
	}
	return t.client.SendAll(nc, t.targets, Command{
		Action: ActionInstallMySQL,
		Config: cfg,
	})
}

type mysqlConfigTask struct {
	client   Client
	targets  []Target
	topology *types.MySQLTopology
}

func (t *mysqlConfigTask) Execute(nc *dag.NodeContext) error {
	for i, target := range t.targets {
		role := "replica"
		if i == 0 {
			role = "primary"
		}
		cfg := MySQLClusterConfig{
			Role:        role,
			PrimaryHost: t.targets[0].Host,
			GroupRepl:   t.topology.GroupRepl,
			Options:     t.topology.Options,
		}
		if err := t.client.Send(nc, target, Command{Action: ActionConfigMySQL, Config: cfg}); err != nil {
			return err
		}
	}
	return nil
}
