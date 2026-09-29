// Package admin is the platform operator's view (§16.9): tenants across
// the platform, users, the global run queue, the whole audit log, the
// system settings and the status page. A platform admin is a profile
// flagged in the database or an e-mail listed in the server config.
package admin

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/audit"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/profile"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/settings"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/tenant"
)

// TenantView is a tenant with its operator details.
type TenantView struct {
	Tenant          tenant.Tenant
	Owner           profile.Profile
	SuspendedReason string
	Limits          settings.Limits
	Counters        Counters
	LastActivityAt  *time.Time
}

// Counters of a tenant.
type Counters struct {
	Members, RunsTotal, RunsRunning, KeptStands, Providers int
}

// UserView is a profile with its platform facts.
type UserView struct {
	Profile     profile.Profile
	AdminSource string // config | db | ""
	OwnedTenant *run.Ref
	Memberships int
	// LastActivityAt is the newest audit entry the user acted in.
	LastActivityAt *time.Time
}

// RunView is a run with its tenant.
type RunView struct {
	Run        run.Run
	TenantSlug string
	TenantName string
}

// SystemSettings is the platform's single row.
type SystemSettings struct {
	TenantCreation      string
	PublicRatingEnabled bool
	ExamplesEnabled     bool
	DefaultLimits       *settings.Limits
	RunRetentionMaxDays int
	StroppyCatalog      json.RawMessage
	UpdatedAt           time.Time
	UpdatedBy           *uuid.UUID
}

// SystemPatch changes system settings.
type SystemPatch struct {
	TenantCreation      *string
	PublicRatingEnabled *bool
	ExamplesEnabled     *bool
	DefaultLimits       *settings.Limits
	SetDefaultLimits    bool
	RunRetentionMaxDays *int
	StroppyCatalog      json.RawMessage
	SetCatalog          bool
}

// TenantQuery filters tenants.
type TenantQuery struct {
	Search string
	Status string
	Sort   string
	Desc   bool
	Limit  int
	Offset int
}

// UserQuery filters users.
type UserQuery struct {
	Search     string
	OnlyAdmins bool
	Sort       string
	Desc       bool
	Limit      int
	Offset     int
}

// RunQuery filters the global queue.
type RunQuery struct {
	Statuses []string
	Tenant   string
	Sort     string
	Desc     bool
	Limit    int
	Offset   int
}

// AuditQuery filters the whole log.
type AuditQuery struct {
	Tenant   string
	Action   string
	ActorID  string
	Since    *time.Time
	BeforeID int64
	Limit    int
}

// Status is the platform status page.
type Status struct {
	Components map[string]Component
	Runs       RunCounts
	Tenants    TenantCounts
	Namespaces []Namespace
}

// Component is one dependency.
type Component struct {
	Status  string // ok | degraded | down
	Detail  string
	Version string
}

// RunCounts across tenants.
type RunCounts struct{ Running, Pending, KeptStands int }

// TenantCounts by status.
type TenantCounts struct{ Active, Suspended, Orphaned int }

// Namespace is a tenant's pipeline push state.
type Namespace struct {
	Namespace  string
	TenantSlug string
	Status     string // synced | behind | failed | pending
	Revision   string
	Error      string
	PushedAt   *time.Time
}

// Repository is the storage port.
type Repository interface {
	Tenants(ctx context.Context, q TenantQuery) ([]tenant.Tenant, error)
	TenantCounts(ctx context.Context) (TenantCounts, error)
	SuspendedReason(ctx context.Context, tenantID uuid.UUID) (string, error)
	SetSuspended(ctx context.Context, tenantID uuid.UUID, status tenant.Status, reason string) error
	Counters(ctx context.Context, tenantID uuid.UUID) (Counters, *time.Time, error)
	Users(ctx context.Context, q UserQuery) ([]UserView, error)
	SetPlatformAdmin(ctx context.Context, userID uuid.UUID, on bool) error
	Runs(ctx context.Context, q RunQuery) ([]RunView, error)
	RunCounts(ctx context.Context) (RunCounts, error)
	Audit(ctx context.Context, q AuditQuery) ([]audit.Entry, error)
	System(ctx context.Context) (SystemSettings, error)
	UpdateSystem(ctx context.Context, p SystemPatch, by *uuid.UUID) (SystemSettings, error)
}

