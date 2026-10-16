package run

import "slices"

// EventKinds is every kind a run's timeline carries: Graphene's run
// lifecycle (one spelling of cancelled), a failed step, and the pipeline's
// milestones (pipelines/workflows). The contract's RunEventKind is the same
// list; a kind outside it is not stored.
var EventKinds = []string{
	"run-started", "run-completed", "run-failed", "run-timed-out", "run-terminated", "run-cancelled",
	"activity-failed", "activity-timed-out",
	"phase.started", "phase.finished", "phase.failed",
	"machine.ready", "container.ready",
	"segment.started", "segment.finished", "segment.failed",
	"baseline.started", "baseline.finished",
	"stand.kept", "result.published",
}

// EventKind is the timeline's name of an event kind; false for a kind the
// timeline does not show. Graphene spells cancellation "run-canceled".
func EventKind(kind string) (string, bool) {
	if kind == "run-canceled" {
		kind = "run-cancelled"
	}
	return kind, slices.Contains(EventKinds, kind)
}
