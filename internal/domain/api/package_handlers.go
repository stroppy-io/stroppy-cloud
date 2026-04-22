package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
	pgdb "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/generated"
)

type pkgListItem struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	DbKind      string   `json:"db_kind"`
	DbVersion   string   `json:"db_version"`
	IsBuiltin   bool     `json:"is_builtin"`
	AptPackages []string `json:"apt_packages"`
	PreInstall  []string `json:"pre_install"`
	CustomRepo  string   `json:"custom_repo,omitempty"`
	DebFilename string   `json:"deb_filename,omitempty"`
	HasDeb      bool     `json:"has_deb"`
	CreatedAt   string   `json:"created_at"`
}

// ─── List ────────────────────────────────────────────────────────

func (s *Server) listPackages(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	q := pgdb.New(s.pool)

	dbKind := r.URL.Query().Get("db_kind")
	dbVersion := r.URL.Query().Get("db_version")

	// Helper to map any of the three row types to pkgListItem.
	mapRow := func(id, name, desc, kind, ver string, builtin bool, apt, pre []string, repo, debFn string, ts string) pkgListItem {
		return pkgListItem{
			ID: id, Name: name, Description: desc, DbKind: kind, DbVersion: ver,
			IsBuiltin: builtin, AptPackages: apt, PreInstall: pre,
			CustomRepo: repo, DebFilename: debFn, HasDeb: debFn != "", CreatedAt: ts,
		}
	}

	var out []pkgListItem
	var err error

	switch {
	case dbKind != "" && dbVersion != "":
		rows, e := q.ListPackagesByKindVersion(r.Context(), pgdb.ListPackagesByKindVersionParams{
			TenantID: tenantID, DbKind: dbKind, DbVersion: dbVersion,
		})
		err = e
		for _, r := range rows {
			out = append(out, mapRow(r.ID, r.Name, r.Description, r.DbKind, r.DbVersion, r.IsBuiltin, r.AptPackages, r.PreInstall, r.CustomRepo, r.DebFilename, r.CreatedAt.Time.Format("2006-01-02T15:04:05Z")))
		}
	case dbKind != "":
		rows, e := q.ListPackagesByKind(r.Context(), pgdb.ListPackagesByKindParams{
			TenantID: tenantID, DbKind: dbKind,
		})
		err = e
		for _, r := range rows {
			out = append(out, mapRow(r.ID, r.Name, r.Description, r.DbKind, r.DbVersion, r.IsBuiltin, r.AptPackages, r.PreInstall, r.CustomRepo, r.DebFilename, r.CreatedAt.Time.Format("2006-01-02T15:04:05Z")))
		}
	default:
		rows, e := q.ListPackages(r.Context(), tenantID)
		err = e
		for _, r := range rows {
			out = append(out, mapRow(r.ID, r.Name, r.Description, r.DbKind, r.DbVersion, r.IsBuiltin, r.AptPackages, r.PreInstall, r.CustomRepo, r.DebFilename, r.CreatedAt.Time.Format("2006-01-02T15:04:05Z")))
		}
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, ensureSlice(out))
}

// ─── Get ─────────────────────────────────────────────────────────

func (s *Server) getPackage(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	id := chi.URLParam(r, "id")
	q := pgdb.New(s.pool)

	pkg, err := q.GetPackage(r.Context(), pgdb.GetPackageParams{ID: id, TenantID: tenantID})
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "package not found"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id": pkg.ID, "name": pkg.Name, "description": pkg.Description,
		"db_kind": pkg.DbKind, "db_version": pkg.DbVersion, "is_builtin": pkg.IsBuiltin,
		"apt_packages": pkg.AptPackages, "pre_install": pkg.PreInstall,
		"custom_repo": pkg.CustomRepo, "custom_repo_key": pkg.CustomRepoKey,
		"deb_filename": pkg.DebFilename, "has_deb": pkg.DebFilename != "",
		"created_at": pkg.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
	})
}

// ─── Create ──────────────────────────────────────────────────────

type packageReq struct {
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	DbKind        string   `json:"db_kind"`
	DbVersion     string   `json:"db_version"`
	AptPackages   []string `json:"apt_packages"`
	PreInstall    []string `json:"pre_install"`
	CustomRepo    string   `json:"custom_repo"`
	CustomRepoKey string   `json:"custom_repo_key"`
}

