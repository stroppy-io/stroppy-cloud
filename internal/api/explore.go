package api

import (
	"context"
	"time"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/observe"
	"github.com/stroppy-io/stroppy-cloud/internal/oas"
)

// ListRunMetricNames — every metric the run stored.
func (h *Handler) ListRunMetricNames(ctx context.Context, params oas.ListRunMetricNamesParams) (*oas.RunMetricNames, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	names, w, err := h.deps.Observe.MetricNames(ctx, a, t.ID, params.ID, params.Segment.Or(""), params.Start.Or(time.Time{}), params.End.Or(time.Time{}))
	if err != nil {
		return nil, err
	}
	return metricNamesOf(names, w), nil
}

// GetRunMetricSeries — the series of one stored metric.
func (h *Handler) GetRunMetricSeries(ctx context.Context, params oas.GetRunMetricSeriesParams) (*oas.RunMetricSeries, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	q := observe.SeriesQuery{
		Name: params.Name, Fn: string(params.Fn.Or("")), By: params.By, Agg: string(params.Agg.Or("")),
		Roles: params.Role, Machines: params.Machine, Containers: params.Container, Segment: params.Segment.Or(""),
		Start: params.Start.Or(time.Time{}), End: params.End.Or(time.Time{}),
	}
	if v, ok := params.Step.Get(); ok && v != "" {
		if q.Step, err = time.ParseDuration(v); err != nil {
			return nil, errs.Invalid("step is not a duration").Because("step_is_not_a_duration")
		}
	}
	res, err := h.deps.Observe.MetricSeries(ctx, a, t.ID, params.ID, q)
	if err != nil {
		return nil, err
	}
	return metricSeriesOf(res), nil
}

// ListPublicShareMetricNames — every metric of a shared run.
func (h *Handler) ListPublicShareMetricNames(ctx context.Context, params oas.ListPublicShareMetricNamesParams) (*oas.RunMetricNames, error) {
	names, w, err := h.deps.Observe.SharedMetricNames(ctx, h.deps.Shares, params.Token, params.Start.Or(time.Time{}), params.End.Or(time.Time{}))
	if err != nil {
		return nil, err
	}
	return metricNamesOf(names, w), nil
}

// GetPublicShareMetricSeries — the series of one metric of a shared run.
func (h *Handler) GetPublicShareMetricSeries(ctx context.Context, params oas.GetPublicShareMetricSeriesParams) (*oas.RunMetricSeries, error) {
	res, err := h.deps.Observe.SharedMetricSeries(ctx, h.deps.Shares, params.Token, observe.SeriesQuery{
		Name: params.Name, Fn: string(params.Fn.Or("")), By: params.By, Agg: string(params.Agg.Or("")),
		Roles: params.Role, Machines: params.Machine, Containers: params.Container,
		Start: params.Start.Or(time.Time{}), End: params.End.Or(time.Time{}),
	})
	if err != nil {
		return nil, err
	}
	return metricSeriesOf(res), nil
}

func metricNamesOf(names []observe.MetricName, w observe.Window) *oas.RunMetricNames {
	out := &oas.RunMetricNames{Window: oas.RunMetricNamesWindow{Start: w.Start, End: w.End}, Data: make([]oas.RunMetricNamesDataItem, 0, len(names))}
	if w.Segment != "" {
		out.Window.Segment = oas.NewOptString(w.Segment)
	}
	for _, n := range names {
		out.Data = append(out.Data, oas.RunMetricNamesDataItem{
			Name: n.Name, Origin: oas.MetricOrigin(n.Origin), Kind: oas.MetricKind(n.Kind), Series: n.Series,
			Machines: nonNil(n.Machines), Roles: nonNil(n.Roles), Containers: nonNil(n.Containers),
		})
	}
	return out
}

func metricSeriesOf(res observe.SeriesResult) *oas.RunMetricSeries {
	out := &oas.RunMetricSeries{
		Window: oas.RunMetricSeriesWindow{Start: res.Window.Start, End: res.Window.End}, Name: res.Name, Fn: oas.MetricSeriesFn(res.Fn),
		Labels: nonNil(res.Labels), Total: res.Total, Truncated: res.Truncated, Series: make([]oas.RunMetricSeriesSeriesItem, 0, len(res.Series)),
	}
	if res.Window.Segment != "" {
		out.Window.Segment = oas.NewOptString(res.Window.Segment)
	}
	for _, s := range res.Series {
		item := oas.RunMetricSeriesSeriesItem{
			Labels: oas.RunMetricSeriesSeriesItemLabels(s.Labels), Points: make([][]float64, 0, len(s.Points)),
			Aggregates: oas.NewOptRunMetricSeriesSeriesItemAggregates(oas.RunMetricSeriesSeriesItemAggregates{
				Avg: oas.NewOptFloat64(s.Avg), Min: oas.NewOptFloat64(s.Min), Max: oas.NewOptFloat64(s.Max), Last: oas.NewOptFloat64(s.Last), P95: oas.NewOptFloat64(s.P95),
			}),
		}
		if s.Machine != "" {
			item.Machine = oas.NewOptString(s.Machine)
		}
		if s.Role != "" {
			item.Role = oas.NewOptString(s.Role)
		}
		if s.Container != "" {
			item.Container = oas.NewOptString(s.Container)
		}
		for _, p := range s.Points {
			item.Points = append(item.Points, []float64{p[0], p[1]})
		}
		out.Series = append(out.Series, item)
	}
	return out
}

func nonNil(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}
