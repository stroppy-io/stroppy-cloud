// Package share publishes snapshots of runs, suite runs and comparisons
// under a token (§16.8). The snapshot is an allowlisted projection taken
// at share time — secret fields masked, no credentials, no run spec —
// and can be rebuilt on demand.
package share

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/audit"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/compare"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/suite"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/tenant"
)

// Kind of the shared target.
type Kind string

// Target kinds.
const (
	KindRun        Kind = "run"
	KindSuiteRun   Kind = "suite_run"
	KindComparison Kind = "comparison"
)

// Scope is how much of the run the page shows.
type Scope string

// Scopes, each including the previous.
const (
	ScopeOverview Scope = "overview"
	ScopeMetrics  Scope = "metrics"
	ScopeConfigs  Scope = "configs"
)

// Share is the stored record.
type Share struct {
	ID         uuid.UUID
	TenantID   uuid.UUID
	Token      string
	Kind       Kind
	TargetID   *uuid.UUID
	TargetName string
	RunIDs     []uuid.UUID
	Scope      Scope
	Title      string
	Snapshot   Snapshot
	CapturedAt time.Time
	ExpiresAt  *time.Time
	RevokedAt  *time.Time
	ViewCount  int
	CreatedBy  *uuid.UUID
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Active reports whether the link works now.
func (s Share) Active() bool {
	return s.RevokedAt == nil && (s.ExpiresAt == nil || s.ExpiresAt.After(time.Now()))
}

// Snapshot is what the public page shows.
type Snapshot struct {
	Kind       Kind                 `json:"kind"`
	Scope      Scope                `json:"scope"`
	Title      string               `json:"title,omitempty"`
	CapturedAt time.Time            `json:"captured_at"`
	TenantName string               `json:"tenant_name,omitempty"`
	Run        *SharedRun           `json:"run,omitempty"`
	SuiteRun   *SharedSuiteRun      `json:"suite_run,omitempty"`
	Comparison *SharedComparison    `json:"comparison,omitempty"`
	Runs       map[string]SharedRun `json:"runs,omitempty"`
}

// SharedRun is the allowlisted projection of a run.
type SharedRun struct {
	ID          uuid.UUID                    `json:"id"`
	Name        string                       `json:"name"`
	Status      run.Status                   `json:"status"`
	StartedAt   *time.Time                   `json:"started_at,omitempty"`
	FinishedAt  *time.Time                   `json:"finished_at,omitempty"`
	Duration    *float64                     `json:"duration_seconds,omitempty"`
	Summary     run.Summary                  `json:"summary"`
	Result      json.RawMessage              `json:"result,omitempty"`
	Timeline    []run.Event                  `json:"timeline"`
	Database    SharedDatabase               `json:"database"`
	Workload    SharedWorkload               `json:"workload"`
	Machines    []run.MachineSnapshot        `json:"machines"`
	Topology    string                       `json:"topology_label,omitempty"`
	Configs     map[string]map[string]string `json:"configs,omitempty"`
	Segments    []run.SegmentState           `json:"segments,omitempty"`
	RunSpecView map[string]any               `json:"-"`
}

// SharedDatabase is the database part without external DSNs.
type SharedDatabase struct {
	Kind    string          `json:"kind"`
	Version string          `json:"version"`
	Image   string          `json:"image,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// SharedWorkload is the workload part.
type SharedWorkload struct {
	StroppyVersion string            `json:"stroppy_version"`
	Protocol       string            `json:"protocol"`
	Segments       []json.RawMessage `json:"segments"`
}

// SharedSuiteRun is the suite run projection.
type SharedSuiteRun struct {
	Name    string           `json:"name"`
	Status  run.Status       `json:"status"`
	Cells   []SharedRun      `json:"cells"`
	Summary suite.RunSummary `json:"summary"`
}

// SharedComparison is the comparison projection.
type SharedComparison struct {
	Request    compare.Request    `json:"request"`
	Comparison compare.Comparison `json:"comparison"`
}

// Create is the share input.
type Create struct {
	Kind       Kind
	TargetID   *uuid.UUID
	RunIDs     []uuid.UUID
	Comparison *compare.Request
	TTL        *time.Duration
	Scope      Scope
	Title      string
}

// Patch changes the ttl or scope.
type Patch struct {
	TTL      *time.Duration
	ClearTTL bool
	Scope    *Scope
}

// ListQuery filters shares.
type ListQuery struct {
	Kind     string
	TargetID string
	Active   *bool
	Limit    int
	Offset   int
}

// Repository is the storage port.
type Repository interface {
	Insert(ctx context.Context, s Share) error
	ByID(ctx context.Context, id uuid.UUID) (Share, error)
	ByToken(ctx context.Context, token string) (Share, error)
	List(ctx context.Context, tenantID uuid.UUID, q ListQuery) ([]Share, error)
	OfTarget(ctx context.Context, kind string, id uuid.UUID) ([]run.Ref, error)
	Update(ctx context.Context, id uuid.UUID, p Patch, expiresAt *time.Time) error
	SetSnapshot(ctx context.Context, id uuid.UUID, snap Snapshot, capturedAt time.Time, title string) error
	Revoke(ctx context.Context, id uuid.UUID) error
	BumpViews(ctx context.Context, id uuid.UUID) error
}

// Masker masks secret fields of a config value.
type Masker interface {
	MaskSecrets(schemaID string, value json.RawMessage) json.RawMessage
	Render(ctx context.Context, schemaID, template string, value json.RawMessage) (string, error)
}

// Access resolves the caller and the tenant.
type Access interface {
	RoleIn(ctx context.Context, actor auth.Actor, tenantID uuid.UUID) (slug, role string, err error)
	ByID(ctx context.Context, id uuid.UUID) (tenant.Tenant, error)
}

// Service is the share use cases.
type Service struct {
	repo      Repository
	runs      run.Repository
	suites    *suite.Service
	compare   *compare.Service
	access    Access
	masker    Masker
	audit     *audit.Service
	publicURL string
}

// NewService wires the use cases; publicURL is the SPA origin the links
// point at (/s/<token>).
func NewService(repo Repository, runs run.Repository, suites *suite.Service, cmp *compare.Service, access Access, masker Masker, auditSvc *audit.Service, publicURL string) *Service {
	return &Service{repo: repo, runs: runs, suites: suites, compare: cmp, access: access, masker: masker, audit: auditSvc, publicURL: publicURL}
}

// URL of a share.
func (s *Service) URL(sh Share) string { return s.publicURL + "/s/" + sh.Token }

func (s *Service) writer(ctx context.Context, actor auth.Actor, tenantID uuid.UUID) error {
	_, role, err := s.access.RoleIn(ctx, actor, tenantID)
	if err != nil {
		return err
	}
	if role == string(tenant.RoleViewer) {
		return errs.Forbidden("viewers cannot share")
	}
	return nil
}

func (s *Service) owned(ctx context.Context, tenantID, id uuid.UUID) (Share, error) {
	sh, err := s.repo.ByID(ctx, id)
	if err != nil {
		return Share{}, err
	}
	if sh.TenantID != tenantID {
		return Share{}, errs.NotFound("share")
	}
	return sh, nil
}

// Create captures a snapshot and stores the share.
func (s *Service) Create(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, in Create) (Share, error) {
	if err := s.writer(ctx, actor, tenantID); err != nil {
		return Share{}, err
	}
	if in.Scope == "" {
		in.Scope = ScopeOverview
	}
	if in.Scope != ScopeOverview && in.Scope != ScopeMetrics && in.Scope != ScopeConfigs {
		return Share{}, errs.Invalid("scope must be overview, metrics or configs")
	}
	sh := Share{
		ID: uuid.New(), TenantID: tenantID, Token: newToken(), Kind: in.Kind, TargetID: in.TargetID, RunIDs: in.RunIDs, Scope: in.Scope, Title: in.Title,
		CreatedBy: authorOf(actor), CreatedAt: time.Now().UTC(),
	}
	if in.TTL != nil && *in.TTL > 0 {
		t := time.Now().UTC().Add(*in.TTL)
		sh.ExpiresAt = &t
	}
	if in.Comparison != nil {
		sh.RunIDs = in.Comparison.RunIDs
		sh.Snapshot.Comparison = &SharedComparison{Request: *in.Comparison}
	}
	snap, name, err := s.capture(ctx, actor, sh)
	if err != nil {
		return Share{}, err
	}
	sh.Snapshot, sh.TargetName, sh.CapturedAt = snap, name, snap.CapturedAt
	if sh.Title == "" {
		sh.Title = name
	}
	sh.Snapshot.Title = sh.Title
	if err := s.repo.Insert(ctx, sh); err != nil {
		return Share{}, err
	}
	_ = s.audit.Record(ctx, audit.Entry{TenantID: &tenantID, Action: "share.create", Target: audit.Target{Kind: "share", ID: sh.ID.String(), Name: sh.Title}, Details: map[string]any{"kind": sh.Kind, "scope": sh.Scope}}) //nolint:errcheck // audit never blocks
	return sh, nil
}

// capture builds the snapshot of the share's target.
func (s *Service) capture(ctx context.Context, actor auth.Actor, sh Share) (Snapshot, string, error) {
	t, err := s.access.ByID(ctx, sh.TenantID)
	if err != nil {
		return Snapshot{}, "", err
	}
	snap := Snapshot{Kind: sh.Kind, Scope: sh.Scope, Title: sh.Title, CapturedAt: time.Now().UTC()}
	if t.PublicName != nil {
		snap.TenantName = *t.PublicName
	}
	switch sh.Kind {
	case KindRun:
		if sh.TargetID == nil {
			return Snapshot{}, "", errs.Invalid("run id is required")
		}
		r, err := s.run(ctx, sh.TenantID, *sh.TargetID)
		if err != nil {
			return Snapshot{}, "", err
		}
		sr, err := s.project(ctx, r, sh.Scope)
		if err != nil {
			return Snapshot{}, "", err
		}
		snap.Run = &sr
		return snap, r.Name, nil
	case KindSuiteRun:
		if sh.TargetID == nil {
			return Snapshot{}, "", errs.Invalid("suite run id is required")
		}
		sr, err := s.suites.GetRun(ctx, actor, sh.TenantID, *sh.TargetID)
		if err != nil {
			return Snapshot{}, "", err
		}
		summary, err := s.suites.Summary(ctx, actor, sh.TenantID, sr.ID, nil)
		if err != nil {
			return Snapshot{}, "", err
		}
		out := &SharedSuiteRun{Name: sr.Name, Status: sr.Status, Summary: summary, Cells: make([]SharedRun, 0, len(sr.Runs))}
		for _, child := range sr.Runs {
			p, err := s.project(ctx, child, sh.Scope)
			if err != nil {
				return Snapshot{}, "", err
			}
			out.Cells = append(out.Cells, p)
		}
		snap.SuiteRun = out
		return snap, sr.Name, nil
	case KindComparison:
		if sh.Snapshot.Comparison == nil {
			return Snapshot{}, "", errs.Invalid("comparison request is required")
		}
		req := sh.Snapshot.Comparison.Request
		cmp, err := s.compare.Compare(ctx, actor, sh.TenantID, req)
		if err != nil {
			return Snapshot{}, "", err
		}
		snap.Comparison = &SharedComparison{Request: req, Comparison: cmp}
		snap.Runs = map[string]SharedRun{}
		for _, c := range cmp.Columns {
			p, err := s.project(ctx, c.Run, sh.Scope)
			if err != nil {
				return Snapshot{}, "", err
			}
			snap.Runs[c.Run.ID.String()] = p
		}
		return snap, "comparison of " + itoa(len(cmp.Columns)) + " runs", nil
	default:
		return Snapshot{}, "", errs.Invalid("unknown share kind")
	}
}

func (s *Service) run(ctx context.Context, tenantID, id uuid.UUID) (run.Run, error) {
	r, err := s.runs.ByID(ctx, id)
	if err != nil {
		return run.Run{}, err
	}
	if r.TenantID != tenantID || r.DeletedAt != nil {
		return run.Run{}, errs.NotFound("run")
	}
	return r, nil
}

// project is the allowlist: nothing from the run spec, no external DSN,
// configs rendered with secrets masked (scope configs only).
func (s *Service) project(ctx context.Context, r run.Run, scope Scope) (SharedRun, error) {
	events, err := s.runs.EventsAfter(ctx, r.ID, 0, 1000)
	if err != nil {
		return SharedRun{}, err
	}
	if events == nil {
		events = []run.Event{}
	}
	out := SharedRun{
		ID: r.ID, Name: r.Name, Status: r.Status, StartedAt: r.StartedAt, FinishedAt: r.FinishedAt, Duration: r.DurationSeconds, Summary: r.Summary, Result: r.Result, Timeline: events,
		Database: SharedDatabase{Kind: string(r.Snapshot.Database.Kind), Version: r.Snapshot.Database.Version, Image: r.Snapshot.Database.Image, Params: r.Snapshot.Database.Params},
		Workload: SharedWorkload{StroppyVersion: r.Snapshot.Workload.StroppyVersion, Protocol: string(r.Snapshot.Workload.Protocol), Segments: r.Snapshot.Workload.Segments},
		Machines: r.Snapshot.Machines, Topology: r.Snapshot.TopologyLabel, Segments: r.State.Segments,
	}
	if out.Machines == nil {
		out.Machines = []run.MachineSnapshot{}
	}
	if out.Workload.Segments == nil {
		out.Workload.Segments = []json.RawMessage{}
	}
	if scope == ScopeConfigs && s.masker != nil {
		out.Configs = map[string]map[string]string{}
		for role, byID := range r.Snapshot.EffectiveConfigs {
			for id, raw := range byID {
				masked := s.masker.MaskSecrets(id, raw)
				rendered, err := s.masker.Render(ctx, id, "", masked)
				if err != nil {
					// Masked values may not fit the schema (patterns); keep the JSON.
					rendered = string(masked)
				}
				if out.Configs[role] == nil {
					out.Configs[role] = map[string]string{}
				}
				out.Configs[role][id] = rendered
			}
		}
	}
	return out, nil
}

// Get reads one share.
func (s *Service) Get(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID) (Share, error) {
	if _, _, err := s.access.RoleIn(ctx, actor, tenantID); err != nil {
		return Share{}, err
	}
	return s.owned(ctx, tenantID, id)
}

// List reads the tenant's shares.
func (s *Service) List(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, q ListQuery) ([]Share, error) {
	if _, _, err := s.access.RoleIn(ctx, actor, tenantID); err != nil {
		return nil, err
	}
	return s.repo.List(ctx, tenantID, q)
}

// OfTarget lists active shares of a run or suite run (the run's `shares`).
func (s *Service) OfTarget(ctx context.Context, kind Kind, id uuid.UUID) []run.Ref {
	refs, err := s.repo.OfTarget(ctx, string(kind), id)
	if err != nil {
		return nil
	}
	return refs
}

// PublicTokenOf resolves the newest active share of a run (global rating).
func (s *Service) PublicTokenOf(ctx context.Context, runID uuid.UUID) (string, bool) {
	refs, err := s.repo.OfTarget(ctx, string(KindRun), runID)
	if err != nil || len(refs) == 0 {
		return "", false
	}
	sh, err := s.repo.ByID(ctx, refs[0].ID)
	if err != nil || !sh.Active() {
		return "", false
	}
	return sh.Token, true
}

// Update changes ttl / scope; a scope change re-captures the snapshot.
func (s *Service) Update(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID, p Patch) (Share, error) {
	if err := s.writer(ctx, actor, tenantID); err != nil {
		return Share{}, err
	}
	sh, err := s.owned(ctx, tenantID, id)
	if err != nil {
		return Share{}, err
	}
	var expires *time.Time
	if p.TTL != nil && *p.TTL > 0 {
		t := time.Now().UTC().Add(*p.TTL)
		expires = &t
	}
	if err := s.repo.Update(ctx, id, p, expires); err != nil {
		return Share{}, err
	}
	if p.Scope != nil && *p.Scope != sh.Scope {
		sh.Scope = *p.Scope
		if _, err := s.rebuild(ctx, actor, sh); err != nil {
			return Share{}, err
		}
	}
	return s.repo.ByID(ctx, id)
}

// Rebuild re-captures the snapshot.
func (s *Service) Rebuild(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID) (Share, error) {
	if err := s.writer(ctx, actor, tenantID); err != nil {
		return Share{}, err
	}
	sh, err := s.owned(ctx, tenantID, id)
	if err != nil {
		return Share{}, err
	}
	return s.rebuild(ctx, actor, sh)
}

func (s *Service) rebuild(ctx context.Context, actor auth.Actor, sh Share) (Share, error) {
	snap, name, err := s.capture(ctx, actor, sh)
	if err != nil {
		return Share{}, err
	}
	title := sh.Title
	if title == "" {
		title = name
	}
	snap.Title = title
	if err := s.repo.SetSnapshot(ctx, sh.ID, snap, snap.CapturedAt, title); err != nil {
		return Share{}, err
	}
	_ = s.audit.Record(ctx, audit.Entry{TenantID: &sh.TenantID, Action: "share.rebuild", Target: audit.Target{Kind: "share", ID: sh.ID.String()}}) //nolint:errcheck // audit never blocks
	return s.repo.ByID(ctx, sh.ID)
}

// Revoke stops the link.
func (s *Service) Revoke(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID) error {
	if err := s.writer(ctx, actor, tenantID); err != nil {
		return err
	}
	if _, err := s.owned(ctx, tenantID, id); err != nil {
		return err
	}
	if err := s.repo.Revoke(ctx, id); err != nil {
		return err
	}
	_ = s.audit.Record(ctx, audit.Entry{TenantID: &tenantID, Action: "share.revoke", Target: audit.Target{Kind: "share", ID: id.String()}}) //nolint:errcheck // audit never blocks
	return nil
}

// Public resolves a token for the public page and counts the view.
func (s *Service) Public(ctx context.Context, token string) (Share, error) {
	sh, err := s.repo.ByToken(ctx, token)
	if err != nil {
		return Share{}, errs.NotFound("share")
	}
	if !sh.Active() {
		return Share{}, errs.NotFound("share")
	}
	_ = s.repo.BumpViews(ctx, sh.ID) //nolint:errcheck // best-effort
	return sh, nil
}

func newToken() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b) // crypto/rand never fails on supported platforms
	return base64.RawURLEncoding.EncodeToString(b)
}

func itoa(i int) string {
	b, _ := json.Marshal(i) //nolint:errcheck // int
	return string(b)
}

func authorOf(a auth.Actor) *uuid.UUID {
	if a.UserID == uuid.Nil {
		return nil
	}
	id := a.UserID
	return &id
}

// MetricsRun resolves the sole run authorized by an active metrics/configs link.
// Its time bounds come from the published snapshot, not later run changes.
func (s *Service) MetricsRun(ctx context.Context, token string) (run.Run, error) {
	sh, err := s.Public(ctx, token)
	if err != nil {
		return run.Run{}, err
	}
	if sh.Scope == ScopeOverview {
		return run.Run{}, errs.Forbidden("this share does not include metrics")
	}
	if sh.Kind != KindRun || sh.TargetID == nil || sh.Snapshot.Run == nil {
		return run.Run{}, errs.Invalid("metrics endpoint requires a single-run share")
	}
	r, err := s.run(ctx, sh.TenantID, *sh.TargetID)
	if err != nil {
		return run.Run{}, err
	}
	r.StartedAt = sh.Snapshot.Run.StartedAt
	r.FinishedAt = sh.Snapshot.Run.FinishedAt
	if r.FinishedAt == nil {
		r.FinishedAt = &sh.CapturedAt
	}
	return r, nil
}
