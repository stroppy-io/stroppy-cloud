// Package suite is the matrix launcher (§16.7): a Suite is tests × axes
// materialized into cells; a SuiteRun queues the enabled cells as ordinary
// Runs with suite_run_id / cell_id — the run queue starts at most the
// suite's concurrency of them at a time — and takes its status from them.
package suite

import (
	"context"
	"crypto/sha1" //nolint:gosec // cell ids, not security
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/library"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
)

// TestEntry is a suite's test: a library reference or an inline spec.
type TestEntry struct {
	Ref    *uuid.UUID        `json:"ref,omitempty"`
	Inline *library.TestSpec `json:"inline,omitempty"`
	// InlineID / InlineName identify an inline test in cells.
	InlineID   uuid.UUID `json:"inline_id,omitempty"`
	InlineName string    `json:"inline_name,omitempty"`
}

// ID of the entry in cells.
func (t TestEntry) ID() uuid.UUID {
	if t.Ref != nil {
		return *t.Ref
	}
	return t.InlineID
}

// WorkloadVariant overrides every segment of the workload.
type WorkloadVariant struct {
	Name        string         `json:"name"`
	VUs         int            `json:"vus,omitempty"`
	ScaleFactor int            `json:"scale_factor,omitempty"`
	Duration    string         `json:"duration,omitempty"`
	Params      map[string]any `json:"params,omitempty"`
}

// Axes are the variation dimensions.
type Axes struct {
	ProviderProfiles []uuid.UUID                   `json:"provider_profiles,omitempty"`
	Sizes            []map[string]library.RoleSize `json:"sizes,omitempty"`
	DatabaseVersions []string                      `json:"database_versions,omitempty"`
	WorkloadVariants []WorkloadVariant             `json:"workload_variants,omitempty"`
}

// Axis is the point of the axes a cell was generated from.
type Axis struct {
	ProviderProfileID *uuid.UUID                  `json:"provider_profile_id,omitempty"`
	Sizes             map[string]library.RoleSize `json:"sizes,omitempty"`
	DatabaseVersion   string                      `json:"database_version,omitempty"`
	WorkloadVariant   string                      `json:"workload_variant,omitempty"`
}

// Overrides are a cell's launch deviations (LaunchOverrides).
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

// Cell is one launchable combination.
type Cell struct {
	ID        string    `json:"id"`
	Name      string    `json:"name,omitempty"`
	TestID    uuid.UUID `json:"test_id"`
	TestName  string    `json:"test_name,omitempty"`
	Enabled   bool      `json:"enabled"`
	Axis      Axis      `json:"axis,omitzero"`
	Overrides Overrides `json:"overrides,omitzero"`
	Generated bool      `json:"generated"`
}

// Defaults apply to every cell unless overridden.
type Defaults struct {
	RatingTenant *bool          `json:"rating_tenant,omitempty"`
	RatingGlobal *bool          `json:"rating_global,omitempty"`
	Keep         *time.Duration `json:"keep,omitempty"`
}

// Suite is the stored definition; Cells are the manual edits over the
// generated set.
type Suite struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	Name        string
	Description string
	Tags        map[string]string
	AuthorID    *uuid.UUID
	Tests       []TestEntry
	Axes        Axes
	Cells       []Cell
	Concurrency int
	Defaults    Defaults
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Write is create/import input.
type Write struct {
	Name        string
	Description string
	Tags        map[string]string
	Tests       []TestEntry
	Axes        Axes
	Cells       []Cell
	Concurrency int
	Defaults    Defaults
}

// Patch is a partial update; nil = keep.
type Patch struct {
	Name        *string
	Description *string
	Tags        map[string]string
	Tests       []TestEntry
	SetTests    bool
	Axes        *Axes
	Cells       []Cell
	SetCells    bool
	Concurrency *int
	Defaults    *Defaults
}

// Computed is a suite with its final cell set and validation.
type Computed struct {
	Suite    Suite
	Cells    []Cell
	Fits     map[string]library.Fit
	Resolved map[string]library.Resolved
	Summary  Summary
}

