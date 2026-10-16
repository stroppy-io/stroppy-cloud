// Package topology is the machine plan a run needs: which roles, how many
// of each, which engine they run, how traffic flows between them, where a
// client connects, and the minimum hardware every role wants. Each kind
// builds its plan in dbkind; the RunSpec compiler and the test validator
// consume it, the UI shows it as the topology preview.
package topology

import (
	"fmt"
	"math"
	"strconv"
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
	// Roles that share another role's machines (Plan.Colocate).
	RolePatroni        = "patroni"
	RolePatroniReplica = "patroni-replica"
	RolePgBouncer      = "pgbouncer"
)

// Roles is every role (the contract's TopologyRole is the same list).
var Roles = []string{RoleDB, RoleDBReplica, RoleDBCompute, RoleEtcd, RoleProxy, RoleRunner, RoleCoordinator, RolePatroni, RolePatroniReplica, RolePgBouncer}

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

// Requirement is the minimum hardware of a role and what drove it: a
// reason code (the contract's RequirementReason) with its numbers.
type Requirement struct {
	CPU          int
	MemoryGB     float64
	DiskGB       float64
	Reason       string
	ReasonParams map[string]string
}

// Requirement reasons.
const (
	// ReasonBaseline: the engine's own minimum (params: engine).
	ReasonBaseline = "engine_baseline"
	// ReasonWALArchive: the baseline plus room for the WAL archive.
	ReasonWALArchive = "wal_archive"
	// ReasonSharedBuffers: twice shared_buffers (params: shared_buffers_gb).
	ReasonSharedBuffers = "shared_buffers"
	// ReasonProxy: a proxy in front of the database (params: engine).
	ReasonProxy = "proxy"
	// ReasonEtcd: the etcd of Patroni.
	ReasonEtcd = "etcd"
	// ReasonImage: a database image, the stand's baseline.
	ReasonImage = "image_baseline"
	// ReasonCPUBound: a CPU-bound engine (params: engine).
	ReasonCPUBound = "cpu_bound"
	// ReasonMemtx: 1.5 × memtx memory + 1 GB (params: memtx_gb).
	ReasonMemtx = "memtx"
	// ReasonPDisks: pdisks × 40 GB + 20 GB filesystem (params: pdisks).
	ReasonPDisks = "pdisks"
	// ReasonRunnerLoad: virtual users and load workers (params: vus, load_workers).
	ReasonRunnerLoad = "runner_load"
)

// RequirementReasons is every reason.
var RequirementReasons = []string{ReasonBaseline, ReasonWALArchive, ReasonSharedBuffers, ReasonProxy, ReasonEtcd, ReasonImage, ReasonCPUBound, ReasonMemtx, ReasonPDisks, ReasonRunnerLoad}

// Because is a requirement with its reason.
func Because(cpu int, memGB, diskGB float64, reason string, params ...string) Requirement {
	r := Requirement{CPU: cpu, MemoryGB: memGB, DiskGB: diskGB, Reason: reason}
	if len(params) > 0 {
		r.ReasonParams = map[string]string{}
		for i := 0; i+1 < len(params); i += 2 {
			r.ReasonParams[params[i]] = params[i+1]
		}
	}
	return r
}

// Plan is the compiled topology.
type Plan struct {
	Kind catalog.DatabaseKind
	// Mode is the flavor the nodes do not show (Modes); empty for none.
	Mode  string
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
	Login    Login
	// URL, when set, is the whole client URL and wins over the one built
	// from the fields above.
	URL string
}

// Login is what a client signs in with, and the database it opens.
type Login struct {
	User, Password, Database string
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

// Add puts count machines of a role running an engine into the plan.
func (p *Plan) Add(role, engine string, count int) {
	if count <= 0 {
		return
	}
	p.Nodes = append(p.Nodes, Node{Role: role, Engine: engine, Count: count})
}

// Colocate puts a role on the machines of another one.
func (p *Plan) Colocate(role, engine, with string) {
	p.Nodes = append(p.Nodes, Node{Role: role, Engine: engine, Count: 0, ColocatedWith: with})
}

// Modes: the flavor of a topology its nodes do not show (the contract's
// TopologyMode); empty for a plain one. The values are the params' own.
const (
	ModeSemiSync   = "semi_sync"
	ModeGroup      = "group"
	ModeGalera     = "galera"
	ModePatroni    = "patroni"
	ModeBlock42    = "block-4-2"
	ModeMirror3DC  = "mirror-3-dc"
	ModeDedicated  = "dedicated"
	ModeServerless = "serverless"
)

// Modes is every mode.
var Modes = []string{ModeSemiSync, ModeGroup, ModeGalera, ModePatroni, ModeBlock42, ModeMirror3DC, ModeDedicated, ModeServerless}

// Shape is a topology as lists show it: the mode and the nodes; the UI
// names it in its language.
type Shape struct {
	Mode  string      `json:"mode,omitempty"`
	Nodes []ShapeNode `json:"nodes"`
}

// ShapeNode is one role of a shape; Count 0 shares another role's machines.
type ShapeNode struct {
	Role   string `json:"role"`
	Engine string `json:"engine,omitempty"`
	Count  int    `json:"count"`
}

// Shape is the plan's shape.
func (p Plan) Shape() Shape {
	out := Shape{Mode: p.Mode, Nodes: make([]ShapeNode, 0, len(p.Nodes))}
	for _, n := range p.Nodes {
		out.Nodes = append(out.Nodes, ShapeNode{Role: n.Role, Engine: n.Engine, Count: n.Count})
	}
	return out
}

// String is the shape for plain-text exports (the UI names it itself):
// "patroni: db ×1, db-replica ×2, patroni (on db), proxy haproxy ×1".
func (s Shape) String() string {
	parts := make([]string, 0, len(s.Nodes))
	for _, n := range s.Nodes {
		switch {
		case n.Count == 0:
			parts = append(parts, n.Role+" (shared)")
		case n.Engine != "" && n.Engine != n.Role:
			parts = append(parts, fmt.Sprintf("%s %s ×%d", n.Role, n.Engine, n.Count))
		default:
			parts = append(parts, fmt.Sprintf("%s ×%d", n.Role, n.Count))
		}
	}
	out := strings.Join(parts, ", ")
	if s.Mode != "" {
		out = s.Mode + ": " + out
	}
	return out
}

// IsZero reports a shape with nothing in it (a run from before shapes).
func (s Shape) IsZero() bool { return s.Mode == "" && len(s.Nodes) == 0 }

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
	return Because(cpu, mem, 20, ReasonRunnerLoad, "vus", strconv.Itoa(maxVUs), "load_workers", strconv.Itoa(maxLoadWorkers))
}

// IntOf reads an integer param, def when absent.
func IntOf(m map[string]any, key string, def int) int {
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

// StrOf reads a string param, def when absent or empty.
func StrOf(m map[string]any, key, def string) string {
	if v, ok := m[key].(string); ok && v != "" {
		return v
	}
	return def
}

// BoolOf reads a boolean param, false when absent.
func BoolOf(m map[string]any, key string) bool {
	v, _ := m[key].(bool) //nolint:errcheck // absent = false
	return v
}
