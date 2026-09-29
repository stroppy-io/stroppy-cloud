package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-faster/jx"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/observe"
	"github.com/stroppy-io/stroppy-cloud/internal/oas"
)

func logLineOf(l observe.LogLine) oas.LogLine {
	out := oas.LogLine{Time: l.Time, Message: l.Message}
	if l.Seq > 0 {
		out.Seq = oas.NewOptInt(l.Seq)
	}
	set := func(dst *oas.OptString, v string) {
		if v != "" {
			*dst = oas.NewOptString(v)
		}
	}
	set(&out.Level, l.Level)
	set(&out.Stream, l.Stream)
	set(&out.Role, l.Role)
	set(&out.Machine, l.Machine)
	set(&out.Container, l.Container)
	set(&out.Phase, l.Phase)
	set(&out.Segment, l.Segment)
	if len(l.Fields) > 0 {
		out.Fields = oas.NewOptLogLineFields(oas.LogLineFields(l.Fields))
	}
	return out
}

func logPageOf(p observe.LogPage) *oas.LogPage {
	out := &oas.LogPage{Data: make([]oas.LogLine, 0, len(p.Lines)), Truncated: oas.NewOptBool(p.Truncated), Older: oas.OptNilString{Null: true, Set: true}, Newer: oas.OptNilString{Null: true, Set: true}}
	for _, l := range p.Lines {
		out.Data = append(out.Data, logLineOf(l))
	}
	if p.Older != "" {
		out.Older = oas.NewOptNilString(p.Older)
	}
	if p.Newer != "" {
		out.Newer = oas.NewOptNilString(p.Newer)
	}
	return out
}

// QueryRunLogs — typed filters over the run's logs.
func (h *Handler) QueryRunLogs(ctx context.Context, params oas.QueryRunLogsParams) (*oas.LogPage, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	q := observe.LogQuery{RunID: params.ID, Roles: params.Role, Machines: params.Machine, Containers: params.Container, Phases: params.Phase, Segment: params.Segment.Or(""), Text: params.Q.Or(""), Cursor: params.Cursor.Or(""), Direction: string(params.Direction.Or(oas.QueryRunLogsDirectionOlder)), Limit: params.Limit.Or(200)}
	for _, s := range params.Stream {
		q.Streams = append(q.Streams, string(s))
	}
	for _, l := range params.Level {
		q.Levels = append(q.Levels, string(l))
	}
	if v, ok := params.Start.Get(); ok {
		q.Start = v
	}
	if v, ok := params.End.Get(); ok {
		q.End = v
	}
	page, _, err := h.deps.Observe.Logs(ctx, a, t.ID, q)
	if err != nil {
		return nil, err
	}
	return logPageOf(page), nil
}

// QueryRunLogsRaw — LogsQL inside the run scope.
func (h *Handler) QueryRunLogsRaw(ctx context.Context, req *oas.QueryRunLogsRawReq, params oas.QueryRunLogsRawParams) (*oas.LogPage, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	var start, end time.Time
	if v, ok := req.Start.Get(); ok {
		start = v
	}
	if v, ok := req.End.Get(); ok {
		end = v
	}
	page, err := h.deps.Observe.RawLogs(ctx, a, t.ID, params.ID, req.Query, start, end, req.Limit.Or(200))
	if err != nil {
		return nil, err
	}
	return logPageOf(page), nil
}

// GetRunLogFacets — filter values present in the run's logs.
func (h *Handler) GetRunLogFacets(ctx context.Context, params oas.GetRunLogFacetsParams) (*oas.GetRunLogFacetsOK, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	facets, err := h.deps.Observe.LogFacets(ctx, a, t.ID, params.ID)
	if err != nil {
		return nil, err
	}
	out := &oas.GetRunLogFacetsOK{Data: make([]oas.Facet, 0, len(facets))}
	for _, f := range facets {
		item := oas.Facet{Field: f.Field, Values: make([]oas.FacetValuesItem, 0, len(f.Values))}
		for _, v := range f.Values {
			item.Values = append(item.Values, oas.FacetValuesItem{Value: v.Value, Count: v.Count})
		}
		out.Data = append(out.Data, item)
	}
	return out, nil
}

func seriesOf(list []observe.Series) []oas.RunMetricsSeriesItem {
	out := make([]oas.RunMetricsSeriesItem, 0, len(list))
	for _, s := range list {
		item := oas.RunMetricsSeriesItem{Key: s.Key, Points: make([][]float64, 0, len(s.Points)), Aggregates: oas.NewOptRunMetricsSeriesItemAggregates(oas.RunMetricsSeriesItemAggregates{Avg: oas.NewOptFloat64(s.Avg), Min: oas.NewOptFloat64(s.Min), Max: oas.NewOptFloat64(s.Max), Last: oas.NewOptFloat64(s.Last), P95: oas.NewOptFloat64(s.P95)})}
		if s.Title != "" {
			item.Title = oas.NewOptString(s.Title)
		}
		if s.Unit != "" {
			item.Unit = oas.NewOptString(s.Unit)
		}
		if s.Role != "" {
			item.Role = oas.NewOptString(s.Role)
		}
		if s.Machine != "" {
			item.Machine = oas.NewOptString(s.Machine)
		}
		for _, p := range s.Points {
			// Prometheus range queries return unix seconds; the contract is `[unix_ms, value]`.
			item.Points = append(item.Points, []float64{p[0] * 1000, p[1]})
		}
		out = append(out, item)
	}
	return out
}

