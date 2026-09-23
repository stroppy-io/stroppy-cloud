// Package observe reads a run's logs and metrics from the installation's
// telemetry stores (§16.6): typed filters and raw LogsQL/PromQL, always
// inside the run's scope, over a window that defaults to the run (or one
// of its segments). The stores sit behind ports; an installation without
// them answers "unavailable", never an empty page.
package observe

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// LogQuery is the typed log filter.
type LogQuery struct {
	RunID      uuid.UUID
	Roles      []string
	Machines   []string
	Containers []string
	Streams    []string
	Phases     []string
	Levels     []string
	Segment    string
	Start, End time.Time
	Text       string
	Cursor     string
	Direction  string // older | newer
	Limit      int
	// Resolved by the service from the run: where its telemetry is and
	// which agents/entities the role, machine and container filters mean.
	Scope    Scope
	Agents   []string
	Entities []string
}

// LogLine is one entry.
type LogLine struct {
	Time      time.Time
	Seq       int
	Message   string
	Level     string
	Stream    string
	Role      string
	Machine   string
	Container string
	Phase     string
	Segment   string
	Fields    map[string]string
}

// LogPage is a page with cursors both ways.
type LogPage struct {
	Lines     []LogLine
	Older     string
	Newer     string
	Truncated bool
}

// MetricQuery selects catalog metrics over a window.
type MetricQuery struct {
	Scope      Scope
	Start, End time.Time
	Step       time.Duration
	Metrics    []catalog.Metric
}

// Series is one metric line.
type Series struct {
	Key, Title, Unit, Role, Machine string
	Points                          [][2]float64
	Avg, Min, Max, Last, P95        float64
}

// Logs is the log store port.
type Logs interface {
	Query(ctx context.Context, q LogQuery) (LogPage, error)
	Raw(ctx context.Context, scope Scope, query string, start, end time.Time, limit int) (LogPage, error)
	// Facets counts the values of the stream, level, agent and entity fields.
	Facets(ctx context.Context, scope Scope, start, end time.Time) ([]run.Facet, error)
}

// Metrics is the metric store port.
type Metrics interface {
	Query(ctx context.Context, q MetricQuery) ([]Series, []error, error)
	Raw(ctx context.Context, scope Scope, query string, start, end time.Time, step time.Duration) (json.RawMessage, error)
}

// Runs resolves runs with access.
type Runs interface {
	Get(ctx context.Context, actor auth.Actor, tenantID, id uuid.UUID) (run.Run, error)
}

// Window is the resolved time range.
type Window struct {
	Start, End time.Time
	Segment    string
}

// Service is the use cases.
type Service struct {
	runs    Runs
	logs    Logs
	metrics Metrics
	catalog *catalog.Catalog
}

// NewService wires the use cases; nil stores answer unavailable.
func NewService(runs Runs, logs Logs, metrics Metrics, cat *catalog.Catalog) *Service {
	return &Service{runs: runs, logs: logs, metrics: metrics, catalog: cat}
}

// WindowOf resolves the window: explicit bounds, else the segment, else
// the run's own span (open runs end now).
func WindowOf(r run.Run, segment string, start, end time.Time) (Window, error) {
	w := Window{Segment: segment}
	if segment != "" {
		found := false
		for _, s := range r.State.Segments {
			if s.Name == segment {
				found = true
				if s.StartedAt != nil {
					w.Start = *s.StartedAt
				}
				if s.FinishedAt != nil {
					w.End = *s.FinishedAt
				}
			}
		}
		if !found {
			return Window{}, errs.NotFound("segment " + segment)
		}
	}
	if w.Start.IsZero() {
		switch {
		case r.StartedAt != nil:
			w.Start = *r.StartedAt
		default:
			w.Start = r.CreatedAt
		}
	}
	if w.End.IsZero() {
		switch {
		case r.FinishedAt != nil:
			w.End = *r.FinishedAt
		default:
			w.End = time.Now().UTC()
		}
	}
	if !start.IsZero() {
		w.Start = start
	}
	if !end.IsZero() {
		w.End = end
	}
	if !w.End.After(w.Start) {
		w.End = w.Start.Add(time.Minute)
	}
	return w, nil
}

// Logs answers the typed filter.
func (s *Service) Logs(ctx context.Context, actor auth.Actor, tenantID uuid.UUID, q LogQuery) (LogPage, Window, error) {
	if s.logs == nil {
		return LogPage{}, Window{}, errs.New(errs.CodeUnavailable, "logs store is not connected")
	}
	r, err := s.runs.Get(ctx, actor, tenantID, q.RunID)
	if err != nil {
		return LogPage{}, Window{}, err
	}
	start, end := q.Start, q.End
	if len(q.Phases) > 0 && start.IsZero() && end.IsZero() {
		start, end = phaseSpan(r, q.Phases)
	}
	w, err := WindowOf(r, q.Segment, start, end)
	if err != nil {
		return LogPage{}, Window{}, err
	}
	q.Start, q.End = w.Start, w.End
	if q.Limit <= 0 || q.Limit > 1000 {
		q.Limit = 200
	}
	id := identitiesOf(r)
	q.Scope, q.Agents, q.Entities = ScopeOf(r), id.agents(q.Roles, q.Machines), id.entities(q.Containers)
	page, err := s.logs.Query(ctx, q)
	for i := range page.Lines {
		id.enrich(&page.Lines[i])
	}
	return page, w, err
}

// phaseSpan is the time the phases named took (telemetry carries no
// phase: a phase is a window).
func phaseSpan(r run.Run, phases []string) (start, end time.Time) {
	for _, p := range r.State.Phases {
		for _, want := range phases {
			if string(p.ID) != want || p.StartedAt == nil {
				continue
			}
			if start.IsZero() || p.StartedAt.Before(start) {
				start = *p.StartedAt
			}
			if p.FinishedAt != nil && p.FinishedAt.After(end) {
				end = *p.FinishedAt
			}
		}
	}
	return start, end
}

