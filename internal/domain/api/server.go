package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/stroppy-io/stroppy-cloud/internal/core/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/metrics"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
	pgdb "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/generated"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/victoria"
)

// Server is the HTTP server exposing agent, external, and UI APIs.
type Server struct {
	app    *App
	logger *zap.Logger
	hub    *wsHub
	pool   *pgxpool.Pool

	jwtIssuer *auth.JWTIssuer

	// monitoringURL is the vmauth base URL (env MONITORING_URL).
	// Empty means monitoring is disabled.
	monitoringURL string
	// monitoringToken is the bearer token for vmauth (env MONITORING_TOKEN).
	monitoringToken string

	// grafanaURL is the Grafana base URL (env GRAFANA_URL).
	// Empty means Grafana integration is disabled.
	grafanaURL string

	// grafanaDashboards maps dashboard names to UIDs (hardcoded defaults).
	grafanaDashboards map[string]string

	// agentRegistry tracks connected agents by machine ID.
	agentsMu sync.RWMutex
	agents   map[string]agent.Target

	// runCancels tracks cancel functions for running DAG executions.
	runCancelsMu sync.Mutex
	runCancels   map[string]context.CancelFunc

	// pollClient is the command queue used for agent<->server communication.
	pollClient *agent.PollClient

	// spaFS serves the embedded SPA files. If nil, SPA is not served.
	spaFS http.FileSystem
}

// NewServer creates an HTTP server backed by the App.
// monitoringURL is the vmauth base URL (empty = monitoring disabled).
// grafanaURL is the Grafana base URL (empty = Grafana integration disabled).
func NewServer(app *App, logger *zap.Logger, pool *pgxpool.Pool, jwtSecret, monitoringURL, monitoringToken, grafanaURL, listenAddr string) *Server {
	pc := agent.NewPollClient(logger)
	s := &Server{
		app:             app,
		logger:          logger,
		hub:             newWSHub(),
		agents:          make(map[string]agent.Target),
		runCancels:      make(map[string]context.CancelFunc),
		pollClient:      pc,
		pool:            pool,
		jwtIssuer:       auth.NewJWTIssuer(jwtSecret),
		monitoringURL:   monitoringURL,
		monitoringToken: monitoringToken,
		grafanaURL:      grafanaURL,
		grafanaDashboards: map[string]string{
			"system":   "stroppy-system",
			"postgres": "stroppy-postgres",
			"mysql":    "stroppy-mysql",
			"picodata": "stroppy-picodata",
			"ydb":      "stroppy-ydb",
			"stroppy":  "stroppy-metrics-v1",
			"compare":  "stroppy-compare",
		},
	}
	// Wire LogSink so executor logs stream to WebSocket clients and VictoriaLogs.
	if monitoringURL != "" {
		s.hub.victoriaLogs = victoria.NewLogsClient(monitoringURL, monitoringToken)
	}
	s.hub.logger = logger
	s.hub.accountIDResolver = func(runID string) int32 {
		id, _ := s.accountIDFromRunID(context.Background(), runID)
		return id
	}
	app.sink = s.hub
	// Wire settings getter so buildDeps can access current cloud settings from DB.
	app.settingsFunc = s.settingsForTenant
	// Wire monitoring config so buildDeps can derive OTLP/metrics endpoints.
	app.monitoringURL = monitoringURL
	app.monitoringToken = monitoringToken
	// Wire accountID resolver so buildDeps can set per-tenant victoria accountID.
	app.accountIDFunc = func(tenantID string) int32 {
		id, _ := s.tenantAccountID(context.Background(), tenantID)
		return id
	}
	// Wire PollClient as the agent client — all command dispatch goes through polling.
	app.client = pc
	// Wire listen address so Docker agents can reach the server.
	app.listenAddr = listenAddr
	// Wire JWT issuer for agent token generation.
	app.jwtIssuer = s.jwtIssuer
	return s
}

