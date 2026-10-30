package compile

import (
	"encoding/json"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
	"github.com/stroppy-io/stroppy-cloud/pipelines/stroppycfg"
)

func TestCockroach24DialectReachesStroppy(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, script, version string
		params                map[string]any
		dialect, file         any
	}{
		{"current version keeps the driver's dialect", "tpcc/procs", "25.4", map[string]any{}, nil, nil},
		{"24.1 tx", "tpcc/tx", "24.1", map[string]any{}, "crdb24", nil},
		{"24.1 procs", "tpcc/procs", "24.1", map[string]any{}, "crdb24", nil},
		{"24.1 patch release", "tpcc/procs", "24.1.27", map[string]any{}, "crdb24", nil},
		{"24.1 tpcb has one dialect", "tpcb/procs", "24.1", map[string]any{}, nil, nil},
		{"own dialect wins", "tpcc/procs", "24.1", map[string]any{"sql_dialect": "crdb"}, "crdb", nil},
		{"own file wins", "tpcc/tx", "24.1", map[string]any{"sql_file": "custom.sql"}, nil, "custom.sql"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			raw, err := json.Marshal(spec.Segment{Name: "main", Workload: spec.WorkloadParams{Script: tc.script, Params: tc.params}})
			if err != nil {
				t.Fatal(err)
			}
			raw, err = cockroach24Dialect(raw, tc.version)
			if err != nil {
				t.Fatal(err)
			}
			var segment spec.Segment
			if err := json.Unmarshal(raw, &segment); err != nil {
				t.Fatal(err)
			}
			params, err := stroppycfg.Params(segment)
			if err != nil {
				t.Fatal(err)
			}
			if params["sqlDialect"] != tc.dialect || params["sqlFile"] != tc.file {
				t.Fatalf("sqlDialect = %v, sqlFile = %v; want %v, %v", params["sqlDialect"], params["sqlFile"], tc.dialect, tc.file)
			}
		})
	}
}

func BenchmarkCockroach24Dialect(b *testing.B) {
	raw, err := json.Marshal(spec.Segment{Name: "main", Workload: spec.WorkloadParams{Script: "tpcc/procs", Params: map[string]any{"scale_factor": 200}}})
	if err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		if _, err := cockroach24Dialect(raw, "24.1"); err != nil {
			b.Fatal(err)
		}
	}
}
