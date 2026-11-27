package catalog_test

import (
	"context"
	"slices"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/dbkind"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/schemas"
)

// TestCatalogBakes is the whole point of the static tables: every size row,
// the stroppy catalog and every topology template must fit their schemas,
// and every referenced schema id must exist.
func TestCatalogBakes(t *testing.T) {
	reg := schemas.New()
	c, err := catalog.New(context.Background(), reg, reg, dbkind.Databases())
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Databases) != 17 {
		t.Fatalf("databases = %d", len(c.Databases))
	}
	for _, d := range c.Databases {
		defaults := 0
		for _, v := range d.Versions {
			if v.Default {
				defaults++
			}
		}
		if defaults != 1 {
			t.Errorf("%s: %d default versions", d.Kind, defaults)
		}
		if len(d.Topologies) == 0 || len(d.Roles) == 0 {
			t.Errorf("%s: no topologies or roles", d.Kind)
		}
		hasRunner := false
		for _, r := range d.Roles {
			if r.Role == "runner" {
				hasRunner = true
			}
		}
		if !hasRunner {
			t.Errorf("%s: no runner role", d.Kind)
		}
	}
	if _, ok := c.StroppyVersion(""); !ok {
		t.Error("no default stroppy version")
	}
	if len(c.SchemaList("cfg")) == 0 || len(c.SchemaList("nope")) != 0 {
		t.Error("schema listing by namespace")
	}
	if len(c.MetricsFor(catalog.Postgres)) >= len(c.Metrics) || len(c.MetricsFor("")) != len(c.Metrics) {
		t.Error("metric filtering")
	}
}

// The coordinator of the Greenplum family is a PostgreSQL server the postgres
// exporter reads, so its metrics are the PostgreSQL group's.
func TestMPPKindsHavePostgresMetrics(t *testing.T) {
	reg := schemas.New()
	c, err := catalog.New(context.Background(), reg, reg, dbkind.Databases())
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []catalog.DatabaseKind{catalog.Greenplum, catalog.Greengage, catalog.WarehousePG, catalog.OpenGPDB, catalog.Cloudberry} {
		found := false
		for _, m := range c.MetricsFor(kind) {
			found = found || m.Key == "pg_stat_database_xact_commit"
			if m.Group == "CockroachDB" || m.Group == "Picodata" {
				t.Errorf("%s gets the %s metric %s", kind, m.Group, m.Key)
			}
		}
		if !found {
			t.Errorf("%s has no PostgreSQL commit metric", kind)
		}
	}
}

// TestStroppyScriptMatrix pins which drivers each stroppy script runs on
// (stroppy README and workloads/*: dialect files per driver).
func TestStroppyScriptMatrix(t *testing.T) {
	reg := schemas.New()
	c, err := catalog.New(context.Background(), reg, reg, dbkind.Databases())
	if err != nil {
		t.Fatal(err)
	}
	v, ok := c.StroppyVersion("")
	if !ok {
		t.Fatal("no default stroppy version")
	}
	speaks := func(script string, p catalog.Protocol) bool {
		s, ok := v.Script(script)
		if !ok {
			t.Fatalf("no script %s", script)
		}
		for _, sp := range s.Protocols {
			if sp == p {
				return true
			}
		}
		return false
	}
	cases := []struct {
		script string
		proto  catalog.Protocol
		want   bool
	}{
		{"tpcc/tx", catalog.ProtoCockroach, true},
		{"tpcc/tx", catalog.ProtoPicodata, true},
		{"tpcc/procs", catalog.ProtoPicodata, false},
		{"tpcc/procs", catalog.ProtoYDBGrpc, false},
		{"tpcb/procs", catalog.ProtoCockroach, true},
		{"banking", catalog.ProtoPicodata, true},
		{"banking", catalog.ProtoCockroach, false},
		{"banking/inject", catalog.ProtoPg, true},
		{"banking/inject", catalog.ProtoMySQL, false},
		{"tpch/tx", catalog.ProtoCockroach, true},
		{"tpcds", catalog.ProtoCockroach, false},
		{"tpch/tx", catalog.ProtoYDBGrpcs, true},
	}
	for _, tc := range cases {
		if got := speaks(tc.script, tc.proto); got != tc.want {
			t.Errorf("%s on %s = %v, want %v", tc.script, tc.proto, got, tc.want)
		}
	}
}

// TestScriptRatingMetric pins each benchmark's headline and that it is a
// cataloged metric the rating accepts.
func TestScriptRatingMetric(t *testing.T) {
	reg := schemas.New()
	c, err := catalog.New(context.Background(), reg, reg, dbkind.Databases())
	if err != nil {
		t.Fatal(err)
	}
	v, ok := c.StroppyVersion("")
	if !ok {
		t.Fatal("no default stroppy version")
	}
	eligible := map[string]bool{}
	for _, m := range c.MetricsFor("") {
		eligible[m.Key] = m.RatingEligible
	}
	want := map[string]string{
		"tpcc/tx": "tpmc", "tpcc/procs": "tpmc", "tpcb/tx": "tps", "tpcb/procs": "tps",
		"tpch/tx": "qphh", "tpcds": "qphds", "simple": "qps", "banking": "tps",
		"execute_sql": "", "baseline": "", "banking/inject": "",
	}
	for script, metric := range want {
		s, ok := v.Script(script)
		if !ok {
			t.Fatalf("no script %s", script)
		}
		if s.RatingMetric != metric {
			t.Errorf("%s rates by %q, want %q", script, s.RatingMetric, metric)
		}
		if metric != "" && !eligible[metric] {
			t.Errorf("%s rates by %q, which is not a rating-eligible metric", script, metric)
		}
	}
}

// A mode changes which steps a script runs: the catalog lists the steps of
// every mode in run order, each naming the modes it belongs to.
func TestStroppyStepsCarryTheirModes(t *testing.T) {
	t.Parallel()
	steps := map[string]catalog.StroppyStep{}
	var order []string
	for _, s := range scriptOf(t, "tpcc/procs").Steps {
		steps[s.ID] = s
		order = append(order, s.ID)
	}
	want := []string{"drop_schema", "create_schema", "create_procedures", "create_terminal_procedures", "load_data", "create_indexes",
		"create_foreign_keys", "analyze", "validate_population", "initialize_terminals", "workload", "delivery_results"}
	if !slices.Equal(order, want) {
		t.Fatalf("tpcc/procs steps %v", order)
	}
	if len(steps["workload"].Modes) != 0 || steps["workload"].Phase != "workload" {
		t.Errorf("workload: %+v", steps["workload"])
	}
	if !slices.Equal(steps["initialize_terminals"].Modes, []string{"strict"}) {
		t.Errorf("initialize_terminals: %+v", steps["initialize_terminals"])
	}
	var full []string
	for _, s := range scriptOf(t, "tpch/tx").Steps {
		if slices.Equal(s.Modes, []string{"full"}) && s.Phase == "workload" {
			full = append(full, s.ID)
		}
	}
	if len(full) != 10 {
		t.Errorf("tpch full measured steps %v", full)
	}
}

func scriptOf(t *testing.T, id string) catalog.StroppyScript {
	t.Helper()
	reg := schemas.New()
	c, err := catalog.New(context.Background(), reg, reg, dbkind.Databases())
	if err != nil {
		t.Fatal(err)
	}
	v, ok := c.StroppyVersion("")
	if !ok {
		t.Fatal("no default stroppy version")
	}
	s, ok := v.Script(id)
	if !ok {
		t.Fatalf("no script %s", id)
	}
	return s
}
