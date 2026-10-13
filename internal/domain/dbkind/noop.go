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
			p.Requirements[topology.RoleDB] = topology.Requirement{CPU: 2, MemoryGB: 2, DiskGB: 10, Reason: "pg-noop is CPU-bound"}
			p.Label = "pg-noop"
		},
		login: func(map[string]any) topology.Login {
			// pg-noop accepts any login and needs no password.
			return topology.Login{User: PostgresLogin.User, Database: PostgresLogin.Database}
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
			p.Label = "noop (runner only)"
		},
	}
}
