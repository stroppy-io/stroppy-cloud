package dbkind

import (
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
)

// YDBDatabasePath is the database the deployed YDB stands create, unless
// the params name another.
const YDBDatabasePath = "/Root/testdb"

func ydb() Kind {
	return Kind{
		Database: catalog.Database{
			Kind: catalog.YDB, Title: "YDB", Description: "Self-hosted YDB: storage and database nodes, erasure modes, pdisks per node.",
			Versions: []catalog.Version{
				{Version: "26.3", Image: "ydbplatform/local-ydb:26.3.1.14"},
				{Version: "26.2", Image: "ydbplatform/local-ydb:26.2.1.14", Default: true},
				{Version: "26.1", Image: "ydbplatform/local-ydb:26.1.1.22"},
				{Version: "25.4", Image: "ydbplatform/local-ydb:25.4.1.15"},
			},
			Roles: []catalog.Role{
				{Role: "db", Title: "Storage node", Engine: "ydb", ConfigSchemas: []string{"cfg.ydb.config.yaml@26", "cfg.host.disks@1"}},
				{Role: "db-compute", Title: "Database node", Engine: "ydb", ConfigSchemas: []string{"cfg.ydb.config.yaml@26"}},
				{Role: "runner", Title: "Stroppy runner", Engine: "stroppy"},
			},
			Topologies: []catalog.Topology{
				{ID: "single", Title: "Single node", Params: map[string]any{"version": "26.2"}},
				{ID: "mirror-3-dc", Title: "mirror-3-dc", Description: "Nine storage nodes, three pdisks each, three database nodes.", Params: map[string]any{"version": "26.2", "fault_tolerance": "mirror-3-dc", "failure_domain": "disk", "storage_nodes": 9, "database_nodes": 3, "pdisks_per_node": 3}},
			},
			ParamsSchema: "db.ydb.params@1", Protocols: []catalog.Protocol{catalog.ProtoYDBGrpc, catalog.ProtoYDBGrpcs}, Deployable: true,
		},
		plan: planYDB,
		login: func(params map[string]any) topology.Login {
			return topology.Login{Database: topology.StrOf(params, "database_path", YDBDatabasePath)}
		},
	}
}

func ydbManaged() Kind {
	return Kind{
		Database: catalog.Database{
			Kind: catalog.YDBManaged, Title: "YDB (Yandex Managed)", Description: "Managed Service for YDB: dedicated or serverless database created by the run.",
			Versions: []catalog.Version{{Version: "managed", Default: true}},
			Roles: []catalog.Role{
				{Role: "runner", Title: "Stroppy runner", Engine: "stroppy"},
			},
			Topologies: []catalog.Topology{
				{ID: "dedicated-medium", Title: "Dedicated, medium ×3", Params: map[string]any{"type": "dedicated", "resource_preset_id": "medium", "node_count": 3}},
				{ID: "serverless", Title: "Serverless", Params: map[string]any{"type": "serverless"}},
			},
			ParamsSchema: "db.ydb_managed.params@1", Protocols: []catalog.Protocol{catalog.ProtoYDBGrpcs}, Deployable: true,
		},
		plan: func(p *topology.Plan, params map[string]any) {
			p.Client = topology.Endpoint{Role: "", Protocol: "ydb_grpcs", Port: 2135}
			p.Label = fmt.Sprintf("managed ydb %s", topology.StrOf(params, "type", "dedicated"))
		},
	}
}

func planYDB(p *topology.Plan, params map[string]any) {
	storage := topology.IntOf(params, "storage_nodes", 1)
	compute := topology.IntOf(params, "database_nodes", 1)
	pdisks := topology.IntOf(params, "pdisks_per_node", 1)
	p.Add(topology.RoleDB, "ydb", storage)
	p.Add(topology.RoleDBCompute, "ydb", compute)
	p.Flows = append(p.Flows, topology.Flow{From: topology.RoleDBCompute, To: topology.RoleDB, Protocol: "ydb-ic", Port: 19001}, topology.Flow{From: topology.RoleDB, To: topology.RoleDB, Protocol: "ydb-ic", Port: 19001})
	p.Client = topology.Endpoint{Role: topology.RoleDBCompute, Protocol: "ydb_grpc", Port: 2136}
	// The compiler's own floor: every pdisk file plus filesystem headroom.
	p.Requirements[topology.RoleDB] = topology.Requirement{CPU: 4, MemoryGB: 8, DiskGB: float64(40*pdisks + 20), Reason: fmt.Sprintf("%d pdisk(s) × 40 GB + 20 GB filesystem", pdisks)}
	p.Requirements[topology.RoleDBCompute] = topology.Requirement{CPU: 4, MemoryGB: 8, DiskGB: 20, Reason: "ydb database node"}
	p.Label = fmt.Sprintf("ydb %s: %d storage + %d database", topology.StrOf(params, "fault_tolerance", "none"), storage, compute)
}
