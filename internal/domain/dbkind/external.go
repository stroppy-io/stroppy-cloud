package dbkind

import (
	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
)

func external() Kind {
	return Kind{
		Database: catalog.Database{
			Kind: catalog.External, Title: "External database", Description: "A database you already run: the platform only places the runner and connects.",
			Versions: []catalog.Version{{Version: "external", Default: true}},
			Roles: []catalog.Role{
				{Role: "runner", Title: "Stroppy runner", Engine: "stroppy"},
			},
			Topologies:   []catalog.Topology{{ID: "dsn", Title: "By DSN", Params: map[string]any{"protocol": "pg", "dsn": "postgres://stroppy@db.example.internal:5432/stroppy"}}},
			ParamsSchema: "db.external.params@1",
			Protocols:    []catalog.Protocol{catalog.ProtoPg, catalog.ProtoMySQL, catalog.ProtoPicodata, catalog.ProtoYDBGrpc, catalog.ProtoYDBGrpcs, catalog.ProtoCockroach},
			Deployable:   false,
		},
		plan: func(p *topology.Plan, params map[string]any) {
			p.Client = topology.Endpoint{Role: "", Protocol: topology.StrOf(params, "protocol", "pg")}
		},
		protocols: func(params map[string]any) []catalog.Protocol {
			// The DSN speaks one protocol; the workload has to speak the same.
			return []catalog.Protocol{catalog.Protocol(topology.StrOf(params, "protocol", "pg"))}
		},
	}
}
