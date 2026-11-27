package stroppycfg

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	schemapb "github.com/gopherex/schemapb/go/schemapb"

	workloadschema "github.com/stroppy-io/stroppy-cloud/pipelines/schemas/workload"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// The probe fixture is produced by the actual Stroppy binary, not by these
// schemas. Both directions must agree, including workloads with no parameters.
func TestStroppyParameterContract(t *testing.T) {
	raw, err := os.ReadFile("testdata/stroppy-probe.json")
	if err != nil {
		t.Fatal(err)
	}
	var probe struct {
		Workloads []struct {
			Name   string
			Params []struct {
				Name, Scope, Config string
				Default             any
			}
			Steps []struct {
				Name     string
				Measured bool
			}
		}
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		t.Fatal(err)
	}
	var variants map[string]*schemapb.Schema
	var runFields []*schemapb.Schema_Field
	for _, f := range workloadschema.Segment().GetFields() {
		if f.GetName() == "workload" {
			variants = f.GetOneOf().GetVariants()
		}
		if f.GetName() == "run" {
			runFields = f.GetObject().GetSchema().GetFields()
		}
	}
	seen := map[string]bool{}
	for _, upstream := range probe.Workloads {
		t.Run(upstream.Name, func(t *testing.T) {
			seen[upstream.Name] = true
			schema, ok := variants[upstream.Name]
			if !ok {
				t.Fatal("executable workload missing from schema")
			}
			mode := workloadschema.Modes(upstream.Name)[0]
			var steps []string
			for _, s := range upstream.Steps {
				steps = append(steps, s.Name)
				if s.Measured != workloadschema.Measured(upstream.Name, mode, s.Name) {
					t.Errorf("step %s measured=%t, schema disagrees", s.Name, s.Measured)
				}
			}
			if !slices.Equal(steps, workloadschema.Steps(upstream.Name, mode)) {
				t.Errorf("steps %v, schema says %v", steps, workloadschema.Steps(upstream.Name, mode))
			}
			fields := map[string]bool{}
			for _, f := range schema.GetFields() {
				if f.GetName() != "script" {
					fields["workload/"+lowerCamel(f.GetName())] = true
				}
			}
			for _, f := range runFields {
				fields["run/"+lowerCamel(f.GetName())] = true
			}
			for _, p := range upstream.Params {
				key := p.Scope + "/" + p.Config
				if !fields[key] {
					t.Errorf("executable parameter is not expressible: %s", key)
				}
				delete(fields, key)
				if p.Scope == "workload" {
					got, err := Params(spec.Segment{Workload: spec.WorkloadParams{Script: upstream.Name, Params: map[string]any{strings.ReplaceAll(p.Name, "-", "_"): p.Default}}})
					if err != nil {
						t.Fatal(err)
					}
					if p.Default != nil && p.Default != "" && !reflect.DeepEqual(got[p.Config], p.Default) {
						t.Errorf("parameter %s lost in renderer: %v", p.Name, got)
					}
				}
			}
			if len(fields) > 0 {
				t.Errorf("schema declares non-executable parameters: %v", fields)
			}
		})
	}
	for script := range variants {
		if !seen[script] {
			t.Errorf("schema workload missing from Stroppy: %s", script)
		}
	}
}

func bakeSegment(t *testing.T, value map[string]any) spec.Segment {
	t.Helper()
	e, err := schemapb.Compile(workloadschema.Segment())
	if err != nil {
		t.Fatal(err)
	}
	_, result, err := e.Bake(value)
	if err != nil || result.Blocking() {
		t.Fatalf("bake: %v %v", err, result)
	}
	// Bake canonicalizes durations to nanoseconds; spec.Duration accepts them.
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var seg spec.Segment
	if err := json.Unmarshal(raw, &seg); err != nil {
		t.Fatal(err)
	}
	return seg
}

