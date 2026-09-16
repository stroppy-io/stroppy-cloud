package provider

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/audit"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
)

// QuotaFreshness is how old a quota cache may be before a launch re-probes.
const QuotaFreshness = time.Minute

// Access resolves the caller's role and namespace in a tenant.
type Access interface {
	RoleIn(ctx context.Context, actor auth.Actor, tenantID uuid.UUID) (slug, role string, err error)
	NamespaceOf(ctx context.Context, tenantID uuid.UUID) (string, error)
}

// Scoper scopes a context to a Graphene namespace.
type Scoper func(ctx context.Context, namespace string) context.Context

// Service is the provider-profile use cases.
type Service struct {
	repo      Repository
	secrets   Secrets
	pipelines Pipelines
	validate  Validator
	access    Access
	scope     Scoper
	audit     *audit.Service
	// background runs the verification after the request returns.
	background func(func(ctx context.Context))
}

// NewService builds the service. background runs detached work with a
// process-scoped context (the application's shutdown manager).
func NewService(repo Repository, secrets Secrets, pipelines Pipelines, validate Validator, access Access, scope Scoper, auditSvc *audit.Service, background func(func(ctx context.Context))) *Service {
	return &Service{repo: repo, secrets: secrets, pipelines: pipelines, validate: validate, access: access, scope: scope, audit: auditSvc, background: background}
}

// Create is the request.
type Create struct {
	Name        string
	Kind        Kind
	Settings    json.RawMessage
	Credentials json.RawMessage
}

// Patch is a partial update; nil = keep. New credentials re-verify.
type Patch struct {
	Name        *string
	Settings    json.RawMessage
	Credentials json.RawMessage
}

func (s *Service) admin(ctx context.Context, actor auth.Actor, tenantID uuid.UUID) error {
	_, role, err := s.access.RoleIn(ctx, actor, tenantID)
	if err != nil {
		return err
	}
	if role != "owner" && role != "admin" {
		return errs.Forbidden("requires role admin")
	}
	return nil
}

func (s *Service) member(ctx context.Context, actor auth.Actor, tenantID uuid.UUID) error {
	_, _, err := s.access.RoleIn(ctx, actor, tenantID)
	return err
}

// List returns the tenant's profiles (any member).
func (s *Service) List(ctx context.Context, actor auth.Actor, tenantID uuid.UUID) ([]Profile, error) {
	if err := s.member(ctx, actor, tenantID); err != nil {
		return nil, err
	}
	return s.repo.OfTenant(ctx, tenantID)
}

// Get returns one profile (any member).
func (s *Service) Get(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID) (Profile, error) {
	if err := s.member(ctx, actor, tenantID); err != nil {
		return Profile{}, err
	}
	return s.owned(ctx, tenantID, id)
}

// ByID reads a profile of the tenant without an access check (internal
// callers that already authorized the actor).
func (s *Service) ByID(ctx context.Context, tenantID, id uuid.UUID) (Profile, error) {
	return s.owned(ctx, tenantID, id)
}

func (s *Service) owned(ctx context.Context, tenantID, id uuid.UUID) (Profile, error) {
	p, err := s.repo.ByID(ctx, id)
	if err != nil {
		return Profile{}, err
	}
	if p.TenantID != tenantID {
		return Profile{}, errs.NotFound("provider profile")
	}
	return p, nil
}

// Create validates settings and credentials, stores the credentials as a
// Graphene secret of the tenant, records the profile as verifying and
// starts the verification in the background.
func (s *Service) Create(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, req Create) (Profile, error) {
	if err := s.admin(ctx, actor, tenantID); err != nil {
		return Profile{}, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" || len(name) > 64 {
		return Profile{}, errs.Invalid("name must be 1..64 characters")
	}
	if !req.Kind.Valid() {
		return Profile{}, errs.Invalid("kind must be yandex or aws")
	}
	settings, err := s.validate.Bake(ctx, settingsSchema(req.Kind), req.Settings)
	if err != nil {
		return Profile{}, err
	}
	creds, err := s.validate.Bake(ctx, credentialsSchema(req.Kind), req.Credentials)
	if err != nil {
		return Profile{}, err
	}
	ns, err := s.access.NamespaceOf(ctx, tenantID)
	if err != nil {
		return Profile{}, err
	}
	p := Profile{
		ID: uuid.New(), TenantID: tenantID, Name: name, Kind: req.Kind, Settings: settings,
		Status: StatusVerifying, CreatedAt: time.Now().UTC(),
	}
	p.SecretNames = []string{CredentialsSecret(p.ID)}
	if !actor.IsAPIToken() {
		by := actor.UserID
		p.CreatedBy = &by
	}
	if err := s.secrets.SetSecret(s.scope(ctx, ns), p.SecretNames[0], string(creds)); err != nil {
		return Profile{}, errs.Wrap(errs.CodeUnavailable, "graphene secret", err)
	}
	if err := s.repo.Insert(ctx, p); err != nil {
		return Profile{}, err
	}
	_ = s.audit.Record(ctx, audit.Entry{TenantID: &tenantID, Action: "provider.create", Target: audit.Target{Kind: "provider", ID: p.ID.String(), Name: name}, Details: map[string]any{"kind": req.Kind}}) //nolint:errcheck // audit never blocks
	s.startVerify(ns, p)
	return s.repo.ByID(ctx, p.ID)
}

