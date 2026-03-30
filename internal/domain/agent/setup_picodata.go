package agent

import (
	"github.com/stroppy-io/hatchet-workflow/internal/core/dag"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/types"
)

// PicodataInstallConfig is the agent payload for Picodata installation.
type PicodataInstallConfig struct {
	Version string `json:"version"`
	DataDir string `json:"data_dir"`
}

// PicodataClusterConfig is the agent payload for Picodata cluster setup.
type PicodataClusterConfig struct {
	InstanceID  int               `json:"instance_id"`
	Peers       []string          `json:"peers"` // addresses of all instances
	Replication int               `json:"replication_factor"`
	Shards      int               `json:"shards"`
	Options     map[string]string `json:"options,omitempty"`
}

// --- DAG tasks ---

type picodataInstallTask struct {
	client   Client
	targets  []Target
	topology *types.PicodataTopology
	version  string
}

func (t *picodataInstallTask) Execute(nc *dag.NodeContext) error {
	cfg := PicodataInstallConfig{
		Version: t.version,
		DataDir: "/var/lib/picodata",
	}
	return t.client.SendAll(nc, t.targets, Command{
		Action: ActionInstallPicodata,
		Config: cfg,
	})
}

type picodataConfigTask struct {
	client   Client
	targets  []Target
	topology *types.PicodataTopology
}

func (t *picodataConfigTask) Execute(nc *dag.NodeContext) error {
	peers := make([]string, len(t.targets))
	for i, tgt := range t.targets {
		peers[i] = tgt.Host
	}

	for i, target := range t.targets {
		cfg := PicodataClusterConfig{
			InstanceID:  i,
			Peers:       peers,
			Replication: t.topology.Replication,
			Shards:      t.topology.Shards,
			Options:     t.topology.Options,
		}
		if err := t.client.Send(nc, target, Command{Action: ActionConfigPicodata, Config: cfg}); err != nil {
			return err
		}
	}
	return nil
}
