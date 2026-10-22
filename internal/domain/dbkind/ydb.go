package dbkind

import (
	"strconv"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
)

// YDBDatabasePath is the database the deployed YDB stands create, unless
// the params name another.
const YDBDatabasePath = "/Root/testdb"

// The data disk of a storage node: every pdisk at least YDBPdiskMinGB
// (below it a node cannot use the device) plus filesystem headroom.
//
// doc: https://ydb.tech/docs/en/devops/concepts/system-requirements
const (
	YDBPdiskMinGB     = 80
	ydbDiskHeadroomGB = 20
)

// YDBDiskMinGB is the data disk a storage node with pdisks pdisks needs.
func YDBDiskMinGB(pdisks int) int { return YDBPdiskMinGB*pdisks + ydbDiskHeadroomGB }

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
				{ID: "mirror-3-dc", Title: "mirror-3-dc", Description: "Nine storage nodes, three pdisks each, three database nodes.", Params: map[string]any{"version": "26.2", "fault_tolerance": "mirror-3-dc", "storage_nodes": 9, "database_nodes": 3, "pdisks_per_node": 3}},
			},
			// The stand serves plain gRPC only (no TLS certificates are issued):
			// a grpcs client would hang in its handshake until the segment times out.
			ParamsSchema: "db.ydb.params@1", Protocols: []catalog.Protocol{catalog.ProtoYDBGrpc}, Deployable: true,
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
			p.Mode = topology.StrOf(params, "type", topology.ModeDedicated)
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
	p.Requirements[topology.RoleDB] = topology.Because(4, 8, float64(YDBDiskMinGB(pdisks)), topology.ReasonPDisks, "pdisks", strconv.Itoa(pdisks))
	// A database node runs every query, transaction and gRPC call. Under
	// auto it is sized 8 vCPU / 32 GB: at 4 vCPU the actor system gets one
	// user-pool thread, and the docs' slot is 10 cores / 50 GB. Stock keeps
	// the old 4 / 8 floor.
	// doc: https://ydb.tech/docs/en/concepts/glossary — "slot"
	if topology.StrOf(params, "tuning", "auto") == "stock" {
		p.Requirements[topology.RoleDBCompute] = topology.Because(4, 8, 20, topology.ReasonBaseline, "engine", "ydb")
	} else {
		p.Requirements[topology.RoleDBCompute] = topology.Because(8, 32, 20, topology.ReasonBaseline, "engine", "ydb")
	}
	if ft := topology.StrOf(params, "fault_tolerance", "none"); ft != "none" {
		p.Mode = ft
	}
}
