//go:build integration

package application

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/schedule"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/tenant"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/repositories"
)

type suiteRunView struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Progress struct {
		Total, Done, Failed, Running, Pending, Cancelled int
		Pct                                              float64 `json:"pct"`
	} `json:"progress"`
	Cells []struct {
		CellID string `json:"cell_id"`
		Status string `json:"status"`
		Run    struct {
			ID string `json:"id"`
		} `json:"run"`
	} `json:"cells"`
	TriggerRef struct {
		RetryOf    string `json:"retry_of"`
		ScheduleID string `json:"schedule_id"`
	} `json:"trigger_ref"`
	Trigger string `json:"trigger"`
}

func TestE2ESuitesAndSchedules(t *testing.T) {
	e := e2eServer(t)
	base, tok, testID, tn := runFixture(t, e)
	ctx := e.ctx

	var created struct {
		ID            string `json:"id"`
		ComputedCells []struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			Enabled    bool   `json:"enabled"`
			Generated  bool   `json:"generated"`
			Validation struct {
				Fits bool `json:"fits"`
			} `json:"validation"`
			Axis struct {
				WorkloadVariant string `json:"workload_variant"`
			} `json:"axis"`
		} `json:"computed_cells"`
		Summary struct {
			CellCount        int `json:"cell_count"`
			EnabledCellCount int `json:"enabled_cell_count"`
			RunCount         int `json:"run_count"`
		} `json:"summary"`
	}
	body := map[string]any{
		"name": "matrix", "tests": []any{map[string]any{"ref": map[string]any{"id": testID}}},
		"axes": map[string]any{
			"sizes":             []any{map[string]any{"db": map[string]any{"size": "S"}, "runner": map[string]any{"size": "S"}}, map[string]any{"db": map[string]any{"size": "M"}, "runner": map[string]any{"size": "S"}}},
			"workload_variants": []any{map[string]any{"name": "light", "vus": 4}},
		},
		"concurrency": 1,
	}
	t.Run("preview and create materialise the matrix", func(t *testing.T) {
		var preview struct {
			Cells      []map[string]any `json:"cells"`
			Validation struct {
				Fits bool `json:"fits"`
			} `json:"validation"`
			Totals struct {
				Machines int `json:"machines"`
			} `json:"totals"`
			QuotaCheck []map[string]any `json:"quota_check"`
		}
		e.want(e.req(http.MethodPost, base+"/suites:preview", body, tok), http.StatusOK, &preview)
		if len(preview.Cells) != 2 || !preview.Validation.Fits || preview.Totals.Machines != 4 || len(preview.QuotaCheck) != 1 {
			t.Fatalf("preview %+v", preview)
		}
		e.want(e.req(http.MethodPost, base+"/suites", body, tok), http.StatusCreated, &created)
		if len(created.ComputedCells) != 2 || created.Summary.EnabledCellCount != 2 || !created.ComputedCells[0].Validation.Fits || created.ComputedCells[0].Axis.WorkloadVariant != "light" {
			t.Fatalf("created %+v", created)
		}
		e.problem(e.req(http.MethodPost, base+"/suites", body, tok), http.StatusConflict, "conflict")
	})

	t.Run("manual cell edits survive regeneration", func(t *testing.T) {
		first := created.ComputedCells[0]
		var patched struct {
			ComputedCells []struct {
				ID      string `json:"id"`
				Enabled bool   `json:"enabled"`
				Name    string `json:"name"`
			} `json:"computed_cells"`
			Summary struct {
				EnabledCellCount int `json:"enabled_cell_count"`
			} `json:"summary"`
		}
		e.want(e.req(http.MethodPatch, base+"/suites/"+created.ID, map[string]any{
			"cells": []any{map[string]any{"id": first.ID, "test": map[string]any{"id": testID}, "enabled": false, "name": "skip me"}},
		}, tok), http.StatusOK, &patched)
		if patched.Summary.EnabledCellCount != 1 || patched.ComputedCells[0].Enabled || patched.ComputedCells[0].Name != "skip me" {
			t.Fatalf("patched %+v", patched)
		}
		e.want(e.req(http.MethodPatch, base+"/suites/"+created.ID, map[string]any{"cells": []any{}}, tok), http.StatusOK, &patched)
		if patched.Summary.EnabledCellCount != 2 {
			t.Fatalf("restored %+v", patched)
		}
		var list struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		e.want(e.req(http.MethodGet, base+"/suites?search=matr", nil, tok), http.StatusOK, &list)
		if len(list.Data) != 1 || list.Data[0].ID != created.ID {
			t.Fatalf("list %+v", list)
		}
		r := e.reqAccept(http.MethodGet, base+"/suites/"+created.ID+":export", "application/yaml", tok)
		if r.Status != http.StatusOK || !strings.Contains(string(r.Body), "kind: Suite") {
			t.Fatalf("export %d %s", r.Status, r.Body)
		}
	})

	// Cell i of the matrix measures 1000+100*i transactions per second.
	cellTPS := func(tps func(i int) float64) func(string, json.RawMessage) simScenario {
		return func(runID string, _ json.RawMessage) simScenario {
			for i, c := range created.ComputedCells {
				if strings.HasSuffix(runID, "-"+c.ID) {
					return simScenario{TPS: tps(i)}
				}
			}
			return simScenario{}
		}
	}
	var sr suiteRunView
	t.Run("launch prepares every cell and starts stroppy-suite", func(t *testing.T) {
		e.graphene.scenario = cellTPS(func(i int) float64 { return float64(1000 + 100*i) })
		e.want(e.req(http.MethodPost, base+"/suites/"+created.ID+":launch", map[string]any{"name": "m1"}, tok), http.StatusCreated, &sr)
		if sr.Status != "pending" || sr.Progress.Total != 2 || sr.Progress.Pending != 2 || len(sr.Cells) != 2 || sr.Cells[0].Run.ID == "" {
			t.Fatalf("suite run %+v", sr)
		}
		fr, ok := e.graphene.runOf(sr.ID)
		if !ok || fr.pipeline != "stroppy-suite" || fr.namespace != tn.GrapheneNamespace {
			t.Fatalf("graphene %+v", fr)
		}
		if !strings.Contains(string(fr.params), `"suite_run_id":"`+sr.ID+`"`) || strings.Count(string(fr.params), `"run_spec"`) != 2 {
			t.Fatalf("params %s", fr.params[:200])
		}
		var child runView
		e.want(e.req(http.MethodGet, base+"/runs/"+sr.Cells[0].Run.ID, nil, tok), http.StatusOK, &child)
		if child.Trigger != "suite" || child.Status != "pending" {
			t.Fatalf("child %+v", child)
		}
		var runs struct {
			Data []runView `json:"data"`
		}
		e.want(e.req(http.MethodGet, base+"/runs?suite_run_id="+sr.ID, nil, tok), http.StatusOK, &runs)
		if len(runs.Data) != 2 {
			t.Fatalf("children %d", len(runs.Data))
		}
		e.want(e.req(http.MethodGet, base+"/runs?standalone=true", nil, tok), http.StatusOK, &runs)
		for _, r := range runs.Data {
			if r.Trigger == "suite" {
				t.Fatalf("standalone filter leaked %+v", r)
			}
		}
	})

	t.Run("children and the suite are projected to completion", func(t *testing.T) {
		// stroppy-suite ran its cells as child runs "<suite_run_id>-<cell_id>".
		eventually(t, 10*time.Second, func() bool {
			e.app.services.SuiteProj.Tick(ctx)
			e.app.services.Projector.Tick(ctx)
			var got suiteRunView
			e.want(e.req(http.MethodGet, base+"/suite-runs/"+sr.ID, nil, tok), http.StatusOK, &got)
			return got.Status == "completed"
		})
		var got suiteRunView
		eventually(t, 10*time.Second, func() bool {
			e.app.services.Projector.Tick(ctx)
			e.want(e.req(http.MethodGet, base+"/suite-runs/"+sr.ID, nil, tok), http.StatusOK, &got)
			return got.Progress.Done == 2
		})
		if got.Progress.Pct != 100 {
			t.Fatalf("progress %+v", got.Progress)
		}
		var summary struct {
			MetricKeys []string `json:"metric_keys"`
			Rows       []struct {
				CellID  string `json:"cell_id"`
				Metrics map[string]struct {
					Value   float64 `json:"value"`
					Verdict string  `json:"verdict"`
				} `json:"metrics"`
			} `json:"rows"`
		}
		e.want(e.req(http.MethodGet, base+"/suite-runs/"+sr.ID+"/summary", nil, tok), http.StatusOK, &summary)
		if len(summary.Rows) != 2 || summary.Rows[0].Metrics["tps"].Value != 1000 || summary.Rows[0].Metrics["tps"].Verdict != "missing" {
			t.Fatalf("summary %+v", summary)
		}
		e.problem(e.req(http.MethodPost, base+"/suite-runs/"+sr.ID+":retry-failed", nil, tok), http.StatusConflict, "conflict")
		e.problem(e.req(http.MethodPost, base+"/suite-runs/"+sr.ID+":cancel", nil, tok), http.StatusConflict, "conflict")
		var suiteView struct {
			Summary struct {
				RunCount int `json:"run_count"`
				LastRun  struct {
					Status string `json:"status"`
				} `json:"last_run"`
			} `json:"summary"`
		}
		e.want(e.req(http.MethodGet, base+"/suites/"+created.ID, nil, tok), http.StatusOK, &suiteView)
		if suiteView.Summary.RunCount != 1 || suiteView.Summary.LastRun.Status != "completed" {
			t.Fatalf("suite summary %+v", suiteView.Summary)
		}
	})

	t.Run("cancel cascades, retry-failed relaunches cancelled cells, compare shows regression", func(t *testing.T) {
		var second suiteRunView
		// The suite pauses once it started its first cell.
		e.graphene.scenario = func(runID string, _ json.RawMessage) simScenario {
			if strings.Count(runID, "-") == 4 {
				return simScenario{Hold: "cell.started"}
			}
			return simScenario{}
		}
		e.want(e.req(http.MethodPost, base+"/suites/"+created.ID+":launch", map[string]any{"name": "m2"}, tok), http.StatusCreated, &second)
		e.app.services.SuiteProj.Tick(ctx)
		e.app.services.Projector.Tick(ctx)
		var cancelled suiteRunView
		e.want(e.req(http.MethodPost, base+"/suite-runs/"+second.ID+":cancel", nil, tok), http.StatusOK, &cancelled)
		eventually(t, 10*time.Second, func() bool {
			e.app.services.SuiteProj.Tick(ctx)
			e.app.services.Projector.Tick(ctx)
			e.want(e.req(http.MethodGet, base+"/suite-runs/"+second.ID, nil, tok), http.StatusOK, &cancelled)
			return cancelled.Status == "cancelled" && cancelled.Progress.Cancelled == 2
		})
		e.problem(e.req(http.MethodDelete, base+"/suite-runs/"+sr.ID+"x", nil, tok), http.StatusUnprocessableEntity, "invalid")

		var retry suiteRunView
		e.graphene.scenario = cellTPS(func(int) float64 { return 500 })
		e.want(e.req(http.MethodPost, base+"/suite-runs/"+second.ID+":retry-failed", nil, tok), http.StatusCreated, &retry)
		if retry.TriggerRef.RetryOf != second.ID || retry.Progress.Total != 2 {
			t.Fatalf("retry %+v", retry)
		}
		// The retry measures lower tps: compared to the first run it regresses.
		var done suiteRunView
		eventually(t, 10*time.Second, func() bool {
			e.app.services.SuiteProj.Tick(ctx)
			e.app.services.Projector.Tick(ctx)
			e.want(e.req(http.MethodGet, base+"/suite-runs/"+retry.ID, nil, tok), http.StatusOK, &done)
			return done.Status == "completed" && done.Progress.Done == 2
		})
		e.graphene.scenario = nil
		var summary struct {
			ComparedTo string `json:"compared_to"`
			Rows       []struct {
				Metrics map[string]struct {
					Previous float64 `json:"previous"`
					Verdict  string  `json:"verdict"`
				} `json:"metrics"`
			} `json:"rows"`
		}
		e.want(e.req(http.MethodGet, base+"/suite-runs/"+retry.ID+"/summary", nil, tok), http.StatusOK, &summary)
		if summary.ComparedTo != sr.ID || summary.Rows[0].Metrics["tps"].Verdict != "worse" || summary.Rows[0].Metrics["tps"].Previous != 1000 {
			t.Fatalf("compare %+v", summary)
		}
		e.want(e.req(http.MethodDelete, base+"/suite-runs/"+second.ID, nil, tok), http.StatusNoContent, nil)
		e.problem(e.req(http.MethodGet, base+"/suite-runs/"+second.ID, nil, tok), http.StatusNotFound, "not_found")
		var list struct {
			Data []suiteRunView `json:"data"`
		}
		e.want(e.req(http.MethodGet, base+"/suite-runs?status=completed&suite_id="+created.ID, nil, tok), http.StatusOK, &list)
		if len(list.Data) != 2 {
			t.Fatalf("list %d", len(list.Data))
		}
	})

	t.Run("schedules fire tests and suites", func(t *testing.T) {
		sbase := base + "/schedules"
		var sch struct {
			ID        string  `json:"id"`
			Enabled   bool    `json:"enabled"`
			NextRunAt *string `json:"next_run_at"`
			Target    struct {
				Name string `json:"name"`
			} `json:"target"`
			LastRun struct {
				Kind   string `json:"kind"`
				Status string `json:"status"`
			} `json:"last_run"`
		}
		e.problem(e.req(http.MethodPost, sbase, map[string]any{"name": "bad", "target": map[string]any{"kind": "test", "id": testID}, "cron": "99 * * * *"}, tok), http.StatusUnprocessableEntity, "invalid")
		e.want(e.req(http.MethodPost, sbase, map[string]any{"name": "nightly", "target": map[string]any{"kind": "test", "id": testID}, "cron": "0 3 * * *", "timezone": "Europe/Moscow", "overrides": map[string]any{"labels": map[string]any{"nightly": "1"}}}, tok), http.StatusCreated, &sch)
		if !sch.Enabled || sch.NextRunAt == nil || sch.Target.Name != "pg tpcc" {
			t.Fatalf("schedule %+v", sch)
		}
		e.want(e.req(http.MethodPost, sbase+"/"+sch.ID+":pause", nil, tok), http.StatusOK, &sch)
		if sch.Enabled || sch.NextRunAt != nil {
			t.Fatalf("paused %+v", sch)
		}
		e.want(e.req(http.MethodPost, sbase+"/"+sch.ID+":resume", nil, tok), http.StatusOK, &sch)
		if !sch.Enabled || sch.NextRunAt == nil {
			t.Fatalf("resumed %+v", sch)
		}
		var ref struct {
			Kind   string `json:"kind"`
			ID     string `json:"id"`
			Status string `json:"status"`
		}
		e.want(e.req(http.MethodPost, sbase+"/"+sch.ID+":run-now", nil, tok), http.StatusCreated, &ref)
		if ref.Kind != "run" || ref.Status != "pending" || ref.ID == "" {
			t.Fatalf("run-now %+v", ref)
		}
		var r runView
		e.want(e.req(http.MethodGet, base+"/runs/"+ref.ID, nil, tok), http.StatusOK, &r)
		if r.Trigger != "schedule" {
			t.Fatalf("trigger %+v", r)
		}
		e.want(e.req(http.MethodPost, base+"/runs/"+ref.ID+":cancel", nil, tok), http.StatusOK, nil)

		// The loop fires what is due, as the schedule's author.
		past := time.Now().Add(-time.Minute)
		if err := repositories.NewScheduleRepo(testDB.TxDB).SetFired(ctx, uuid.MustParse(sch.ID), &past, schedule.RunRef{Kind: "run", Status: "pending", At: past}); err != nil {
			t.Fatal(err)
		}
		if n := e.app.services.Schedules.Tick(ctx); n != 1 {
			t.Fatalf("fired %d", n)
		}
		var history struct {
			Data []struct {
				Kind   string `json:"kind"`
				Status string `json:"status"`
				Error  string `json:"error"`
			} `json:"data"`
		}
		e.want(e.req(http.MethodGet, sbase+"/"+sch.ID+"/history", nil, tok), http.StatusOK, &history)
		if len(history.Data) != 2 || history.Data[0].Status != "pending" {
			t.Fatalf("history %+v", history)
		}
		e.want(e.req(http.MethodGet, sbase+"/"+sch.ID, nil, tok), http.StatusOK, &sch)
		if sch.LastRun.Kind != "run" || sch.NextRunAt == nil {
			t.Fatalf("after tick %+v", sch)
		}

		var ss struct {
			ID string `json:"id"`
		}
		e.want(e.req(http.MethodPost, sbase, map[string]any{"name": "weekly matrix", "target": map[string]any{"kind": "suite", "id": created.ID}, "cron": "@weekly"}, tok), http.StatusCreated, &ss)
		var sref struct {
			Kind string `json:"kind"`
			ID   string `json:"id"`
		}
		e.want(e.req(http.MethodPost, sbase+"/"+ss.ID+":run-now", nil, tok), http.StatusCreated, &sref)
		if sref.Kind != "suite_run" {
			t.Fatalf("suite run-now %+v", sref)
		}
		var srun suiteRunView
		e.want(e.req(http.MethodGet, base+"/suite-runs/"+sref.ID, nil, tok), http.StatusOK, &srun)
		if srun.Trigger != "schedule" || srun.TriggerRef.ScheduleID != ss.ID {
			t.Fatalf("scheduled suite run %+v", srun)
		}
		var suiteView struct {
			Summary struct {
				Schedules []map[string]any `json:"schedules"`
			} `json:"summary"`
		}
		e.want(e.req(http.MethodGet, base+"/suites/"+created.ID, nil, tok), http.StatusOK, &suiteView)
		if len(suiteView.Summary.Schedules) != 1 {
			t.Fatalf("suite schedules %+v", suiteView.Summary)
		}
		var list struct {
			Data []map[string]any `json:"data"`
		}
		e.want(e.req(http.MethodGet, sbase+"?target_kind=suite", nil, tok), http.StatusOK, &list)
		if len(list.Data) != 1 {
			t.Fatalf("schedules list %d", len(list.Data))
		}
		e.want(e.req(http.MethodDelete, sbase+"/"+ss.ID, nil, tok), http.StatusNoContent, nil)
		e.problem(e.req(http.MethodGet, sbase+"/"+ss.ID, nil, tok), http.StatusNotFound, "not_found")
	})

	t.Run("viewer cannot write, delete suite keeps runs", func(t *testing.T) {
		viewer := e.person(slug("viewer")+"@example.com", "Viewer")
		e.member(tn, viewer, tenant.RoleViewer)
		vtok := e.token(viewer, tn)
		e.problem(e.req(http.MethodPost, base+"/suites/"+created.ID+":launch", map[string]any{}, vtok), http.StatusForbidden, "forbidden")
		e.want(e.req(http.MethodGet, base+"/suites/"+created.ID, nil, vtok), http.StatusOK, nil)
		// The test is referenced by the suite and the nightly schedule.
		e.problem(e.req(http.MethodDelete, base+"/tests/"+testID, nil, tok), http.StatusConflict, "conflict")
		e.want(e.req(http.MethodDelete, base+"/suites/"+created.ID, nil, tok), http.StatusNoContent, nil)
		e.problem(e.req(http.MethodGet, base+"/suites/"+created.ID, nil, tok), http.StatusNotFound, "not_found")
		e.want(e.req(http.MethodGet, base+"/suite-runs/"+sr.ID, nil, tok), http.StatusOK, nil)
		e.problem(e.req(http.MethodDelete, base+"/tests/"+testID, nil, tok), http.StatusConflict, "conflict")
		var schedules struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		e.want(e.req(http.MethodGet, base+"/schedules", nil, tok), http.StatusOK, &schedules)
		for _, s := range schedules.Data {
			e.want(e.req(http.MethodDelete, base+"/schedules/"+s.ID, nil, tok), http.StatusNoContent, nil)
		}
		e.want(e.req(http.MethodDelete, base+"/tests/"+testID, nil, tok), http.StatusNoContent, nil)
	})
}
