package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/stroppy-io/stroppy-cloud/internal/core/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres"
)

// App is the top-level application facade.
// It wires DAG, executor, storage, and agent client together.
type App struct {
	storage    dag.Storage
	logger     *zap.Logger
	client     agent.Client
	sink       dag.LogSink
	listenAddr string // server listen address (e.g. ":8080"), set by Server
	// settingsFunc returns the current server settings snapshot for a tenant.
	// Set by Server after construction.
	settingsFunc func(tenantID string) *types.ServerSettings
	// monitoringURL is the vmauth base URL for metrics/logs.
	// Set by Server after construction.
	monitoringURL   string
	monitoringToken string
	// accountIDFunc resolves tenantID → victoria accountID.
	// Set by Server after construction.
	accountIDFunc func(tenantID string) int32
	// jwtIssuer generates JWT tokens for agent auth.
	// Set by Server after construction.
	jwtIssuer *auth.JWTIssuer
}

// Config holds application-level settings.
type Config struct {
	Pool   *pgxpool.Pool
	Logger *zap.Logger
}

// New creates a new App backed by PostgreSQL storage.
func New(cfg Config) *App {
	if cfg.Logger == nil {
		cfg.Logger = zap.NewNop()
	}

	return &App{
		storage: postgres.NewRunStorage(cfg.Pool),
		logger:  cfg.Logger,
	}
}

// Start builds a DAG from RunConfig and executes it.
func (a *App) Start(ctx context.Context, tenantID string, cfg types.RunConfig) error {
	if err := run.ValidateConfig(cfg); err != nil {
		return fmt.Errorf("api: validate: %w", err)
	}
	run.FillMachinesFromTopology(&cfg)

	deps, cleanup, err := a.buildDeps(tenantID, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	graph, _, err := run.Build(cfg, deps)
	if err != nil {
		return fmt.Errorf("api: build graph: %w", err)
	}

	exec := dag.NewExecutor(tenantID, cfg.ID, graph, a.storage, a.logger, a.sink)

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
	if err := run.ValidateConfig(cfg); err != nil {
		return err
	}
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
func (a *App) RecoverRun(ctx context.Context, tenantID string, snap *dag.Snapshot) error {
	if snap.State == nil {
		return fmt.Errorf("api: snapshot has no run state for recovery")
	}

	var cfg types.RunConfig
	if err := json.Unmarshal(snap.State.RunConfig, &cfg); err != nil {
		return fmt.Errorf("api: unmarshal run config: %w", err)
	}

	run.FillMachinesFromTopology(&cfg)

	deps, cleanup, err := a.buildDeps(tenantID, cfg)
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

	exec := dag.NewExecutor(tenantID, cfg.ID, restoredGraph, a.storage, a.logger, a.sink)

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

func (a *App) buildDeps(tenantID string, cfg types.RunConfig) (run.Deps, func(), error) {
	state := run.NewState()
	cl := a.client
	if cl == nil {
		// Fallback for CLI mode (no server) — direct HTTP push.
		cl = agent.NewHTTPClient()
	}
	deps := run.Deps{Client: cl, State: state, MonitoringURL: a.monitoringURL, MonitoringToken: a.monitoringToken, TenantID: tenantID, JWTIssuer: a.jwtIssuer}
	noop := func() {}

	// Resolve per-tenant victoria accountID.
	if a.accountIDFunc != nil && tenantID != "" {
		deps.AccountID = a.accountIDFunc(tenantID)
	}

	// Attach current settings snapshot if available.
	if a.settingsFunc != nil {
		deps.Settings = a.settingsFunc(tenantID)
	}

	// Server address for agent→server communication.
	if deps.Settings != nil {
		deps.ServerAddr = deps.Settings.Cloud.ServerAddr
	}

	if cfg.Provider == types.ProviderDocker {
		// Docker agents run on the same host — auto-detect addresses
		// so it works without manual settings configuration.
		if deps.ServerAddr == "" {
			deps.ServerAddr = dockerHostAddr(a.listenAddr)
		}

		// Rewrite monitoringURL for Docker containers (localhost -> host gateway).
		if a.monitoringURL != "" {
			dockerHost := dockerHostOnly()
			deps.MonitoringURL = rewriteLocalhost(a.monitoringURL, dockerHost)
		}

		deployer, err := agent.NewDockerDeployer(fmt.Sprintf("stroppy-%s", cfg.ID))
		if err != nil {
			return deps, noop, fmt.Errorf("api: docker deployer: %w", err)
		}
		deps.Deployer = deployer
		return deps, func() { deployer.Close() }, nil
	}

	// Yandex Cloud: agents run on remote VMs — rewrite localhost in monitoringURL
	// to the server's public address so vmagent on VMs can reach vmauth.
	if cfg.Provider == types.ProviderYandex && a.monitoringURL != "" && deps.ServerAddr != "" {
		// Extract host from ServerAddr (e.g. "http://51.250.1.2:8080" → "51.250.1.2").
		serverHost := deps.ServerAddr
		for _, prefix := range []string{"https://", "http://"} {
			serverHost = strings.TrimPrefix(serverHost, prefix)
		}
		if idx := strings.IndexByte(serverHost, ':'); idx > 0 {
			serverHost = serverHost[:idx]
		}
		deps.MonitoringURL = rewriteLocalhost(a.monitoringURL, serverHost)
	}

	return deps, noop, nil
}

// dockerHostAddr builds an HTTP address that Docker containers can use to reach the server.
// On Linux containers use the Docker bridge gateway (172.17.0.1).
// On macOS/Windows they use host.docker.internal.
func dockerHostAddr(listenAddr string) string {
	_, port, _ := net.SplitHostPort(listenAddr)
	if port == "" {
		port = "8080"
	}

	host := "host.docker.internal"
	if runtime.GOOS == "linux" {
		host = "172.17.0.1"
	}

	return fmt.Sprintf("http://%s:%s", host, port)
}

// dockerHostOnly returns the Docker host gateway without port.
func dockerHostOnly() string {
	if runtime.GOOS == "linux" {
		return "172.17.0.1"
	}
	return "host.docker.internal"
}

// rewriteLocalhost replaces localhost/127.0.0.1 in a URL with the given host.
func rewriteLocalhost(rawURL, host string) string {
	for _, local := range []string{"localhost", "127.0.0.1"} {
		rawURL = strings.ReplaceAll(rawURL, local, host)
	}
	return rawURL
}
