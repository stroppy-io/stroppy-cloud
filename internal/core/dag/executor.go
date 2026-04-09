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
	// Node state machine: pending → running → done/failed/cancelled.
	nodeStatus map[string]Status
	nodeErrors map[string]string // id → error message
	errs       []error           // original errors for return
	cancelled  bool              // true when cancel was requested
	mu         sync.Mutex

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
		id:         id,
		tenantID:   tenantID,
		graph:      g,
		storage:    storage,
		log:        log,
		sink:       sink,
		retry:      DefaultRetryPolicy(),
		nodeStatus: make(map[string]Status, len(g.Nodes)),
		nodeErrors: make(map[string]string),
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
	e.nodeStatus[id] = StatusDone
}

// Run executes the graph to completion using a proper state machine.
//
// Node FSM: pending → running → done | failed | cancelled
// Run FSM:  running → completed | failed | cancelling → cancelled
func (e *Executor) Run(ctx context.Context) error {
	if e.startedAt.IsZero() {
		e.startedAt = time.Now()
	}

	// cancellableCtx for non-MustComplete tasks.
	cancellableCtx, cancelNonCritical := context.WithCancel(ctx)
	defer cancelNonCritical()

	for {
		// Check for cancel before scheduling new work.
		if ctx.Err() != nil {
			e.mu.Lock()
			e.cancelled = true
			// Mark all pending nodes as cancelled (not failed — they never ran).
			for _, n := range e.graph.Nodes {
				if e.nodeStatus[n.ID] == "" || e.nodeStatus[n.ID] == StatusPending {
					if !n.AlwaysRun {
						e.nodeStatus[n.ID] = StatusCancelled
					}
				}
			}
			e.mu.Unlock()
			_ = e.save(context.Background())
			break
		}

		ready := e.getReady()
		if len(ready) == 0 {
			break
		}

		// Mark nodes as running.
		e.mu.Lock()
		for _, n := range ready {
			e.nodeStatus[n.ID] = StatusRunning
		}
		e.mu.Unlock()
		_ = e.save(context.Background())

		var wg sync.WaitGroup
		wg.Add(len(ready))
		for _, n := range ready {
			go func(n *Node) {
				defer wg.Done()
				if n.MustComplete {
					e.executeWithRetry(context.Background(), n)
				} else {
					e.executeWithRetry(cancellableCtx, n)
				}
			}(n)
		}
		wg.Wait()

		if e.hasFailedOrCancelled() {
			_ = e.save(context.Background())
			break
		}
	}

	cancelNonCritical()

	// Teardown (AlwaysRun) — must complete fully regardless of cancel.
	if e.hasFailedOrCancelled() {
		teardownCtx, teardownCancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer teardownCancel()
		e.runAlwaysRunNodes(teardownCtx)
	}

	_ = e.save(context.Background())

	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.errs) > 0 {
		return e.errs[0]
	}
	if e.cancelled {
		return fmt.Errorf("run cancelled")
	}

	completed := 0
	for _, s := range e.nodeStatus {
		if s == StatusDone {
			completed++
		}
	}
	if completed < len(e.graph.Nodes) {
		return fmt.Errorf("dag: %d nodes not reached", len(e.graph.Nodes)-completed)
	}
	return nil
}

// runAlwaysRunNodes executes AlwaysRun nodes (teardown) after the main loop stops.
func (e *Executor) runAlwaysRunNodes(ctx context.Context) {
	for {
		e.mu.Lock()
		doneCopy := make(map[string]bool, len(e.nodeStatus))
		for id, s := range e.nodeStatus {
			if s == StatusDone || s == StatusFailed || s == StatusCancelled {
				doneCopy[id] = true
			}
		}
		e.mu.Unlock()

		ready := e.graph.ReadyAlwaysRun(doneCopy)
		if len(ready) == 0 {
			return
		}

		e.mu.Lock()
		for _, n := range ready {
			e.nodeStatus[n.ID] = StatusRunning
		}
		e.mu.Unlock()

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

// executeWithRetry runs a node with retries. Updates state to done/failed/cancelled.
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
				// Context cancelled during retry wait.
				e.mu.Lock()
				e.nodeStatus[n.ID] = StatusCancelled
				e.nodeErrors[n.ID] = "cancelled"
				e.mu.Unlock()
				return
			case <-time.After(delay):
			}
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

	e.markFailed(n.ID, fmt.Errorf("after %d attempts: %w", e.retry.MaxRetries+1, lastErr))
}

func (e *Executor) getReady() []*Node {
	e.mu.Lock()
	defer e.mu.Unlock()
	// Stop scheduling if any node failed or cancelled (except during cancel — AlwaysRun handled separately).
	for _, s := range e.nodeStatus {
		if s == StatusFailed {
			return nil
		}
	}
	if e.cancelled {
		return nil
	}
	doneCopy := make(map[string]bool, len(e.nodeStatus))
	for id, s := range e.nodeStatus {
		if s == StatusDone {
			doneCopy[id] = true
		}
	}
	return e.graph.Ready(doneCopy)
}

func (e *Executor) markDone(ctx context.Context, id string) {
	e.mu.Lock()
	e.nodeStatus[id] = StatusDone
	e.mu.Unlock()
	_ = e.save(ctx)
}

func (e *Executor) markFailed(id string, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.nodeStatus[id] = StatusFailed
	e.nodeErrors[id] = err.Error()
	e.errs = append(e.errs, fmt.Errorf("node %q: %w", id, err))
}

func (e *Executor) hasFailedOrCancelled() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cancelled {
		return true
	}
	for _, s := range e.nodeStatus {
		if s == StatusFailed {
			return true
		}
	}
	return false
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
		s := e.nodeStatus[n.ID]
		if s == "" {
			s = StatusPending
		}
		ns := NodeStatus{ID: n.ID, Status: s}
		if msg := e.nodeErrors[n.ID]; msg != "" {
			ns.Error = msg
		}
		nodes = append(nodes, ns)
	}

	var state *RunState
	if e.stateExporter != nil {
		state = e.stateExporter()
	}

	finishedAt := time.Time{}
	allDone := true
	for _, ns := range nodes {
		if ns.Status == StatusPending || ns.Status == StatusRunning {
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
