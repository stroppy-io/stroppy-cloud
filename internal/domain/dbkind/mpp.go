package dbkind

import (
	"strconv"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
)

// The Greenplum family: five engines on one deployment shape (a coordinator
// and segment hosts, gpinitsystem over ssh, primaries only), built into the
// mpp-builds images with one runtime contract. They are analytic engines:
// the platform runs TPC-H and TPC-DS on them, not the transactional mixes.
//
// doc: https://github.com/stroppy-io/mpp-builds — "Image contract", "Fork-specific notes for the deployer"

// Ports of the family's deployment.
const (
	// MPPSSHPort is the port of the sshd in every container (MPP_SSH_PORT of the image).
	MPPSSHPort = 2222
	// MPPPrimaryPortBase is PORT_BASE of gpinitsystem: the primaries count up from it.
	MPPPrimaryPortBase = 6000
	// MPPCoordinatorPort is where clients connect.
	MPPCoordinatorPort = 5432
)

// mppLogin names the role, its password and the database stroppy works in.
const mppLogin = "stroppy"

// mppRelease is one image of an engine. base is the PostgreSQL major the
// release is built on: 9 for 9.4 (the 6.x engines), 12 or 14.
type mppRelease struct {
	version, image string
	base           int
	isDefault      bool
}

// mppEngine is one kind of the family.
type mppEngine struct {
	kind        catalog.DatabaseKind
	title       string
	description string
	releases    []mppRelease
}

func mppEngines() []mppEngine {
	const registry = "docker.stroppy.io/stroppy-io/"
	return []mppEngine{
		{
			kind: catalog.Greenplum, title: "Greenplum", description: "Greenplum 6 and 7 MPP cluster: a coordinator and segment hosts, no mirrors, GPORCA or the PostgreSQL planner.",
			releases: []mppRelease{
				{version: "7.1.0", image: registry + "greenplum:7.1.0", base: 12, isDefault: true},
				{version: "6.27.1", image: registry + "greenplum:6.27.1", base: 9},
			},
		},
		{
			kind: catalog.Greengage, title: "Greengage", description: "Greengage MPP cluster: a coordinator and segment hosts, no mirrors. 8.0.0-alpha.1 is a pre-release.",
			releases: []mppRelease{
				{version: "8.0.0-alpha.1", image: registry + "greengage:8.0.0-alpha.1", base: 14},
				{version: "7.5.1", image: registry + "greengage:7.5.1", base: 12, isDefault: true},
				{version: "6.32.0", image: registry + "greengage:6.32.0", base: 9},
			},
		},
		{
			kind: catalog.WarehousePG, title: "WarehousePG", description: "WarehousePG MPP cluster: a coordinator and segment hosts, no mirrors.",
			releases: []mppRelease{
				{version: "7.6.1", image: registry + "warehousepg:7.6.1-whpg", base: 12, isDefault: true},
				{version: "6.27.7", image: registry + "warehousepg:6.27.7-whpg", base: 9},
			},
		},
		{
			kind: catalog.OpenGPDB, title: "OpenGPDB", description: "OpenGPDB 6 MPP cluster: a coordinator and segment hosts, no mirrors.",
			releases: []mppRelease{
				{version: "6.30.2", image: registry + "opengpdb:6.30.2", base: 9, isDefault: true},
			},
		},
		{
			kind: catalog.Cloudberry, title: "Apache Cloudberry", description: "Apache Cloudberry (incubating) MPP cluster: a coordinator and segment hosts, no mirrors.",
			releases: []mppRelease{
				{version: "2.1.0", image: registry + "cloudberry:2.1.0-incubating", base: 14, isDefault: true},
			},
		},
	}
}

// MPPLogin is the role and database the stands of the family are created
// with (the stand is throwaway; nothing here is a secret).
func MPPLogin() topology.Login {
	return topology.Login{User: mppLogin, Password: mppLogin, Database: mppLogin}
}

// MPPCoordinatorKeyword is how the version spells the coordinator in
// gpinitsystem_config: MASTER on a PostgreSQL 9.4 base, COORDINATOR on 12
// and 14. ok is false for a version the family does not ship.
func MPPCoordinatorKeyword(kind catalog.DatabaseKind, version string) (keyword string, ok bool) {
	for _, e := range mppEngines() {
		if e.kind != kind {
			continue
		}
		for _, r := range e.releases {
			if r.version == version {
				if r.base == 9 {
					return "MASTER", true
				}
				return "COORDINATOR", true
			}
		}
	}
	return "", false
}

// IsMPP reports a kind of the Greenplum family.
func IsMPP(kind catalog.DatabaseKind) bool {
	for _, e := range mppEngines() {
		if e.kind == kind {
			return true
		}
	}
	return false
}

// MPPSegmentsPerHost is the segments on each segment host: the explicit
// parameter, or half of the host's vCPUs (at least 1) for 0. cpu is the
// vCPUs of the segment machine.
func MPPSegmentsPerHost(params map[string]any, cpu int) int {
	if n := topology.IntOf(params, "segments_per_host", 0); n > 0 {
		return n
	}
	return max(1, cpu/2)
}

func mpp() []Kind {
	engines := mppEngines()
	out := make([]Kind, len(engines))
	for i, e := range engines {
		out[i] = mppKind(e)
	}
	return out
}

