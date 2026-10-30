//go:build integration

package application

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/dbkind"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/provider"
)

// TestE2EDatabaseMatrix launches every version of every topology template
// of every deployable database kind through the API — the server's
// compiler, the real pipeline, the projection and the telemetry — and
// reads what the UI shows for it: overview, logs, metrics, tree,
// artifacts. Every workload script the database's protocol supports is
// run too, as the segments of the kind's default combination, so each
// script's parameters are rendered against each database at least once.
func TestE2EDatabaseMatrix(t *testing.T) {
	e := e2eServer(t)
	// The biggest template (ydb mirror-3-dc) needs 50 vCPU.
	e.graphene.quotas = []provider.Quota{{Name: "compute.instanceCores.count", Limit: 256, Used: 4, Unit: "cores"}}
	base, tok, _, _ := runFixture(t, e)
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

	var stroppy struct {
		Versions []struct {
			Version string `json:"version"`
			Default bool   `json:"default"`
			Scripts []struct {
				ID        string   `json:"id"`
				Protocols []string `json:"protocols"`
			} `json:"scripts"`
		} `json:"versions"`
	}
	e.want(e.req(http.MethodGet, "/api/v1/catalog/stroppy", nil, tok), http.StatusOK, &stroppy)
	build := stroppy.Versions[0]
	for _, v := range stroppy.Versions {
		if v.Default {
			build = v
		}
	}

	tuned := map[string]bool{"postgres": true, "orioledb": true, "mysql": true, "mariadb": true, "cockroach": true, "ydb": true, "picodata": true}
	type matrixCase struct {
		name, kind, version, topology, protocol, stroppy string
		params                                           map[string]any
		segments                                         []any
	}
	var cases []matrixCase
	for _, db := range catalog.Data {
		if !db.Deployable {
			continue
		}
		defaultVersion := db.Versions[0].Version
		for _, v := range db.Versions {
			if v.Default {
				defaultVersion = v.Version
			}
		}
		paramsOf := func(topo map[string]any, version string) map[string]any {
			params := map[string]any{}
			for k, v := range topo {
				params[k] = v
			}
			for _, k := range []string{"version", "image_tag"} {
				if _, ok := params[k]; ok {
					params[k] = version
				}
			}
			return params
		}
		// Every version of every topology, with the shortest workload: the
		// matrix is about deploying the database.
		for _, dbVersion := range db.Versions {
			for _, topo := range db.Topologies {
				cases = append(cases, matrixCase{
					name: db.Kind + "/" + dbVersion.Version + "/" + topo.ID, kind: db.Kind, version: dbVersion.Version,
					topology: topo.ID, protocol: db.Protocols[0], stroppy: build.Version,
					params: paramsOf(topo.Params, dbVersion.Version), segments: []any{smokeSegment("smoke")},
				})
			}
		}
		// Every script on every protocol the database speaks, as the
		// segments of the kind's default combination, so each script's
		// parameters are rendered against each driver at least once.
		first := db.Topologies[0]
		protocols := db.Protocols
		// A kind whose template picks the protocol (custom) speaks only that one.
		if p, ok := first.Params["protocol"].(string); ok {
			protocols = []string{p}
		}
		for _, protocol := range protocols {
			var scripts []string
			for _, sc := range build.Scripts {
				for _, p := range sc.Protocols {
					if p == protocol {
						scripts = append(scripts, sc.ID)
						break
					}
				}
			}
			cases = append(cases, matrixCase{
				name: db.Kind + "/" + defaultVersion + "/" + first.ID + "/scripts-" + protocol, kind: db.Kind, version: defaultVersion,
				topology: first.ID, protocol: protocol, stroppy: build.Version,
				params: paramsOf(first.Params, defaultVersion), segments: scriptSegments(protocol, scripts),
			})
		}
		// The engine as shipped, untuned: every kind whose parameters carry
		// the tuning mode (pipelines/schemas/dbparams).
		if tuned[db.Kind] {
			stock := paramsOf(first.Params, defaultVersion)
			stock["tuning"] = "stock"
			cases = append(cases, matrixCase{
				name: db.Kind + "/" + defaultVersion + "/" + first.ID + "/stock", kind: db.Kind, version: defaultVersion,
				topology: first.ID, protocol: db.Protocols[0], stroppy: build.Version,
				params: stock, segments: []any{smokeSegment("smoke")},
			})
		}
		// Every Stroppy release against the kind's default database: each
		// writes its own config dialect (driver keys, report schemas).
		if db.Kind == "postgres" {
			for _, v := range stroppy.Versions {
				if v.Version == build.Version {
					continue
				}
				cases = append(cases, matrixCase{
					name: db.Kind + "/" + defaultVersion + "/" + first.ID + "/stroppy-" + v.Version, kind: db.Kind, version: defaultVersion,
					topology: first.ID, protocol: db.Protocols[0], stroppy: v.Version,
					params: paramsOf(first.Params, defaultVersion), segments: []any{smokeSegment("smoke")},
				})
			}
		}
	}
	for _, c := range cases {
		segments := runnable(c.kind, c.version, c.params, c.segments)
		if len(segments) == 0 {
			continue
		}
		t.Run(c.name, func(t *testing.T) {
			params := c.params
			var dbv, wl struct {
				ID string `json:"id"`
			}
			e.want(e.req(http.MethodPost, base+"/databases", map[string]any{"name": slug(c.kind + "-" + c.topology), "kind": c.kind, "version": c.version, "params": params}, tok), http.StatusCreated, &dbv)
			e.want(e.req(http.MethodPost, base+"/workloads", map[string]any{
				"name": slug("smoke"), "stroppy_version": c.stroppy, "protocol": c.protocol,
				"segments": segments,
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
				"name": slug(c.kind + " " + c.topology), "database": map[string]any{"ref": map[string]any{"id": dbv.ID}}, "workload": map[string]any{"ref": map[string]any{"id": wl.ID}},
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
			for _, seg := range segments {
				name := seg.(map[string]any)["name"].(string)
				if ov.segment(name) != "completed" {
					t.Fatalf("segment %s: %+v", name, ov.WorkloadSegments)
				}
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
			if len(m.Errors) != 0 || !keys["iterations_per_second"] || !keys["node_cpu_usage"] {
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
			if workload != 1 && c.kind != "noop" {
				t.Fatalf("workload edges = %d in %s", workload, r.ID)
			}
		})
	}
}

// runnable drops the segments the database version refuses (dbkind
// UnsupportedScript); a refused smoke segment becomes a short TPC-B.
func runnable(kind, version string, params map[string]any, segments []any) []any {
	out := make([]any, 0, len(segments))
	for _, s := range segments {
		seg := s.(map[string]any)
		script, _ := seg["workload"].(map[string]any)["script"].(string)
		if dbkind.UnsupportedScript(catalog.DatabaseKind(kind), version, params, script) == "" {
			out = append(out, s)
			continue
		}
		if seg["name"] == "smoke" && dbkind.UnsupportedScript(catalog.DatabaseKind(kind), version, params, "tpcb/tx") == "" {
			out = append(out, map[string]any{"name": "smoke", "workload": map[string]any{"script": "tpcb/tx", "scale_factor": 1}, "run": map[string]any{"executor": "shared-iterations", "vus": 2, "iterations": 100}})
		}
	}
	return out
}

// smokeSegment is the shortest measurable segment.
func smokeSegment(name string) any {
	return map[string]any{"name": name, "workload": map[string]any{"script": "simple"}, "run": map[string]any{"executor": "constant-vus", "vus": 2, "duration": "30s"}}
}

// scriptSegments is one segment per script, with the parameters each one
// needs and a data volume that fits the matrix's disks.
func scriptSegments(protocol string, scripts []string) []any {
	out := make([]any, 0, len(scripts))
	for _, id := range scripts {
		params := map[string]any{"script": id}
		switch id {
		case "tpch/tx", "tpcds":
			// A scale factor of 1 is a gigabyte of generated data.
			params["scale_factor"] = 0.01
		case "execute_sql":
			params["sql_body"] = "--= probe\nSELECT 1"
		}
		name := strings.NewReplacer("/", "-", "_", "-").Replace(id)
		seg := map[string]any{
			"name": name, "workload": params,
			"run": map[string]any{"executor": "shared-iterations", "vus": 2, "iterations": 100},
		}
		// Picodata loads TPC-DS but cannot query it.
		if protocol == "picodata" && id == "tpcds" {
			seg["no_steps"] = []any{"workload"}
		}
		out = append(out, seg)
	}
	return out
}
