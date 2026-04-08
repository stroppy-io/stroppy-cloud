package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
	pgdb "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/generated"
)

type presetListItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	DbKind      string `json:"db_kind"`
	IsBuiltin   bool   `json:"is_builtin"`
	Topology    any    `json:"topology"`
	CreatedAt   string `json:"created_at"`
}

func presetRowToListItem(row pgdb.Preset) presetListItem {
	var topo any
	_ = json.Unmarshal([]byte(row.Topology), &topo)
	return presetListItem{
		ID:          row.ID,
		Name:        row.Name,
		Description: row.Description,
		DbKind:      row.DbKind,
		IsBuiltin:   row.IsBuiltin,
		Topology:    topo,
		CreatedAt:   row.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
	}
}

// ─── List ────────────────────────────────────────────────────────

func (s *Server) listPresetsTenant(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	q := pgdb.New(s.pool)

	dbKind := r.URL.Query().Get("db_kind")

	var items []pgdb.Preset
	var err error

	if dbKind != "" {
		items, err = q.ListPresetsByKind(r.Context(), pgdb.ListPresetsByKindParams{
			TenantID: tenantID, DbKind: dbKind,
		})
	} else {
		items, err = q.ListPresetsTenant(r.Context(), tenantID)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	out := make([]presetListItem, 0, len(items))
	for _, row := range items {
		out = append(out, presetRowToListItem(row))
	}
	writeJSON(w, http.StatusOK, ensureSlice(out))
}

// ─── Get ─────────────────────────────────────────────────────────

func (s *Server) getPreset(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	id := chi.URLParam(r, "id")
	q := pgdb.New(s.pool)

	row, err := q.GetPreset(r.Context(), pgdb.GetPresetParams{ID: id, TenantID: tenantID})
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "preset not found"})
		return
	}

	writeJSON(w, http.StatusOK, presetRowToListItem(row))
}

// ─── Create ──────────────────────────────────────────────────────

type presetReq struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	DbKind      string          `json:"db_kind"`
	Topology    json.RawMessage `json:"topology"`
}

func (s *Server) createPreset(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	var req presetReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" || req.DbKind == "" || len(req.Topology) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name, db_kind, and topology are required"})
		return
	}

	// Validate topology parses for the given db_kind.
	p := types.Preset{DbKind: req.DbKind}
	if err := p.ParseTopology(string(req.Topology)); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid topology: " + err.Error()})
		return
	}

	q := pgdb.New(s.pool)
	id := uuid.New().String()

	if err := q.CreatePreset(r.Context(), pgdb.CreatePresetParams{
		ID: id, TenantID: tenantID, Name: req.Name, Description: req.Description,
		DbKind: req.DbKind, Topology: string(req.Topology), IsBuiltin: false,
	}); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "name already exists"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

// ─── Update ──────────────────────────────────────────────────────

func (s *Server) updatePreset(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	id := chi.URLParam(r, "id")

	var req presetReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	q := pgdb.New(s.pool)
	existing, err := q.GetPreset(r.Context(), pgdb.GetPresetParams{ID: id, TenantID: tenantID})
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if existing.IsBuiltin {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "cannot edit built-in preset, clone it first"})
		return
	}

	// Use existing topology if not provided in update.
	topoStr := existing.Topology
	if len(req.Topology) > 0 {
		p := types.Preset{DbKind: existing.DbKind}
		if err := p.ParseTopology(string(req.Topology)); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid topology: " + err.Error()})
			return
		}
		topoStr = string(req.Topology)
	}

	if req.Name == "" {
		req.Name = existing.Name
	}

	if err := q.UpdatePreset(r.Context(), pgdb.UpdatePresetParams{
		ID: id, TenantID: tenantID, Name: req.Name, Description: req.Description,
		Topology: topoStr,
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// ─── Delete ──────────────────────────────────────────────────────

func (s *Server) deletePreset(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	id := chi.URLParam(r, "id")
	q := pgdb.New(s.pool)

	if err := q.DeletePreset(r.Context(), pgdb.DeletePresetParams{ID: id, TenantID: tenantID}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ─── Clone ───────────────────────────────────────────────────────

func (s *Server) clonePreset(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	srcID := chi.URLParam(r, "id")
	q := pgdb.New(s.pool)

	src, err := q.GetPreset(r.Context(), pgdb.GetPresetParams{ID: srcID, TenantID: tenantID})
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "source not found"})
		return
	}

	newID := uuid.New().String()
	newName := src.Name + " (copy)"

	if err := q.CreatePreset(r.Context(), pgdb.CreatePresetParams{
		ID: newID, TenantID: tenantID, Name: newName, Description: src.Description,
		DbKind: src.DbKind, Topology: src.Topology, IsBuiltin: false,
	}); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "clone failed: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"id": newID, "name": newName})
}

// ─── Seed built-ins for new tenant ───────────────────────────────

func (s *Server) seedBuiltinPresets(ctx context.Context, tenantID string) {
	q := pgdb.New(s.pool)
	for _, bp := range types.BuiltinPresets() {
		topoJSON, err := bp.TopologyJSON()
		if err != nil {
			continue
		}
		_ = q.CreatePreset(ctx, pgdb.CreatePresetParams{
			ID: uuid.New().String(), TenantID: tenantID,
			Name: bp.Name, Description: bp.Description,
			DbKind: bp.DbKind, Topology: topoJSON, IsBuiltin: true,
		})
	}
}

// ─── Resolve preset for run ─────────────────────────────────────

// resolveRunPreset loads a Preset by ID and applies its topology to RunConfig.Database
// if the topology is not already set in the request.
func (s *Server) resolveRunPreset(ctx context.Context, tenantID string, cfg *types.RunConfig) error {
	if cfg.PresetID == "" {
		return nil
	}

	q := pgdb.New(s.pool)
	row, err := q.GetPreset(ctx, pgdb.GetPresetParams{ID: cfg.PresetID, TenantID: tenantID})
	if err != nil {
		return fmt.Errorf("preset not found: %w", err)
	}

	// Set db_kind from preset if not specified.
	if cfg.Database.Kind == "" {
		cfg.Database.Kind = types.DatabaseKind(row.DbKind)
	}

	// Topology from request takes priority over preset.
	if cfg.Database.Postgres != nil || cfg.Database.MySQL != nil || cfg.Database.Picodata != nil {
		return nil
	}

	// Apply topology from preset.
	preset := types.Preset{DbKind: row.DbKind}
	if err := preset.ParseTopology(row.Topology); err != nil {
		return fmt.Errorf("invalid preset topology: %w", err)
	}

	switch types.DatabaseKind(row.DbKind) {
	case types.DatabasePostgres:
		cfg.Database.Postgres = preset.Postgres
	case types.DatabaseMySQL:
		cfg.Database.MySQL = preset.MySQL
	case types.DatabasePicodata:
		cfg.Database.Picodata = preset.Picodata
	}

	return nil
}
