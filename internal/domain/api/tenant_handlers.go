package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	pgdb "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/generated"
)

// ============================================================
// Tenant management handlers (owner+)
// ============================================================

// listMembers handles GET /api/v1/tenant/members.
func (s *Server) listMembers(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	q := pgdb.New(s.pool)

	members, err := q.ListTenantMembers(r.Context(), tenantID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	type memberDTO struct {
		TenantID  string `json:"tenant_id"`
		UserID    string `json:"user_id"`
		Role      string `json:"role"`
		CreatedAt string `json:"created_at"`
		Username  string `json:"username"`
	}
	out := make([]memberDTO, 0, len(members))
	for _, m := range members {
		out = append(out, memberDTO{
			TenantID:  m.TenantID,
			UserID:    m.UserID,
			Role:      m.Role,
			CreatedAt: formatTimestamptz(m.CreatedAt),
			Username:  m.Username,
		})
	}
	writeJSON(w, http.StatusOK, ensureSlice(out))
}

// addMember handles POST /api/v1/tenant/members.
func (s *Server) addMember(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())

	var req struct {
		UserID string `json:"user_id"`
		Role   string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == "" || req.Role == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "user_id and role are required"})
		return
	}

	if auth.RoleLevel(req.Role) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "role must be viewer, operator, or owner"})
		return
	}

	q := pgdb.New(s.pool)
	if err := q.AddMember(r.Context(), pgdb.AddMemberParams{
		TenantID: tenantID,
		UserID:   req.UserID,
		Role:     req.Role,
	}); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "member already exists or user not found"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

// updateMember handles PUT /api/v1/tenant/members/{userID}.
func (s *Server) updateMember(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	userID := chi.URLParam(r, "userID")

	var req struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Role == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "role is required"})
		return
	}

	if auth.RoleLevel(req.Role) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "role must be viewer, operator, or owner"})
		return
	}

	q := pgdb.New(s.pool)
	if err := q.UpdateMemberRole(r.Context(), pgdb.UpdateMemberRoleParams{
		Role:     req.Role,
		TenantID: tenantID,
		UserID:   userID,
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// removeMember handles DELETE /api/v1/tenant/members/{userID}.
func (s *Server) removeMember(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	userID := chi.URLParam(r, "userID")

	q := pgdb.New(s.pool)
	if err := q.RemoveMember(r.Context(), pgdb.RemoveMemberParams{
		TenantID: tenantID,
		UserID:   userID,
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// listAPITokens handles GET /api/v1/tenant/tokens.
func (s *Server) listAPITokens(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	q := pgdb.New(s.pool)

	tokens, err := q.ListTenantAPITokens(r.Context(), tenantID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	type tokenDTO struct {
		ID        string  `json:"id"`
		TenantID  string  `json:"tenant_id"`
		Name      string  `json:"name"`
		Role      string  `json:"role"`
		CreatedBy string  `json:"created_by"`
		ExpiresAt *string `json:"expires_at"`
		CreatedAt string  `json:"created_at"`
	}
	out := make([]tokenDTO, 0, len(tokens))
	for _, t := range tokens {
		var expiresAt *string
		if t.ExpiresAt.Valid {
			s := t.ExpiresAt.Time.Format(time.RFC3339)
			expiresAt = &s
		}
		out = append(out, tokenDTO{
			ID:        t.ID,
			TenantID:  t.TenantID,
			Name:      t.Name,
			Role:      t.Role,
			CreatedBy: t.CreatedBy,
			ExpiresAt: expiresAt,
			CreatedAt: formatTimestamptz(t.CreatedAt),
		})
	}
	writeJSON(w, http.StatusOK, ensureSlice(out))
}

// createAPIToken handles POST /api/v1/tenant/tokens.
func (s *Server) createAPIToken(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	claims := auth.GetClaims(r.Context())

	var req struct {
		Name      string     `json:"name"`
		Role      string     `json:"role"`
		ExpiresAt *time.Time `json:"expires_at,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" || req.Role == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name and role are required"})
		return
	}

	// Generate random token.
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	plaintext := hex.EncodeToString(b)

	h := sha256.Sum256([]byte(plaintext))
	tokenHash := hex.EncodeToString(h[:])

	var expiresAt pgtype.Timestamptz
	if req.ExpiresAt != nil {
		expiresAt = pgtype.Timestamptz{Time: *req.ExpiresAt, Valid: true}
	}

	q := pgdb.New(s.pool)
	id := uuid.New().String()

	if err := q.CreateAPIToken(r.Context(), pgdb.CreateAPITokenParams{
		ID:        id,
		TenantID:  tenantID,
		Name:      req.Name,
		TokenHash: tokenHash,
		Role:      req.Role,
		CreatedBy: claims.UserID,
		ExpiresAt: expiresAt,
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"id":    id,
		"name":  req.Name,
		"token": plaintext, // returned only once
	})
}

// revokeAPIToken handles DELETE /api/v1/tenant/tokens/{id}.
func (s *Server) revokeAPIToken(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.TenantID(r.Context())
	tokenID := chi.URLParam(r, "id")

	q := pgdb.New(s.pool)
	if err := q.DeleteAPIToken(r.Context(), pgdb.DeleteAPITokenParams{
		ID:       tokenID,
		TenantID: tenantID,
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}
