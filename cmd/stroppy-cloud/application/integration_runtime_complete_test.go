//go:build integration

package application

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"testing"
)

// Check explicit settings at the real HTTP -> PostgreSQL -> compiler -> Graphene boundary.
// Added runtime fields must be represented here or explicitly classified in the acceptance map.
func TestE2ECompleteRuntimeRoundtrip(t *testing.T) {
	e := e2eServer(t)
	base, tok, testID, _ := runFixture(t, e)
	raw, err := os.ReadFile("../../../pipelines/live/tests/platform/server/acceptance/runtime-fixture.json")
	if err != nil {
		t.Fatal(err)
	}
	var execution map[string]any
	if err = json.Unmarshal(raw, &execution); err != nil {
		t.Fatal(err)
	}
	var saved struct {
		Status    string
		Execution map[string]any
	}
	e.want(e.req(http.MethodPatch, base+"/tests/"+testID, map[string]any{"execution": execution}, tok), http.StatusOK, &saved)
	e.want(e.req(http.MethodGet, base+"/tests/"+testID, nil, tok), http.StatusOK, &saved)
	if saved.Status != "ready" {
		t.Fatalf("runtime not ready: %s", saved.Status)
	}
	assertAcceptanceContains(t, "saved.execution", saved.Execution, execution)
	// Editing an unrelated field must preserve the complete runtime document.
	e.want(e.req(http.MethodPatch, base+"/tests/"+testID, map[string]any{"name": "all explicit settings"}, tok), http.StatusOK, nil)
	var r runView
	e.want(e.req(http.MethodPost, base+"/tests/"+testID+":launch", map[string]any{}, tok), http.StatusCreated, &r)
	submitted, ok := e.graphene.runOf(r.ID)
	if !ok {
		t.Fatal("not submitted")
	}
	var compiled map[string]any
	if err = json.Unmarshal(submitted.params, &compiled); err != nil {
		t.Fatal(err)
	}
	machines := compiled["machines"].([]any)
	for name, want := range execution["machines"].(map[string]any) {
		found := false
		for _, m := range machines {
			if m.(map[string]any)["name"] == name {
				assertAcceptanceContains(t, "machine."+name, m, want)
				found = true
			}
		}
		if !found {
			t.Fatal("machine missing", name)
		}
	}
	containers := compiled["containers"].([]any)
	for _, x := range execution["containers"].([]any) {
		override := x.(map[string]any)
		found := false
		for _, c := range containers {
			if c.(map[string]any)["name"] == override["name"] {
				assertAcceptanceContains(t, "container", c, override["set"])
				found = true
			}
		}
		if !found {
			t.Fatal("container missing", override["name"])
		}
	}
	assertAcceptanceContains(t, "additional_containers", containers, execution["additional_containers"])
	for _, key := range []string{"network", "host_prep", "scrapes", "flows", "workload", "observability", "result_expectations"} {
		assertAcceptanceContains(t, key, compiled[key], execution[key])
	}
	// A run is an immutable snapshot even if its source test is edited afterwards.
	e.want(e.req(http.MethodPatch, base+"/tests/"+testID, map[string]any{"execution": map[string]any{}}, tok), http.StatusOK, nil)
	var read runView
	e.want(e.req(http.MethodGet, base+"/runs/"+r.ID, nil, tok), http.StatusOK, &read)
	assertAcceptanceContains(t, "run snapshot", read.RunSpec.Values, compiled)
	finishRun(t, e, base, tok, r.ID)
}

func assertAcceptanceContains(t *testing.T, path string, got, want any) {
	t.Helper()
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			t.Fatalf("%s: wanted object, got %T", path, got)
		}
		for k, v := range w {
			assertAcceptanceContains(t, path+"."+k, g[k], v)
		}
	case []any:
		g, ok := got.([]any)
		// omitempty may omit empty lists in the compiled wire representation.
		if len(w) == 0 && (got == nil || ok && len(g) == 0) {
			return
		}
		if !ok {
			t.Fatalf("%s: wanted array, got %T", path, got)
		}
		for i, v := range w {
			found := false
			raw, _ := json.Marshal(v)
			for _, candidate := range g {
				if reflect.DeepEqual(candidate, v) {
					found = true
					break
				}
				wm, wok := v.(map[string]any)
				gm, gok := candidate.(map[string]any)
				if wok && gok {
					key := "name"
					if _, ok := wm[key]; !ok {
						key = "path"
					}
					if val, ok := wm[key]; ok && val == gm[key] {
						assertAcceptanceContains(t, fmt.Sprintf("%s[%d]", path, i), candidate, v)
						found = true
						break
					}
				}
			}
			if !found {
				t.Fatalf("%s: missing element %s in %+v", path, raw, g)
			}
		}
	default:
		// Schema-value responses encode integers as decimal strings for browsers.
		if n, ok := want.(float64); ok {
			if s, ok := got.(string); ok && fmt.Sprint(n) == s {
				return
			}
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: got %#v want %#v", path, got, want)
		}
	}
}

