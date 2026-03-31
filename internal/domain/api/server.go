package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/stroppy-io/stroppy-cloud/internal/core/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/metrics"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/victoria"
)

// Server is the HTTP server exposing agent, external, and UI APIs.
type Server struct {
	app       *App
	logger    *zap.Logger
	hub       *wsHub
	collector *metrics.Collector
	apiKey    string
	users     *UserStore

	// agentRegistry tracks connected agents by machine ID.
	agentsMu sync.RWMutex
	agents   map[string]agent.Target

	// settings holds admin-managed server settings, protected by settingsMu.
	settingsMu   sync.RWMutex
	settings     *types.ServerSettings
	settingsPath string

	// spaFS serves the embedded SPA files. If nil, SPA is not served.
	spaFS http.FileSystem

	// victoriaLogs forwards agent log lines to VictoriaLogs for persistence.
	// nil when VictoriaLogs is not configured.
	victoriaLogs *victoria.LogsClient
}

// NewServer creates an HTTP server backed by the App.
// victoriaURL may be empty to disable metrics endpoints.
// victoriaLogsURL may be empty to disable log persistence.
// apiKey may be empty to disable authentication (development mode).
// settingsPath may be empty to disable settings persistence.
func NewServer(app *App, logger *zap.Logger, victoriaURL, victoriaLogsURL, apiKey, settingsPath string) *Server {
	defaults := types.DefaultServerSettings()
	s := &Server{
		app:          app,
		logger:       logger,
		hub:          newWSHub(),
		agents:       make(map[string]agent.Target),
		settings:     &defaults,
		apiKey:       apiKey,
		users:        NewUserStore(), // always created for SPA login
		settingsPath: settingsPath,
	}
	if victoriaURL != "" {
		s.collector = metrics.NewCollector(victoria.NewClient(victoriaURL))
	}
	if victoriaLogsURL != "" {
		s.victoriaLogs = victoria.NewLogsClient(victoriaLogsURL)
	}
	// Load persisted settings from disk (if available).
	s.loadSettingsFromDisk()
	// Wire LogSink so executor logs stream to WebSocket clients.
	app.sink = s.hub
	// Wire settings getter so buildDeps can access current cloud settings.
	app.settingsFunc = s.currentSettings
	return s
}