// Summary is the header of a suite.
type Summary struct {
	CellCount        int
	EnabledCellCount int
	RunCount         int
	LastRun          *SuiteRun
	Schedules        []run.Ref
}

// Preview is the dry run of a definition.
type Preview struct {
	Cells      []Cell
	Fit        library.Fit
	Fits       map[string]library.Fit
	Totals     Totals
	QuotaCheck []QuotaCheck
}

// Totals across enabled cells.
type Totals struct {
	Machines, CPU, MemoryGB, DiskGB int
	EstimatedDuration               time.Duration
}

// QuotaCheck per provider profile.
type QuotaCheck struct {
	ProviderProfileID uuid.UUID
	Fits              bool
	Issues            []string
}

// CellRun is a cell inside a SuiteRun.
type CellRun struct {
	CellID string    `json:"cell_id"`
	Name   string    `json:"name,omitempty"`
	RunID  uuid.UUID `json:"run_id"`
}

// SuiteRun is one execution.
type SuiteRun struct {
	ID                uuid.UUID
	TenantID          uuid.UUID
	SuiteID           *uuid.UUID
	SuiteName         string
	Name              string
	Status            run.Status
	StatusReason      string
	Trigger           run.Trigger
	ScheduleID        *uuid.UUID
	RetryOf           *uuid.UUID
	Concurrency       int
	Cells             []CellRun
	Labels            map[string]string
	AuthorID          *uuid.UUID
	GrapheneNamespace string
	LastEventID       int64
	CreatedAt         time.Time
	StartedAt         *time.Time
	FinishedAt        *time.Time
	DurationSeconds   *float64
	UpdatedAt         time.Time
	DeletedAt         *time.Time
	// Runs are the child runs (filled on read).
	Runs []run.Run
}

// GrapheneRef of the suite run.
func (r SuiteRun) GrapheneRef() string { return "run/" + r.ID.String() }

// Progress counts the child runs.
type Progress struct {
	Total, Done, Failed, Running, Pending, Cancelled int
}

// Pct is the share of finished cells.
func (p Progress) Pct() float64 {
	if p.Total == 0 {
		return 0
	}
	return float64(p.Done+p.Failed+p.Cancelled) / float64(p.Total) * 100
}

// Progress of the child runs.
func (r SuiteRun) Progress() Progress {
	p := Progress{Total: len(r.Cells)}
	for _, x := range r.Runs {
		switch x.Status {
		case run.StatusCompleted:
			p.Done++
		case run.StatusFailed:
			p.Failed++
		case run.StatusCancelled:
			p.Cancelled++
		case run.StatusRunning, run.StatusCancelling:
			p.Running++
		case run.StatusPending:
			p.Pending++
		}
	}
	return p
}

// Launch is the launch-time input.
type Launch struct {
	Name         string
	Concurrency  int
	CellIDs      []string
	RatingTenant *bool
	RatingGlobal *bool
	Keep         *time.Duration
	Labels       map[string]string
	Trigger      run.Trigger
	ScheduleID   *uuid.UUID
	RetryOf      *uuid.UUID
}

// ListQuery filters suite runs.
type ListQuery struct {
	Statuses      []string
	Triggers      []string
	SuiteID       string
	StartedAfter  time.Time
	StartedBefore time.Time
	Sort          string
	Desc          bool
	Limit         int
	Offset        int
}

