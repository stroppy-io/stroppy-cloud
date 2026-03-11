package api

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/oklog/ulid/v2"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	workflowTest "github.com/stroppy-io/hatchet-workflow/internal/domain/workflows/test"
	pb "github.com/stroppy-io/hatchet-workflow/internal/proto/api"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/deployment"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/workflows"
)

func (h *Handler) RunTestSuite(
	ctx context.Context,
	req *connect.Request[pb.RunTestSuiteRequest],
) (*connect.Response[pb.RunTestSuiteResponse], error) {
	msg := req.Msg
	suiteID := ulid.Make().String()

	suiteJSON, _ := protojson.Marshal(msg.GetSuite())
	if _, err := h.pool.Exec(ctx,
		`INSERT INTO suites (id, status, test_suite, target, created_at)
		 VALUES ($1, $2, $3, $4, now())`,
		suiteID, int32(pb.RunStatus_RUN_STATUS_QUEUED), suiteJSON, int32(msg.GetTarget()),
	); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	settingsJSON, _ := protojson.Marshal(msg.GetSettings())
	var settingsProto = msg.GetSettings()
	_ = protojson.Unmarshal(settingsJSON, settingsProto)

	input := &workflows.Workflows_StroppyTestSuite_Input{
		Suite:    msg.GetSuite(),
		Settings: msg.GetSettings(),
		Target:   msg.GetTarget(),
	}

	ref, err := h.hatchet.Run(ctx, workflowTest.SuiteWorkflowName, input)
	if err != nil {
		h.pool.Exec(ctx,
			`UPDATE suites SET status = $1, error_message = $2 WHERE id = $3`,
			int32(pb.RunStatus_RUN_STATUS_FAILED), err.Error(), suiteID,
		)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("dispatch workflow: %w", err))
	}

	hatchetRunID := ref.RunId
	h.pool.Exec(ctx,
		`UPDATE suites SET hatchet_run_id = $1, status = $2 WHERE id = $3`,
		hatchetRunID, int32(pb.RunStatus_RUN_STATUS_RUNNING), suiteID,
	)

	return connect.NewResponse(&pb.RunTestSuiteResponse{
		SuiteId: suiteID,
	}), nil
}

func (h *Handler) ListSuites(
	ctx context.Context,
	req *connect.Request[pb.ListSuitesRequest],
) (*connect.Response[pb.ListSuitesResponse], error) {
	msg := req.Msg
	limit := msg.GetLimit()
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := msg.GetOffset()

	rows, err := h.pool.Query(ctx,
		`SELECT id, status, test_suite, target, created_at, started_at, finished_at,
		        duration_ms, error_message, results
		 FROM suites ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer rows.Close()

	var suites []*pb.Suite
	for rows.Next() {
		s, err := scanSuite(rows)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		suites = append(suites, s)
	}

	var total int64
	h.pool.QueryRow(ctx, `SELECT count(*) FROM suites`).Scan(&total)

	return connect.NewResponse(&pb.ListSuitesResponse{
		Suites: suites,
		Total:  total,
	}), nil
}

func (h *Handler) GetSuite(
	ctx context.Context,
	req *connect.Request[pb.GetSuiteRequest],
) (*connect.Response[pb.GetSuiteResponse], error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, status, test_suite, target, created_at, started_at, finished_at,
		        duration_ms, error_message, results
		 FROM suites WHERE id = $1`,
		req.Msg.GetSuiteId(),
	)

	s, err := scanSuiteRow(row)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("suite not found"))
	}

	// Load child runs.
	runs, err := h.loadRunsForSuite(ctx, s.SuiteId)
	if err == nil {
		s.Runs = runs
	}

	return connect.NewResponse(&pb.GetSuiteResponse{Suite: s}), nil
}

func (h *Handler) loadRunsForSuite(ctx context.Context, suiteID string) ([]*pb.Run, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, suite_id, hatchet_run_id, status, test, target, created_at,
		        started_at, finished_at, duration_ms, error_message, dag, results
		 FROM runs WHERE suite_id = $1 ORDER BY created_at`,
		suiteID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []*pb.Run
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, r)
	}
	return runs, nil
}

func scanSuite(rows pgx.Rows) (*pb.Suite, error) {
	var (
		id, errMsg                 string
		status, target             int32
		testSuiteJSON, resultsJSON []byte
		createdAt                  time.Time
		startedAt, finishedAt      *time.Time
		durationMs                 *int64
		errorMessage               *string
	)

	if err := rows.Scan(
		&id, &status, &testSuiteJSON, &target, &createdAt,
		&startedAt, &finishedAt, &durationMs, &errorMessage, &resultsJSON,
	); err != nil {
		return nil, err
	}

	_ = errMsg
	return buildSuite(id, status, testSuiteJSON, target, createdAt, startedAt, finishedAt, durationMs, errorMessage, resultsJSON), nil
}

func scanSuiteRow(row pgx.Row) (*pb.Suite, error) {
	var (
		id                         string
		status, target             int32
		testSuiteJSON, resultsJSON []byte
		createdAt                  time.Time
		startedAt, finishedAt      *time.Time
		durationMs                 *int64
		errorMessage               *string
	)

	if err := row.Scan(
		&id, &status, &testSuiteJSON, &target, &createdAt,
		&startedAt, &finishedAt, &durationMs, &errorMessage, &resultsJSON,
	); err != nil {
		return nil, err
	}

	return buildSuite(id, status, testSuiteJSON, target, createdAt, startedAt, finishedAt, durationMs, errorMessage, resultsJSON), nil
}

func buildSuite(
	id string, status int32, testSuiteJSON []byte, target int32,
	createdAt time.Time, startedAt, finishedAt *time.Time,
	durationMs *int64, errorMessage *string, resultsJSON []byte,
) *pb.Suite {
	s := &pb.Suite{
		SuiteId:   id,
		Status:    pb.RunStatus(status),
		Target:    deployment.Target(target),
		CreatedAt: timestamppb.New(createdAt),
	}

	if startedAt != nil {
		s.StartedAt = timestamppb.New(*startedAt)
	}
	if finishedAt != nil {
		s.FinishedAt = timestamppb.New(*finishedAt)
	}
	if durationMs != nil {
		s.DurationMs = durationMs
	}
	if errorMessage != nil {
		s.ErrorMessage = errorMessage
	}

	return s
}