// Update renames, changes settings (re-verifies) or rotates credentials
// (re-verifies).
func (s *Service) Update(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID, patch Patch) (Profile, error) {
	if err := s.admin(ctx, actor, tenantID); err != nil {
		return Profile{}, err
	}
	p, err := s.owned(ctx, tenantID, id)
	if err != nil {
		return Profile{}, err
	}
	reverify := false
	if patch.Name != nil {
		n := strings.TrimSpace(*patch.Name)
		if n == "" || len(n) > 64 {
			return Profile{}, errs.Invalid("name must be 1..64 characters")
		}
		patch.Name = &n
	}
	var settings json.RawMessage
	if patch.Settings != nil {
		settings, err = s.validate.Bake(ctx, settingsSchema(p.Kind), patch.Settings)
		if err != nil {
			return Profile{}, err
		}
		reverify = true
	}
	ns, err := s.access.NamespaceOf(ctx, tenantID)
	if err != nil {
		return Profile{}, err
	}
	if patch.Credentials != nil {
		creds, err := s.validate.Bake(ctx, credentialsSchema(p.Kind), patch.Credentials)
		if err != nil {
			return Profile{}, err
		}
		if err := s.secrets.SetSecret(s.scope(ctx, ns), CredentialsSecret(p.ID), string(creds)); err != nil {
			return Profile{}, errs.Wrap(errs.CodeUnavailable, "graphene secret", err)
		}
		reverify = true
	}
	if err := s.repo.Update(ctx, id, patch.Name, settings); err != nil {
		return Profile{}, err
	}
	_ = s.audit.Record(ctx, audit.Entry{TenantID: &tenantID, Action: "provider.update", Target: audit.Target{Kind: "provider", ID: id.String(), Name: p.Name}, Details: map[string]any{"reverify": reverify}}) //nolint:errcheck // audit never blocks
	if reverify {
		if err := s.repo.SetStatus(ctx, id, StatusVerifying, "", ""); err != nil {
			return Profile{}, err
		}
		p, err = s.repo.ByID(ctx, id)
		if err != nil {
			return Profile{}, err
		}
		s.startVerify(ns, p)
	}
	return s.repo.ByID(ctx, id)
}

// Delete removes a profile and its secret (admin+; live-run guard is the
// launch layer's — a run keeps its baked credentials reference).
func (s *Service) Delete(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID) error {
	if err := s.admin(ctx, actor, tenantID); err != nil {
		return err
	}
	p, err := s.owned(ctx, tenantID, id)
	if err != nil {
		return err
	}
	ns, err := s.access.NamespaceOf(ctx, tenantID)
	if err != nil {
		return err
	}
	if err := s.repo.SoftDelete(ctx, id); err != nil {
		return err
	}
	for _, name := range p.SecretNames {
		if err := s.secrets.DeleteSecret(s.scope(ctx, ns), name); err != nil {
			return errs.Wrap(errs.CodeUnavailable, "graphene secret", err)
		}
	}
	return s.audit.Record(ctx, audit.Entry{TenantID: &tenantID, Action: "provider.delete", Target: audit.Target{Kind: "provider", ID: id.String(), Name: p.Name}})
}

