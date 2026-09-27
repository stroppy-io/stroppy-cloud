//go:build integration

package application

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestE2EPortableLibrary(t *testing.T) {
	e := e2eServer(t)
	base, tok, testID, _ := runFixture(t, e)
	var original struct {
		Workload struct{ Ref struct{ ID string } }
	}
	e.want(e.req(http.MethodGet, base+"/tests/"+testID, nil, tok), http.StatusOK, &original)
	var suite struct{ ID string }
	e.want(e.req(http.MethodPost, base+"/suites", map[string]any{"name": "portable-suite", "tests": []any{map[string]any{"ref": map[string]any{"id": testID}}}, "concurrency": 1}, tok), http.StatusCreated, &suite)
	for _, item := range []struct{ kind, id string }{{"workloads", original.Workload.Ref.ID}, {"tests", testID}, {"suites", suite.ID}} {
		t.Run(item.kind, func(t *testing.T) {
			path := base + "/" + item.kind
			var clone struct{ ID, Name string }
			e.want(e.req(http.MethodPost, path+"/"+item.id+":clone", map[string]any{"name": "portable-" + item.kind}, tok), http.StatusCreated, &clone)
			if clone.ID == item.id || clone.Name != "portable-"+item.kind {
				t.Fatalf("clone not independent: %+v", clone)
			}
			e.want(e.req(http.MethodPatch, path+"/"+clone.ID, map[string]any{"description": "roundtrip"}, tok), http.StatusOK, nil)
			var document map[string]any
			e.want(e.req(http.MethodGet, path+"/"+clone.ID+":export", nil, tok), http.StatusOK, &document)
			document["metadata"].(map[string]any)["name"] = "imported-" + item.kind
			var imported struct{ ID, Description string }
			e.want(e.req(http.MethodPost, path+":import", document, tok), http.StatusOK, &imported)
			if imported.ID == "" || imported.ID == clone.ID {
				t.Fatal("import reused a distinct named object")
			}
			e.want(e.req(http.MethodGet, path+"/"+imported.ID, nil, tok), http.StatusOK, &imported)
			if imported.Description != "roundtrip" {
				t.Fatal("export/import lost metadata")
			}
			if item.kind != "suites" {
				var diff struct{ Changes []any }
				e.want(e.req(http.MethodGet, path+":diff?a="+clone.ID+"&b="+item.id, nil, tok), http.StatusOK, &diff)
			}
			e.want(e.req(http.MethodDelete, path+"/"+imported.ID, nil, tok), http.StatusNoContent, nil)
			e.problem(e.req(http.MethodGet, path+"/"+imported.ID, nil, tok), http.StatusNotFound, "not_found")
		})
	}
	var sr suiteRunView
	e.want(e.req(http.MethodPost, base+"/suites/"+suite.ID+":launch", nil, tok), http.StatusCreated, &sr)
	eventually(t, 10*time.Second, func() bool {
		e.app.services.SuiteProj.Tick(e.ctx)
		e.app.services.Projector.Tick(e.ctx)
		e.want(e.req(http.MethodGet, base+"/suite-runs/"+sr.ID, nil, tok), http.StatusOK, &sr)
		return sr.Status == "completed" && sr.Progress.Done == 1
	})
	var listed struct{ Data []suiteRunView }
	e.want(e.req(http.MethodGet, base+"/suites/"+suite.ID+"/runs", nil, tok), http.StatusOK, &listed)
	if len(listed.Data) != 1 || listed.Data[0].ID != sr.ID {
		t.Fatal("suite history lost launched run")
	}
	var sh struct {
		ID     string
		Target struct{ Kind string }
	}
	e.want(e.req(http.MethodPost, base+"/suite-runs/"+sr.ID+":share", map[string]any{"scope": "overview"}, tok), http.StatusCreated, &sh)
	e.want(e.req(http.MethodGet, base+"/shares/"+sh.ID, nil, tok), http.StatusOK, &sh)
	if sh.Target.Kind != "suite_run" {
		t.Fatal("wrong shared target")
	}
	var schedule struct {
		ID, Name, Cron string
		Enabled        bool
	}
	e.want(e.req(http.MethodPost, base+"/schedules", map[string]any{"name": "portable schedule", "target": map[string]any{"kind": "test", "id": testID}, "cron": "0 0 * * *", "enabled": false}, tok), http.StatusCreated, &schedule)
	e.want(e.req(http.MethodPatch, base+"/schedules/"+schedule.ID, map[string]any{"name": "edited schedule", "cron": "0 1 * * *"}, tok), http.StatusOK, &schedule)
	if schedule.Cron != "0 1 * * *" || schedule.Enabled {
		t.Fatal("schedule patch changed enable state")
	}
	e.want(e.req(http.MethodPut, base+"/favorites/test/"+testID, nil, tok), http.StatusNoContent, nil)
	e.want(e.req(http.MethodDelete, base+"/favorites/test/"+testID, nil, tok), http.StatusNoContent, nil)
	e.want(e.req(http.MethodGet, base+"/webhooks", nil, tok), http.StatusOK, nil)
}

