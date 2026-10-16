//go:build integration

package application

import (
	"net/http"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/provider"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/tenant"
)

func TestE2EAdmin(t *testing.T) {
	e := e2eServer(t)
	base, tok, testID, tn := runFixture(t, e)
	// root is a config-listed admin; boss becomes one through the UI.
	root := e.person("root@example.com", "Root")
	rootTenant := e.tenant(root, slug("ops"))
	rtok := e.token(root, rootTenant)
	boss := e.person(slug("boss")+"@example.com", "Boss")
	bossTenant := e.tenant(boss, slug("boss"))
	btok := e.token(boss, bossTenant)

	t.Run("only platform admins pass", func(t *testing.T) {
		e.problem(e.req(http.MethodGet, "/api/v1/admin/tenants", nil, tok), http.StatusForbidden, "forbidden")
		e.problem(e.req(http.MethodGet, "/api/v1/admin/tenants", nil, btok), http.StatusForbidden, "forbidden")
		e.want(e.req(http.MethodGet, "/api/v1/admin/tenants", nil, rtok), http.StatusOK, nil)
		var u struct {
			IsPlatformAdmin bool   `json:"is_platform_admin"`
			AdminSource     string `json:"admin_source"`
		}
		e.want(e.req(http.MethodPatch, "/api/v1/admin/users/"+boss.UserID.String(), map[string]any{"is_platform_admin": true}, rtok), http.StatusOK, &u)
		if !u.IsPlatformAdmin || u.AdminSource != "db" {
			t.Fatalf("user %+v", u)
		}
		e.want(e.req(http.MethodGet, "/api/v1/admin/tenants", nil, btok), http.StatusOK, nil)
		var users struct {
			Data []struct {
				Email       string `json:"email"`
				AdminSource string `json:"admin_source"`
			} `json:"data"`
		}
		e.want(e.req(http.MethodGet, "/api/v1/admin/users?platform_admin=true", nil, rtok), http.StatusOK, &users)
		sources := map[string]string{}
		for _, x := range users.Data {
			sources[x.Email] = x.AdminSource
		}
		if sources["root@example.com"] != "config" || sources[boss.Email] != "db" {
			t.Fatalf("admins %+v", sources)
		}
	})

	t.Run("tenants: list, details, suspend/resume, assign owner", func(t *testing.T) {
		var list struct {
			Data []struct {
				Slug string `json:"slug"`
			} `json:"data"`
		}
		e.want(e.req(http.MethodGet, "/api/v1/admin/tenants?search="+tn.Slug, nil, rtok), http.StatusOK, &list)
		if len(list.Data) != 1 || list.Data[0].Slug != tn.Slug {
			t.Fatalf("list %+v", list)
		}
		var det struct {
			Status   string `json:"status"`
			Counters struct {
				Members   int `json:"members"`
				Providers int `json:"providers"`
			} `json:"counters"`
			SuspendedReason string `json:"suspended_reason"`
		}
		e.want(e.req(http.MethodGet, "/api/v1/admin/tenants/"+tn.Slug, nil, rtok), http.StatusOK, &det)
		if det.Status != "active" || det.Counters.Members != 1 || det.Counters.Providers != 1 {
			t.Fatalf("details %+v", det)
		}
		// One run starts; with no room left in the cloud's quota the next
		// one waits in the queue.
		var r runView
		e.want(e.req(http.MethodPost, base+"/tests/"+testID+":launch", map[string]any{}, tok), http.StatusCreated, &r)
		var provs struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		e.want(e.req(http.MethodGet, base+"/providers", nil, tok), http.StatusOK, &provs)
		e.graphene.mu.Lock()
		e.graphene.quotas = []provider.Quota{{Name: "compute.instanceCores.count", Limit: 1000, Used: 999, Unit: "cores"}}
		e.graphene.mu.Unlock()
		e.want(e.req(http.MethodPost, base+"/providers/"+provs.Data[0].ID+"/quotas:refresh", nil, tok), http.StatusOK, nil)
		var queued runView
		e.want(e.req(http.MethodPost, base+"/tests/"+testID+":launch", map[string]any{}, tok), http.StatusCreated, &queued)
		if queued.Queue.WaitingFor != "quota" {
			t.Fatalf("no room in the quota holds the second run in the queue: %+v", queued)
		}

		e.want(e.req(http.MethodPost, "/api/v1/admin/tenants/"+tn.Slug+":suspend", map[string]any{"reason": "unpaid"}, rtok), http.StatusOK, &det)
		if det.Status != "suspended" || det.SuspendedReason != "unpaid" {
			t.Fatalf("suspended %+v", det)
		}
		e.problem(e.req(http.MethodGet, base+"/runs", nil, tok), http.StatusForbidden, "forbidden")
		e.want(e.req(http.MethodPost, "/api/v1/admin/tenants/"+tn.Slug+":resume", nil, rtok), http.StatusOK, &det)
		if det.Status != "active" {
			t.Fatalf("resumed %+v", det)
		}
		e.want(e.req(http.MethodGet, base+"/runs", nil, tok), http.StatusOK, nil)

		// The global queue sees the pending run; the admin cancels it.
		var queue struct {
			Data []struct {
				ID     string `json:"id"`
				Status string `json:"status"`
				Tenant struct {
					Name string `json:"name"`
				} `json:"tenant"`
			} `json:"data"`
		}
		e.want(e.req(http.MethodGet, "/api/v1/admin/runs?status=pending&tenant="+tn.Slug, nil, rtok), http.StatusOK, &queue)
		seen := map[string]bool{}
		for _, q := range queue.Data {
			seen[q.ID] = q.Tenant.Name == tn.Slug
		}
		if len(queue.Data) != 2 || !seen[r.ID] || !seen[queued.ID] {
			t.Fatalf("queue %+v", queue)
		}
		e.want(e.req(http.MethodPost, "/api/v1/admin/runs/"+queued.ID+":cancel", nil, rtok), http.StatusOK, &queued)
		if queued.Status != "cancelled" {
			t.Fatalf("a queued run is cancelled at once: %+v", queued)
		}
		e.want(e.req(http.MethodPost, "/api/v1/admin/runs/"+r.ID+":cancel", nil, rtok), http.StatusOK, &r)
		if r.Status != "cancelling" {
			t.Fatalf("cancelled %+v", r)
		}

		// Ownership moves to an heir; the previous owner stays as admin. An
		// account that already owns a tenant cannot take another.
		heir := e.person(slug("heir")+"@example.com", "Heir")
		e.problem(e.req(http.MethodPost, "/api/v1/admin/tenants/"+bossTenant.Slug+":assign-owner", map[string]any{"user_id": root.UserID.String()}, rtok), http.StatusConflict, "conflict")
		var assigned struct {
			Owner struct {
				ID string `json:"id"`
			} `json:"owner"`
		}
		e.want(e.req(http.MethodPost, "/api/v1/admin/tenants/"+bossTenant.Slug+":assign-owner", map[string]any{"user_id": heir.UserID.String()}, rtok), http.StatusOK, &assigned)
		if assigned.Owner.ID != heir.UserID.String() {
			t.Fatalf("assigned %+v", assigned)
		}
		members := map[string]string{}
		list2, err := e.app.services.Tenants.Members(e.ctx, heir, bossTenant.Slug)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range list2 {
			members[m.UserID.String()] = string(m.Role)
		}
		if members[heir.UserID.String()] != string(tenant.RoleOwner) || members[boss.UserID.String()] != string(tenant.RoleAdmin) {
			t.Fatalf("members %+v", members)
		}
	})

	t.Run("system settings drive public config and tenant creation", func(t *testing.T) {
		var sys struct {
			TenantCreation      string `json:"tenant_creation"`
			PublicRatingEnabled bool   `json:"public_rating_enabled"`
		}
		e.want(e.req(http.MethodGet, "/api/v1/admin/settings", nil, rtok), http.StatusOK, &sys)
		if sys.TenantCreation != "anyone" || !sys.PublicRatingEnabled {
			t.Fatalf("settings %+v", sys)
		}
		e.want(e.req(http.MethodPatch, "/api/v1/admin/settings", map[string]any{"tenant_creation": "admin_only", "public_rating_enabled": false}, rtok), http.StatusOK, &sys)
		if sys.TenantCreation != "admin_only" || sys.PublicRatingEnabled {
			t.Fatalf("patched %+v", sys)
		}
		var pub struct {
			TenantCreation      string `json:"tenant_creation"`
			PublicRatingEnabled bool   `json:"public_rating_enabled"`
		}
		e.want(e.req(http.MethodGet, "/api/v1/public/config", nil, ""), http.StatusOK, &pub)
		if pub.TenantCreation != "admin_only" || pub.PublicRatingEnabled {
			t.Fatalf("public config %+v", pub)
		}
		e.problem(e.req(http.MethodGet, "/api/v1/public/rating", nil, ""), http.StatusNotFound, "not_found")
		nobody := e.person(slug("nobody")+"@example.com", "Nobody")
		e.member(tn, nobody, tenant.RoleMember)
		ntok := e.token(nobody, tn)
		e.problem(e.req(http.MethodPost, "/api/v1/tenants", map[string]any{"name": "Blocked"}, ntok), http.StatusForbidden, "forbidden")
		e.want(e.req(http.MethodPatch, "/api/v1/admin/settings", map[string]any{"tenant_creation": "anyone", "public_rating_enabled": true}, rtok), http.StatusOK, nil)
	})

	t.Run("status, resync and the global audit", func(t *testing.T) {
		var st struct {
			Version    string `json:"version"`
			Components map[string]struct {
				Status string `json:"status"`
			} `json:"components"`
			Pipelines struct {
				Namespaces []map[string]any `json:"namespaces"`
			} `json:"pipelines"`
			Tenants struct {
				Active int `json:"active"`
			} `json:"tenants"`
		}
		e.want(e.req(http.MethodGet, "/api/v1/admin/status", nil, rtok), http.StatusOK, &st)
		if st.Components["postgres"].Status != "ok" || st.Components["graphene"].Status != "ok" || len(st.Pipelines.Namespaces) < 3 || st.Tenants.Active < 3 {
			t.Fatalf("status %+v", st)
		}
		var re struct {
			Namespaces []string `json:"namespaces"`
		}
		e.want(e.req(http.MethodPost, "/api/v1/admin/pipelines:resync", map[string]any{"tenant_slug": tn.Slug}, rtok), http.StatusAccepted, &re)
		if len(re.Namespaces) != 1 || re.Namespaces[0] != tn.GrapheneNamespace {
			t.Fatalf("resync %+v", re)
		}
		var audit struct {
			Data []struct {
				Action string `json:"action"`
				Actor  struct {
					Kind string `json:"kind"`
				} `json:"actor"`
			} `json:"data"`
		}
		e.want(e.req(http.MethodGet, "/api/v1/admin/audit?action=admin.tenant.suspend", nil, rtok), http.StatusOK, &audit)
		if len(audit.Data) != 1 || audit.Data[0].Actor.Kind != "admin" {
			t.Fatalf("audit %+v", audit)
		}
		e.want(e.req(http.MethodGet, "/api/v1/admin/audit?tenant="+tn.Slug, nil, rtok), http.StatusOK, &audit)
		if len(audit.Data) < 3 {
			t.Fatalf("tenant audit %d", len(audit.Data))
		}
	})
}
