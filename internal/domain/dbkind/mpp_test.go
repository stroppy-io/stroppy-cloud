package dbkind_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/dbkind"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
)

func mppKindList() []catalog.DatabaseKind {
	return []catalog.DatabaseKind{catalog.Greenplum, catalog.Greengage, catalog.WarehousePG, catalog.OpenGPDB, catalog.Cloudberry}
}

func TestMPPCatalogEntries(t *testing.T) {
	t.Parallel()
	// kind → version → image, as the images are published.
	images := map[catalog.DatabaseKind]map[string]string{
		catalog.Greenplum:   {"7.1.0": "docker.stroppy.io/stroppy-io/greenplum:7.1.0", "6.27.1": "docker.stroppy.io/stroppy-io/greenplum:6.27.1"},
		catalog.Greengage:   {"8.0.0-alpha.1": "docker.stroppy.io/stroppy-io/greengage:8.0.0-alpha.1", "7.5.1": "docker.stroppy.io/stroppy-io/greengage:7.5.1", "6.32.0": "docker.stroppy.io/stroppy-io/greengage:6.32.0"},
		catalog.WarehousePG: {"7.6.1": "docker.stroppy.io/stroppy-io/warehousepg:7.6.1-whpg", "6.27.7": "docker.stroppy.io/stroppy-io/warehousepg:6.27.7-whpg"},
		catalog.OpenGPDB:    {"6.30.2": "docker.stroppy.io/stroppy-io/opengpdb:6.30.2"},
		catalog.Cloudberry:  {"2.1.0": "docker.stroppy.io/stroppy-io/cloudberry:2.1.0-incubating"},
	}
	defaults := map[catalog.DatabaseKind]string{
		catalog.Greenplum: "7.1.0", catalog.Greengage: "7.5.1", catalog.WarehousePG: "7.6.1", catalog.OpenGPDB: "6.30.2", catalog.Cloudberry: "2.1.0",
	}
	for _, kind := range mppKindList() {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			k, ok := dbkind.Lookup(kind)
			if !ok {
				t.Fatal("kind is not in the catalog")
			}
			got := map[string]string{}
			defaultVersion := ""
			for _, v := range k.Versions {
				got[v.Version] = v.Image
				if v.Default {
					defaultVersion = v.Version
				}
			}
			if len(got) != len(images[kind]) {
				t.Errorf("versions %v, want %v", got, images[kind])
			}
			for version, image := range images[kind] {
				if got[version] != image {
					t.Errorf("version %s image %q, want %q", version, got[version], image)
				}
			}
			if defaultVersion != defaults[kind] {
				t.Errorf("default version %q, want %q", defaultVersion, defaults[kind])
			}
			if !k.Deployable || k.ParamsSchema != "db."+string(kind)+".params@1" || !slices.Equal(k.Protocols, []catalog.Protocol{catalog.ProtoPg}) {
				t.Errorf("deployable %v schema %q protocols %v", k.Deployable, k.ParamsSchema, k.Protocols)
			}
			roles := map[string][]string{}
			for _, r := range k.Roles {
				roles[r.Role] = r.ConfigSchemas
			}
			if !slices.Contains(roles["coordinator"], "cfg.mpp.gpinitsystem@1") || !slices.Contains(roles["coordinator"], "cfg.mpp.gucs@1") || !slices.Equal(roles["db"], []string{"cfg.mpp.gucs@1"}) {
				t.Errorf("role schemas %v", roles)
			}
			if len(k.Topologies) != 1 || k.Topologies[0].ID != "cluster-4" || k.Topologies[0].Params["segment_hosts"] != 4 || k.Topologies[0].Params["version"] != defaultVersion {
				t.Errorf("topologies %+v", k.Topologies)
			}
		})
	}
	if k, _ := dbkind.Lookup(catalog.Greengage); !strings.Contains(k.Description, "pre-release") {
		t.Errorf("Greengage does not say 8.0.0-alpha.1 is a pre-release: %q", k.Description)
	}
}

