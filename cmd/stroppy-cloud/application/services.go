package application

import (
	"context"
	"encoding/json"
	"time"

	"connectrpc.com/connect"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/gopherex/xlog"
	"github.com/gopherex/xshutdown"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/admin"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/audit"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/compare"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/compile"
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
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/graphene"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/mail"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/pipelines"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/repositories"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/schemas"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/victoria"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

/*
COMPOSITION: repositories over the ctx-aware executor, services over
repositories. Everything below this line is a value the transport receives
ready-made.
*/

// Services is the assembled application core.
type Services struct {
	Profiles  *profile.Service
	Tenants   *tenant.Service
	Tokens    *token.Service
	Audit     *audit.Service
	Settings  *settings.Service
	Providers *provider.Service
	Webhooks  *webhook.Service
	Schemas   *schemas.Registry
	Catalog   *catalog.Catalog
	Library   *library.Service
	Runs      *run.Service
	Projector *run.Projector
	Suites    *suite.Service
	SuiteProj *suite.Projector
	Schedules *schedule.Service
	Shares    *share.Service
	Compare   *compare.Service
	Rating    *rating.Service
	Admin     *admin.Service
	Pipelines *pipelines.Pusher
	Examples  *examples.Service
	Observe   *observe.Service
	IAM       *repositories.IAMRepo
	// Dispatcher delivers webhooks; started by the worker loop.
	Dispatcher *webhook.Dispatcher
}

func buildServices(ctx context.Context, cfg *Config, infra *Infra, manager *xshutdown.Manager, log *xlog.Logger) (*Services, error) {
	db := infra.Postgres.TxDB
	tx := postgres.NewTransactor(infra.Postgres.Tx)
	auditSvc := audit.NewService(repositories.NewAuditRepo(db), middleware.GetReqID)
	tokenRepo := repositories.NewTokenRepo(db)
	tenants := tenant.NewService(repositories.NewTenantRepo(db), namespaces{infra.Graphene}, tokenRepo, tx, auditSvc, nil).
		WithMailer(inviteMailer{sender: mail.New(&cfg.Mail, log), publicURL: cfg.HTTP.PublicURL, log: log})
	tokens := token.NewService(tokenRepo, tenants, auditSvc)
	registry := schemas.New()
	cat, err := catalog.New(ctx, registry, registry)
	if err != nil {
		return nil, err
	}
	webhookRepo := repositories.NewWebhookRepo(db)
	// background: detached work under the shutdown manager's context, so a
	// stop waits for verifications in flight (bounded by its timeout).
	background := func(fn func(ctx context.Context)) { manager.Go(fn) }
	providerRepo := repositories.NewProviderRepo(db, tx)
	providers := provider.NewService(
		providerRepo, infra.Graphene, probePipelines{infra.Graphene}, registry,
		tenants, graphene.WithNamespace, auditSvc, background,
	)
	tenants.WithProviderCleanup(providers)
	settingsSvc := settings.NewService(repositories.NewSettingsRepo(db), tenants)
	lib := library.NewService(repositories.NewLibraryRepo(db), registry, cat, tenants, providerRepo, keepLimit{settingsSvc}, auditSvc)
	webhooks := webhook.NewService(webhookRepo, tenants, auditSvc)
	compiler := compile.NewService(registry, cat, lib)
	compiler.Observability = spec.Observability{OTLPEndpoint: cfg.Infra.Observability.OTLPEndpoint, OTLPHeaders: cfg.Infra.Observability.OTLPHeaders}
	runRepo := repositories.NewRunRepo(db, tx)
	runs := run.NewService(runRepo, infra.Graphene, tenants, lib, providers, settingsSvc, compiler, webhooks, auditSvc, graphene.WithNamespace)
	projector := run.NewProjector(runRepo, infra.Graphene, webhooks, graphene.WithNamespace, log)
	suiteRepo := repositories.NewSuiteRepo(db)
	suites := suite.NewService(suiteRepo, runs, runRepo, infra.Graphene, tenants, lib, webhooks, auditSvc, graphene.WithNamespace)
	suiteProj := suite.NewProjector(suiteRepo, runRepo, infra.Graphene, webhooks, graphene.WithNamespace, log)
	schedules := schedule.NewService(repositories.NewScheduleRepo(db), tenants, scheduleLauncher{runs: runs, suites: suites, lib: lib}, auditSvc, log)
	lib.WithTestUsers(testUsers{suites: suites, schedules: suiteRepo})
	cmp := compare.NewService(runRepo, tenants, cat)
	shares := share.NewService(repositories.NewShareRepo(db), runRepo, suites, cmp, tenants, registry, auditSvc, cfg.HTTP.PublicURL)
	ratings := rating.NewService(runRepo, tenants, cat, shares)
	tenantRepo := repositories.NewTenantRepo(db)
	adminSvc := admin.NewService(repositories.NewAdminRepo(db), tenantRepo, repositories.NewProfileRepo(db), runRepo, runs, repositories.NewSettingsRepo(db), namespaces{infra.Graphene}, probeComponents{infra}, auditSvc, cfg.AdminEmails, infra.Graphene, graphene.WithNamespace)
	settingsSvc.WithDefaults(adminSvc)
	var runner pipelines.Runner = pipelines.ExecRunner{Dir: cfg.Infra.Pipelines.Dir, Cfg: &cfg.Infra.Graphene, Timeout: cfg.Infra.Pipelines.PushTimeout}
	if cfg.Infra.Pipelines.Runner != nil {
		runner = cfg.Infra.Pipelines.Runner
	}
	pusher := pipelines.New(runner, repositories.NewPipelineRepo(db), tenantNamespaces{tenantRepo}, log)
	tenants.WithPipelines(pusher)
	adminSvc.WithPipelines(pusherStates{pusher})
	return &Services{
		Profiles:   profile.NewService(repositories.NewProfileRepo(db)),
		Tenants:    tenants,
		Tokens:     tokens,
		Audit:      auditSvc,
		Settings:   settingsSvc,
		Library:    lib,
		Providers:  providers,
		Webhooks:   webhooks,
		Runs:       runs,
		Projector:  projector,
		Suites:     suites,
		SuiteProj:  suiteProj,
		Schedules:  schedules,
		Shares:     shares,
		Compare:    cmp,
		Rating:     ratings,
		Admin:      adminSvc,
		Pipelines:  pusher,
		Examples:   examples.NewService(lib, suites),
		Observe:    observe.NewService(runs, nilLogs(victoria.NewLogs(&cfg.Infra.Victoria)), nilMetrics(victoria.NewMetrics(&cfg.Infra.Victoria)), cat),
		Schemas:    registry,
		Catalog:    cat,
		IAM:        repositories.NewIAMRepo(db),
		Dispatcher: webhook.NewDispatcher(webhookRepo),
	}, nil
}

// probePipelines runs the two probe pipelines through Graphene.
type probePipelines struct{ c *graphene.Client }

func (p probePipelines) Verify(ctx context.Context, runID string, params provider.VerifyParams) (provider.VerifyResult, error) {
	var out provider.VerifyResult
	err := p.c.RunOnce(ctx, runID, graphene.PipelineProviderVerify, params, &out, map[string]string{"stroppy.io/kind": "provider-verify"})
	return out, err
}

func (p probePipelines) Quotas(ctx context.Context, runID string, params provider.QuotasParams) (provider.QuotasResult, error) {
	var out provider.QuotasResult
	err := p.c.RunOnce(ctx, runID, graphene.PipelineQuotas, params, &out, map[string]string{"stroppy.io/kind": "quotas"})
	return out, err
}

// keepLimit adapts the settings service to the library's Limits port.
type keepLimit struct{ s *settings.Service }

func (k keepLimit) MaxKeep(ctx context.Context, tenantID uuid.UUID) (time.Duration, error) {
	l, err := k.s.EffectiveLimits(ctx, tenantID)
	if err != nil {
		return 0, err
	}
	return l.MaxKeep, nil
}

// probeComponents adapts the dependency probes to the admin status page.
type probeComponents struct{ infra *Infra }

func (p probeComponents) Check(ctx context.Context) map[string]admin.Component {
	out := map[string]admin.Component{}
	for name, probe := range p.infra.Probes() {
		st := probe.Check(ctx)
		c := admin.Component{Status: "ok"}
		if !st.OK() {
			c.Status, c.Detail = "down", st.String()
		}
		out[name] = c
	}
	return out
}

// tenantNamespaces lists every live tenant's namespace for the push worker.
type tenantNamespaces struct{ repo *repositories.TenantRepo }

func (t tenantNamespaces) Namespaces(ctx context.Context) ([]string, error) {
	return t.repo.Namespaces(ctx)
}

// pusherStates adapts the push worker to the admin status page.
type pusherStates struct{ p *pipelines.Pusher }

func (a pusherStates) Revision() string                            { return a.p.Revision() }
func (a pusherStates) Sync(ctx context.Context, ns string) bool    { return a.p.Sync(ctx, ns) }
func (a pusherStates) SyncAll(ctx context.Context, force bool) int { return a.p.SyncAll(ctx, force) }
func (a pusherStates) States(ctx context.Context) ([]admin.Namespace, error) {
	states, err := a.p.States(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]admin.Namespace, 0, len(states))
	for _, st := range states {
		out = append(out, admin.Namespace{Namespace: st.Namespace, Status: st.Status, Revision: st.Revision, Error: st.Error, PushedAt: st.PushedAt})
	}
	return out, nil
}

// nilLogs / nilMetrics keep a nil client a nil port (typed nil pointers
// would look connected).
func nilLogs(l *victoria.Logs) observe.Logs {
	if l == nil {
		return nil
	}
	return l
}

func nilMetrics(m *victoria.Metrics) observe.Metrics {
	if m == nil {
		return nil
	}
	return m
}

func (p probePipelines) Configure(ctx context.Context, runID string, params provider.ConfigureParams) (provider.VerifyResult, error) {
	err := p.c.StartRun(ctx, runID, graphene.PipelineProviderConfig, params, map[string]string{"stroppy.io/kind": "provider-config"})
	if err != nil && connect.CodeOf(err) != connect.CodeAlreadyExists {
		return provider.VerifyResult{}, err
	}
	raw, failure, err := p.c.RunClose(ctx, runID)
	if err != nil {
		return provider.VerifyResult{}, err
	}
	if failure != "" {
		return provider.VerifyResult{OK: false, Error: failure}, nil
	}
	var out provider.VerifyResult
	err = json.Unmarshal(raw, &out)
	return out, err
}
