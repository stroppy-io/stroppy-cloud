package run

import (
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/core/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

type picoInstallTask struct {
	client   agent.Client
	state    *State
	version  string
	topology *types.PicodataTopology
	pkg      *types.Package
}

func (t *picoInstallTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.DBTargets()
	nc.Log().Info("installing picodata on targets")
	return t.client.SendAll(nc, targets, agent.Command{
		Action: agent.ActionInstallPicodata,
		Config: agent.PicodataInstallConfig{
			Version: t.version,
			DataDir: "/var/lib/picodata",
			Package: t.pkg,
		},
	})
}

type picoConfigTask struct {
	client   agent.Client
	state    *State
	topology *types.PicodataTopology
}

func (t *picoConfigTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.DBTargets()
	nc.Log().Info("configuring picodata cluster")

	peers := make([]string, len(targets))
	for i, tgt := range targets {
		// Use InternalHost (container name) for container-to-container communication.
		h := tgt.InternalHost
		if h == "" {
			h = tgt.Host
		}
		peers[i] = h
	}

	for i, target := range targets {
		// Use InternalHost as advertise address so other nodes can resolve it.
		// On Docker this is the container name; on YC VMs this is the internal IP.
		advHost := target.InternalHost
		if advHost == "" {
			advHost = target.Host
		}
		cfg := agent.PicodataClusterConfig{
			InstanceID:    i,
			AdvertiseHost: advHost,
			Peers:         peers,
			Replication:   t.topology.Replication,
			Shards:        t.topology.Shards,
			Options:       t.topology.InstanceOptions,
		}
		if err := t.client.Send(nc, target, agent.Command{Action: agent.ActionConfigPicodata, Config: cfg}); err != nil {
			return err
		}
	}

	// Store effective config.
	ec := map[string]string{
		"kind":      "picodata",
		"instances": fmt.Sprintf("%d", len(targets)),
		"shards":    fmt.Sprintf("%d", t.topology.Shards),
		"rf":        fmt.Sprintf("%d", t.topology.Replication),
	}
	if len(t.topology.Instances) > 0 {
		inst := t.topology.Instances[0]
		ec["per_node"] = fmt.Sprintf("%d vCPU / %d MB / %d GB", inst.CPUs, inst.MemoryMB, inst.DiskGB)
	}
	for k, v := range t.topology.InstanceOptions {
		ec[k] = v
	}
	t.state.SetEffectiveConfig("database", ec)

	return nil
}
