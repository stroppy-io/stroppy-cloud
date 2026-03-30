package run

import (
	"fmt"

	"github.com/stroppy-io/hatchet-workflow/internal/core/dag"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/agent"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/types"
)

// Deps holds external dependencies injected into DAG tasks.
type Deps struct {
	Client   agent.Client
	Deployer *agent.DockerDeployer
	State    *State
	// ServerAddr is the server callback address for agents.
	ServerAddr string
}

// Build constructs a dag.Graph and dag.Registry from a RunConfig.
func Build(cfg types.RunConfig, deps Deps) (*dag.Graph, *dag.Registry, error) {
	b := &builder{
		g:    dag.New(),
		reg:  dag.NewRegistry(),
		cfg:  cfg,
		deps: deps,
	}
	if err := b.build(); err != nil {
		return nil, nil, err
	}
	return b.g, b.reg, nil
}

type builder struct {
	g    *dag.Graph
	reg  *dag.Registry
	cfg  types.RunConfig
	deps Deps
	// runStroppyDeps collects all phases that must complete before run_stroppy.
	runStroppyDeps []string
}

func (b *builder) ph(p types.Phase) string { return string(p) }

func (b *builder) add(phase string, deps []string, task dag.Task) {
	_ = b.g.Add(&dag.Node{ID: phase, Type: phase, Deps: deps, Task: task})
	b.reg.Register(phase, func() dag.Task { return task })
}

func (b *builder) build() error {
	afterMachines := []string{b.ph(types.PhaseMachines)}

	// --- infrastructure ---
	b.add(b.ph(types.PhaseNetwork), nil,
		&networkTask{cfg: b.cfg.Network, provider: b.cfg.Provider, deployer: b.deps.Deployer, state: b.deps.State})

	b.add(b.ph(types.PhaseMachines), []string{b.ph(types.PhaseNetwork)},
		&machinesTask{runCfg: b.cfg, state: b.deps.State, deployer: b.deps.Deployer, serverAddr: b.deps.ServerAddr})

	// --- etcd (if Postgres HA with etcd) ---
	configDBDeps := []string{b.ph(types.PhaseInstallDB)}
	if b.needsEtcd() {
		b.addEtcd(afterMachines)
		configDBDeps = append(configDBDeps, b.ph(types.PhaseConfigureEtcd))
	}

	// --- install DB ---
	installDB, configDB, err := b.dbTasks()
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}
	b.add(b.ph(types.PhaseInstallDB), afterMachines, installDB)
	b.add(b.ph(types.PhaseConfigureDB), configDBDeps, configDB)

	// --- monitoring ---
	b.add(b.ph(types.PhaseInstallMonitor), afterMachines,
		&monitorInstallTask{client: b.deps.Client, state: b.deps.State})
	b.add(b.ph(types.PhaseConfigureMonitor), []string{b.ph(types.PhaseInstallMonitor)},
		&monitorConfigTask{client: b.deps.Client, state: b.deps.State, monitor: b.cfg.Monitor})

	// --- pgbouncer (if Postgres HA with pgbouncer, colocated on DB nodes) ---
	if b.needsPgBouncer() {
		b.addPgBouncer(afterMachines)
	}

	// --- proxy (HAProxy for PG/Picodata, ProxySQL for MySQL) ---
	if b.needsProxy() {
		b.addProxy(afterMachines)
	}

	// --- stroppy ---
	b.add(b.ph(types.PhaseInstallStroppy), afterMachines,
		&stroppyInstallTask{client: b.deps.Client, state: b.deps.State, stroppy: b.cfg.Stroppy})

	// --- run stroppy (depends on all configure phases + install stroppy) ---
	b.runStroppyDeps = append(b.runStroppyDeps,
		b.ph(types.PhaseConfigureDB),
		b.ph(types.PhaseConfigureMonitor),
		b.ph(types.PhaseInstallStroppy),
	)
	b.add(b.ph(types.PhaseRunStroppy), b.runStroppyDeps,
		&stroppyRunTask{client: b.deps.Client, state: b.deps.State, stroppy: b.cfg.Stroppy, dbKind: b.cfg.Database.Kind})

	// --- teardown ---
	b.add(b.ph(types.PhaseTeardown), []string{b.ph(types.PhaseRunStroppy)},
		&teardownTask{provider: b.cfg.Provider, state: b.deps.State, deployer: b.deps.Deployer})

	if err := b.g.Validate(); err != nil {
		return fmt.Errorf("run: invalid graph: %w", err)
	}
	return nil
}

