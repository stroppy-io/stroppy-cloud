package observe

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// Every metric a run stored, not only the catalog's: the explorer lists the
// names and draws any one of them by its own labels.

// MetricOrigin is where a metric comes from.
type MetricOrigin string

// Origins (the contract's MetricOrigin mirrors them).
const (
	// OriginExporter: a container's own Prometheus endpoint.
	OriginExporter MetricOrigin = "exporter"
	// OriginWorkload: stroppy.
	OriginWorkload MetricOrigin = "workload"
	// OriginContainer: the container statistics the agent samples.
	OriginContainer MetricOrigin = "container"
	// OriginPipeline: the pipeline's own telemetry.
	OriginPipeline MetricOrigin = "pipeline"
)

// MetricOrigins lists every origin.
var MetricOrigins = []MetricOrigin{OriginExporter, OriginWorkload, OriginContainer, OriginPipeline}

// MetricKind is what a value means over time.
type MetricKind string

// Kinds (the contract's MetricKind mirrors them).
const (
	KindCounter MetricKind = "counter"
	KindGauge   MetricKind = "gauge"
)

// MetricKinds lists every kind.
var MetricKinds = []MetricKind{KindCounter, KindGauge}

// MetricName is one stored name and where its series are.
type MetricName struct {
	Name                        string
	Origin                      MetricOrigin
	Kind                        MetricKind
	Series                      int
	Machines, Roles, Containers []string
}

// SeriesQuery draws one name.
type SeriesQuery struct {
	Name string
	// Fn: "value" or "rate"; empty picks rate for counters.
	Fn string
	// By groups the series; "machine" and "container" group by origin.
	By []string
	// Agg combines grouped series: sum (default), avg, max, min.
	Agg                         string
	Roles, Machines, Containers []string
	Segment                     string
	Start, End                  time.Time
	Step                        time.Duration
}

// LabeledSeries is one drawn series with the metric's own labels.
type LabeledSeries struct {
	Labels                   map[string]string
	Machine, Role, Container string
	Points                   [][2]float64
	Avg, Min, Max, Last, P95 float64
}

// SeriesResult is a drawn name.
type SeriesResult struct {
	Window    Window
	Name, Fn  string
	Labels    []string
	Total     int
	Truncated bool
	Series    []LabeledSeries
}

// attrRole is the pipeline's role of a series: machine (scraped) or
// workload (stroppy's own).
const attrRole = "graphene.role"

// maxSeries bounds a drawn name: the highest averages win.
const maxSeries = 100

// Labels the pipeline and the OTLP SDK stamp on every series; the
// explorer shows the metric's own labels only.
var internalLabel = regexp.MustCompile(`^(__name__|graphene\..*|scope\..*|service\..*|telemetry\..*|stroppy\.run\.id|stroppy_run_id|stroppy\.tenant|stroppy_tenant|job|instance)$`)

