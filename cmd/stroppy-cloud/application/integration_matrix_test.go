//go:build integration

package application

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/provider"
)

// TestE2EDatabaseMatrix launches every topology template of every
// deployable database kind through the API — the server's compiler, the
// real pipeline, the projection and the telemetry — and reads what the UI
// shows for it: overview, logs, metrics, tree, artifacts.
func TestE2EDatabaseMatrix(t *testing.T) {
	e := e2eServer(t)
	// The biggest template (ydb mirror-3-dc) needs 50 vCPU.
	e.graphene.quotas = []provider.Quota{{Name: "compute.instanceCores.count", Limit: 256, Used: 4, Unit: "cores"}}
	base, tok, _, tn := runFixture(t, e)
	// mirror-3-dc is 13 machines, past the default tenant ceiling of 12.
	root := e.person("root@example.com", "Root")
	rtok := e.token(root, e.tenant(root, slug("ops")))
	e.want(e.req(http.MethodPut, "/api/v1/admin/tenants/"+tn.Slug+"/limits", map[string]any{"max_machines_per_run": 20}, rtok), http.StatusOK, nil)
	var profiles struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	e.want(e.req(http.MethodGet, base+"/providers", nil, tok), http.StatusOK, &profiles)

	var catalog struct {
		Data []struct {
			Kind       string `json:"kind"`
			Deployable bool   `json:"deployable"`
			Protocols  []string
			Versions   []struct {
				Version string `json:"version"`
				Default bool   `json:"default"`
			} `json:"versions"`
			Roles []struct {
				Role string `json:"role"`
			} `json:"roles"`
			Topologies []struct {
				ID     string         `json:"id"`
				Params map[string]any `json:"params"`
			} `json:"topologies"`
		} `json:"data"`
	}
	e.want(e.req(http.MethodGet, "/api/v1/catalog/databases", nil, tok), http.StatusOK, &catalog)
	for _, db := range catalog.Data {
		if !db.Deployable {
			continue
		}
		version := db.Versions[0].Version
		for _, v := range db.Versions {
			if v.Default {
				version = v.Version
			}
		}
		for _, topo := range db.Topologies {
			t.Run(db.Kind+"/"+topo.ID, func(t *testing.T) {
				params := map[string]any{}
				for k, v := range topo.Params {
					params[k] = v
				}
				for _, k := range []string{"version", "image_tag"} {
					if _, ok := params[k]; ok {
						params[k] = version
					}
				}
				var dbv, wl struct {
					ID string `json:"id"`
				}
				e.want(e.req(http.MethodPost, base+"/databases", map[string]any{"name": slug(db.Kind + "-" + topo.ID), "kind": db.Kind, "version": version, "params": params}, tok), http.StatusCreated, &dbv)
				e.want(e.req(http.MethodPost, base+"/workloads", map[string]any{
					"name": slug("smoke"), "stroppy_version": "6.0.0", "protocol": db.Protocols[0],
					"segments": []any{map[string]any{"name": "smoke", "workload": map[string]any{"script": "simple"}, "run": map[string]any{"executor": "constant-vus", "vus": 2, "duration": "30s"}}},
				}, tok), http.StatusCreated, &wl)
				type testView struct {
					ID         string `json:"id"`
					Status     string `json:"status"`
					Validation struct {
						Issues []struct {
							Code      string         `json:"code"`
							Path      string         `json:"path"`
							Message   string         `json:"message"`
							Severity  string         `json:"severity"`
							Suggested map[string]any `json:"suggested"`
						} `json:"issues"`
					} `json:"validation"`
				}
				var test testView
				e.want(e.req(http.MethodPost, base+"/tests", map[string]any{
					"name": slug(db.Kind + " " + topo.ID), "database": map[string]any{"ref": map[string]any{"id": dbv.ID}}, "workload": map[string]any{"ref": map[string]any{"id": wl.ID}},
					"provider_profile_id": profiles.Data[0].ID, "sizes": map[string]any{},
				}, tok), http.StatusCreated, &test)
				// Take the sizes the server suggests, as the form does.
				sizes := map[string]any{}
				for _, i := range test.Validation.Issues {
					role, ok := strings.CutPrefix(i.Path, "sizes.")
					if !ok || i.Code != "required" {
						continue
					}
					if i.Suggested != nil {
						sizes[role] = i.Suggested
					} else {
						sizes[role] = map[string]any{"size": "XS"}
					}
				}
				e.want(e.req(http.MethodPatch, base+"/tests/"+test.ID, map[string]any{"sizes": sizes}, tok), http.StatusOK, &test)
				if test.Status != "ready" {
					t.Fatalf("test not ready with %v: %+v", sizes, test.Validation.Issues)
				}
				var r runView
				launched := e.req(http.MethodPost, base+"/tests/"+test.ID+":launch", map[string]any{}, tok)
				if launched.Status != http.StatusCreated {
					t.Fatalf("launch %d %s", launched.Status, launched.Body)
				}
				e.want(launched, http.StatusCreated, &r)
				if r = finishRun(t, e, base, tok, r.ID); r.Status != "completed" {
					t.Fatalf("run %s: %s", r.Status, r.StatusReason)
				}
				var ov overviewView
				e.want(e.req(http.MethodGet, base+"/runs/"+r.ID+"/overview", nil, tok), http.StatusOK, &ov)
				if len(ov.Machines) == 0 {
					t.Fatalf("no machines %+v", ov)
				}
				for _, m := range ov.Machines {
					if m.Status != "ready" {
						t.Fatalf("machine %+v", m)
					}
				}
				for _, c := range ov.Components {
					if c.Status != "ready" {
						t.Fatalf("component %+v", c)
					}
				}
				if ov.segment("smoke") != "completed" {
					t.Fatalf("segments %+v", ov.WorkloadSegments)
				}
				var m struct {
					Series []struct {
						Key string `json:"key"`
					} `json:"series"`
					Errors []map[string]string `json:"errors"`
				}
				e.want(e.req(http.MethodGet, base+"/runs/"+r.ID+"/metrics", nil, tok), http.StatusOK, &m)
				keys := map[string]bool{}
				for _, s := range m.Series {
					keys[s.Key] = true
				}
				if len(m.Errors) != 0 || !keys["tps"] || !keys["node_cpu_usage"] {
					t.Fatalf("metrics keys %v errors %v", keys, m.Errors)
				}
				var logs struct {
					Data []struct {
						Container string `json:"container"`
					} `json:"data"`
				}
				e.want(e.req(http.MethodGet, base+"/runs/"+r.ID+"/logs?limit=200", nil, tok), http.StatusOK, &logs)
				if len(logs.Data) == 0 {
					t.Fatal("no logs")
				}
				if n := artifactCount(t, e, base, tok, r.ID); n < 2 {
					t.Fatalf("artifacts %d", n)
				}
				// The run's tree is its topology: every container says what
				// it is, and a topology whose parts talk to each other says
				// so with edges between them.
				tree := runTree(t, e, base, tok, r.ID)
				containers, workload := 0, 0
				for _, n := range tree.flatten() {
					for _, f := range n.Flows {
						if f.To == "" || f.Protocol == "" {
							t.Fatalf("edge %+v of %s", f, n.Ref)
						}
					}
					switch {
					case n.Labels["container"] != "":
						containers++
						if n.Labels["kind"] == "" {
							t.Fatalf("container without a kind: %+v", n.Labels)
						}
					case strings.HasPrefix(n.Ref, "agent/") && n.Labels["role"] == "runner":
						// Whatever the topology, the workload reaches the
						// database (or its proxy) from the runner's agent.
						workload += len(n.Flows)
					}
				}
				if containers == 0 {
					t.Fatalf("no containers in the tree of %s", r.ID)
				}
				if workload != 1 && db.Kind != "noop" {
					t.Fatalf("workload edges = %d in %s", workload, r.ID)
				}
			})
		}
	}
}
