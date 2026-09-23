package run

import (
	"encoding/json"
	"time"
)

/*
STATE is the projection of a run: what the pipeline told us through
milestones, plus what Graphene's own event stream says about the workflow.
Persisted as jsonb on the run; the UI's overview is rendered from it, and
the WebSocket pushes it on every change.
*/

// State is the runtime projection.
type State struct {
	Phases     []PhaseState              `json:"phases"`
	Machines   map[string]MachineState   `json:"machines,omitempty"`
	Containers map[string]ContainerState `json:"containers,omitempty"`
	Segments   []SegmentState            `json:"segments,omitempty"`
	Baseline   *BaselineState            `json:"baseline,omitempty"`
	Pending    *PendingActivity          `json:"pending,omitempty"`
	// Stand is where the run handed its infrastructure over (keep).
	Stand      *StandState `json:"stand,omitempty"`
	ObservedAt time.Time   `json:"observed_at"`
	// Degraded lists why the projection may lag (stream lost, …).
	Degraded []string `json:"degraded,omitempty"`
}

// StandState is the kept infrastructure: the pipeline moved its root
// resource (the network, with everything under it) to the pipeline's stand.
type StandState struct {
	// Root is the held resource; stand commands address it.
	Root string `json:"root"`
	// Keep is the deadline the pipeline asked for ("1h0m0s").
	Keep string `json:"keep,omitempty"`
}

// PhaseState is one phase with its steps (activities).
type PhaseState struct {
	ID         Phase      `json:"id"`
	Status     string     `json:"status"` // pending|running|completed|failed|skipped|cancelled
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Error      string     `json:"error,omitempty"`
	Steps      []Step     `json:"steps,omitempty"`
}

// Step is one activity inside a phase.
type Step struct {
	ID         string     `json:"id"`
	Title      string     `json:"title"`
	Status     string     `json:"status"`
	Role       string     `json:"role,omitempty"`
	Machine    string     `json:"machine,omitempty"`
	Attempt    int        `json:"attempt,omitempty"`
	Error      string     `json:"error,omitempty"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

// MachineState is one VM/agent.
type MachineState struct {
	Role               string     `json:"role"`
	Status             string     `json:"status"`   // pending|creating|ready|failed|deleting|deleted
	Presence           string     `json:"presence"` // online|stale|offline|terminated
	ProviderResourceID string     `json:"provider_resource_id,omitempty"`
	PrivateIP          string     `json:"private_ip,omitempty"`
	PublicIP           string     `json:"public_ip,omitempty"`
	AgentID            string     `json:"agent_id,omitempty"`
	LastHeartbeatAt    *time.Time `json:"last_heartbeat_at,omitempty"`
}

// ContainerState is one deployed container.
type ContainerState struct {
	Role    string `json:"role"`
	Machine string `json:"machine"`
	Status  string `json:"status"`
	ID      string `json:"id,omitempty"`
}

// SegmentState is one workload segment.
type SegmentState struct {
	Name       string             `json:"name"`
	Index      int                `json:"index"`
	Script     string             `json:"script,omitempty"`
	Status     string             `json:"status"` // pending|running|completed|failed|skipped|cancelled
	StartedAt  *time.Time         `json:"started_at,omitempty"`
	FinishedAt *time.Time         `json:"finished_at,omitempty"`
	Error      string             `json:"error,omitempty"`
	Metrics    map[string]float64 `json:"metrics,omitempty"`
}

// BaselineState is the runner baseline.
type BaselineState struct {
	Status string `json:"status"`
	OK     bool   `json:"ok"`
	Error  string `json:"error,omitempty"`
}

// PendingActivity is what the run is stuck on.
type PendingActivity struct {
	Activity    string    `json:"activity"`
	Attempt     int       `json:"attempt"`
	Since       time.Time `json:"since"`
	LastFailure string    `json:"last_failure,omitempty"`
}

// NewState seeds the phases from the spec so the UI has a skeleton before
// the first event.
func NewState(machines []MachineSnapshot, segments []string) State {
	st := State{Phases: make([]PhaseState, 0, len(PhaseOrder)), Machines: map[string]MachineState{}, ObservedAt: time.Now().UTC()}
	for _, p := range PhaseOrder {
		if p == PhaseQueued || p == PhaseDone {
			continue
		}
		st.Phases = append(st.Phases, PhaseState{ID: p, Status: "pending"})
	}
	for _, m := range machines {
		st.Machines[m.Name] = MachineState{Role: m.Role, Status: "pending", Presence: "offline"}
	}
	for i, s := range segments {
		st.Segments = append(st.Segments, SegmentState{Name: s, Index: i, Status: "pending"})
	}
	return st
}

// Progress is the 0..100 estimate from the phase and segments.
func (st State) Progress(phase Phase) float64 {
	weights := map[Phase]float64{PhaseQueued: 0, PhaseProvisioning: 10, PhaseDeploying: 30, PhaseWorkload: 40, PhaseCollecting: 90, PhaseTeardown: 95, PhaseDone: 100}
	base := weights[phase]
	if phase == PhaseWorkload && len(st.Segments) > 0 {
		done := 0
		for _, s := range st.Segments {
			if s.Status == "completed" || s.Status == "failed" || s.Status == "skipped" {
				done++
			}
		}
		base += 50 * float64(done) / float64(len(st.Segments))
	}
	return base
}

// CurrentSegment is the running segment's name.
func (st State) CurrentSegment() string {
	for _, s := range st.Segments {
		if s.Status == "running" {
			return s.Name
		}
	}
	return ""
}

// JSON renders the state.
func (st State) JSON() json.RawMessage {
	raw, _ := json.Marshal(st) //nolint:errcheck // plain struct
	return raw
}
