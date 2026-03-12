package api

import (
	"context"
	"sort"

	"github.com/hatchet-dev/hatchet/pkg/client/rest"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/stroppy-io/hatchet-workflow/internal/proto/api"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/deployment"
)

func (h *Handler) syncSuiteFromHatchet(
	ctx context.Context,
	suite *pb.Suite,
	hatchetRunID *string,
) {
	if suite == nil || hatchetRunID == nil || *hatchetRunID == "" {
		return
	}

	parentRun, err := h.hatchet.Runs().Get(ctx, *hatchetRunID)
	if err == nil {
		applyWorkflowRunToSuite(suite, &parentRun.Run)
		childRuns := childWorkflowRuns(parentRun.Tasks, *hatchetRunID)
		if len(childRuns) == 0 {
			return
		}

		sort.Slice(childRuns, func(i, j int) bool {
			return childRuns[i].CreatedAt.Before(childRuns[j].CreatedAt)
		})

		suite.Runs = make([]*pb.Run, 0, len(childRuns))
		for _, row := range childRuns {
			suite.Runs = append(suite.Runs, hatchetTaskSummaryToRun(row, suite.Target))
		}

		suite.Status = deriveSuiteStatusFromRuns(suite.Status, suite.Runs, len(suite.GetTestSuite().GetTests()))
	}
}

func applyWorkflowRunToSuite(suite *pb.Suite, run *rest.V1WorkflowRun) {
	if suite == nil || run == nil {
		return
	}

	suite.Status = hatchetStatusToRunStatus(run.Status)
	if run.StartedAt != nil {
		suite.StartedAt = timestamppb.New(*run.StartedAt)
	}
	if run.FinishedAt != nil {
		suite.FinishedAt = timestamppb.New(*run.FinishedAt)
	}
	if run.Duration != nil {
		durationMs := int64(*run.Duration)
		suite.DurationMs = &durationMs
	}
	if run.ErrorMessage != nil && *run.ErrorMessage != "" {
		suite.ErrorMessage = run.ErrorMessage
	}
}

func hatchetTaskSummaryToRun(task rest.V1TaskSummary, target deployment.Target) *pb.Run {
	run := &pb.Run{
		RunId:     task.WorkflowRunExternalId.String(),
		Status:    hatchetStatusToRunStatus(task.Status),
		Target:    target,
		CreatedAt: timestamppb.New(task.CreatedAt),
	}

	if task.StartedAt != nil {
		run.StartedAt = timestamppb.New(*task.StartedAt)
	}
	if task.FinishedAt != nil {
		run.FinishedAt = timestamppb.New(*task.FinishedAt)
	}
	if task.Duration != nil {
		durationMs := int64(*task.Duration)
		run.DurationMs = &durationMs
	}
	if task.ErrorMessage != nil && *task.ErrorMessage != "" {
		run.ErrorMessage = task.ErrorMessage
	}

	return run
}

func childWorkflowRuns(tasks []rest.V1TaskSummary, parentHatchetRunID string) []rest.V1TaskSummary {
	seen := make(map[string]struct{})
	runs := make([]rest.V1TaskSummary, 0)

	for _, task := range tasks {
		runID := task.WorkflowRunExternalId.String()
		if runID == "" || runID == parentHatchetRunID {
			continue
		}
		if _, ok := seen[runID]; ok {
			continue
		}
		seen[runID] = struct{}{}
		runs = append(runs, task)
	}

	return runs
}

func deriveSuiteStatusFromRuns(current pb.RunStatus, runs []*pb.Run, expectedRuns int) pb.RunStatus {
	if len(runs) == 0 {
		return current
	}

	allCompleted := true
	allCancelled := true
	hasRunning := false
	hasQueued := false

	for _, run := range runs {
		switch run.GetStatus() {
		case pb.RunStatus_RUN_STATUS_FAILED:
			return pb.RunStatus_RUN_STATUS_FAILED
		case pb.RunStatus_RUN_STATUS_CANCELLED:
			allCompleted = false
		case pb.RunStatus_RUN_STATUS_COMPLETED:
			allCancelled = false
		case pb.RunStatus_RUN_STATUS_RUNNING:
			hasRunning = true
			allCompleted = false
			allCancelled = false
		case pb.RunStatus_RUN_STATUS_QUEUED:
			hasQueued = true
			allCompleted = false
			allCancelled = false
		default:
			allCompleted = false
			allCancelled = false
		}
	}

	if hasRunning {
		return pb.RunStatus_RUN_STATUS_RUNNING
	}
	if hasQueued {
		return pb.RunStatus_RUN_STATUS_QUEUED
	}
	if allCompleted && (expectedRuns == 0 || len(runs) >= expectedRuns) {
		return pb.RunStatus_RUN_STATUS_COMPLETED
	}
	if allCancelled {
		return pb.RunStatus_RUN_STATUS_CANCELLED
	}

	return current
}

func hatchetStatusToRunStatus(status rest.V1TaskStatus) pb.RunStatus {
	switch status {
	case rest.V1TaskStatusRUNNING:
		return pb.RunStatus_RUN_STATUS_RUNNING
	case rest.V1TaskStatusCOMPLETED:
		return pb.RunStatus_RUN_STATUS_COMPLETED
	case rest.V1TaskStatusFAILED:
		return pb.RunStatus_RUN_STATUS_FAILED
	case rest.V1TaskStatusCANCELLED:
		return pb.RunStatus_RUN_STATUS_CANCELLED
	case rest.V1TaskStatusQUEUED:
		return pb.RunStatus_RUN_STATUS_QUEUED
	default:
		return pb.RunStatus_RUN_STATUS_NONE
	}
}
