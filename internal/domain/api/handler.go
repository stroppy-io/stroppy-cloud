package api

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stroppy-io/hatchet-workflow/internal/auth"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/api/apiconnect"

	hatchetLib "github.com/hatchet-dev/hatchet/sdks/go"
)

type Handler struct {
	apiconnect.UnimplementedStroppyAPIHandler

	pool    *pgxpool.Pool
	jwt     *auth.JWTService
	hatchet *hatchetLib.Client
}

func NewHandler(pool *pgxpool.Pool, jwt *auth.JWTService, hatchet *hatchetLib.Client) *Handler {
	return &Handler{
		pool:    pool,
		jwt:     jwt,
		hatchet: hatchet,
	}
}
