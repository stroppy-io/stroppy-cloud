package dag

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"go.uber.org/zap/zapcore"
)

// --- test task implementations ---

type noopTask struct{}

func (noopTask) Execute(*NodeContext) error { return nil }

type counterTask struct {
	counter *atomic.Int64
}

func (t *counterTask) Execute(*NodeContext) error {
	t.counter.Add(1)
	return nil
}

type failTask struct {
	err error
}

func (t *failTask) Execute(*NodeContext) error { return t.err }

type ctxTask struct{}

func (ctxTask) Execute(nc *NodeContext) error { return nc.Err() }

type logTask struct{}

func (logTask) Execute(nc *NodeContext) error {
	nc.Log().Info("hello from node")
	return nil
}

// --- Graph tests ---

func TestRoundTrip(t *testing.T) {
	g := New()
	must(t, g.Add(&Node{ID: "a", Type: "setup"}))
	must(t, g.Add(&Node{ID: "b", Type: "run", Deps: []string{"a"}}))
	must(t, g.Add(&Node{ID: "c", Type: "run", Deps: []string{"a"}}))
	must(t, g.Add(&Node{ID: "d", Type: "teardown", Deps: []string{"b", "c"}}))

	data, err := json.Marshal(g)
	must(t, err)

	reg := NewRegistry()
	reg.Register("setup", func() Task { return noopTask{} })
	reg.Register("run", func() Task { return noopTask{} })
	reg.Register("teardown", func() Task { return noopTask{} })

	g2, err := reg.Unmarshal(data)
	must(t, err)

	if len(g2.Nodes) != 4 {
		t.Fatalf("expected 4 nodes, got %d", len(g2.Nodes))
	}
	for _, n := range g2.Nodes {
		if n.Task == nil {
			t.Fatalf("node %q has nil Task after unmarshal", n.ID)
		}
	}
}

func TestReady(t *testing.T) {
	g := New()
	must(t, g.Add(&Node{ID: "a", Type: "x"}))
	must(t, g.Add(&Node{ID: "b", Type: "x", Deps: []string{"a"}}))
	must(t, g.Add(&Node{ID: "c", Type: "x", Deps: []string{"a"}}))
	must(t, g.Add(&Node{ID: "d", Type: "x", Deps: []string{"b", "c"}}))

	ready := g.Ready(map[string]bool{})
	assertIDs(t, ready, "a")

	ready = g.Ready(map[string]bool{"a": true})
	assertIDs(t, ready, "b", "c")

	ready = g.Ready(map[string]bool{"a": true, "b": true, "c": true})
	assertIDs(t, ready, "d")

	ready = g.Ready(map[string]bool{"a": true, "b": true, "c": true, "d": true})
	assertIDs(t, ready)
}