func TestMPPPlan(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                  string
		params                map[string]any
		hosts                 int
		cpu                   int
		memoryGB              float64
		interconnectOnDBFlows bool
	}{
		{"default", map[string]any{}, 4, 4, 16, true},
		{"one host", map[string]any{"segment_hosts": 1}, 1, 4, 16, false},
		{"eight segments per host", map[string]any{"segment_hosts": 3, "segments_per_host": 8}, 3, 16, 64, true},
		{"two segments per host is the floor", map[string]any{"segments_per_host": 2}, 4, 4, 16, true},
	} {
		for _, kind := range mppKindList() {
			t.Run(tc.name+"/"+string(kind), func(t *testing.T) {
				t.Parallel()
				p, err := dbkind.Plan(kind, bake(t, kind, tc.params))
				if err != nil {
					t.Fatal(err)
				}
				counts := map[string]int{}
				for _, n := range p.Nodes {
					counts[n.Role] += n.Count
					if n.Role != topology.RoleRunner && n.Engine != string(kind) {
						t.Errorf("role %s runs %q", n.Role, n.Engine)
					}
				}
				if counts[topology.RoleCoordinator] != 1 || counts[topology.RoleDB] != tc.hosts || counts[topology.RoleRunner] != 1 || p.NodeCount() != tc.hosts+2 {
					t.Errorf("machines %v", counts)
				}
				if p.Client.Role != topology.RoleCoordinator || p.Client.Protocol != "pg" || p.Client.Port != 5432 {
					t.Errorf("client %+v", p.Client)
				}
				if p.Client.Login != (topology.Login{User: "stroppy", Password: "stroppy", Database: "stroppy"}) {
					t.Errorf("login %+v", p.Client.Login)
				}
				if p.Mode != "" {
					t.Errorf("mode %q", p.Mode)
				}
				coordinator, segment := p.Requirements[topology.RoleCoordinator], p.Requirements[topology.RoleDB]
				if coordinator.CPU != 2 || coordinator.MemoryGB != 8 || coordinator.Reason != topology.ReasonBaseline || coordinator.ReasonParams["engine"] != string(kind) {
					t.Errorf("coordinator requirement %+v", coordinator)
				}
				if segment.CPU != tc.cpu || segment.MemoryGB != tc.memoryGB || segment.DiskGB != 50 || segment.Reason != topology.ReasonBaseline {
					t.Errorf("segment requirement %+v", segment)
				}
				if _, ok := p.Requirements[topology.RoleRunner]; ok {
					t.Error("the runner's requirement belongs to the workload")
				}
				has := func(from, to, protocol string) bool {
					return slices.ContainsFunc(p.Flows, func(f topology.Flow) bool { return f.From == from && f.To == to && f.Protocol == protocol })
				}
				for _, want := range [][3]string{
					{"coordinator", "db", "mpp-ssh"}, {"coordinator", "db", "mpp-segment"}, {"coordinator", "db", "mpp-interconnect"},
					{"db", "coordinator", "mpp-interconnect"}, {"runner", "coordinator", "pg"},
				} {
					if !has(want[0], want[1], want[2]) {
						t.Errorf("flow %v is missing in %v", want, p.Flows)
					}
				}
				if has("db", "db", "mpp-interconnect") != tc.interconnectOnDBFlows {
					t.Errorf("db → db interconnect = %v with %d hosts", !tc.interconnectOnDBFlows, tc.hosts)
				}
				for _, f := range p.Flows {
					if f.Protocol == "pg" && f.Port != 5432 {
						t.Errorf("pg flow on port %d", f.Port)
					}
				}
			})
		}
	}
}

func TestMPPUnsupportedScripts(t *testing.T) {
	t.Parallel()
	for _, kind := range mppKindList() {
		k, _ := dbkind.Lookup(kind)
		version := k.Versions[0].Version
		for script, refused := range map[string]bool{
			"tpch/tx": false, "tpcds": false, "simple": false, "execute_sql": false,
			"tpcc/tx": true, "tpcc/procs": true, "tpcb/tx": true, "tpcb/procs": true, "banking": true, "banking/inject": true, "baseline": true,
		} {
			why := dbkind.UnsupportedScript(kind, version, nil, script)
			if refused && why == "" {
				t.Errorf("%s runs %s, which is transactional", kind, script)
			}
			if refused && !strings.Contains(why, "analytic") {
				t.Errorf("%s refuses %s without saying why: %q", kind, script, why)
			}
			if !refused && why != "" {
				t.Errorf("%s refuses %s: %s", kind, script, why)
			}
		}
	}
}

func TestMPPCoordinatorKeyword(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		kind    catalog.DatabaseKind
		version string
		want    string
		ok      bool
	}{
		{catalog.Greenplum, "6.27.1", "MASTER", true},
		{catalog.Greenplum, "7.1.0", "COORDINATOR", true},
		{catalog.Greengage, "6.32.0", "MASTER", true},
		{catalog.Greengage, "7.5.1", "COORDINATOR", true},
		{catalog.Greengage, "8.0.0-alpha.1", "COORDINATOR", true},
		{catalog.WarehousePG, "6.27.7", "MASTER", true},
		{catalog.WarehousePG, "7.6.1", "COORDINATOR", true},
		{catalog.OpenGPDB, "6.30.2", "MASTER", true},
		{catalog.Cloudberry, "2.1.0", "COORDINATOR", true},
		{catalog.Cloudberry, "9.9", "", false},
		{catalog.Postgres, "17", "", false},
	} {
		t.Run(string(tc.kind)+"/"+tc.version, func(t *testing.T) {
			t.Parallel()
			got, ok := dbkind.MPPCoordinatorKeyword(tc.kind, tc.version)
			if got != tc.want || ok != tc.ok {
				t.Errorf("MPPCoordinatorKeyword = %q, %v, want %q, %v", got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestMPPSegmentsPerHost(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		params map[string]any
		cpu    int
		want   int
	}{
		{"auto is half the vCPUs", map[string]any{"segments_per_host": 0}, 32, 16},
		{"auto without the parameter", nil, 8, 4},
		{"auto is at least one", nil, 1, 1},
		{"explicit wins", map[string]any{"segments_per_host": 6}, 32, 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := dbkind.MPPSegmentsPerHost(tc.params, tc.cpu); got != tc.want {
				t.Errorf("MPPSegmentsPerHost = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestIsMPP(t *testing.T) {
	t.Parallel()
	for _, kind := range mppKindList() {
		if !dbkind.IsMPP(kind) {
			t.Errorf("%s is not of the family", kind)
		}
	}
	for _, kind := range []catalog.DatabaseKind{catalog.Postgres, catalog.Cockroach, catalog.External, ""} {
		if dbkind.IsMPP(kind) {
			t.Errorf("%q is of the family", kind)
		}
	}
}

func BenchmarkMPPPlan(b *testing.B) {
	params := map[string]any{"version": "7.5.1", "segment_hosts": 4, "segments_per_host": 0, "tuning": "auto", "optimizer": "orca"}
	for b.Loop() {
		if _, err := dbkind.Plan(catalog.Greengage, params); err != nil {
			b.Fatal(err)
		}
	}
}
