package dag

import (
	"context"
	"encoding/json"
	"time"
)

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

// RunState holds the runtime state needed to recover a run after restart.
type RunState struct {
	Provider     string          `json:"provider"`
	RunConfig    json.RawMessage `json:"run_config"`
	Targets      []TargetInfo    `json:"targets"`
	ContainerIDs []string        `json:"container_ids"`
	NetworkID    string          `json:"network_id"`
	DBHost       string          `json:"db_host"`
	DBPort       int             `json:"db_port"`
}

// TargetInfo is a serializable representation of an agent target.
type TargetInfo struct {
	ID           string `json:"id"`
	Host         string `json:"host"`
	InternalHost string `json:"internal_host"`
	AgentPort    int    `json:"agent_port"`
	Role         string `json:"role"` // "database", "monitor", "stroppy", "proxy"
}

// Snapshot is the full persisted state of a graph execution.
type Snapshot struct {
	GraphJSON  []byte       `json:"graph"`
	Nodes      []NodeStatus `json:"nodes"`
	State      *RunState    `json:"state,omitempty"`
	StartedAt  time.Time    `json:"started_at,omitempty"`
	FinishedAt time.Time    `json:"finished_at,omitempty"`
}

// RunSummary is a compact view of a saved snapshot for listing.
type RunSummary struct {
	ID         string       `json:"id"`
	Nodes      []NodeStatus `json:"nodes"`
	Total      int          `json:"total"`
	Done       int          `json:"done"`
	Failed     int          `json:"failed"`
	Pending    int          `json:"pending"`
	StartedAt  time.Time    `json:"started_at,omitempty"`
	FinishedAt time.Time    `json:"finished_at,omitempty"`
	DBKind     string       `json:"db_kind,omitempty"`
	Provider   string       `json:"provider,omitempty"`
}

// Storage is the interface for persisting executor state.
type Storage interface {
	// Save persists the current execution snapshot.
	Save(ctx context.Context, id string, snap *Snapshot) error
	// Load restores a previously saved snapshot. Returns nil, nil if not found.
	Load(ctx context.Context, id string) (*Snapshot, error)
	// List returns IDs and summary of all saved snapshots.
	List(ctx context.Context) ([]RunSummary, error)
	// Delete removes a saved snapshot by ID.
	Delete(ctx context.Context, id string) error
}
