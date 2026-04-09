package run

import (
	"fmt"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/core/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

// Deps holds external dependencies injected into DAG tasks.
type Deps struct {
	Client   agent.Client
	Deployer *agent.DockerDeployer
	State    *State
	// ServerAddr is the server callback address for agents.
	ServerAddr string
	// Settings provides cloud configuration (Yandex credentials, binary URL, etc.).
	Settings *types.ServerSettings
	// MonitoringURL is the vmauth base URL for metrics/logs ingestion.
	MonitoringURL string
	// MonitoringToken is the bearer token for vmauth.
	MonitoringToken string
	// AccountID is the per-tenant VictoriaMetrics account ID for data isolation.
	AccountID int32
	// JWTIssuer issues tokens for agent authentication.
	JWTIssuer *auth.JWTIssuer
	// TenantID is the tenant running this job.
	TenantID string
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
	// g.Add only fails on duplicate IDs; phases are unique constants, so error is impossible.
	_ = b.g.Add(&dag.Node{ID: phase, Type: phase, Deps: deps, Task: task})
	b.reg.Register(phase, func() dag.Task { return task })
}

func (b *builder) addMustComplete(phase string, deps []string, task dag.Task) {
	_ = b.g.Add(&dag.Node{ID: phase, Type: phase, Deps: deps, Task: task, MustComplete: true})
	b.reg.Register(phase, func() dag.Task { return task })
}

func (b *builder) addAlwaysRun(phase string, deps []string, task dag.Task) {
	_ = b.g.Add(&dag.Node{ID: phase, Type: phase, Deps: deps, Task: task, AlwaysRun: true})
	b.reg.Register(phase, func() dag.Task { return task })
}

func (b *builder) build() error {
	afterMachines := []string{b.ph(types.PhaseMachines)}

	// --- infrastructure (MustComplete — must finish before teardown on cancel) ---
	b.addMustComplete(b.ph(types.PhaseNetwork), nil,
		&networkTask{cfg: b.cfg.Network, provider: b.cfg.Provider, deployer: b.deps.Deployer, state: b.deps.State, runID: b.cfg.ID})

	b.addMustComplete(b.ph(types.PhaseMachines), []string{b.ph(types.PhaseNetwork)},
		&machinesTask{runCfg: b.cfg, state: b.deps.State, deployer: b.deps.Deployer, serverAddr: b.deps.ServerAddr, settings: b.deps.Settings, jwtIssuer: b.deps.JWTIssuer, tenantID: b.deps.TenantID})

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

	// --- Patroni (if Postgres HA with Patroni) ---
	if b.needsPatroni() {
		// When Patroni is enabled, it manages postgresql.conf and cluster bootstrap.
		// Patroni REPLACES the PG configure phase entirely.
		b.add(b.ph(types.PhaseInstallPatroni), afterMachines,
			&patroniInstallTask{client: b.deps.Client, state: b.deps.State})
		patroniConfigDeps := append(append([]string{}, configDBDeps...), b.ph(types.PhaseInstallPatroni))
		b.add(b.ph(types.PhaseConfigurePatroni), patroniConfigDeps,
			&patroniConfigTask{
				client:   b.deps.Client,
				state:    b.deps.State,
				version:  b.cfg.Database.Version,
				topology: b.cfg.Database.Postgres,
			})
		// PhaseConfigureDB becomes a no-op that depends on Patroni configure.
		b.add(b.ph(types.PhaseConfigureDB), []string{b.ph(types.PhaseConfigurePatroni)}, &noopTask{})
	} else {
		b.add(b.ph(types.PhaseConfigureDB), configDBDeps, configDB)
	}

	// --- monitoring ---
	// Install exporters on ALL machines (node_exporter everywhere, DB exporter on DB nodes, vmagent on monitor).
	b.add(b.ph(types.PhaseInstallMonitor), afterMachines,
		&monitorInstallTask{client: b.deps.Client, state: b.deps.State, dbKind: b.cfg.Database.Kind})
	// Configure/start daemons after install AND after DB is configured (so postgres_exporter can connect).
	monitorConfigDeps := []string{b.ph(types.PhaseInstallMonitor), b.ph(types.PhaseConfigureDB)}
	if b.needsYDBInit() {
		monitorConfigDeps = append(monitorConfigDeps, b.ph(types.PhaseStartYDBDatabase))
	}
	b.add(b.ph(types.PhaseConfigureMonitor), monitorConfigDeps,
		&monitorConfigTask{client: b.deps.Client, state: b.deps.State, monitor: b.cfg.Monitor, runID: b.cfg.ID, dbKind: b.cfg.Database.Kind, monitoringURL: b.deps.MonitoringURL, monitoringToken: b.deps.MonitoringToken, accountID: b.deps.AccountID})

	// --- pgbouncer (if Postgres HA with pgbouncer, colocated on DB nodes) ---
	if b.needsPgBouncer() {
		b.addPgBouncer(afterMachines)
	}

	// --- proxy (HAProxy for PG/Picodata, ProxySQL for MySQL) ---
	if b.needsProxy() {
		b.addProxy(afterMachines)
	}

	// --- YDB cluster init + database start ---
	if b.needsYDBInit() {
		b.addYDBPhases()
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
	stroppySettings := types.DefaultStroppySettings()
	// Metric prefix = runID so each run's metrics are namespaced (e.g. run_xxx_vus, run_xxx_iterations).
	// Replace dashes with underscores — PromQL metric names don't support dashes.
	stroppySettings.OTLPMetricPrefix = strings.ReplaceAll(b.cfg.ID, "-", "_") + "_"
	if b.deps.MonitoringURL != "" {
		stroppySettings.SetFromMonitoringURL(b.deps.MonitoringURL, b.deps.MonitoringToken, b.deps.AccountID)
	}
	b.add(b.ph(types.PhaseRunStroppy), b.runStroppyDeps,
		&stroppyRunTask{
			client:          b.deps.Client,
			state:           b.deps.State,
			stroppy:         b.cfg.Stroppy,
			stroppySettings: stroppySettings,
			dbKind:          b.cfg.Database.Kind,
			runID:           b.cfg.ID,
			monitoringURL:   b.deps.MonitoringURL,
			monitoringToken: b.deps.MonitoringToken,
			accountID:       b.deps.AccountID,
		})

	// --- teardown (always runs, even if upstream fails) ---
	b.addAlwaysRun(b.ph(types.PhaseTeardown), []string{b.ph(types.PhaseRunStroppy)},
		&teardownTask{provider: b.cfg.Provider, state: b.deps.State, deployer: b.deps.Deployer})

	if err := b.g.Validate(); err != nil {
		return fmt.Errorf("run: invalid graph: %w", err)
	}
	return nil
}

// --- conditional phase helpers ---

func (b *builder) needsEtcd() bool {
	if b.cfg.Database.Kind == types.DatabasePostgres && b.cfg.Database.Postgres != nil {
		return b.cfg.Database.Postgres.Etcd
	}
	return false
}

func (b *builder) needsPatroni() bool {
	if b.cfg.Database.Kind == types.DatabasePostgres && b.cfg.Database.Postgres != nil {
		return b.cfg.Database.Postgres.Patroni
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
	case types.DatabaseYDB:
		return db.YDB != nil && db.YDB.HAProxy != nil
	}
	return false
}

func (b *builder) needsYDBInit() bool {
	return b.cfg.Database.Kind == types.DatabaseYDB && b.cfg.Database.YDB != nil
}

func (b *builder) addYDBPhases() {
	b.add(b.ph(types.PhaseInitYDBCluster), []string{b.ph(types.PhaseConfigureDB)},
		&ydbInitTask{client: b.deps.Client, state: b.deps.State, topology: b.cfg.Database.YDB})
	b.add(b.ph(types.PhaseStartYDBDatabase), []string{b.ph(types.PhaseInitYDBCluster)},
		&ydbStartDBTask{client: b.deps.Client, state: b.deps.State, topology: b.cfg.Database.YDB})
	b.runStroppyDeps = append(b.runStroppyDeps, b.ph(types.PhaseStartYDBDatabase))
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
		&pgBouncerConfigTask{client: b.deps.Client, state: b.deps.State, topology: b.cfg.Database.Postgres})
	b.runStroppyDeps = append(b.runStroppyDeps, b.ph(types.PhaseConfigurePgBouncer))
}

func (b *builder) addProxy(afterMachines []string) {
	// Proxy depends on DB being configured (needs backends list).
	b.add(b.ph(types.PhaseInstallProxy), afterMachines,
		&proxyInstallTask{client: b.deps.Client, state: b.deps.State, dbKind: b.cfg.Database.Kind})
	b.add(b.ph(types.PhaseConfigureProxy), []string{b.ph(types.PhaseInstallProxy), b.ph(types.PhaseConfigureDB)},
		&proxyConfigTask{client: b.deps.Client, state: b.deps.State, dbKind: b.cfg.Database.Kind,
			pgTopology: b.cfg.Database.Postgres, mysqlTopology: b.cfg.Database.MySQL, picoTopology: b.cfg.Database.Picodata, ydbTopology: b.cfg.Database.YDB})
	b.runStroppyDeps = append(b.runStroppyDeps, b.ph(types.PhaseConfigureProxy))
}

// --- DB task factory ---

func (b *builder) dbTasks() (install dag.Task, config dag.Task, err error) {
	db := b.cfg.Database
	pkg := b.cfg.ResolvedPackage // set by runStart before building DAG
	switch db.Kind {
	case types.DatabasePostgres:
		return &pgInstallTask{client: b.deps.Client, state: b.deps.State, version: db.Version, topology: db.Postgres, pkg: pkg},
			&pgConfigTask{client: b.deps.Client, state: b.deps.State, version: db.Version, topology: db.Postgres}, nil
	case types.DatabaseMySQL:
		return &mysqlInstallTask{client: b.deps.Client, state: b.deps.State, version: db.Version, topology: db.MySQL, pkg: pkg},
			&mysqlConfigTask{client: b.deps.Client, state: b.deps.State, topology: db.MySQL}, nil
	case types.DatabasePicodata:
		return &picoInstallTask{client: b.deps.Client, state: b.deps.State, version: db.Version, topology: db.Picodata, pkg: pkg},
			&picoConfigTask{client: b.deps.Client, state: b.deps.State, topology: db.Picodata}, nil
	case types.DatabaseYDB:
		return &ydbInstallTask{client: b.deps.Client, state: b.deps.State, version: db.Version, topology: db.YDB, pkg: pkg},
			&ydbConfigTask{client: b.deps.Client, state: b.deps.State, topology: db.YDB}, nil
	default:
		return nil, nil, fmt.Errorf("unsupported database kind %q", db.Kind)
	}
}
