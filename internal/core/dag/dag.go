package dag

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Task is the interface that node implementations must satisfy.
type Task interface {
	Execute(nc *NodeContext) error
}

// Node is a single step in the DAG.
type Node struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	// Deps lists IDs of nodes that must complete before this one.
	Deps []string `json:"deps,omitempty"`
	// AlwaysRun means this node executes even when upstream nodes have failed.
	// Its deps are considered resolved when they are either done or failed.
	// Use this for cleanup/teardown nodes that must run regardless of errors.
	AlwaysRun bool `json:"always_run,omitempty"`

	// Task is restored from the registry on Unmarshal; not serialized.
	Task Task `json:"-"`
}

// Graph is a serializable directed acyclic graph.
// Nodes carry a Type that maps to an ExecuteFunc via a Registry.
type Graph struct {
	Nodes []*Node `json:"nodes"`
}

// New creates an empty graph.
func New() *Graph {
	return &Graph{}
}

// Add appends a node. Returns error if the ID is duplicate.
func (g *Graph) Add(n *Node) error {
	for _, existing := range g.Nodes {
		if existing.ID == n.ID {
			return fmt.Errorf("dag: duplicate node id %q", n.ID)
		}
	}
	g.Nodes = append(g.Nodes, n)
	return nil
}

// Validate checks that all deps reference existing nodes and there are no cycles.
func (g *Graph) Validate() error {
	idx := g.index()

	for _, n := range g.Nodes {
		for _, dep := range n.Deps {
			if _, ok := idx[dep]; !ok {
				return fmt.Errorf("dag: node %q depends on unknown node %q", n.ID, dep)
			}
		}
	}

	return g.detectCycle(idx)
}

// Ready returns nodes whose dependencies are all satisfied.
// done is the set of completed node IDs.
func (g *Graph) Ready(done map[string]bool) []*Node {
	var ready []*Node
	for _, n := range g.Nodes {
		if done[n.ID] {
			continue
		}
		if g.depsComplete(n, done) {
			ready = append(ready, n)
		}
	}
	return ready
}

// MarshalJSON implements json.Marshaler.
func (g *Graph) MarshalJSON() ([]byte, error) {
	if err := g.Validate(); err != nil {
		return nil, err
	}
	type alias Graph
	return json.Marshal((*alias)(g))
}

// Registry maps node Type strings to Task factories.
type Registry struct {
	factories map[string]func() Task
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{factories: make(map[string]func() Task)}
}

// Register binds a node type to a factory that produces its Task.
func (r *Registry) Register(nodeType string, factory func() Task) {
	r.factories[nodeType] = factory
}

// Unmarshal deserializes a Graph from JSON and restores callbacks from the registry.
func (r *Registry) Unmarshal(data []byte) (*Graph, error) {
	var g Graph
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, fmt.Errorf("dag: unmarshal: %w", err)
	}

	for _, n := range g.Nodes {
		factory, ok := r.factories[n.Type]
		if !ok {
			return nil, fmt.Errorf("dag: unknown node type %q (node %q)", n.Type, n.ID)
		}
		n.Task = factory()
	}

	if err := g.Validate(); err != nil {
		return nil, err
	}

	return &g, nil
}

// ReadyAlwaysRun returns AlwaysRun nodes whose dependencies are all resolved
// (either completed or failed). This is used after a failure to find cleanup nodes.
func (g *Graph) ReadyAlwaysRun(done map[string]bool, failed map[string]bool) []*Node {
	resolved := make(map[string]bool, len(done)+len(failed))
	for id := range done {
		resolved[id] = true
	}
	for id := range failed {
		resolved[id] = true
	}

	var ready []*Node
	for _, n := range g.Nodes {
		if done[n.ID] || failed[n.ID] || !n.AlwaysRun {
			continue
		}
		if g.depsComplete(n, resolved) {
			ready = append(ready, n)
		}
	}
	return ready
}

// --- internals ---

func (g *Graph) index() map[string]*Node {
	idx := make(map[string]*Node, len(g.Nodes))
	for _, n := range g.Nodes {
		idx[n.ID] = n
	}
	return idx
}

func (g *Graph) depsComplete(n *Node, done map[string]bool) bool {
	for _, dep := range n.Deps {
		if !done[dep] {
			return false
		}
	}
	return true
}

func (g *Graph) detectCycle(idx map[string]*Node) error {
	const (
		white = 0
		gray  = 1
		black = 2
	)

	color := make(map[string]int, len(idx))

	var visit func(string) error
	visit = func(id string) error {
		color[id] = gray
		for _, dep := range idx[id].Deps {
			switch color[dep] {
			case gray:
				return fmt.Errorf("dag: cycle detected at node %q", dep)
			case white:
				if err := visit(dep); err != nil {
					return err
				}
			}
		}
		color[id] = black
		return nil
	}

	var errs []error
	for id := range idx {
		if color[id] == white {
			if err := visit(id); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}
