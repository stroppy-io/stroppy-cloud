package api

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/oklog/ulid/v2"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/stroppy-io/hatchet-workflow/internal/proto/api"
	"time"
)

func (h *Handler) RegisterWorkload(
	ctx context.Context,
	req *connect.Request[pb.RegisterWorkloadRequest],
) (*connect.Response[pb.RegisterWorkloadResponse], error) {
	msg := req.Msg
	id := ulid.Make().String()

	probeResult, err := runProbe(msg.GetScript(), msg.GetSql())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("probe script: %w", err))
	}

	probeJSON, _ := protojson.Marshal(probeResult)

	if _, err := h.pool.Exec(ctx,
		`INSERT INTO workloads (id, name, description, script, sql_data, probe)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		id, msg.GetName(), strPtr(msg.Description), msg.GetScript(), msg.Sql, probeJSON,
	); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&pb.RegisterWorkloadResponse{
		Workload: &pb.Workload{
			Id:          id,
			Name:        msg.GetName(),
			Description: msg.Description,
			Script:      msg.GetScript(),
			Sql:         msg.Sql,
			Probe:       probeResult,
			CreatedAt:   timestamppb.Now(),
		},
	}), nil
}

func (h *Handler) ListWorkloads(
	ctx context.Context,
	req *connect.Request[pb.ListWorkloadsRequest],
) (*connect.Response[pb.ListWorkloadsResponse], error) {
	limit := req.Msg.GetLimit()
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	rows, err := h.pool.Query(ctx,
		`SELECT id, name, description, builtin, script, sql_data, probe, created_at
		 FROM workloads ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		limit, req.Msg.GetOffset(),
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer rows.Close()

	var workloads []*pb.Workload
	for rows.Next() {
		w, err := scanWorkload(rows)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		workloads = append(workloads, w)
	}

	var total int64
	h.pool.QueryRow(ctx, `SELECT count(*) FROM workloads`).Scan(&total)

	return connect.NewResponse(&pb.ListWorkloadsResponse{
		Workloads: workloads,
		Total:     total,
	}), nil
}

func (h *Handler) GetWorkload(
	ctx context.Context,
	req *connect.Request[pb.GetWorkloadRequest],
) (*connect.Response[pb.GetWorkloadResponse], error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, name, description, builtin, script, sql_data, probe, created_at
		 FROM workloads WHERE id = $1`,
		req.Msg.GetWorkloadId(),
	)

	var (
		id, name           string
		description        *string
		builtin            bool
		script             []byte
		sqlData, probeJSON []byte
		createdAt          time.Time
	)
	if err := row.Scan(&id, &name, &description, &builtin, &script, &sqlData, &probeJSON, &createdAt); err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("workload not found"))
	}

	w := &pb.Workload{
		Id:        id,
		Name:      name,
		Builtin:   builtin,
		Script:    script,
		Sql:       sqlData,
		CreatedAt: timestamppb.New(createdAt),
	}
	if description != nil {
		w.Description = description
	}
	if len(probeJSON) > 0 {
		probe := &pb.ProbeResult{}
		if err := protojson.Unmarshal(probeJSON, probe); err == nil {
			w.Probe = probe
		}
	}

	return connect.NewResponse(&pb.GetWorkloadResponse{Workload: w}), nil
}

func (h *Handler) DeleteWorkload(
	ctx context.Context,
	req *connect.Request[pb.DeleteWorkloadRequest],
) (*connect.Response[pb.DeleteWorkloadResponse], error) {
	tag, err := h.pool.Exec(ctx,
		`DELETE FROM workloads WHERE id = $1`, req.Msg.GetWorkloadId(),
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if tag.RowsAffected() == 0 {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("workload not found"))
	}

	return connect.NewResponse(&pb.DeleteWorkloadResponse{}), nil
}

func scanWorkload(rows interface{ Scan(dest ...any) error }) (*pb.Workload, error) {
	var (
		id, name           string
		description        *string
		builtin            bool
		script             []byte
		sqlData, probeJSON []byte
		createdAt          time.Time
	)
	if err := rows.Scan(&id, &name, &description, &builtin, &script, &sqlData, &probeJSON, &createdAt); err != nil {
		return nil, err
	}

	w := &pb.Workload{
		Id:        id,
		Name:      name,
		Builtin:   builtin,
		Script:    script,
		Sql:       sqlData,
		CreatedAt: timestamppb.New(createdAt),
	}
	if description != nil {
		w.Description = description
	}
	if len(probeJSON) > 0 {
		probe := &pb.ProbeResult{}
		if err := protojson.Unmarshal(probeJSON, probe); err == nil {
			w.Probe = probe
		}
	}

	return w, nil
}

func (h *Handler) ProbeScript(
	ctx context.Context,
	req *connect.Request[pb.ProbeScriptRequest],
) (*connect.Response[pb.ProbeScriptResponse], error) {
	probeResult, err := runProbe(req.Msg.GetScript(), req.Msg.GetSql())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("probe script: %w", err))
	}

	return connect.NewResponse(&pb.ProbeScriptResponse{
		Probe: probeResult,
	}), nil
}

func strPtr(s *string) *string {
	return s
}
