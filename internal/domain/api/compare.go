package api

import (
	"context"

	"connectrpc.com/connect"

	pb "github.com/stroppy-io/hatchet-workflow/internal/proto/api"
)

func (h *Handler) CompareRuns(
	ctx context.Context,
	req *connect.Request[pb.CompareRunsRequest],
) (*connect.Response[pb.CompareRunsResponse], error) {
	// TODO: implement PromQL-based metric comparison via Grafana/Prometheus.
	return connect.NewResponse(&pb.CompareRunsResponse{
		Baseline:    &pb.RunSummary{RunId: req.Msg.GetBaselineRunId()},
		Candidate:   &pb.RunSummary{RunId: req.Msg.GetCandidateRunId()},
		Comparisons: nil,
	}), nil
}