// Tenants is what the admin needs from the tenant domain.
type Tenants interface {
	ByID(ctx context.Context, id uuid.UUID) (tenant.Tenant, error)
	OwnedBy(ctx context.Context, userID uuid.UUID) (uuid.UUID, string, bool, error)
	BySlug(ctx context.Context, slug string) (tenant.Tenant, error)
	SetOwner(ctx context.Context, id, ownerID uuid.UUID) error
	UpsertMember(ctx context.Context, tenantID, userID uuid.UUID, role tenant.Role) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

// Namespaces removes a tenant's Graphene namespace on delete.
type Namespaces interface {
	DeleteNamespace(ctx context.Context, name string) error
}

// Probes report the dependencies' health.
type Probes interface {
	Check(ctx context.Context) map[string]Component
}

// Pipelines is the pipeline push worker (§7).
type Pipelines interface {
	Revision() string
	States(ctx context.Context) ([]Namespace, error)
	Sync(ctx context.Context, namespace string) bool
	SyncAll(ctx context.Context, force bool) int
}

// Service is the admin use cases.
type Service struct {
	repo       Repository
	tenants    Tenants
	profiles   profile.Repository
	runs       run.Repository
	runSvc     *run.Service
	limits     settings.Repository
	namespaces Namespaces
	probes     Probes
	audit      *audit.Service
	// adminEmails are the config-listed platform admins (lowercase).
	adminEmails map[string]bool
	graphene    run.Graphene
	scope       func(ctx context.Context, namespace string) context.Context
	pipelines   Pipelines
}

// WithPipelines registers the push worker (status + resync).
func (s *Service) WithPipelines(p Pipelines) *Service {
	s.pipelines = p
	return s
}

// Revision is the expected pipeline revision ("" when no worker).
func (s *Service) Revision() string {
	if s.pipelines == nil {
		return ""
	}
	return s.pipelines.Revision()
}

// NewService wires the use cases.
func NewService(repo Repository, tenants Tenants, profiles profile.Repository, runs run.Repository, runSvc *run.Service, limits settings.Repository, ns Namespaces, probes Probes, auditSvc *audit.Service, adminEmails []string, g run.Graphene, scope func(context.Context, string) context.Context) *Service {
	emails := map[string]bool{}
	for _, e := range adminEmails {
		if e = strings.ToLower(strings.TrimSpace(e)); e != "" {
			emails[e] = true
		}
	}
	return &Service{repo: repo, tenants: tenants, profiles: profiles, runs: runs, runSvc: runSvc, limits: limits, namespaces: ns, probes: probes, audit: auditSvc, adminEmails: emails, graphene: g, scope: scope}
}

// IsAdmin reports whether the actor is a platform admin: a person flagged
// in the database or listed in the config. A personal token acts as its
// user; a service token never is an admin.
func (s *Service) IsAdmin(ctx context.Context, actor auth.Actor) bool {
	if actor.UserID == uuid.Nil {
		return false
	}
	if s.adminEmails[strings.ToLower(actor.Email)] {
		return true
	}
	p, err := s.profiles.Get(ctx, actor.UserID)
	if err != nil {
		return false
	}
	return p.IsPlatformAdmin || s.adminEmails[strings.ToLower(p.Email)]
}

// Require fails unless the actor is a platform admin.
func (s *Service) Require(ctx context.Context, actor auth.Actor) error {
	if !s.IsAdmin(ctx, actor) {
		return errs.Forbidden("platform admin required")
	}
	return nil
}

func (s *Service) record(ctx context.Context, action string, tenantID *uuid.UUID, target audit.Target, details map[string]any) {
	_ = s.audit.Record(ctx, audit.Entry{TenantID: tenantID, ActorKind: audit.ActorAdmin, Action: action, Target: target, Details: details}) //nolint:errcheck // audit never blocks
}

// --- tenants ----------------------------------------------------------------

// Tenants lists tenants.
func (s *Service) Tenants(ctx context.Context, actor auth.Actor, q TenantQuery) ([]TenantView, error) {
	if err := s.Require(ctx, actor); err != nil {
		return nil, err
	}
	list, err := s.repo.Tenants(ctx, q)
	if err != nil {
		return nil, err
	}
	out := make([]TenantView, 0, len(list))
	for _, t := range list {
		v, err := s.view(ctx, t, false)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// Tenant reads one tenant with details.
func (s *Service) Tenant(ctx context.Context, actor auth.Actor, slug string) (TenantView, error) {
	if err := s.Require(ctx, actor); err != nil {
		return TenantView{}, err
	}
	t, err := s.tenants.BySlug(ctx, slug)
	if err != nil {
		return TenantView{}, err
	}
	return s.view(ctx, t, true)
}

func (s *Service) view(ctx context.Context, t tenant.Tenant, full bool) (TenantView, error) {
	v := TenantView{Tenant: t}
	if owner, err := s.profiles.Get(ctx, t.OwnerID); err == nil {
		v.Owner = owner
	}
	if t.Status == tenant.StatusSuspended {
		v.SuspendedReason, _ = s.repo.SuspendedReason(ctx, t.ID) //nolint:errcheck // detail only
	}
	counters, last, err := s.repo.Counters(ctx, t.ID)
	if err != nil {
		return TenantView{}, err
	}
	v.Counters, v.LastActivityAt = counters, last
	if full {
		_, override, err := s.limits.Ensure(ctx, t.ID)
		if err != nil {
			return TenantView{}, err
		}
		if override != nil {
			v.Limits = *override
			v.Limits.Overridden = true
		} else {
			v.Limits = s.defaultLimits(ctx)
		}
	}
	return v, nil
}

// defaultLimits are the system default limits (platform ceiling).
func (s *Service) defaultLimits(ctx context.Context) settings.Limits {
	sys, err := s.repo.System(ctx)
	if err == nil && sys.DefaultLimits != nil {
		return *sys.DefaultLimits
	}
	return settings.PlatformLimits
}

// DefaultLimits implements settings.Defaults.
func (s *Service) DefaultLimits(ctx context.Context) settings.Limits { return s.defaultLimits(ctx) }

// Suspend blocks a tenant.
func (s *Service) Suspend(ctx context.Context, actor auth.Actor, slug, reason string) (TenantView, error) {
	if err := s.Require(ctx, actor); err != nil {
		return TenantView{}, err
	}
	t, err := s.tenants.BySlug(ctx, slug)
	if err != nil {
		return TenantView{}, err
	}
	if err := s.repo.SetSuspended(ctx, t.ID, tenant.StatusSuspended, reason); err != nil {
		return TenantView{}, err
	}
	s.record(ctx, "admin.tenant.suspend", &t.ID, audit.Target{Kind: "tenant", ID: t.ID.String(), Name: t.Slug}, map[string]any{"reason": reason})
	return s.Tenant(ctx, actor, slug)
}

// Resume lifts a suspension.
func (s *Service) Resume(ctx context.Context, actor auth.Actor, slug string) (TenantView, error) {
	if err := s.Require(ctx, actor); err != nil {
		return TenantView{}, err
	}
	t, err := s.tenants.BySlug(ctx, slug)
	if err != nil {
		return TenantView{}, err
	}
	if err := s.repo.SetSuspended(ctx, t.ID, tenant.StatusActive, ""); err != nil {
		return TenantView{}, err
	}
	s.record(ctx, "admin.tenant.resume", &t.ID, audit.Target{Kind: "tenant", ID: t.ID.String(), Name: t.Slug}, nil)
	return s.Tenant(ctx, actor, slug)
}

// SetLimits overrides a tenant's limits (nil = back to defaults).
func (s *Service) SetLimits(ctx context.Context, actor auth.Actor, slug string, l *settings.Limits) (settings.Limits, error) {
	if err := s.Require(ctx, actor); err != nil {
		return settings.Limits{}, err
	}
	t, err := s.tenants.BySlug(ctx, slug)
	if err != nil {
		return settings.Limits{}, err
	}
	if l != nil {
		def := s.defaultLimits(ctx)
		if l.MaxConcurrentRuns <= 0 {
			l.MaxConcurrentRuns = def.MaxConcurrentRuns
		}
		if l.MaxMachinesPerRun <= 0 {
			l.MaxMachinesPerRun = def.MaxMachinesPerRun
		}
		if l.MaxSize == "" {
			l.MaxSize = def.MaxSize
		}
		if l.MaxKeep <= 0 {
			l.MaxKeep = def.MaxKeep
		}
		if l.RunRetentionMaxDays <= 0 {
			l.RunRetentionMaxDays = def.RunRetentionMaxDays
		}
	}
	if err := s.limits.SetLimitsOverride(ctx, t.ID, l); err != nil {
		return settings.Limits{}, err
	}
	s.record(ctx, "admin.tenant.limits", &t.ID, audit.Target{Kind: "tenant", ID: t.ID.String(), Name: t.Slug}, map[string]any{"limits": l})
	v, err := s.view(ctx, t, true)
	if err != nil {
		return settings.Limits{}, err
	}
	return v.Limits, nil
}

// AssignOwner gives an orphaned (or any) tenant a new owner.
func (s *Service) AssignOwner(ctx context.Context, actor auth.Actor, slug string, userID uuid.UUID) (TenantView, error) {
	if err := s.Require(ctx, actor); err != nil {
		return TenantView{}, err
	}
	t, err := s.tenants.BySlug(ctx, slug)
	if err != nil {
		return TenantView{}, err
	}
	if _, err := s.profiles.Get(ctx, userID); err != nil {
		return TenantView{}, err
	}
	if _, _, owned, err := s.ownedBy(ctx, userID); err != nil {
		return TenantView{}, err
	} else if owned {
		return TenantView{}, errs.Conflict("user already owns a tenant")
	}
	if err := s.tenants.UpsertMember(ctx, t.ID, userID, tenant.RoleOwner); err != nil {
		return TenantView{}, err
	}
	if t.OwnerID != userID && t.OwnerID != uuid.Nil {
		_ = s.tenants.UpsertMember(ctx, t.ID, t.OwnerID, tenant.RoleAdmin) //nolint:errcheck // previous owner demoted best-effort
	}
	if err := s.tenants.SetOwner(ctx, t.ID, userID); err != nil {
		return TenantView{}, err
	}
	if t.Status == tenant.StatusOrphaned {
		_ = s.repo.SetSuspended(ctx, t.ID, tenant.StatusActive, "") //nolint:errcheck // status refresh best-effort
	}
	s.record(ctx, "admin.tenant.assign_owner", &t.ID, audit.Target{Kind: "tenant", ID: t.ID.String(), Name: t.Slug}, map[string]any{"owner": userID})
	return s.Tenant(ctx, actor, slug)
}

func (s *Service) ownedBy(ctx context.Context, userID uuid.UUID) (id uuid.UUID, slug string, owned bool, err error) {
	return s.tenants.OwnedBy(ctx, userID)
}

// DeleteTenant removes a tenant with no live runs.
func (s *Service) DeleteTenant(ctx context.Context, actor auth.Actor, slug string) error {
	if err := s.Require(ctx, actor); err != nil {
		return err
	}
	t, err := s.tenants.BySlug(ctx, slug)
	if err != nil {
		return err
	}
	live, err := s.runs.HasLive(ctx, t.ID)
	if err != nil {
		return err
	}
	if live {
		return errs.Conflict("tenant has live runs")
	}
	if err := s.namespaces.DeleteNamespace(ctx, t.GrapheneNamespace); err != nil {
		return errs.Wrap(errs.CodeUnavailable, "graphene namespace", err)
	}
	if err := s.tenants.SoftDelete(ctx, t.ID); err != nil {
		return err
	}
	s.record(ctx, "admin.tenant.delete", &t.ID, audit.Target{Kind: "tenant", ID: t.ID.String(), Name: t.Slug}, nil)
	return nil
}

// --- users ------------------------------------------------------------------

// Users lists profiles.
func (s *Service) Users(ctx context.Context, actor auth.Actor, q UserQuery) ([]UserView, error) {
	if err := s.Require(ctx, actor); err != nil {
		return nil, err
	}
	if q.OnlyAdmins && len(s.adminEmails) > 0 {
		// Config-listed admins carry no database flag: scan and filter.
		all, err := s.repo.Users(ctx, UserQuery{Search: q.Search, Limit: 200})
		if err != nil {
			return nil, err
		}
		out := make([]UserView, 0, len(all))
		for _, u := range all {
			if src := s.adminSource(u.Profile); src != "" {
				u.AdminSource = src
				out = append(out, u)
			}
		}
		if q.Offset < len(out) {
			out = out[q.Offset:]
		} else {
			out = nil
		}
		if q.Limit > 0 && len(out) > q.Limit {
			out = out[:q.Limit]
		}
		return out, nil
	}
	list, err := s.repo.Users(ctx, q)
	if err != nil {
		return nil, err
	}
	for i := range list {
		list[i].AdminSource = s.adminSource(list[i].Profile)
	}
	return list, nil
}

func (s *Service) adminSource(p profile.Profile) string {
	switch {
	case s.adminEmails[strings.ToLower(p.Email)]:
		return "config"
	case p.IsPlatformAdmin:
		return "db"
	}
	return ""
}

// SetPlatformAdmin toggles the database flag.
func (s *Service) SetPlatformAdmin(ctx context.Context, actor auth.Actor, userID uuid.UUID, on bool) (UserView, error) {
	if err := s.Require(ctx, actor); err != nil {
		return UserView{}, err
	}
	if err := s.repo.SetPlatformAdmin(ctx, userID, on); err != nil {
		return UserView{}, err
	}
	s.record(ctx, "admin.user.platform_admin", nil, audit.Target{Kind: "user", ID: userID.String()}, map[string]any{"is_platform_admin": on})
	list, err := s.repo.Users(ctx, UserQuery{Search: userID.String(), Limit: 1})
	if err != nil {
		return UserView{}, err
	}
	for _, u := range list {
		if u.Profile.ID == userID {
			u.AdminSource = s.adminSource(u.Profile)
			return u, nil
		}
	}
	p, err := s.profiles.Get(ctx, userID)
	if err != nil {
		return UserView{}, err
	}
	return UserView{Profile: p, AdminSource: s.adminSource(p)}, nil
}

// --- runs -------------------------------------------------------------------

// Runs is the global queue.
func (s *Service) Runs(ctx context.Context, actor auth.Actor, q RunQuery) ([]RunView, error) {
	if err := s.Require(ctx, actor); err != nil {
		return nil, err
	}
	return s.repo.Runs(ctx, q)
}

// CancelRun cancels any run.
func (s *Service) CancelRun(ctx context.Context, actor auth.Actor, id uuid.UUID) (run.Run, error) {
	if err := s.Require(ctx, actor); err != nil {
		return run.Run{}, err
	}
	r, err := s.runs.ByID(ctx, id)
	if err != nil {
		return run.Run{}, err
	}
	if r.Status.Terminal() || r.DeletedAt != nil {
		return run.Run{}, errs.Conflict("run already finished")
	}
	if err := s.runs.SetStatus(ctx, id, run.StatusCancelling, r.Phase, "cancelled by platform admin", nil, nil); err != nil {
		return run.Run{}, err
	}
	if err := s.graphene.CancelRun(s.scope(ctx, r.GrapheneNamespace), r.GrapheneID()); err != nil {
		return run.Run{}, errs.Wrap(errs.CodeUnavailable, "cancel", err)
	}
	s.record(ctx, "admin.run.cancel", &r.TenantID, audit.Target{Kind: "run", ID: id.String(), Name: r.Name}, nil)
	return s.runs.ByID(ctx, id)
}

// --- audit / settings / status ----------------------------------------------

// Audit is the whole log.
func (s *Service) Audit(ctx context.Context, actor auth.Actor, q AuditQuery) ([]audit.Entry, error) {
	if err := s.Require(ctx, actor); err != nil {
		return nil, err
	}
	return s.repo.Audit(ctx, q)
}

// System reads the system settings (no admin check: public config reads it).
func (s *Service) System(ctx context.Context) (SystemSettings, error) {
	return s.repo.System(ctx)
}

// UpdateSystem changes the system settings.
func (s *Service) UpdateSystem(ctx context.Context, actor auth.Actor, p SystemPatch) (SystemSettings, error) {
	if err := s.Require(ctx, actor); err != nil {
		return SystemSettings{}, err
	}
	if p.TenantCreation != nil && *p.TenantCreation != "anyone" && *p.TenantCreation != "admin_only" {
		return SystemSettings{}, errs.Invalid("tenant_creation must be anyone or admin_only")
	}
	if p.RunRetentionMaxDays != nil && *p.RunRetentionMaxDays <= 0 {
		return SystemSettings{}, errs.Invalid("run_retention_max_days must be positive")
	}
	var by *uuid.UUID
	if actor.UserID != uuid.Nil {
		id := actor.UserID
		by = &id
	}
	out, err := s.repo.UpdateSystem(ctx, p, by)
	if err != nil {
		return SystemSettings{}, err
	}
	s.record(ctx, "admin.settings.update", nil, audit.Target{Kind: "system_settings", ID: "1"}, nil)
	return out, nil
}

// Status is the status page.
func (s *Service) Status(ctx context.Context, actor auth.Actor) (Status, error) {
	if err := s.Require(ctx, actor); err != nil {
		return Status{}, err
	}
	out := Status{Components: map[string]Component{}}
	if s.probes != nil {
		out.Components = s.probes.Check(ctx)
	}
	var err error
	if out.Runs, err = s.repo.RunCounts(ctx); err != nil {
		return Status{}, err
	}
	if out.Tenants, err = s.repo.TenantCounts(ctx); err != nil {
		return Status{}, err
	}
	tenants, err := s.repo.Tenants(ctx, TenantQuery{Limit: 200})
	if err != nil {
		return Status{}, err
	}
	known := map[string]Namespace{}
	if s.pipelines != nil {
		states, err := s.pipelines.States(ctx)
		if err != nil {
			return Status{}, err
		}
		for _, st := range states {
			known[st.Namespace] = st
		}
	}
	for _, t := range tenants {
		ns := Namespace{Namespace: t.GrapheneNamespace, TenantSlug: t.Slug, Status: "pending"}
		if st, ok := known[t.GrapheneNamespace]; ok {
			ns.Status, ns.Revision, ns.Error, ns.PushedAt = st.Status, st.Revision, st.Error, st.PushedAt
		}
		out.Namespaces = append(out.Namespaces, ns)
	}
	return out, nil
}

// Resync asks for a pipeline push of the namespaces (all, or one tenant's).
// The push itself is the pipeline worker's job; this returns what will be
// pushed.
func (s *Service) Resync(ctx context.Context, actor auth.Actor, tenantSlug string) ([]string, error) {
	if err := s.Require(ctx, actor); err != nil {
		return nil, err
	}
	if tenantSlug != "" {
		t, err := s.tenants.BySlug(ctx, tenantSlug)
		if err != nil {
			return nil, err
		}
		s.record(ctx, "admin.pipelines.resync", &t.ID, audit.Target{Kind: "namespace", ID: t.GrapheneNamespace}, nil)
		if s.pipelines != nil {
			ns := t.GrapheneNamespace
			go s.pipelines.Sync(context.WithoutCancel(ctx), ns)
		}
		return []string{t.GrapheneNamespace}, nil
	}
	tenants, err := s.repo.Tenants(ctx, TenantQuery{Limit: 200})
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(tenants))
	for _, t := range tenants {
		out = append(out, t.GrapheneNamespace)
	}
	s.record(ctx, "admin.pipelines.resync", nil, audit.Target{Kind: "namespace", ID: "*"}, map[string]any{"count": len(out)})
	if s.pipelines != nil {
		go s.pipelines.SyncAll(context.WithoutCancel(ctx), true)
	}
	return out, nil
}
