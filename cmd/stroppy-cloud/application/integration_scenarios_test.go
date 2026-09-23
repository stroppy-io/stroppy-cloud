//go:build integration

package application

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// overviewView is the run overview as the UI reads it.
type overviewView struct {
	Status string `json:"status"`
	Phase  string `json:"phase"`
	Phases []struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Steps  []struct {
			Title  string `json:"title"`
			Status string `json:"status"`
			Error  string `json:"error"`
		} `json:"steps"`
	} `json:"phases"`
	Machines []struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	} `json:"machines"`
	Components []struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	} `json:"components"`
	WorkloadSegments []struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	} `json:"workload_segments"`
}

type baselineView struct {
	OK       bool   `json:"ok"`
	Error    string `json:"error"`
	Verdicts []struct {
		Check  string `json:"check"`
		Status string `json:"status"`
	} `json:"verdicts"`
}

// baselineOf reads the machine baseline from the run's result.
func baselineOf(t *testing.T, e *e2e, base, tok, id string) *baselineView {
	t.Helper()
	var r struct {
		Result struct {
			Baseline *baselineView `json:"baseline"`
		} `json:"result"`
	}
	e.want(e.req(http.MethodGet, base+"/runs/"+id, nil, tok), http.StatusOK, &r)
	return r.Result.Baseline
}

func (o overviewView) phase(id string) string {
	for _, p := range o.Phases {
		if p.ID == id {
			return p.Status
		}
	}
	return ""
}

func (o overviewView) segment(name string) string {
	for _, s := range o.WorkloadSegments {
		if s.Name == name {
			return s.Status
		}
	}
	return ""
}

