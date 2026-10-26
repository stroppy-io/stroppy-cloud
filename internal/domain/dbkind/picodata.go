package dbkind

import (
	"math"
	"strconv"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
)

// PicodataLogin is the admin account of the deployed Picodata stands.
var PicodataLogin = topology.Login{User: "admin", Password: "T0psecret"}

func picodata() Kind {
	return Kind{
		Database: catalog.Database{
			Kind: catalog.Picodata, Title: "Picodata", Description: "Distributed in-memory SQL; tiers of instances with replication factors, pgproto for stroppy.",
			Versions: []catalog.Version{
				{Version: "26.2", Image: "docker.binary.picodata.io/picodata:26.2.1"},
				{Version: "26.1", Image: "docker.binary.picodata.io/picodata:26.1.3", Default: true},
				{Version: "25.3", Image: "docker.binary.picodata.io/picodata:25.3.8", Deprecated: true},
			},
			Roles: []catalog.Role{
				{Role: "db", Title: "Picodata instance", Engine: "picodata", ConfigSchemas: []string{"cfg.picodata.yaml@26"}},
				{Role: "proxy", Title: "HAProxy", Engine: "haproxy", ConfigSchemas: []string{"cfg.haproxy.cfg@2"}},
				{Role: "runner", Title: "Stroppy runner", Engine: "stroppy"},
			},
			Topologies: []catalog.Topology{
				{ID: "single", Title: "Single instance", Params: map[string]any{"version": "26.1", "tiers": []any{map[string]any{"name": "default", "instances": 1}}}},
				{ID: "three-node", Title: "Three instances, RF 3", Params: map[string]any{"version": "26.1", "tiers": []any{map[string]any{"name": "default", "instances": 3, "replication_factor": 3}}, "haproxy": 1}},
			},
			ParamsSchema: "db.picodata.params@1", Protocols: []catalog.Protocol{catalog.ProtoPicodata}, Deployable: true,
		},
		plan:  planPicodata,
		login: func(map[string]any) topology.Login { return PicodataLogin },
		// stroppy's picodata driver speaks pgwire to the pgproto listener;
		// its postgres driver does not work against Picodata (pgx Ping sends
		// "-- ping", which sbroad rejects). Without pgproto nothing connects.
		protocols: func(params map[string]any) []catalog.Protocol {
			if !topology.BoolOf(params, "pgproto") {
				return []catalog.Protocol{}
			}
			return []catalog.Protocol{catalog.ProtoPicodata}
		},
		// 25.3 cannot bind a date string to a timestamptz parameter, and the
		// TPC-H load passes its dates that way (26.1 and later accept it).
		unsupported: map[string]map[string]unsupportedScript{
			"25.3": {"tpch/tx": {why: "Picodata 25.3 cannot load TPC-H: it does not accept a date for a timestamptz parameter"}},
		},
	}
}

func planPicodata(p *topology.Plan, params map[string]any) {
	instances := 0
	if tiers, ok := params["tiers"].([]any); ok {
		for _, t := range tiers {
			if m, ok := t.(map[string]any); ok {
				instances += topology.IntOf(m, "instances", 1)
			}
		}
	}
	if instances == 0 {
		instances = 1
	}
	// instances_per_machine packs several instances on one machine; the
	// last machine may run fewer.
	perMachine := PicodataInstancesPerMachine(params)
	p.Add(topology.RoleDB, "picodata", (instances+perMachine-1)/perMachine)
	if instances > 1 {
		p.Flows = append(p.Flows, topology.Flow{From: topology.RoleDB, To: topology.RoleDB, Protocol: "picodata-raft", Port: 3301})
	}
	p.Client = topology.Endpoint{Role: topology.RoleDB, Protocol: "picodata", Port: 3301}
	if topology.BoolOf(params, "pgproto") {
		p.Client = topology.Endpoint{Role: topology.RoleDB, Protocol: "pg", Port: 4327}
	}
	haproxy := topology.IntOf(params, "haproxy", 0)
	if haproxy > 0 {
		p.Add(topology.RoleProxy, "haproxy", haproxy)
		p.Flows = append(p.Flows, topology.Flow{From: topology.RoleProxy, To: topology.RoleDB, Protocol: p.Client.Protocol, Port: p.Client.Port})
		p.Client.Role = topology.RoleProxy
		p.Client.Port = 5000
		p.Requirements[topology.RoleProxy] = topology.Because(2, 2, 10, topology.ReasonProxy, "engine", "haproxy")
	}
	// The machine holds the memtx arenas of all its instances. An empty
	// memtx_memory_mb is sized from the machine under tuning auto and is
	// 2 GB under stock; the minimum machine has room for 2 GB per
	// instance, the product's floor for a benchmark.
	memtx := float64(topology.IntOf(params, "memtx_memory_mb", 2048)*perMachine) / 1024
	p.Requirements[topology.RoleDB] = topology.Because(2*perMachine, math.Ceil(memtx*1.5)+1, float64(20*perMachine), topology.ReasonMemtx, "memtx_gb", strconv.FormatFloat(memtx, 'f', -1, 64))
}

// PicodataInstancesPerMachine is the instances_per_machine of the params
// (at least 1): the compiler deals the instances out over the machines
// the plan sized with it.
func PicodataInstancesPerMachine(params map[string]any) int {
	return max(1, topology.IntOf(params, "instances_per_machine", 1))
}
