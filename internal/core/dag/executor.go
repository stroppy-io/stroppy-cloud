package dag

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

const (
	// DefaultMaxRetries is the default number of retry attempts per node.
	DefaultMaxRetries = 3
	// DefaultRetryBaseDelay is the initial backoff delay between retries.
	DefaultRetryBaseDelay = 5 * time.Second
	// DefaultRetryMaxDelay caps the exponential backoff.
	DefaultRetryMaxDelay = 60 * time.Second
)

// RetryPolicy controls how failed nodes are retried.
type RetryPolicy struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
}

// DefaultRetryPolicy returns the default retry policy.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxRetries: DefaultMaxRetries,
		BaseDelay:  DefaultRetryBaseDelay,
		MaxDelay:   DefaultRetryMaxDelay,
	}
}

// Executor walks a Graph and runs nodes as their dependencies complete.
// Failed nodes are automatically retried with exponential backoff.
type Executor struct {
	id       string
	tenantID string
	graph    *Graph
	storage  Storage
	log      *zap.Logger
	sink     LogSink
	retry    RetryPolicy
	done     map[string]bool
	failed   map[string]string // id → error message for snapshots
	errs     []error           // original errors for return
	mu       sync.Mutex

	startedAt time.Time

	// stateExporter is called before each snapshot save to capture
	// the current run state (targets, container IDs, etc.) for recovery.
	stateExporter func() *RunState
}

// NewExecutor creates an executor for the given graph.
func NewExecutor(tenantID, id string, g *Graph, storage Storage, log *zap.Logger, sink LogSink) *Executor {
	if log == nil {
		log = zap.NewNop()
	}
	return &Executor{
		id:       id,
		tenantID: tenantID,
		graph:    g,
		storage:  storage,
		log:      log,
		sink:     sink,
		retry:    DefaultRetryPolicy(),
		done:     make(map[string]bool, len(g.Nodes)),
		failed:   make(map[string]string),
	}
}

// SetRetryPolicy overrides the default retry policy.
func (e *Executor) SetRetryPolicy(p RetryPolicy) {
	e.retry = p
}

// SetStateExporter registers a callback that exports the current run state
// for inclusion in every snapshot. This enables run recovery after restart.
func (e *Executor) SetStateExporter(fn func() *RunState) {
	e.stateExporter = fn
}

// MarkNodeDone marks a node as completed without executing it.
// Used during recovery to restore previously completed nodes.
func (e *Executor) MarkNodeDone(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.done[id] = true
}

// Run executes the graph to completion.
// Failed nodes are retried with exponential backoff up to MaxRetries times.
// After all retries are exhausted, AlwaysRun nodes (e.g. teardown) still execute,
// then the run fails with the original error.
func (e *Executor) Run(ctx context.Context) error {
	if e.startedAt.IsZero() {
		e.startedAt = time.Now()
	}
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
				e.executeWithRetry(ctx, n)
			}(n)
		}

		wg.Wait()

		if e.hasFailed() {
			_ = e.save(ctx)
			break
		}
	}

	// Run AlwaysRun nodes (teardown) even after failure.
	if e.hasFailed() {
		e.runAlwaysRunNodes(ctx)
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

// runAlwaysRunNodes executes AlwaysRun nodes whose deps are all resolved (done or failed).
// This ensures cleanup/teardown runs even when upstream nodes have failed.
func (e *Executor) runAlwaysRunNodes(ctx context.Context) {
	for {
		e.mu.Lock()
		failedSet := make(map[string]bool, len(e.failed))
		for id := range e.failed {
			failedSet[id] = true
		}
		doneCopy := make(map[string]bool, len(e.done))
		for id := range e.done {
			doneCopy[id] = true
		}
		e.mu.Unlock()

		ready := e.graph.ReadyAlwaysRun(doneCopy, failedSet)
		if len(ready) == 0 {
			return
		}

		var wg sync.WaitGroup
		wg.Add(len(ready))
		for _, n := range ready {
			go func(n *Node) {
				defer wg.Done()
				e.executeWithRetry(ctx, n)
			}(n)
		}
		wg.Wait()

		_ = e.save(ctx)
	}
}

// executeWithRetry runs a node's task with automatic retries and exponential backoff.
func (e *Executor) executeWithRetry(ctx context.Context, n *Node) {
	nc := &NodeContext{
		Context: ctx,
		log:     newNodeLogger(e.log, e.sink, e.id, n.ID),
	}

	var lastErr error
	delay := e.retry.BaseDelay

	for attempt := 0; attempt <= e.retry.MaxRetries; attempt++ {
		if attempt > 0 {
			nc.Log().Warn("retrying node",
				zap.Int("attempt", attempt),
				zap.Int("max_retries", e.retry.MaxRetries),
				zap.Duration("backoff", delay),
				zap.Error(lastErr),
			)
			select {
			case <-ctx.Done():
				e.markFailed(n.ID, ctx.Err())
				return
			case <-time.After(delay):
			}
			// Exponential backoff with cap.
			delay *= 2
			if delay > e.retry.MaxDelay {
				delay = e.retry.MaxDelay
			}
		}

		err := n.Task.Execute(nc)
		if err == nil {
			e.markDone(ctx, n.ID)
			return
		}

		lastErr = err
		nc.Log().Error("node execution failed",
			zap.Int("attempt", attempt+1),
			zap.Int("max_attempts", e.retry.MaxRetries+1),
			zap.Error(err),
		)
	}

	// All retries exhausted.
	e.markFailed(n.ID, fmt.Errorf("after %d attempts: %w", e.retry.MaxRetries+1, lastErr))
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

	return e.storage.Save(ctx, e.tenantID, e.id, snap)
}

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

	var state *RunState
	if e.stateExporter != nil {
		state = e.stateExporter()
	}

	// Compute finishedAt if all nodes are done or failed.
	finishedAt := time.Time{}
	allDone := true
	for _, ns := range nodes {
		if ns.Status == StatusPending {
			allDone = false
			break
		}
	}
	if allDone {
		finishedAt = time.Now()
	}

	return &Snapshot{
		GraphJSON:  graphJSON,
		Nodes:      nodes,
		State:      state,
		StartedAt:  e.startedAt,
		FinishedAt: finishedAt,
	}
}
