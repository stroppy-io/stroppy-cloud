//go:build integration

package application

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

func contractFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile("../../../pipelines/live/tests/platform/contracts/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestE2EContractHandoff(t *testing.T) {
	e := e2eServer(t)
	base, tok, testID, _ := runFixture(t, e)
	patch := contractFixture(t, "test-patch.json")
	e.want(e.req(http.MethodPatch, base+"/tests/"+testID, json.RawMessage(patch), tok), http.StatusOK, nil)
	saved := e.req(http.MethodGet, base+"/tests/"+testID, nil, tok)
	e.want(saved, http.StatusOK, nil)
	if !bytes.Contains(saved.Body, []byte(`"18446744073709551615"`)) {
		t.Fatal("seed response is not an exact browser-safe string")
	}
	// Exercise the actual TypeScript request boundary against a real HTTP response.
	module, err := filepath.Abs("../../../web/src/api/json.ts")
	if err != nil {
		t.Fatal(err)
	}
	quoted, _ := json.Marshal("file://" + module)
	program := `import {readFileSync} from 'node:fs'; import {parseSchemaJSON,stringifyRequest} from ` + string(quoted) + `;
const saved=parseSchemaJSON(readFileSync(0,'utf8')); process.stdout.write(stringifyRequest({name:'edited by client',execution:saved.execution}));`
	cmd := exec.CommandContext(t.Context(), "node", "--experimental-strip-types", "--input-type=module", "-e", program)
	cmd.Stdin = bytes.NewReader(saved.Body)
	edited, err := cmd.Output()
	if err != nil {
		t.Fatalf("client roundtrip: %v", err)
	}
	e.want(e.req(http.MethodPatch, base+"/tests/"+testID, json.RawMessage(edited), tok), http.StatusOK, nil)
	var launched runView
	e.want(e.req(http.MethodPost, base+"/tests/"+testID+":launch", map[string]any{}, tok), http.StatusCreated, &launched)
	fr, ok := e.graphene.runOf(launched.ID)
	if !ok {
		t.Fatal("compiled run not submitted")
	}
	var compiled spec.Run
	if err := json.Unmarshal(fr.params, &compiled); err != nil {
		t.Fatal(err)
	}
	var segment spec.Segment
	if err := json.Unmarshal(compiled.Workload.Segments[0], &segment); err != nil {
		t.Fatal(err)
	}
	segmentJSON := string(compiled.Workload.Segments[0])
	if !strings.Contains(segmentJSON, "18446744073709551615") || !strings.Contains(segmentJSON, `"error_rate":0`) {
		t.Fatal("native seed or explicit zero lost")
	}
	found := map[string]string{}
	for _, c := range compiled.Containers {
		if c.Name == "db-1-postgres" {
			if c.Healthcheck != nil {
				t.Fatal("explicit null healthcheck lost")
			}
			for _, f := range c.Files {
				found[f.Path] = f.Content
			}
		}
	}
	if text, ok := found["/etc/empty-example.conf"]; !ok || text != "" {
		t.Fatal("empty file disappeared")
	}
	if found["/etc/contract-example.conf"] != "# exact content\nvalue = '$literal'\nempty = \"\"\n" {
		t.Fatal("config text changed")
	}
	m := compiled.MachineByName()["db-1"]
	if m.Preemptible == nil || *m.Preemptible {
		t.Fatal("explicit false lost")
	}
	// Project the complete native result through the observer, PostgreSQL and HTTP.
	result := contractFixture(t, "result-complete.json")
	e.graphene.finish(launched.ID, "run-failed", "failed", json.RawMessage(result))
	if e.app.services.Projector.Tick(e.ctx) < 1 {
		t.Fatal("projector did not start")
	}
	var view struct {
		Result json.RawMessage `json:"result"`
	}
	eventually(t, 10*time.Second, func() bool {
		e.want(e.req(http.MethodGet, base+"/runs/"+launched.ID, nil, tok), http.StatusOK, &view)
		return len(view.Result) > 0
	})
	var got, want any
	if err := json.Unmarshal(view.Result, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(bytes.ReplaceAll(result, []byte(`"canceled"`), []byte(`"cancelled"`)), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("HTTP result lost native fields: %s", view.Result)
	}
	// Failed preflight keeps exact paths and warnings through Fit and launch Problem.
	bad := json.RawMessage(`{"execution":{"machines":{"db-1":{"boot_disk":{"gb":10,"type":"network-ssd"},"disks":[{"name":"data","gb":10,"type":"network-ssd","mount":"/data"}]}},"workload":{"segments":[{"name":"huge","workload":{"script":"tpcc/tx","scale_factor":100000},"run":{"executor":"shared-iterations","vus":1,"iterations":1}}]}}}`)
	var fit struct {
		Validation struct {
			Issues []spec.ResourceIssue `json:"issues"`
		} `json:"validation"`
	}
	e.want(e.req(http.MethodPatch, base+"/tests/"+testID, bad, tok), http.StatusOK, &fit)
	var problem struct {
		Validation struct {
			Errors []spec.ResourceIssue `json:"errors"`
		} `json:"validation"`
	}
	e.want(e.req(http.MethodPost, base+"/tests/"+testID+":launch", map[string]any{}, tok), http.StatusUnprocessableEntity, &problem)
	for _, issues := range [][]spec.ResourceIssue{fit.Validation.Issues, problem.Validation.Errors} {
		found := false
		for _, i := range issues {
			if i.Code == "capacity_insufficient" && i.Scope == "run_spec" && strings.HasPrefix(i.Path, "machines[") && i.Severity == "ERROR" {
				found = true
			}
		}
		if !found {
			t.Fatalf("capacity diagnostic collapsed: %+v", issues)
		}
	}
}
