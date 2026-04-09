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
		cfg := agent.YDBDatabaseConfig{
			StaticEndpoints: staticHosts,
			AdvertiseHost:   advHost,
			DatabasePath:    dbPath,
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
