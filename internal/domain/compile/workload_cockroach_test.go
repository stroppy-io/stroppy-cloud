package compile

import (
	"encoding/json"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
	"github.com/stroppy-io/stroppy-cloud/pipelines/stroppycfg"
)

func TestCockroachSQLDialectReachesStroppy(t *testing.T) {
	for _, tc := range []struct{ script, override, want, version string }{
		{"tpcb/tx", "", "crdb.sql", "26.3"},
		{"tpcb/procs", "", "crdb.sql", "26.3"},
		{"tpcc/tx", "", "crdb.sql", "26.3"},
		{"tpcc/procs", "", "crdb.sql", "26.3"},
		{"tpcc/tx", "tpcc/custom", "tpcc/custom", "24.1"},
		{"tpcc/procs", "", "crdb24.sql", "24.1"},
	} {
		t.Run(tc.script+tc.override, func(t *testing.T) {
			raw, err := json.Marshal(spec.Segment{Workload: spec.WorkloadParams{Script: tc.script, Params: map[string]any{"sql_file": tc.override}}})
			if err != nil {
				t.Fatal(err)
			}
			raw, err = cockroachSQL(raw, tc.version)
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
			if params["sqlFile"] != tc.want {
				t.Fatalf("SQL file = %v, want %s", params["sqlFile"], tc.want)
			}
		})
	}
}
