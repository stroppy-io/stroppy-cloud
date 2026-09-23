// Package compare is the ad-hoc comparison of runs (§16.8): metric rows
// against a baseline with verdicts, and a field-by-field diff of the
// specs (params, configs, workload, sizes).
package compare

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/library"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
)

// Request is what to compare.
type Request struct {
	RunIDs      []uuid.UUID
	BaselineID  *uuid.UUID
	MetricKeys  []string
	DeadbandPct float64
}

// Column is one compared run.
type Column struct {
	Run     run.Run
	Verdict VerdictCounts
}

// VerdictCounts summarizes a column against the baseline.
type VerdictCounts struct {
	Better, Worse, Same, Missing int
}

// Cell is one run's value of one metric.
type Cell struct {
	RunID   uuid.UUID
	Present bool
	Value   float64
	DiffPct *float64
	Verdict string // better|worse|same|baseline|missing
}

// MetricRow is one metric across the columns.
type MetricRow struct {
	Metric catalog.Metric
	Cells  []Cell
}

// Comparison is the result.
type Comparison struct {
	BaselineID uuid.UUID
	Columns    []Column
	Metrics    []MetricRow
	// SpecDiff: section (database, workload, sizes, configs.<role>) →
	// changes of each non-baseline run against the baseline, keyed by
	// run id.
	SpecDiff map[string]map[uuid.UUID][]library.Change
}

// Access resolves the caller.
type Access interface {
	RoleIn(ctx context.Context, actor auth.Actor, tenantID uuid.UUID) (slug, role string, err error)
}

// Service compares runs of one tenant.
type Service struct {
	runs    run.Repository
	access  Access
	catalog *catalog.Catalog
}

// NewService wires the use case.
func NewService(runs run.Repository, access Access, cat *catalog.Catalog) *Service {
	return &Service{runs: runs, access: access, catalog: cat}
}

// Compare loads the runs (all of the tenant) and compares them.
func (s *Service) Compare(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, req Request) (Comparison, error) {
	if _, _, err := s.access.RoleIn(ctx, actor, tenantID); err != nil {
		return Comparison{}, err
	}
	if len(req.RunIDs) < 2 || len(req.RunIDs) > 16 {
		return Comparison{}, errs.Invalid("run_ids: 2 to 16 runs")
	}
	list, err := s.runs.ByIDs(ctx, req.RunIDs)
	if err != nil {
		return Comparison{}, err
	}
	byID := map[uuid.UUID]run.Run{}
	for _, r := range list {
		if r.TenantID == tenantID {
			byID[r.ID] = r
		}
	}
	runs := make([]run.Run, 0, len(req.RunIDs))
	for _, id := range req.RunIDs {
		r, ok := byID[id]
		if !ok {
			return Comparison{}, errs.NotFound("run " + id.String())
		}
		runs = append(runs, r)
	}
	return Build(runs, req, s.catalog), nil
}