// Router returns the chi router with all routes mounted.
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	r.Use(auth.NewAuthMiddleware(s.jwtIssuer, s.pool))

	// --- Health ---
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// --- Agent API ---
	r.Route("/api/agent", func(r chi.Router) {
		r.Post("/register", s.agentRegister)
		r.Post("/poll", s.agentPoll)
		r.Post("/report", s.agentReport)
		r.Post("/log", s.agentLog)
	})

	// --- Auth (public — handled by isPublicPath in middleware) ---
	r.Post("/api/v1/auth/login", s.login)
	r.Post("/api/v1/auth/refresh", s.refresh)
	r.Post("/api/v1/auth/logout", s.logout)

	// --- Authenticated, no tenant required ---
	r.Get("/api/v1/auth/me", s.authMe)
	r.Post("/api/v1/auth/select-tenant", s.selectTenant)
	r.Put("/api/v1/auth/password", s.changePassword)

	// --- Grafana config (infrastructure, not tenant) ---
	r.Get("/api/v1/grafana", s.getGrafanaConfig)

	// --- Tenant-scoped ---
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(auth.TenantRequired())

		// Viewer+
		r.Get("/packages", s.listPackages)
		r.Get("/packages/{id}", s.getPackage)
		r.Get("/packages/{id}/deb", s.downloadPackageDeb)
		r.Get("/runs", s.listRuns)
		r.Get("/run/{runID}/status", s.runStatus)
		r.Get("/run/{runID}/logs", s.runLogs)
		r.Get("/run/{runID}/metrics", s.runMetrics)
		r.Get("/compare", s.compareRuns)
		r.Get("/presets", s.listPresetsTenant)
		r.Get("/presets/{id}", s.getPreset)
		r.Get("/settings", s.getSettings)

		// Operator+
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRole("operator"))
			r.Post("/run", s.runStart)
			r.Post("/validate", s.runValidate)
			r.Post("/dry-run", s.runDryRun)
			r.Post("/probe", s.stroppyProbe)
			r.Delete("/run/{runID}", s.deleteRun)
			r.Post("/run/{runID}/cancel", s.cancelRun)
			r.Put("/baseline/{name}", s.setBaseline)
			r.Get("/baseline/{name}", s.getBaseline)
			r.Get("/baselines", s.listBaselines)
			r.Post("/upload/deb", s.uploadPackage)
			r.Post("/upload/rpm", s.uploadPackage)
			r.Post("/presets", s.createPreset)
			r.Put("/presets/{id}", s.updatePreset)
			r.Delete("/presets/{id}", s.deletePreset)
			r.Post("/presets/{id}/clone", s.clonePreset)
			r.Post("/packages", s.createPackage)
			r.Put("/packages/{id}", s.updatePackage)
			r.Delete("/packages/{id}", s.deletePackage)
			r.Post("/packages/{id}/clone", s.clonePackage)
			r.Post("/packages/{id}/deb", s.uploadPackageDeb)
		})

		// Owner+
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRole("owner"))
			r.Put("/settings", s.updateSettings)
			r.Route("/tenant", func(r chi.Router) {
				r.Get("/members", s.listMembers)
				r.Post("/members", s.addMember)
				r.Put("/members/{userID}", s.updateMember)
				r.Delete("/members/{userID}", s.removeMember)
				r.Get("/tokens", s.listAPITokens)
				r.Post("/tokens", s.createAPIToken)
				r.Delete("/tokens/{id}", s.revokeAPIToken)
			})
		})
	})

	// --- Root only ---
	r.Route("/api/v1/admin", func(r chi.Router) {
		r.Use(auth.RequireRoot())
		r.Get("/tenants", s.listTenantsAdmin)
		r.Post("/tenants", s.createTenantAdmin)
		r.Delete("/tenants/{id}", s.deleteTenantAdmin)
		r.Get("/users", s.listUsersAdmin)
		r.Post("/users", s.createUserAdmin)
		r.Delete("/users/{id}", s.deleteUserAdmin)
		r.Put("/users/{id}/password", s.resetPasswordAdmin)
	})

	// --- UI WebSocket ---
	r.Get("/ws/logs", s.wsLogs)
	r.Get("/ws/logs/{runID}", s.wsLogsRun)

	// --- Package serving ---
	r.Get("/packages/*", http.StripPrefix("/packages/", http.FileServer(http.Dir(uploadDir))).ServeHTTP)

	// --- Agent binary download (public — agent has no token yet at download time) ---
	r.Get("/agent/binary", s.serveBinary)

	// --- SPA (embedded frontend) ---
	if s.spaFS != nil {
		r.Get("/*", s.serveSPA)
		// Unknown paths: SPA fallback for non-API, JSON 404 for API.
		r.NotFound(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
				return
			}
			s.serveSPA(w, r)
		})
	}

	return r
}

