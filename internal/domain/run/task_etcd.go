package run

import (
	"fmt"
	"strings"
	"sync"

	"go.uber.org/zap"

	"github.com/stroppy-io/stroppy-cloud/internal/core/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
)

type etcdInstallTask struct {
	client agent.Client
	state  *State
}

func (t *etcdInstallTask) Execute(nc *dag.NodeContext) error {
	// etcd is colocated on DB nodes (first 3).
	targets := t.state.DBTargets()
	if len(targets) > 3 {
		targets = targets[:3]
	}
	nc.Log().Info("installing etcd on DB nodes")
	return t.client.SendAll(nc, targets, agent.Command{
		Action: agent.ActionInstallEtcd,
		Config: agent.EtcdInstallConfig{Version: "3.5.17"},
	})
}

type etcdConfigTask struct {
	client agent.Client
	state  *State
}

func (t *etcdConfigTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.DBTargets()
	if len(targets) > 3 {
		targets = targets[:3]
	}
	nc.Log().Info("configuring etcd cluster", zap.Int("nodes", len(targets)))

	// Build initial-cluster string.
	var clusterParts []string
	for i, tgt := range targets {
		name := fmt.Sprintf("etcd%d", i)
		host := tgt.InternalHost
		if host == "" {
			host = tgt.Host
		}
		clusterParts = append(clusterParts, fmt.Sprintf("%s=http://%s:2380", name, host))
	}
	initialCluster := strings.Join(clusterParts, ",")

	// All etcd nodes must start ~simultaneously for cluster formation.
	// Send config to all in parallel.
	var wg sync.WaitGroup
	var firstErr error
	var once sync.Once

	for i, target := range targets {
		host := target.InternalHost
		if host == "" {
			host = target.Host
		}
		cfg := agent.EtcdClusterConfig{
			Name:            fmt.Sprintf("etcd%d", i),
			InitialCluster:  initialCluster,
			ClientURL:       "http://0.0.0.0:2379",
			PeerURL:         "http://0.0.0.0:2380",
			AdvertiseClient: fmt.Sprintf("http://%s:2379", host),
			AdvertisePeer:   fmt.Sprintf("http://%s:2380", host),
			State:           "new",
		}

		wg.Add(1)
		go func(tgt agent.Target, c agent.EtcdClusterConfig) {
			defer wg.Done()
			if err := t.client.Send(nc, tgt, agent.Command{Action: agent.ActionConfigEtcd, Config: c}); err != nil {
				once.Do(func() { firstErr = err })
			}
		}(target, cfg)
	}

	wg.Wait()
	return firstErr
}
