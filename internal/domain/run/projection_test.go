package run_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
)

// note is a milestone as Graphene delivers it: kind "note", the
// milestone's name as the subject, its payload as the input.
func note(id int64, name string, payload map[string]any) run.RawEvent {
	raw, _ := json.Marshal(payload)
	return run.RawEvent{ID: id, At: time.Unix(id, 0), Kind: "note", Subject: name, Input: raw}
}

func TestProjectionHappyPath(t *testing.T) {
	st := run.NewState([]run.MachineSnapshot{{Name: "db-1", Role: "db"}, {Name: "runner-1", Role: "runner"}}, []string{"main"})
	status := run.StatusPending
	apply := func(e run.RawEvent) run.Outcome {
		o := st.Apply(status, e)
		if o.Status != "" {
			status = o.Status
		}
		return o
	}
	apply(run.RawEvent{ID: 1, At: time.Unix(1, 0), Kind: "run-started"})
	if status != run.StatusRunning {
		t.Fatalf("status %s", status)
	}
	apply(note(2, "phase.started", map[string]any{"phase": "provisioning"}))
	apply(run.RawEvent{ID: 3, At: time.Unix(3, 0), Kind: "activity-started", Subject: "stroppy.provider.ensure-config", ActivityID: "a1"})
	apply(run.RawEvent{ID: 4, At: time.Unix(4, 0), Kind: "activity-completed", Subject: "stroppy.provider.ensure-config", ActivityID: "a1"})
	apply(note(5, "machine.ready", map[string]any{"machine": "db-1", "role": "db", "id": "fhm1", "private_ip": "10.0.0.5", "public_ip": "1.2.3.4"}))
	apply(note(6, "phase.finished", map[string]any{"phase": "provisioning"}))
	apply(note(7, "phase.started", map[string]any{"phase": "deploying"}))
	apply(note(8, "container.ready", map[string]any{"container": "postgres", "role": "db", "machine": "db-1", "id": "abc"}))
	apply(note(9, "phase.finished", map[string]any{"phase": "deploying"}))
	apply(note(10, "phase.started", map[string]any{"phase": "workload"}))
	apply(note(11, "segment.started", map[string]any{"segment": "main", "index": 0, "script": "tpcc/tx"}))
	if st.Progress(run.PhaseWorkload) != 40 || st.CurrentSegment() != "main" {
		t.Fatalf("progress %v segment %q", st.Progress(run.PhaseWorkload), st.CurrentSegment())
	}
	apply(note(12, "segment.finished", map[string]any{"segment": "main", "status": "completed", "metrics": map[string]any{"tps": 1234.5, "latency_p99_ms": map[string]any{"value": 12.0}}}))
	apply(note(13, "phase.finished", map[string]any{"phase": "workload"}))
	o := apply(note(14, "result.published", map[string]any{"segments": 1}))
	if !o.ResultReady {
		t.Fatal("result not flagged")
	}
	o = apply(run.RawEvent{ID: 15, At: time.Unix(15, 0), Kind: "run-completed"})
	if !o.Finished || status != run.StatusCompleted || o.Phase != run.PhaseDone {
		t.Fatalf("outcome %+v status %s", o, status)
	}
	if st.Machines["db-1"].PrivateIP != "10.0.0.5" || st.Machines["db-1"].Status != "ready" || st.Containers["postgres"].Status != "ready" {
		t.Fatalf("state %+v", st)
	}
	if st.Segments[0].Metrics["tps"] != 1234.5 || st.Segments[0].Metrics["latency_p99_ms"] != 12 {
		t.Fatalf("metrics %+v", st.Segments[0].Metrics)
	}
	if len(st.Phases[0].Steps) != 1 || st.Phases[0].Steps[0].Status != "completed" || st.Phases[0].Steps[0].Title != "ensure-config" {
		t.Fatalf("steps %+v", st.Phases[0].Steps)
	}
	for _, p := range st.Phases {
		if p.Status == "pending" || p.Status == "running" {
			t.Fatalf("phase %s left %s", p.ID, p.Status)
		}
	}
}

func TestProjectionFailureSettlesPhases(t *testing.T) {
	st := run.NewState(nil, []string{"a", "b"})
	st.Apply(run.StatusRunning, note(1, "phase.started", map[string]any{"phase": "workload"}))
	st.Apply(run.StatusRunning, note(2, "segment.started", map[string]any{"segment": "a"}))
	o := st.Apply(run.StatusRunning, run.RawEvent{ID: 3, At: time.Unix(3, 0), Kind: "run-failed", Error: "boom"})
	if o.Status != run.StatusFailed || o.Reason != "boom" {
		t.Fatalf("outcome %+v", o)
	}
	if st.Segments[0].Status != "failed" || st.Segments[1].Status != "skipped" {
		t.Fatalf("segments %+v", st.Segments)
	}
}
