// Package api implements the ogen Handler: one file per area (me.go,
// tenants.go, ...). Handlers translate wire types to domain calls and back;
// no business rules live here. Unimplemented operations fall through to
// ogen's UnimplementedHandler (501) until their area lands.
package api

import (
	"github.com/google/uuid"

	"github.com/gopherex/xlog"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/admin"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/audit"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/compare"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/examples"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/library"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/observe"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/profile"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/provider"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/rating"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/schedule"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/settings"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/share"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/suite"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/tenant"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/token"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/webhook"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/schemas"
	"github.com/stroppy-io/stroppy-cloud/internal/oas"
)

// Deps are the services the handlers call.
type Deps struct {
	Profiles  *profile.Service
	Tenants   *tenant.Service
	Tokens    *token.Service
	Audit     *audit.Service
	Settings  *settings.Service
	Providers *provider.Service
	Webhooks  *webhook.Service
	Catalog   *catalog.Catalog
	Library   *library.Service
	Runs      *run.Service
	Suites    *suite.Service
	Schedules *schedule.Service
	Shares    *share.Service
	Compare   *compare.Service
	Rating    *rating.Service
	Admin     *admin.Service
	Examples  *examples.Service
	Observe   *observe.Service
	Live      LiveProjection
	Grafana   GrafanaConfig
	Schemas   *schemas.Registry
	Public    PublicConfig
	Probes    Probes
	Log       *xlog.Logger
}

// LiveProjection tells whether a run's projection is fed from Graphene
// right now (run.Projector); nil reports every run as persisted.
type LiveProjection interface {
	Live(id uuid.UUID) bool
}

// Handler is the ogen Handler.
type Handler struct {
	oas.UnimplementedHandler
	deps Deps
}

var _ oas.Handler = (*Handler)(nil)

// New builds the handler.
func New(deps Deps) *Handler { return &Handler{deps: deps} }

func (h *Handler) live(id uuid.UUID) bool {
	return h.deps.Live != nil && h.deps.Live.Live(id)
}