func TestCycleDetected(t *testing.T) {
	g := New()
	must(t, g.Add(&Node{ID: "a", Type: "x", Deps: []string{"b"}}))
	must(t, g.Add(&Node{ID: "b", Type: "x", Deps: []string{"a"}}))

	if err := g.Validate(); err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestDuplicateID(t *testing.T) {
	g := New()
	must(t, g.Add(&Node{ID: "a", Type: "x"}))
	if err := g.Add(&Node{ID: "a", Type: "x"}); err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestUnknownType(t *testing.T) {
	g := New()
	must(t, g.Add(&Node{ID: "a", Type: "missing"}))

	data, err := json.Marshal(g)
	must(t, err)

	reg := NewRegistry()
	if _, err := reg.Unmarshal(data); err == nil {
		t.Fatal("expected unknown type error")
	}
}

// --- Executor tests ---

func TestExecutorHappyPath(t *testing.T) {
	var counter atomic.Int64

	g := New()
	must(t, g.Add(&Node{ID: "a", Type: "x", Task: &counterTask{&counter}}))
	must(t, g.Add(&Node{ID: "b", Type: "x", Task: &counterTask{&counter}, Deps: []string{"a"}}))
	must(t, g.Add(&Node{ID: "c", Type: "x", Task: &counterTask{&counter}, Deps: []string{"a"}}))
	must(t, g.Add(&Node{ID: "d", Type: "x", Task: &counterTask{&counter}, Deps: []string{"b", "c"}}))

	err := NewExecutor("test", g, nil, nil, nil).Run(context.Background())
	must(t, err)

	if counter.Load() != 4 {
		t.Fatalf("expected 4 executions, got %d", counter.Load())
	}
}

func TestExecutorFailFast(t *testing.T) {
	boom := errors.New("boom")

	g := New()
	must(t, g.Add(&Node{ID: "a", Type: "x", Task: &failTask{boom}}))
	must(t, g.Add(&Node{ID: "b", Type: "x", Task: noopTask{}, Deps: []string{"a"}}))

	err := NewExecutor("test", g, nil, nil, nil).Run(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, boom) {
		t.Fatalf("expected boom, got: %v", err)
	}
}

func TestExecutorContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	g := New()
	must(t, g.Add(&Node{ID: "a", Type: "x", Task: ctxTask{}}))

	err := NewExecutor("test", g, nil, nil, nil).Run(ctx)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestExecutorResume(t *testing.T) {
	var calls atomic.Int64
	reg := NewRegistry()
	reg.Register("x", func() Task { return &counterTask{&calls} })

	g := New()
	must(t, g.Add(&Node{ID: "a", Type: "x", Task: noopTask{}}))
	must(t, g.Add(&Node{ID: "b", Type: "x", Task: noopTask{}, Deps: []string{"a"}}))
	must(t, g.Add(&Node{ID: "c", Type: "x", Task: noopTask{}, Deps: []string{"b"}}))

	graphJSON, err := json.Marshal(g)
	must(t, err)

	store := &memStorage{data: map[string]*Snapshot{
		"run-1": {
			GraphJSON: graphJSON,
			Nodes: []NodeStatus{
				{ID: "a", Status: StatusDone},
				{ID: "b", Status: StatusFailed, Error: "transient"},
				{ID: "c", Status: StatusPending},
			},
		},
	}}

	exec := NewExecutor("run-1", g, store, nil, nil)
	must(t, exec.Resume(context.Background(), reg))

	if calls.Load() != 2 {
		t.Fatalf("expected 2 calls (b + c), got %d", calls.Load())
	}
}

func TestExecutorLogSink(t *testing.T) {
	sink := &captureSink{}

	g := New()
	must(t, g.Add(&Node{ID: "a", Type: "x", Task: logTask{}}))

	err := NewExecutor("exec-1", g, nil, nil, sink).Run(context.Background())
	must(t, err)

	if len(sink.entries()) == 0 {
		t.Fatal("expected log entries from sink")
	}

	got := sink.entries()[0]
	if got.executionID != "exec-1" || got.nodeID != "a" {
		t.Fatalf("unexpected entry: exec=%q node=%q", got.executionID, got.nodeID)
	}
}

// --- helpers ---

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func assertIDs(t *testing.T, nodes []*Node, expected ...string) {
	t.Helper()
	got := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		got[n.ID] = true
	}
	if len(got) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, ids(nodes))
	}
	for _, id := range expected {
		if !got[id] {
			t.Fatalf("expected %v, got %v", expected, ids(nodes))
		}
	}
}

func ids(nodes []*Node) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = n.ID
	}
	return out
}

// --- test doubles ---

type memStorage struct {
	mu   sync.Mutex
	data map[string]*Snapshot
}

func (m *memStorage) Save(_ context.Context, id string, snap *Snapshot) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data == nil {
		m.data = make(map[string]*Snapshot)
	}
	m.data[id] = snap
	return nil
}

func (m *memStorage) Load(_ context.Context, id string) (*Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.data[id], nil
}

type logEntry struct {
	executionID string
	nodeID      string
	entry       zapcore.Entry
	fields      []zapcore.Field
}

type captureSink struct {
	mu   sync.Mutex
	logs []logEntry
}

func (s *captureSink) WriteLog(_ context.Context, execID, nodeID string, entry zapcore.Entry, fields []zapcore.Field) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logs = append(s.logs, logEntry{execID, nodeID, entry, fields})
}

func (s *captureSink) entries() []logEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]logEntry{}, s.logs...)
}