// RawLogs runs a LogsQL fragment inside the run scope.
func (s *Service) RawLogs(ctx context.Context, actor auth.Actor, tenantID, runID uuid.UUID, query string, start, end time.Time, limit int) (LogPage, error) {
	if s.logs == nil {
		return LogPage{}, errs.New(errs.CodeUnavailable, "logs store is not connected")
	}
	if strings.TrimSpace(query) == "" {
		return LogPage{}, errs.Invalid("query is required")
	}
	r, err := s.runs.Get(ctx, actor, tenantID, runID)
	if err != nil {
		return LogPage{}, err
	}
	w, err := WindowOf(r, "", start, end)
	if err != nil {
		return LogPage{}, err
	}
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	page, err := s.logs.Raw(ctx, ScopeOf(r), query, w.Start, w.End, limit)
	id := identitiesOf(r)
	for i := range page.Lines {
		id.enrich(&page.Lines[i])
	}
	return page, err
}

// LogFacets are the filter values seen in the run's logs.
func (s *Service) LogFacets(ctx context.Context, actor auth.Actor, tenantID, runID uuid.UUID) ([]run.Facet, error) {
	if s.logs == nil {
		return nil, errs.New(errs.CodeUnavailable, "logs store is not connected")
	}
	r, err := s.runs.Get(ctx, actor, tenantID, runID)
	if err != nil {
		return nil, err
	}
	w, err := WindowOf(r, "", time.Time{}, time.Time{})
	if err != nil {
		return nil, err
	}
	raw, err := s.logs.Facets(ctx, ScopeOf(r), w.Start, w.End)
	if err != nil {
		return nil, err
	}
	return identitiesOf(r).facets(raw), nil
}

// Metrics answers catalog keys over a window.
func (s *Service) Metrics(ctx context.Context, actor auth.Actor, tenantID, runID uuid.UUID, keys []string, segment string, start, end time.Time, step time.Duration) ([]Series, []error, Window, error) {
	if s.metrics == nil {
		return nil, nil, Window{}, errs.New(errs.CodeUnavailable, "metrics store is not connected")
	}
	r, err := s.runs.Get(ctx, actor, tenantID, runID)
	if err != nil {
		return nil, nil, Window{}, err
	}
	w, err := WindowOf(r, segment, start, end)
	if err != nil {
		return nil, nil, Window{}, err
	}
	defs := s.catalog.MetricsFor(catalog.DatabaseKind(r.Summary.DBKind))
	if len(keys) > 0 {
		want := map[string]bool{}
		for _, k := range keys {
			want[k] = true
		}
		filtered := defs[:0:0]
		for _, d := range defs {
			if want[d.Key] {
				filtered = append(filtered, d)
			}
		}
		if len(filtered) == 0 {
			return nil, nil, w, errs.Invalid("no known metric keys among " + strings.Join(keys, ","))
		}
		defs = filtered
	}
	// A number of the result only has no series to draw.
	drawn := defs[:0:0]
	for _, d := range defs {
		if d.Expr != "" {
			drawn = append(drawn, d)
		}
	}
	defs = drawn
	if step <= 0 {
		step = stepFor(w.End.Sub(w.Start))
	}
	series, perKey, err := s.metrics.Query(ctx, MetricQuery{Scope: ScopeOf(r), Start: w.Start, End: w.End, Step: step, Metrics: defs})
	id := identitiesOf(r)
	for i := range series {
		id.series(&series[i])
	}
	return series, perKey, w, err
}

// RawMetrics runs PromQL inside the run scope (the label matcher is
// injected by the store).
func (s *Service) RawMetrics(ctx context.Context, actor auth.Actor, tenantID, runID uuid.UUID, query string, start, end time.Time, step time.Duration) (json.RawMessage, error) {
	if s.metrics == nil {
		return nil, errs.New(errs.CodeUnavailable, "metrics store is not connected")
	}
	if strings.TrimSpace(query) == "" {
		return nil, errs.Invalid("query is required")
	}
	r, err := s.runs.Get(ctx, actor, tenantID, runID)
	if err != nil {
		return nil, err
	}
	if !end.After(start) {
		return nil, errs.Invalid("end must be after start")
	}
	if step <= 0 {
		step = stepFor(end.Sub(start))
	}
	return s.metrics.Raw(ctx, ScopeOf(r), query, start, end, step)
}

// stepFor picks a step giving ~300 points.
func stepFor(span time.Duration) time.Duration {
	step := span / 300
	switch {
	case step < 5*time.Second:
		return 5 * time.Second
	case step < time.Minute:
		return step.Round(5 * time.Second)
	default:
		return step.Round(time.Minute)
	}
}

// Matchers are the label matchers of the scope: Graphene's attributes
// (the scraped component series) and Stroppy's own spelling (native
// workload series).
func (s Scope) Matchers() (component, native string) {
	component = fmt.Sprintf(`%q=%q,%q=%q`, spec.AttrNamespace, s.Namespace, spec.AttrRun, s.Run)
	native = fmt.Sprintf(`%s=%q,%s=%q`, spec.LabelNativeNamespace, s.Namespace, spec.LabelNativeRun, s.Run)
	return component, native
}

// KeyExpr is the MetricsQL of a catalog metric inside the run scope:
// `$run` is the component matcher, `$native` Stroppy's.
func KeyExpr(m catalog.Metric, scope Scope) string {
	component, native := scope.Matchers()
	return strings.NewReplacer("$run", component, "$native", native).Replace(m.Expr)
}
