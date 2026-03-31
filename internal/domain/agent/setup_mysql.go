package agent

import (
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

// MySQLInstallConfig is the agent payload for MySQL installation.
type MySQLInstallConfig struct {
	Version  string            `json:"version"`
	DataDir  string            `json:"data_dir"`
	Packages *types.PackageSet `json:"packages,omitempty"`
}

// MySQLClusterConfig is the agent payload for MySQL cluster setup.
type MySQLClusterConfig struct {
	Role        string            `json:"role"` // "primary" or "replica"
	PrimaryHost string            `json:"primary_host,omitempty"`
	GroupRepl   bool              `json:"group_replication"`
	Options     map[string]string `json:"options,omitempty"`
}