func TestAnalyticalDefaultsAndExplicitZero(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		wl := map[string]any{"script": "tpcds"}
		if explicit {
			wl["query_stream"] = int64(0)
		}
		seg := bakeSegment(t, map[string]any{"name": "analytics", "workload": wl, "run": map[string]any{"executor": "shared-iterations", "iterations": int64(1)}, "thresholds": map[string]any{"error_rate": 0.0}})
		cfg, err := Config(Input{Segment: seg, Workload: spec.Workload{DriverType: "noop", URL: "noop://localhost"}})
		if err != nil {
			t.Fatal(err)
		}
		_, present := cfg["params"].(map[string]any)["queryStream"]
		if present != explicit {
			t.Fatalf("queryStream presence=%v, explicit=%v", present, explicit)
		}
		if seg.Thresholds.ErrorRate == nil || *seg.Thresholds.ErrorRate != 0 {
			t.Fatal("zero error threshold lost")
		}
		summary := Summary{Metrics: map[string]spec.MetricValue{"iterations_total": {Value: 10}, "failed_iterations_total": {Value: 1}}}
		if ThresholdViolation(seg.Thresholds, summary) == "" {
			t.Fatal("zero threshold did not reject errors")
		}
	}
}

// A mode changes which steps a script runs. The modes fixture is Stroppy's
// own resolved graph of every mode the schema knows (check_contract.py).
func TestStroppyModeStepContract(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("testdata/stroppy-probe-modes.json")
	if err != nil {
		t.Fatal(err)
	}
	var probed map[string][]struct {
		Name     string `json:"name"`
		Measured bool   `json:"measured"`
	}
	if err := json.Unmarshal(raw, &probed); err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, key := range workloadschema.Scripts() {
		script := string(key)
		for _, mode := range workloadschema.Modes(script) {
			if mode == "" {
				continue
			}
			upstream, ok := probed[script+" "+mode]
			if !ok {
				t.Errorf("%s %s: mode not probed", script, mode)
				continue
			}
			seen++
			var steps []string
			for _, s := range upstream {
				steps = append(steps, s.Name)
				if s.Measured != workloadschema.Measured(script, mode, s.Name) {
					t.Errorf("%s %s: step %s measured=%t, schema disagrees", script, mode, s.Name, s.Measured)
				}
			}
			if got := workloadschema.Steps(script, mode); !slices.Equal(steps, got) {
				t.Errorf("%s %s: steps %v, schema says %v", script, mode, steps, got)
			}
		}
	}
	if seen != len(probed) {
		t.Errorf("fixture has %d modes, schema %d", len(probed), seen)
	}
}

// Opt-in conformance against the real executable, always on noop and without
// setup/data generation or cloud calls. This proves parsing and dispatch, not DB
// dialect correctness. CI still checks the independent probe fixture above.
func TestStroppyExecutableContract(t *testing.T) {
	bin := os.Getenv("STROPPY_CONTRACT_BINARY")
	if bin == "" {
		t.Skip("set STROPPY_CONTRACT_BINARY to run local Stroppy conformance")
	}
	for _, key := range workloadschema.Scripts() {
		script := string(key)
		t.Run(script, func(t *testing.T) {
			wl := map[string]any{"script": script}
			if script == "execute_sql" {
				wl["sql_body"] = "SELECT 1"
			}
			seg := bakeSegment(t, map[string]any{"name": "contract", "workload": wl, "run": map[string]any{"executor": "shared-iterations", "iterations": int64(1)}, "steps": []any{measuredStep(script)}, "seed": uint64(42)})
			cfg, err := MarshalConfig(Input{Segment: seg, Workload: spec.Workload{DriverType: "noop", URL: "noop://localhost"}})
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), ConfigFile)
			if err := os.WriteFile(path, cfg, 0o600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
			defer cancel()
			cmd := exec.CommandContext(ctx, bin, "run", "-f", path)
			cmd.WaitDelay = 5 * time.Second
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("Stroppy rejected rendered input: %v\n%s", err, out)
			}
			if !ParseOutput(out).Found {
				t.Fatalf("no summary: %s", out)
			}
		})
	}
}