// A label the explorer may group by: a plain Prometheus or OTLP name.
var labelName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.]{0,127}$`)

// KindOf guesses the kind from the name alone: Prometheus counters end in
// _total, histogram and summary parts in _count/_sum/_bucket. Exporters
// also keep counters without the suffix (pg_stat_database_xact_commit),
// so the stored data decides the rest (kindsOf).
func KindOf(name string) MetricKind {
	for _, suffix := range []string{"_total", "_count", "_sum", "_bucket", "_created", ".total", ".count"} {
		if strings.HasSuffix(name, suffix) {
			return KindCounter
		}
	}
	return KindGauge
}

// MetricNames lists every metric of the run over the window.
func (s *Service) MetricNames(ctx context.Context, actor auth.Actor, tenantID, runID uuid.UUID, segment string, start, end time.Time) ([]MetricName, Window, error) {
	if s.metrics == nil {
		return nil, Window{}, errs.New(errs.CodeUnavailable, "metrics store is not connected").Because("metrics_store_is_not_connected")
	}
	r, err := s.runs.Get(ctx, actor, tenantID, runID)
	if err != nil {
		return nil, Window{}, err
	}
	w, err := WindowOf(r, segment, start, end)
	if err != nil {
		return nil, Window{}, err
	}
	return s.namesOf(ctx, r, w)
}

// SharedMetricNames lists the metrics of a shared run.
func (s *Service) SharedMetricNames(ctx context.Context, shares SharedRuns, token string, start, end time.Time) ([]MetricName, Window, error) {
	r, w, err := s.sharedWindow(ctx, shares, token, start, end)
	if err != nil {
		return nil, Window{}, err
	}
	names, w, err := s.namesOf(ctx, r, w)
	return publicNames(names), w, err
}

// publicNames is what a public share shows: the run's own telemetry
// (database exporters, containers, the workload), never Graphene's
// internals (its activities, the agent's calls to the docker socket).
func publicNames(names []MetricName) []MetricName {
	out := make([]MetricName, 0, len(names))
	for _, n := range names {
		if n.Origin != OriginPipeline {
			out = append(out, n)
		}
	}
	return out
}

// MetricSeries draws one name of the run.
func (s *Service) MetricSeries(ctx context.Context, actor auth.Actor, tenantID, runID uuid.UUID, q SeriesQuery) (SeriesResult, error) {
	if s.metrics == nil {
		return SeriesResult{}, errs.New(errs.CodeUnavailable, "metrics store is not connected").Because("metrics_store_is_not_connected")
	}
	r, err := s.runs.Get(ctx, actor, tenantID, runID)
	if err != nil {
		return SeriesResult{}, err
	}
	w, err := WindowOf(r, q.Segment, q.Start, q.End)
	if err != nil {
		return SeriesResult{}, err
	}
	return s.seriesOf(ctx, r, w, q)
}

// SharedMetricSeries draws one name of a shared run.
func (s *Service) SharedMetricSeries(ctx context.Context, shares SharedRuns, token string, q SeriesQuery) (SeriesResult, error) {
	r, w, err := s.sharedWindow(ctx, shares, token, q.Start, q.End)
	if err != nil {
		return SeriesResult{}, err
	}
	names, _, err := s.namesOf(ctx, r, w)
	if err != nil {
		return SeriesResult{}, err
	}
	if !slices.ContainsFunc(publicNames(names), func(n MetricName) bool { return n.Name == strings.TrimSpace(q.Name) }) {
		return SeriesResult{}, errs.NotFound("metric")
	}
	return s.seriesOf(ctx, r, w, q)
}

// sharedWindow is the share's run and the asked window clipped to it.
func (s *Service) sharedWindow(ctx context.Context, shares SharedRuns, token string, start, end time.Time) (run.Run, Window, error) {
	if s.metrics == nil {
		return run.Run{}, Window{}, errs.New(errs.CodeUnavailable, "metrics store is not connected").Because("metrics_store_is_not_connected")
	}
	r, err := shares.MetricsRun(ctx, token)
	if err != nil {
		return run.Run{}, Window{}, err
	}
	bounds, err := WindowOf(r, "", time.Time{}, time.Time{})
	if err != nil {
		return run.Run{}, Window{}, err
	}
	if start.IsZero() || start.Before(bounds.Start) {
		start = bounds.Start
	}
	if end.IsZero() || end.After(bounds.End) {
		end = bounds.End
	}
	if !end.After(start) {
		return run.Run{}, Window{}, errs.Invalid("window does not overlap the shared run").Because("window_does_not_overlap_the_shared_run")
	}
	return r, Window{Start: start, End: end}, nil
}

// kindsOf tells counters by their data over the window: a series that never
// went down and moved more than twice is counting. The store keeps no
// metric type, and exporters name counters without _total.
func (s *Service) kindsOf(ctx context.Context, r run.Run, w Window, selector string) map[string]MetricKind {
	span := int(w.End.Sub(w.Start).Seconds())
	if span < 60 {
		span = 60
	}
	at, step := w.End.Add(-time.Second), w.End.Sub(w.Start)
	read := func(fn string) map[string]float64 {
		raw, err := s.metrics.Raw(ctx, ScopeOf(r), fmt.Sprintf("max by (__name__) (%s(%s[%ds]) keep_metric_names)", fn, selector, span), at, w.End, step)
		if err != nil {
			return nil
		}
		rows, err := promMatrix(raw)
		if err != nil {
			return nil
		}
		out := map[string]float64{}
		for _, row := range rows {
			if len(row.points) > 0 {
				out[row.labels["__name__"]] = row.points[len(row.points)-1][1]
			}
		}
		return out
	}
	resets, changes := read("resets"), read("changes")
	kinds := map[string]MetricKind{}
	for name, moved := range changes {
		if KindOf(name) == KindCounter || (moved > 2 && resets[name] == 0) {
			kinds[name] = KindCounter
		} else {
			kinds[name] = KindGauge
		}
	}
	return kinds
}

func (s *Service) namesOf(ctx context.Context, r run.Run, w Window) ([]MetricName, Window, error) {
	// A range query at a coarse step: every name present at some point of
	// the window shows up (an instant query sees only one moment).
	step := max(w.End.Sub(w.Start)/20, time.Minute)
	query := fmt.Sprintf(`count by (__name__, %q, %q, %q, %q) ({__name__=~".+"})`, spec.AttrAgent, spec.AttrEntity, attrRole, "service.name")
	raw, err := s.metrics.Raw(ctx, ScopeOf(r), query, w.Start, w.End, step)
	if err != nil {
		return nil, w, err
	}
	groups, err := promMatrix(raw)
	if err != nil {
		return nil, w, errs.Wrap(errs.CodeUnavailable, "metric names", err).Because("metrics_snapshot_invalid")
	}
	kinds := s.kindsOf(ctx, r, w, `{__name__=~".+"}`)
	id := identitiesOf(r)
	type acc struct {
		MetricName
		machines, roles, containers map[string]bool
	}
	byName := map[string]*acc{}
	for _, g := range groups {
		name := g.labels["__name__"]
		if name == "" {
			continue
		}
		a, ok := byName[name]
		if !ok {
			kind, known := kinds[name]
			if !known {
				kind = KindOf(name)
			}
			a = &acc{MetricName: MetricName{Name: name, Kind: kind, Origin: originOf(name, g.labels)}, machines: map[string]bool{}, roles: map[string]bool{}, containers: map[string]bool{}}
			byName[name] = a
		}
		count := 0
		for _, p := range g.points {
			count = max(count, int(p[1]))
		}
		a.Series += count
		machine := id.machineOf[g.labels[spec.AttrAgent]]
		if c, ok := id.containerOf[g.labels[spec.AttrEntity]]; ok {
			a.containers[c] = true
			if machine == "" {
				machine = id.hostOf[c]
			}
		}
		if machine != "" {
			a.machines[machine] = true
			if role := id.roleOf[machine]; role != "" {
				a.roles[role] = true
			}
		}
	}
	out := make([]MetricName, 0, len(byName))
	for _, a := range byName {
		a.Machines, a.Roles, a.Containers = keysOf(a.machines), keysOf(a.roles), keysOf(a.containers)
		out = append(out, a.MetricName)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, w, nil
}

// originOf tells where a name comes from by the labels its series carry.
func originOf(name string, labels map[string]string) MetricOrigin {
	switch {
	case labels["service.name"] == "stroppy" || labels[attrRole] == "workload":
		return OriginWorkload
	// The agent's own instruments (its activities, its HTTP and RPC
	// clients) carry the container entity they act on: the name tells them
	// apart from what the container's exporter served.
	case strings.HasPrefix(name, "graphene.") || strings.HasPrefix(name, "http.") || strings.HasPrefix(name, "rpc."):
		return OriginPipeline
	case strings.HasPrefix(name, "docker.container."):
		return OriginContainer
	case strings.HasPrefix(labels[spec.AttrEntity], "docker/"):
		return OriginExporter
	default:
		return OriginPipeline
	}
}

func (s *Service) seriesOf(ctx context.Context, r run.Run, w Window, q SeriesQuery) (SeriesResult, error) {
	name := strings.TrimSpace(q.Name)
	if name == "" {
		return SeriesResult{}, errs.Invalid("name is required").Because("metric_name_required")
	}
	fn := q.Fn
	if fn == "" {
		fn = "value"
		kind, known := s.kindsOf(ctx, r, w, fmt.Sprintf("{__name__=%q}", name))[name]
		if !known {
			kind = KindOf(name)
		}
		if kind == KindCounter {
			fn = "rate"
		}
	}
	if fn != "value" && fn != "rate" {
		return SeriesResult{}, errs.Invalid("fn is value or rate").Because("metric_fn_unknown", "fn", fn)
	}
	agg := q.Agg
	if agg == "" {
		agg = "sum"
	}
	if !slices.Contains([]string{"sum", "avg", "max", "min"}, agg) {
		return SeriesResult{}, errs.Invalid("agg is sum, avg, max or min").Because("metric_agg_unknown", "agg", agg)
	}
	id := identitiesOf(r)
	matchers := []string{fmt.Sprintf("__name__=%q", name)}
	if agents := id.agents(q.Roles, q.Machines); agents != nil {
		matchers = append(matchers, fmt.Sprintf("%q=~%q", spec.AttrAgent, anyOf(agents)))
	}
	if entities := id.entities(q.Containers); entities != nil {
		matchers = append(matchers, fmt.Sprintf("%q=~%q", spec.AttrEntity, anyOf(entities)))
	}
	step := q.Step
	if step <= 0 {
		step = stepFor(w.End.Sub(w.Start))
	}
	expr := "{" + strings.Join(matchers, ",") + "}"
	if fn == "rate" {
		expr = fmt.Sprintf("rate(%s[%ds])", expr, int(max(2*step, time.Minute).Seconds()))
	}
	if len(q.By) > 0 {
		by := make([]string, 0, len(q.By))
		for _, l := range q.By {
			switch l {
			case "machine":
				by = append(by, strconv.Quote(spec.AttrAgent))
			case "container":
				by = append(by, strconv.Quote(spec.AttrEntity))
			default:
				if !labelName.MatchString(l) || internalLabel.MatchString(l) {
					return SeriesResult{}, errs.Invalid("cannot group by "+l).Because("metric_label_unknown", "label", l)
				}
				by = append(by, strconv.Quote(l))
			}
		}
		expr = fmt.Sprintf("%s by (%s) (%s)", agg, strings.Join(by, ", "), expr)
	}
	raw, err := s.metrics.Raw(ctx, ScopeOf(r), expr, w.Start, w.End, step)
	if err != nil {
		return SeriesResult{}, err
	}
	rows, err := promMatrix(raw)
	if err != nil {
		return SeriesResult{}, errs.Wrap(errs.CodeUnavailable, "metric series", err).Because("metrics_snapshot_invalid")
	}
	rows = mergeRows(rows)
	own := map[string]bool{}
	out := SeriesResult{Window: w, Name: name, Fn: fn, Total: len(rows), Series: make([]LabeledSeries, 0, len(rows))}
	for _, row := range rows {
		ls := LabeledSeries{Labels: map[string]string{}, Points: row.points}
		for k, v := range row.labels {
			if !internalLabel.MatchString(k) {
				ls.Labels[k] = v
				own[k] = true
			}
		}
		ls.Machine = id.machineOf[row.labels[spec.AttrAgent]]
		if c, ok := id.containerOf[row.labels[spec.AttrEntity]]; ok {
			ls.Container = c
			if ls.Machine == "" {
				ls.Machine = id.hostOf[c]
			}
		}
		ls.Role = id.roleOf[ls.Machine]
		ls.Avg, ls.Min, ls.Max, ls.Last, ls.P95 = aggregatesOf(row.points)
		out.Series = append(out.Series, ls)
	}
	// The metric's own labels, from an ungrouped look when grouped.
	out.Labels = keysOf(own)
	if len(q.By) > 0 {
		out.Labels = s.labelsOf(ctx, r, w, name)
	}
	sort.SliceStable(out.Series, func(i, j int) bool { return out.Series[i].Avg > out.Series[j].Avg })
	if len(out.Series) > maxSeries {
		out.Series, out.Truncated = out.Series[:maxSeries], true
	}
	return out, nil
}

// labelsOf are the metric's own label names.
func (s *Service) labelsOf(ctx context.Context, r run.Run, w Window, name string) []string {
	raw, err := s.metrics.Raw(ctx, ScopeOf(r), fmt.Sprintf("{__name__=%q}", name), w.Start, w.End, max(w.End.Sub(w.Start)/10, time.Minute))
	if err != nil {
		return []string{}
	}
	rows, err := promMatrix(raw)
	if err != nil {
		return []string{}
	}
	own := map[string]bool{}
	for _, row := range rows {
		for k := range row.labels {
			if !internalLabel.MatchString(k) {
				own[k] = true
			}
		}
	}
	return keysOf(own)
}

// mergeRows joins the rows that differ only in the pipeline's own labels
// (an attempt, the activity that sampled — the last sample before a
// container is removed comes from another one): to the reader they are one
// series. Points at the same moment keep the larger value.
func mergeRows(rows []promRow) []promRow {
	index := map[string]int{}
	out := make([]promRow, 0, len(rows))
	for _, row := range rows {
		parts := make([]string, 0, len(row.labels))
		for k, v := range row.labels {
			if k == spec.AttrAgent || k == spec.AttrEntity || !internalLabel.MatchString(k) {
				parts = append(parts, k+"="+v)
			}
		}
		sort.Strings(parts)
		key := strings.Join(parts, "\x00")
		i, ok := index[key]
		if !ok {
			index[key] = len(out)
			out = append(out, row)
			continue
		}
		byT := map[float64]float64{}
		for _, p := range out[i].points {
			byT[p[0]] = p[1]
		}
		for _, p := range row.points {
			if v, seen := byT[p[0]]; !seen || p[1] > v {
				byT[p[0]] = p[1]
			}
		}
		merged := make([][2]float64, 0, len(byT))
		for t, v := range byT {
			merged = append(merged, [2]float64{t, v})
		}
		sort.Slice(merged, func(a, b int) bool { return merged[a][0] < merged[b][0] })
		out[i].points = merged
	}
	return out
}

type promRow struct {
	labels map[string]string
	points [][2]float64
}

// promMatrix reads a Prometheus range-query answer: [unix_ms, value]
// points, NaN and Inf dropped.
func promMatrix(raw json.RawMessage) ([]promRow, error) {
	var body struct {
		Status string `json:"status"`
		Error  string `json:"error"`
		Data   struct {
			Result []struct {
				Metric map[string]string `json:"metric"`
				Values [][2]any          `json:"values"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, err
	}
	if body.Status != "success" {
		return nil, fmt.Errorf("%s", body.Error)
	}
	out := make([]promRow, 0, len(body.Data.Result))
	for _, r := range body.Data.Result {
		row := promRow{labels: r.Metric}
		for _, v := range r.Values {
			ts, ok := v[0].(float64)
			str, ok2 := v[1].(string)
			if !ok || !ok2 {
				continue
			}
			f, err := strconv.ParseFloat(str, 64)
			if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
				continue
			}
			row.points = append(row.points, [2]float64{ts * 1000, f})
		}
		out = append(out, row)
	}
	return out, nil
}

func aggregatesOf(points [][2]float64) (avg, lo, hi, last, p95 float64) {
	if len(points) == 0 {
		return 0, 0, 0, 0, 0
	}
	values := make([]float64, len(points))
	sum := 0.0
	lo, hi = points[0][1], points[0][1]
	for i, p := range points {
		values[i] = p[1]
		sum += p[1]
		lo, hi = min(lo, p[1]), max(hi, p[1])
	}
	sort.Float64s(values)
	return sum / float64(len(points)), lo, hi, points[len(points)-1][1], values[(len(values)-1)*95/100]
}

// anyOf is an anchored regexp matching exactly the values.
func anyOf(values []string) string {
	quoted := make([]string, len(values))
	for i, v := range values {
		quoted[i] = regexp.QuoteMeta(v)
	}
	return strings.Join(quoted, "|")
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		if k != "" {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}