// Repository is the storage port.
type Repository interface {
	Insert(ctx context.Context, s Suite) error
	ByID(ctx context.Context, id uuid.UUID) (Suite, error)
	ByName(ctx context.Context, tenantID uuid.UUID, name string) (uuid.UUID, bool, error)
	List(ctx context.Context, tenantID uuid.UUID, q library.ListQuery, favoritesOf string) ([]Suite, error)
	Update(ctx context.Context, id uuid.UUID, p Patch) error
	Delete(ctx context.Context, id uuid.UUID) error
	UsingTest(ctx context.Context, testID uuid.UUID) ([]library.Usage, error)

	InsertRun(ctx context.Context, r SuiteRun, idempotencyKey string) error
	RunByID(ctx context.Context, id uuid.UUID) (SuiteRun, error)
	RunByIdempotencyKey(ctx context.Context, tenantID uuid.UUID, key string) (uuid.UUID, bool, error)
	Runs(ctx context.Context, tenantID uuid.UUID, q ListQuery) ([]SuiteRun, error)
	RunsOfSuite(ctx context.Context, suiteID uuid.UUID, limit, offset int) ([]SuiteRun, error)
	RunCountOfSuite(ctx context.Context, suiteID uuid.UUID) (int, error)
	LiveRuns(ctx context.Context) ([]run.Live, error)
	SetRunStatus(ctx context.Context, id uuid.UUID, status run.Status, reason string, startedAt, finishedAt *time.Time) error
	SetRunEvent(ctx context.Context, id uuid.UUID, lastEventID int64) error
	DeleteRun(ctx context.Context, id uuid.UUID) error
	SchedulesOf(ctx context.Context, kind string, id uuid.UUID) ([]run.Ref, error)
}

// Access resolves the caller.
type Access interface {
	RoleIn(ctx context.Context, actor auth.Actor, tenantID uuid.UUID) (slug, role string, err error)
	NamespaceOf(ctx context.Context, tenantID uuid.UUID) (string, error)
}

// --- cell generation --------------------------------------------------------

// Generate materializes tests × axes, then applies the manual cells: a
// manual cell with a generated id edits it (enabled, name, overrides), a
// manual cell with an unknown id is added.
func Generate(s Suite, names map[uuid.UUID]string) []Cell {
	var out []Cell
	pp := s.Axes.ProviderProfiles
	sizes := s.Axes.Sizes
	versions := s.Axes.DatabaseVersions
	variants := s.Axes.WorkloadVariants
	one := func(n int) int {
		if n == 0 {
			return 1
		}
		return n
	}
	for _, t := range s.Tests {
		tid := t.ID()
		tname := names[tid]
		if tname == "" {
			tname = t.InlineName
		}
		for i := 0; i < one(len(pp)); i++ {
			for j := 0; j < one(len(sizes)); j++ {
				for k := 0; k < one(len(versions)); k++ {
					for l := 0; l < one(len(variants)); l++ {
						axis := Axis{}
						parts := []string{short(tid.String())}
						if len(pp) > 0 {
							id := pp[i]
							axis.ProviderProfileID = &id
							parts = append(parts, short(id.String()))
						}
						if len(sizes) > 0 {
							axis.Sizes = sizes[j]
							parts = append(parts, sizesLabel(sizes[j]))
						}
						if len(versions) > 0 {
							axis.DatabaseVersion = versions[k]
							parts = append(parts, versions[k])
						}
						if len(variants) > 0 {
							axis.WorkloadVariant = variants[l].Name
							parts = append(parts, variants[l].Name)
						}
						out = append(out, Cell{ID: cellID(parts), Name: strings.Join(append([]string{tname}, parts[1:]...), " · "), TestID: tid, TestName: tname, Enabled: true, Axis: axis, Generated: true})
					}
				}
			}
		}
	}
	byID := map[string]int{}
	for i, c := range out {
		byID[c.ID] = i
	}
	for _, m := range s.Cells {
		if i, ok := byID[m.ID]; ok {
			out[i].Enabled = m.Enabled
			if m.Name != "" {
				out[i].Name = m.Name
			}
			out[i].Overrides = m.Overrides
			continue
		}
		m.Generated = false
		if m.TestName == "" {
			m.TestName = names[m.TestID]
		}
		if m.Name == "" {
			m.Name = m.TestName
		}
		out = append(out, m)
	}
	return out
}

func short(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

func sizesLabel(sizes map[string]library.RoleSize) string {
	keys := make([]string, 0, len(sizes))
	for k := range sizes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+sizes[k].Size)
	}
	return strings.Join(parts, ",")
}

