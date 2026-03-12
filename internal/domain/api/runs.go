package api

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/hatchet-dev/hatchet/pkg/client/rest"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/stroppy-io/hatchet-workflow/internal/proto/api"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/deployment"
)

func (h *Handler) GetRun(
	ctx context.Context,
	req *connect.Request[pb.GetRunRequest],
) (*connect.Response[pb.GetRunResponse], error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, suite_id, hatchet_run_id, status, test, target, created_at,
		        started_at, finished_at, duration_ms, error_message, dag, results
		 FROM runs WHERE id = $1`,
		req.Msg.GetRunId(),
	)

	r, err := scanRunRow(row)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("run not found"))
	}

	return connect.NewResponse(&pb.GetRunResponse{Run: r}), nil
}

func (h *Handler) ListRuns(
	ctx context.Context,
	req *connect.Request[pb.ListRunsRequest],
) (*connect.Response[pb.ListRunsResponse], error) {
	msg := req.Msg
	limit := msg.GetLimit()
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := msg.GetOffset()

	rows, err := h.pool.Query(ctx,
		`SELECT id, suite_id, hatchet_run_id, status, test, target, created_at,
		        started_at, finished_at, duration_ms, error_message, dag, results
		 FROM runs ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer rows.Close()

	var runs []*pb.Run
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		runs = append(runs, r)
	}

	var total int64
	h.pool.QueryRow(ctx, `SELECT count(*) FROM runs`).Scan(&total)

	return connect.NewResponse(&pb.ListRunsResponse{
		Runs:  runs,
		Total: total,
	}), nil
}

func (h *Handler) CancelRun(
	ctx context.Context,
	req *connect.Request[pb.CancelRunRequest],
) (*connect.Response[pb.CancelRunResponse], error) {
	runID := req.Msg.GetRunId()

	hatchetRunID, err := h.resolveHatchetRunID(ctx, runID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("run not found"))
	}

	if hatchetRunID != "" {
		uid, _ := uuid.Parse(hatchetRunID)
		h.hatchet.Runs().Cancel(ctx, rest.V1CancelTaskRequest{
			ExternalIds: &[]uuid.UUID{uid},
		})
	}

	if _, err := h.pool.Exec(ctx,
		`UPDATE runs SET status = $1 WHERE id = $2`,
		int32(pb.RunStatus_RUN_STATUS_CANCELLED), runID,
	); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		// Dynamic Hatchet runs won't have a local row to update.
	}

	return connect.NewResponse(&pb.CancelRunResponse{}), nil
}

func scanRun(rows pgx.Rows) (*pb.Run, error) {
	var (
		id, suiteID                    string
		hatchetRunID                   *string
		status, target                 int32
		testJSON, dagJSON, resultsJSON []byte
		createdAt                      time.Time
		startedAt, finishedAt          *time.Time
		durationMs                     *int64
		errorMessage                   *string
	)

	if err := rows.Scan(
		&id, &suiteID, &hatchetRunID, &status, &testJSON, &target,
		&createdAt, &startedAt, &finishedAt, &durationMs, &errorMessage,
		&dagJSON, &resultsJSON,
	); err != nil {
		return nil, err
	}

	return buildRun(id, status, target, createdAt, startedAt, finishedAt, durationMs, errorMessage), nil
}

func scanRunRow(row pgx.Row) (*pb.Run, error) {
	var (
		id, suiteID                    string
		hatchetRunID                   *string
		status, target                 int32
		testJSON, dagJSON, resultsJSON []byte
		createdAt                      time.Time
		startedAt, finishedAt          *time.Time
		durationMs                     *int64
		errorMessage                   *string
	)

	if err := row.Scan(
		&id, &suiteID, &hatchetRunID, &status, &testJSON, &target,
		&createdAt, &startedAt, &finishedAt, &durationMs, &errorMessage,
		&dagJSON, &resultsJSON,
	); err != nil {
		return nil, err
	}

	_ = suiteID
	return buildRun(id, status, target, createdAt, startedAt, finishedAt, durationMs, errorMessage), nil
}

func buildRun(
	id string, status, target int32,
	createdAt time.Time, startedAt, finishedAt *time.Time,
	durationMs *int64, errorMessage *string,
) *pb.Run {
	r := &pb.Run{
		RunId:     id,
		Status:    pb.RunStatus(status),
		Target:    deployment.Target(target),
		CreatedAt: timestamppb.New(createdAt),
	}

	if startedAt != nil {
		r.StartedAt = timestamppb.New(*startedAt)
	}
	if finishedAt != nil {
		r.FinishedAt = timestamppb.New(*finishedAt)
	}
	if durationMs != nil {
		r.DurationMs = durationMs
	}
	if errorMessage != nil {
		r.ErrorMessage = errorMessage
	}

	return r
}