// SetSPA configures the embedded SPA filesystem.
// Call with the sub-directory containing index.html (e.g. web.Dist "dist" subdir).
func (s *Server) SetSPA(fsys fs.FS) {
	s.spaFS = http.FS(fsys)
}

func (s *Server) serveSPA(w http.ResponseWriter, r *http.Request) {
	// Try to serve the exact file first.
	path := r.URL.Path
	if path == "/" {
		path = "/index.html"
	}
	f, err := s.spaFS.Open(path[1:]) // strip leading /
	if err == nil {
		f.Close()
		http.FileServer(s.spaFS).ServeHTTP(w, r)
		return
	}
	// File not found -> serve index.html for client-side routing.
	r.URL.Path = "/"
	http.FileServer(s.spaFS).ServeHTTP(w, r)
}

// ============================================================
// Agent API handlers
// ============================================================

// RegisterRequest is the payload sent by an agent on startup.
type RegisterRequest struct {
	MachineID string `json:"machine_id"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
}

func (s *Server) agentRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MachineID string `json:"machine_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Pre-create the healthy channel so PollClient can detect this agent.
	s.pollClient.MarkAgentReady(req.MachineID)

	s.agentsMu.Lock()
	s.agents[req.MachineID] = agent.Target{ID: req.MachineID}
	s.agentsMu.Unlock()

	s.logger.Info("agent registered", zap.String("machine_id", req.MachineID))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) agentPoll(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MachineID string `json:"machine_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Long-poll: block up to 60s waiting for a command.
	cmd := s.pollClient.Poll(req.MachineID, 60*time.Second)
	if cmd == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	writeJSON(w, http.StatusOK, cmd)
}

func (s *Server) agentReport(w http.ResponseWriter, r *http.Request) {
	var report agent.Report
	if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.logger.Info("agent report",
		zap.String("command_id", report.CommandID),
		zap.String("status", string(report.Status)),
	)

	// Route report to the waiting PollClient.Send() call.
	s.pollClient.DeliverReport(report)

	// Broadcast to WS clients.
	s.hub.broadcast(wsMessage{Type: "report", Payload: report})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) agentLog(w http.ResponseWriter, r *http.Request) {
	var line agent.LogLine
	if err := json.NewDecoder(r.Body).Decode(&line); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Extract run ID from machine ID (format: {runID}-{role}-{index}).
	runID := extractRunID(line.MachineID)

	// Broadcast to all connected WebSocket clients for live viewing.
	s.hub.broadcast(wsMessage{Type: "agent_log", RunID: runID, Payload: line})

	// Persist to VictoriaLogs via vmauth (fire-and-forget).
	if s.monitoringURL != "" {
		// Look up tenant accountID from run → tenant_id (agent endpoints have no auth context).
		accountID, accErr := s.accountIDFromRunID(r.Context(), runID)
		if accErr != nil {
			s.logger.Warn("vlogs: cannot resolve accountID for run", zap.String("run_id", runID), zap.Error(accErr))
		}
		go func() {
			vlClient := victoria.NewLogsClient(s.monitoringURL, s.monitoringToken)
			if err := vlClient.IngestWithAccount(accountID, line.MachineID, line.CommandID, line.Action, runID, line.Stream, line.Line); err != nil {
				s.logger.Debug("vlogs ingest failed", zap.Error(err))
			}
		}()
	}

	w.WriteHeader(http.StatusNoContent)
}

// ============================================================
// External API handlers
// ============================================================

func (s *Server) runStart(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())

	var cfg types.RunConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Auto-generate ID if empty.
	if cfg.ID == "" {
		cfg.ID = fmt.Sprintf("run-%d", time.Now().UnixMilli())
	}

	// Resolve preset topology if preset_id is provided and no explicit topology set.
	if err := s.resolveRunPreset(r.Context(), tenantID, &cfg); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Resolve package (from package_id or default built-in).
	if err := s.resolveRunPackage(r.Context(), tenantID, &cfg); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Check for duplicate (run already exists in storage).
	if snap, _ := s.app.Storage().Load(r.Context(), tenantID, cfg.ID); snap != nil {
		writeJSON(w, http.StatusConflict, map[string]string{
			"error": fmt.Sprintf("run %q already exists", cfg.ID),
		})
		return
	}

	// Run asynchronously with cancellable context.
	ctx, cancel := context.WithCancel(context.Background())
	s.runCancelsMu.Lock()
	s.runCancels[cfg.ID] = cancel
	s.runCancelsMu.Unlock()

	go func() {
		defer func() {
			s.runCancelsMu.Lock()
			delete(s.runCancels, cfg.ID)
			s.runCancelsMu.Unlock()
			cancel()
		}()
		if err := s.app.Start(ctx, tenantID, cfg); err != nil {
			s.logger.Error("run failed", zap.String("run_id", cfg.ID), zap.Error(err))
		}
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{"run_id": cfg.ID, "status": "started"})
}

func (s *Server) runValidate(w http.ResponseWriter, r *http.Request) {
	var cfg types.RunConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.app.Validate(cfg); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "valid"})
}

func (s *Server) runDryRun(w http.ResponseWriter, r *http.Request) {
	var cfg types.RunConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	data, err := s.app.DryRun(cfg)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func (s *Server) runStatus(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	runID := chi.URLParam(r, "runID")

	snap, err := s.app.storage.Load(r.Context(), tenantID, runID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if snap == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

func (s *Server) listRuns(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())

	runs, err := s.app.Storage().List(r.Context(), tenantID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, runs)
}

func (s *Server) cancelRun(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "runID")

	s.runCancelsMu.Lock()
	cancel, ok := s.runCancels[runID]
	s.runCancelsMu.Unlock()

	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "run not active or already finished"})
		return
	}

	s.logger.Info("cancelling run", zap.String("run_id", runID))
	cancel()

	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelling", "run_id": runID})
}

func (s *Server) deleteRun(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	runID := chi.URLParam(r, "runID")

	// Check exists.
	snap, err := s.app.Storage().Load(r.Context(), tenantID, runID)
	if err != nil || snap == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	// Clean up Docker resources (best-effort).
	s.cleanupRunResources(runID)

	// Delete from storage.
	if err := s.app.Storage().Delete(r.Context(), tenantID, runID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "run_id": runID})
}

// cleanupRunResources removes Docker containers and networks associated with a run.
func (s *Server) cleanupRunResources(runID string) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		s.logger.Warn("cleanup: docker client failed", zap.Error(err))
		return
	}
	defer cli.Close()

	ctx := context.Background()

	// Remove containers matching this run (name pattern: stroppy-agent-{runID}-*)
	containers, _ := cli.ContainerList(ctx, container.ListOptions{All: true})
	prefix := fmt.Sprintf("stroppy-agent-%s-", runID)
	for _, c := range containers {
		for _, name := range c.Names {
			cleanName := strings.TrimPrefix(name, "/")
			if strings.HasPrefix(cleanName, prefix) {
				s.logger.Info("cleanup: removing container", zap.String("name", cleanName))
				cli.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true})
			}
		}
	}

	// Remove network for this run (name pattern: stroppy-{runID})
	netName := fmt.Sprintf("stroppy-%s", runID)
	networks, _ := cli.NetworkList(ctx, network.ListOptions{})
	for _, n := range networks {
		if n.Name == netName {
			s.logger.Info("cleanup: removing network", zap.String("name", n.Name))
			cli.NetworkRemove(ctx, n.ID)
		}
	}
}

// RecoverOrCleanupRuns attempts to resume incomplete runs whose Docker containers
// are still alive. Runs that cannot be recovered are marked as failed and their
// resources are cleaned up. Called on server startup.
//
// For recovery, we query all runs across all tenants.
func (s *Server) RecoverOrCleanupRuns() {
	ctx := context.Background()

	// Query all runs across all tenants for recovery.
	rows, err := s.pool.Query(ctx, "SELECT id, tenant_id, snapshot FROM runs ORDER BY created_at DESC")
	if err != nil {
		s.logger.Warn("recovery: failed to list runs", zap.Error(err))
		return
	}
	defer rows.Close()

	type runInfo struct {
		id       string
		tenantID string
		snap     *dag.Snapshot
	}

	var incompleteRuns []runInfo
	for rows.Next() {
		var id, tenantID, data string
		if err := rows.Scan(&id, &tenantID, &data); err != nil {
			continue
		}
		var snap dag.Snapshot
		if json.Unmarshal([]byte(data), &snap) != nil {
			continue
		}
		// Check if any nodes are pending.
		pending := false
		for _, n := range snap.Nodes {
			if n.Status == dag.StatusPending {
				pending = true
				break
			}
		}
		if pending {
			incompleteRuns = append(incompleteRuns, runInfo{id: id, tenantID: tenantID, snap: &snap})
		}
	}

	// Track run IDs that are being recovered so we don't clean up their containers.
	activeRunIDs := make(map[string]bool)

	for _, r := range incompleteRuns {
		s.logger.Info("recovery: found incomplete run", zap.String("id", r.id), zap.String("tenant", r.tenantID))

		if r.snap.State == nil {
			s.logger.Warn("recovery: no state saved, marking as failed", zap.String("id", r.id))
			s.markRunFailed(ctx, r.tenantID, r.snap, r.id)
			s.cleanupRunResources(r.id)
			continue
		}

		if s.canRecoverRun(r.snap.State) {
			s.logger.Info("recovery: containers alive, resuming run", zap.String("id", r.id))
			activeRunIDs[r.id] = true
			go s.recoverRun(r.id, r.tenantID, r.snap)
		} else {
			s.logger.Warn("recovery: containers dead, marking as failed", zap.String("id", r.id))
			s.markRunFailed(ctx, r.tenantID, r.snap, r.id)
			s.cleanupRunResources(r.id)
		}
	}

	// Clean up orphaned containers that don't belong to any active/recovering run.
	s.cleanupOrphanedContainers(activeRunIDs)
}

// CleanupOrphanedRuns is kept for backward compatibility; it delegates to RecoverOrCleanupRuns.
func (s *Server) CleanupOrphanedRuns() {
	s.RecoverOrCleanupRuns()
}

// canRecoverRun checks whether the Docker containers from a previous run are still running.
func (s *Server) canRecoverRun(state *dag.RunState) bool {
	if state == nil || len(state.ContainerIDs) == 0 {
		return false
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return false
	}
	defer cli.Close()

	for _, cid := range state.ContainerIDs {
		inspect, err := cli.ContainerInspect(context.Background(), cid)
		if err == nil && inspect.State.Running {
			return true // at least one container alive -- worth trying
		}
	}

	return false
}

// recoverRun rebuilds state from a snapshot and resumes execution.
func (s *Server) recoverRun(runID, tenantID string, snap *dag.Snapshot) {
	ctx := context.Background()

	if err := s.app.RecoverRun(ctx, tenantID, snap); err != nil {
		s.logger.Error("recovery: run failed",
			zap.String("id", runID), zap.Error(err))
	}
}

// markRunFailed marks all pending nodes in a snapshot as failed.
func (s *Server) markRunFailed(ctx context.Context, tenantID string, snap *dag.Snapshot, runID string) {
	if snap == nil {
		return
	}
	changed := false
	for i := range snap.Nodes {
		if snap.Nodes[i].Status == dag.StatusPending {
			snap.Nodes[i].Status = dag.StatusFailed
			snap.Nodes[i].Error = "server restarted -- run orphaned"
			changed = true
		}
	}
	if changed {
		_ = s.app.Storage().Save(ctx, tenantID, runID, snap)
	}
}

// cleanupOrphanedContainers removes stroppy-agent containers and networks
// that don't belong to any actively recovering run.
func (s *Server) cleanupOrphanedContainers(activeRunIDs map[string]bool) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		s.logger.Warn("orphan cleanup: docker client failed", zap.Error(err))
		return
	}
	defer cli.Close()

	ctx := context.Background()

	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return
	}

	removed := 0
	for _, c := range containers {
		for _, name := range c.Names {
			cleanName := strings.TrimPrefix(name, "/")
			if !strings.HasPrefix(cleanName, "stroppy-agent-") {
				continue
			}
			// Check if this container belongs to an active run.
			belongsToActive := false
			for runID := range activeRunIDs {
				prefix := fmt.Sprintf("stroppy-agent-%s-", runID)
				if strings.HasPrefix(cleanName, prefix) {
					belongsToActive = true
					break
				}
			}
			if !belongsToActive {
				s.logger.Info("orphan cleanup: removing container", zap.String("name", cleanName))
				cli.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true})
				removed++
			}
		}
	}

	if removed > 0 {
		s.logger.Info("orphan cleanup: removed stale containers", zap.Int("count", removed))
	}

	// Remove stroppy-* networks not belonging to active runs.
	networks, _ := cli.NetworkList(ctx, network.ListOptions{})
	for _, n := range networks {
		if !strings.HasPrefix(n.Name, "stroppy-") || strings.Contains(n.Name, "cloud") {
			continue
		}
		// Check if network belongs to active run (name pattern: stroppy-{runID}).
		belongsToActive := false
		for runID := range activeRunIDs {
			if n.Name == fmt.Sprintf("stroppy-%s", runID) {
				belongsToActive = true
				break
			}
		}
		if !belongsToActive {
			s.logger.Info("orphan cleanup: removing network", zap.String("name", n.Name))
			cli.NetworkRemove(ctx, n.ID)
		}
	}
}

// extractRunID gets the run ID from a machine ID.
// Machine IDs follow the pattern "{runID}-{role}-{index}".
// extractDBKind gets the database kind from a run snapshot.
func extractDBKind(snap *dag.Snapshot) string {
	if snap == nil || snap.State == nil {
		return "postgres" // default
	}
	rcBytes := snap.State.RunConfig
	if rcBytes == nil {
		return "postgres"
	}
	var cfg struct {
		Database struct {
			Kind string `json:"kind"`
		} `json:"database"`
	}
	if err := json.Unmarshal(rcBytes, &cfg); err != nil {
		return "postgres"
	}
	if cfg.Database.Kind != "" {
		return cfg.Database.Kind
	}
	return "postgres"
}

func extractRunID(machineID string) string {
	// Find the last two "-" separated segments and strip them.
	parts := strings.Split(machineID, "-")
	if len(parts) >= 3 {
		return strings.Join(parts[:len(parts)-2], "-")
	}
	return machineID
}

// ============================================================
// Agent binary download
// ============================================================

func (s *Server) serveBinary(w http.ResponseWriter, r *http.Request) {
	binPath, err := agent.SelfBinaryPath()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=stroppy-agent")
	http.ServeFile(w, r, binPath)
}

// ============================================================
// WebSocket for UI log streaming
// ============================================================

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (s *Server) wsLogs(w http.ResponseWriter, r *http.Request) {
	s.handleWS(w, r, "")
}

func (s *Server) wsLogsRun(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "runID")
	s.handleWS(w, r, runID)
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request, filterRunID string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("ws upgrade failed", zap.Error(err))
		return
	}
	s.hub.addClient(conn, filterRunID)
}

// Agents returns currently registered agent targets.
func (s *Server) Agents() map[string]agent.Target {
	s.agentsMu.RLock()
	defer s.agentsMu.RUnlock()
	cp := make(map[string]agent.Target, len(s.agents))
	for k, v := range s.agents {
		cp[k] = v
	}
	return cp
}

// ============================================================
// Log query handler (VictoriaLogs)
// ============================================================

func (s *Server) runLogs(w http.ResponseWriter, r *http.Request) {
	if s.monitoringURL == "" {
		http.Error(w, "log storage not configured (no MONITORING_URL)", http.StatusServiceUnavailable)
		return
	}

	tenantID := auth.TenantID(r.Context())
	accountID, err := s.tenantAccountID(r.Context(), tenantID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	runID := chi.URLParam(r, "runID")

	// Build VictoriaLogs LogsQL query with optional pipes.
	query := fmt.Sprintf(`run_id:"%s"`, runID)
	start := r.URL.Query().Get("start")
	end := r.URL.Query().Get("end")
	limit := r.URL.Query().Get("limit")
	search := r.URL.Query().Get("search")

	// Text search: simple substring match on _msg field.
	if search != "" {
		query += fmt.Sprintf(` _msg:"%s"`, strings.ReplaceAll(search, `"`, `\"`))
	}

	// Sort direction: "desc" returns newest first (for chat-like UI), default is "asc".
	dir := r.URL.Query().Get("dir")
	if dir == "desc" {
		query += " | sort by (_time) desc"
	} else {
		query += " | sort by (_time)"
	}
	if limit != "" {
		query += " | limit " + limit
	}

	vlURL := fmt.Sprintf("%s/select/logsql/query?query=%s",
		s.monitoringURL, url.QueryEscape(query))
	if start != "" {
		vlURL += "&start=" + url.QueryEscape(start)
	}
	if end != "" {
		vlURL += "&end=" + url.QueryEscape(end)
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, vlURL, nil)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	if s.monitoringToken != "" {
		req.Header.Set("Authorization", "Bearer "+s.monitoringToken)
	}
	if accountID > 0 {
		req.Header.Set("AccountID", fmt.Sprintf("%d", accountID))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// ============================================================
// Metrics handlers
// ============================================================

// MetricsRequest is parsed from query parameters for metrics endpoints.
type MetricsRequest struct {
	Start string `json:"start"` // RFC3339
	End   string `json:"end"`   // RFC3339
}

// accountIDFromRunID resolves accountID by looking up the run's tenant, then the tenant's account_id.
func (s *Server) accountIDFromRunID(ctx context.Context, runID string) (int32, error) {
	var tenantID string
	err := s.pool.QueryRow(ctx, "SELECT tenant_id FROM runs WHERE id = $1 LIMIT 1", runID).Scan(&tenantID)
	if err != nil {
		return 0, fmt.Errorf("lookup tenant for run %s: %w", runID, err)
	}
	return s.tenantAccountID(ctx, tenantID)
}

// tenantAccountID looks up the account_id for a tenant.
func (s *Server) tenantAccountID(ctx context.Context, tenantID string) (int32, error) {
	q := pgdb.New(s.pool)
	t, err := q.GetTenant(ctx, tenantID)
	if err != nil {
		return 0, fmt.Errorf("get tenant: %w", err)
	}
	if !t.AccountID.Valid {
		return 0, fmt.Errorf("tenant %s has no account_id", tenantID)
	}
	return t.AccountID.Int32, nil
}

// metricsCollector returns a metrics.Collector configured for the given accountID and DB kind.
func (s *Server) metricsCollector(accountID int32, dbKind string) *metrics.Collector {
	prefix := fmt.Sprintf("%s/select/%d/prometheus", s.monitoringURL, accountID)
	return metrics.NewCollectorForDB(victoria.NewClient(prefix, s.monitoringToken), dbKind)
}

// logsBaseURL returns the VictoriaLogs query base URL for the given accountID.
func (s *Server) logsBaseURL(accountID int32) string {
	return fmt.Sprintf("%s/select/%d", s.monitoringURL, accountID)
}

// getGrafanaConfig handles GET /api/v1/grafana.
func (s *Server) getGrafanaConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"url":           s.grafanaURL,
		"embed_enabled": s.grafanaURL != "",
		"dashboards":    s.grafanaDashboards,
	})
}

