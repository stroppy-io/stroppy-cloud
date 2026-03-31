package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

// ============================================================
// Admin API handlers
// ============================================================

// getSettings returns the current server settings.
// GET /api/v1/admin/settings
func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	s.settingsMu.RLock()
	settings := *s.settings
	s.settingsMu.RUnlock()

	writeJSON(w, http.StatusOK, settings)
}

// updateSettings replaces the server settings with the provided payload.
// PUT /api/v1/admin/settings
func (s *Server) updateSettings(w http.ResponseWriter, r *http.Request) {
	var updated types.ServerSettings
	if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.settingsMu.Lock()
	s.settings = &updated
	if err := s.saveSettingsToDisk(); err != nil {
		s.settingsMu.Unlock()
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "persist settings: " + err.Error()})
		return
	}
	s.settingsMu.Unlock()

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// getPackages returns the current package defaults.
// GET /api/v1/admin/packages
func (s *Server) getPackages(w http.ResponseWriter, r *http.Request) {
	s.settingsMu.RLock()
	packages := s.settings.Packages
	s.settingsMu.RUnlock()

	writeJSON(w, http.StatusOK, packages)
}

// updatePackages replaces the package defaults.
// PUT /api/v1/admin/packages
func (s *Server) updatePackages(w http.ResponseWriter, r *http.Request) {
	var updated types.PackageDefaults
	if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.settingsMu.Lock()
	s.settings.Packages = updated
	if err := s.saveSettingsToDisk(); err != nil {
		s.settingsMu.Unlock()
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "persist settings: " + err.Error()})
		return
	}
	s.settingsMu.Unlock()

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// getDBDefaults returns the topology presets for the given database kind.
// GET /api/v1/admin/db-defaults/{kind}
func (s *Server) getDBDefaults(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")

	switch types.DatabaseKind(kind) {
	case types.DatabasePostgres:
		writeJSON(w, http.StatusOK, types.PostgresPresets)
	case types.DatabaseMySQL:
		writeJSON(w, http.StatusOK, types.MySQLPresets)
	case types.DatabasePicodata:
		writeJSON(w, http.StatusOK, types.PicodataPresets)
	default:
		http.Error(w, "unknown database kind: "+kind, http.StatusBadRequest)
	}
}

// getDBDefaultsVersion returns the topology presets for a specific database kind and version.
// GET /api/v1/admin/db-defaults/{kind}/{version}
func (s *Server) getDBDefaultsVersion(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")
	version := chi.URLParam(r, "version")

	var presets any
	switch types.DatabaseKind(kind) {
	case types.DatabasePostgres:
		presets = types.PostgresPresets
	case types.DatabaseMySQL:
		presets = types.MySQLPresets
	case types.DatabasePicodata:
		presets = types.PicodataPresets
	default:
		http.Error(w, "unknown database kind: "+kind, http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"kind":    kind,
		"version": version,
		"presets": presets,
	})
}
