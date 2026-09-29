// Package schedule runs tests and suites on a cron (§16.7). The server
// fires the schedules itself; Graphene triggers are not used.
package schedule

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/library"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
)

// TargetKind is what a schedule launches.
type TargetKind string

// Target kinds.
const (
	TargetTest  TargetKind = "test"
	TargetSuite TargetKind = "suite"
)

// Overrides are applied to every launch (LaunchOverrides).
type Overrides struct {
	Name              string                      `json:"name,omitempty"`
	ProviderProfileID *uuid.UUID                  `json:"provider_profile_id,omitempty"`
	Sizes             map[string]library.RoleSize `json:"sizes,omitempty"`
	Keep              *time.Duration              `json:"keep,omitempty"`
	RatingTenant      *bool                       `json:"rating_tenant,omitempty"`
	RatingGlobal      *bool                       `json:"rating_global,omitempty"`
	Labels            map[string]string           `json:"labels,omitempty"`
	Notes             string                      `json:"notes,omitempty"`
}

// RunRef is what a firing produced.
type RunRef struct {
	Kind   string     `json:"kind"` // run | suite_run
	ID     *uuid.UUID `json:"id,omitempty"`
	Name   string     `json:"name,omitempty"`
	Status string     `json:"status"`
	At     time.Time  `json:"at"`
	Error  string     `json:"error,omitempty"`
}

// Schedule is the stored definition.
type Schedule struct {
	ID         uuid.UUID
	TenantID   uuid.UUID
	Name       string
	TargetKind TargetKind
	TargetID   uuid.UUID
	TargetName string
	Cron       string
	Timezone   string
	Enabled    bool
	Overrides  Overrides
	NextRunAt  *time.Time
	LastRun    *RunRef
	AuthorID   *uuid.UUID
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Write is create input.
type Write struct {
	Name       string
	TargetKind TargetKind
	TargetID   uuid.UUID
	Cron       string
	Timezone   string
	Enabled    *bool
	Overrides  Overrides
}

// Patch is a partial update.
type Patch struct {
	Name       *string
	TargetKind *TargetKind
	TargetID   *uuid.UUID
	Cron       *string
	Timezone   *string
	Enabled    *bool
	Overrides  *Overrides
}

// ListQuery filters schedules.
type ListQuery struct {
	TargetKind string
	Enabled    *bool
	Sort       string
	Desc       bool
	Limit      int
	Offset     int
}

// Repository is the storage port.
type Repository interface {
	Insert(ctx context.Context, s Schedule) error
	ByID(ctx context.Context, id uuid.UUID) (Schedule, error)
	List(ctx context.Context, tenantID uuid.UUID, q ListQuery) ([]Schedule, error)
	Due(ctx context.Context, now time.Time) ([]Schedule, error)
	Upcoming(ctx context.Context, tenantID uuid.UUID, limit int) ([]Schedule, error)
	Update(ctx context.Context, id uuid.UUID, p Patch, targetName string, nextRunAt *time.Time) error
	SetFired(ctx context.Context, id uuid.UUID, nextRunAt *time.Time, last RunRef) error
	Delete(ctx context.Context, id uuid.UUID) error
	AddHistory(ctx context.Context, id uuid.UUID, ref RunRef) error
	History(ctx context.Context, id uuid.UUID, limit, offset int) ([]RunRef, error)
}

// Access resolves the caller.
type Access interface {
	RoleIn(ctx context.Context, actor auth.Actor, tenantID uuid.UUID) (slug, role string, err error)
}

// Launcher starts the targets.
type Launcher interface {
	LaunchTest(ctx context.Context, actor auth.Actor, tenantID, testID uuid.UUID, o run.Overrides) (id uuid.UUID, name string, status string, err error)
	LaunchSuite(ctx context.Context, actor auth.Actor, tenantID, suiteID uuid.UUID, o run.Overrides) (id uuid.UUID, name string, status string, err error)
	TargetName(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, kind TargetKind, id uuid.UUID) (string, error)
}