// Build compares already loaded runs; the first is the baseline unless
// the request names one.
func Build(runs []run.Run, req Request, cat *catalog.Catalog) Comparison {
	baseline := runs[0]
	if req.BaselineID != nil {
		for _, r := range runs {
			if r.ID == *req.BaselineID {
				baseline = r
			}
		}
	}
	deadband := req.DeadbandPct
	if deadband <= 0 {
		deadband = 2
	}
	out := Comparison{BaselineID: baseline.ID, SpecDiff: map[string]map[uuid.UUID][]library.Change{}}
	keys := req.MetricKeys
	if len(keys) == 0 {
		seen := map[string]bool{}
		for _, r := range runs {
			for k := range r.Summary.Headline {
				if !seen[k] {
					seen[k] = true
					keys = append(keys, k)
				}
			}
		}
		sort.Strings(keys)
	}
	defs := map[string]catalog.Metric{}
	if cat != nil {
		for _, m := range cat.MetricsFor("") {
			defs[m.Key] = m
		}
	}
	verdicts := map[uuid.UUID]*VerdictCounts{}
	for _, r := range runs {
		verdicts[r.ID] = &VerdictCounts{}
	}
	for _, k := range keys {
		def, ok := defs[k]
		if !ok {
			def = catalog.Metric{Key: k, Title: k, HigherIsBetter: !lowerIsBetter(k)}
		}
		row := MetricRow{Metric: def}
		base, baseOK := baseline.Summary.Headline[k]
		for _, r := range runs {
			v, ok := r.Summary.Headline[k]
			c := Cell{RunID: r.ID, Present: ok, Value: v}
			switch {
			case r.ID == baseline.ID:
				c.Verdict = "baseline"
				if !ok {
					c.Verdict = "missing"
				}
			case !ok || !baseOK:
				c.Verdict = "missing"
				verdicts[r.ID].Missing++
			default:
				c.Verdict = verdictOf(def.HigherIsBetter, v, base, deadband)
				if base != 0 {
					d := (v - base) / base * 100
					c.DiffPct = &d
				}
				switch c.Verdict {
				case "better":
					verdicts[r.ID].Better++
				case "worse":
					verdicts[r.ID].Worse++
				default:
					verdicts[r.ID].Same++
				}
			}
			row.Cells = append(row.Cells, c)
		}
		out.Metrics = append(out.Metrics, row)
	}
	for _, r := range runs {
		out.Columns = append(out.Columns, Column{Run: r, Verdict: *verdicts[r.ID]})
	}
	for section, of := range sections {
		diffs := map[uuid.UUID][]library.Change{}
		baseRaw := of(baseline)
		for _, r := range runs {
			if r.ID == baseline.ID {
				continue
			}
			changes, err := library.Diff(baseRaw, of(r))
			if err != nil {
				continue
			}
			diffs[r.ID] = changes
		}
		out.SpecDiff[section] = diffs
	}
	return out
}

// sections are the compared parts of the snapshot.
var sections = map[string]func(run.Run) json.RawMessage{
	"database": func(r run.Run) json.RawMessage {
		b, _ := json.Marshal(map[string]any{"kind": r.Snapshot.Database.Kind, "version": r.Snapshot.Database.Version, "image": r.Snapshot.Database.Image, "params": r.Snapshot.Database.Params}) //nolint:errcheck // struct
		return b
	},
	"workload": func(r run.Run) json.RawMessage {
		b, _ := json.Marshal(r.Snapshot.Workload) //nolint:errcheck // struct
		return b
	},
	"sizes": func(r run.Run) json.RawMessage {
		b, _ := json.Marshal(r.Snapshot.Sizes) //nolint:errcheck // struct
		return b
	},
	"configs": func(r run.Run) json.RawMessage {
		b, _ := json.Marshal(r.Snapshot.EffectiveConfigs) //nolint:errcheck // struct
		return b
	},
	"machines": func(r run.Run) json.RawMessage {
		b, _ := json.Marshal(r.Snapshot.Machines) //nolint:errcheck // struct
		return b
	},
}

func verdictOf(higherIsBetter bool, v, base, deadbandPct float64) string {
	if base == 0 {
		if v == base {
			return "same"
		}
		return "missing"
	}
	diff := (v - base) / base * 100
	if diff > -deadbandPct && diff < deadbandPct {
		return "same"
	}
	if (diff > 0) == higherIsBetter {
		return "better"
	}
	return "worse"
}

// lowerIsBetter reads a native metric key (optionally "<segment>.<key>"):
// failures and retries are better fewer; counts and totals are volume,
// better more (iteration_duration_count is how many iterations were
// measured, not a latency); latencies and durations are better lower.
func lowerIsBetter(key string) bool {
	if i := strings.LastIndexByte(key, '.'); i >= 0 {
		key = key[i+1:]
	}
	switch {
	case strings.Contains(key, "failed") || strings.Contains(key, "error") || strings.Contains(key, "retry"):
		return true
	case strings.HasSuffix(key, "_count") || strings.HasSuffix(key, "_total"):
		return false
	}
	return strings.Contains(key, "latency") || strings.Contains(key, "duration")
}

// Duration of a run, for the columns.
func Duration(r run.Run) time.Duration {
	if r.DurationSeconds == nil {
		return 0
	}
	return time.Duration(*r.DurationSeconds * float64(time.Second))
}
