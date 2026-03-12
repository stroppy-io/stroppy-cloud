package api

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hatchet-dev/hatchet/pkg/client/rest"

	pb "github.com/stroppy-io/hatchet-workflow/internal/proto/api"
)

func TestDeriveSuiteStatusFromRuns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		current  pb.RunStatus
		runs     []*pb.Run
		expected pb.RunStatus
	}{
		{
			name:    "failed child makes suite failed",
			current: pb.RunStatus_RUN_STATUS_RUNNING,
			runs: []*pb.Run{
				{Status: pb.RunStatus_RUN_STATUS_FAILED},
			},
			expected: pb.RunStatus_RUN_STATUS_FAILED,
		},
		{
			name:    "running child keeps suite running",
			current: pb.RunStatus_RUN_STATUS_QUEUED,
			runs: []*pb.Run{
				{Status: pb.RunStatus_RUN_STATUS_RUNNING},
			},
			expected: pb.RunStatus_RUN_STATUS_RUNNING,
		},
		{
			name:    "all completed completes suite",
			current: pb.RunStatus_RUN_STATUS_RUNNING,
			runs: []*pb.Run{
				{Status: pb.RunStatus_RUN_STATUS_COMPLETED},
				{Status: pb.RunStatus_RUN_STATUS_COMPLETED},
			},
			expected: pb.RunStatus_RUN_STATUS_COMPLETED,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := deriveSuiteStatusFromRuns(tt.current, tt.runs, len(tt.runs))
			if got != tt.expected {
				t.Fatalf("deriveSuiteStatusFromRuns() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestChildWorkflowRunsFiltersParentAndDuplicates(t *testing.T) {
	t.Parallel()

	parentRunID := uuid.NewString()
	childA := uuid.New()
	childB := uuid.New()
	now := time.Now()

	runs := childWorkflowRuns([]rest.V1TaskSummary{
		{WorkflowRunExternalId: uuid.MustParse(parentRunID), CreatedAt: now},
		{WorkflowRunExternalId: childA, CreatedAt: now.Add(time.Second)},
		{WorkflowRunExternalId: childA, CreatedAt: now.Add(2 * time.Second)},
		{WorkflowRunExternalId: childB, CreatedAt: now.Add(3 * time.Second)},
	}, parentRunID)

	if len(runs) != 2 {
		t.Fatalf("childWorkflowRuns() returned %d runs, want 2", len(runs))
	}
	if runs[0].WorkflowRunExternalId != childA {
		t.Fatalf("first child run = %s, want %s", runs[0].WorkflowRunExternalId, childA)
	}
	if runs[1].WorkflowRunExternalId != childB {
		t.Fatalf("second child run = %s, want %s", runs[1].WorkflowRunExternalId, childB)
	}
}
