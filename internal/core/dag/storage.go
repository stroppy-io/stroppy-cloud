package dag

import "context"

// NodeStatus represents the persisted state of a single node.
type NodeStatus struct {
	ID     string `json:"id"`
	Status Status `json:"status"`
	Error  string `json:"error,omitempty"`
}

// Status is the execution state of a node.
type Status string

const (
	StatusPending Status = "pending"
	StatusDone    Status = "done"
	StatusFailed  Status = "failed"
)

// Snapshot is the full persisted state of a graph execution.
type Snapshot struct {
	GraphJSON []byte       `json:"graph"`
	Nodes     []NodeStatus `json:"nodes"`
}

// Storage is the interface for persisting executor state.
type Storage interface {
	// Save persists the current execution snapshot.
	Save(ctx context.Context, id string, snap *Snapshot) error
	// Load restores a previously saved snapshot. Returns nil, nil if not found.
	Load(ctx context.Context, id string) (*Snapshot, error)
}
