package run

import (
	"fmt"
	"sync"

	"github.com/stroppy-io/stroppy-cloud/internal/core/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

type ydbInstallTask struct {
	client   agent.Client
	state    *State
	version  string
	topology *types.YDBTopology
	pkg      *types.Package
}

func (t *ydbInstallTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.DBTargets()
	nc.Log().Info("installing YDB on targets")
	return t.client.SendAll(nc, targets, agent.Command{
		Action: agent.ActionInstallYDB,
		Config: agent.YDBInstallConfig{Version: t.version},
	})
}

type ydbConfigTask struct {
	client   agent.Client
	state    *State
	topology *types.YDBTopology
}

func (t *ydbConfigTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.DBTargets()
	nc.Log().Info("configuring YDB static nodes")

	hosts := make([]string, len(targets))
	for i, tgt := range targets {
		h := tgt.InternalHost
		if h == "" {
			h = tgt.Host
		}
		hosts[i] = h
	}

	ft := t.topology.FaultTolerance
	if ft == "" {
		ft = "none"
	}

	// Start all static nodes in parallel — YDB needs all nodes up to form a cluster.
	var wg sync.WaitGroup
	errs := make([]error, len(targets))
	for i, target := range targets {
		advHost := target.InternalHost
		if advHost == "" {
			advHost = target.Host
		}
		cfg := agent.YDBStaticConfig{
			Hosts:          hosts,
			InstanceID:     i,
			AdvertiseHost:  advHost,
			DiskPath:       "/ydb_data",
			DiskGB:         t.topology.Storage.DiskGB,
			MemoryMB:       t.topology.Storage.MemoryMB,
			CPUs:           t.topology.Storage.CPUs,
			FaultTolerance: ft,
			Options:        t.topology.StorageOptions,
		}
		wg.Add(1)
		go func(idx int, tgt agent.Target, c agent.YDBStaticConfig) {
			defer wg.Done()
			errs[idx] = t.client.Send(nc, tgt, agent.Command{
				Action: agent.ActionConfigYDB, Config: c,
			})
		}(i, target, cfg)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return err
		}
	}

	// Store effective config for UI display.
	st := t.topology.Storage
	diskGB := st.DiskGB
	if diskGB <= 0 {
		diskGB = 80
	}
	memMB := st.MemoryMB
	if memMB <= 0 {
		memMB = 4096
	}
	cpus := st.CPUs
	if cpus <= 0 {
		cpus = 2
	}
	pdiskGB := diskGB - 2
	if pdiskGB < 10 {
		pdiskGB = 10
	}
	hardMB := memMB * 85 / 100
	softMB := hardMB * 90 / 100
	nodes := fmt.Sprintf("%d storage", st.Count)
	if t.topology.Database != nil {
		nodes += fmt.Sprintf(" + %d compute", t.topology.Database.Count)
	} else {
		nodes += " (combined)"
	}
	t.state.SetEffectiveConfig("database", map[string]string{
		"kind":      "ydb",
		"nodes":     nodes,
		"per_node":  fmt.Sprintf("%d vCPU / %d MB / %d GB", cpus, memMB, diskGB),
		"pdisk_gb":  fmt.Sprintf("%d", pdiskGB),
		"mem_hard":  fmt.Sprintf("%d MB", hardMB),
		"mem_soft":  fmt.Sprintf("%d MB", softMB),
		"cpu_count": fmt.Sprintf("%d", cpus),
		"erasure":   ft,
		"db_path":   t.topology.DatabasePath,
	})

	return nil
}

type ydbInitTask struct {
	client   agent.Client
	state    *State
	topology *types.YDBTopology
}

func (t *ydbInitTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.DBTargets()
	if len(targets) == 0 {
		return fmt.Errorf("no DB targets for YDB init")
	}

	first := targets[0]
	host := first.InternalHost
	if host == "" {
		host = first.Host
	}

	dbPath := t.topology.DatabasePath
	if dbPath == "" {
		dbPath = "/Root/testdb"
	}

	nc.Log().Info("initializing YDB cluster")
	return t.client.Send(nc, first, agent.Command{
		Action: agent.ActionInitYDB,
		Config: agent.YDBInitConfig{
			StaticEndpoint: fmt.Sprintf("grpc://%s:2136", host),
			DatabasePath:   dbPath,
			ConfigPath:     "/opt/ydb/cfg/config.yaml",
		},
	})
}

type ydbStartDBTask struct {
	client   agent.Client
	state    *State
	topology *types.YDBTopology
}

func (t *ydbStartDBTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.DBTargets()
	nc.Log().Info("starting YDB database nodes")

	staticHosts := make([]string, len(targets))
	for i, tgt := range targets {
		h := tgt.InternalHost
		if h == "" {
			h = tgt.Host
		}
		staticHosts[i] = h
	}

	dbPath := t.topology.DatabasePath
	if dbPath == "" {
		dbPath = "/Root/testdb"
	}

	// Start all dynamic nodes in parallel.
	var wg sync.WaitGroup
	errs := make([]error, len(targets))
	for i, target := range targets {
		advHost := target.InternalHost
		if advHost == "" {
			advHost = target.Host
		}
		// Use database node specs if split mode, otherwise storage specs.
		memMB := t.topology.Storage.MemoryMB
		cpus := t.topology.Storage.CPUs
		if t.topology.Database != nil {
			memMB = t.topology.Database.MemoryMB
			cpus = t.topology.Database.CPUs
		}
		cfg := agent.YDBDatabaseConfig{
			StaticEndpoints: staticHosts,
			AdvertiseHost:   advHost,
			DatabasePath:    dbPath,
			MemoryMB:        memMB,
			CPUs:            cpus,
			Options:         t.topology.DatabaseOptions,
		}
		wg.Add(1)
		go func(idx int, tgt agent.Target, c agent.YDBDatabaseConfig) {
			defer wg.Done()
			errs[idx] = t.client.Send(nc, tgt, agent.Command{
				Action: agent.ActionStartYDBDB, Config: c,
			})
		}(i, target, cfg)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
