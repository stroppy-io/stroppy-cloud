package run

import (
	"encoding/json"
	"strings"
	"time"
)

/*
PROJECTION: Graphene's event stream → State + status. Two vocabularies
meet here:

  - Graphene's own kinds (run-started/completed/failed/canceled,
    activity-scheduled/started/completed/failed/timed-out) — the workflow
    skeleton and the steps of every phase;
  - the pipeline's milestones (phase.*, machine.ready, container.ready,
    segment.*, baseline.*, stand.kept, result.published), delivered as
    events of kind `note` whose subject is the milestone's name and whose
    input is its payload.

Apply is pure: it takes one raw event and returns what changed, so it is
unit-testable without Graphene and replayable from the stored timeline.
*/

// RawEvent is a Graphene observe event as the client hands it over.
type RawEvent struct {
	ID         int64
	At         time.Time
	Kind       string
	Subject    string
	Agent      string
	Status     string
	Error      string
	Attempt    int
	Input      json.RawMessage
	Result     json.RawMessage
	ActivityID string
}

// Milestone is a decoded pipeline note.
type Milestone struct {
	Name    string         `json:"name"`
	Payload map[string]any `json:"payload"`
}

// Outcome is what applying one event changed.
type Outcome struct {
	// Timeline is the event to store (nil = not worth the timeline).
	Timeline *Event
	// Status/Phase are set when the run's status changed.
	Status Status
	Phase  Phase
	Reason string
	// Finished marks a terminal transition.
	Finished bool
	// ResultReady says the pipeline published its result.
	ResultReady bool
}

// milestone decodes a note event: the name is the subject, the payload
// the input.
func (e RawEvent) milestone() (Milestone, bool) {
	if e.Kind != "note" || e.Subject == "" {
		return Milestone{}, false
	}
	m := Milestone{Name: e.Subject}
	if len(e.Input) > 0 {
		_ = json.Unmarshal(e.Input, &m.Payload) //nolint:errcheck // a payloadless note is a note
	}
	return m, true
}

// Apply folds one event into the state.
func (st *State) Apply(current Status, e RawEvent) Outcome {
	st.ObservedAt = e.At
	if m, ok := e.milestone(); ok {
		return st.applyMilestone(e, m)
	}
	switch e.Kind {
	case "run-started":
		return Outcome{Status: StatusRunning, Phase: PhaseProvisioning, Timeline: e.timeline("Run started", "")}
	case "run-completed":
		st.finishPhases("completed")
		return Outcome{Status: StatusCompleted, Phase: PhaseDone, Finished: true, Timeline: e.timeline("Run completed", "")}
	case "run-failed", "run-timed-out", "run-terminated":
		st.finishPhases("failed")
		reason := e.Error
		if reason == "" {
			reason = strings.TrimPrefix(e.Kind, "run-")
		}
		return Outcome{Status: StatusFailed, Phase: PhaseDone, Reason: reason, Finished: true, Timeline: e.timeline("Run failed", reason)}
	case "run-canceled", "run-cancelled":
		st.finishPhases("cancelled")
		return Outcome{Status: StatusCancelled, Phase: PhaseDone, Finished: true, Timeline: e.timeline("Run cancelled", "")}
	case "activity-scheduled", "activity-started", "activity-completed", "activity-failed", "activity-timed-out":
		st.applyActivity(e)
		if e.Kind == "activity-failed" || e.Kind == "activity-timed-out" {
			return Outcome{Timeline: e.timeline("Step failed: "+shortActivity(e.Subject), e.Error)}
		}
		return Outcome{}
	}
	return Outcome{}
}

func (e RawEvent) timeline(title, errText string) *Event {
	return &Event{GrapheneID: e.ID, At: e.At, Kind: e.Kind, Title: title, Subject: e.Subject, Status: e.Status, Error: errText, Attempt: e.Attempt}
}

