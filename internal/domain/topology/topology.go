// Package topology turns a database definition (kind + baked params) into
// the machine plan a run needs: which roles, how many of each, which
// engine they run, how traffic flows between them, and the minimum
// hardware every role wants. It is the single place that knows what
// "replicas: 2, ha: patroni, haproxy: 1" means in machines.
//
// The RunSpec compiler and the test validator both consume the Plan; the
// UI shows it as the topology preview.
package topology

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
)

// Roles the plans use. Sizes are per role family (before the dash).
const (
	RoleDB          = "db"
	RoleDBReplica   = "db-replica"
	RoleDBCompute   = "db-compute"
	RoleEtcd        = "etcd"
	RoleProxy       = "proxy"
	RoleRunner      = "runner"
	RoleCoordinator = "coordinator"
)

// Node is one role of the plan.
type Node struct {
	Role   string
	Engine string
	Count  int
	// ColocatedWith names the role this one shares machines with (no
	// machines of its own).
	ColocatedWith string
}

// Flow is one allowed traffic edge.
type Flow struct {
	From, To string
	Protocol string
	Port     int
}

// Requirement is the minimum hardware of a role.
type Requirement struct {
	CPU      int
	MemoryGB float64
	DiskGB   float64
	Reason   string
}

// Plan is the compiled topology.
type Plan struct {
	Kind  catalog.DatabaseKind
	Label string
	Nodes []Node
	Flows []Flow
	// Requirements per role (only roles with machines).
	Requirements map[string]Requirement
	// Client is where stroppy connects: role, protocol and port.
	Client Endpoint
}

// Endpoint is the client entry of the database.
type Endpoint struct {
	Role     string
	Protocol string
	Port     int
}

// NodeCount is the number of machines.
func (p Plan) NodeCount() int {
	n := 0
	for _, node := range p.Nodes {
		if node.ColocatedWith == "" {
			n += node.Count
		}
	}
	return n
}

// Family is the size-table family of a role (db-replica → db).
func Family(role string) string {
	if i := strings.IndexByte(role, '-'); i > 0 {
		return role[:i]
	}
	return role
}

// Compile builds the plan of a kind from its baked params. Params are the
// resolved db.<kind>.params@1 value (defaults applied), so every key the
// kind declares is present.
func Compile(kind catalog.DatabaseKind, params map[string]any) (Plan, error) {
	p := Plan{Kind: kind, Requirements: map[string]Requirement{}}
	switch kind {
	case catalog.Postgres, catalog.OrioleDB:
		compilePostgres(&p, kind, params)
	case catalog.MySQL:
		compileMySQL(&p, params)
	case catalog.MariaDB:
		compileMariaDB(&p, params)
	case catalog.Picodata:
		compilePicodata(&p, params)
	case catalog.YDB:
		compileYDB(&p, params)
	case catalog.Cockroach:
		compileCockroach(&p, params)
	case catalog.PgNoop:
		p.add(RoleDB, "pg_noop", 1)
		p.Client = Endpoint{RoleDB, "pg", intOf(params, "port", 5432)}
		p.Requirements[RoleDB] = Requirement{CPU: 2, MemoryGB: 2, DiskGB: 10, Reason: "pg-noop is CPU-bound"}
		p.Label = "pg-noop"
	case catalog.Noop:
		p.Client = Endpoint{Role: RoleRunner, Protocol: "noop"}
		p.Label = "noop (runner only)"
	case catalog.YDBManaged:
		p.Client = Endpoint{Role: "", Protocol: "ydb_grpcs", Port: 2135}
		p.Label = fmt.Sprintf("managed ydb %s", strOf(params, "type", "dedicated"))
	case catalog.External:
		p.Client = Endpoint{Role: "", Protocol: strOf(params, "protocol", "pg")}
		p.Label = "external " + p.Client.Protocol
	default:
		return Plan{}, fmt.Errorf("topology: unknown kind %q", kind)
	}
	// Every plan has a runner; its hardware comes from the workload.
	p.add(RoleRunner, "stroppy", 1)
	if p.Client.Role != "" {
		p.Flows = append(p.Flows, Flow{From: RoleRunner, To: p.Client.Role, Protocol: p.Client.Protocol, Port: p.Client.Port})
	}
	sort.SliceStable(p.Flows, func(i, j int) bool { return p.Flows[i].From+p.Flows[i].To < p.Flows[j].From+p.Flows[j].To })
	return p, nil
}