func (s *Server) runMetrics(w http.ResponseWriter, r *http.Request) {
	if s.monitoringURL == "" {
		http.Error(w, "metrics not configured (no MONITORING_URL)", http.StatusServiceUnavailable)
		return
	}

	tenantID := auth.TenantID(r.Context())
	runID := chi.URLParam(r, "runID")

	accountID, err := s.tenantAccountID(r.Context(), tenantID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// Load snapshot to get timestamps and db_kind.
	snap, _ := s.app.storage.Load(r.Context(), tenantID, runID)
	dbKind := extractDBKind(snap)

	collector := s.metricsCollector(accountID, dbKind)
	tr, err := parseTimeRange(r)
	if err != nil {
		if snap != nil && !snap.StartedAt.IsZero() {
			end := snap.FinishedAt
			if end.IsZero() {
				end = time.Now()
			}
			tr = metrics.TimeRange{Start: snap.StartedAt.Add(-30 * time.Second), End: end.Add(30 * time.Second)}
		} else {
			http.Error(w, "no time range provided and run has no timestamps", http.StatusBadRequest)
			return
		}
	}

	result, err := collector.Collect(r.Context(), runID, tr)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) compareRuns(w http.ResponseWriter, r *http.Request) {
	if s.monitoringURL == "" {
		http.Error(w, "metrics not configured (no MONITORING_URL)", http.StatusServiceUnavailable)
		return
	}

	tenantID := auth.TenantID(r.Context())

	accountID, err := s.tenantAccountID(r.Context(), tenantID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	runA := r.URL.Query().Get("a")
	runB := r.URL.Query().Get("b")
	if runA == "" || runB == "" {
		http.Error(w, "query params 'a' and 'b' (run IDs) are required", http.StatusBadRequest)
		return
	}

	// Auto-resolve time range from run snapshots if not provided.
	tr, err := parseTimeRange(r)
	if err != nil {
		// Try to derive from run snapshots.
		snapA, _ := s.app.storage.Load(r.Context(), tenantID, runA)
		snapB, _ := s.app.storage.Load(r.Context(), tenantID, runB)
		if snapA == nil || snapB == nil {
			http.Error(w, "runs not found and no explicit time range provided", http.StatusBadRequest)
			return
		}
		start := snapA.StartedAt
		if !snapB.StartedAt.IsZero() && snapB.StartedAt.Before(start) {
			start = snapB.StartedAt
		}
		end := snapA.FinishedAt
		if !snapB.FinishedAt.IsZero() && snapB.FinishedAt.After(end) {
			end = snapB.FinishedAt
		}
		if start.IsZero() || end.IsZero() {
			http.Error(w, "runs have no timestamps and no explicit time range provided", http.StatusBadRequest)
			return
		}
		// Add padding to capture metrics at boundaries.
		tr = metrics.TimeRange{Start: start.Add(-30 * time.Second), End: end.Add(30 * time.Second)}
	}

	// Use dbKind from run A for metric queries.
	snapA2, _ := s.app.storage.Load(r.Context(), tenantID, runA)
	collector := s.metricsCollector(accountID, extractDBKind(snapA2))

	metricsA, err := collector.Collect(r.Context(), runA, tr)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "run A: " + err.Error()})
		return
	}

	metricsB, err := collector.Collect(r.Context(), runB, tr)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "run B: " + err.Error()})
		return
	}

	threshold := 5.0 // default
	if t := r.URL.Query().Get("threshold"); t != "" {
		if v, err := strconv.ParseFloat(t, 64); err == nil && v > 0 {
			threshold = v
		}
	}
	comp := metrics.Compare(metricsA, metricsB, threshold)
	comp.Start = tr.Start
	comp.End = tr.End
	writeJSON(w, http.StatusOK, comp)
}