// Router returns the chi router with all routes mounted.
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	r.Use(AuthMiddleware(s.apiKey, s.users))

	// --- Health ---
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// --- Agent API ---
	r.Route("/api/agent", func(r chi.Router) {
		r.Post("/register", s.agentRegister)
		r.Post("/report", s.agentReport)
		r.Post("/log", s.agentLog)
	})

	// --- Auth API ---
	r.Post("/api/v1/auth/login", s.login)
	r.Get("/api/v1/auth/me", s.me)

	// --- External API ---
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/run", s.runStart)
		r.Post("/validate", s.runValidate)
		r.Post("/dry-run", s.runDryRun)
		r.Get("/run/{runID}/status", s.runStatus)
		r.Delete("/run/{runID}", s.deleteRun)
		r.Get("/runs", s.listRuns)
		r.Get("/presets", s.listPresets)

		// --- Logs (VictoriaLogs) ---
		r.Get("/run/{runID}/logs", s.runLogs)

		// --- Metrics ---
		r.Get("/run/{runID}/metrics", s.runMetrics)
		r.Get("/compare", s.compareRuns) // ?a=runA&b=runB&start=...&end=...

		// --- Admin ---
		r.Route("/admin", func(r chi.Router) {
			r.Get("/settings", s.getSettings)
			r.Put("/settings", s.updateSettings)
			r.Get("/packages", s.getPackages)
			r.Put("/packages", s.updatePackages)
			r.Get("/db-defaults/{kind}", s.getDBDefaults)
			r.Get("/db-defaults/{kind}/{version}", s.getDBDefaultsVersion)
			r.Get("/grafana", s.getGrafanaSettings)
		})
	})

	// --- UI WebSocket ---
	r.Get("/ws/logs", s.wsLogs)
	r.Get("/ws/logs/{runID}", s.wsLogsRun)

	// --- Upload & package serving ---
	r.Post("/api/v1/upload/deb", s.uploadDeb)
	r.Get("/packages/*", http.StripPrefix("/packages/", http.FileServer(http.Dir(uploadDir))).ServeHTTP)

	// --- Agent binary download ---
	r.Get("/agent/binary", s.serveBinary)

	// --- SPA (embedded frontend) ---
	if s.spaFS != nil {
		// Serve static files, fallback to index.html for SPA routing.
		r.Get("/*", s.serveSPA)
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
	// File not found → serve index.html for client-side routing.
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
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	target := agent.Target{
		ID:        req.MachineID,
		Host:      req.Host,
		AgentPort: req.Port,
	}

	s.agentsMu.Lock()
	s.agents[req.MachineID] = target
	s.agentsMu.Unlock()

	s.logger.Info("agent registered", zap.String("machine_id", req.MachineID), zap.String("host", req.Host))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
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

	// Persist to VictoriaLogs (fire-and-forget).
	if s.victoriaLogs != nil {
		go func() {
			if err := s.victoriaLogs.Ingest(line.MachineID, line.CommandID, runID, line.Stream, line.Line); err != nil {
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
	var cfg types.RunConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Auto-generate ID if empty.
	if cfg.ID == "" {
		cfg.ID = fmt.Sprintf("run-%d", time.Now().UnixMilli())
	}

	// Check for duplicate (run already exists in storage).
	if snap, _ := s.app.Storage().Load(r.Context(), cfg.ID); snap != nil {
		writeJSON(w, http.StatusConflict, map[string]string{
			"error": fmt.Sprintf("run %q already exists", cfg.ID),
		})
		return
	}

	// Run asynchronously, return run ID immediately.
	go func() {
		if err := s.app.Start(context.Background(), cfg); err != nil {
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
	runID := chi.URLParam(r, "runID")

	snap, err := s.app.storage.Load(r.Context(), runID)
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
	runs, err := s.app.Storage().List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, runs)
}

func (s *Server) deleteRun(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "runID")

	// Check exists.
	snap, err := s.app.Storage().Load(r.Context(), runID)
	if err != nil || snap == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	// Clean up Docker resources (best-effort).
	s.cleanupRunResources(runID)

	// Delete from storage.
	if err := s.app.Storage().Delete(r.Context(), runID); err != nil {
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
func (s *Server) RecoverOrCleanupRuns() {
	ctx := context.Background()

	runs, err := s.app.Storage().List(ctx)
	if err != nil {
		s.logger.Warn("recovery: failed to list runs", zap.Error(err))
		return
	}

	// Track run IDs that are being recovered so we don't clean up their containers.
	activeRunIDs := make(map[string]bool)

	for _, r := range runs {
		if r.Pending == 0 {
			continue // completed or fully failed — nothing to do
		}

		s.logger.Info("recovery: found incomplete run",
			zap.String("id", r.ID), zap.Int("pending", r.Pending))

		snap, err := s.app.Storage().Load(ctx, r.ID)
		if err != nil || snap == nil || snap.State == nil {
			s.logger.Warn("recovery: no state saved, marking as failed", zap.String("id", r.ID))
			s.markRunFailed(ctx, snap, r.ID)
			s.cleanupRunResources(r.ID)
			continue
		}

		if s.canRecoverRun(snap.State) {
			s.logger.Info("recovery: containers alive, resuming run", zap.String("id", r.ID))
			activeRunIDs[r.ID] = true
			go s.recoverRun(r.ID, snap)
		} else {
			s.logger.Warn("recovery: containers dead, marking as failed", zap.String("id", r.ID))
			s.markRunFailed(ctx, snap, r.ID)
			s.cleanupRunResources(r.ID)
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
			return true // at least one container alive — worth trying
		}
	}

	return false
}

// recoverRun rebuilds state from a snapshot and resumes execution.
func (s *Server) recoverRun(runID string, snap *dag.Snapshot) {
	ctx := context.Background()

	if err := s.app.RecoverRun(ctx, snap); err != nil {
		s.logger.Error("recovery: run failed",
			zap.String("id", runID), zap.Error(err))
	}
}

// markRunFailed marks all pending nodes in a snapshot as failed.
func (s *Server) markRunFailed(ctx context.Context, snap *dag.Snapshot, runID string) {
	if snap == nil {
		return
	}
	changed := false
	for i := range snap.Nodes {
		if snap.Nodes[i].Status == dag.StatusPending {
			snap.Nodes[i].Status = dag.StatusFailed
			snap.Nodes[i].Error = "server restarted — run orphaned"
			changed = true
		}
	}
	if changed {
		_ = s.app.Storage().Save(ctx, runID, snap)
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
func extractRunID(machineID string) string {
	// Find the last two "-" separated segments and strip them.
	parts := strings.Split(machineID, "-")
	if len(parts) >= 3 {
		return strings.Join(parts[:len(parts)-2], "-")
	}
	return machineID
}

func (s *Server) listPresets(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"postgres": types.PostgresPresets,
		"mysql":    types.MySQLPresets,
		"picodata": types.PicodataPresets,
	})
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
	if s.victoriaLogs == nil {
		http.Error(w, "log storage not configured (no VictoriaLogs URL)", http.StatusServiceUnavailable)
		return
	}

	runID := chi.URLParam(r, "runID")

	// Pass through optional query params to VictoriaLogs.
	query := fmt.Sprintf(`run_id:"%s"`, runID)
	start := r.URL.Query().Get("start")
	end := r.URL.Query().Get("end")

	vlURL := fmt.Sprintf("%s/select/logsql/query?query=%s",
		s.victoriaLogs.BaseURL(), url.QueryEscape(query))
	if start != "" {
		vlURL += "&start=" + url.QueryEscape(start)
	}
	if end != "" {
		vlURL += "&end=" + url.QueryEscape(end)
	}

	resp, err := http.Get(vlURL)
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

func (s *Server) runMetrics(w http.ResponseWriter, r *http.Request) {
	if s.collector == nil {
		http.Error(w, "metrics not configured (no VictoriaMetrics URL)", http.StatusServiceUnavailable)
		return
	}

	runID := chi.URLParam(r, "runID")
	tr, err := parseTimeRange(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := s.collector.Collect(r.Context(), runID, tr)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) compareRuns(w http.ResponseWriter, r *http.Request) {
	if s.collector == nil {
		http.Error(w, "metrics not configured (no VictoriaMetrics URL)", http.StatusServiceUnavailable)
		return
	}

	runA := r.URL.Query().Get("a")
	runB := r.URL.Query().Get("b")
	if runA == "" || runB == "" {
		http.Error(w, "query params 'a' and 'b' (run IDs) are required", http.StatusBadRequest)
		return
	}

	tr, err := parseTimeRange(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	metricsA, err := s.collector.Collect(r.Context(), runA, tr)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "run A: " + err.Error()})
		return
	}

	metricsB, err := s.collector.Collect(r.Context(), runB, tr)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "run B: " + err.Error()})
		return
	}

	threshold := 5.0 // 5% threshold for "same"
	comp := metrics.Compare(metricsA, metricsB, threshold)
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
// Settings persistence
// ============================================================

// currentSettings returns a snapshot of the current server settings.
func (s *Server) currentSettings() *types.ServerSettings {
	s.settingsMu.RLock()
	defer s.settingsMu.RUnlock()
	cp := *s.settings
	return &cp
}

// loadSettingsFromDisk loads settings from the JSON file at settingsPath.
// If the file does not exist or cannot be parsed, defaults are kept.
func (s *Server) loadSettingsFromDisk() {
	if s.settingsPath == "" {
		return
	}
	data, err := os.ReadFile(s.settingsPath)
	if err != nil {
		return // file missing or unreadable — use defaults
	}
	var settings types.ServerSettings
	if json.Unmarshal(data, &settings) == nil {
		s.settings = &settings
	}
}

// saveSettingsToDisk persists the current settings to the JSON file at settingsPath.
func (s *Server) saveSettingsToDisk() error {
	if s.settingsPath == "" {
		return nil
	}
	data, err := json.MarshalIndent(s.settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.settingsPath, data, 0644)
}