func (p *Plan) add(role, engine string, count int) {
	if count <= 0 {
		return
	}
	p.Nodes = append(p.Nodes, Node{Role: role, Engine: engine, Count: count})
}

func (p *Plan) colocate(role, engine, with string) {
	p.Nodes = append(p.Nodes, Node{Role: role, Engine: engine, Count: 0, ColocatedWith: with})
}

func compilePostgres(p *Plan, kind catalog.DatabaseKind, params map[string]any) {
	engine := "postgres"
	if kind == catalog.OrioleDB {
		engine = "orioledb"
	}
	replicas := intOf(params, "replicas", 0)
	ha := strOf(params, "ha", "none")
	haproxy := intOf(params, "haproxy", 0)
	pgbouncer := boolOf(params, "pgbouncer")

	p.add(RoleDB, engine, 1)
	p.add(RoleDBReplica, engine, replicas)
	if ha == "patroni" {
		p.add(RoleEtcd, "etcd", intOf(params, "etcd_nodes", 3))
		p.colocate("patroni", "patroni", RoleDB)
		p.colocate("patroni-replica", "patroni", RoleDBReplica)
		p.Flows = append(p.Flows, Flow{From: RoleEtcd, To: RoleEtcd, Protocol: "etcd-peer", Port: 2380})
		for _, from := range []string{RoleDB, RoleDBReplica} {
			for _, to := range []string{RoleDB, RoleDBReplica} {
				p.Flows = append(p.Flows, Flow{From: from, To: to, Protocol: "patroni-api", Port: 8008}, Flow{From: from, To: to, Protocol: "pg-streaming", Port: 5432})
			}
			if haproxy > 0 {
				p.Flows = append(p.Flows, Flow{From: RoleProxy, To: from, Protocol: "patroni-api", Port: 8008})
			}
		}
		p.Flows = append(p.Flows, Flow{From: RoleDB, To: RoleEtcd, Protocol: "etcd", Port: 2379}, Flow{From: RoleDBReplica, To: RoleEtcd, Protocol: "etcd", Port: 2379})
	}
	if replicas > 0 {
		p.Flows = append(p.Flows, Flow{From: RoleDBReplica, To: RoleDB, Protocol: "pg-streaming", Port: 5432})
	}
	client := Endpoint{RoleDB, "pg", 5432}
	switch {
	case haproxy > 0:
		p.add(RoleProxy, "haproxy", haproxy)
		p.Flows = append(p.Flows, Flow{From: RoleProxy, To: RoleDB, Protocol: "pg", Port: 5432})
		if replicas > 0 {
			p.Flows = append(p.Flows, Flow{From: RoleProxy, To: RoleDBReplica, Protocol: "pg", Port: 5432})
		}
		client = Endpoint{RoleProxy, "pg", 5000}
		if pgbouncer {
			p.colocate("pgbouncer", "pgbouncer", RoleProxy)
			client.Port = 6432
		}
	case pgbouncer:
		p.colocate("pgbouncer", "pgbouncer", RoleDB)
		client.Port = 6432
	}
	p.Client = client

	dbReq := Requirement{CPU: 2, MemoryGB: 4, DiskGB: 20, Reason: "postgres baseline"}
	if boolOf(params, "wal_archive") {
		dbReq.DiskGB += 20
		dbReq.Reason = "postgres baseline + WAL archive"
	}
	if kind == catalog.OrioleDB {
		sb := float64(intOf(params, "shared_buffers_mb", 1024)) / 1024
		if sb*2 > dbReq.MemoryGB {
			dbReq.MemoryGB = math.Ceil(sb * 2)
			dbReq.Reason = "2× shared_buffers"
		}
	}
	p.Requirements[RoleDB] = dbReq
	if replicas > 0 {
		p.Requirements[RoleDBReplica] = dbReq
	}
	if ha == "patroni" {
		p.Requirements[RoleEtcd] = Requirement{CPU: 2, MemoryGB: 2, DiskGB: 10, Reason: "etcd"}
	}
	if haproxy > 0 {
		p.Requirements[RoleProxy] = Requirement{CPU: 2, MemoryGB: 2, DiskGB: 10, Reason: "haproxy"}
	}
	p.Label = label(engine, replicas, "replica", ha == "patroni", "patroni", haproxy > 0, "haproxy", pgbouncer, "pgbouncer")
}

