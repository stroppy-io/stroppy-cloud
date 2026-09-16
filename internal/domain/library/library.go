// Package library is the tenant's reusable definitions: Database (a
// database shape), Workload (a stroppy load) and Test (database + workload
// + sizes + provider = something that can be launched). Definitions are
// stored as baked schemapb values; everything derived — topology preview,
// requirements, effective configs, fit — is computed on read from the
// catalog and the topology compiler, so a schema or catalog change shows
// up immediately as "needs attention" rather than as stale stored data.
package library

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/provider"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
)

// Entity is the common header of a definition.
type Entity struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	Name        string
	Description string
	Tags        map[string]string
	AuthorID    *uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// EntityWrite is the common header of a create/import.
type EntityWrite struct {
	Name        string
	Description string
	Tags        map[string]string
}

// EntityPatch is the common header of a patch; nil = keep.
type EntityPatch struct {
	Name        *string
	Description *string
	Tags        map[string]string
}

// DatabaseSpec is the shape of a database: kind, version, params baked
// through db.<kind>.params@1, per-role config deviations, external DSN.
type DatabaseSpec struct {
	Kind    catalog.DatabaseKind `json:"kind"`
	Version string               `json:"version"`
	Image   string               `json:"image,omitempty"`
	Params  json.RawMessage      `json:"params"`
	// Configs: role → config schema id → deviations from the schema defaults.
	Configs map[string]map[string]json.RawMessage `json:"configs,omitempty"`
	Runtime json.RawMessage                       `json:"runtime,omitempty"`
	// ExternalDSN is set for kind external only (write-only on the API).
	ExternalDSN string `json:"external_dsn,omitempty"`
}

// Database is a stored definition.
type Database struct {
	Entity
	Spec DatabaseSpec
}

// DatabaseDerived is what the definition means.
type DatabaseDerived struct {
	Plan             topology.Plan
	EffectiveConfigs map[string]map[string]json.RawMessage
	// Validation is the schema result of the params (nil = fits).
	Validation any
}

// WorkloadSpec is workload.stroppy@1: version, protocol, segments and the
// driver/connection/baseline options, stored as one baked value.
type WorkloadSpec struct {
	StroppyVersion string            `json:"stroppy_version"`
	Protocol       catalog.Protocol  `json:"protocol"`
	Segments       []json.RawMessage `json:"segments"`
	// Options is the rest of the value (driver, connection, baseline).
	Options json.RawMessage `json:"options,omitempty"`
}

// Workload is a stored definition.
type Workload struct {
	Entity
	Spec  WorkloadSpec
	Baked json.RawMessage // the full workload.stroppy@1 value
}

// WorkloadDerived is what the workload means.
type WorkloadDerived struct {
	Runner     topology.Requirement
	Segments   []SegmentSummary
	Validation any
}

// SegmentSummary is one segment at a glance.
type SegmentSummary struct {
	Name   string
	Script string
	VUs    int
	Limit  string
	Steps  []string
}

// RoleSize is the size choice of one role.
type RoleSize struct {
	Machine  json.RawMessage `json:"machine,omitempty"`
	Size     string          `json:"size"`
	DiskType string          `json:"disk_type,omitempty"`
	DiskGB   int             `json:"disk_gb,omitempty"`
}

// TestSpec is the launchable combination.
type TestSpec struct {
	Execution         json.RawMessage     `json:"execution,omitempty"`
	DatabaseRef       *uuid.UUID          `json:"database_ref,omitempty"`
	DatabaseInline    *DatabaseSpec       `json:"database_inline,omitempty"`
	WorkloadRef       *uuid.UUID          `json:"workload_ref,omitempty"`
	WorkloadInline    *WorkloadSpec       `json:"workload_inline,omitempty"`
	Sizes             map[string]RoleSize `json:"sizes,omitempty"`
	ProviderProfileID *uuid.UUID          `json:"provider_profile_id,omitempty"`
	Keep              time.Duration       `json:"keep,omitempty"`
	RatingTenant      bool                `json:"rating_tenant"`
	RatingGlobal      bool                `json:"rating_global"`
}

// TestStatus of a test.
type TestStatus string

// Statuses.
const (
	TestDraft          TestStatus = "draft"
	TestReady          TestStatus = "ready"
	TestNeedsAttention TestStatus = "needs_attention"
)

// Test is a stored definition.
type Test struct {
	Entity
	Spec        TestSpec
	Status      TestStatus
	ValidatedAt *time.Time
}

