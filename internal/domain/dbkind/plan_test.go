package dbkind_test

import (
	"context"
	"encoding/json"
	"math"
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
	if why := dbkind.UnsupportedScript(catalog.Picodata, "25.3", nil, "tpch/tx"); why == "" {
		t.Fatal("Picodata 25.3 TPC-H is not refused")
	}
	// Serverless times out compiling the scripts' DDL and queries even on
	// stroppy 6.4.0 (stroppy-io/stroppy#231); dedicated runs them, and TPC-B
	// runs on both.
	for _, script := range []string{"tpcc/tx", "tpcc/procs", "tpch/tx", "tpcds", "simple"} {
		if why := dbkind.UnsupportedScript(catalog.YDBManaged, "managed", map[string]any{"type": "serverless"}, script); why == "" {
			t.Fatalf("serverless YDB runs %s", script)
		}
		if why := dbkind.UnsupportedScript(catalog.YDBManaged, "managed", map[string]any{"type": "dedicated"}, script); why != "" {
			t.Fatalf("dedicated YDB refuses %s: %s", script, why)
		}
	}
	if why := dbkind.UnsupportedScript(catalog.YDBManaged, "managed", map[string]any{"type": "serverless"}, "tpcb/tx"); why != "" {
		t.Fatalf("serverless YDB refuses tpcb/tx: %s", why)
	}
	if why := dbkind.UnsupportedScript(catalog.PgNoop, "0.1.2", nil, "simple"); why == "" {
		t.Fatal("pg-noop simple is not refused")
	}
	for _, kind := range []catalog.DatabaseKind{catalog.Postgres, catalog.OrioleDB} {
		if why := dbkind.UnsupportedScript(kind, "17", nil, "banking/inject"); why == "" {
			t.Fatalf("%s without Macavity runs banking/inject", kind)
		}
	}
	for _, c := range [][3]string{{"picodata", "26.1", "tpch/tx"}, {"picodata", "25.3", "tpcc/tx"}, {"postgres", "17", "tpch/tx"}, {"cockroach", "24.1", "tpcc/procs"}, {"postgres", "18", "banking"}, {"external", "external", "banking/inject"}} {
		if why := dbkind.UnsupportedScript(catalog.DatabaseKind(c[0]), c[1], nil, c[2]); why != "" {
			t.Fatalf("%v refused: %s", c, why)
		}
	}
}

// DataShare is the part of the workload's dataset one machine of a role
// stores: a full copy on every node of a replicated single-writer engine,
// the copies over the data nodes of a distributed one, none on roles that
// hold no table data.
func TestPlanDataShare(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		kind   catalog.DatabaseKind
		params map[string]any
		want   map[string]float64
	}{
		{"postgres with replicas", catalog.Postgres, map[string]any{"version": "17", "replicas": 2}, map[string]float64{topology.RoleDB: 1, topology.RoleDBReplica: 1}},
		{"greenplum 1+4", catalog.Greenplum, map[string]any{"version": "7.1.0", "segment_hosts": 4}, map[string]float64{topology.RoleDB: 0.25}},
		{"cockroach three nodes", catalog.Cockroach, map[string]any{"version": "25.4", "nodes": 3}, map[string]float64{topology.RoleDB: 1}},
		{"cockroach five nodes", catalog.Cockroach, map[string]any{"version": "25.4", "nodes": 5}, map[string]float64{topology.RoleDB: 0.6}},
		{"ydb no redundancy", catalog.YDB, map[string]any{"version": "26.2", "fault_tolerance": "none", "storage_nodes": 3, "database_nodes": 3}, map[string]float64{topology.RoleDB: 1.0 / 3}},
		{"ydb block-4-2", catalog.YDB, map[string]any{"version": "26.2", "fault_tolerance": "block-4-2", "storage_nodes": 8, "database_nodes": 3}, map[string]float64{topology.RoleDB: 1.5 / 8}},
		{"ydb mirror-3-dc", catalog.YDB, map[string]any{"version": "26.2", "fault_tolerance": "mirror-3-dc", "storage_nodes": 9, "database_nodes": 3}, map[string]float64{topology.RoleDB: 3.0 / 9}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p, err := dbkind.Plan(tc.kind, bake(t, tc.kind, tc.params))
			if err != nil {
				t.Fatal(err)
			}
			for _, n := range p.Nodes {
				if got, want := p.DataShare[n.Role], tc.want[n.Role]; math.Abs(got-want) > 1e-9 {
					t.Errorf("%s: share %v, want %v", n.Role, got, want)
				}
			}
		})
	}
}