func compileMySQL(p *Plan, params map[string]any) {
	replicas := intOf(params, "replicas", 0)
	proxysql := intOf(params, "proxysql", 0)
	mode := strOf(params, "replication", "async")
	p.add(RoleDB, "mysql", 1)
	p.add(RoleDBReplica, "mysql", replicas)
	if replicas > 0 {
		p.Flows = append(p.Flows, Flow{From: RoleDBReplica, To: RoleDB, Protocol: "mysql-replication", Port: 3306})
		if mode == "group" {
			for _, from := range []string{RoleDB, RoleDBReplica} {
				for _, to := range []string{RoleDB, RoleDBReplica} {
					p.Flows = append(p.Flows, Flow{From: from, To: to, Protocol: "group-replication", Port: 33061},
						Flow{From: from, To: to, Protocol: "group-recovery", Port: 3306})
				}
			}
		}
	}
	p.Client = Endpoint{RoleDB, "mysql", 3306}
	if proxysql > 0 {
		p.add(RoleProxy, "proxysql", proxysql)
		p.Flows = append(p.Flows, Flow{From: RoleProxy, To: RoleDB, Protocol: "mysql", Port: 3306})
		if replicas > 0 {
			p.Flows = append(p.Flows, Flow{From: RoleProxy, To: RoleDBReplica, Protocol: "mysql", Port: 3306})
		}
		p.Client = Endpoint{RoleProxy, "mysql", 6033}
		p.Requirements[RoleProxy] = Requirement{CPU: 2, MemoryGB: 2, DiskGB: 10, Reason: "proxysql"}
	}
	req := Requirement{CPU: 2, MemoryGB: 4, DiskGB: 20, Reason: "mysql baseline"}
	p.Requirements[RoleDB] = req
	if replicas > 0 {
		p.Requirements[RoleDBReplica] = req
	}
	p.Label = label("mysql", replicas, "replica", mode == "group", "group replication", proxysql > 0, "proxysql", false, "")
}

