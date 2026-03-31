---
sidebar_position: 2
---

# DAG Engine

The DAG engine (`internal/core/dag/`) is the execution backbone of Stroppy Cloud. It models a test run as a directed acyclic graph of typed task nodes, validates the graph for correctness, executes nodes with maximum parallelism, and persists state for resumability.

## Core Types

### Graph

```go
type Graph struct {
    Nodes []*Node `json:"nodes"`
}
```

A `Graph` is a collection of nodes. It provides:

- `Add(n *Node) error` -- appends a node, returning an error if the ID is duplicate.
- `Validate() error` -- checks that all dependency references exist and there are no cycles.
- `Ready(done map[string]bool) []*Node` -- returns nodes whose dependencies are all satisfied.
- `MarshalJSON()` -- serializes the graph (validates before marshaling).

### Node

```go
type Node struct {
    ID   string   `json:"id"`
    Type string   `json:"type"`
    Deps []string `json:"deps,omitempty"`
    Task Task     `json:"-"` // not serialized
}
```

Each node has a unique `ID`, a `Type` string that maps to a task factory in the registry, a list of dependency node IDs, and a `Task` implementation that is not serialized (restored from the registry on deserialization).

### Task

```go
type Task interface {
    Execute(nc *NodeContext) error
}
```

Every node carries a `Task` that performs the actual work. The `NodeContext` provides a cancellable `context.Context` and a node-scoped `zap.Logger` that streams log entries to the WebSocket hub.

### Registry

```go
type Registry struct {
    factories map[string]func() Task
}
```

The registry maps node type strings to factory functions. When a serialized graph is restored, the registry creates fresh `Task` instances for each node. This is used during run resumption.

```go
reg := dag.NewRegistry()
reg.Register("install_db", func() dag.Task { return &pgInstallTask{...} })

graph, err := reg.Unmarshal(savedJSON)
```

## Executor

The `Executor` walks the graph in waves:

```go
exec := dag.NewExecutor(runID, graph, storage, logger, sink)
err := exec.Run(ctx)
```

### Execution Algorithm

1. Find all "ready" nodes (dependencies satisfied, not yet done).
2. Launch all ready nodes in parallel goroutines.
3. Wait for the wave to complete.
4. If any node failed, stop (no new nodes start). Save state.
5. If all nodes succeeded, go to step 1.
6. When no more ready nodes exist and all nodes are done, the run is complete.

### Fail-Fast Behavior

On first failure:
- Currently running nodes are allowed to finish.
- No new nodes are started.
- The executor persists the current state (which nodes are done, which failed).
- The error from the first failed node is returned.

### Resumption

```go
exec := dag.NewExecutor(runID, graph, storage, logger, sink)
err := exec.Resume(ctx, registry)
```

`Resume` loads the saved snapshot from storage, restores the graph via the registry, marks previously completed nodes as done, and continues execution from where it stopped. Failed nodes are retried.

## Storage

```go
type Storage interface {
    Save(ctx context.Context, id string, snap *Snapshot) error
    Load(ctx context.Context, id string) (*Snapshot, error)
}
```

A `Snapshot` contains the serialized graph JSON and per-node status:

```go
type Snapshot struct {
    GraphJSON []byte       `json:"graph"`
    Nodes     []NodeStatus `json:"nodes"`
}

type NodeStatus struct {
    ID     string `json:"id"`
    Status Status `json:"status"` // "pending", "done", "failed"
    Error  string `json:"error,omitempty"`
}
```

The production implementation uses BadgerDB, a fast embedded key-value store.

## Node Context and Log Streaming

Each node receives a `NodeContext` with:

- An embedded `context.Context` for cancellation.
- A node-scoped `zap.Logger` tagged with the `node_id`.

If a `LogSink` is configured, every log entry is also forwarded to it. The server's WebSocket hub implements `LogSink`, so all node logs are streamed to connected UI clients in real time.

```go
type LogSink interface {
    WriteLog(ctx context.Context, executionID string, nodeID string, 
             entry zapcore.Entry, fields []zapcore.Field)
}
```

## Cycle Detection

The graph validates itself using a DFS-based cycle detection algorithm (white/gray/black coloring). A cycle causes `Validate()` to return an error, preventing execution.

## DAG Structure for a Typical Run

A single-node Postgres run produces this DAG:

```
network
  |
  v
machines
  |------------------+------------------+
  v                  v                  v
install_db      install_monitor    install_stroppy
  |                  |
  v                  v
configure_db   configure_monitor
  |                  |
  +--------+---------+------ install_stroppy
           v
       run_stroppy
           |
           v
        teardown
```

With HA topologies, additional nodes appear: `install_etcd`, `configure_etcd`, `install_pgbouncer`, `configure_pgbouncer`, `install_proxy`, `configure_proxy`. These are wired as additional dependencies of `run_stroppy`.
