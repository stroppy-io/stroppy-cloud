package dbkind

import (
	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
)

func pgNoop() Kind {
	return Kind{
		Database: catalog.Database{
			Kind: catalog.PgNoop, Title: "pg-noop", Description: "PostgreSQL-protocol server that acknowledges everything: the driver-side ceiling.",
			Versions: []catalog.Version{{Version: "0.1.2", Image: "docker.stroppy.io/stroppy-io/pg-noop@sha256:c35aea48379fcefadb8f06cab4e6023257f28c23f3f27855680f97e468df4f3d", Default: true}},
			Roles: []catalog.Role{
				{Role: "db", Title: "pg-noop", Engine: "pg_noop"},
				{Role: "runner", Title: "Stroppy runner", Engine: "stroppy"},
			},
			Topologies:   []catalog.Topology{{ID: "single", Title: "Single node", Params: map[string]any{"version": "0.1.2"}}},
			ParamsSchema: "db.pg_noop.params@1", Protocols: []catalog.Protocol{catalog.ProtoPg}, Deployable: true,
		},
		plan: func(p *topology.Plan, params map[string]any) {
			p.Add(topology.RoleDB, "pg_noop", 1)
			p.Client = topology.Endpoint{Role: topology.RoleDB, Protocol: "pg", Port: topology.IntOf(params, "port", 5432)}
			p.Requirements[topology.RoleDB] = topology.Because(2, 2, 10, topology.ReasonCPUBound, "engine", "pg-noop")
		},
		login: func(map[string]any) topology.Login {
			// pg-noop accepts any login and needs no password.
			return topology.Login{User: PostgresLogin.User, Database: PostgresLogin.Database}
		},
		// pg-noop 0.1.2 answers a binary int8 with one byte and mismatches the
		// bulk INSERT parameters (stroppy-io/pg-noop#8): TPC-C reads ids back,
		// simple bulk-inserts.
		unsupported: map[string]map[string]unsupportedScript{
			"0.1.2": {
				"tpcc/tx":    {why: "pg-noop 0.1.2 cannot answer TPC-C: its binary int8 values are malformed"},
				"tpcc/procs": {why: "pg-noop 0.1.2 cannot answer TPC-C: its binary int8 values are malformed"},
				"simple":     {why: "pg-noop 0.1.2 rejects the simple workload's bulk INSERT parameters"},
			},
		},
	}
}

func noop() Kind {
	return Kind{
		Database: catalog.Database{
			Kind: catalog.Noop, Title: "Noop", Description: "No database at all: stroppy's own generator, the framework ceiling.",
			Versions: []catalog.Version{{Version: "builtin", Default: true}},
			Roles: []catalog.Role{
				{Role: "runner", Title: "Stroppy runner", Engine: "stroppy"},
			},
			Topologies:   []catalog.Topology{{ID: "runner-only", Title: "Runner only", Params: map[string]any{}}},
			ParamsSchema: "db.noop.params@1", Protocols: []catalog.Protocol{catalog.ProtoNoop}, Deployable: true,
		},
		plan: func(p *topology.Plan, _ map[string]any) {
			p.Client = topology.Endpoint{Role: topology.RoleRunner, Protocol: "noop"}
		},
	}
}
