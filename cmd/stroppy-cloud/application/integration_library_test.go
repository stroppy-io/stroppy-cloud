//go:build integration

package application

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// yandexProfile creates a ready provider profile through the API and the
// fake door, returning its id.
func (e *e2e) yandexProfile(tok, slug string) string {
	e.t.Helper()
	body := map[string]any{
		"name": "yc", "kind": "yandex",
		"settings":    map[string]any{"cloud_id": "b1gcloud000000000000", "folder_id": "b1gfolder00000000000", "zone": "ru-central1-a", "network": map[string]any{"kind": "create"}},
		"credentials": map[string]any{"sa_key_json": `{"id":"ajekey00000000000000","service_account_id":"ajesa000000000000000","created_at":"2026-01-01T00:00:00Z","key_algorithm":"RSA_2048","public_key":"-----BEGIN PUBLIC KEY-----\nMIIB\n-----END PUBLIC KEY-----\n","private_key":"-----BEGIN PRIVATE KEY-----\nMIIE\n-----END PRIVATE KEY-----\n"}`},
	}
	var created struct {
		ID string `json:"id"`
	}
	e.want(e.req(http.MethodPost, "/api/v1/t/"+slug+"/providers", body, tok), http.StatusCreated, &created)
	var got struct {
		Status string `json:"status"`
	}
	eventually(e.t, 10*time.Second, func() bool {
		e.want(e.req(http.MethodGet, "/api/v1/t/"+slug+"/providers/"+created.ID, nil, tok), http.StatusOK, &got)
		return got.Status == "ready"
	})
	return created.ID
}