// Verify re-runs the verification (admin+) and returns the profile as
// verifying; the result lands asynchronously.
func (s *Service) Verify(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID) (Profile, error) {
	if err := s.admin(ctx, actor, tenantID); err != nil {
		return Profile{}, err
	}
	p, err := s.owned(ctx, tenantID, id)
	if err != nil {
		return Profile{}, err
	}
	ns, err := s.access.NamespaceOf(ctx, tenantID)
	if err != nil {
		return Profile{}, err
	}
	if err := s.repo.SetStatus(ctx, id, StatusVerifying, "", ""); err != nil {
		return Profile{}, err
	}
	s.startVerify(ns, p)
	return s.repo.ByID(ctx, id)
}

// startVerify runs the pipeline detached and records the outcome.
func (s *Service) startVerify(ns string, p Profile) {
	s.background(func(ctx context.Context) {
		runID := uuid.NewString()
		res, err := s.pipelines.Verify(s.scope(ctx, ns), runID, VerifyParams{
			Provider: p.Kind, Settings: p.Settings, CredentialsSecret: CredentialsSecret(p.ID), DryRun: true,
		})
		switch {
		case err != nil:
			_ = s.repo.SetStatus(ctx, p.ID, StatusFailed, "verification did not complete: "+err.Error(), runID) //nolint:errcheck // nothing else to do
		case !res.OK:
			_ = s.repo.SetStatus(ctx, p.ID, StatusFailed, res.Error, runID) //nolint:errcheck // nothing else to do
		default:
			_ = s.repo.SetStatus(ctx, p.ID, StatusReady, "", runID) //nolint:errcheck // nothing else to do
			_ = s.refreshQuotas(ctx, ns, p)                         //nolint:errcheck // first quota snapshot is best-effort
		}
	})
}

// Quotas returns the cached quotas (any member); stale is the caller's to judge.
func (s *Service) Quotas(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID) (Profile, error) {
	return s.Get(ctx, actor, tenantID, id)
}

// RefreshQuotas runs the quota pipeline now and waits (admin+).
func (s *Service) RefreshQuotas(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID) (Profile, error) {
	if err := s.admin(ctx, actor, tenantID); err != nil {
		return Profile{}, err
	}
	p, err := s.owned(ctx, tenantID, id)
	if err != nil {
		return Profile{}, err
	}
	if p.Status != StatusReady {
		return Profile{}, errs.Conflict("profile is not verified")
	}
	ns, err := s.access.NamespaceOf(ctx, tenantID)
	if err != nil {
		return Profile{}, err
	}
	if err := s.refreshQuotas(ctx, ns, p); err != nil {
		return Profile{}, errs.Wrap(errs.CodeUnavailable, "quota pipeline", err)
	}
	return s.repo.ByID(ctx, id)
}

// FreshQuotas is the launch path: cache when young enough, else a probe.
// Permission-denied snapshots have no quotas; YC enforces limits at creation.
func (s *Service) FreshQuotas(ctx context.Context, p Profile) ([]Quota, error) {
	if p.QuotasObservedAt != nil && time.Since(*p.QuotasObservedAt) <= QuotaFreshness {
		return p.Quotas, nil
	}
	ns, err := s.access.NamespaceOf(ctx, p.TenantID)
	if err != nil {
		return nil, err
	}
	if err := s.refreshQuotas(ctx, ns, p); err != nil {
		return nil, err
	}
	fresh, err := s.repo.ByID(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	return fresh.Quotas, nil
}

// RefreshAllReady is the periodic sweep over every ready profile.
func (s *Service) RefreshAllReady(ctx context.Context) error {
	profiles, err := s.repo.Ready(ctx)
	if err != nil {
		return err
	}
	for _, p := range profiles {
		ns, err := s.access.NamespaceOf(ctx, p.TenantID)
		if err != nil {
			continue
		}
		_ = s.refreshQuotas(ctx, ns, p) //nolint:errcheck // one bad profile must not stop the sweep
	}
	return nil
}

func (s *Service) refreshQuotas(ctx context.Context, ns string, p Profile) error {
	res, err := s.pipelines.Quotas(s.scope(ctx, ns), uuid.NewString(), QuotasParams{
		Provider: p.Kind, Settings: p.Settings, CredentialsSecret: CredentialsSecret(p.ID),
	})
	if err != nil {
		return err
	}
	observed := res.ObservedAt
	if observed.IsZero() {
		observed = time.Now().UTC()
	}
	res.ObservedAt = observed
	if res.UnavailableReason != "" {
		res.Quotas = []Quota{}
	}
	return s.repo.SetQuotas(ctx, p.ID, res)
}
