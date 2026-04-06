package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
	pgdb "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/generated"
)

func formatTimestamptz(ts pgtype.Timestamptz) string {
	if !ts.Valid {
		return ""
	}
	return ts.Time.Format(time.RFC3339)
}

func ensureSlice[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// ============================================================
// Root-only admin handlers
// ============================================================

// listTenantsAdmin handles GET /api/v1/admin/tenants.
func (s *Server) listTenantsAdmin(w http.ResponseWriter, r *http.Request) {
	q := pgdb.New(s.pool)
	tenants, err := q.ListTenants(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	type tenantDTO struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		CreatedAt string `json:"created_at"`
	}
	out := make([]tenantDTO, 0, len(tenants))
	for _, t := range tenants {
		out = append(out, tenantDTO{
			ID:        t.ID,
			Name:      t.Name,
			CreatedAt: formatTimestamptz(t.CreatedAt),
		})
	}
	writeJSON(w, http.StatusOK, ensureSlice(out))
}

// createTenantAdmin handles POST /api/v1/admin/tenants.
func (s *Server) createTenantAdmin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	q := pgdb.New(s.pool)
	id := uuid.New().String()

	if err := q.CreateTenant(r.Context(), pgdb.CreateTenantParams{
		ID:   id,
		Name: req.Name,
	}); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "tenant name already exists"})
		return
	}

	// Create default settings for the tenant (with sensible defaults).
	defaultSettings := types.DefaultServerSettings()
	settingsJSON, _ := json.Marshal(defaultSettings)
	_ = q.UpsertTenantSettings(r.Context(), pgdb.UpsertTenantSettingsParams{
		TenantID: id,
		Settings: string(settingsJSON),
	})

	writeJSON(w, http.StatusCreated, map[string]string{"id": id, "name": req.Name})
}

// deleteTenantAdmin handles DELETE /api/v1/admin/tenants/{id}.
func (s *Server) deleteTenantAdmin(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	q := pgdb.New(s.pool)

	if err := q.DeleteTenant(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// listUsersAdmin handles GET /api/v1/admin/users.
func (s *Server) listUsersAdmin(w http.ResponseWriter, r *http.Request) {
	q := pgdb.New(s.pool)
	users, err := q.ListUsers(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	type userDTO struct {
		ID        string `json:"id"`
		Username  string `json:"username"`
		IsRoot    bool   `json:"is_root"`
		CreatedAt string `json:"created_at"`
	}
	out := make([]userDTO, 0, len(users))
	for _, u := range users {
		out = append(out, userDTO{
			ID:        u.ID,
			Username:  u.Username,
			IsRoot:    u.IsRoot,
			CreatedAt: formatTimestamptz(u.CreatedAt),
		})
	}
	writeJSON(w, http.StatusOK, ensureSlice(out))
}

// createUserAdmin handles POST /api/v1/admin/users.
func (s *Server) createUserAdmin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		IsRoot   bool   `json:"is_root"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username and password are required"})
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	q := pgdb.New(s.pool)
	id := uuid.New().String()

	if err := q.CreateUser(r.Context(), pgdb.CreateUserParams{
		ID:           id,
		Username:     req.Username,
		PasswordHash: hash,
		IsRoot:       req.IsRoot,
	}); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "username already exists"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"id": id, "username": req.Username})
}

// deleteUserAdmin handles DELETE /api/v1/admin/users/{id}.
func (s *Server) deleteUserAdmin(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	q := pgdb.New(s.pool)

	if err := q.DeleteUser(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// resetPasswordAdmin handles PUT /api/v1/admin/users/{id}/password.
func (s *Server) resetPasswordAdmin(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "password is required"})
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	q := pgdb.New(s.pool)
	if err := q.UpdatePassword(r.Context(), pgdb.UpdatePasswordParams{
		PasswordHash: hash,
		ID:           id,
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