func (s *Server) createPackage(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	var req packageReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" || req.DbKind == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name and db_kind are required"})
		return
	}

	q := pgdb.New(s.pool)
	id := uuid.New().String()
	if req.AptPackages == nil {
		req.AptPackages = []string{}
	}
	if req.PreInstall == nil {
		req.PreInstall = []string{}
	}

	if err := q.CreatePackage(r.Context(), pgdb.CreatePackageParams{
		ID: id, TenantID: tenantID, Name: req.Name, Description: req.Description,
		DbKind: req.DbKind, DbVersion: req.DbVersion, IsBuiltin: false,
		AptPackages: req.AptPackages, PreInstall: req.PreInstall,
		CustomRepo: req.CustomRepo, CustomRepoKey: req.CustomRepoKey,
		DebFilename: "",
	}); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "name already exists"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

// ─── Update ──────────────────────────────────────────────────────

func (s *Server) updatePackage(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	id := chi.URLParam(r, "id")
	var req packageReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	q := pgdb.New(s.pool)
	pkg, err := q.GetPackage(r.Context(), pgdb.GetPackageParams{ID: id, TenantID: tenantID})
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if pkg.IsBuiltin {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "cannot edit built-in package, clone it first"})
		return
	}
	if req.AptPackages == nil {
		req.AptPackages = []string{}
	}
	if req.PreInstall == nil {
		req.PreInstall = []string{}
	}

	if err := q.UpdatePackage(r.Context(), pgdb.UpdatePackageParams{
		ID: id, TenantID: tenantID, Name: req.Name, Description: req.Description,
		DbKind: req.DbKind, DbVersion: req.DbVersion,
		AptPackages: req.AptPackages, PreInstall: req.PreInstall,
		CustomRepo: req.CustomRepo, CustomRepoKey: req.CustomRepoKey,
		DebFilename: pkg.DebFilename, // preserve existing deb
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// ─── Delete ──────────────────────────────────────────────────────

func (s *Server) deletePackage(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	id := chi.URLParam(r, "id")
	q := pgdb.New(s.pool)

	if err := q.DeletePackage(r.Context(), pgdb.DeletePackageParams{ID: id, TenantID: tenantID}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ─── Clone ───────────────────────────────────────────────────────

func (s *Server) clonePackage(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	srcID := chi.URLParam(r, "id")
	q := pgdb.New(s.pool)

	src, err := q.GetPackage(r.Context(), pgdb.GetPackageParams{ID: srcID, TenantID: tenantID})
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "source not found"})
		return
	}

	newID := uuid.New().String()
	newName := src.Name + " (copy)"

	if err := q.CreatePackage(r.Context(), pgdb.CreatePackageParams{
		ID: newID, TenantID: tenantID, Name: newName, Description: src.Description,
		DbKind: src.DbKind, DbVersion: src.DbVersion, IsBuiltin: false,
		AptPackages: src.AptPackages, PreInstall: src.PreInstall,
		CustomRepo: src.CustomRepo, CustomRepoKey: src.CustomRepoKey,
		DebFilename: "", // don't copy deb binary, user re-uploads
	}); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "clone failed: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"id": newID, "name": newName})
}

// ─── Upload .deb to a package ────────────────────────────────────

func (s *Server) uploadPackageDeb(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	id := chi.URLParam(r, "id")

	if err := r.ParseMultipartForm(500 << 20); err != nil {
		http.Error(w, "parse form: "+err.Error(), http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing 'file' field", http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "read file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	q := pgdb.New(s.pool)
	if err := q.UpdatePackageDebData(r.Context(), pgdb.UpdatePackageDebDataParams{
		ID: id, TenantID: tenantID, DebData: data, DebFilename: header.Filename,
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":   "uploaded",
		"filename": header.Filename,
		"size":     fmt.Sprintf("%d", len(data)),
	})
}

// ─── Download .deb from a package ────────────────────────────────

func (s *Server) downloadPackageDeb(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	id := chi.URLParam(r, "id")
	q := pgdb.New(s.pool)

	row, err := q.GetPackageDebData(r.Context(), pgdb.GetPackageDebDataParams{ID: id, TenantID: tenantID})
	if err != nil || len(row.DebData) == 0 {
		http.Error(w, "no deb file", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.debian.binary-package")
	w.Header().Set("Content-Disposition", `attachment; filename="`+row.DebFilename+`"`)
	w.Write(row.DebData)
}

// ─── Seed built-ins for new tenant ───────────────────────────────

func (s *Server) seedBuiltinPackages(ctx context.Context, tenantID string) {
	q := pgdb.New(s.pool)
	for _, bp := range types.BuiltinPackages() {
		if bp.AptPackages == nil {
			bp.AptPackages = []string{}
		}
		if bp.PreInstall == nil {
			bp.PreInstall = []string{}
		}
		_ = q.CreatePackage(ctx, pgdb.CreatePackageParams{
			ID: uuid.New().String(), TenantID: tenantID,
			Name: bp.Name, Description: bp.Description,
			DbKind: bp.DbKind, DbVersion: bp.DbVersion, IsBuiltin: true,
			AptPackages: bp.AptPackages, PreInstall: bp.PreInstall,
			CustomRepo: bp.CustomRepo, CustomRepoKey: bp.CustomRepoKey,
			DebFilename: "",
		})
	}
}

// ─── Resolve package for run ─────────────────────────────────────

// resolveRunPackage loads a Package by ID or finds the default built-in for db_kind+version.
// It sets the DebFilename to the download URL so the agent can curl it.
func (s *Server) resolveRunPackage(ctx context.Context, tenantID string, cfg *types.RunConfig) error {
	// YDB doesn't use apt packages — it downloads the ydbd binary directly.
	if cfg.Database.Kind == types.DatabaseYDB {
		return nil
	}

	q := pgdb.New(s.pool)

	var pkg pgdb.GetPackageRow
	var err error

	if cfg.PackageID != "" {
		pkg, err = q.GetPackage(ctx, pgdb.GetPackageParams{ID: cfg.PackageID, TenantID: tenantID})
	} else {
		// Find default built-in for this db kind+version.
		rows, e := q.ListPackagesByKindVersion(ctx, pgdb.ListPackagesByKindVersionParams{
			TenantID: tenantID, DbKind: string(cfg.Database.Kind), DbVersion: cfg.Database.Version,
		})
		if e != nil || len(rows) == 0 {
			return fmt.Errorf("no package found for %s %s", cfg.Database.Kind, cfg.Database.Version)
		}
		// Pick the first (built-ins sort first).
		r := rows[0]
		pkg = pgdb.GetPackageRow{
			ID: r.ID, TenantID: r.TenantID, Name: r.Name, Description: r.Description,
			DbKind: r.DbKind, DbVersion: r.DbVersion, IsBuiltin: r.IsBuiltin,
			AptPackages: r.AptPackages, PreInstall: r.PreInstall,
			CustomRepo: r.CustomRepo, CustomRepoKey: r.CustomRepoKey,
			DebFilename: r.DebFilename,
		}
		err = nil
	}
	if err != nil {
		return fmt.Errorf("package not found: %w", err)
	}

	resolved := &types.Package{
		ID:            pkg.ID,
		Name:          pkg.Name,
		DbKind:        pkg.DbKind,
		DbVersion:     pkg.DbVersion,
		AptPackages:   pkg.AptPackages,
		PreInstall:    pkg.PreInstall,
		CustomRepo:    pkg.CustomRepo,
		CustomRepoKey: pkg.CustomRepoKey,
	}

	// If the package has a .deb, set DebFilename to the download URL + auth token.
	if pkg.DebFilename != "" {
		serverAddr := ""
		if settings := s.settingsForTenant(tenantID); settings != nil {
			serverAddr = settings.Cloud.ServerAddr
		}
		if serverAddr == "" {
			serverAddr = dockerHostAddr(s.app.listenAddr)
		}
		resolved.DebFilename = fmt.Sprintf("%s/api/v1/packages/%s/deb", serverAddr, pkg.ID)

		// Generate a short-lived token for the agent to download this .deb.
		if s.jwtIssuer != nil {
			token, err := s.jwtIssuer.Issue(auth.Claims{
				UserID: "deb-download", TenantID: tenantID, Role: "viewer",
			}, 2*time.Hour)
			if err == nil {
				resolved.DebToken = token
			}
		}
	}

	cfg.ResolvedPackage = resolved
	return nil
}
