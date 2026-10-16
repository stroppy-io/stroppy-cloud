package dbkind

import (
	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
)

// custom is a database the product has no recipe for: the tenant's Docker
// image on the db machines, reached in a protocol stroppy speaks, with the
// login the image is told to create.
func custom() Kind {
	return Kind{
		Database: catalog.Database{
			Kind: catalog.Custom, Title: "Custom image", Description: "Your own database image on the stand's machines: you set the image, its env and command, and how stroppy connects.",
			Versions: []catalog.Version{{Version: "custom", Default: true}},
			Roles: []catalog.Role{
				{Role: "db", Title: "Your database", Engine: "custom"},
				{Role: "runner", Title: "Stroppy runner", Engine: "stroppy"},
			},
			Topologies: []catalog.Topology{
				{
					ID: "postgres-image", Title: "PostgreSQL image", Description: "The official postgres image as an example of a custom one.",
					Params: map[string]any{
						"image": "postgres:17", "protocol": "pg", "port": 5432,
						"env":         map[string]any{"POSTGRES_USER": "stroppy", "POSTGRES_PASSWORD": "stroppy", "POSTGRES_DB": "stroppy"},
						"data_dir":    "/var/lib/postgresql/data",
						"healthcheck": "pg_isready -U stroppy -d stroppy",
					},
				},
			},
			ParamsSchema: "db.custom.params@1",
			Protocols:    []catalog.Protocol{catalog.ProtoPg, catalog.ProtoMySQL, catalog.ProtoPicodata, catalog.ProtoYDBGrpc, catalog.ProtoYDBGrpcs, catalog.ProtoCockroach},
			Deployable:   true,
		},
		plan: func(p *topology.Plan, params map[string]any) {
			nodes := topology.IntOf(params, "nodes", 1)
			p.Add(topology.RoleDB, "custom", nodes)
			p.Client = topology.Endpoint{
				Role: topology.RoleDB, Protocol: topology.StrOf(params, "protocol", "pg"), Port: topology.IntOf(params, "port", 5432),
				URL: topology.StrOf(params, "url", ""),
			}
			p.Requirements[topology.RoleDB] = topology.Because(2, 4, 20, topology.ReasonImage)
		},
		login: func(params map[string]any) topology.Login {
			return topology.Login{User: topology.StrOf(params, "user", ""), Password: topology.StrOf(params, "password", ""), Database: topology.StrOf(params, "database", "")}
		},
		protocols: func(params map[string]any) []catalog.Protocol {
			return []catalog.Protocol{catalog.Protocol(topology.StrOf(params, "protocol", "pg"))}
		},
	}
}
