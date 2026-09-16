// Package suite is the orchestration of stroppy-suite: a fan-out of
// stroppy-run child runs, one per cell, bounded by concurrency. A failed
// cell never aborts its siblings; the suite result carries every cell's
// outcome.
package suite

import (
	"fmt"

	"github.com/graphene-ci/pipeline/pkg/pipeline"

	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/events"
	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/run"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// PipelineID is the Graphene pipeline id of stroppy-suite.
const PipelineID = "stroppy-suite"

// Milestones of a suite run.
const (
	CellStarted  = "cell.started"
	CellFinished = "cell.finished"
	CellFailed   = "cell.failed"
)

// Run is the pipeline body: SuiteSpec in, SuiteResult out.
func Run(ctx pipeline.Context, s spec.Suite) (spec.SuiteResult, error) {
	if ctx.Recording() {
		events.Register(ctx)
		pipeline.RunAll[spec.Result](ctx, run.PipelineID, nil, 1)
		return spec.SuiteResult{}, nil
	}
	normalized, normalizeErr := spec.NormalizeSuite(s)
	if normalizeErr != nil {
		return spec.SuiteResult{}, normalizeErr
	}
	s = normalized
	if len(s.Cells) == 0 {
		return spec.SuiteResult{}, fmt.Errorf("a suite needs at least one cell")
	}
	cells := make([]pipeline.Cell, 0, len(s.Cells))
	for _, c := range s.Cells {
		if c.ID == "" {
			return spec.SuiteResult{}, fmt.Errorf("a cell needs an id")
		}
		// Every child run is stamped with the suite so the server groups
		// them; the cell's own labels come from the server-compiled spec.
		cells = append(cells, pipeline.Cell{ID: c.ID, Params: c.RunSpec})
		events.Emit(ctx, CellStarted, events.Payload{"cell": c.ID, "run_id": c.RunSpec.RunID})
	}
	labels := map[string]string{"stroppy-suite-run": s.SuiteRunID, "stroppy-tenant": s.Tenant}
	for k, v := range s.Defaults.Labels {
		labels[k] = v
	}
	handles := pipeline.RunAll[spec.Result](ctx, run.PipelineID, cells, s.Concurrency, pipeline.WithLabels(labels))

	result := spec.SuiteResult{Total: len(s.Cells)}
	for i, h := range handles {
		cell := s.Cells[i]
		res, err := h.TryReady(ctx)
		// Operator cancellation is not a failed cell, even when the suite
		// is configured to continue after ordinary workload failures.
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		out := spec.SuiteCellResult{ID: cell.ID, RunID: string(h.ResourceRef())}
		if err != nil {
			out.Status = "failed"
			out.Error = err.Error()
			result.Fail++
			events.Emit(ctx, CellFailed, events.Payload{"cell": cell.ID, "error": err.Error()})
		} else {
			out.Status = "completed"
			out.Result = &res
			result.Done++
			events.Emit(ctx, CellFinished, events.Payload{"cell": cell.ID, "tps": res.Summary.TPS})
		}
		result.Cells = append(result.Cells, out)
		if err != nil && !s.Defaults.ContinueOnFailure {
			// Stop reading further cells: the remaining handles keep running
			// under the parent's cascade; the suite reports what it has.
			break
		}
	}
	if result.Fail > 0 && !s.Defaults.ContinueOnFailure {
		return result, fmt.Errorf("suite: %d of %d cells failed", result.Fail, result.Total)
	}
	return result, nil
}
