package run

import (
	"slices"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/workflows"
)

// Every milestone the pipeline emits is a timeline kind.
func TestEventKindsCoverTheMilestones(t *testing.T) {
	for _, m := range []string{
		workflows.PhaseStarted, workflows.PhaseFinished, workflows.PhaseFailed, workflows.MachineReady, workflows.ContainerReady,
		workflows.SegmentStarted, workflows.SegmentFinished, workflows.SegmentFailed, workflows.BaselineStarted, workflows.BaselineFinished,
		workflows.StandKept, workflows.ResultPublished,
	} {
		if !slices.Contains(EventKinds, m) {
			t.Errorf("milestone %q is not a timeline kind", m)
		}
	}
	if k, ok := EventKind("run-canceled"); !ok || k != "run-cancelled" {
		t.Fatalf("Graphene's spelling: %q %v", k, ok)
	}
}
