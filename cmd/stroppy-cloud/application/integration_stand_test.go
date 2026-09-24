//go:build integration

package application

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"testing"
	"time"
)

/*
LOCAL STAND: the whole server, clickable, with nothing installed — the
real application over testcontainers Postgres, VictoriaLogs and
VictoriaMetrics, the Graphene door running the real pipelines on a clock,
dev-mode login. Start it with

	STROPPY_LOCAL_STAND=1 go test -tags integration ./cmd/stroppy-cloud/application \
	  -run TestLocalStand -timeout 0 -v

then open the SPA (or point the Vite dev server) at the printed address and
log in with a dev token. Runs play out `speed` times faster than real time
(STROPPY_LOCAL_STAND_SPEED, default 10); a run's `sim` label picks what goes
wrong (see parseSimLabel). Ctrl+C stops it.
*/
func TestLocalStand(t *testing.T) {
	if os.Getenv("STROPPY_LOCAL_STAND") == "" {
		t.Skip("set STROPPY_LOCAL_STAND=1 to start the local stand")
	}
	addr := envOr("STROPPY_LOCAL_STAND_ADDR", "127.0.0.1:18347")
	speed, err := strconv.ParseFloat(envOr("STROPPY_LOCAL_STAND_SPEED", "10"), 64)
	if err != nil || speed <= 0 {
		t.Fatalf("STROPPY_LOCAL_STAND_SPEED: %q", os.Getenv("STROPPY_LOCAL_STAND_SPEED"))
	}
	e := e2eServerWith(t, e2eOptions{
		Speed: speed, Addr: addr, PublicURL: "http://" + addr,
		DevUsers: []string{"dev=dev@stroppy.local=Developer", "admin=root@example.com=Admin", "viewer=viewer@stroppy.local=Viewer"},
	})
	e.graphene.labelScenarios = true
	e.app.startWorkers()
	slug := seedStand(t, e)

	fmt.Printf(`
  stroppy-cloud local stand is up

    server   http://%s   (API /api/v1, SPA /)
    tokens   dev (tenant owner) · admin (platform admin) · viewer (invited viewer)
    tenant   %s
    speed    %gx real time
    stores   VictoriaLogs %s · VictoriaMetrics %s

  pick a failure with a run label, e.g.  sim=fail-segment:main  sim=hold:segment.started  sim=tps:900
  Ctrl+C stops the stand.

`, addr, slug, speed, victoriaLogsURL, victoriaMetricsURL)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// seedStand prepares a tenant the way a person would through the UI: a
// verified provider, a database, a workload, a test, the examples, and a
// few runs in different states.
func seedStand(t *testing.T, e *e2e) string {
	t.Helper()
	tok := "dev"
	var tn struct {
		Slug string `json:"slug"`
	}
	e.want(e.req(http.MethodPost, "/api/v1/tenants", map[string]any{"name": "demo"}, tok), http.StatusCreated, &tn)
	base := "/api/v1/t/" + tn.Slug
	// A stand shows several runs at once: past the default of three.
	e.want(e.req(http.MethodPut, "/api/v1/admin/tenants/"+tn.Slug+"/limits", map[string]any{"max_concurrent_runs": 20, "max_machines_per_run": 20}, "admin"), http.StatusOK, nil)
	e.want(e.req(http.MethodPost, "/api/v1/tenants/"+tn.Slug+"/invites", map[string]any{"email": "viewer@stroppy.local", "role": "viewer"}, tok), http.StatusCreated, nil)
	var prof struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	e.want(e.req(http.MethodPost, base+"/providers", map[string]any{
		"name": "yc", "kind": "yandex",
		"settings":    map[string]any{"cloud_id": "b1gcloud000000000000", "folder_id": "b1gfolder00000000000", "zone": "ru-central1-a", "network": map[string]any{"kind": "create"}},
		"credentials": map[string]any{"sa_key_json": `{"id":"ajekey00000000000000","service_account_id":"ajesa000000000000000","created_at":"2026-01-01T00:00:00Z","key_algorithm":"RSA_2048","public_key":"-----BEGIN PUBLIC KEY-----\nMIIB\n-----END PUBLIC KEY-----\n","private_key":"-----BEGIN PRIVATE KEY-----\nMIIE\n-----END PRIVATE KEY-----\n"}`},
	}, tok), http.StatusCreated, &prof)
	eventually(t, 10*time.Second, func() bool {
		e.want(e.req(http.MethodGet, base+"/providers/"+prof.ID, nil, tok), http.StatusOK, &prof)
		return prof.Status == "ready"
	})
	for _, example := range []string{"pg-single", "tpcc-smoke", "pg-vs-cockroach"} {
		e.want(e.req(http.MethodPost, base+"/examples/"+example+":clone", map[string]any{}, tok), http.StatusCreated, nil)
	}
	var db, wl, test struct {
		ID string `json:"id"`
	}
	e.want(e.req(http.MethodPost, base+"/databases", map[string]any{"name": "pg-ha", "kind": "postgres", "version": "17", "params": map[string]any{"version": "17", "replicas": 1, "haproxy": 1}}, tok), http.StatusCreated, &db)
	e.want(e.req(http.MethodPost, base+"/workloads", map[string]any{
		"name": "tpcc 5m", "stroppy_version": "6.0.0", "protocol": "pg",
		"segments": []any{
			map[string]any{"name": "warmup", "workload": map[string]any{"script": "tpcc/tx", "scale_factor": 1}, "run": map[string]any{"vus": 4, "duration": "1m"}},
			map[string]any{"name": "main", "workload": map[string]any{"script": "tpcc/tx", "scale_factor": 1}, "run": map[string]any{"vus": 16, "duration": "5m"}},
		},
		"options": map[string]any{"baseline": map[string]any{"enabled": true, "tiers": []any{"noop"}, "quick": true}},
	}, tok), http.StatusCreated, &wl)
	e.want(e.req(http.MethodPost, base+"/tests", map[string]any{
		"name": "pg ha tpcc", "database": map[string]any{"ref": map[string]any{"id": db.ID}}, "workload": map[string]any{"ref": map[string]any{"id": wl.ID}},
		"provider_profile_id": prof.ID, "keep": "2h",
		"sizes": map[string]any{"db": map[string]any{"size": "S"}, "db-replica": map[string]any{"size": "S"}, "proxy": map[string]any{"size": "XS"}, "runner": map[string]any{"size": "S"}},
	}, tok), http.StatusCreated, &test)
	for _, launch := range []map[string]any{
		{"name": "baseline run", "labels": map[string]any{"sim": "tps:1180"}},
		{"name": "faster candidate", "labels": map[string]any{"sim": "tps:1320"}},
		{"name": "broken segment", "labels": map[string]any{"sim": "fail-segment:main"}},
		{"name": "live run (held)", "labels": map[string]any{"sim": "hold:segment.finished"}},
	} {
		e.want(e.req(http.MethodPost, base+"/tests/"+test.ID+":launch", launch, tok), http.StatusCreated, nil)
	}
	return tn.Slug
}
