package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
	pgdb "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/generated"
)

// getSettings handles GET /api/v1/settings.
func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	settings := s.loadTenantSettings(r, tenantID)
	writeJSON(w, http.StatusOK, settings)
}

// updateSettings handles PUT /api/v1/settings.
func (s *Server) updateSettings(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())

	var updated types.ServerSettings
	if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	data, err := json.Marshal(updated)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "marshal settings: " + err.Error()})
		return
	}

	q := pgdb.New(s.pool)
	if err := q.UpsertTenantSettings(r.Context(), pgdb.UpsertTenantSettingsParams{
		TenantID: tenantID,
		Settings: string(data),
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "save settings: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// loadTenantSettings reads settings from DB, returning defaults if not found.
func (s *Server) loadTenantSettings(r *http.Request, tenantID string) *types.ServerSettings {
	q := pgdb.New(s.pool)
	row, err := q.GetTenantSettings(r.Context(), tenantID)
	if err != nil {
		if err == pgx.ErrNoRows {
			defaults := types.DefaultServerSettings()
			return &defaults
		}
		defaults := types.DefaultServerSettings()
		return &defaults
	}

	var settings types.ServerSettings
	if json.Unmarshal([]byte(row.Settings), &settings) != nil {
		defaults := types.DefaultServerSettings()
		return &defaults
	}
	return &settings
}

// settingsForTenant returns settings for a tenant, for use by buildDeps.
func (s *Server) settingsForTenant(tenantID string) *types.ServerSettings {
	if tenantID == "" {
		defaults := types.DefaultServerSettings()
		return &defaults
	}
	q := pgdb.New(s.pool)
	row, err := q.GetTenantSettings(context.Background(), tenantID)
	if err != nil {
		defaults := types.DefaultServerSettings()
		return &defaults
	}

	var settings types.ServerSettings
	if json.Unmarshal([]byte(row.Settings), &settings) != nil {
		defaults := types.DefaultServerSettings()
		return &defaults
	}
	return &settings
}
