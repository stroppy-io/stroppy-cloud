package api

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	badgerdb "github.com/dgraph-io/badger/v4"
	"go.uber.org/zap"

	"github.com/stroppy-io/stroppy-cloud/internal/core/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
	badgerstorage "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/badger"
)

// App is the top-level application facade.
// It wires DAG, executor, storage, and agent client together.
type App struct {
	db      *badgerdb.DB
	storage dag.Storage
	logger  *zap.Logger
	client  agent.Client
	sink    dag.LogSink
	// settingsFunc returns the current server settings snapshot.
	// Set by Server after construction.
	settingsFunc func() *types.ServerSettings
}

// Config holds application-level settings.
type Config struct {
	DataDir string // badger data directory; empty = in-memory
	Logger  *zap.Logger
	Client  agent.Client
	Sink    dag.LogSink
}

// New creates a new App. Call Close() when done.
func New(cfg Config) (*App, error) {
	if cfg.Logger == nil {
		cfg.Logger = zap.NewNop()
	}

	opts := badgerdb.DefaultOptions(cfg.DataDir).WithLogger(nil)
	if cfg.DataDir == "" {
		opts = opts.WithInMemory(true)
	}
	db, err := badgerdb.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("api: open badger: %w", err)
	}

	return &App{
		db:      db,
		storage: badgerstorage.NewDAGStorage(db),
		logger:  cfg.Logger,
		client:  cfg.Client,
		sink:    cfg.Sink,
	}, nil
}

// Start builds a DAG from RunConfig and executes it.
func (a *App) Start(ctx context.Context, cfg types.RunConfig) error {
	run.FillMachinesFromTopology(&cfg)

	deps, cleanup, err := a.buildDeps(cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	graph, _, err := run.Build(cfg, deps)
	if err != nil {
		return fmt.Errorf("api: build graph: %w", err)
	}

	exec := dag.NewExecutor(cfg.ID, graph, a.storage, a.logger, a.sink)

	// Wire state exporter so snapshots include recoverable run state.
	cfgJSON, _ := json.Marshal(cfg)
	exec.SetStateExporter(func() *dag.RunState {
		rs := deps.State.ExportRunState()
		rs.Provider = string(cfg.Provider)
		rs.RunConfig = cfgJSON
		return rs
	})

	return exec.Run(ctx)
}

// Validate checks that a RunConfig produces a valid DAG without executing it.
func (a *App) Validate(cfg types.RunConfig) error {
	run.FillMachinesFromTopology(&cfg)
	state := run.NewState()
	deps := run.Deps{Client: a.client, State: state}
	_, _, err := run.Build(cfg, deps)
	return err
}

// DryRun builds the DAG and returns its structure as JSON.
func (a *App) DryRun(cfg types.RunConfig) ([]byte, error) {
	run.FillMachinesFromTopology(&cfg)
	state := run.NewState()
	deps := run.Deps{Client: a.client, State: state}
	graph, _, err := run.Build(cfg, deps)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(graph, "", "  ")
}

// RecoverRun resumes execution of an incomplete run from a saved snapshot.
// It rebuilds deps, restores state from the snapshot, and re-executes
// only the nodes that were not yet completed.
func (a *App) RecoverRun(ctx context.Context, snap *dag.Snapshot) error {
	if snap.State == nil {
		return fmt.Errorf("api: snapshot has no run state for recovery")
	}

	var cfg types.RunConfig
	if err := json.Unmarshal(snap.State.RunConfig, &cfg); err != nil {
		return fmt.Errorf("api: unmarshal run config: %w", err)
	}

	run.FillMachinesFromTopology(&cfg)

	deps, cleanup, err := a.buildDeps(cfg)
	if err != nil {
		return fmt.Errorf("api: build deps for recovery: %w", err)
	}
	defer cleanup()

	// Restore run state (targets, container IDs, etc.) from snapshot.
	deps.State.ImportRunState(snap.State)

	// Build a fresh graph with tasks wired to the restored deps/state.
	graph, reg, err := run.Build(cfg, deps)
	if err != nil {
		return fmt.Errorf("api: build graph for recovery: %w", err)
	}

	// Try to restore the graph from snapshot (preserves structure),
	// falling back to the freshly built graph.
	restoredGraph, unmarshalErr := reg.Unmarshal(snap.GraphJSON)
	if unmarshalErr != nil {
		restoredGraph = graph
	}

	exec := dag.NewExecutor(cfg.ID, restoredGraph, a.storage, a.logger, a.sink)

	// Mark previously completed nodes so they are skipped.
	for _, ns := range snap.Nodes {
		if ns.Status == dag.StatusDone {
			exec.MarkNodeDone(ns.ID)
		}
	}

	// Wire state exporter for continued snapshots.
	cfgJSON, _ := json.Marshal(cfg)
	exec.SetStateExporter(func() *dag.RunState {
		rs := deps.State.ExportRunState()
		rs.Provider = string(cfg.Provider)
		rs.RunConfig = cfgJSON
		return rs
	})

	return exec.Run(ctx)
}

// Storage returns the DAG storage (used by server for status queries).
func (a *App) Storage() dag.Storage {
	return a.storage
}

// LoadConfig reads a RunConfig from a JSON file.
func LoadConfig(path string) (types.RunConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return types.RunConfig{}, fmt.Errorf("api: read config: %w", err)
	}
	var cfg types.RunConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return types.RunConfig{}, fmt.Errorf("api: parse config: %w", err)
	}
	return cfg, nil
}

// Close releases resources.
func (a *App) Close() error {
	return a.db.Close()
}

func (a *App) buildDeps(cfg types.RunConfig) (run.Deps, func(), error) {
	state := run.NewState()
	cl := a.client
	if cl == nil {
		cl = agent.NewHTTPClient()
	}
	deps := run.Deps{Client: cl, State: state}
	noop := func() {}

	// Attach current settings snapshot if available.
	if a.settingsFunc != nil {
		deps.Settings = a.settingsFunc()
	}

	if cfg.Provider == types.ProviderDocker {
		deployer, err := agent.NewDockerDeployer(fmt.Sprintf("stroppy-%s", cfg.ID))
		if err != nil {
			return deps, noop, fmt.Errorf("api: docker deployer: %w", err)
		}
		deps.Deployer = deployer
		deps.Client = agent.NewHTTPClient()
		// Server address for agent registration callbacks (best-effort).
		// Agent→server communication is non-critical; server→agent is what matters.
		serverAddr := os.Getenv("STROPPY_SERVER_ADDR")
		if serverAddr == "" {
			serverAddr = "http://172.17.0.1:8080" // docker0 bridge — host reachable from containers
		}
		deps.ServerAddr = serverAddr
		return deps, func() { deployer.Close() }, nil
	}

	// For cloud providers, use settings-based server address and HTTP client.
	if deps.Settings != nil && deps.Settings.Cloud.ServerAddr != "" {
		deps.ServerAddr = deps.Settings.Cloud.ServerAddr
		deps.Client = agent.NewHTTPClient()
	}

	return deps, noop, nil
}