func compileMariaDB(p *Plan, params map[string]any) {
	mode := strOf(params, "replication", "async")
	replicas := intOf(params, "replicas", 0)
	if mode == "galera" {
		nodes := intOf(params, "galera_nodes", 3)
		p.add(RoleDB, "mariadb", nodes)
		p.Flows = append(p.Flows,
			Flow{From: RoleDB, To: RoleDB, Protocol: "galera", Port: 4567},
			Flow{From: RoleDB, To: RoleDB, Protocol: "galera-ist", Port: 4568},
			Flow{From: RoleDB, To: RoleDB, Protocol: "galera-sst", Port: 4444})
		replicas = 0
	} else {
		p.add(RoleDB, "mariadb", 1)
		p.add(RoleDBReplica, "mariadb", replicas)
		if replicas > 0 {
			p.Flows = append(p.Flows, Flow{From: RoleDBReplica, To: RoleDB, Protocol: "mysql-replication", Port: 3306})
		}
	}
	p.Client = Endpoint{RoleDB, "mysql", 3306}
	proxysql := intOf(params, "proxysql", 0)
	maxscale := boolOf(params, "maxscale")
	switch {
	case proxysql > 0:
		p.add(RoleProxy, "proxysql", proxysql)
		p.Client = Endpoint{RoleProxy, "mysql", 6033}
	case maxscale:
		p.add(RoleProxy, "maxscale", 1)
		p.Client = Endpoint{RoleProxy, "mysql", 4006}
	}
	if p.Client.Role == RoleProxy {
		p.Flows = append(p.Flows, Flow{From: RoleProxy, To: RoleDB, Protocol: "mysql", Port: 3306})
		if replicas > 0 {
			p.Flows = append(p.Flows, Flow{From: RoleProxy, To: RoleDBReplica, Protocol: "mysql", Port: 3306})
		}
		p.Requirements[RoleProxy] = Requirement{CPU: 2, MemoryGB: 2, DiskGB: 10, Reason: "proxy"}
	}
	req := Requirement{CPU: 2, MemoryGB: 4, DiskGB: 20, Reason: "mariadb baseline"}
	p.Requirements[RoleDB] = req
	if replicas > 0 {
		p.Requirements[RoleDBReplica] = req
	}
	if mode == "galera" {
		p.Label = fmt.Sprintf("galera ×%d", intOf(params, "galera_nodes", 3))
		if p.Client.Role == RoleProxy {
			p.Label += " + " + p.Nodes[len(p.Nodes)-1].Engine
		}
		return
	}
	p.Label = label("mariadb", replicas, "replica", false, "", p.Client.Role == RoleProxy, "proxy", false, "")
}

func compilePicodata(p *Plan, params map[string]any) {
	instances := 0
	if tiers, ok := params["tiers"].([]any); ok {
		for _, t := range tiers {
			if m, ok := t.(map[string]any); ok {
				instances += intOf(m, "instances", 1)
			}
		}
	}
	if instances == 0 {
		instances = 1
	}
	p.add(RoleDB, "picodata", instances)
	if instances > 1 {
		p.Flows = append(p.Flows, Flow{From: RoleDB, To: RoleDB, Protocol: "picodata-raft", Port: 3301})
	}
	p.Client = Endpoint{RoleDB, "picodata", 3301}
	if boolOf(params, "pgproto") {
		p.Client = Endpoint{RoleDB, "pg", 4327}
	}
	haproxy := intOf(params, "haproxy", 0)
	if haproxy > 0 {
		p.add(RoleProxy, "haproxy", haproxy)
		p.Flows = append(p.Flows, Flow{From: RoleProxy, To: RoleDB, Protocol: p.Client.Protocol, Port: p.Client.Port})
		p.Client.Role = RoleProxy
		p.Client.Port = 5000
		p.Requirements[RoleProxy] = Requirement{CPU: 2, MemoryGB: 2, DiskGB: 10, Reason: "haproxy"}
	}
	memtx := float64(intOf(params, "memtx_memory_mb", 2048)) / 1024
	p.Requirements[RoleDB] = Requirement{CPU: 2, MemoryGB: math.Ceil(memtx*1.5) + 1, DiskGB: 20, Reason: "1.5× memtx_memory + 1 GB"}
	p.Label = fmt.Sprintf("picodata ×%d", instances)
	if haproxy > 0 {
		p.Label += " + haproxy"
	}
}

