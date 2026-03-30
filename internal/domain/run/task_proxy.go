package run

import (
	"fmt"

	"github.com/stroppy-io/hatchet-workflow/internal/core/dag"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/agent"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/types"
)

type proxyInstallTask struct {
	client agent.Client
	state  *State
	dbKind types.DatabaseKind
}

func (t *proxyInstallTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.ProxyTargets()
	if len(targets) == 0 {
		nc.Log().Info("no proxy targets, skipping install")
		return nil
	}

	switch t.dbKind {
	case types.DatabasePostgres, types.DatabasePicodata:
		nc.Log().Info("installing haproxy")
		return t.client.SendAll(nc, targets, agent.Command{
			Action: agent.ActionInstallHAProxy,
			Config: agent.HAProxyInstallConfig{},
		})
	case types.DatabaseMySQL:
		nc.Log().Info("installing proxysql")
		return t.client.SendAll(nc, targets, agent.Command{
			Action: agent.ActionInstallProxySQL,
			Config: agent.ProxySQLInstallConfig{},
		})
	default:
		return nil
	}
}

type proxyConfigTask struct {
	client        agent.Client
	state         *State
	dbKind        types.DatabaseKind
	pgTopology    *types.PostgresTopology
	mysqlTopology *types.MySQLTopology
	picoTopology  *types.PicodataTopology
}

func (t *proxyConfigTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.ProxyTargets()
	if len(targets) == 0 {
		nc.Log().Info("no proxy targets, skipping config")
		return nil
	}

	dbTargets := t.state.DBTargets()

	switch t.dbKind {
	case types.DatabasePostgres:
		return t.configHAProxyPostgres(nc, targets, dbTargets)
	case types.DatabaseMySQL:
		return t.configProxySQLMySQL(nc, targets, dbTargets)
	case types.DatabasePicodata:
		return t.configHAProxyPicodata(nc, targets, dbTargets)
	default:
		return nil
	}
}

func (t *proxyConfigTask) configHAProxyPostgres(nc *dag.NodeContext, proxyTargets, dbTargets []agent.Target) error {
	nc.Log().Info("configuring haproxy for postgres (patroni health checks)")

	var backends []string
	for _, tgt := range dbTargets {
		host := tgt.InternalHost
		if host == "" {
			host = tgt.Host
		}
		backends = append(backends, fmt.Sprintf("%s:5432", host))
	}

	healthCheck := "patroni"
	patroniPort := 8008
	if t.pgTopology != nil && !t.pgTopology.Patroni {
		healthCheck = "tcp"
		patroniPort = 0
	}

	cfg := agent.HAProxyConfig{
		DBKind:      "postgres",
		WritePort:   5000,
		ReadPort:    5001,
		Backends:    backends,
		HealthCheck: healthCheck,
		PatroniPort: patroniPort,
	}

	return t.client.SendAll(nc, proxyTargets, agent.Command{
		Action: agent.ActionConfigHAProxy,
		Config: cfg,
	})
}

func (t *proxyConfigTask) configProxySQLMySQL(nc *dag.NodeContext, proxyTargets, dbTargets []agent.Target) error {
	nc.Log().Info("configuring proxysql for mysql")

	var backends []string
	for _, tgt := range dbTargets {
		host := tgt.InternalHost
		if host == "" {
			host = tgt.Host
		}
		backends = append(backends, fmt.Sprintf("%s:3306", host))
	}

	gr := false
	if t.mysqlTopology != nil {
		gr = t.mysqlTopology.GroupRepl
	}

	cfg := agent.ProxySQLConfig{
		ListenPort:       6033,
		AdminPort:        6032,
		Backends:         backends,
		GroupReplication: gr,
		WriterHostgroup:  10,
		ReaderHostgroup:  20,
	}

	return t.client.SendAll(nc, proxyTargets, agent.Command{
		Action: agent.ActionConfigProxySQL,
		Config: cfg,
	})
}

func (t *proxyConfigTask) configHAProxyPicodata(nc *dag.NodeContext, proxyTargets, dbTargets []agent.Target) error {
	nc.Log().Info("configuring haproxy for picodata (pgproto)")

	var backends []string
	for _, tgt := range dbTargets {
		host := tgt.InternalHost
		if host == "" {
			host = tgt.Host
		}
		backends = append(backends, fmt.Sprintf("%s:4327", host))
	}

	cfg := agent.HAProxyConfig{
		DBKind:      "picodata",
		WritePort:   4327,
		ReadPort:    4328,
		Backends:    backends,
		HealthCheck: "tcp",
	}

	return t.client.SendAll(nc, proxyTargets, agent.Command{
		Action: agent.ActionConfigHAProxy,
		Config: cfg,
	})
}
