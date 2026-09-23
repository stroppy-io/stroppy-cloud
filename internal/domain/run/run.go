// Package run is a launched benchmark: the frozen snapshot of what was
// asked, the baked RunSpec handed to Graphene, and the projection of what
// Graphene reports back (status, phases, machines, containers, segments,
// result). The server never executes anything itself — it compiles,
// starts, watches and remembers.
package run

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/library"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/provider"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/settings"
)

// Status of a run (the server's vocabulary, §16.6).
type Status string

// Statuses.
const (
	StatusPending    Status = "pending"
	StatusRunning    Status = "running"
	StatusCancelling Status = "cancelling"
	StatusCompleted  Status = "completed"
	StatusFailed     Status = "failed"
	StatusCancelled  Status = "cancelled"
)

// Terminal reports a finished run.
func (s Status) Terminal() bool {
	return s == StatusCompleted || s == StatusFailed || s == StatusCancelled
}

// Phase of a run, for progress.
type Phase string

// Phases in order.
const (
	PhaseQueued       Phase = "queued"
	PhaseProvisioning Phase = "provisioning"
	PhaseDeploying    Phase = "deploying"
	PhaseWorkload     Phase = "workload"
	PhaseCollecting   Phase = "collecting"
	PhaseTeardown     Phase = "teardown"
	PhaseDone         Phase = "done"
)

// PhaseOrder is the progress scale.
var PhaseOrder = []Phase{PhaseQueued, PhaseProvisioning, PhaseDeploying, PhaseWorkload, PhaseCollecting, PhaseTeardown, PhaseDone}

// Trigger of a run.
type Trigger string

// Triggers.
const (
	TriggerManual   Trigger = "manual"
	TriggerSuite    Trigger = "suite"
	TriggerAPI      Trigger = "api"
	TriggerSchedule Trigger = "schedule"
)

// Snapshot is what the run was launched from — self-contained, so a
// deleted library entry or profile does not change history.
type Snapshot struct {
	Execution       json.RawMessage             `json:"execution,omitempty"`
	Database        library.DatabaseSpec        `json:"database"`
	DatabaseName    string                      `json:"database_name,omitempty"`
	Workload        library.WorkloadSpec        `json:"workload"`
	WorkloadName    string                      `json:"workload_name,omitempty"`
	Sizes           map[string]library.RoleSize `json:"sizes"`
	ProviderProfile Ref                         `json:"provider_profile"`
	ProviderKind    string                      `json:"provider_kind"`
	Keep            string                      `json:"keep,omitempty"`
	// EffectiveConfigs: role → config schema id → resolved value.
	EffectiveConfigs map[string]map[string]json.RawMessage `json:"effective_configs,omitempty"`
	Machines         []MachineSnapshot                     `json:"machines"`
	TopologyLabel    string                                `json:"topology_label,omitempty"`
}

// Ref is an id with a name.
type Ref struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name,omitempty"`
}

// MachineSnapshot is one planned machine.
type MachineSnapshot struct {
	Name         string `json:"name"`
	Role         string `json:"role"`
	Size         string `json:"size"`
	CPU          int    `json:"cpu"`
	MemoryGB     int    `json:"memory_gb"`
	DiskGB       int    `json:"disk_gb"`
	DiskType     string `json:"disk_type,omitempty"`
	InstanceType string `json:"instance_type,omitempty"`
	Location     string `json:"location,omitempty"`
}

// Summary is the denormalized header for lists and sorting.
type Summary struct {
	DBKind          string             `json:"db_kind,omitempty"`
	DBVersion       string             `json:"db_version,omitempty"`
	WorkloadName    string             `json:"workload_name,omitempty"`
	Protocol        string             `json:"protocol,omitempty"`
	StroppyVersion  string             `json:"stroppy_version,omitempty"`
	TopologyLabel   string             `json:"topology_label,omitempty"`
	NodeCount       int                `json:"node_count,omitempty"`
	ProviderKind    string             `json:"provider_kind,omitempty"`
	ProviderProfile *Ref               `json:"provider_profile,omitempty"`
	Sizes           map[string]string  `json:"sizes,omitempty"`
	ProgressPct     float64            `json:"progress_pct"`
	Segment         string             `json:"segment,omitempty"`
	Headline        map[string]float64 `json:"headline,omitempty"`
}

// Run is the record.
type Run struct {
	ID                uuid.UUID
	TenantID          uuid.UUID
	Name              string
	Status            Status
	Phase             Phase
	StatusReason      string
	Trigger           Trigger
	SuiteRunID        *uuid.UUID
	CellID            string
	ScheduleID        *uuid.UUID
	ParentRunID       *uuid.UUID
	TestID            *uuid.UUID
	TestName          string
	AuthorID          *uuid.UUID
	Snapshot          Snapshot
	RunSpec           json.RawMessage
	Summary           Summary
	Result            json.RawMessage
	State             State
	LastEventID       int64
	RatingTenant      bool
	RatingGlobal      bool
	Keep              time.Duration
	KeepUntil         *time.Time
	StandKept         bool
	Notes             string
	Labels            map[string]string
	GrapheneNamespace string
	// GrapheneRunID is the run id on the Graphene side when it differs
	// from ID (suite cells: "<suite_run_id>-<cell_id>").
	GrapheneRunID    string
	PipelineRevision string
	TPS              *float64
	DurationSeconds  *float64
	CreatedAt        time.Time
	StartedAt        *time.Time
	FinishedAt       *time.Time
	UpdatedAt        time.Time
	DeletedAt        *time.Time
}

