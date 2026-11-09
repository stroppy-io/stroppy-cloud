package compile

import (
	"encoding/json"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
)

// build is a stroppy build whose tpch/tx lists sql-dialect and whose tpcds
// does not.
func build() catalog.StroppyVersion {
	return catalog.StroppyVersion{Scripts: []catalog.StroppyScript{
		{ID: "tpch/tx", Params: []catalog.StroppyParam{{Name: "scale-factor"}, {Name: sqlDialectParam}}},
		{ID: "tpcds", Params: []catalog.StroppyParam{{Name: "scale-factor"}}},
		{ID: "simple"},
	}}
}

func TestGPDialect(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, in, want string }{
		{"tpch gets gp", `{"workload":{"script":"tpch/tx","scale_factor":1}}`, `{"workload":{"script":"tpch/tx","scale_factor":1,"sql_dialect":"gp"}}`},
		{"a dialect of the workload wins", `{"workload":{"script":"tpch/tx","sql_dialect":"pg"}}`, `{"workload":{"script":"tpch/tx","sql_dialect":"pg"}}`},
		{"an sql file wins", `{"workload":{"script":"tpch/tx","sql_file":"tpch/x"}}`, `{"workload":{"script":"tpch/tx","sql_file":"tpch/x"}}`},
		{"a script that is not analytic", `{"workload":{"script":"simple"}}`, `{"workload":{"script":"simple"}}`},
		{"an unknown script", `{"workload":{"script":"nope"}}`, `{"workload":{"script":"nope"}}`},
		{"no workload", `{"name":"x"}`, `{"name":"x"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := gpDialect(json.RawMessage(tc.in), build())
			if err != nil {
				t.Fatal(err)
			}
			var a, b any
			if err := json.Unmarshal(got, &a); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(tc.want), &b); err != nil {
				t.Fatal(err)
			}
			ga, _ := json.Marshal(a)
			gb, _ := json.Marshal(b)
			if string(ga) != string(gb) {
				t.Errorf("gpDialect(%s) = %s, want %s", tc.in, got, tc.want)
			}
		})
	}
	if _, err := gpDialect(json.RawMessage(`{`), build()); err == nil {
		t.Error("a broken segment is accepted")
	}
	// A build without the dialect would measure the pg schema: refused.
	_, err := gpDialect(json.RawMessage(`{"workload":{"script":"tpcds"}}`), build())
	if reason, _ := errs.ReasonOf(err); reason != "mpp_dialect_unavailable" {
		t.Errorf("a build without the gp dialect: %v", err)
	}
	// Its own dialect or SQL file keeps the workload author's choice.
	if _, err := gpDialect(json.RawMessage(`{"workload":{"script":"tpcds","sql_dialect":"pg"}}`), build()); err != nil {
		t.Errorf("an explicit dialect is refused: %v", err)
	}
}

func BenchmarkGPDialect(b *testing.B) {
	raw := json.RawMessage(`{"workload":{"script":"tpch/tx","scale_factor":1},"run":{"vus":1}}`)
	for b.Loop() {
		if _, err := gpDialect(raw, build()); err != nil {
			b.Fatal(err)
		}
	}
}
