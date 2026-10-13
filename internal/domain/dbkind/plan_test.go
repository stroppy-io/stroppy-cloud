package dbkind_test

import (
	"context"
	"encoding/json"
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
	if p.Label != "postgres + 2 replicas + patroni + haproxy + pgbouncer" {
		t.Fatalf("label %q", p.Label)
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
			if plan.Label == "" {
				t.Errorf("%s/%s: empty label", d.Kind, tpl.ID)
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
