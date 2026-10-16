// Package settings is the tenant's defaults. The tenant has no ceilings of
// its own: what a run may take is bounded by the cloud's quotas alone.
package settings

import (
	"context"
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

// Repository is the storage port.
type Repository interface {
	Ensure(ctx context.Context, tenantID uuid.UUID) (Settings, error)
	Update(ctx context.Context, tenantID uuid.UUID, p Patch) (Settings, error)
}

// Access resolves the caller's role in a tenant.
type Access interface {
	RoleIn(ctx context.Context, actor auth.Actor, tenantID uuid.UUID) (slug, role string, err error)
}

// Service is the settings use cases.
type Service struct {
	repo   Repository
	access Access
}

// NewService builds the service.
func NewService(repo Repository, access Access) *Service { return &Service{repo: repo, access: access} }

// Get returns the settings (any member).
func (s *Service) Get(ctx context.Context, actor auth.Actor, tenantID uuid.UUID) (Settings, error) {
	if _, _, err := s.access.RoleIn(ctx, actor, tenantID); err != nil {
		return Settings{}, err
	}
	return s.repo.Ensure(ctx, tenantID)
}

// Update applies a patch (admin+).
func (s *Service) Update(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, p Patch) (Settings, error) {
	_, role, err := s.access.RoleIn(ctx, actor, tenantID)
	if err != nil {
		return Settings{}, err
	}
	if role != "owner" && role != "admin" {
		return Settings{}, errs.Forbidden("requires role admin").Because("requires_role_admin")
	}
	if p.RunRetentionDays != nil && *p.RunRetentionDays < 1 {
		return Settings{}, errs.Invalid("run_retention_days must be at least 1").Because("retention_days_range")
	}
	if p.DefaultKeep != nil && *p.DefaultKeep < 0 {
		return Settings{}, errs.Invalid("default_keep must not be negative").Because("default_keep_range")
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