func TestE2EIdentityHTTP(t *testing.T) {
	suffix := slug("identity")
	ownerEmail := "root@example.com"
	guestEmail := suffix + "@example.com"
	e := e2eServerWith(t, e2eOptions{DevUsers: []string{"owner=" + ownerEmail, "guest=" + guestEmail}})
	var public struct {
		Databases []struct {
			Kind, Title          string
			Versions, Topologies []string
		}
		Providers []string
	}
	e.want(e.req(http.MethodGet, "/api/v1/public/catalog", nil, ""), http.StatusOK, &public)
	if len(public.Databases) != len(e.app.services.Catalog.Databases) || len(public.Providers) != len(e.app.services.Catalog.Providers) {
		t.Fatal("public catalog omitted entries")
	}
	for i, d := range e.app.services.Catalog.Databases {
		p := public.Databases[i]
		if p.Kind != string(d.Kind) || p.Title != d.Title || len(p.Versions) != len(d.Versions) || len(p.Topologies) != len(d.Topologies) {
			t.Fatalf("public catalog identity mismatch for %s", d.Kind)
		}
		for j, v := range d.Versions {
			if p.Versions[j] != v.Version {
				t.Fatal("public version mismatch")
			}
		}
		for j, v := range d.Topologies {
			if p.Topologies[j] != v.ID {
				t.Fatal("public topology mismatch")
			}
		}
	}
	e.want(e.req(http.MethodGet, "/api/v1/tenants/suggest-name", nil, "owner"), http.StatusOK, nil)
	e.want(e.req(http.MethodGet, "/api/v1/me", nil, "guest"), http.StatusOK, nil)
	for _, action := range []string{"accept", "decline"} {
		var tn struct{ ID, Slug string }
		e.want(e.req(http.MethodPost, "/api/v1/tenants", map[string]any{"name": suffix + action}, "owner"), http.StatusCreated, &tn)
		var invite struct{ ID string }
		e.want(e.req(http.MethodPost, "/api/v1/tenants/"+tn.Slug+"/invites", map[string]any{"email": guestEmail, "role": "viewer"}, "owner"), http.StatusCreated, &invite)
		var invitations struct{ Data []map[string]any }
		e.want(e.req(http.MethodGet, "/api/v1/me/invites", nil, "guest"), http.StatusOK, &invitations)
		if len(invitations.Data) == 0 {
			t.Fatal("invite absent")
		}
		status := http.StatusOK
		if action == "decline" {
			status = http.StatusNoContent
		}
		e.want(e.req(http.MethodPost, "/api/v1/me/invites/"+invite.ID+":"+action, nil, "guest"), status, nil)
		if action == "accept" {
			var token struct{ ID, Secret string }
			e.want(e.req(http.MethodPost, "/api/v1/me/tokens", map[string]any{"name": "personal", "tenant_id": tn.ID}, "guest"), http.StatusCreated, &token)
			if token.Secret == "" {
				t.Fatal("token not returned once")
			}
			e.want(e.req(http.MethodGet, "/api/v1/tenants/"+tn.Slug, nil, token.Secret), http.StatusOK, nil)
			e.problem(e.req(http.MethodDelete, "/api/v1/tenants/"+tn.Slug, nil, token.Secret), http.StatusForbidden, "forbidden")
		}
		e.want(e.req(http.MethodDelete, "/api/v1/admin/tenants/"+tn.Slug, nil, "owner"), http.StatusNoContent, nil)
	}
}

func TestE2ESharedMetrics(t *testing.T) {
	e := e2eServer(t)
	base, tok, testID, _ := runFixture(t, e)
	var a, b runView
	launchWithTPS(t, e, base, testID, tok, 1111, map[string]any{}, &a)
	launchWithTPS(t, e, base, testID, tok, 2222, map[string]any{}, &b)
	finishRun(t, e, base, tok, a.ID)
	finishRun(t, e, base, tok, b.ID)
	var sh struct{ ID, Token string }
	e.want(e.req(http.MethodPost, base+"/runs/"+a.ID+":share", map[string]any{"scope": "metrics"}, tok), http.StatusCreated, &sh)
	path := "/api/v1/public/share/" + sh.Token + "/metrics"
	var metrics struct {
		Series []struct {
			Key    string
			Points [][]float64
		}
		Errors []any
	}
	e.want(e.req(http.MethodGet, path, nil, ""), http.StatusOK, &metrics)
	if len(metrics.Errors) != 0 {
		t.Fatalf("public metrics errors: %v", metrics.Errors)
	}
	found := false
	for _, s := range metrics.Series {
		if s.Key == "tps" {
			for _, p := range s.Points {
				if p[1] != 1111 {
					t.Fatal("foreign run metrics exposed")
				}
				found = true
			}
		}
	}
	if !found {
		t.Fatal("public metric series missing")
	}
	e.problem(e.req(http.MethodGet, path+"?start=2099-01-01T00:00:00Z&end=2099-01-02T00:00:00Z", nil, ""), http.StatusUnprocessableEntity, "invalid")
	e.problem(e.req(http.MethodPost, "/api/v1/public/share/"+sh.Token+"/grafana-session", nil, ""), http.StatusServiceUnavailable, "unavailable")
	e.want(e.req(http.MethodPatch, base+"/shares/"+sh.ID, map[string]any{"scope": "overview"}, tok), http.StatusOK, nil)
	e.problem(e.req(http.MethodGet, path, nil, ""), http.StatusForbidden, "forbidden")
	e.want(e.req(http.MethodPatch, base+"/shares/"+sh.ID, map[string]any{"scope": "metrics"}, tok), http.StatusOK, nil)
	e.want(e.req(http.MethodDelete, base+"/shares/"+sh.ID, nil, tok), http.StatusNoContent, nil)
	response := e.req(http.MethodGet, path, nil, "")
	if response.Status != http.StatusNotFound && response.Status != http.StatusGone {
		t.Fatalf("revoked link: %d", response.Status)
	}
	if strings.Contains(string(response.Body), "1111") {
		t.Fatal("revoked link leaked data")
	}
}
