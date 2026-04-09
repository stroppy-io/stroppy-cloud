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
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusDone      Status = "done"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
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
	Script     string       `json:"script,omitempty"`
	Duration   string       `json:"duration,omitempty"`
	VUs        int          `json:"vus,omitempty"`
	DBVersion  string       `json:"db_version,omitempty"`
	NodeCount  int          `json:"node_count,omitempty"`
	Cancelled  bool         `json:"cancelled,omitempty"`
}

// Storage is the interface for persisting executor state.
type Storage interface {
	// Save persists the current execution snapshot.
	Save(ctx context.Context, tenantID, id string, snap *Snapshot) error
	// Load restores a previously saved snapshot. Returns nil, nil if not found.
	Load(ctx context.Context, tenantID, id string) (*Snapshot, error)
	// List returns IDs and summary of all saved snapshots.
	List(ctx context.Context, tenantID string) ([]RunSummary, error)
	// Delete removes a saved snapshot by ID.
	Delete(ctx context.Context, tenantID, id string) error
	// SetBaseline marks a run as the baseline for a given name (e.g., "postgres-16").
	SetBaseline(ctx context.Context, tenantID, name string, runID string) error
	// GetBaseline returns the run ID for a named baseline. Returns "" if not set.
	GetBaseline(ctx context.Context, tenantID, name string) (string, error)
	// ListBaselines returns all named baselines.
	ListBaselines(ctx context.Context, tenantID string) (map[string]string, error)
}
