package dag

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"go.uber.org/zap"
)

// Executor walks a Graph and runs nodes as their dependencies complete.
// Independent branches run in parallel.
type Executor struct {
	id      string
	graph   *Graph
	storage Storage
	log     *zap.Logger
	sink    LogSink
	done    map[string]bool
	failed  map[string]string // id → error message for snapshots
	errs    []error           // original errors for return
	mu      sync.Mutex
}

// NewExecutor creates an executor for the given graph.
// id identifies this execution for storage. storage may be nil.
// log is the base zap logger; sink (optional) receives per-node log entries for streaming.
func NewExecutor(id string, g *Graph, storage Storage, log *zap.Logger, sink LogSink) *Executor {
	if log == nil {
		log = zap.NewNop()
	}
	return &Executor{
		id:      id,
		graph:   g,
		storage: storage,
		log:     log,
		sink:    sink,
		done:    make(map[string]bool, len(g.Nodes)),
		failed:  make(map[string]string),
	}
}

// Resume restores state from storage and continues execution.
// If no saved state exists, starts from the beginning.
func (e *Executor) Resume(ctx context.Context, reg *Registry) error {
	if e.storage == nil {
		return e.Run(ctx)
	}

	snap, err := e.storage.Load(ctx, e.id)
	if err != nil {
		return fmt.Errorf("dag: load state: %w", err)
	}

	if snap != nil {
		restored, err := reg.Unmarshal(snap.GraphJSON)
		if err != nil {
			return fmt.Errorf("dag: restore graph: %w", err)
		}
		e.graph = restored

		for _, ns := range snap.Nodes {
			switch ns.Status {
			case StatusDone:
				e.done[ns.ID] = true
			case StatusFailed:
				// failed nodes are retried on resume
			}
		}
	}

	return e.Run(ctx)
}

// Run executes the graph to completion.
// It fails fast: on first error, pending nodes finish but no new ones start.
func (e *Executor) Run(ctx context.Context) error {
	for {
		ready := e.getReady()
		if len(ready) == 0 {
			break
		}

		var wg sync.WaitGroup
		wg.Add(len(ready))

		for _, n := range ready {
			go func(n *Node) {
				defer wg.Done()
				nc := &NodeContext{
					Context: ctx,
					log:     newNodeLogger(e.log, e.sink, e.id, n.ID),
				}
				if err := n.Task.Execute(nc); err != nil {
					e.markFailed(n.ID, err)
					return
				}
				e.markDone(ctx, n.ID)
			}(n)
		}

		wg.Wait()

		if e.hasFailed() {
			_ = e.save(ctx)
			break
		}
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.errs) > 0 {
		return e.errs[0]
	}

	if len(e.done) < len(e.graph.Nodes) {
		return fmt.Errorf("dag: %d nodes not reached (upstream failure)", len(e.graph.Nodes)-len(e.done))
	}

	return nil
}

func (e *Executor) getReady() []*Node {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.failed) > 0 {
		return nil
	}
	return e.graph.Ready(e.done)
}

func (e *Executor) markDone(ctx context.Context, id string) {
	e.mu.Lock()
	e.done[id] = true
	e.mu.Unlock()

	_ = e.save(ctx)
}

func (e *Executor) markFailed(id string, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.failed[id] = err.Error()
	e.errs = append(e.errs, fmt.Errorf("node %q: %w", id, err))
}

func (e *Executor) hasFailed() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.failed) > 0
}

func (e *Executor) save(ctx context.Context) error {
	if e.storage == nil {
		return nil
	}

	e.mu.Lock()
	snap := e.snapshot()
	e.mu.Unlock()

	return e.storage.Save(ctx, e.id, snap)
}

// snapshot builds the current state for persistence. Caller must hold e.mu.
func (e *Executor) snapshot() *Snapshot {
	graphJSON, _ := json.Marshal(e.graph)

	nodes := make([]NodeStatus, 0, len(e.graph.Nodes))
	for _, n := range e.graph.Nodes {
		ns := NodeStatus{ID: n.ID, Status: StatusPending}
		if e.done[n.ID] {
			ns.Status = StatusDone
		} else if msg, ok := e.failed[n.ID]; ok {
			ns.Status = StatusFailed
			ns.Error = msg
		}
		nodes = append(nodes, ns)
	}

	return &Snapshot{
		GraphJSON: graphJSON,
		Nodes:     nodes,
	}
}
