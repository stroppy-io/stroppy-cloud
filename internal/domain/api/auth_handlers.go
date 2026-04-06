package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	pgdb "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/generated"
)

const (
	accessTokenTTL  = 15 * time.Minute
	refreshTokenTTL = 7 * 24 * time.Hour
)

// login handles POST /api/v1/auth/login.
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	q := pgdb.New(s.pool)

	user, err := q.GetUserByUsername(r.Context(), req.Username)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	if !auth.CheckPassword(req.Password, user.PasswordHash) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	tenants, err := q.ListUserTenants(r.Context(), user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	claims := auth.Claims{
		UserID:   user.ID,
		Username: user.Username,
		IsRoot:   user.IsRoot,
	}

	switch {
	case len(tenants) == 1:
		claims.TenantID = tenants[0].TenantID
		claims.Role = tenants[0].Role
	case len(tenants) == 0 && user.IsRoot:
		// Root with no tenants — JWT without tenant_id
	default:
		// Multiple tenants — JWT without tenant_id, frontend shows selector
	}

	accessToken, err := s.jwtIssuer.Issue(claims, accessTokenTTL)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "token generation failed"})
		return
	}

	// Generate refresh token.
	refreshHex, refreshHash := generateRefreshToken()
	if err := q.CreateRefreshToken(r.Context(), pgdb.CreateRefreshTokenParams{
		ID:        uuid.New().String(),
		UserID:    user.ID,
		TokenHash: refreshHash,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(refreshTokenTTL), Valid: true},
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshHex,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Path:     "/api/v1/auth",
		MaxAge:   int(refreshTokenTTL.Seconds()),
	})

	writeJSON(w, http.StatusOK, map[string]string{"access_token": accessToken})
}

// refresh handles POST /api/v1/auth/refresh.
func (s *Server) refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing refresh token"})
		return
	}

	hash := sha256Hex(cookie.Value)
	q := pgdb.New(s.pool)

	rt, err := q.GetRefreshToken(r.Context(), hash)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid refresh token"})
		return
	}

	// Delete old token (rotation).
	_ = q.DeleteRefreshToken(r.Context(), rt.ID)

	// Re-query user and tenants for fresh data.
	user, err := q.GetUser(r.Context(), rt.UserID)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "user not found"})
		return
	}

	tenants, _ := q.ListUserTenants(r.Context(), user.ID)

	claims := auth.Claims{
		UserID:   user.ID,
		Username: user.Username,
		IsRoot:   user.IsRoot,
	}

	if len(tenants) == 1 {
		claims.TenantID = tenants[0].TenantID
		claims.Role = tenants[0].Role
	}

	accessToken, err := s.jwtIssuer.Issue(claims, accessTokenTTL)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "token generation failed"})
		return
	}

	// Issue new refresh token.
	refreshHex, refreshHash := generateRefreshToken()
	_ = q.CreateRefreshToken(r.Context(), pgdb.CreateRefreshTokenParams{
		ID:        uuid.New().String(),
		UserID:    user.ID,
		TokenHash: refreshHash,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(refreshTokenTTL), Valid: true},
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshHex,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Path:     "/api/v1/auth",
		MaxAge:   int(refreshTokenTTL.Seconds()),
	})

	writeJSON(w, http.StatusOK, map[string]string{"access_token": accessToken})
}

// logout handles POST /api/v1/auth/logout.
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err == nil {
		hash := sha256Hex(cookie.Value)
		q := pgdb.New(s.pool)
		rt, dbErr := q.GetRefreshToken(r.Context(), hash)
		if dbErr == nil {
			_ = q.DeleteRefreshToken(r.Context(), rt.ID)
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Path:     "/api/v1/auth",
		MaxAge:   0,
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// authMe handles GET /api/v1/auth/me.
func (s *Server) authMe(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	q := pgdb.New(s.pool)
	tenants, _ := q.ListUserTenants(r.Context(), claims.UserID)

	type tenantInfo struct {
		ID   string `json:"id"`
		Name string `json:"tenant_name"`
		Role string `json:"role"`
	}

	tenantList := make([]tenantInfo, 0, len(tenants))
	for _, t := range tenants {
		tenantList = append(tenantList, tenantInfo{
			ID:   t.TenantID,
			Name: t.TenantName,
			Role: t.Role,
		})
	}

	var tenantID any = claims.TenantID
	if claims.TenantID == "" {
		tenantID = nil
	}
	var role any = claims.Role
	if claims.Role == "" {
		role = nil
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":   claims.UserID,
		"username":  claims.Username,
		"is_root":   claims.IsRoot,
		"tenant_id": tenantID,
		"role":      role,
		"tenants":   ensureSlice(tenantList),
	})
}

// selectTenant handles POST /api/v1/auth/select-tenant.
func (s *Server) selectTenant(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req struct {
		TenantID string `json:"tenant_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TenantID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "tenant_id required"})
		return
	}

	q := pgdb.New(s.pool)

	var role string
	if claims.IsRoot {
		// Root can select any tenant.
		_, err := q.GetTenant(r.Context(), req.TenantID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "tenant not found"})
			return
		}
		role = "owner"
	} else {
		member, err := q.GetMember(r.Context(), pgdb.GetMemberParams{
			TenantID: req.TenantID,
			UserID:   claims.UserID,
		})
		if err != nil {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "not a member of this tenant"})
			return
		}
		role = member.Role
	}

	newClaims := auth.Claims{
		UserID:   claims.UserID,
		Username: claims.Username,
		TenantID: req.TenantID,
		Role:     role,
		IsRoot:   claims.IsRoot,
	}

	accessToken, err := s.jwtIssuer.Issue(newClaims, accessTokenTTL)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "token generation failed"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"access_token": accessToken})
}

// changePassword handles PUT /api/v1/auth/password.
func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	q := pgdb.New(s.pool)

	user, err := q.GetUser(r.Context(), claims.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	if !auth.CheckPassword(req.CurrentPassword, user.PasswordHash) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "current password is incorrect"})
		return
	}

	newHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	if err := q.UpdatePassword(r.Context(), pgdb.UpdatePasswordParams{
		PasswordHash: newHash,
		ID:           claims.UserID,
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// generateRefreshToken creates a random refresh token and returns the hex value and its SHA256 hash.
func generateRefreshToken() (hexValue, hashValue string) {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	hexValue = hex.EncodeToString(b)
	hashValue = sha256Hex(hexValue)
	return
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
