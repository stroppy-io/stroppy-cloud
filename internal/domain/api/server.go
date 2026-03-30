package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/stroppy-io/hatchet-workflow/internal/domain/agent"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/metrics"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/types"
	"github.com/stroppy-io/hatchet-workflow/internal/infrastructure/victoria"
)

// Server is the HTTP server exposing agent, external, and UI APIs.
type Server struct {
	app       *App
	logger    *zap.Logger
	hub       *wsHub
	collector *metrics.Collector

	// agentRegistry tracks connected agents by machine ID.
	agentsMu sync.RWMutex
	agents   map[string]agent.Target

	// settings holds admin-managed server settings, protected by settingsMu.
	settingsMu sync.RWMutex
	settings   *types.ServerSettings
}

// NewServer creates an HTTP server backed by the App.
// victoriaURL may be empty to disable metrics endpoints.
func NewServer(app *App, logger *zap.Logger, victoriaURL string) *Server {
	defaults := types.DefaultServerSettings()
	s := &Server{
		app:      app,
		logger:   logger,
		hub:      newWSHub(),
		agents:   make(map[string]agent.Target),
		settings: &defaults,
	}
	if victoriaURL != "" {
		s.collector = metrics.NewCollector(victoria.NewClient(victoriaURL))
	}
	// Wire LogSink so executor logs stream to WebSocket clients.
	app.sink = s.hub
	return s
}

// Router returns the chi router with all routes mounted.
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	// --- Agent API ---
	r.Route("/api/agent", func(r chi.Router) {
		r.Post("/register", s.agentRegister)
		r.Post("/report", s.agentReport)
		r.Post("/log", s.agentLog)
	})

	// --- External API ---
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/run", s.runStart)
		r.Post("/run/{runID}/resume", s.runResume)
		r.Post("/validate", s.runValidate)
		r.Post("/dry-run", s.runDryRun)
		r.Get("/run/{runID}/status", s.runStatus)
		r.Get("/presets", s.listPresets)

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
		})
	})

	// --- UI WebSocket ---
	r.Get("/ws/logs", s.wsLogs)
	r.Get("/ws/logs/{runID}", s.wsLogsRun)

	// --- Agent binary download ---
	r.Get("/agent/binary", s.serveBinary)

	return r
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

	s.hub.broadcast(wsMessage{Type: "agent_log", Payload: line})
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

	// Run asynchronously, return run ID immediately.
	go func() {
		if err := s.app.Start(context.Background(), cfg); err != nil {
			s.logger.Error("run failed", zap.String("run_id", cfg.ID), zap.Error(err))
		}
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{"run_id": cfg.ID, "status": "started"})
}

func (s *Server) runResume(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "runID")

	var cfg types.RunConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	go func() {
		if err := s.app.Resume(context.Background(), runID, cfg); err != nil {
			s.logger.Error("resume failed", zap.String("run_id", runID), zap.Error(err))
		}
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{"run_id": runID, "status": "resumed"})
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