// TestE2EScenarios drives the real pipeline through every way a run
// ends — the simulations of pipelines/internal/run, now seen from the
// server: status, reason, phases, segments, timeline, artifacts, logs and
// metrics, as the UI reads them.
func TestE2EScenarios(t *testing.T) {
	e := e2eServer(t)
	base, tok, testID, _ := runFixtureWith(t, e, map[string]any{
		"options": map[string]any{"baseline": map[string]any{"enabled": true, "tiers": []any{"noop"}, "quick": true}},
	}, "0s")

	cases := []struct {
		name string
		sc   simScenario
		// cancel cancels the run once it is held.
		cancel bool
		status string
		reason string
		check  func(t *testing.T, id string, ov overviewView)
	}{
		{name: "completes", status: "completed", check: func(t *testing.T, id string, ov overviewView) {
			for _, p := range []string{"provisioning", "deploying", "workload", "collecting"} {
				if ov.phase(p) != "completed" {
					t.Fatalf("phase %s %q: %+v", p, ov.phase(p), ov.Phases)
				}
			}
			if b := baselineOf(t, e, base, tok, id); b == nil || !b.OK || len(b.Verdicts) == 0 {
				t.Fatalf("baseline %+v", b)
			}
			if n := artifactCount(t, e, base, tok, id); n < 3 {
				t.Fatalf("artifacts %d", n)
			}
		}},
		{name: "segment fails", sc: simScenario{FailSegment: "main"}, status: "failed", reason: "segment main failed", check: func(t *testing.T, _ string, ov overviewView) {
			if ov.phase("workload") != "failed" || ov.segment("main") != "failed" {
				t.Fatalf("overview %+v", ov)
			}
		}},
		{name: "segment activity lost", sc: simScenario{LoseSegment: "main"}, status: "failed", reason: "heartbeat timeout", check: func(t *testing.T, _ string, ov overviewView) {
			if ov.segment("main") != "failed" {
				t.Fatalf("overview %+v", ov)
			}
		}},
		{name: "baseline fails, run goes on", sc: simScenario{FailBaseline: true}, status: "completed", check: func(t *testing.T, id string, ov overviewView) {
			if b := baselineOf(t, e, base, tok, id); b == nil || b.OK || !strings.Contains(b.Error, "pg-noop") || ov.segment("main") != "completed" {
				t.Fatalf("baseline %+v segments %+v", b, ov.WorkloadSegments)
			}
		}},
		{name: "database never healthy", sc: simScenario{FailHealthcheck: "db-1-postgres"}, status: "failed", reason: "unhealthy", check: func(t *testing.T, _ string, ov overviewView) {
			if ov.phase("deploying") != "failed" || ov.phase("workload") == "completed" {
				t.Fatalf("phases %+v", ov.Phases)
			}
		}},
		{name: "docker install fails", sc: simScenario{FailDockerInstall: true}, status: "failed", reason: "package repository unavailable", check: func(t *testing.T, _ string, ov overviewView) {
			if ov.phase("deploying") != "failed" {
				t.Fatalf("phases %+v", ov.Phases)
			}
		}},
		{name: "provider credentials rejected", sc: simScenario{FailProviderConfig: true}, status: "failed", reason: "credentials rejected", check: func(t *testing.T, _ string, ov overviewView) {
			if ov.phase("provisioning") != "failed" || len(ov.Machines) != 0 && ov.Machines[0].Status == "ready" {
				t.Fatalf("overview %+v", ov)
			}
		}},
		{name: "machine never converges", sc: simScenario{FailMachine: "db-1"}, status: "failed", reason: "Quota limit", check: func(t *testing.T, _ string, ov overviewView) {
			if ov.phase("provisioning") != "failed" {
				t.Fatalf("phases %+v", ov.Phases)
			}
		}},
		{name: "agents never connect", sc: simScenario{AgentsNeverConnect: true}, status: "failed", reason: "agent", check: func(t *testing.T, _ string, ov overviewView) {
			if ov.phase("provisioning") != "failed" {
				t.Fatalf("phases %+v", ov.Phases)
			}
		}},
		{name: "artifact uploads fail, run completes", sc: simScenario{FailArtifacts: true}, status: "completed", check: func(t *testing.T, id string, _ overviewView) {
			if n := artifactCount(t, e, base, tok, id); n != 0 {
				t.Fatalf("artifacts %d", n)
			}
		}},
		{name: "cancelled while provisioning", sc: simScenario{Hold: "phase.started"}, cancel: true, status: "cancelled", check: func(t *testing.T, _ string, ov overviewView) {
			if ov.phase("provisioning") != "cancelled" || ov.phase("workload") == "completed" {
				t.Fatalf("phases %+v", ov.Phases)
			}
		}},
		{name: "cancelled during the workload", sc: simScenario{Hold: "segment.started"}, cancel: true, status: "cancelled", check: func(t *testing.T, _ string, ov overviewView) {
			if ov.segment("main") == "completed" || ov.phase("provisioning") != "completed" {
				t.Fatalf("overview %+v", ov)
			}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e.graphene.mu.Lock()
			e.graphene.defaultScenario = c.sc
			e.graphene.mu.Unlock()
			var r runView
			e.want(e.req(http.MethodPost, base+"/tests/"+testID+":launch", map[string]any{"name": c.name}, tok), http.StatusCreated, &r)
			if c.cancel {
				eventually(t, 10*time.Second, func() bool {
					e.app.services.Projector.Tick(e.ctx)
					e.want(e.req(http.MethodGet, base+"/runs/"+r.ID, nil, tok), http.StatusOK, &r)
					return r.Status == "running"
				})
				e.want(e.req(http.MethodPost, base+"/runs/"+r.ID+":cancel", nil, tok), http.StatusOK, nil)
			}
			r = finishRun(t, e, base, tok, r.ID)
			if r.Status != c.status || !strings.Contains(r.StatusReason, c.reason) {
				t.Fatalf("run %s %q, want %s %q", r.Status, r.StatusReason, c.status, c.reason)
			}
			var ov overviewView
			e.want(e.req(http.MethodGet, base+"/runs/"+r.ID+"/overview", nil, tok), http.StatusOK, &ov)
			if ov.Status != c.status || ov.Phase != "done" {
				t.Fatalf("overview %+v", ov)
			}
			// The timeline tells the same story.
			var events struct {
				Data []struct {
					Kind  string `json:"kind"`
					Title string `json:"title"`
					Error string `json:"error"`
				} `json:"data"`
			}
			e.want(e.req(http.MethodGet, base+"/runs/"+r.ID+"/events?limit=200", nil, tok), http.StatusOK, &events)
			terminal := map[string]string{"completed": "run-completed", "failed": "run-failed", "cancelled": "run-canceled"}[c.status]
			if len(events.Data) == 0 || events.Data[0].Kind != "run-started" || events.Data[len(events.Data)-1].Kind != terminal {
				t.Fatalf("timeline %+v", events.Data)
			}
			// Whatever happened, the observability endpoints answer.
			for _, path := range []string{"/logs?limit=5", "/logs:facets", "/metrics", "/tree", "/artifacts"} {
				if res := e.req(http.MethodGet, base+"/runs/"+r.ID+path, nil, tok); res.Status != http.StatusOK {
					t.Fatalf("%s: %d %s", path, res.Status, res.Body)
				}
			}
			c.check(t, r.ID, ov)
		})
	}
}

func artifactCount(t *testing.T, e *e2e, base, tok, id string) int {
	t.Helper()
	var arts struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	e.want(e.req(http.MethodGet, base+"/runs/"+id+"/artifacts", nil, tok), http.StatusOK, &arts)
	return len(arts.Data)
}