func TestE2EWorkloadParameterTransport(t *testing.T) {
	e := e2eServer(t)
	base, tok, testID, _ := runFixture(t, e)
	raw, err := os.ReadFile("../../../pipelines/live/tests/platform/server/acceptance/workloads-fixture.json")
	if err != nil {
		t.Fatal(err)
	}
	var segments []any
	if err = json.Unmarshal(raw, &segments); err != nil {
		t.Fatal(err)
	}
	// Detect new workload parameters rather than silently narrowing the acceptance fixture.
	schemaRaw, err := os.ReadFile("../../../web/src/schemas/workload.segment_v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err = json.Unmarshal(schemaRaw, &schema); err != nil {
		t.Fatal(err)
	}
	covered := map[string]map[string]bool{}
	for _, s := range segments {
		w := s.(map[string]any)["workload"].(map[string]any)
		script := w["script"].(string)
		if covered[script] == nil {
			covered[script] = map[string]bool{}
		}
		for k := range w {
			covered[script][k] = true
		}
	}
	for _, f := range schema["fields"].([]any) {
		field := f.(map[string]any)
		if field["name"] != "workload" {
			continue
		}
		for script, v := range field["oneOf"].(map[string]any)["variants"].(map[string]any) {
			for _, p := range v.(map[string]any)["fields"].([]any) {
				key := p.(map[string]any)["name"].(string)
				if !covered[script][key] {
					t.Fatalf("uncovered workload input: %s.%s", script, key)
				}
			}
		}
	}
	patch := map[string]any{"workload": map[string]any{"inline": map[string]any{"protocol": "pg", "stroppy_version": "6.0.0", "segments": segments}}}
	e.want(e.req(http.MethodPatch, base+"/tests/"+testID, patch, tok), http.StatusOK, nil)
	e.want(e.req(http.MethodPatch, base+"/tests/"+testID, map[string]any{"name": "every workload parameter"}, tok), http.StatusOK, nil)
	var r runView
	e.want(e.req(http.MethodPost, base+"/tests/"+testID+":launch", nil, tok), http.StatusCreated, &r)
	fr, ok := e.graphene.runOf(r.ID)
	if !ok {
		t.Fatal("not submitted")
	}
	var compiled map[string]any
	if err = json.Unmarshal(fr.params, &compiled); err != nil {
		t.Fatal(err)
	}
	assertAcceptanceContains(t, "all workloads", compiled["workload"].(map[string]any)["segments"], segments)
	finishRun(t, e, base, tok, r.ID)
}

func TestE2EInvalidExecutionNeverLaunches(t *testing.T) {
	e := e2eServer(t)
	base, tok, testID, _ := runFixture(t, e)
	for name, execution := range map[string]any{
		"unknown field":               map[string]any{"workload": map[string]any{"typo": 1}},
		"impossible disk granularity": map[string]any{"machines": map[string]any{"db-1": map[string]any{"disks": []any{map[string]any{"name": "data", "gb": 94, "type": "network-ssd-nonreplicated", "mount": "/data"}}}}},
		"unknown flow target":         map[string]any{"flows": []any{map[string]any{"from_role": "runner", "to_role": "missing", "protocol": "tcp", "port": 1234}}},
		"path traversal":              map[string]any{"containers": []any{map[string]any{"name": "db-1-postgres", "set": map[string]any{"files": []any{map[string]any{"path": "/etc/../x", "content": "x"}}}}}},
		"conflicting sql source":      map[string]any{"workload": map[string]any{"segments": []any{map[string]any{"name": "bad", "workload": map[string]any{"script": "execute_sql", "sql_body": "SELECT 1", "sql_file": "tpcc/postgres"}, "run": map[string]any{"vus": 1, "duration": "1s"}}}}},
	} {
		t.Run(name, func(t *testing.T) {
			e.want(e.req(http.MethodPatch, base+"/tests/"+testID, map[string]any{"execution": map[string]any{}}, tok), http.StatusOK, nil)
			response := e.req(http.MethodPatch, base+"/tests/"+testID, map[string]any{"execution": execution}, tok)
			if response.Status == http.StatusOK {
				var draft struct{ Status string }
				if err := json.Unmarshal(response.Body, &draft); err != nil {
					t.Fatal(err)
				}
				if draft.Status == "ready" {
					t.Fatalf("invalid input accepted: %s", response.Body)
				}
				e.want(e.req(http.MethodPost, base+"/tests/"+testID+":launch", nil, tok), http.StatusUnprocessableEntity, nil)
			} else {
				e.want(response, http.StatusUnprocessableEntity, nil)
			}
		})
	}
	var runs struct{ Data []runView }
	e.want(e.req(http.MethodGet, base+"/runs", nil, tok), http.StatusOK, &runs)
	if len(runs.Data) != 0 {
		t.Fatal("invalid input created a run")
	}
}
