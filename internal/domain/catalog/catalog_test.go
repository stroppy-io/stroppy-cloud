package catalog_test

import (
	"context"
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
	if len(c.Databases) != 12 {
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
		{"tpch/tx", catalog.ProtoCockroach, false},
		{"tpcds", catalog.ProtoCockroach, false},
		{"tpch/tx", catalog.ProtoYDBGrpcs, true},
	}
	for _, tc := range cases {
		if got := speaks(tc.script, tc.proto); got != tc.want {
			t.Errorf("%s on %s = %v, want %v", tc.script, tc.proto, got, tc.want)
		}
	}
}
