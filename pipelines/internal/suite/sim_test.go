package suite

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"

	"github.com/graphene-ci/pipeline/pkg/pipelinetest"
	"github.com/graphene-ci/pipeline/pkg/wire"

	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/events"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// The suite is simulated with the SERVER's child-run contract: start-child
// records the child, await-child returns the cell's result after a virtual
// delay or fails it. The child pipeline itself is covered by internal/run.

type childSim struct {
	w        *pipelinetest.World
	started  []wire.StartChildRunRequest
	results  map[string]spec.Result
	failures map[string]error
	delays   map[string]time.Duration
	events   []string
	running  int
	peak     int
}

func newChildSim(t *testing.T) *childSim {
	t.Helper()
	var suite testsuite.WorkflowTestSuite
	w := pipelinetest.Install(t, suite.NewTestWorkflowEnvironment())
	s := &childSim{w: w, results: map[string]spec.Result{}, failures: map[string]error{}, delays: map[string]time.Duration{}}
	pipelinetest.Handle1(w, wire.StartChildRunActivity, func(_ workflow.Context, req wire.StartChildRunRequest) (any, error) {
		s.started = append(s.started, req)
		return nil, nil
	})
	pipelinetest.Handle1(w, wire.AwaitChildRunActivity, func(ctx workflow.Context, req wire.AwaitChildRunRequest) (any, error) {
		s.running++
		if s.running > s.peak {
			s.peak = s.running
		}
		defer func() { s.running-- }()
		if err := workflow.Sleep(ctx, s.delays[req.RunId]); err != nil {
			return nil, err
		}
		if err := s.failures[req.RunId]; err != nil {
			return nil, err
		}
		return s.results[req.RunId], nil
	})
	w.Handle(events.ActivityName, func(_ workflow.Context, args []any) (any, error) {
		var req struct {
			Name string `json:"name"`
		}
		if err := pipelinetest.Decode(args, 0, &req); err != nil {
			return nil, err
		}
		s.events = append(s.events, req.Name)
		return nil, nil
	})
	return s
}

const suiteID = "8f1c3f2a-0000-4000-8000-000000000001"

func cell(id string) spec.SuiteCell {
	return spec.SuiteCell{ID: id, RunSpec: spec.Run{
		RunID: uuid.NewSHA1(uuid.Nil, []byte(id)).String(), Tenant: "acme",
		Provider: spec.Provider{Kind: spec.ProviderYandex, Settings: json.RawMessage(`{"cloud_id":"b1glku4lgd6gabcdefgh","folder_id":"b1gia87mbaomkfvsleds","network":{"kind":"create"}}`), CredentialsSecret: "yc", ProviderConfigName: "t-acme"},
		Machines: []spec.Machine{{Name: "runner", Role: "runner", CPU: 2, MemoryGB: 4, Image: "ubuntu", Location: "ru-central1-d", InstanceType: "standard-v3"}},
		Workload: spec.Workload{RunnerRole: "runner", StroppyImage: "stroppy:6", DriverType: "noop", URL: "noop://localhost", Segments: []json.RawMessage{json.RawMessage(`{"name":"test","workload":{"script":"simple"},"run":{"executor":"shared-iterations","iterations":1}}`)}},
	}}
}

func TestSimulatedSuiteFanOut(t *testing.T) {
	s := newChildSim(t)
	wf := pipelinetest.Workflow(s.w, PipelineID, Run)
	parent := "test-" + PipelineID
	for _, c := range []string{"a", "b", "c"} {
		s.results[parent+"-"+c] = spec.Result{Summary: spec.Summary{TPS: 100}}
		s.delays[parent+"-"+c] = 10 * time.Minute
	}
	s.w.Env.ExecuteWorkflow(wf, spec.Suite{
		SuiteRunID: suiteID, Tenant: "acme", Cells: []spec.SuiteCell{cell("a"), cell("b"), cell("c")}, Concurrency: 2,
		Defaults: spec.SuiteDefaults{Labels: map[string]string{"nightly": "1"}},
	})
	require.NoError(t, s.w.Env.GetWorkflowError())
	var result spec.SuiteResult
	require.NoError(t, s.w.Env.GetWorkflowResult(&result))
	require.Equal(t, 3, result.Total)
	require.Equal(t, 3, result.Done)
	require.Equal(t, 0, result.Fail)
	require.Len(t, s.started, 3)
	// Every child runs the run pipeline with the cell's RunSpec and the
	// suite labels; at most two at once.
	require.Equal(t, "stroppy-run", s.started[0].Pipeline)
	require.Equal(t, suiteID, s.started[0].Labels["stroppy-suite-run"])
	require.Equal(t, "1", s.started[0].Labels["nightly"])
	var child spec.Run
	require.NoError(t, json.Unmarshal(s.started[0].Params, &child))
	require.Equal(t, cell("a").RunSpec.RunID, child.RunID)
	require.Equal(t, 3, count(s.events, CellFinished))
	require.Equal(t, "completed", result.Cells[2].Status)
	require.Equal(t, 100.0, result.Cells[2].Result.Summary.TPS)
}

