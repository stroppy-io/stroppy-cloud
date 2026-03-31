package agent

import (
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

// PicodataInstallConfig is the agent payload for Picodata installation.
type PicodataInstallConfig struct {
	Version  string            `json:"version"`
	DataDir  string            `json:"data_dir"`
	Packages *types.PackageSet `json:"packages,omitempty"`
}

// PicodataClusterConfig is the agent payload for Picodata cluster setup.
type PicodataClusterConfig struct {
	InstanceID  int               `json:"instance_id"`
	Peers       []string          `json:"peers"` // addresses of all instances
	Replication int               `json:"replication_factor"`
	Shards      int               `json:"shards"`
	Options     map[string]string `json:"options,omitempty"`
}
