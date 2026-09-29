// Package profile is the person behind an IAM subject as the product sees
// them: display data, preferences, notification switches. Identity itself
// (login, sessions, email ownership) is IAM's; the profile mirrors the email
// and is created lazily on the first authenticated call.
package profile

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
)

// Profile is the stored person.
type Profile struct {
	ID              uuid.UUID
	Email           string
	DisplayName     string
	Avatar          string
	IsPlatformAdmin bool
	Preferences     Preferences
	Notifications   Notifications
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Preferences are UI preferences.
type Preferences struct {
	Theme         string  `json:"theme,omitempty"`
	Timezone      string  `json:"timezone,omitempty"`
	DefaultTenant *string `json:"default_tenant,omitempty"`
}

// Notifications are the e-mail switches.
type Notifications struct {
	RunFinished   *bool `json:"run_finished,omitempty"`
	RunFailed     *bool `json:"run_failed,omitempty"`
	SuiteFinished *bool `json:"suite_finished,omitempty"`
}

// Patch is a partial update; nil = keep.
type Patch struct {
	DisplayName   *string
	Avatar        *string
	Preferences   *Preferences
	Notifications *Notifications
}

// Repository is the storage port.
type Repository interface {
	// Ensure creates the profile on first sight (created=true) and
	// refreshes the email when a non-empty one is given.
	Ensure(ctx context.Context, id uuid.UUID, email string) (p Profile, created bool, err error)
	Get(ctx context.Context, id uuid.UUID) (Profile, error)
	Update(ctx context.Context, p Profile) (Profile, error)
	SetEmail(ctx context.Context, id uuid.UUID, email string) error
	Delete(ctx context.Context, id uuid.UUID) error
	// Refs returns display data for the known ids; unknown ids are absent.
	// Email is filled only where the viewer may see it (see Viewer).
	Refs(ctx context.Context, viewer Viewer, ids []uuid.UUID) ([]Ref, error)
}

// Viewer scopes what Refs discloses. The email of a subject is returned
// only to the subject itself, to a co-member of at least one tenant (only
// Tenant when set — an API token confined to it) or when Reveal is set
// (platform admin); otherwise it is empty.
type Viewer struct {
	UserID uuid.UUID
	Tenant uuid.UUID
	Reveal bool
}

// Ref is the display data other resources show for a subject.
type Ref struct {
	ID          uuid.UUID
	Email       string
	DisplayName string
	Avatar      string
}

// Name is what the UI shows: the display name, else the email when the
// viewer may see it (Refs leaves it empty otherwise), else empty.
func (r Ref) Name() string {
	if r.DisplayName != "" {
		return r.DisplayName
	}
	return r.Email
}

// Service is the profile use cases.
type Service struct {
	repo Repository
}

// NewService builds the service.
func NewService(repo Repository) *Service { return &Service{repo: repo} }

// Ensure is called by the authenticator on every verified call.
func (s *Service) Ensure(ctx context.Context, id uuid.UUID, email string) (Profile, bool, error) {
	return s.repo.Ensure(ctx, id, normalizeEmail(email))
}

// normalizeEmail is the canonical form (invites and admin lists compare
// lowercase).
func normalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

// Seed fills display data on first sight (from IAM users/me).
func (s *Service) Seed(ctx context.Context, id uuid.UUID, email, displayName, avatar string) (Profile, error) {
	p, err := s.repo.Get(ctx, id)
	if err != nil {
		return Profile{}, err
	}
	if email != "" {
		p.Email = normalizeEmail(email)
	}
	if p.DisplayName == "" {
		p.DisplayName = displayName
	}
	if p.Avatar == "" {
		p.Avatar = avatar
	}
	if err := s.repo.SetEmail(ctx, id, p.Email); err != nil {
		return Profile{}, err
	}
	return s.repo.Update(ctx, p)
}

// Get returns one profile.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (Profile, error) {
	return s.repo.Get(ctx, id)
}

// Update applies a patch: read, merge, write the whole row.
func (s *Service) Update(ctx context.Context, id uuid.UUID, patch Patch) (Profile, error) {
	p, err := s.repo.Get(ctx, id)
	if err != nil {
		return Profile{}, err
	}
	if patch.DisplayName != nil {
		if len(*patch.DisplayName) > 64 {
			return Profile{}, errs.Invalid("display_name longer than 64")
		}
		p.DisplayName = *patch.DisplayName
	}
	if patch.Avatar != nil {
		p.Avatar = *patch.Avatar
	}
	if patch.Preferences != nil {
		if err := validateTheme(patch.Preferences.Theme); err != nil {
			return Profile{}, err
		}
		p.Preferences = *patch.Preferences
	}
	if patch.Notifications != nil {
		p.Notifications = *patch.Notifications
	}
	return s.repo.Update(ctx, p)
}

// Refs batch-loads display data by id (one query); unknown ids are absent
// from the map.
func (s *Service) Refs(ctx context.Context, viewer Viewer, ids []uuid.UUID) (map[uuid.UUID]Ref, error) {
	out := make(map[uuid.UUID]Ref, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	refs, err := s.repo.Refs(ctx, viewer, ids)
	if err != nil {
		return nil, err
	}
	for _, r := range refs {
		out[r.ID] = r
	}
	return out, nil
}

// EmailChanged mirrors the IAM webhook.
func (s *Service) EmailChanged(ctx context.Context, id uuid.UUID, email string) error {
	return s.repo.SetEmail(ctx, id, normalizeEmail(email))
}

// UserDeleted mirrors the IAM webhook.
func (s *Service) UserDeleted(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func validateTheme(theme string) error {
	switch theme {
	case "", "system", "light", "dark":
		return nil
	}
	return errs.Invalid(fmt.Sprintf("unknown theme %q", theme))
}

// EncodeJSON / DecodeJSON are the storage shape of the jsonb columns.
func EncodeJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("{}")
	}
	return b
}