// --- conditional phase helpers ---

func (b *builder) needsEtcd() bool {
	if b.cfg.Database.Kind == types.DatabasePostgres && b.cfg.Database.Postgres != nil {
		return false // TODO: enable when etcd Docker support is ready
	}
	return false
}

func (b *builder) needsPgBouncer() bool {
	if b.cfg.Database.Kind == types.DatabasePostgres && b.cfg.Database.Postgres != nil {
		return b.cfg.Database.Postgres.PgBouncer
	}
	return false
}

func (b *builder) needsProxy() bool {
	db := b.cfg.Database
	switch db.Kind {
	case types.DatabasePostgres:
		return db.Postgres != nil && db.Postgres.HAProxy != nil
	case types.DatabaseMySQL:
		return db.MySQL != nil && db.MySQL.ProxySQL != nil
	case types.DatabasePicodata:
		return db.Picodata != nil && db.Picodata.HAProxy != nil
	}
	return false
}

func (b *builder) addEtcd(afterMachines []string) {
	b.add(b.ph(types.PhaseInstallEtcd), afterMachines,
		&etcdInstallTask{client: b.deps.Client, state: b.deps.State})
	b.add(b.ph(types.PhaseConfigureEtcd), []string{b.ph(types.PhaseInstallEtcd)},
		&etcdConfigTask{client: b.deps.Client, state: b.deps.State})
}

func (b *builder) addPgBouncer(afterMachines []string) {
	// PgBouncer depends on DB being configured (needs PG running).
	b.add(b.ph(types.PhaseInstallPgBouncer), afterMachines,
		&pgBouncerInstallTask{client: b.deps.Client, state: b.deps.State})
	b.add(b.ph(types.PhaseConfigurePgBouncer), []string{b.ph(types.PhaseInstallPgBouncer), b.ph(types.PhaseConfigureDB)},
		&pgBouncerConfigTask{client: b.deps.Client, state: b.deps.State})
	b.runStroppyDeps = append(b.runStroppyDeps, b.ph(types.PhaseConfigurePgBouncer))
}

func (b *builder) addProxy(afterMachines []string) {
	// Proxy depends on DB being configured (needs backends list).
	b.add(b.ph(types.PhaseInstallProxy), afterMachines,
		&proxyInstallTask{client: b.deps.Client, state: b.deps.State, dbKind: b.cfg.Database.Kind})
	b.add(b.ph(types.PhaseConfigureProxy), []string{b.ph(types.PhaseInstallProxy), b.ph(types.PhaseConfigureDB)},
		&proxyConfigTask{client: b.deps.Client, state: b.deps.State, dbKind: b.cfg.Database.Kind,
			pgTopology: b.cfg.Database.Postgres, mysqlTopology: b.cfg.Database.MySQL, picoTopology: b.cfg.Database.Picodata})
	b.runStroppyDeps = append(b.runStroppyDeps, b.ph(types.PhaseConfigureProxy))
}

// --- DB task factory ---

func (b *builder) dbTasks() (install dag.Task, config dag.Task, err error) {
	db := b.cfg.Database
	switch db.Kind {
	case types.DatabasePostgres:
		return &pgInstallTask{client: b.deps.Client, state: b.deps.State, version: db.Version, topology: db.Postgres},
			&pgConfigTask{client: b.deps.Client, state: b.deps.State, version: db.Version, topology: db.Postgres}, nil
	case types.DatabaseMySQL:
		return &mysqlInstallTask{client: b.deps.Client, state: b.deps.State, version: db.Version, topology: db.MySQL},
			&mysqlConfigTask{client: b.deps.Client, state: b.deps.State, topology: db.MySQL}, nil
	case types.DatabasePicodata:
		return &picoInstallTask{client: b.deps.Client, state: b.deps.State, version: db.Version, topology: db.Picodata},
			&picoConfigTask{client: b.deps.Client, state: b.deps.State, topology: db.Picodata}, nil
	default:
		return nil, nil, fmt.Errorf("unsupported database kind %q", db.Kind)
	}
}