func compileYDB(p *Plan, params map[string]any) {
	storage := intOf(params, "storage_nodes", 1)
	compute := intOf(params, "database_nodes", 1)
	pdisks := intOf(params, "pdisks_per_node", 1)
	p.add(RoleDB, "ydb", storage)
	p.add(RoleDBCompute, "ydb", compute)
	p.Flows = append(p.Flows, Flow{From: RoleDBCompute, To: RoleDB, Protocol: "ydb-ic", Port: 19001}, Flow{From: RoleDB, To: RoleDB, Protocol: "ydb-ic", Port: 19001})
	p.Client = Endpoint{RoleDBCompute, "ydb_grpc", 2136}
	// The compiler's own floor: every pdisk file plus filesystem headroom.
	p.Requirements[RoleDB] = Requirement{CPU: 4, MemoryGB: 8, DiskGB: float64(40*pdisks + 20), Reason: fmt.Sprintf("%d pdisk(s) × 40 GB + 20 GB filesystem", pdisks)}
	p.Requirements[RoleDBCompute] = Requirement{CPU: 4, MemoryGB: 8, DiskGB: 20, Reason: "ydb database node"}
	p.Label = fmt.Sprintf("ydb %s: %d storage + %d database", strOf(params, "fault_tolerance", "none"), storage, compute)
}

func compileCockroach(p *Plan, params map[string]any) {
	nodes := intOf(params, "nodes", 3)
	p.add(RoleDB, "cockroach", nodes)
	if nodes > 1 {
		p.Flows = append(p.Flows, Flow{From: RoleDB, To: RoleDB, Protocol: "cockroach-rpc", Port: 26257})
	}
	p.Client = Endpoint{RoleDB, "cockroach", 26257}
	haproxy := intOf(params, "haproxy", 0)
	if haproxy > 0 {
		p.add(RoleProxy, "haproxy", haproxy)
		p.Flows = append(p.Flows, Flow{From: RoleProxy, To: RoleDB, Protocol: "cockroach", Port: 26257})
		p.Client = Endpoint{RoleProxy, "cockroach", 26257}
		p.Requirements[RoleProxy] = Requirement{CPU: 2, MemoryGB: 2, DiskGB: 10, Reason: "haproxy"}
	}
	p.Requirements[RoleDB] = Requirement{CPU: 4, MemoryGB: 8, DiskGB: 40, Reason: "cockroach node"}
	p.Label = fmt.Sprintf("cockroach ×%d", nodes)
	if haproxy > 0 {
		p.Label += " + haproxy"
	}
}

// label renders "engine + N replica(s) + patroni + haproxy".
func label(engine string, replicas int, replicaWord string, ha bool, haWord string, proxy bool, proxyWord string, extra bool, extraWord string) string {
	parts := []string{engine}
	switch replicas {
	case 0:
	case 1:
		parts = append(parts, "1 "+replicaWord)
	default:
		parts = append(parts, fmt.Sprintf("%d %ss", replicas, replicaWord))
	}
	if ha {
		parts = append(parts, haWord)
	}
	if proxy {
		parts = append(parts, proxyWord)
	}
	if extra {
		parts = append(parts, extraWord)
	}
	return strings.Join(parts, " + ")
}

// RunnerRequirement is the runner's hardware from the workload: virtual
// users drive CPU, load workers drive memory.
func RunnerRequirement(maxVUs, maxLoadWorkers int) Requirement {
	cpu := int(math.Ceil(float64(maxVUs) / 64))
	if w := int(math.Ceil(float64(maxLoadWorkers) / 4)); w > cpu {
		cpu = w
	}
	if cpu < 2 {
		cpu = 2
	}
	mem := math.Max(2, math.Ceil(float64(maxVUs)/128)+1)
	return Requirement{CPU: cpu, MemoryGB: mem, DiskGB: 20, Reason: fmt.Sprintf("%d VUs, %d load workers", maxVUs, maxLoadWorkers)}
}

func intOf(m map[string]any, key string, def int) int {
	switch v := m[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			return n
		}
	}
	return def
}

func strOf(m map[string]any, key, def string) string {
	if v, ok := m[key].(string); ok && v != "" {
		return v
	}
	return def
}

func boolOf(m map[string]any, key string) bool {
	v, _ := m[key].(bool) //nolint:errcheck // absent = false
	return v
}