func TestE2ELibrary(t *testing.T) {
	e := e2eServer(t)
	owner := e.person(slug("lib")+"@example.com", "Lib")
	tn := e.tenant(owner, "Library")
	tok := e.token(owner, tn)
	base := "/api/v1/t/" + tn.Slug
	profileID := e.yandexProfile(tok, tn.Slug)

	var dbID string
	t.Run("database: create derives topology and effective configs", func(t *testing.T) {
		var got struct {
			ID              string `json:"id"`
			TopologyPreview struct {
				Label     string `json:"label"`
				NodeCount int    `json:"node_count"`
			} `json:"topology_preview"`
			Requirements     map[string]map[string]any            `json:"requirements"`
			EffectiveConfigs map[string]map[string]map[string]any `json:"effective_configs"`
			Params           map[string]any                       `json:"params"`
		}
		e.want(e.req(http.MethodPost, base+"/databases", map[string]any{
			"name": "pg-ha", "kind": "postgres", "version": "17",
			"params":  map[string]any{"version": "17", "replicas": 2, "ha": "patroni", "haproxy": 1},
			"configs": map[string]any{"db": map[string]any{"cfg.postgresql.conf@17": map[string]any{"shared_buffers": 2048}}},
		}, tok), http.StatusCreated, &got)
		dbID = got.ID
		if got.TopologyPreview.NodeCount != 1+2+3+1+1 || !strings.Contains(got.TopologyPreview.Label, "patroni") {
			t.Fatalf("topology %+v", got.TopologyPreview)
		}
		if got.Params["etcd_nodes"] == nil || got.Requirements["db"]["cpu"] == nil {
			t.Fatalf("derived %+v", got)
		}
		if sb := got.EffectiveConfigs["db"]["cfg.postgresql.conf@17"]["shared_buffers"]; sb != float64(2048) {
			t.Fatalf("effective shared_buffers %v", sb)
		}
	})

	t.Run("database: bad params are a validation problem, bad kind is invalid", func(t *testing.T) {
		e.problem(e.req(http.MethodPost, base+"/databases", map[string]any{"name": "bad", "kind": "postgres", "version": "17", "params": map[string]any{"version": "17", "replicas": 99}}, tok), http.StatusUnprocessableEntity, "validation_failed")
		e.problem(e.req(http.MethodPost, base+"/databases", map[string]any{"name": "bad", "kind": "postgres", "version": "9", "params": map[string]any{}}, tok), http.StatusUnprocessableEntity, "invalid")
		e.problem(e.req(http.MethodPost, base+"/databases", map[string]any{"name": "pg-ha", "kind": "postgres", "version": "17", "params": map[string]any{}}, tok), http.StatusConflict, "conflict")
	})

	t.Run("database: preview, list, patch, clone, export, diff", func(t *testing.T) {
		var preview struct {
			Validation struct {
				Errors []any `json:"errors"`
			} `json:"validation"`
			TopologyPreview struct {
				Label string `json:"label"`
			} `json:"topology_preview"`
		}
		e.want(e.req(http.MethodPost, base+"/databases:preview", map[string]any{"name": "x", "kind": "mysql", "version": "8.4", "params": map[string]any{"version": "8.4", "replicas": 1, "replication": "semi_sync"}}, tok), http.StatusOK, &preview)
		if len(preview.Validation.Errors) != 0 || preview.TopologyPreview.Label != "mysql + 1 replica" {
			t.Fatalf("preview %+v", preview)
		}
		var list struct {
			Data []struct {
				Kind string `json:"kind"`
			} `json:"data"`
			Meta struct {
				HasMore bool `json:"has_more"`
			} `json:"meta"`
		}
		e.want(e.req(http.MethodGet, base+"/databases?kind=postgres&sort=name", nil, tok), http.StatusOK, &list)
		if len(list.Data) != 1 || list.Data[0].Kind != "postgres" || list.Meta.HasMore {
			t.Fatalf("list %+v", list)
		}
		var patched struct {
			Name            string `json:"name"`
			TopologyPreview struct {
				NodeCount int `json:"node_count"`
			} `json:"topology_preview"`
		}
		e.want(e.req(http.MethodPatch, base+"/databases/"+dbID, map[string]any{"name": "pg-ha-2", "params": map[string]any{"version": "17", "replicas": 1}}, tok), http.StatusOK, &patched)
		if patched.Name != "pg-ha-2" || patched.TopologyPreview.NodeCount != 3 {
			t.Fatalf("patched %+v", patched)
		}
		var clone struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		e.want(e.req(http.MethodPost, base+"/databases/"+dbID+":clone", map[string]any{"name": "pg-copy"}, tok), http.StatusCreated, &clone)
		var doc struct {
			Kind     string         `json:"kind"`
			Metadata map[string]any `json:"metadata"`
			Spec     map[string]any `json:"spec"`
		}
		e.want(e.req(http.MethodGet, base+"/databases/"+dbID+":export", nil, tok), http.StatusOK, &doc)
		if doc.Kind != "Database" || doc.Spec["kind"] != "postgres" {
			t.Fatalf("export %+v", doc)
		}
		r := e.req(http.MethodPost, base+"/databases/"+clone.ID, nil, tok) // no such op → 405/404, sanity only
		_ = r
		// A version change must also replace the explicitly selected config schema.
		e.problem(e.req(http.MethodPatch, base+"/databases/"+clone.ID, map[string]any{"params": map[string]any{"version": "16"}, "version": "16"}, tok), http.StatusUnprocessableEntity, "invalid")
		e.want(e.req(http.MethodPatch, base+"/databases/"+clone.ID, map[string]any{
			"params": map[string]any{"version": "16"}, "version": "16",
			"configs": map[string]any{"db": map[string]any{"cfg.postgresql.conf@16": map[string]any{"shared_buffers": 2048}}},
		}, tok), http.StatusOK, nil)
		var diff struct {
			Changes []struct {
				Path string `json:"path"`
				Op   string `json:"op"`
			} `json:"changes"`
		}
		e.want(e.req(http.MethodGet, base+"/databases:diff?a="+dbID+"&b="+clone.ID, nil, tok), http.StatusOK, &diff)
		paths := map[string]string{}
		for _, c := range diff.Changes {
			paths[c.Path] = c.Op
		}
		if paths["version"] != "replace" || paths["params.version"] != "replace" || paths["params.replicas"] != "replace" {
			t.Fatalf("diff %+v", diff)
		}
	})

	t.Run("database: import creates then updates by name; yaml round-trips", func(t *testing.T) {
		doc := map[string]any{"api_version": "stroppy.io/v1", "kind": "Database", "metadata": map[string]any{"name": "imported"}, "spec": map[string]any{"kind": "cockroach", "version": "25.4", "params": map[string]any{"version": "25.4", "nodes": 3}}}
		var first, second struct {
			ID   string `json:"id"`
			Kind string `json:"kind"`
		}
		e.want(e.req(http.MethodPost, base+"/databases:import", doc, tok), http.StatusOK, &first)
		e.want(e.req(http.MethodPost, base+"/databases:import", doc, tok), http.StatusOK, &second)
		if first.ID != second.ID || first.Kind != "cockroach" {
			t.Fatalf("import %+v / %+v", first, second)
		}
		r := e.reqAccept(http.MethodGet, base+"/databases/"+first.ID+":export", "application/yaml", tok)
		if r.Status != http.StatusOK || !strings.Contains(string(r.Body), "kind: Database") {
			t.Fatalf("yaml export %d %s", r.Status, r.Body)
		}
	})

	var wlID string
	t.Run("workload: create, requirements, script checks", func(t *testing.T) {
		var got struct {
			ID           string                    `json:"id"`
			Requirements map[string]map[string]any `json:"requirements"`
			Segments     []map[string]any          `json:"segments"`
		}
		e.want(e.req(http.MethodPost, base+"/workloads", map[string]any{
			"name": "tpcc", "stroppy_version": "6.0.0", "protocol": "pg",
			"segments": []any{map[string]any{"name": "main", "workload": map[string]any{"script": "tpcc/tx", "scale_factor": 10}, "run": map[string]any{"vus": 256, "duration": "5m"}}},
		}, tok), http.StatusCreated, &got)
		wlID = got.ID
		if got.Requirements["runner"]["cpu"] != float64(4) || len(got.Segments) != 1 {
			t.Fatalf("workload %+v", got)
		}
		e.problem(e.req(http.MethodPost, base+"/workloads", map[string]any{"name": "bad", "stroppy_version": "9.9.9", "protocol": "pg", "segments": []any{map[string]any{"name": "x", "workload": map[string]any{"script": "tpcc/tx"}}}}, tok), http.StatusUnprocessableEntity, "invalid")
		e.problem(e.req(http.MethodPost, base+"/workloads", map[string]any{"name": "bad", "stroppy_version": "6.0.0", "protocol": "pg", "segments": []any{map[string]any{"name": "x", "workload": map[string]any{"script": "tpcc/tx", "nope": 1}}}}, tok), http.StatusUnprocessableEntity, "validation_failed")
		var preview struct {
			SegmentsSummary []struct {
				Script string `json:"script"`
				Vus    int    `json:"vus"`
				Limit  string `json:"limit"`
			} `json:"segments_summary"`
		}
		e.want(e.req(http.MethodPost, base+"/workloads:preview", map[string]any{"name": "p", "stroppy_version": "6.0.0", "protocol": "mysql", "segments": []any{map[string]any{"name": "s", "workload": map[string]any{"script": "tpcb/tx"}, "run": map[string]any{"executor": "shared-iterations", "vus": 8, "iterations": 100}}}}, tok), http.StatusOK, &preview)
		if len(preview.SegmentsSummary) != 1 || preview.SegmentsSummary[0].Vus != 8 || preview.SegmentsSummary[0].Limit != "100 iterations" {
			t.Fatalf("preview %+v", preview)
		}
		var list struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		e.want(e.req(http.MethodGet, base+"/workloads?script=tpcc/tx&protocol=pg", nil, tok), http.StatusOK, &list)
		if len(list.Data) != 1 || list.Data[0].ID != wlID {
			t.Fatalf("list %+v", list)
		}
	})

	t.Run("test: draft → ready, fit issues with suggestions", func(t *testing.T) {
		var draft struct {
			ID         string `json:"id"`
			Status     string `json:"status"`
			Validation struct {
				Fits   bool `json:"fits"`
				Issues []struct {
					Path      string         `json:"path"`
					Code      string         `json:"code"`
					Suggested map[string]any `json:"suggested"`
				} `json:"issues"`
			} `json:"validation"`
		}
		e.want(e.req(http.MethodPost, base+"/tests", map[string]any{"name": "t1", "database": map[string]any{"ref": map[string]any{"id": dbID}}}, tok), http.StatusCreated, &draft)
		if draft.Status != "draft" || draft.Validation.Fits {
			t.Fatalf("draft %+v", draft)
		}
		codes := map[string]string{}
		for _, i := range draft.Validation.Issues {
			codes[i.Path] = i.Code
		}
		if codes["workload"] != "required" || codes["provider_profile_id"] != "required" || codes["sizes.db"] != "required" {
			t.Fatalf("issues %+v", codes)
		}
		// Sizes too small carry a suggestion.
		var patched struct {
			Status     string `json:"status"`
			Validation struct {
				Issues []struct {
					Path      string         `json:"path"`
					Code      string         `json:"code"`
					Suggested map[string]any `json:"suggested"`
				} `json:"issues"`
			} `json:"validation"`
		}
		e.want(e.req(http.MethodPatch, base+"/tests/"+draft.ID, map[string]any{
			"workload": map[string]any{"ref": map[string]any{"id": wlID}}, "provider_profile_id": profileID,
			"sizes": map[string]any{"db": map[string]any{"size": "XS"}, "db-replica": map[string]any{"size": "XS"}, "runner": map[string]any{"size": "XS"}},
		}, tok), http.StatusOK, &patched)
		var runnerIssue map[string]any
		for _, i := range patched.Validation.Issues {
			if i.Path == "sizes.runner" && i.Code == "too_small" {
				runnerIssue = i.Suggested
			}
		}
		if patched.Status != "draft" || runnerIssue == nil || runnerIssue["size"] != "S" {
			t.Fatalf("patched %+v", patched)
		}
		var ready struct {
			Status   string `json:"status"`
			Resolved struct {
				Database struct {
					Name string `json:"name"`
				} `json:"database"`
			} `json:"resolved"`
			Summary struct {
				NodeCount int `json:"node_count"`
			} `json:"summary"`
		}
		e.want(e.req(http.MethodPatch, base+"/tests/"+draft.ID, map[string]any{
			"sizes": map[string]any{"db": map[string]any{"size": "S"}, "db-replica": map[string]any{"size": "S"}, "runner": map[string]any{"size": "S"}},
		}, tok), http.StatusOK, &ready)
		if ready.Status != "ready" || ready.Resolved.Database.Name != "pg-ha-2" || ready.Summary.NodeCount != 3 {
			t.Fatalf("ready %+v", ready)
		}

		// A referenced database changing after validation → needs_attention.
		e.want(e.req(http.MethodPatch, base+"/databases/"+dbID, map[string]any{"description": "touched"}, tok), http.StatusOK, nil)
		var stale struct {
			Status     string `json:"status"`
			Validation struct {
				Stale struct {
					Database bool `json:"database"`
				} `json:"stale"`
			} `json:"validation"`
		}
		e.want(e.req(http.MethodGet, base+"/tests/"+draft.ID, nil, tok), http.StatusOK, &stale)
		if stale.Status != "needs_attention" || !stale.Validation.Stale.Database {
			t.Fatalf("stale %+v", stale)
		}

		// Deleting the referenced database is blocked unless inlined.
		e.problem(e.req(http.MethodDelete, base+"/databases/"+dbID, nil, tok), http.StatusConflict, "conflict")
		e.want(e.req(http.MethodDelete, base+"/databases/"+dbID+"?inline_usages=true", nil, tok), http.StatusNoContent, nil)
		var inlined struct {
			Database struct {
				Inline map[string]any `json:"inline"`
			} `json:"database"`
			Status string `json:"status"`
		}
		e.want(e.req(http.MethodGet, base+"/tests/"+draft.ID, nil, tok), http.StatusOK, &inlined)
		if inlined.Database.Inline["kind"] != "postgres" || inlined.Status == "draft" {
			t.Fatalf("inlined %+v", inlined)
		}
	})

	t.Run("test: validate live form, export/import, viewer cannot write", func(t *testing.T) {
		var v struct {
			Status    string `json:"status"`
			Estimated struct {
				Machines []struct {
					Role string `json:"role"`
					CPU  int    `json:"cpu"`
				} `json:"machines"`
			} `json:"estimated"`
		}
		e.want(e.req(http.MethodPost, base+"/tests:validate", map[string]any{
			"name":                "live",
			"database":            map[string]any{"inline": map[string]any{"kind": "pg_noop", "version": "0.1.2", "params": map[string]any{}}},
			"workload":            map[string]any{"ref": map[string]any{"id": wlID}},
			"provider_profile_id": profileID,
			"sizes":               map[string]any{"db": map[string]any{"size": "M"}, "runner": map[string]any{"size": "M"}},
		}, tok), http.StatusOK, &v)
		if v.Status != "ready" || len(v.Estimated.Machines) != 2 {
			t.Fatalf("validate %+v", v)
		}
		viewer := e.person(slug("viewer")+"@example.com", "V")
		e.member(tn, viewer, "viewer")
		vtok := e.token(viewer, tn)
		e.want(e.req(http.MethodGet, base+"/tests", nil, vtok), http.StatusOK, nil)
		e.problem(e.req(http.MethodPost, base+"/tests", map[string]any{"name": "nope"}, vtok), http.StatusForbidden, "forbidden")
	})
}