func parseTimeRange(r *http.Request) (metrics.TimeRange, error) {
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")

	if startStr == "" || endStr == "" {
		return metrics.TimeRange{}, fmt.Errorf("query params 'start' and 'end' (RFC3339) are required")
	}

	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		return metrics.TimeRange{}, fmt.Errorf("invalid 'start': %w", err)
	}
	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		return metrics.TimeRange{}, fmt.Errorf("invalid 'end': %w", err)
	}

	return metrics.TimeRange{Start: start, End: end}, nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// ============================================================
// Baseline management handlers
// ============================================================

func (s *Server) setBaseline(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	name := chi.URLParam(r, "name")
	var body struct {
		RunID string `json:"run_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.RunID == "" {
		http.Error(w, "request body must contain run_id", http.StatusBadRequest)
		return
	}
	if err := s.app.Storage().SetBaseline(r.Context(), tenantID, name, body.RunID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "baseline": name, "run_id": body.RunID})
}

func (s *Server) getBaseline(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	name := chi.URLParam(r, "name")
	runID, err := s.app.Storage().GetBaseline(r.Context(), tenantID, name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if runID == "" {
		http.Error(w, "baseline not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"baseline": name, "run_id": runID})
}

func (s *Server) listBaselines(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	baselines, err := s.app.Storage().ListBaselines(r.Context(), tenantID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, baselines)
}
