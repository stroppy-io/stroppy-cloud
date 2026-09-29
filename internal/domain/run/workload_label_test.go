package run

import (
	"encoding/json"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/library"
)

func TestWorkloadLabel(t *testing.T) {
	seg := func(name, script string) json.RawMessage {
		raw, _ := json.Marshal(map[string]any{"name": name, "workload": map[string]any{"script": script}}) //nolint:errcheck // literal
		return raw
	}
	cases := []struct {
		name string
		ref  string
		segs []json.RawMessage
		want string
	}{
		{"reference wins", "tpcc-small", []json.RawMessage{seg("main", "tpcc/tx")}, "tpcc-small"},
		{"one script", "", []json.RawMessage{seg("load", "tpcc/tx"), seg("main", "tpcc/tx")}, "tpcc/tx"},
		{"several scripts", "", []json.RawMessage{seg("a", "tpch/tx"), seg("b", "tpch/tx"), seg("c", "execute_sql"), seg("d", "tpcds")}, "tpch/tx +2"},
		{"no scripts", "", []json.RawMessage{seg("warm", ""), seg("main", "")}, "warm"},
		{"empty", "", nil, ""},
	}
	for _, c := range cases {
		if got := workloadLabel(c.ref, library.WorkloadSpec{Segments: c.segs}); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
