package run

import (
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/graphene-ci/pipeline/pkg/id"
	"github.com/graphene-ci/pipeline/pkg/pipeline"
	"github.com/graphene-ci/pipeline/pkg/pipelinetest"

	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/activities"
	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/events"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// resumeRun is simRun continued on the stand of an earlier run.
func resumeRun() (spec.Run, string) {
	run := simRun()
	from := "0b9d2c11-0000-4000-8000-000000000002"
	run.Keep = spec.Duration(time.Hour) // ignored: the stand is not this run's
	run.Workload.Baseline = nil
	run.Resume = &spec.Resume{FromRunID: from, Machines: map[string]spec.ResumeMachine{
		"db-1": {PrivateIP: "10.130.0.10"}, "runner-1": {PrivateIP: "10.130.0.20", PublicIP: "84.0.0.1"},
	}}
	return run, from
}

func TestSimulatedRunResumesOnKeptStand(t *testing.T) {
	s := newSim(t)
	run, from := resumeRun()
	wf := pipelinetest.Workflow(s.w, PipelineID, Run)
	for _, m := range run.Machines {
		s.w.SeedAgent(id.AgentId(spec.AgentName(from, m.Name)), map[string]string{"machine": m.Name}, pipeline.AgentState{AgentConnected: true})
	}
	runner := id.AgentId(spec.AgentName(from, "runner-1"))
	var segmentReq activities.RunSegmentRequest
	s.w.OnAgentActivity(runner, activities.NameRunSegment, mock.Anything, mock.MatchedBy(func(req activities.RunSegmentRequest) bool {
		segmentReq = req
		return req.Segment.Name == "load"
	})).Return(activities.RunSegmentResult{
		Result: spec.SegmentResult{Name: "load", Status: spec.SegmentCompleted, Metrics: map[string]spec.MetricValue{"iterations_total": {Value: 10}}},
	}, nil).Once()

	s.w.Env.ExecuteWorkflow(wf, run)
	require.NoError(t, s.w.Env.GetWorkflowError())
	s.w.Env.AssertExpectations(t)

	// The stand's addresses, not new machines; no provider config, no deploy.
	require.Equal(t, "postgres://postgres:x@10.130.0.10:5432/postgres?sslmode=disable", segmentReq.URL)
	require.Empty(t, s.configs)
	require.Equal(t, []string{
		events.PhaseStarted, events.MachineReady, events.MachineReady, events.PhaseFinished,
		events.PhaseStarted, events.PhaseFinished,
		events.PhaseStarted, events.SegmentStarted, events.SegmentFinished, events.PhaseFinished,
		events.PhaseStarted, events.ResultPublished, events.PhaseFinished,
	}, s.events)
	require.False(t, s.teardownStarted)
	require.NotContains(t, s.events, events.StandKept)
	// The kept agents are still there, still foreign.
	for _, m := range run.Machines {
		require.True(t, s.w.AgentState(id.AgentId(spec.AgentName(from, m.Name))).AgentConnected)
	}
}

func TestSimulatedRunResumeFailsWhenStandAgentIsGone(t *testing.T) {
	s := newSim(t)
	run, from := resumeRun()
	// Only the runner survived; the db machine's agent is gone.
	s.w.SeedAgent(id.AgentId(spec.AgentName(from, "runner-1")), nil, pipeline.AgentState{AgentConnected: true})

	s.w.Env.ExecuteWorkflow(pipelinetest.Workflow(s.w, PipelineID, Run), run)
	err := s.w.Env.GetWorkflowError()
	require.Error(t, err)
	require.Contains(t, err.Error(), "kept machine db-1")
	require.NotContains(t, s.events, events.SegmentStarted)
}
