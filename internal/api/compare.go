package api

import (
	"context"

	"github.com/go-faster/jx"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/compare"
	"github.com/stroppy-io/stroppy-cloud/internal/oas"
)

func comparisonOf(c compare.Comparison) oas.Comparison {
	out := oas.Comparison{BaselineRunID: oas.NewOptUUID(c.BaselineID), Columns: make([]oas.ComparisonColumnsItem, 0, len(c.Columns)), Metrics: make([]oas.ComparisonMetricsItem, 0, len(c.Metrics)), SpecDiff: oas.ComparisonSpecDiff{}}
	for _, col := range c.Columns {
		item := oas.ComparisonColumnsItem{
			RunID: col.Run.ID, Name: oas.NewOptString(col.Run.Name), Status: oas.NewOptRunStatus(oas.RunStatus(col.Run.Status)), Summary: oas.NewOptRunSummary(runSummaryOf(col.Run)),
			Verdict: oas.NewOptComparisonColumnsItemVerdict(oas.ComparisonColumnsItemVerdict{Better: oas.NewOptInt(col.Verdict.Better), Worse: oas.NewOptInt(col.Verdict.Worse), Same: oas.NewOptInt(col.Verdict.Same), Missing: oas.NewOptInt(col.Verdict.Missing)}),
		}
		if col.Run.StartedAt != nil {
			item.StartedAt = oas.NewOptDateTime(*col.Run.StartedAt)
		}
		if d := compare.Duration(col.Run); d > 0 {
			item.Duration = oas.NewOptString(d.String())
		}
		out.Columns = append(out.Columns, item)
	}
	for _, row := range c.Metrics {
		item := oas.ComparisonMetricsItem{Key: row.Metric.Key, Title: oas.NewOptString(row.Metric.Title), HigherIsBetter: oas.NewOptBool(row.Metric.HigherIsBetter), Cells: make([]oas.ComparisonMetricsItemCellsItem, 0, len(row.Cells))}
		if row.Metric.Unit != "" {
			item.Unit = oas.NewOptString(row.Metric.Unit)
		}
		if row.Metric.Group != "" {
			item.Group = oas.NewOptString(row.Metric.Group)
		}
		for _, cell := range row.Cells {
			x := oas.ComparisonMetricsItemCellsItem{RunID: cell.RunID, Present: cell.Present, Verdict: oas.NewOptComparisonMetricsItemCellsItemVerdict(oas.ComparisonMetricsItemCellsItemVerdict(cell.Verdict))}
			if cell.Present {
				x.Value = oas.NewOptFloat64(cell.Value)
			}
			if cell.DiffPct != nil {
				x.DiffPct = oas.NewOptFloat64(*cell.DiffPct)
			}
			item.Cells = append(item.Cells, x)
		}
		out.Metrics = append(out.Metrics, item)
	}
	for section, byRun := range c.SpecDiff {
		for runID, changes := range byRun {
			d := oas.Diff{Changes: make([]oas.DiffChangesItem, 0, len(changes))}
			for _, ch := range changes {
				d.Changes = append(d.Changes, oas.DiffChangesItem{Path: ch.Path, Op: oas.DiffChangesItemOp(ch.Op), A: jx.Raw(ch.A), B: jx.Raw(ch.B)})
			}
			out.SpecDiff[section+"/"+runID.String()] = d
		}
	}
	return out
}

// CompareRuns — ad-hoc comparison.
func (h *Handler) CompareRuns(ctx context.Context, req *oas.CompareRequest, params oas.CompareRunsParams) (*oas.Comparison, error) {
	a, t, err := h.tenantOf(ctx, params.Slug)
	if err != nil {
		return nil, err
	}
	c, err := h.deps.Compare.Compare(ctx, a, t.ID, compareRequestOf(req.RunIds, req.BaselineRunID, req.MetricKeys, req.DeadbandPct))
	if err != nil {
		return nil, err
	}
	out := comparisonOf(c)
	return &out, nil
}
