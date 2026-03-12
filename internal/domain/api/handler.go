package api

import (
	v0Client "github.com/hatchet-dev/hatchet/pkg/client"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stroppy-io/hatchet-workflow/internal/auth"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/api/apiconnect"

	hatchetLib "github.com/hatchet-dev/hatchet/sdks/go"
)

type Handler struct {
	apiconnect.UnimplementedStroppyAPIHandler

	pool      *pgxpool.Pool
	jwt       *auth.JWTService
	hatchet   *hatchetLib.Client
	hatchetV0 v0Client.Client
}

func NewHandler(pool *pgxpool.Pool, jwt *auth.JWTService, hatchet *hatchetLib.Client, hatchetV0 v0Client.Client) *Handler {
	return &Handler{
		pool:      pool,
		jwt:       jwt,
		hatchet:   hatchet,
		hatchetV0: hatchetV0,
	}
}