// GrapheneRef is the run's ref in Graphene.
func (r Run) GrapheneRef() string { return "run/" + r.GrapheneID() }

// GrapheneID is the run id Graphene knows the run by.
func (r Run) GrapheneID() string {
	if r.GrapheneRunID != "" {
		return r.GrapheneRunID
	}
	return r.ID.String()
}

// Event is one timeline entry.
type Event struct {
	ID         int64
	RunID      uuid.UUID
	GrapheneID int64
	At         time.Time
	Kind       string
	Title      string
	Subject    string
	Status     string
	Error      string
	Attempt    int
	Payload    map[string]any
}

// ListQuery is the run list filter.
type ListQuery struct {
	Search         string
	AuthorID       string
	Statuses       []string
	Kinds          []string
	Profiles       []string
	Triggers       []string
	TestID         string
	SuiteRunID     string
	Standalone     bool
	Labels         map[string]string
	StartedAfter   time.Time
	StartedBefore  time.Time
	FinishedAfter  time.Time
	FinishedBefore time.Time
	DurationMin    time.Duration
	DurationMax    time.Duration
	StandKeptOnly  bool
	FavoritesOf    string
	Sort           string
	Desc           bool
	Limit          int
	Offset         int
}

// Facet is one filter field with its value counts.
type Facet struct {
	Field  string
	Values []FacetValue
}

// FacetValue is one value with its count.
type FacetValue struct {
	Value string
	Count int
}

// Live is a run the projection follows.
type Live struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	Namespace   string
	LastEventID int64
	Status      Status
}

// MetaPatch is the editable header; nil = keep.
type MetaPatch struct {
	Name         *string
	Notes        *string
	Labels       map[string]string
	RatingTenant *bool
	RatingGlobal *bool
}

// Repository is the storage port.
type Repository interface {
	Insert(ctx context.Context, r Run, idempotencyKey string) error
	ByID(ctx context.Context, id uuid.UUID) (Run, error)
	ByIdempotencyKey(ctx context.Context, tenantID uuid.UUID, key string) (uuid.UUID, bool, error)
	List(ctx context.Context, tenantID uuid.UUID, q ListQuery) ([]Run, error)
	Facets(ctx context.Context, tenantID uuid.UUID) ([]Facet, error)
	OfTest(ctx context.Context, testID uuid.UUID, limit, offset int) ([]Run, error)
	OfSuiteRun(ctx context.Context, suiteRunID uuid.UUID) ([]Run, error)
	LiveRuns(ctx context.Context) ([]Live, error)
	LiveCount(ctx context.Context, tenantID uuid.UUID) (int, error)
	HasLive(ctx context.Context, tenantID uuid.UUID) (bool, error)
	UpdateMeta(ctx context.Context, id uuid.UUID, p MetaPatch) error
	SetStatus(ctx context.Context, id uuid.UUID, status Status, phase Phase, reason string, startedAt, finishedAt *time.Time) error
	SetProjection(ctx context.Context, id uuid.UUID, state State, lastEventID int64) error
	SetResult(ctx context.Context, id uuid.UUID, result json.RawMessage, summary Summary, tps *float64) error
	SetKeep(ctx context.Context, id uuid.UUID, kept bool, until *time.Time) error
	SoftDelete(ctx context.Context, id uuid.UUID) error

	InsertEvent(ctx context.Context, e Event) (bool, error)
	EventsAfter(ctx context.Context, runID uuid.UUID, after int64, limit int) ([]Event, error)

	ByIDs(ctx context.Context, ids []uuid.UUID) ([]Run, error)
	Counts(ctx context.Context, tenantID uuid.UUID) (Counts, error)
	Rating(ctx context.Context, q RatingQuery) ([]RatingRow, error)
	Leagues(ctx context.Context, tenantID string) ([]string, error)
	SetFavorite(ctx context.Context, userID, tenantID uuid.UUID, kind string, targetID uuid.UUID, on bool) error
	FavoritesOf(ctx context.Context, userID, tenantID uuid.UUID, kind string) (map[uuid.UUID]bool, error)
}

