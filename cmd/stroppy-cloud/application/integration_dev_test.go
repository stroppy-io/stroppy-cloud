//go:build integration

package application

import (
	"net/http"
	"testing"
)

// TestE2EDevMode is a local installation without IAM: static tokens are
// users, stable across restarts, and the SPA learns the mode.
func TestE2EDevMode(t *testing.T) {
	e := e2eServerWith(t, e2eOptions{DevUsers: []string{"dev-alice=alice@stroppy.local=Alice", "dev-root=root@example.com"}})

	var cfg struct {
		AuthMode string `json:"auth_mode"`
	}
	e.want(e.req(http.MethodGet, "/config.json", nil, ""), http.StatusOK, &cfg)
	if cfg.AuthMode != "dev" {
		t.Fatalf("config %+v", cfg)
	}
	var me struct {
		ID              string `json:"id"`
		Email           string `json:"email"`
		DisplayName     string `json:"display_name"`
		IsPlatformAdmin bool   `json:"is_platform_admin"`
	}
	e.want(e.req(http.MethodGet, "/api/v1/me", nil, "dev-alice"), http.StatusOK, &me)
	if me.Email != "alice@stroppy.local" || me.DisplayName != "Alice" || me.IsPlatformAdmin {
		t.Fatalf("me %+v", me)
	}
	first := me.ID
	e.want(e.req(http.MethodGet, "/api/v1/me", nil, "dev-alice"), http.StatusOK, &me)
	if me.ID != first {
		t.Fatalf("dev user id moved: %s → %s", first, me.ID)
	}
	e.want(e.req(http.MethodGet, "/api/v1/me", nil, "dev-root"), http.StatusOK, &me)
	if !me.IsPlatformAdmin {
		t.Fatalf("configured admin %+v", me)
	}
	e.problem(e.req(http.MethodGet, "/api/v1/me", nil, "dev-nobody"), http.StatusUnauthorized, "unauthenticated")
	// A dev user works like any: a tenant of its own.
	var tn struct {
		Slug string `json:"slug"`
	}
	e.want(e.req(http.MethodPost, "/api/v1/tenants", map[string]any{"name": slug("dev")}, "dev-alice"), http.StatusCreated, &tn)
	// Limits set on a tenant nobody opened the settings of yet hold.
	e.want(e.req(http.MethodPut, "/api/v1/admin/tenants/"+tn.Slug+"/limits", map[string]any{"max_concurrent_runs": 7}, "dev-root"), http.StatusOK, nil)
	var lim struct {
		MaxConcurrentRuns int    `json:"max_concurrent_runs"`
		Source            string `json:"source"`
	}
	e.want(e.req(http.MethodGet, "/api/v1/t/"+tn.Slug+"/limits", nil, "dev-alice"), http.StatusOK, &lim)
	if lim.MaxConcurrentRuns != 7 || lim.Source != "tenant_override" {
		t.Fatalf("limits %+v", lim)
	}
}