func mppKind(e mppEngine) Kind {
	engine := string(e.kind)
	versions := make([]catalog.Version, len(e.releases))
	defaultVersion := ""
	for i, r := range e.releases {
		versions[i] = catalog.Version{Version: r.version, Image: r.image, Default: r.isDefault}
		if r.isDefault {
			defaultVersion = r.version
		}
	}
	return Kind{
		Database: catalog.Database{
			Kind: e.kind, Title: e.title, Description: e.description,
			Versions: versions,
			Roles: []catalog.Role{
				{Role: "coordinator", Title: "Coordinator", Engine: engine, ConfigSchemas: []string{"cfg.mpp.gpinitsystem@1", "cfg.mpp.gucs@1", "cfg.exporter.postgres@1"}},
				{Role: "db", Title: "Segment host", Engine: engine, ConfigSchemas: []string{"cfg.mpp.gucs@1"}},
				{Role: "runner", Title: "Stroppy runner", Engine: "stroppy"},
			},
			Topologies: []catalog.Topology{
				{ID: "cluster-4", Title: "Coordinator + 4 segment hosts", Description: "One coordinator and four segment hosts, primaries only.", Params: map[string]any{"version": defaultVersion, "segment_hosts": 4}},
			},
			ParamsSchema: "db." + engine + ".params@1", Protocols: []catalog.Protocol{catalog.ProtoPg}, Deployable: true,
		},
		plan:           func(p *topology.Plan, params map[string]any) { planMPP(p, e.kind, params) },
		login:          func(map[string]any) topology.Login { return MPPLogin() },
		unsupportedFor: analyticOnly,
	}
}

func planMPP(p *topology.Plan, kind catalog.DatabaseKind, params map[string]any) {
	engine := string(kind)
	hosts := topology.IntOf(params, "segment_hosts", 4)
	perHost := topology.IntOf(params, "segments_per_host", 0)
	p.Add(topology.RoleCoordinator, engine, 1)
	p.Add(topology.RoleDB, engine, hosts)
	// The coordinator reaches every segment host over ssh (gpinitsystem and the
	// management utilities) and every primary over libpq; segments talk to each
	// other and to the coordinator over the UDP interconnect on ports the
	// engine picks at run time, so those edges carry no port (they are for the
	// topology view; the run's subnets are open to each other).
	p.Flows = append(p.Flows,
		topology.Flow{From: topology.RoleCoordinator, To: topology.RoleDB, Protocol: "mpp-ssh", Port: MPPSSHPort},
		topology.Flow{From: topology.RoleCoordinator, To: topology.RoleDB, Protocol: "mpp-segment", Port: MPPPrimaryPortBase},
		topology.Flow{From: topology.RoleCoordinator, To: topology.RoleDB, Protocol: "mpp-interconnect"},
		topology.Flow{From: topology.RoleDB, To: topology.RoleCoordinator, Protocol: "mpp-interconnect"},
	)
	if hosts > 1 {
		p.Flows = append(p.Flows, topology.Flow{From: topology.RoleDB, To: topology.RoleDB, Protocol: "mpp-interconnect"})
	}
	p.Client = topology.Endpoint{Role: topology.RoleCoordinator, Protocol: "pg", Port: MPPCoordinatorPort}

	// The coordinator plans and gathers; a segment host runs a process per
	// segment, and the vendor guide asks for at least 8 GB of RAM for each
	// (4 vCPU / 16 GB is the floor of two segments).
	//
	// doc: https://techdocs.broadcom.com/us/en/vmware-tanzu/data-solutions/tanzu-greenplum/7/greenplum-database/public_cloud_requirements.html
	p.Requirements[topology.RoleCoordinator] = topology.Because(2, 8, 20, topology.ReasonBaseline, "engine", engine)
	segmentCPU, segmentMemory := 4, 16
	if perHost > 0 {
		segmentCPU, segmentMemory = max(segmentCPU, 2*perHost), max(segmentMemory, 8*perHost)
	}
	p.Requirements[topology.RoleDB] = topology.Because(segmentCPU, float64(segmentMemory), 50, topology.ReasonBaseline, "engine", engine, "segments_per_host", strconv.Itoa(perHost))
}

// analyticOnly refuses the transactional scripts: the family is built for
// analytic queries, and a TPC-C or TPC-B mix of single-row transactions
// measures the coordinator's dispatch, not the engine.
func analyticOnly(map[string]any) map[string]unsupportedScript {
	oltp := unsupportedScript{why: "Greenplum-family engines are analytic MPP engines: the transactional TPC-C and TPC-B mixes are out of scope for them, use TPC-H or TPC-DS"}
	banking := unsupportedScript{why: "banking is a transactional invariant workload; Greenplum-family engines are analytic MPP engines and run TPC-H and TPC-DS"}
	baseline := unsupportedScript{why: "the baseline workload is a transactional load probe; Greenplum-family engines are analytic MPP engines and run TPC-H and TPC-DS"}
	return map[string]unsupportedScript{
		"tpcc/tx": oltp, "tpcc/procs": oltp, "tpcb/tx": oltp, "tpcb/procs": oltp,
		"banking": banking, "banking/inject": banking, "baseline": baseline,
	}
}
