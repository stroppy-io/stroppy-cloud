package api

import (
	"context"
	"time"

	"connectrpc.com/connect"

	pb "github.com/stroppy-io/hatchet-workflow/internal/proto/api"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (h *Handler) StreamRun(
	ctx context.Context,
	req *connect.Request[pb.StreamRunRequest],
	stream *connect.ServerStream[pb.StreamRunUpdate],
) error {
	runID := req.Msg.GetRunId()

	var hatchetRunID *string
	err := h.pool.QueryRow(ctx,
		`SELECT hatchet_run_id FROM runs WHERE id = $1`, runID,
	).Scan(&hatchetRunID)
	if err != nil {
		return connect.NewError(connect.CodeNotFound, err)
	}

	// Poll Hatchet for status changes and stream them.
	// TODO: Replace with Hatchet's SubscribeToStream when available.
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var lastStatus int32

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			var status int32
			h.pool.QueryRow(ctx,
				`SELECT status FROM runs WHERE id = $1`, runID,
			).Scan(&status)

			if status != lastStatus {
				lastStatus = status
				if err := stream.Send(&pb.StreamRunUpdate{
					Timestamp: timestamppb.Now(),
					Update: &pb.StreamRunUpdate_TaskEvent{
						TaskEvent: &pb.TaskEvent{
							TaskId:    runID,
							TaskName:  "run",
							EventType: statusToEventType(pb.RunStatus(status)),
						},
					},
				}); err != nil {
					return err
				}
			}

			// Terminal statuses end the stream.
			if pb.RunStatus(status) == pb.RunStatus_RUN_STATUS_COMPLETED ||
				pb.RunStatus(status) == pb.RunStatus_RUN_STATUS_FAILED ||
				pb.RunStatus(status) == pb.RunStatus_RUN_STATUS_CANCELLED {
				return nil
			}
		}
	}
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
