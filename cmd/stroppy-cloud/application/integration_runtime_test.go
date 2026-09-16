//go:build integration

package application

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// Real PostgreSQL persistence + HTTP contract + compiler; Graphene is fake.
func TestE2ERuntimeOverrides(t *testing.T) {
	e := e2eServer(t)
	base, tok, testID, _ := runFixture(t, e)
	runtime := map[string]any{"containers": []any{map[string]any{"role": "db", "set": map[string]any{"env": map[string]any{"RUNTIME_MARKER": "saved"}}}}}
	var database struct {
		ID      string          `json:"id"`
		Runtime json.RawMessage `json:"runtime"`
	}
	e.want(e.req(http.MethodPost, base+"/databases", map[string]any{"name": "custom-pg", "kind": "postgres", "version": "17", "params": map[string]any{}, "runtime": runtime}, tok), http.StatusCreated, &database)
	e.want(e.req(http.MethodGet, base+"/databases/"+database.ID, nil, tok), http.StatusOK, &database)
	if len(database.Runtime) == 0 {
		t.Fatal("database runtime not persisted")
	}
	execution := map[string]any{"machines": map[string]any{"db-1": map[string]any{"cpu": 8, "memory_gb": 32, "preemptible": false, "boot_disk": map[string]any{"gb": 80, "type": "network-ssd"}}}}
	var saved struct {
		Status     string                     `json:"status"`
		Execution  json.RawMessage            `json:"execution"`
		Sizes      map[string]json.RawMessage `json:"sizes"`
		Validation any                        `json:"validation"`
	}
	e.want(e.req(http.MethodPatch, base+"/tests/"+testID, map[string]any{"database": map[string]any{"ref": map[string]any{"id": database.ID}}, "execution": execution, "sizes": map[string]any{"db": map[string]any{"size": "S", "machine": map[string]any{"cpu": 4, "memory_gb": 16}}, "runner": map[string]any{"size": "S"}}}, tok), http.StatusOK, &saved)
	e.want(e.req(http.MethodGet, base+"/tests/"+testID, nil, tok), http.StatusOK, &saved)
	if saved.Status != "ready" || len(saved.Execution) == 0 {
		t.Fatalf("saved test did not preserve executable runtime: %+v", saved)
	}
	var launched runView
	e.want(e.req(http.MethodPost, base+"/tests/"+testID+":launch", map[string]any{}, tok), http.StatusCreated, &launched)
	fr, ok := e.graphene.runOf(launched.ID)
	if !ok {
		t.Fatal("run not submitted")
	}
	var r spec.Run
	if err := json.Unmarshal(fr.params, &r); err != nil {
		t.Fatal(err)
	}
	m := r.MachineByName()["db-1"]
	if m.CPU != 8 || m.MemoryGB != 32 || m.BootDisk == nil || m.BootDisk.GB != 80 || m.Preemptible == nil || *m.Preemptible {
		t.Fatalf("compiled runtime differs: %+v", m)
	}
	found := false
	for _, c := range r.Containers {
		if c.Env["RUNTIME_MARKER"] == "saved" {
			found = true
		}
	}
	if !found {
		t.Fatal("database runtime lost before Graphene")
	}
	// An unmatched target must become an invalid draft before a launch is attempted.
	e.want(e.req(http.MethodPatch, base+"/tests/"+testID, map[string]any{"execution": map[string]any{"machines": map[string]any{"missing-vm": map[string]any{"cpu": 8}}}}, tok), http.StatusOK, &saved)
	if saved.Status == "ready" {
		t.Fatal("unmatched machine silently marked ready")
	}
}