// The step graphs of the 6.5.0 modes accept what the schema renders, on noop:
// strict TPC-C writes its Delivery record where DeliveryResults says, the
// TPC-DS query mode runs its own steps. Named TPC-H modes refuse noop; their
// config must pass Stroppy's definition and fail only on the closed port.
func TestStroppyExecutableModes(t *testing.T) {
	bin := os.Getenv("STROPPY_CONTRACT_BINARY")
	if bin == "" {
		t.Skip("set STROPPY_CONTRACT_BINARY to run local Stroppy conformance")
	}
	once := map[string]any{"executor": "shared-iterations", "iterations": int64(1)}
	for _, tc := range []struct {
		name     string
		workload map[string]any
		run      map[string]any
		steps    []any
	}{
		{"tpcc strict", map[string]any{"script": "tpcc/procs", "terminal_model": "strict", "delivery_results": "delivery.jsonl"},
			map[string]any{"executor": "constant-vus", "vus": int64(2), "duration": "3s"},
			[]any{"create_terminal_procedures", "initialize_terminals", "workload", "delivery_results"}},
		{"tpch full", map[string]any{"script": "tpch/tx", "mode": "full", "scale_factor": 10.0}, once, nil},
		{"tpcds queries", map[string]any{"script": "tpcds", "mode": "queries"}, once, []any{"workload"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := map[string]any{"name": "contract", "workload": tc.workload, "run": tc.run, "seed": uint64(42)}
			if tc.steps != nil {
				doc["steps"] = tc.steps
			}
			seg := bakeSegment(t, doc)
			driver := spec.Workload{DriverType: "noop", URL: "noop://localhost"}
			if seg.Workload.Script == "tpch/tx" {
				driver = spec.Workload{DriverType: "postgres", URL: "postgres://stroppy:stroppy@127.0.0.1:1/stroppy?connect_timeout=1"}
			}
			cfg, err := MarshalConfig(Input{Segment: seg, Workload: driver})
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			path := filepath.Join(dir, ConfigFile)
			if err := os.WriteFile(path, cfg, 0o600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
			defer cancel()
			cmd := exec.CommandContext(ctx, bin, "run", "-f", path)
			cmd.Dir = dir
			cmd.WaitDelay = 5 * time.Second
			out, err := cmd.CombinedOutput()
			if seg.Workload.Script == "tpch/tx" {
				if strings.Contains(string(out), `define "tpch/tx"`) {
					t.Fatalf("Stroppy rejected rendered input:\n%s", out)
				}
				return
			}
			if err != nil {
				t.Fatalf("Stroppy rejected rendered input: %v\n%s", err, out)
			}
			if name := DeliveryResults(seg); name != "" {
				if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
					t.Fatalf("no Delivery record %s: %v\n%s", name, err, out)
				}
			}
		})
	}
}

// Independently generated by Stroppy's Go config types, including native
// driver subobjects. Snake-case aliases refer to the same executable option.
func TestNativeDriverFieldContract(t *testing.T) {
	raw, err := os.ReadFile("testdata/stroppy-run.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var upstream struct {
		Defs map[string]struct {
			Properties map[string]any `json:"properties"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(raw, &upstream); err != nil {
		t.Fatal(err)
	}
	native := workloadschema.NativeDriver()
	objects := map[string]*schemapb.Schema{"DriverRunConfig": native}
	names := map[string]string{"pool": "PoolConfig", "postgres": "PostgresConfig", "sql": "SQLConfig", "insertProgress": "InsertProgressConfig", "transactions": "TransactionConfig"}
	for _, f := range native.GetFields() {
		if name := names[f.GetName()]; name != "" {
			objects[name] = f.GetObject().GetSchema()
		}
	}
	for name, schema := range objects {
		t.Run(name, func(t *testing.T) {
			fields := map[string]bool{}
			for _, f := range schema.GetFields() {
				fields[f.GetName()] = true
			}
			if name == "DriverRunConfig" {
				// The pipeline owns these: workload driver_type and url, and the
				// managed-YDB key file it writes at run time.
				fields["driverType"] = true
				fields["url"] = true
				fields["serviceAccountKeyFile"] = true
			}
			for key := range upstream.Defs[name].Properties {
				if strings.Contains(key, "_") {
					continue
				}
				if !fields[key] {
					t.Errorf("native field cannot be expressed: %s", key)
				}
				delete(fields, key)
			}
			for key := range fields {
				t.Errorf("schema field is not native: %s", key)
			}
		})
	}
}

// measuredStep is the default mode's measured step of a script.
func measuredStep(script string) string {
	mode := workloadschema.Modes(script)[0]
	for _, step := range workloadschema.Steps(script, mode) {
		if workloadschema.Measured(script, mode, step) {
			return step
		}
	}
	return ""
}
