package api

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"

	pb "github.com/stroppy-io/hatchet-workflow/internal/proto/api"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/settings"
)

func (h *Handler) GetSettings(
	ctx context.Context,
	req *connect.Request[pb.GetSettingsRequest],
) (*connect.Response[pb.GetSettingsResponse], error) {
	var data []byte
	err := h.pool.QueryRow(ctx,
		`SELECT data FROM settings WHERE id = 'default'`,
	).Scan(&data)
	if err != nil {
		return connect.NewResponse(&pb.GetSettingsResponse{
			Settings: &settings.Settings{},
		}), nil
	}

	s := &settings.Settings{}
	if len(data) > 0 {
		if err := protojson.Unmarshal(data, s); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&pb.GetSettingsResponse{Settings: s}), nil
}

func (h *Handler) UpdateSettings(
	ctx context.Context,
	req *connect.Request[pb.UpdateSettingsRequest],
) (*connect.Response[pb.UpdateSettingsResponse], error) {
	data, err := protojson.Marshal(req.Msg.GetSettings())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if _, err := h.pool.Exec(ctx,
		`INSERT INTO settings (id, data, updated_at) VALUES ('default', $1, now())
		 ON CONFLICT (id) DO UPDATE SET data = $1, updated_at = now()`,
		data,
	); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&pb.UpdateSettingsResponse{
		Settings: req.Msg.GetSettings(),
	}), nil
}
