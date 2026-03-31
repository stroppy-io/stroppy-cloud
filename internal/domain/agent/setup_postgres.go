package agent

import (
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

// PostgresInstallConfig is the agent payload for postgres installation.
type PostgresInstallConfig struct {
	Version  string            `json:"version"`
	DataDir  string            `json:"data_dir"`
	Packages *types.PackageSet `json:"packages,omitempty"` // custom packages override
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
