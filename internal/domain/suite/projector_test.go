package suite

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gopherex/xlog"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
)

type fakeSuites struct {
	Repository
	r SuiteRun
}

func (m *fakeSuites) RunByID(context.Context, uuid.UUID) (SuiteRun, error) { return m.r, nil }
func (m *fakeSuites) SetRunStatus(_ context.Context, _ uuid.UUID, status run.Status, reason string, start, end *time.Time) error {
	m.r.Status, m.r.StatusReason, m.r.FinishedAt = status, reason, end
	if m.r.StartedAt == nil {
		m.r.StartedAt = start
	}
	return nil
}

type fakeCells struct {
	run.Repository
	cells []run.Run
}

func (m *fakeCells) OfSuiteRun(context.Context, uuid.UUID) ([]run.Run, error) { return m.cells, nil }

func TestSuiteStatusFollowsItsCells(t *testing.T) {
	at := time.Now().UTC()
	cell := func(status run.Status, started bool) run.Run {
		c := run.Run{ID: uuid.New(), Status: status}
		if started {
			c.StartedAt = &at
		}
		if status.Terminal() {
			c.FinishedAt = &at
		}
		return c
	}
	for _, tc := range []struct {
		name   string
		suite  run.Status
		cells  []run.Run
		want   run.Status
		reason string
	}{
		{"all queued", run.StatusPending, []run.Run{cell(run.StatusPending, false), cell(run.StatusPending, false)}, run.StatusPending, ""},
		{"a cell started", run.StatusPending, []run.Run{cell(run.StatusRunning, true), cell(run.StatusPending, false)}, run.StatusRunning, ""},
		{"all completed", run.StatusRunning, []run.Run{cell(run.StatusCompleted, true), cell(run.StatusCompleted, true)}, run.StatusCompleted, ""},
		{"a cell failed", run.StatusRunning, []run.Run{cell(run.StatusCompleted, true), cell(run.StatusFailed, true)}, run.StatusFailed, "1 of 2 cells failed"},
		{"cancelled suite", run.StatusCancelling, []run.Run{cell(run.StatusCompleted, true), cell(run.StatusCancelled, false)}, run.StatusCancelled, ""},
		{"still running", run.StatusRunning, []run.Run{cell(run.StatusCompleted, true), cell(run.StatusRunning, true)}, run.StatusRunning, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			suites := &fakeSuites{r: SuiteRun{ID: uuid.New(), Status: tc.suite}}
			p := NewProjector(suites, &fakeCells{cells: tc.cells}, nil, xlog.NewConsole(xlog.WithWriter(io.Discard)))
			p.follow(context.Background(), run.Live{ID: suites.r.ID})
			if suites.r.Status != tc.want || suites.r.StatusReason != tc.reason {
				t.Fatalf("status %s %q, want %s %q", suites.r.Status, suites.r.StatusReason, tc.want, tc.reason)
			}
		})
	}
}
