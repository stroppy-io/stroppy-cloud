// Package topology is the machine plan a run needs: which roles, how many
// of each, which engine they run, how traffic flows between them, where a
// client connects, and the minimum hardware every role wants. Each kind
// builds its plan in dbkind; the RunSpec compiler and the test validator
// consume it, the UI shows it as the topology preview.
package topology

import (
	"fmt"
	"math"
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

// Label renders "engine + N replica(s) + patroni + haproxy".
func Label(engine string, replicas int, replicaWord string, ha bool, haWord string, proxy bool, proxyWord string, extra bool, extraWord string) string {
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