func (st *State) applyMilestone(e RawEvent, m Milestone) Outcome {
	p := m.Payload
	ev := &Event{GrapheneID: e.ID, At: e.At, Kind: m.Name, Subject: str(p, "phase", "machine", "container", "segment"), Payload: p}
	switch m.Name {
	case "phase.started":
		ph := Phase(str(p, "phase"))
		st.phase(ph, func(x *PhaseState) { x.Status = "running"; x.StartedAt = ptr(e.At) })
		ev.Title = "Phase " + string(ph) + " started"
		return Outcome{Status: StatusRunning, Phase: ph, Timeline: ev}
	case "phase.finished":
		ph := Phase(str(p, "phase"))
		st.phase(ph, func(x *PhaseState) { x.Status = "completed"; x.FinishedAt = ptr(e.At) })
		ev.Title = "Phase " + string(ph) + " finished"
		return Outcome{Timeline: ev}
	case "phase.failed":
		ph := Phase(str(p, "phase"))
		st.phase(ph, func(x *PhaseState) { x.Status = "failed"; x.FinishedAt = ptr(e.At); x.Error = str(p, "error") })
		ev.Title = "Phase " + string(ph) + " failed"
		ev.Error = str(p, "error")
		return Outcome{Timeline: ev}
	case "machine.ready":
		name := str(p, "machine")
		ms := st.Machines[name]
		ms.Role, ms.Status, ms.Presence = str(p, "role"), "ready", "online"
		ms.ProviderResourceID, ms.PrivateIP, ms.PublicIP = str(p, "id"), str(p, "private_ip"), str(p, "public_ip")
		if st.Machines == nil {
			st.Machines = map[string]MachineState{}
		}
		st.Machines[name] = ms
		ev.Title = "Machine " + name + " ready"
		return Outcome{Timeline: ev}
	case "container.ready":
		name := str(p, "container")
		if st.Containers == nil {
			st.Containers = map[string]ContainerState{}
		}
		st.Containers[name] = ContainerState{Role: str(p, "role"), Machine: str(p, "machine"), Status: "ready", ID: str(p, "id")}
		ev.Title = "Container " + name + " ready"
		return Outcome{Timeline: ev}
	case "segment.started":
		st.segment(str(p, "segment"), func(s *SegmentState) { s.Status = "running"; s.StartedAt = ptr(e.At); s.Script = str(p, "script") })
		ev.Title = "Segment " + str(p, "segment") + " started"
		return Outcome{Timeline: ev}
	case "segment.finished":
		st.segment(str(p, "segment"), func(s *SegmentState) {
			s.Status = "completed"
			s.FinishedAt = ptr(e.At)
			s.Metrics = floats(p["metrics"])
		})
		ev.Title = "Segment " + str(p, "segment") + " finished"
		return Outcome{Timeline: ev}
	case "segment.failed":
		st.segment(str(p, "segment"), func(s *SegmentState) { s.Status = "failed"; s.FinishedAt = ptr(e.At); s.Error = str(p, "error") })
		ev.Title = "Segment " + str(p, "segment") + " failed"
		ev.Error = str(p, "error")
		return Outcome{Timeline: ev}
	case "baseline.started":
		st.Baseline = &BaselineState{Status: "running"}
		ev.Title = "Baseline started"
		return Outcome{Timeline: ev}
	case "baseline.finished":
		ok, _ := p["ok"].(bool) //nolint:errcheck // absent = false
		st.Baseline = &BaselineState{Status: "completed", OK: ok, Error: str(p, "error")}
		ev.Title = "Baseline finished"
		return Outcome{Timeline: ev}
	case "stand.kept":
		st.Stand = &StandState{Root: str(p, "root"), Keep: str(p, "keep")}
		ev.Title = "Stand kept for " + str(p, "keep")
		return Outcome{Timeline: ev}
	case "result.published":
		ev.Title = "Result published"
		return Outcome{Timeline: ev, ResultReady: true}
	}
	ev.Title = m.Name
	return Outcome{Timeline: ev}
}

// applyActivity keeps steps per phase: the phase is the currently running
// one; steps are named by the activity type.
func (st *State) applyActivity(e RawEvent) {
	ph := st.runningPhase()
	if ph == nil {
		return
	}
	id := e.ActivityID
	if id == "" {
		id = e.Subject
	}
	var step *Step
	for i := range ph.Steps {
		if ph.Steps[i].ID == id {
			step = &ph.Steps[i]
			break
		}
	}
	if step == nil {
		ph.Steps = append(ph.Steps, Step{ID: id, Title: shortActivity(e.Subject), Machine: e.Agent})
		step = &ph.Steps[len(ph.Steps)-1]
	}
	step.Attempt = e.Attempt
	switch e.Kind {
	case "activity-scheduled":
		step.Status = "pending"
	case "activity-started":
		step.Status = "running"
		step.StartedAt = ptr(e.At)
	case "activity-completed":
		step.Status = "completed"
		step.FinishedAt = ptr(e.At)
	case "activity-failed", "activity-timed-out":
		step.Status = "failed"
		step.Error = e.Error
		step.FinishedAt = ptr(e.At)
	}
}

func (st *State) runningPhase() *PhaseState {
	for i := range st.Phases {
		if st.Phases[i].Status == "running" {
			return &st.Phases[i]
		}
	}
	return nil
}

func (st *State) phase(id Phase, fn func(*PhaseState)) {
	for i := range st.Phases {
		if st.Phases[i].ID == id {
			fn(&st.Phases[i])
			return
		}
	}
	p := PhaseState{ID: id, Status: "pending"}
	fn(&p)
	st.Phases = append(st.Phases, p)
}

func (st *State) segment(name string, fn func(*SegmentState)) {
	for i := range st.Segments {
		if st.Segments[i].Name == name {
			fn(&st.Segments[i])
			return
		}
	}
	s := SegmentState{Name: name, Index: len(st.Segments), Status: "pending"}
	fn(&s)
	st.Segments = append(st.Segments, s)
}

// finishPhases settles what is still open when the run ends. A cancelled
// run's pipeline reports the phase it was in as failed with the
// cancellation as the error: that phase was cancelled, not failed.
func (st *State) finishPhases(status string) {
	for i := range st.Phases {
		p := &st.Phases[i]
		switch p.Status {
		case "running":
			p.Status = status
		case "pending":
			p.Status = "skipped"
		case "failed":
			if status == "cancelled" && strings.Contains(strings.ToLower(p.Error), "cancel") {
				p.Status = status
			}
		}
		for j := range p.Steps {
			if p.Steps[j].Status == "running" || p.Steps[j].Status == "pending" {
				p.Steps[j].Status = status
			}
		}
	}
	for i := range st.Segments {
		switch st.Segments[i].Status {
		case "running":
			st.Segments[i].Status = status
		case "pending":
			st.Segments[i].Status = "skipped"
		}
	}
}

func shortActivity(s string) string {
	if i := strings.LastIndexByte(s, '.'); i >= 0 && i < len(s)-1 {
		return s[i+1:]
	}
	return s
}

func str(p map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := p[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func floats(v any) map[string]float64 {
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	out := map[string]float64{}
	for k, x := range m {
		switch n := x.(type) {
		case float64:
			out[k] = n
		case int64:
			out[k] = float64(n)
		case int:
			out[k] = float64(n)
		case map[string]any:
			if val, ok := n["value"].(float64); ok {
				out[k] = val
			}
		}
	}
	return out
}

func ptr(t time.Time) *time.Time { return &t }
