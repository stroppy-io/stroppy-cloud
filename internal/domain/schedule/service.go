package schedule

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gopherex/xlog"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/audit"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/tenant"
)

// Service is the schedule use cases and the firing loop.
type Service struct {
	repo     Repository
	access   Access
	launcher Launcher
	audit    *audit.Service
	log      *xlog.Logger
	now      func() time.Time
}

// NewService wires the use cases.
func NewService(repo Repository, access Access, launcher Launcher, auditSvc *audit.Service, log *xlog.Logger) *Service {
	return &Service{repo: repo, access: access, launcher: launcher, audit: auditSvc, log: log, now: func() time.Time { return time.Now().UTC() }}
}

func (s *Service) member(ctx context.Context, actor auth.Actor, tenantID uuid.UUID) error {
	_, _, err := s.access.RoleIn(ctx, actor, tenantID)
	return err
}

func (s *Service) writer(ctx context.Context, actor auth.Actor, tenantID uuid.UUID) error {
	_, role, err := s.access.RoleIn(ctx, actor, tenantID)
	if err != nil {
		return err
	}
	if role == string(tenant.RoleViewer) {
		return errs.Forbidden("viewers cannot change schedules").Because("viewers_cannot_change_schedules")
	}
	return nil
}

func (s *Service) owned(ctx context.Context, tenantID, id uuid.UUID) (Schedule, error) {
	x, err := s.repo.ByID(ctx, id)
	if err != nil {
		return Schedule{}, err
	}
	if x.TenantID != tenantID {
		return Schedule{}, errs.NotFound("schedule")
	}
	return x, nil
}

// next computes the next firing of an expression in a timezone.
func next(expr, tz string, from time.Time) (*time.Time, error) {
	c, err := ParseCron(expr)
	if err != nil {
		return nil, errs.Invalid(err.Error()).Because("cron_invalid", "error", err.Error())
	}
	loc, err := time.LoadLocation(orDefault(tz, "UTC"))
	if err != nil {
		return nil, errs.Invalid("unknown timezone "+tz).Because("timezone_unknown", "tz", tz)
	}
	t := c.Next(from.In(loc))
	if t.IsZero() {
		return nil, errs.Invalid("cron never fires").Because("cron_never_fires")
	}
	u := t.UTC()
	return &u, nil
}

