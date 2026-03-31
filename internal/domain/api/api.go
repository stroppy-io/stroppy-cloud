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
	return exec.Run(ctx)
}

// Resume restores a previously interrupted run and continues from where it stopped.
func (a *App) Resume(ctx context.Context, runID string, cfg types.RunConfig) error {
	deps, cleanup, err := a.buildDeps(cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	_, reg, err := run.Build(cfg, deps)
	if err != nil {
		return fmt.Errorf("api: build registry: %w", err)
	}

	graph, err := a.loadGraph(ctx, runID, reg)
	if err != nil {
		return err
	}

	exec := dag.NewExecutor(runID, graph, a.storage, a.logger, a.sink)
	return exec.Resume(ctx, reg)
}

// Validate checks that a RunConfig produces a valid DAG without executing it.
func (a *App) Validate(cfg types.RunConfig) error {
	state := run.NewState()
	deps := run.Deps{Client: a.client, State: state}
	_, _, err := run.Build(cfg, deps)
	return err
}

// DryRun builds the DAG and returns its structure as JSON.
func (a *App) DryRun(cfg types.RunConfig) ([]byte, error) {
	state := run.NewState()
	deps := run.Deps{Client: a.client, State: state}
	graph, _, err := run.Build(cfg, deps)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(graph, "", "  ")
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
		deployer, err := agent.NewDockerDeployer("stroppy-run-net")
		if err != nil {
			return deps, noop, fmt.Errorf("api: docker deployer: %w", err)
		}
		deps.Deployer = deployer
		deps.Client = agent.NewHTTPClient()
		deps.ServerAddr = "http://host.docker.internal:8080" // server from inside containers
		return deps, func() { deployer.Close() }, nil
	}

	// For cloud providers, use settings-based server address and HTTP client.
	if deps.Settings != nil && deps.Settings.Cloud.ServerAddr != "" {
		deps.ServerAddr = deps.Settings.Cloud.ServerAddr
		deps.Client = agent.NewHTTPClient()
	}

	return deps, noop, nil
}

func (a *App) loadGraph(ctx context.Context, runID string, reg *dag.Registry) (*dag.Graph, error) {
	snap, err := a.storage.Load(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("api: load snapshot: %w", err)
	}
	if snap == nil {
		return nil, fmt.Errorf("api: no saved state for run %q", runID)
	}
	graph, err := reg.Unmarshal(snap.GraphJSON)
	if err != nil {
		return nil, fmt.Errorf("api: restore graph: %w", err)
	}
	return graph, nil
}
