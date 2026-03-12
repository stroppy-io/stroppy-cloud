package api

import (
	"context"

	"connectrpc.com/connect"
	v0Client "github.com/hatchet-dev/hatchet/pkg/client"

	pb "github.com/stroppy-io/hatchet-workflow/internal/proto/api"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (h *Handler) StreamSuite(
	ctx context.Context,
	req *connect.Request[pb.StreamSuiteRequest],
	stream *connect.ServerStream[pb.StreamSuiteUpdate],
) error {
	suite, hatchetRunID, err := h.getSuiteSnapshot(ctx, req.Msg.GetSuiteId())
	if err != nil {
		return connect.NewError(connect.CodeNotFound, err)
	}

	sendSuiteSnapshot := func(snapshot *pb.Suite) error {
		return stream.Send(&pb.StreamSuiteUpdate{
			Timestamp: timestamppb.Now(),
			Suite:     snapshot,
		})
	}

	if err := sendSuiteSnapshot(suite); err != nil {
		return err
	}
	if isTerminalStatus(suite.GetStatus()) || hatchetRunID == nil || *hatchetRunID == "" {
		return nil
	}

	return h.hatchetV0.Subscribe().On(ctx, *hatchetRunID, func(_ v0Client.WorkflowEvent) error {
		latest, _, err := h.getSuiteSnapshot(ctx, req.Msg.GetSuiteId())
		if err != nil {
			return err
		}
		return sendSuiteSnapshot(latest)
	})
}
