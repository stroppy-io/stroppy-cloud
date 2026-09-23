//go:build integration

package application

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// launchWithTPS launches the test; its pipeline measures tps.
func launchWithTPS(t *testing.T, e *e2e, base, testID, tok string, tps float64, body map[string]any, dst *runView) {
	t.Helper()
	e.graphene.withTPS(tps)
	defer e.graphene.withTPS(0)
	e.want(e.req(http.MethodPost, base+"/tests/"+testID+":launch", body, tok), http.StatusCreated, dst)
}

// finishRun waits for a launched run to be projected to its end.
func finishRun(t *testing.T, e *e2e, base, tok, id string) runView {
	t.Helper()
	var r runView
	eventually(t, 10*time.Second, func() bool {
		e.app.services.Projector.Tick(e.ctx)
		e.want(e.req(http.MethodGet, base+"/runs/"+id, nil, tok), http.StatusOK, &r)
		return r.Status == "completed" || r.Status == "failed" || r.Status == "cancelled"
	})
	return r
}

func TestE2EShareCompareRating(t *testing.T) {
	e := e2eServer(t)
	base, tok, testID, tn := runFixture(t, e)

	var a, b runView
	launchWithTPS(t, e, base, testID, tok, 1000, map[string]any{"name": "base", "rating": map[string]any{"global": true}}, &a)
	launchWithTPS(t, e, base, testID, tok, 1100, map[string]any{"name": "candidate"}, &b)
	a, b = finishRun(t, e, base, tok, a.ID), finishRun(t, e, base, tok, b.ID)
	if a.Status != "completed" || b.Status != "completed" {
		t.Fatalf("runs %s %s", a.Status, b.Status)
	}

	t.Run("compare: verdicts and spec diff", func(t *testing.T) {
		var cmp struct {
			BaselineRunID string `json:"baseline_run_id"`
			Columns       []struct {
				RunID   string `json:"run_id"`
				Verdict struct {
					Better int `json:"better"`
					Worse  int `json:"worse"`
				} `json:"verdict"`
			} `json:"columns"`
			Metrics []struct {
				Key   string `json:"key"`
				Cells []struct {
					RunID   string  `json:"run_id"`
					Verdict string  `json:"verdict"`
					DiffPct float64 `json:"diff_pct"`
				} `json:"cells"`
			} `json:"metrics"`
			SpecDiff map[string]struct {
				Changes []map[string]any `json:"changes"`
			} `json:"spec_diff"`
		}
		e.want(e.req(http.MethodPost, base+"/compare", map[string]any{"run_ids": []string{a.ID, b.ID}}, tok), http.StatusOK, &cmp)
		if cmp.BaselineRunID != a.ID || cmp.Columns[1].Verdict.Better < 2 || cmp.Columns[1].Verdict.Worse != 0 {
			t.Fatalf("compare %+v", cmp)
		}
		var tps, lat string
		for _, m := range cmp.Metrics {
			if m.Key == "tps" {
				tps = m.Cells[1].Verdict
			}
			if m.Key == "latency_p95_ms" {
				lat = m.Cells[1].Verdict
			}
		}
		if tps != "better" || lat != "better" {
			t.Fatalf("verdicts tps=%s latency=%s", tps, lat)
		}
		if _, ok := cmp.SpecDiff["database/"+b.ID]; !ok {
			t.Fatalf("spec diff %+v", cmp.SpecDiff)
		}
		e.problem(e.req(http.MethodPost, base+"/compare", map[string]any{"run_ids": []string{a.ID}}, tok), http.StatusUnprocessableEntity, "invalid")
	})

	var share struct {
		ID     string `json:"id"`
		Token  string `json:"token"`
		URL    string `json:"url"`
		Active bool   `json:"active"`
		Scope  string `json:"scope"`
		Target struct {
			Kind string `json:"kind"`
			ID   string `json:"id"`
		} `json:"target"`
	}
	t.Run("share a run, read it anonymously, configs scope masks secrets", func(t *testing.T) {
		e.want(e.req(http.MethodPost, base+"/runs/"+a.ID+":share", map[string]any{"ttl": "24h", "scope": "configs", "title": "pg tpcc report"}, tok), http.StatusCreated, &share)
		if !share.Active || share.Target.Kind != "run" || share.Target.ID != a.ID || !strings.HasSuffix(share.URL, "/s/"+share.Token) {
			t.Fatalf("share %+v", share)
		}
		var snap struct {
			Kind       string `json:"kind"`
			Scope      string `json:"scope"`
			Title      string `json:"title"`
			TenantName string `json:"tenant_name"`
			Run        struct {
				Name     string                                `json:"name"`
				Status   string                                `json:"status"`
				Summary  struct{ Headline map[string]float64 } `json:"summary"`
				Timeline []map[string]any                      `json:"timeline"`
				Machines []map[string]any                      `json:"machines"`
				Configs  map[string]map[string]string          `json:"configs"`
				Database struct {
					Kind string `json:"kind"`
				} `json:"database"`
			} `json:"run"`
		}
		r := e.req(http.MethodGet, "/api/v1/public/share/"+share.Token, nil, "")
		e.want(r, http.StatusOK, &snap)
		if snap.Kind != "run" || snap.Scope != "configs" || snap.Run.Status != "completed" || snap.Run.Summary.Headline["tps"] != 1000 || len(snap.Run.Timeline) < 3 || len(snap.Run.Machines) != 2 {
			t.Fatalf("snapshot %+v", snap)
		}
		conf := snap.Run.Configs["db"]["cfg.postgresql.conf@17"]
		if !strings.Contains(conf, "shared_buffers") {
			t.Fatalf("configs %+v", snap.Run.Configs)
		}
		if body := string(r.Body); strings.Contains(body, "stroppy_postgres") || strings.Contains(body, "run_spec") || strings.Contains(body, "credentials") {
			t.Fatalf("snapshot leaks: %s", body[:300])
		}
		var runView struct {
			Shares []struct {
				ID string `json:"id"`
			} `json:"shares"`
		}
		e.want(e.req(http.MethodGet, base+"/runs/"+a.ID, nil, tok), http.StatusOK, &runView)
		if len(runView.Shares) != 1 || runView.Shares[0].ID != share.ID {
			t.Fatalf("run shares %+v", runView.Shares)
		}
		if r := e.req(http.MethodGet, "/api/v1/public/share/"+share.Token+"/export?format=md", nil, ""); r.Status != http.StatusOK || !strings.Contains(string(r.Body), "# pg tpcc report") {
			t.Fatalf("export %d %s", r.Status, r.Body)
		}
	})

	t.Run("share lifecycle: patch, rebuild, list, revoke", func(t *testing.T) {
		var got struct {
			Scope     string  `json:"scope"`
			ExpiresAt *string `json:"expires_at"`
			ViewCount int     `json:"view_count"`
		}
		e.want(e.req(http.MethodPatch, base+"/shares/"+share.ID, map[string]any{"scope": "overview", "ttl": ""}, tok), http.StatusOK, &got)
		if got.Scope != "overview" || got.ExpiresAt != nil || got.ViewCount != 2 {
			t.Fatalf("patched %+v", got)
		}
		e.want(e.req(http.MethodPost, base+"/shares/"+share.ID+":rebuild", nil, tok), http.StatusOK, nil)
		var list struct {
			Data []map[string]any `json:"data"`
		}
		e.want(e.req(http.MethodGet, base+"/shares?target_kind=run&active=true", nil, tok), http.StatusOK, &list)
		if len(list.Data) != 1 {
			t.Fatalf("list %+v", list)
		}
		e.problem(e.req(http.MethodGet, "/api/v1/public/share/"+share.Token+"/metrics", nil, ""), http.StatusForbidden, "forbidden")
		e.want(e.req(http.MethodDelete, base+"/shares/"+share.ID, nil, tok), http.StatusNoContent, nil)
		e.problem(e.req(http.MethodGet, "/api/v1/public/share/"+share.Token, nil, ""), http.StatusNotFound, "not_found")
		e.problem(e.req(http.MethodDelete, base+"/shares/"+share.ID, nil, tok), http.StatusConflict, "conflict")
	})

	t.Run("comparison share carries both runs", func(t *testing.T) {
		var cs struct {
			Token  string `json:"token"`
			Target struct {
				Kind   string   `json:"kind"`
				RunIds []string `json:"run_ids"`
			} `json:"target"`
		}
		e.want(e.req(http.MethodPost, base+"/compare:share", map[string]any{"run_ids": []string{a.ID, b.ID}, "title": "a vs b"}, tok), http.StatusCreated, &cs)
		if cs.Target.Kind != "comparison" || len(cs.Target.RunIds) != 2 {
			t.Fatalf("comparison share %+v", cs)
		}
		var snap struct {
			Comparison struct {
				Columns []map[string]any `json:"columns"`
			} `json:"comparison"`
		}
		e.want(e.req(http.MethodGet, "/api/v1/public/share/"+cs.Token, nil, ""), http.StatusOK, &snap)
		if len(snap.Comparison.Columns) != 2 {
			t.Fatalf("snapshot %+v", snap)
		}
	})

	t.Run("rating: tenant and public leagues", func(t *testing.T) {
		var page struct {
			Metric struct {
				Key string `json:"key"`
			} `json:"metric"`
			Leagues []string `json:"leagues"`
			Data    []struct {
				Rank       int     `json:"rank"`
				Value      float64 `json:"value"`
				RunID      string  `json:"run_id"`
				League     string  `json:"league"`
				TenantName string  `json:"tenant_name"`
				ShareToken string  `json:"share_token"`
			} `json:"data"`
		}
		e.want(e.req(http.MethodGet, base+"/rating?metric=tps", nil, tok), http.StatusOK, &page)
		if len(page.Data) != 2 || page.Data[0].RunID != b.ID || page.Data[0].Rank != 1 || page.Data[1].Value != 1000 || len(page.Leagues) != 1 {
			t.Fatalf("tenant rating %+v", page)
		}
		e.want(e.req(http.MethodGet, base+"/rating?metric=latency_p95_ms", nil, tok), http.StatusOK, &page)
		if page.Data[0].RunID != b.ID {
			t.Fatalf("lower-is-better rating %+v", page)
		}
		e.want(e.req(http.MethodGet, base+"/rating?metric=tps&league="+page.Data[0].League, nil, tok), http.StatusOK, &page)
		if len(page.Data) != 2 {
			t.Fatalf("league filter %+v", page)
		}
		e.problem(e.req(http.MethodGet, base+"/rating?metric=node_cpu_usage", nil, tok), http.StatusUnprocessableEntity, "invalid")
		// Public: only the opted-in run, named by the tenant's public name, linked to its share.
		e.want(e.req(http.MethodPatch, "/api/v1/tenants/"+tn.Slug, map[string]any{"public_name": "ACME Labs"}, tok), http.StatusOK, nil)
		var sh struct {
			Token string `json:"token"`
		}
		e.want(e.req(http.MethodPost, base+"/runs/"+a.ID+":share", map[string]any{}, tok), http.StatusCreated, &sh)
		// The global rating spans tenants (other tests opt runs in too):
		// ours is there with its public name and share link, the
		// non-opted-in candidate is not.
		e.want(e.req(http.MethodGet, "/api/v1/public/rating?metric=tps&period=7d", nil, ""), http.StatusOK, &page)
		found := false
		for _, d := range page.Data {
			if d.RunID == b.ID {
				t.Fatalf("non-opted-in run in public rating %+v", d)
			}
			if d.RunID == a.ID {
				found = d.TenantName == "ACME Labs" && d.ShareToken == sh.Token
			}
		}
		if !found {
			t.Fatalf("public rating %+v", page)
		}
	})

	t.Run("dashboard aggregates the tenant", func(t *testing.T) {
		var d struct {
			RunCounts struct {
				Total     int `json:"total"`
				Completed int `json:"completed"`
			} `json:"run_counts"`
			SuccessRate float64          `json:"success_rate"`
			RecentRuns  []map[string]any `json:"recent_runs"`
			TopResults  []map[string]any `json:"top_results"`
			Providers   []map[string]any `json:"providers"`
			Limits      struct {
				MaxConcurrentRuns int `json:"max_concurrent_runs"`
			} `json:"limits"`
		}
		e.want(e.req(http.MethodGet, base+"/dashboard", nil, tok), http.StatusOK, &d)
		if d.RunCounts.Total != 2 || d.RunCounts.Completed != 2 || d.SuccessRate != 100 || len(d.RecentRuns) != 2 || len(d.TopResults) != 2 || len(d.Providers) != 1 || d.Limits.MaxConcurrentRuns == 0 {
			t.Fatalf("dashboard %+v", d)
		}
	})
}
