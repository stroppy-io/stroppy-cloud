// Package settings is the tenant's defaults and effective limits. Limits
// are platform ceilings a platform admin may override per tenant; the
// tenant only reads them.
package settings

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
)

// Settings are the tenant defaults.
type Settings struct {
	TenantID           uuid.UUID
	RunRetentionDays   int
	RatingTenant       bool
	RatingGlobal       bool
	DefaultKeep        time.Duration
	NotificationEmails []string
	UpdatedAt          time.Time
}

// Patch is a partial update; nil = keep.
type Patch struct {
	RunRetentionDays   *int
	RatingTenant       *bool
	RatingGlobal       *bool
	DefaultKeep        *time.Duration
	NotificationEmails []string
}

// Limits are the effective ceilings.
type Limits struct {
	MaxConcurrentRuns   int           `json:"max_concurrent_runs"`
	MaxMachinesPerRun   int           `json:"max_machines_per_run"`
	MaxSize             string        `json:"max_size"`
	MaxKeep             time.Duration `json:"max_keep"`
	RunRetentionMaxDays int           `json:"run_retention_max_days"`
	// Overridden reports a per-tenant override in effect.
	Overridden bool `json:"-"`
}

// PlatformLimits is the default ceiling of every tenant.
var PlatformLimits = Limits{
	MaxConcurrentRuns:   3,
	MaxMachinesPerRun:   12,
	MaxSize:             "XL",
	MaxKeep:             24 * time.Hour,
	RunRetentionMaxDays: 365,
}

// Repository is the storage port.
type Repository interface {
	Ensure(ctx context.Context, tenantID uuid.UUID) (Settings, *Limits, error)
	Update(ctx context.Context, tenantID uuid.UUID, p Patch) (Settings, error)
	SetLimitsOverride(ctx context.Context, tenantID uuid.UUID, l *Limits) error
}

// Access resolves the caller's role in a tenant.
type Access interface {
	RoleIn(ctx context.Context, actor auth.Actor, tenantID uuid.UUID) (slug, role string, err error)
}

// Defaults supplies the platform default limits (system settings).
type Defaults interface {
	DefaultLimits(ctx context.Context) Limits
}

// Service is the settings use cases.
type Service struct {
	repo     Repository
	access   Access
	defaults Defaults
}

// NewService builds the service.
func NewService(repo Repository, access Access) *Service { return &Service{repo: repo, access: access} }

// WithDefaults registers the system-settings source of default limits.
func (s *Service) WithDefaults(d Defaults) *Service {
	s.defaults = d
	return s
}

func (s *Service) platform(ctx context.Context) Limits {
	if s.defaults != nil {
		return s.defaults.DefaultLimits(ctx)
	}
	return PlatformLimits
}

// Get returns the settings (any member).
func (s *Service) Get(ctx context.Context, actor auth.Actor, tenantID uuid.UUID) (Settings, error) {
	if _, _, err := s.access.RoleIn(ctx, actor, tenantID); err != nil {
		return Settings{}, err
	}
	st, _, err := s.repo.Ensure(ctx, tenantID)
	return st, err
}

// Limits returns the effective limits (any member).
func (s *Service) Limits(ctx context.Context, actor auth.Actor, tenantID uuid.UUID) (Limits, error) {
	if _, _, err := s.access.RoleIn(ctx, actor, tenantID); err != nil {
		return Limits{}, err
	}
	return s.EffectiveLimits(ctx, tenantID)
}

// EffectiveLimits is the override when set, else the platform default —
// for the launch path, no access check.
func (s *Service) EffectiveLimits(ctx context.Context, tenantID uuid.UUID) (Limits, error) {
	_, override, err := s.repo.Ensure(ctx, tenantID)
	if err != nil {
		return Limits{}, err
	}
	if override != nil {
		l := *override
		l.Overridden = true
		return l, nil
	}
	return s.platform(ctx), nil
}

// Update applies a patch (admin+), bounded by the effective limits.
func (s *Service) Update(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, p Patch) (Settings, error) {
	_, role, err := s.access.RoleIn(ctx, actor, tenantID)
	if err != nil {
		return Settings{}, err
	}
	if role != "owner" && role != "admin" {
		return Settings{}, errs.Forbidden("requires role admin").Because("requires_role_admin")
	}
	limits, err := s.EffectiveLimits(ctx, tenantID)
	if err != nil {
		return Settings{}, err
	}
	if p.RunRetentionDays != nil && (*p.RunRetentionDays < 1 || *p.RunRetentionDays > limits.RunRetentionMaxDays) {
		return Settings{}, errs.Newf(errs.CodeInvalid, "run_retention_days must be 1..%d", limits.RunRetentionMaxDays).Because("retention_days_range", "max", strconv.Itoa(limits.RunRetentionMaxDays))
	}
	if p.DefaultKeep != nil && (*p.DefaultKeep < 0 || *p.DefaultKeep > limits.MaxKeep) {
		return Settings{}, errs.Newf(errs.CodeInvalid, "default_keep must be 0..%s", limits.MaxKeep).Because("default_keep_range", "max", limits.MaxKeep.String())
	}
	if p.NotificationEmails != nil {
		clean := make([]string, 0, len(p.NotificationEmails))
		for _, e := range p.NotificationEmails {
			e = strings.ToLower(strings.TrimSpace(e))
			if !strings.Contains(e, "@") {
				return Settings{}, errs.Invalid("notification_emails: "+e).Because("notification_email_invalid", "email", e)
			}
			clean = append(clean, e)
		}
		if len(clean) > 20 {
			return Settings{}, errs.Invalid("at most 20 notification emails").Because("at_most_20_notification_emails")
		}
		p.NotificationEmails = clean
	}
	return s.repo.Update(ctx, tenantID, p)
}
