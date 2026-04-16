package run

import (
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/core/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

type mysqlInstallTask struct {
	client   agent.Client
	state    *State
	version  string
	topology *types.MySQLTopology
	pkg      *types.Package
}

func (t *mysqlInstallTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.DBTargets()
	nc.Log().Info("installing mysql on targets")
	return t.client.SendAll(nc, targets, agent.Command{
		Action: agent.ActionInstallMySQL,
		Config: agent.MySQLInstallConfig{
			Version: t.version,
			DataDir: "/var/lib/mysql",
			Package: t.pkg,
		},
	})
}

type mysqlConfigTask struct {
	client   agent.Client
	state    *State
	topology *types.MySQLTopology
}

func (t *mysqlConfigTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.DBTargets()
	nc.Log().Info("configuring mysql cluster")

	// Build group seeds list (all nodes at port 33061) for Group Replication.
	var groupSeeds []string
	if t.topology.GroupRepl {
		for _, tgt := range targets {
			host := tgt.InternalHost
			if host == "" {
				host = tgt.Host
			}
			groupSeeds = append(groupSeeds, fmt.Sprintf("%s:33061", host))
		}
	}

	// Use a fixed UUID for group_replication_group_name.
	groupName := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"

	for i, target := range targets {
		role := "replica"
		if i == 0 {
			role = "primary"
		}
		localHost := target.InternalHost
		if localHost == "" {
			localHost = target.Host
		}
		primaryHost := targets[0].InternalHost
		if primaryHost == "" {
			primaryHost = targets[0].Host
		}
		opts := t.topology.PrimaryOptions
		if role == "replica" {
			opts = t.topology.ReplicaOptions
		}
		cfg := agent.MySQLClusterConfig{
			Role:        role,
			PrimaryHost: primaryHost,
			LocalHost:   localHost,
			NodeIndex:   i,
			SemiSync:    t.topology.SemiSync,
			GroupRepl:   t.topology.GroupRepl,
			GroupSeeds:  groupSeeds,
			GroupName:   groupName,
			Options:     opts,
		}
		if err := t.client.Send(nc, target, agent.Command{Action: agent.ActionConfigMySQL, Config: cfg}); err != nil {
			return err
		}
	}

	// Store effective config.
	p := t.topology.Primary
	ec := map[string]string{
		"kind":    "mysql",
		"primary": fmt.Sprintf("%d× %d vCPU / %d MB / %d GB", p.Count, p.CPUs, p.MemoryMB, p.DiskGB),
	}
	if len(t.topology.Replicas) > 0 {
		r := t.topology.Replicas[0]
		ec["replicas"] = fmt.Sprintf("%d× %d vCPU / %d MB", r.Count, r.CPUs, r.MemoryMB)
	}
	if t.topology.GroupRepl {
		ec["replication"] = "group"
	} else if t.topology.SemiSync {
		ec["replication"] = "semi-sync"
	}
	for k, v := range t.topology.PrimaryOptions {
		ec[k] = v
	}
	t.state.SetEffectiveConfig("database", ec)

	return nil
}