// Graphene is the control-plane port of a run (namespace-scoped ctx).
type Graphene interface {
	StartRun(ctx context.Context, runID, pipeline string, params any, labels map[string]string) error
	CancelRun(ctx context.Context, runID string) error
	DeleteRef(ctx context.Context, ref string) error
	// Holdings are what the run handed to the pipeline's stand (keep).
	Holdings(ctx context.Context, runRef string) ([]Holding, error)
	// KeepExtend/KeepRelease address one holding of the stand.
	KeepExtend(ctx context.Context, held string, keep time.Duration) error
	KeepRelease(ctx context.Context, held string) error
	// RunClose is the run's own account of its end: the result (partial
	// when it failed) and the failure message.
	RunClose(ctx context.Context, runID string) (json.RawMessage, string, error)
	RunStatus(ctx context.Context, runID string) (string, error)
	// Events streams the run's Graphene events after an id; follow keeps
	// the stream open until the run finishes or ctx ends.
	Events(ctx context.Context, runID string, afterID int64, follow bool, fn func(RawEvent) error) error
	Tree(ctx context.Context, owner string) (TreeNode, error)
	// Node describes one live record with its subtree (false = gone).
	Node(ctx context.Context, ref string) (TreeNode, bool, error)
	// Artifacts describes artifact records by ref.
	Artifacts(ctx context.Context, refs []string) ([]Artifact, error)
	Download(ctx context.Context, ref string) (io.ReadCloser, error)
}

// Holding is one record the pipeline's stand keeps for a run.
type Holding struct {
	Ref string
	// KeepUntil is the deadline; nil = until an explicit release.
	KeepUntil *time.Time
}

// TreeNode is a record of the run with what it owns and what it talks
// to: the ownership tree is the topology's nodes, the flows its edges.
type TreeNode struct {
	Ref       string            `json:"ref"`
	Kind      string            `json:"kind"`
	Phase     string            `json:"phase,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
	Flows     []Flow            `json:"flows,omitempty"`
	KeepUntil *time.Time        `json:"keep_until,omitempty"`
	Children  []TreeNode        `json:"children"`
}

// Flow is one declared outgoing edge of a record — who it talks to and
// how. Declared intent, not observed traffic.
type Flow struct {
	// To is a record ref or an external endpoint.
	To       string `json:"to"`
	Protocol string `json:"protocol,omitempty"`
	Port     int    `json:"port,omitempty"`
	Label    string `json:"label,omitempty"`
	// Virtual marks a system edge (agent↔server), not one the run declared.
	Virtual bool `json:"virtual,omitempty"`
}

// Artifact is one downloadable record of a run.
type Artifact struct {
	Ref         string
	Name        string
	Kind        string
	ContentType string
	SizeBytes   int64
	Digest      string
	CreatedAt   *time.Time
}

// Access resolves the caller's role.
// ID is the ref without its kind prefix — what the API exposes, since a
// path segment cannot carry the slash.
func (a Artifact) ID() string {
	if i := strings.IndexByte(a.Ref, '/'); i >= 0 {
		return a.Ref[i+1:]
	}
	return a.Ref
}

type Access interface {
	RoleIn(ctx context.Context, actor auth.Actor, tenantID uuid.UUID) (slug, role string, err error)
	NamespaceOf(ctx context.Context, tenantID uuid.UUID) (string, error)
}

// Publisher emits tenant events (webhooks).
type Publisher interface {
	Publish(ctx context.Context, tenantID uuid.UUID, event string, payload any) error
}

// CompileRequest is what the compiler needs to turn a resolved test into a
// RunSpec.
type CompileRequest struct {
	Execution json.RawMessage
	RunID     uuid.UUID
	Tenant    string
	Namespace string
	Database  library.DatabaseSpec
	Derived   library.DatabaseDerived
	Workload  library.WorkloadSpec
	Sizes     map[string]library.RoleSize
	Profile   provider.Profile
	Keep      time.Duration
	Labels    map[string]string
}

// Compiled is the baked RunSpec with what the snapshot keeps of it.
type Compiled struct {
	Spec     json.RawMessage
	Machines []MachineSnapshot
}

// Compiler builds and bakes the RunSpec.
type Compiler interface {
	Compile(ctx context.Context, req CompileRequest) (Compiled, error)
}

// Profiles resolves provider profiles and their quotas.
type Profiles interface {
	ByID(ctx context.Context, tenantID, id uuid.UUID) (provider.Profile, error)
	FreshQuotas(ctx context.Context, p provider.Profile) ([]provider.Quota, error)
}

// Limits is the tenant's effective ceiling.
type Limits interface {
	EffectiveLimits(ctx context.Context, tenantID uuid.UUID) (settings.Limits, error)
}

// Counts is the status breakdown of a tenant's runs.
type Counts struct {
	Total, Pending, Running, Completed, Failed, Cancelled, KeptStands int
}

// RatingQuery selects opted-in completed runs ranked by a headline
// metric; TenantID empty = the global (public) rating.
type RatingQuery struct {
	Metric          string
	HigherIsBetter  bool
	TenantID        string
	Kinds           []string
	Versions        []string
	Providers       []string
	StroppyVersions []string
	League          string
	Since           time.Time
	Limit           int
	Offset          int
}

// RatingRow is one ranked run.
type RatingRow struct {
	Run              Run
	TenantName       string
	TenantPublicName string
	League           string
	Value            float64
}