// TestSimulatedSuiteConcurrencyBound pins the contract of RunAll's
// concurrency: at most N children RUNNING at once (pipeline v0.2.1; v0.2.0
// released the semaphore right after start-child).
func TestSimulatedSuiteConcurrencyBound(t *testing.T) {
	s := newChildSim(t)
	wf := pipelinetest.Workflow(s.w, PipelineID, Run)
	parent := "test-" + PipelineID
	for _, c := range []string{"a", "b", "c"} {
		s.results[parent+"-"+c] = spec.Result{}
		s.delays[parent+"-"+c] = 10 * time.Minute
	}
	s.w.Env.ExecuteWorkflow(wf, spec.Suite{SuiteRunID: suiteID, Tenant: "acme", Cells: []spec.SuiteCell{cell("a"), cell("b"), cell("c")}, Concurrency: 2})
	require.NoError(t, s.w.Env.GetWorkflowError())
	require.Equal(t, 2, s.peak, "at most two children running at once")
}

func TestSimulatedSuiteStopsAtFirstFailure(t *testing.T) {
	s := newChildSim(t)
	wf := pipelinetest.Workflow(s.w, PipelineID, Run)
	parent := "test-" + PipelineID
	s.failures[parent+"-a"] = errors.New("segment load failed")
	s.results[parent+"-b"] = spec.Result{}
	s.w.Env.ExecuteWorkflow(wf, spec.Suite{SuiteRunID: suiteID, Tenant: "acme", Cells: []spec.SuiteCell{cell("a"), cell("b")}})
	require.ErrorContains(t, s.w.Env.GetWorkflowError(), "1 of 2 cells failed")
	require.Contains(t, s.events, CellFailed)
	require.NotContains(t, s.events, CellFinished, "the second cell is not read after the first failure")
}

func TestSimulatedSuiteContinuesOnFailure(t *testing.T) {
	s := newChildSim(t)
	wf := pipelinetest.Workflow(s.w, PipelineID, Run)
	parent := "test-" + PipelineID
	s.failures[parent+"-a"] = errors.New("quota")
	s.results[parent+"-b"] = spec.Result{Summary: spec.Summary{TPS: 5}}
	s.w.Env.ExecuteWorkflow(wf, spec.Suite{
		SuiteRunID: suiteID, Tenant: "acme", Cells: []spec.SuiteCell{cell("a"), cell("b")},
		Defaults: spec.SuiteDefaults{ContinueOnFailure: true},
	})
	require.NoError(t, s.w.Env.GetWorkflowError())
	var result spec.SuiteResult
	require.NoError(t, s.w.Env.GetWorkflowResult(&result))
	require.Equal(t, 1, result.Fail)
	require.Equal(t, 1, result.Done)
	require.Equal(t, "failed", result.Cells[0].Status)
	require.Contains(t, result.Cells[0].Error, "quota")
	require.Equal(t, "completed", result.Cells[1].Status)
}

func TestSimulatedSuiteRejectsEmpty(t *testing.T) {
	s := newChildSim(t)
	wf := pipelinetest.Workflow(s.w, PipelineID, Run)
	s.w.Env.ExecuteWorkflow(wf, spec.Suite{SuiteRunID: suiteID, Tenant: "acme"})
	require.ErrorContains(t, s.w.Env.GetWorkflowError(), "cells")
	require.Empty(t, s.started)
}

func count(list []string, name string) int {
	n := 0
	for _, s := range list {
		if s == name {
			n++
		}
	}
	return n
}

func TestSimulatedSuiteCancellation(t *testing.T) {
	for _, continueOnFailure := range []bool{false, true} {
		name := "stop on failure"
		if continueOnFailure {
			name = "continue on failure"
		}
		t.Run(name, func(t *testing.T) {
			s := newChildSim(t)
			wf := pipelinetest.Workflow(s.w, PipelineID, Run)
			s.delays["test-"+PipelineID+"-a"] = 10 * time.Minute
			s.delays["test-"+PipelineID+"-b"] = 10 * time.Minute
			s.w.Env.RegisterDelayedCallback(s.w.Env.CancelWorkflow, time.Minute)
			s.w.Env.ExecuteWorkflow(wf, spec.Suite{SuiteRunID: suiteID, Tenant: "acme", Cells: []spec.SuiteCell{cell("a"), cell("b")}, Concurrency: 2, Defaults: spec.SuiteDefaults{ContinueOnFailure: continueOnFailure}})
			require.True(t, temporal.IsCanceledError(s.w.Env.GetWorkflowError()), "operator cancel must remain canceled: %v", s.w.Env.GetWorkflowError())
			require.Len(t, s.started, 2)
			require.NotContains(t, s.events, CellFailed, "operator cancel is not a failed cell")
		})
	}
}