// GetRunMetrics — catalog keys over a window.
func (h *Handler) GetRunMetrics(ctx context.Context, params oas.GetRunMetricsParams) (*oas.RunMetrics, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	var start, end time.Time
	if v, ok := params.Start.Get(); ok {
		start = v
	}
	if v, ok := params.End.Get(); ok {
		end = v
	}
	var step time.Duration
	if v, ok := params.Step.Get(); ok && v != "" {
		if step, err = time.ParseDuration(v); err != nil {
			return nil, errs.Invalid("step is not a duration")
		}
	}
	series, failures, w, err := h.deps.Observe.Metrics(ctx, a, t.ID, params.ID, params.Keys, params.Segment.Or(""), start, end, step)
	if err != nil {
		return nil, err
	}
	out := &oas.RunMetrics{Window: oas.RunMetricsWindow{Start: w.Start, End: w.End}, Series: seriesOf(series), Errors: []oas.RunMetricsErrorsItem{}}
	if w.Segment != "" {
		out.Window.Segment = oas.NewOptString(w.Segment)
	}
	for _, f := range failures {
		key, msg, _ := strings.Cut(f.Error(), ": ")
		out.Errors = append(out.Errors, oas.RunMetricsErrorsItem{Key: oas.NewOptString(key), Error: oas.NewOptString(msg)})
	}
	return out, nil
}

// QueryRunMetricsRaw — PromQL inside the run scope; the store's answer is
// returned as is.
func (h *Handler) QueryRunMetricsRaw(ctx context.Context, req *oas.QueryRunMetricsRawReq, params oas.QueryRunMetricsRawParams) (oas.QueryRunMetricsRawOK, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	var step time.Duration
	if v, ok := req.Step.Get(); ok && v != "" {
		if step, err = time.ParseDuration(v); err != nil {
			return nil, errs.Invalid("step is not a duration")
		}
	}
	raw, err := h.deps.Observe.RawMetrics(ctx, a, t.ID, params.ID, req.Query, req.Start, req.End, step)
	if err != nil {
		return nil, err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, errs.Wrap(errs.CodeUnavailable, "metrics store answer", err)
	}
	out := oas.QueryRunMetricsRawOK{}
	for k, v := range m {
		out[k] = jx.Raw(v)
	}
	return out, nil
}

// GrafanaConfig is the installation's Grafana relay: the dashboards the
// run detail links to, each `uid=Title[:per_machine]`.
type GrafanaConfig struct {
	Enabled    bool
	Dashboards []string
}

// CreateRunGrafanaSession — dashboard links for the run's window. The
// scope cookie of the relay lands with the Grafana relay work.
func (h *Handler) CreateRunGrafanaSession(ctx context.Context, params oas.CreateRunGrafanaSessionParams) (*oas.GrafanaSession, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	if !h.deps.Grafana.Enabled {
		return nil, errs.New(errs.CodeUnavailable, "grafana is not connected")
	}
	r, err := h.deps.Runs.Get(ctx, a, t.ID, params.ID)
	if err != nil {
		return nil, err
	}
	w, err := observe.WindowOf(r, "", time.Time{}, time.Time{})
	if err != nil {
		return nil, err
	}
	out := &oas.GrafanaSession{ExpiresAt: time.Now().UTC().Add(time.Hour), Dashboards: []oas.GrafanaSessionDashboardsItem{}}
	for _, d := range h.deps.Grafana.Dashboards {
		uid, rest, _ := strings.Cut(d, "=")
		title, flag, _ := strings.Cut(rest, ":")
		if uid == "" {
			continue
		}
		if title == "" {
			title = uid
		}
		q := url.Values{"var-run_id": {r.ID.String()}, "from": {strconv.FormatInt(w.Start.UnixMilli(), 10)}, "to": {strconv.FormatInt(w.End.UnixMilli(), 10)}}
		item := oas.GrafanaSessionDashboardsItem{ID: uid, Title: title, URL: fmt.Sprintf("/grafana/d/%s?%s", uid, q.Encode())}
		if flag == "per_machine" {
			item.PerMachine = oas.NewOptBool(true)
		}
		out.Dashboards = append(out.Dashboards, item)
	}
	return out, nil
}

// CreatePublicShareGrafanaSession — dashboards of a shared run (scope
// metrics or wider).
func (h *Handler) CreatePublicShareGrafanaSession(ctx context.Context, params oas.CreatePublicShareGrafanaSessionParams) (*oas.GrafanaSession, error) {
	s, err := h.deps.Shares.Public(ctx, params.Token)
	if err != nil {
		return nil, err
	}
	if s.Scope == "overview" || s.Snapshot.Run == nil {
		return nil, errs.Forbidden("this share does not include metrics")
	}
	if !h.deps.Grafana.Enabled {
		return nil, errs.New(errs.CodeUnavailable, "grafana is not connected")
	}
	run := s.Snapshot.Run
	start, end := s.CapturedAt.Add(-time.Hour), s.CapturedAt
	if run.StartedAt != nil {
		start = *run.StartedAt
	}
	if run.FinishedAt != nil {
		end = *run.FinishedAt
	}
	out := &oas.GrafanaSession{ExpiresAt: time.Now().UTC().Add(time.Hour), Dashboards: []oas.GrafanaSessionDashboardsItem{}}
	for _, d := range h.deps.Grafana.Dashboards {
		uid, rest, _ := strings.Cut(d, "=")
		title, _, _ := strings.Cut(rest, ":")
		if uid == "" {
			continue
		}
		q := url.Values{"var-run_id": {run.ID.String()}, "from": {strconv.FormatInt(start.UnixMilli(), 10)}, "to": {strconv.FormatInt(end.UnixMilli(), 10)}}
		out.Dashboards = append(out.Dashboards, oas.GrafanaSessionDashboardsItem{ID: uid, Title: orDefault(title, uid), URL: fmt.Sprintf("/grafana/d/%s?%s", uid, q.Encode())})
	}
	return out, nil
}
