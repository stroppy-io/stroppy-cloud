package compile

import (
	"encoding/json"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
)

// build is a stroppy build whose tpch/tx and tpcds list sql-dialect.
func build() catalog.StroppyVersion {
	return catalog.StroppyVersion{Version: "6.4.0", Scripts: []catalog.StroppyScript{
		{ID: "tpch/tx", Params: []catalog.StroppyParam{{Name: "scale-factor"}, {Name: sqlDialectParam}}},
		{ID: "tpcds", Params: []catalog.StroppyParam{{Name: "scale-factor"}, {Name: sqlDialectParam}}},
		{ID: "simple"},
	}}
}

// oldBuild is a stroppy build without the dialect flag.
func oldBuild() catalog.StroppyVersion {
	return catalog.StroppyVersion{Version: "6.3.3", Scripts: []catalog.StroppyScript{
		{ID: "tpch/tx", Params: []catalog.StroppyParam{{Name: "scale-factor"}}},
		{ID: "tpcds", Params: []catalog.StroppyParam{{Name: "scale-factor"}}},
	}}
}

func TestKindDialect(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		kind     catalog.DatabaseKind
		in, want string
	}{
		{"tpch on Greengage gets gp", catalog.Greengage, `{"workload":{"script":"tpch/tx","scale_factor":1}}`, `{"workload":{"script":"tpch/tx","scale_factor":1,"sql_dialect":"gp"}}`},
		{"tpcds on Cloudberry gets gp", catalog.Cloudberry, `{"workload":{"script":"tpcds"}}`, `{"workload":{"script":"tpcds","sql_dialect":"gp"}}`},
		{"tpcds on MariaDB gets mariadb", catalog.MariaDB, `{"workload":{"script":"tpcds"}}`, `{"workload":{"script":"tpcds","sql_dialect":"mariadb"}}`},
		{"tpch on MariaDB keeps the mysql dialect", catalog.MariaDB, `{"workload":{"script":"tpch/tx"}}`, `{"workload":{"script":"tpch/tx"}}`},
		{"tpcds on MySQL keeps its own", catalog.MySQL, `{"workload":{"script":"tpcds"}}`, `{"workload":{"script":"tpcds"}}`},
		{"PostgreSQL keeps its own", catalog.Postgres, `{"workload":{"script":"tpch/tx"}}`, `{"workload":{"script":"tpch/tx"}}`},
		{"a dialect of the workload wins", catalog.Greengage, `{"workload":{"script":"tpch/tx","sql_dialect":"pg"}}`, `{"workload":{"script":"tpch/tx","sql_dialect":"pg"}}`},
		{"an sql file wins", catalog.Greengage, `{"workload":{"script":"tpch/tx","sql_file":"tpch/x"}}`, `{"workload":{"script":"tpch/tx","sql_file":"tpch/x"}}`},
		{"a script that is not analytic", catalog.Greengage, `{"workload":{"script":"simple"}}`, `{"workload":{"script":"simple"}}`},
		{"no workload", catalog.Greengage, `{"name":"x"}`, `{"name":"x"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := kindDialect(json.RawMessage(tc.in), build(), tc.kind)
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
				t.Errorf("kindDialect(%s) = %s, want %s", tc.in, got, tc.want)
			}
		})
	}
}

func TestKindDialectWithoutTheFlag(t *testing.T) {
	t.Parallel()
	if _, err := kindDialect(json.RawMessage(`{`), build(), catalog.Greengage); err == nil {
		t.Error("a broken segment is accepted")
	}
	// An MPP engine cannot be measured on the pg schema: refused.
	_, err := kindDialect(json.RawMessage(`{"workload":{"script":"tpcds"}}`), oldBuild(), catalog.Greengage)
	if reason, _ := errs.ReasonOf(err); reason != "mpp_dialect_unavailable" {
		t.Errorf("an MPP run on a build without the gp dialect: %v", err)
	}
	// MariaDB still runs the mysql dialect on an older build.
	raw := json.RawMessage(`{"workload":{"script":"tpcds"}}`)
	got, err := kindDialect(raw, oldBuild(), catalog.MariaDB)
	if err != nil || string(got) != string(raw) {
		t.Errorf("MariaDB on an older build: %s %v", got, err)
	}
}

func BenchmarkKindDialect(b *testing.B) {
	raw := json.RawMessage(`{"workload":{"script":"tpch/tx","scale_factor":1},"run":{"vus":1}}`)
	for b.Loop() {
		if _, err := kindDialect(raw, build(), catalog.Greengage); err != nil {
			b.Fatal(err)
		}
	}
}