// cellID is a stable, run-id-safe id: the test prefix plus a hash of the
// axis values (child run ids are "<suite_run_id>-<cell_id>").
func cellID(parts []string) string {
	sum := sha1.Sum([]byte(strings.Join(parts, "|"))) //nolint:gosec // not security
	return parts[0] + "-" + hex.EncodeToString(sum[:])[:8]
}

// --- spec application -------------------------------------------------------

// applyAxis turns a cell into the test spec to launch: the axis edits the
// spec (version → inline database, variant → inline workload), then the
// launch overrides.
func applyAxis(spec library.TestSpec, cell Cell, variants []WorkloadVariant, db *library.DatabaseSpec, wl *library.WorkloadSpec) (library.TestSpec, run.Overrides, error) {
	o := run.Overrides{
		Name: cell.Overrides.Name, ProviderProfileID: cell.Overrides.ProviderProfileID, Sizes: cell.Overrides.Sizes, Keep: cell.Overrides.Keep,
		RatingTenant: cell.Overrides.RatingTenant, RatingGlobal: cell.Overrides.RatingGlobal, Labels: cell.Overrides.Labels, Notes: cell.Overrides.Notes,
	}
	if cell.Axis.ProviderProfileID != nil && o.ProviderProfileID == nil {
		o.ProviderProfileID = cell.Axis.ProviderProfileID
	}
	if len(cell.Axis.Sizes) > 0 && len(o.Sizes) == 0 {
		o.Sizes = cell.Axis.Sizes
	}
	if cell.Axis.DatabaseVersion != "" {
		if db == nil {
			return spec, o, fmt.Errorf("cell %s: database_version needs a resolved database", cell.ID)
		}
		copyDB := *db
		copyDB.Version = cell.Axis.DatabaseVersion
		params := map[string]any{}
		_ = json.Unmarshal(copyDB.Params, &params) //nolint:errcheck // baked upstream
		if _, ok := params["version"]; ok {
			params["version"] = cell.Axis.DatabaseVersion
			copyDB.Params, _ = json.Marshal(params) //nolint:errcheck // map
		}
		spec.DatabaseRef, spec.DatabaseInline = nil, &copyDB
	}
	if cell.Axis.WorkloadVariant != "" {
		if wl == nil {
			return spec, o, fmt.Errorf("cell %s: workload_variant needs a resolved workload", cell.ID)
		}
		var v *WorkloadVariant
		for i := range variants {
			if variants[i].Name == cell.Axis.WorkloadVariant {
				v = &variants[i]
			}
		}
		if v == nil {
			return spec, o, fmt.Errorf("cell %s: unknown workload variant %q", cell.ID, cell.Axis.WorkloadVariant)
		}
		copyWL := *wl
		copyWL.Segments = make([]json.RawMessage, 0, len(wl.Segments))
		for _, raw := range wl.Segments {
			seg := map[string]any{}
			_ = json.Unmarshal(raw, &seg)                 //nolint:errcheck // baked upstream
			runPart, _ := seg["run"].(map[string]any)     //nolint:errcheck // absent = new
			wlPart, _ := seg["workload"].(map[string]any) //nolint:errcheck // absent = new
			if runPart == nil {
				runPart = map[string]any{}
			}
			if wlPart == nil {
				wlPart = map[string]any{}
			}
			if v.VUs > 0 {
				runPart["vus"] = v.VUs
			}
			if v.Duration != "" {
				runPart["duration"] = v.Duration
			}
			if v.ScaleFactor > 0 {
				wlPart["scale_factor"] = v.ScaleFactor
			}
			for k, x := range v.Params {
				wlPart[k] = x
			}
			seg["run"], seg["workload"] = runPart, wlPart
			b, _ := json.Marshal(seg) //nolint:errcheck // map
			copyWL.Segments = append(copyWL.Segments, b)
		}
		spec.WorkloadRef, spec.WorkloadInline = nil, &copyWL
	}
	return spec, o, nil
}
