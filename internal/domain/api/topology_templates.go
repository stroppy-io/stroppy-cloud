package api

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/oklog/ulid/v2"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/stroppy-io/hatchet-workflow/internal/proto/api"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/database"
)

func (h *Handler) CreateTopologyTemplate(
	ctx context.Context,
	req *connect.Request[pb.CreateTopologyTemplateRequest],
) (*connect.Response[pb.CreateTopologyTemplateResponse], error) {
	msg := req.Msg
	id := ulid.Make().String()

	dbType := detectDatabaseType(msg.GetTemplate())
	templateJSON, err := protojson.Marshal(msg.GetTemplate())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("marshal template: %w", err))
	}

	now := time.Now()
	if _, err := h.pool.Exec(ctx,
		`INSERT INTO topology_templates (id, name, description, database_type, template_data, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		id, msg.GetName(), strPtr(msg.Description), int32(dbType), templateJSON, now, now,
	); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&pb.CreateTopologyTemplateResponse{
		TopologyTemplate: &pb.TopologyTemplate{
			Id:           id,
			Name:         msg.GetName(),
			Description:  msg.Description,
			DatabaseType: dbType,
			Template:     msg.GetTemplate(),
			CreatedAt:    timestamppb.New(now),
			UpdatedAt:    timestamppb.New(now),
		},
	}), nil
}

func (h *Handler) ListTopologyTemplates(
	ctx context.Context,
	req *connect.Request[pb.ListTopologyTemplatesRequest],
) (*connect.Response[pb.ListTopologyTemplatesResponse], error) {
	limit := req.Msg.GetLimit()
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	var args []any
	query := `SELECT id, name, description, database_type, builtin, template_data, created_at, updated_at
		 FROM topology_templates`
	countQuery := `SELECT count(*) FROM topology_templates`

	if req.Msg.DatabaseType != nil && *req.Msg.DatabaseType != pb.DatabaseType_DATABASE_TYPE_UNSPECIFIED {
		query += ` WHERE database_type = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
		countQuery += ` WHERE database_type = $1`
		args = []any{int32(*req.Msg.DatabaseType), limit, req.Msg.GetOffset()}
	} else {
		query += ` ORDER BY created_at DESC LIMIT $1 OFFSET $2`
		args = []any{limit, req.Msg.GetOffset()}
	}

	rows, err := h.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer rows.Close()

	var templates []*pb.TopologyTemplate
	for rows.Next() {
		t, err := scanTopologyTemplate(rows)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		templates = append(templates, t)
	}

	var total int64
	if req.Msg.DatabaseType != nil && *req.Msg.DatabaseType != pb.DatabaseType_DATABASE_TYPE_UNSPECIFIED {
		h.pool.QueryRow(ctx, countQuery, int32(*req.Msg.DatabaseType)).Scan(&total)
	} else {
		h.pool.QueryRow(ctx, countQuery).Scan(&total)
	}

	return connect.NewResponse(&pb.ListTopologyTemplatesResponse{
		TopologyTemplates: templates,
		Total:             total,
	}), nil
}

func (h *Handler) GetTopologyTemplate(
	ctx context.Context,
	req *connect.Request[pb.GetTopologyTemplateRequest],
) (*connect.Response[pb.GetTopologyTemplateResponse], error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, name, description, database_type, builtin, template_data, created_at, updated_at
		 FROM topology_templates WHERE id = $1`,
		req.Msg.GetTemplateId(),
	)

	t, err := scanTopologyTemplate(row)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("topology template not found"))
	}

	return connect.NewResponse(&pb.GetTopologyTemplateResponse{TopologyTemplate: t}), nil
}

func (h *Handler) UpdateTopologyTemplate(
	ctx context.Context,
	req *connect.Request[pb.UpdateTopologyTemplateRequest],
) (*connect.Response[pb.UpdateTopologyTemplateResponse], error) {
	msg := req.Msg

	// Read current.
	row := h.pool.QueryRow(ctx,
		`SELECT id, name, description, database_type, builtin, template_data, created_at, updated_at
		 FROM topology_templates WHERE id = $1`,
		msg.GetTemplateId(),
	)
	current, err := scanTopologyTemplate(row)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("topology template not found"))
	}
	if current.Builtin {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("cannot update builtin template"))
	}

	// Apply updates.
	if msg.Name != nil {
		current.Name = *msg.Name
	}
	if msg.Description != nil {
		current.Description = msg.Description
	}
	if msg.Template != nil {
		current.Template = msg.Template
		current.DatabaseType = detectDatabaseType(msg.Template)
	}

	now := time.Now()
	current.UpdatedAt = timestamppb.New(now)

	templateJSON, err := protojson.Marshal(current.Template)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("marshal template: %w", err))
	}

	if _, err := h.pool.Exec(ctx,
		`UPDATE topology_templates
		 SET name = $1, description = $2, database_type = $3, template_data = $4, updated_at = $5
		 WHERE id = $6`,
		current.Name, current.Description, int32(current.DatabaseType), templateJSON, now, msg.GetTemplateId(),
	); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&pb.UpdateTopologyTemplateResponse{TopologyTemplate: current}), nil
}

func (h *Handler) DeleteTopologyTemplate(
	ctx context.Context,
	req *connect.Request[pb.DeleteTopologyTemplateRequest],
) (*connect.Response[pb.DeleteTopologyTemplateResponse], error) {
	tag, err := h.pool.Exec(ctx,
		`DELETE FROM topology_templates WHERE id = $1 AND builtin = false`,
		req.Msg.GetTemplateId(),
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if tag.RowsAffected() == 0 {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("topology template not found or is builtin"))
	}

	return connect.NewResponse(&pb.DeleteTopologyTemplateResponse{}), nil
}

func scanTopologyTemplate(row interface{ Scan(dest ...any) error }) (*pb.TopologyTemplate, error) {
	var (
		id, name             string
		description          *string
		dbType               int32
		builtin              bool
		templateJSON         []byte
		createdAt, updatedAt time.Time
	)
	if err := row.Scan(&id, &name, &description, &dbType, &builtin, &templateJSON, &createdAt, &updatedAt); err != nil {
		return nil, err
	}

	t := &pb.TopologyTemplate{
		Id:           id,
		Name:         name,
		DatabaseType: pb.DatabaseType(dbType),
		Builtin:      builtin,
		CreatedAt:    timestamppb.New(createdAt),
		UpdatedAt:    timestamppb.New(updatedAt),
	}
	if description != nil {
		t.Description = description
	}
	if len(templateJSON) > 0 {
		tmpl := &database.Database_Template{}
		if err := protojson.Unmarshal(templateJSON, tmpl); err == nil {
			t.Template = tmpl
		}
	}

	return t, nil
}

func detectDatabaseType(tmpl *database.Database_Template) pb.DatabaseType {
	if tmpl == nil {
		return pb.DatabaseType_DATABASE_TYPE_UNSPECIFIED
	}
	switch tmpl.Template.(type) {
	case *database.Database_Template_PostgresInstance, *database.Database_Template_PostgresCluster:
		return pb.DatabaseType_DATABASE_TYPE_POSTGRES
	case *database.Database_Template_PicodataInstance, *database.Database_Template_PicodataCluster:
		return pb.DatabaseType_DATABASE_TYPE_PICODATA
	default:
		return pb.DatabaseType_DATABASE_TYPE_UNSPECIFIED
	}
}
