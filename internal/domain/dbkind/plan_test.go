package dbkind_test

import (
	"context"
	"encoding/json"
	"slices"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/dbkind"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/schemas"
)

// bake resolves params through the kind's schema, as the library does
// before compiling.
func bake(t *testing.T, kind catalog.DatabaseKind, params map[string]any) map[string]any {
	t.Helper()
	raw, _ := json.Marshal(params)
	out, err := schemas.New().Bake(context.Background(), "db."+string(kind)+".params@1", raw)
	if err != nil {
		t.Fatalf("bake %s: %v", kind, err)
	}
	var m map[string]any
	_ = json.Unmarshal(out, &m)
	return m
}

func TestCompilePostgresPatroni(t *testing.T) {
	p, err := dbkind.Plan(catalog.Postgres, bake(t, catalog.Postgres, map[string]any{
		"version": "17", "replicas": 2, "sync_replicas": 1, "ha": "patroni", "etcd_nodes": 3, "haproxy": 1, "pgbouncer": true,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if p.NodeCount() != 1+2+3+1+1 {
		t.Fatalf("node count %d: %+v", p.NodeCount(), p.Nodes)
	}
	if p.Client.Role != topology.RoleProxy || p.Client.Port != 6432 {
		t.Fatalf("client %+v", p.Client)
	}
	if p.Mode != topology.ModePatroni {
		t.Fatalf("mode %q", p.Mode)
	}
	if _, ok := p.Requirements[topology.RoleEtcd]; !ok {
		t.Fatal("etcd requirement missing")
	}
}

func TestCompileEveryCatalogTemplate(t *testing.T) {
	reg := schemas.New()
	c, err := catalog.New(context.Background(), reg, reg, dbkind.Databases())
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range c.Databases {
		for _, tpl := range d.Topologies {
			plan, err := dbkind.Plan(d.Kind, bake(t, d.Kind, tpl.Params))
			if err != nil {
				t.Fatalf("%s/%s: %v", d.Kind, tpl.ID, err)
			}
			if plan.Mode != "" && !slices.Contains(topology.Modes, plan.Mode) {
				t.Errorf("%s/%s: mode %q is not a topology mode", d.Kind, tpl.ID, plan.Mode)
			}
			for role, req := range plan.Requirements {
				if !slices.Contains(topology.RequirementReasons, req.Reason) {
					t.Errorf("%s/%s: %s requirement reason %q is not a reason code", d.Kind, tpl.ID, role, req.Reason)
				}
			}
			// Every role with machines must be a catalog role of the kind.
			known := map[string]bool{}
			for _, r := range d.Roles {
				known[r.Role] = true
			}
			for _, n := range plan.Nodes {
				if n.ColocatedWith == "" && !known[n.Role] {
					t.Errorf("%s/%s: role %s not in catalog", d.Kind, tpl.ID, n.Role)
				}
			}
			if _, ok := plan.Requirements[topology.RoleRunner]; ok {
				t.Errorf("%s/%s: runner requirement belongs to the workload", d.Kind, tpl.ID)
			}
		}
	}
}

func TestRunnerRequirement(t *testing.T) {
	r := topology.RunnerRequirement(256, 8)
	if r.CPU != 4 || r.MemoryGB != 3 {
		t.Fatalf("%+v", r)
	}
	if topology.Family("db-replica") != "db" || topology.Family("runner") != "runner" {
		t.Fatal("family")
	}
}

// Auto sizes a YDB database node 8 vCPU / 32 GB (stock keeps 4 / 8), and a
// storage node's disk holds every pdisk at YDB's 80 GB minimum.
func TestPlanYDBRequirements(t *testing.T) {
	for _, tc := range []struct {
		params      map[string]any
		cpu         int
		mem, diskGB float64
	}{
		{map[string]any{"version": "26.2"}, 8, 32, 100},
		{map[string]any{"version": "26.2", "tuning": "stock", "pdisks_per_node": 3}, 4, 8, 260},
	} {
		p, err := dbkind.Plan(catalog.YDB, bake(t, catalog.YDB, tc.params))
		if err != nil {
			t.Fatal(err)
		}
		compute, storage := p.Requirements[topology.RoleDBCompute], p.Requirements[topology.RoleDB]
		if compute.CPU != tc.cpu || compute.MemoryGB != tc.mem {
			t.Errorf("%v: database node %+v", tc.params, compute)
		}
		if storage.DiskGB != tc.diskGB {
			t.Errorf("%v: storage disk %v GB, want %v", tc.params, storage.DiskGB, tc.diskGB)
		}
	}
}

func TestUnsupportedScript(t *testing.T) {
	if why := dbkind.UnsupportedScript(catalog.Picodata, "25.3", nil, "tpch/tx", "6.2.1"); why == "" {
		t.Fatal("Picodata 25.3 TPC-H is not refused")
	}
	if why := dbkind.UnsupportedScript(catalog.Cockroach, "24.1", nil, "tpcc/procs", "6.2.1"); why == "" {
		t.Fatal("CockroachDB 24.1 TPC-C procedures are not refused on Stroppy 6.2")
	}
	if why := dbkind.UnsupportedScript(catalog.Cockroach, "24.1", nil, "tpcc/procs", "6.1.0"); why != "" {
		t.Fatalf("Stroppy 6.1 refused: %s", why)
	}
	if why := dbkind.UnsupportedScript(catalog.YDBManaged, "managed", map[string]any{"type": "serverless"}, "tpcc/tx", "6.2.1"); why == "" {
		t.Fatal("serverless YDB TPC-C is not refused")
	}
	if why := dbkind.UnsupportedScript(catalog.YDBManaged, "managed", map[string]any{"type": "dedicated"}, "tpcc/tx", "6.2.1"); why != "" {
		t.Fatalf("dedicated YDB TPC-C refused: %s", why)
	}
	if why := dbkind.UnsupportedScript(catalog.PgNoop, "0.1.2", nil, "simple", "6.2.1"); why == "" {
		t.Fatal("pg-noop simple is not refused")
	}
	for _, c := range [][3]string{{"picodata", "26.1", "tpch/tx"}, {"picodata", "25.3", "tpcc/tx"}, {"postgres", "17", "tpch/tx"}} {
		if why := dbkind.UnsupportedScript(catalog.DatabaseKind(c[0]), c[1], nil, c[2], "6.2.1"); why != "" {
			t.Fatalf("%v refused: %s", c, why)
		}
	}
}
