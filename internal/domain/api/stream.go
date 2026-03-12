package api

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	v0Client "github.com/hatchet-dev/hatchet/pkg/client"
	"github.com/jackc/pgx/v5"

	pb "github.com/stroppy-io/hatchet-workflow/internal/proto/api"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (h *Handler) StreamRun(
	ctx context.Context,
	req *connect.Request[pb.StreamRunRequest],
	stream *connect.ServerStream[pb.StreamRunUpdate],
) error {
	runID := req.Msg.GetRunId()

	hatchetRunID, err := h.resolveHatchetRunID(ctx, runID)
	if err != nil {
		return connect.NewError(connect.CodeNotFound, err)
	}

	initial, err := h.hatchet.Runs().Get(ctx, hatchetRunID)
	if err != nil {
		return connect.NewError(connect.CodeUnknown, err)
	}

	initialStatus := hatchetStatusToRunStatus(initial.Run.Status)
	if initialStatus != pb.RunStatus_RUN_STATUS_NONE {
		if err := sendRunTaskEvent(stream, runID, "run", initialStatus, initial.Run.ErrorMessage); err != nil {
			return err
		}
		if isTerminalStatus(initialStatus) {
			return nil
		}
	}

	return h.hatchetV0.Subscribe().On(ctx, hatchetRunID, func(event v0Client.WorkflowEvent) error {
		status := resourceEventToRunStatus(int32(event.EventType))
		if status == pb.RunStatus_RUN_STATUS_NONE {
			return nil
		}

		var errorMessage *string
		if status == pb.RunStatus_RUN_STATUS_FAILED || status == pb.RunStatus_RUN_STATUS_CANCELLED {
			if details, err := h.hatchet.Runs().Get(ctx, hatchetRunID); err == nil {
				errorMessage = details.Run.ErrorMessage
			}
		}

		return sendRunTaskEvent(stream, runID, "run", status, errorMessage)
	})
}

func (h *Handler) resolveHatchetRunID(ctx context.Context, runID string) (string, error) {
	var hatchetRunID *string
	err := h.pool.QueryRow(ctx,
		`SELECT hatchet_run_id FROM runs WHERE id = $1`, runID,
	).Scan(&hatchetRunID)
	if err == nil {
		if hatchetRunID != nil && *hatchetRunID != "" {
			return *hatchetRunID, nil
		}
		return runID, nil
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	if _, parseErr := uuid.Parse(runID); parseErr == nil {
		return runID, nil
	}
	return "", pgx.ErrNoRows
}

func sendRunTaskEvent(
	stream *connect.ServerStream[pb.StreamRunUpdate],
	runID string,
	taskName string,
	status pb.RunStatus,
	errorMessage *string,
) error {
	return stream.Send(&pb.StreamRunUpdate{
		Timestamp: timestamppb.Now(),
		Update: &pb.StreamRunUpdate_TaskEvent{
			TaskEvent: &pb.TaskEvent{
				TaskId:       runID,
				TaskName:     taskName,
				EventType:    statusToEventType(status),
				ErrorMessage: errorMessage,
			},
		},
	})
}

func resourceEventToRunStatus(eventType int32) pb.RunStatus {
	switch eventType {
	case 1:
		return pb.RunStatus_RUN_STATUS_RUNNING
	case 2:
		return pb.RunStatus_RUN_STATUS_COMPLETED
	case 3, 5:
		return pb.RunStatus_RUN_STATUS_FAILED
	case 4:
		return pb.RunStatus_RUN_STATUS_CANCELLED
	default:
		return pb.RunStatus_RUN_STATUS_NONE
	}
}

func isTerminalStatus(status pb.RunStatus) bool {
	return status == pb.RunStatus_RUN_STATUS_COMPLETED ||
		status == pb.RunStatus_RUN_STATUS_FAILED ||
		status == pb.RunStatus_RUN_STATUS_CANCELLED
}

func statusToEventType(s pb.RunStatus) pb.TaskEventType {
	switch s {
	case pb.RunStatus_RUN_STATUS_RUNNING:
		return pb.TaskEventType_TASK_EVENT_TYPE_STARTED
	case pb.RunStatus_RUN_STATUS_COMPLETED:
		return pb.TaskEventType_TASK_EVENT_TYPE_RUN_COMPLETED
	case pb.RunStatus_RUN_STATUS_FAILED:
		return pb.TaskEventType_TASK_EVENT_TYPE_RUN_FAILED
	case pb.RunStatus_RUN_STATUS_CANCELLED:
		return pb.TaskEventType_TASK_EVENT_TYPE_CANCELLED
	default:
		return pb.TaskEventType_TASK_EVENT_TYPE_NONE
	}
}