// Issue is one fit finding.
type Issue struct {
	Scope     string // input (default) | run_spec
	Path      string
	Code      string
	Severity  string // ERROR | WARNING
	Message   string
	Suggested any
}

// Fit is the outcome of validating a test.
type Fit struct {
	Fits          bool
	Issues        []Issue
	StaleDatabase bool
	StaleWorkload bool
}

// Resolved is the test with its references followed.
type Resolved struct {
	Database        *Database
	DatabaseDerived *DatabaseDerived
	Workload        *Workload
	WorkloadDerived *WorkloadDerived
	Profile         *provider.Profile
	// Requirements per role (database roles + runner).
	Requirements map[string]topology.Requirement
	// Machines is the estimate: role → size → hardware.
	Machines []Machine
}

// Machine is one estimated role.
type Machine struct {
	Role     string
	Count    int
	Size     string
	CPU      int
	MemoryGB int
	DiskGB   int
}

// Usage is who references a definition.
type Usage struct {
	Kind string
	ID   uuid.UUID
	Name string
}

// ListQuery is the common listing filter.
type ListQuery struct {
	Search    string
	Tags      map[string]string
	AuthorID  string
	Kinds     []string
	Statuses  []string
	Protocols []string
	Versions  []string
	Script    string
	Sort      string
	Desc      bool
	Limit     int
	Offset    int
}

// Repository is the storage port.
type Repository interface {
	InsertDatabase(ctx context.Context, d Database) error
	DatabaseByID(ctx context.Context, id uuid.UUID) (Database, error)
	Databases(ctx context.Context, tenantID uuid.UUID, q ListQuery) ([]Database, error)
	UpdateDatabase(ctx context.Context, id uuid.UUID, p EntityPatch, spec *DatabaseSpec) error
	DeleteDatabase(ctx context.Context, id uuid.UUID) error
	TestsUsingDatabase(ctx context.Context, id uuid.UUID) ([]Usage, error)
	InlineDatabase(ctx context.Context, id uuid.UUID, spec DatabaseSpec) error

	InsertWorkload(ctx context.Context, w Workload) error
	WorkloadByID(ctx context.Context, id uuid.UUID) (Workload, error)
	Workloads(ctx context.Context, tenantID uuid.UUID, q ListQuery) ([]Workload, error)
	UpdateWorkload(ctx context.Context, id uuid.UUID, p EntityPatch, spec *WorkloadSpec, baked json.RawMessage) error
	DeleteWorkload(ctx context.Context, id uuid.UUID) error
	TestsUsingWorkload(ctx context.Context, id uuid.UUID) ([]Usage, error)
	InlineWorkload(ctx context.Context, id uuid.UUID, spec WorkloadSpec) error

	InsertTest(ctx context.Context, t Test) error
	TestByID(ctx context.Context, id uuid.UUID) (Test, error)
	Tests(ctx context.Context, tenantID uuid.UUID, q ListQuery) ([]Test, error)
	UpdateTest(ctx context.Context, t Test, p TestPatch) error
	SetTestStatus(ctx context.Context, id uuid.UUID, status TestStatus, validatedAt *time.Time) error
	DeleteTest(ctx context.Context, id uuid.UUID) error
}

// TestPatch is a partial update of a test; nil = keep. Database/Workload
// switch reference↔inline together.
type TestPatch struct {
	Execution json.RawMessage
	EntityPatch
	SetDatabase       bool
	DatabaseRef       *uuid.UUID
	DatabaseInline    *DatabaseSpec
	SetWorkload       bool
	WorkloadRef       *uuid.UUID
	WorkloadInline    *WorkloadSpec
	Sizes             map[string]RoleSize
	SetProvider       bool
	ProviderProfileID *uuid.UUID
	Keep              *time.Duration
	RatingTenant      *bool
	RatingGlobal      *bool
}

// Validator bakes values.
type Validator interface {
	Bake(ctx context.Context, schemaID string, value json.RawMessage) (json.RawMessage, error)
}

// Access resolves the caller's role in a tenant.
type Access interface {
	RoleIn(ctx context.Context, actor auth.Actor, tenantID uuid.UUID) (slug, role string, err error)
}

// Profiles reads provider profiles for fit checks.
type Profiles interface {
	ByID(ctx context.Context, id uuid.UUID) (provider.Profile, error)
}

// Limits reads the tenant's effective limits.
type Limits interface {
	MaxKeep(ctx context.Context, tenantID uuid.UUID) (time.Duration, error)
}
