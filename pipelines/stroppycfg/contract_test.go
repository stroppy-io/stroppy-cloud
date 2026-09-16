package stroppycfg

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
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

// Opt-in conformance against the real executable, always on noop and without
// setup/data generation or cloud calls. This proves parsing and dispatch, not DB
// dialect correctness. CI still checks the independent probe fixture above.
func TestStroppyExecutableContract(t *testing.T) {
	bin := os.Getenv("STROPPY_CONTRACT_BINARY")
	if bin == "" {
		t.Skip("set STROPPY_CONTRACT_BINARY to run local Stroppy conformance")
	}
	for _, script := range []string{"simple", "execute_sql", "baseline", "tpcb/tx", "tpcb/procs", "tpcc/tx", "tpcc/procs", "tpch/tx", "tpcds"} {
		t.Run(script, func(t *testing.T) {
			wl := map[string]any{"script": script}
			if script == "execute_sql" {
				wl["sql_body"] = "SELECT 1"
			}
			seg := bakeSegment(t, map[string]any{"name": "contract", "workload": wl, "run": map[string]any{"executor": "shared-iterations", "iterations": int64(1)}, "steps": []any{"workload"}, "seed": uint64(42)})
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
	names := map[string]string{"pool": "PoolConfig", "postgres": "PostgresConfig", "sql": "SQLConfig", "insertProgress": "InsertProgressConfig"}
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
				fields["driverType"] = true
				fields["url"] = true
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