// Create stores a schedule.
func (s *Service) Create(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, w Write) (Schedule, error) {
	if err := s.writer(ctx, actor, tenantID); err != nil {
		return Schedule{}, err
	}
	if strings.TrimSpace(w.Name) == "" {
		return Schedule{}, errs.Invalid("name is required").Because("name_is_required")
	}
	if w.TargetKind != TargetTest && w.TargetKind != TargetSuite {
		return Schedule{}, errs.Invalid("target.kind must be test or suite").Because("target_kind_must_be_test_or_suite")
	}
	name, err := s.launcher.TargetName(ctx, actor, tenantID, w.TargetKind, w.TargetID)
	if err != nil {
		return Schedule{}, err
	}
	enabled := w.Enabled == nil || *w.Enabled
	var nextAt *time.Time
	if enabled {
		if nextAt, err = next(w.Cron, w.Timezone, s.now()); err != nil {
			return Schedule{}, err
		}
	} else if _, err := ParseCron(w.Cron); err != nil {
		return Schedule{}, errs.Invalid(err.Error()).Because("cron_invalid", "error", err.Error())
	}
	now := s.now()
	x := Schedule{
		ID: uuid.New(), TenantID: tenantID, Name: w.Name, TargetKind: w.TargetKind, TargetID: w.TargetID, TargetName: name, Cron: strings.TrimSpace(w.Cron),
		Timezone: orDefault(w.Timezone, "UTC"), Enabled: enabled, Overrides: w.Overrides, NextRunAt: nextAt, AuthorID: authorOf(actor), CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repo.Insert(ctx, x); err != nil {
		return Schedule{}, err
	}
	_ = s.audit.Record(ctx, audit.Entry{TenantID: &tenantID, Action: "schedule.create", Target: audit.Target{Kind: "schedule", ID: x.ID.String(), Name: x.Name}}) //nolint:errcheck // audit never blocks
	return x, nil
}

// Get reads one schedule.
func (s *Service) Get(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID) (Schedule, error) {
	if err := s.member(ctx, actor, tenantID); err != nil {
		return Schedule{}, err
	}
	return s.owned(ctx, tenantID, id)
}

// List reads the tenant's schedules.
func (s *Service) List(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, q ListQuery) ([]Schedule, error) {
	if err := s.member(ctx, actor, tenantID); err != nil {
		return nil, err
	}
	return s.repo.List(ctx, tenantID, q)
}

// Upcoming lists the next firings of the tenant (dashboard).
func (s *Service) Upcoming(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, limit int) ([]Schedule, error) {
	if err := s.member(ctx, actor, tenantID); err != nil {
		return nil, err
	}
	return s.repo.Upcoming(ctx, tenantID, limit)
}

// Update applies a patch and recomputes the next firing.
func (s *Service) Update(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID, p Patch) (Schedule, error) {
	if err := s.writer(ctx, actor, tenantID); err != nil {
		return Schedule{}, err
	}
	x, err := s.owned(ctx, tenantID, id)
	if err != nil {
		return Schedule{}, err
	}
	kind, target := x.TargetKind, x.TargetID
	if p.TargetKind != nil {
		kind = *p.TargetKind
	}
	if p.TargetID != nil {
		target = *p.TargetID
	}
	targetName := x.TargetName
	if p.TargetKind != nil || p.TargetID != nil {
		if kind != TargetTest && kind != TargetSuite {
			return Schedule{}, errs.Invalid("target.kind must be test or suite").Because("target_kind_must_be_test_or_suite")
		}
		if targetName, err = s.launcher.TargetName(ctx, actor, tenantID, kind, target); err != nil {
			return Schedule{}, err
		}
	}
	expr, tz, enabled := x.Cron, x.Timezone, x.Enabled
	if p.Cron != nil {
		expr = *p.Cron
	}
	if p.Timezone != nil {
		tz = *p.Timezone
	}
	if p.Enabled != nil {
		enabled = *p.Enabled
	}
	var nextAt *time.Time
	if enabled {
		if nextAt, err = next(expr, tz, s.now()); err != nil {
			return Schedule{}, err
		}
	} else if _, err := ParseCron(expr); err != nil {
		return Schedule{}, errs.Invalid(err.Error()).Because("cron_invalid", "error", err.Error())
	}
	if err := s.repo.Update(ctx, id, p, targetName, nextAt); err != nil {
		return Schedule{}, err
	}
	_ = s.audit.Record(ctx, audit.Entry{TenantID: &tenantID, Action: "schedule.update", Target: audit.Target{Kind: "schedule", ID: id.String()}}) //nolint:errcheck // audit never blocks
	return s.repo.ByID(ctx, id)
}

// SetEnabled pauses or resumes.
func (s *Service) SetEnabled(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID, enabled bool) (Schedule, error) {
	return s.Update(ctx, actor, tenantID, id, Patch{Enabled: &enabled})
}

// Delete removes a schedule.
func (s *Service) Delete(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID) error {
	if err := s.writer(ctx, actor, tenantID); err != nil {
		return err
	}
	if _, err := s.owned(ctx, tenantID, id); err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	_ = s.audit.Record(ctx, audit.Entry{TenantID: &tenantID, Action: "schedule.delete", Target: audit.Target{Kind: "schedule", ID: id.String()}}) //nolint:errcheck // audit never blocks
	return nil
}

// History lists what the schedule launched, newest first.
func (s *Service) History(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID, limit, offset int) ([]RunRef, error) {
	if _, err := s.Get(ctx, actor, tenantID, id); err != nil {
		return nil, err
	}
	return s.repo.History(ctx, id, limit, offset)
}

// RunNow fires the schedule once on behalf of the actor.
func (s *Service) RunNow(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID) (RunRef, error) {
	if err := s.writer(ctx, actor, tenantID); err != nil {
		return RunRef{}, err
	}
	x, err := s.owned(ctx, tenantID, id)
	if err != nil {
		return RunRef{}, err
	}
	ref := s.fire(ctx, actor, x)
	if ref.Error != "" {
		return ref, errs.Invalid(ref.Error).Because("schedule_fire_failed", "error", ref.Error)
	}
	return ref, nil
}

// fire launches the target and records the outcome (history + last run).
func (s *Service) fire(ctx context.Context, actor auth.Actor, x Schedule) RunRef {
	o := run.Overrides{
		Name: x.Overrides.Name, ProviderProfileID: x.Overrides.ProviderProfileID, Sizes: x.Overrides.Sizes, Keep: x.Overrides.Keep,
		RatingTenant: x.Overrides.RatingTenant, RatingGlobal: x.Overrides.RatingGlobal, Labels: x.Overrides.Labels, Notes: x.Overrides.Notes,
		Trigger: run.TriggerSchedule, ScheduleID: &x.ID,
	}
	ref := RunRef{At: s.now()}
	var id uuid.UUID
	var name, status string
	var err error
	switch x.TargetKind { //nolint:exhaustive // test is the default
	case TargetSuite:
		ref.Kind = "suite_run"
		id, name, status, err = s.launcher.LaunchSuite(ctx, actor, x.TenantID, x.TargetID, o)
	default:
		ref.Kind = "run"
		id, name, status, err = s.launcher.LaunchTest(ctx, actor, x.TenantID, x.TargetID, o)
	}
	if err != nil {
		ref.Status, ref.Error = "failed", err.Error()
	} else {
		ref.ID, ref.Name, ref.Status = &id, name, status
	}
	_ = s.repo.AddHistory(ctx, x.ID, ref) //nolint:errcheck // best-effort
	var nextAt *time.Time
	if x.Enabled {
		nextAt, _ = next(x.Cron, x.Timezone, s.now()) //nolint:errcheck // validated on write
	}
	_ = s.repo.SetFired(ctx, x.ID, nextAt, ref) //nolint:errcheck // best-effort
	return ref
}

// Tick fires every due schedule as its author. Returns how many fired.
func (s *Service) Tick(ctx context.Context) int {
	due, err := s.repo.Due(ctx, s.now())
	if err != nil {
		s.log.Warn("schedules: due", xlog.Error("error", err))
		return 0
	}
	n := 0
	for _, x := range due {
		if x.AuthorID == nil {
			_ = s.repo.SetFired(ctx, x.ID, nil, RunRef{Kind: "run", Status: "failed", At: s.now(), Error: "schedule has no author"}) //nolint:errcheck // best-effort
			continue
		}
		actor := auth.Actor{UserID: *x.AuthorID}
		ref := s.fire(auth.WithActor(ctx, actor), actor, x)
		if ref.Error != "" {
			s.log.Warn("schedule fired with error", xlog.String("schedule", x.ID.String()), xlog.String("error", ref.Error))
		}
		n++
	}
	return n
}

// Run ticks until ctx ends.
func (s *Service) Run(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.Tick(ctx)
		}
	}
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func authorOf(a auth.Actor) *uuid.UUID {
	if a.UserID == uuid.Nil {
		return nil
	}
	id := a.UserID
	return &id
}
