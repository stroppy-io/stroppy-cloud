package dbkind

import (
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
)

// CockroachLogin is how clients reach the insecure-mode Cockroach stands.
var CockroachLogin = topology.Login{User: "root", Database: "defaultdb"}

func cockroach() Kind {
	return Kind{
		Database: catalog.Database{
			Kind: catalog.Cockroach, Title: "CockroachDB", Description: "Multi-node cluster, insecure mode for benchmarks, HAProxy in front.",
			Versions: []catalog.Version{
				{Version: "26.3", Image: "cockroachdb/cockroach:v26.3.1"},
				{Version: "26.2", Image: "cockroachdb/cockroach:v26.2.6"},
				{Version: "25.4", Image: "cockroachdb/cockroach:v25.4.16", Default: true},
				{Version: "25.2", Image: "cockroachdb/cockroach:v25.2.23"},
				{Version: "24.3", Image: "cockroachdb/cockroach:v24.3.36"},
				{Version: "24.1", Image: "cockroachdb/cockroach:v24.1.33", Deprecated: true},
			},
			Roles: []catalog.Role{
				{Role: "db", Title: "Cockroach node", Engine: "cockroach", ConfigSchemas: []string{"cfg.cockroach.flags@25"}},
				{Role: "proxy", Title: "HAProxy", Engine: "haproxy", ConfigSchemas: []string{"cfg.haproxy.cfg@2"}},
				{Role: "runner", Title: "Stroppy runner", Engine: "stroppy"},
			},
			Topologies: []catalog.Topology{
				{ID: "single", Title: "Single node", Params: map[string]any{"version": "25.4", "nodes": 1}},
				{ID: "three-node", Title: "Three nodes + HAProxy", Params: map[string]any{"version": "25.4", "nodes": 3, "haproxy": 1}},
			},
			ParamsSchema: "db.cockroach.params@1", Protocols: []catalog.Protocol{catalog.ProtoCockroach, catalog.ProtoPg}, Deployable: true,
		},
		plan:  planCockroach,
		login: func(map[string]any) topology.Login { return CockroachLogin },
	}
}

func planCockroach(p *topology.Plan, params map[string]any) {
	nodes := topology.IntOf(params, "nodes", 3)
	p.Add(topology.RoleDB, "cockroach", nodes)
	if nodes > 1 {
		p.Flows = append(p.Flows, topology.Flow{From: topology.RoleDB, To: topology.RoleDB, Protocol: "cockroach-rpc", Port: 26257})
	}
	p.Client = topology.Endpoint{Role: topology.RoleDB, Protocol: "cockroach", Port: 26257}
	haproxy := topology.IntOf(params, "haproxy", 0)
	if haproxy > 0 {
		p.Add(topology.RoleProxy, "haproxy", haproxy)
		p.Flows = append(p.Flows, topology.Flow{From: topology.RoleProxy, To: topology.RoleDB, Protocol: "cockroach", Port: 26257})
		p.Client = topology.Endpoint{Role: topology.RoleProxy, Protocol: "cockroach", Port: 26257}
		p.Requirements[topology.RoleProxy] = topology.Requirement{CPU: 2, MemoryGB: 2, DiskGB: 10, Reason: "haproxy"}
	}
	p.Requirements[topology.RoleDB] = topology.Requirement{CPU: 4, MemoryGB: 8, DiskGB: 40, Reason: "cockroach node"}
	p.Label = fmt.Sprintf("cockroach ×%d", nodes)
	if haproxy > 0 {
		p.Label += " + haproxy"
	}
}
