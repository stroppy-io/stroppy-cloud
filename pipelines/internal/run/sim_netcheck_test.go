package run

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/graphene-ci/pipeline/pkg/ref"

	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/activities"
	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/events"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// netRun is postgres with its exporter on db-1 and the runner: the runner
// dials the database and the exporter; db-1 has nothing to dial.
func netRun() spec.Run {
	run := simRun()
	run.HostPrep, run.Workload.Baseline = nil, nil
	run.Containers[0].Kind, run.Containers[0].Files = spec.ContainerKindDatabase, nil
	run.Containers = append(run.Containers, spec.Container{
		Name: "postgres-exporter", Role: "db", Kind: spec.ContainerKindExporter, Machine: "db-1", Image: "exporter:1",
		Ports: []spec.Port{{Container: 9187, Host: 9187}},
	})
	return run
}

func netSim(t *testing.T) (*sim, spec.Run, func(spec.Run)) {
	t.Helper()
	s := newSim(t)
	run := netRun()
	wf := pipelinetestWorkflow(s)
	s.converge(t, run, map[string]string{"db-1": "10.130.0.10", "runner-1": "10.130.0.20"})
	db, runner := s.agent(run, "db-1"), s.agent(run, "runner-1")
	s.w.OnAgentActivity(db, activities.NamePullImage, mock.Anything, mock.Anything).
		Return(func(_ context.Context, req activities.PullImageRequest) (activities.PullImageResult, error) {
			return activities.PullImageResult{Image: req.Image}, nil
		}).Maybe()
	s.w.OnAgentActivity(db, activities.NameWaitHealthy, mock.Anything, mock.Anything).Return(activities.WaitHealthyResult{Attempts: 1}, nil).Maybe()
	s.w.OnAgentActivity(runner, activities.NameRunSegment, mock.Anything, mock.Anything).Return(completed("load", 100), nil).Maybe()
	s.w.File(runner, "/ws/stroppy/load/stroppy.log", []byte("log\n"))
	for _, f := range []string{"probe", "reach"} {
		s.w.File(runner, "/ws/netdiag/"+f+"-runner-1.txt", []byte("$ ip route\n"))
		s.w.File(db, "/ws/netdiag/"+f+"-db-1.txt", []byte("$ ss -ltnp\n"))
	}
	execute := func(run spec.Run) { s.w.Env.ExecuteWorkflow(wf, run) }
	return s, run, execute
}

func pairsOf(t *testing.T, payload map[string]any) []map[string]any {
	t.Helper()
	raw, ok := payload["pairs"].([]any)
	require.True(t, ok, "pairs: %#v", payload["pairs"])
	out := make([]map[string]any, 0, len(raw))
	for _, p := range raw {
		out = append(out, p.(map[string]any))
	}
	return out
}

func TestSimulatedNetCheckAllReachable(t *testing.T) {
	s, run, execute := netSim(t)
	var probed []activities.NetTarget
	s.probe = func(machine string, target activities.NetTarget) activities.NetPairResult {
		require.Equal(t, "runner-1", machine, "db-1 has no peer port to dial")
		probed = append(probed, target)
		return activities.NetPairResult{NetTarget: target, Outcome: activities.NetOK, LatencyMs: 0.3, Attempts: 1}
	}
	execute(run)
	require.NoError(t, s.w.Env.GetWorkflowError())
	require.Equal(t, []activities.NetTarget{
		{Machine: "db-1", Address: "10.130.0.10:5432", Port: 5432, Container: "postgres", Critical: true},
		{Machine: "db-1", Address: "10.130.0.10:9187", Port: 9187, Container: "postgres-exporter"},
	}, probed, "the endpoint is the database port, not a second target")
	require.Len(t, s.network, 1)
	require.EqualValues(t, 2, s.network[0]["total"])
	require.EqualValues(t, 0, s.network[0]["failed"])
	require.Empty(t, s.network[0]["snapshots"])
	var result spec.Result
	require.NoError(t, s.w.Env.GetWorkflowResult(&result))
	for _, a := range result.Artifacts {
		require.NotContains(t, a, "netdiag")
	}
	s.w.AssertNoLeaks(t)
}

func TestSimulatedNetCheckCriticalPathFailsBeforeWorkload(t *testing.T) {
	s, run, execute := netSim(t)
	s.probe = func(_ string, target activities.NetTarget) activities.NetPairResult {
		if target.Port == 5432 {
			return activities.NetPairResult{NetTarget: target, Outcome: activities.NetTimeout, LatencyMs: 3000, Attempts: 4, Error: "dial tcp 10.130.0.10:5432: i/o timeout"}
		}
		return activities.NetPairResult{NetTarget: target, Outcome: activities.NetOK, Attempts: 1}
	}
	execute(run)
	err := s.w.Env.GetWorkflowError()
	require.ErrorContains(t, err, "connectivity: db-1:5432 unreachable from runner-1 (timeout)")
	require.NotContains(t, s.events, events.SegmentStarted, "the workload never starts")
	require.Contains(t, s.events, events.PhaseFailed)

	// The snapshots of both ends were taken and kept before the run failed.
	require.Len(t, s.network, 1)
	p := s.network[0]
	require.EqualValues(t, 1, p["failed"])
	require.EqualValues(t, 1, p["critical"])
	pairs := pairsOf(t, p)
	require.Equal(t, "timeout", pairs[0]["outcome"], "failed pairs first")
	require.Equal(t, "db-1", pairs[0]["to"])
	snaps := p["snapshots"].([]any)
	require.Len(t, snaps, 2)
	for _, name := range []string{"artifact/8f1c3f2a-netdiag-probe-db-1", "artifact/8f1c3f2a-netdiag-probe-runner-1"} {
		a, ok := s.w.Resource(ref.OwnerRef(name))
		require.True(t, ok, name)
		require.Equal(t, "ready", a.Phase)
	}
	// The partial result names them, so the server can serve them.
	var appErr interface{ HasDetails() bool }
	require.True(t, errors.As(err, &appErr) && appErr.HasDetails(), "the partial result rides in the failure")
	require.Equal(t, "failure", s.w.Outcome(runOwner()))
	s.w.AssertNoLeaks(t)
}

func TestSimulatedNetCheckExporterOnlyWarns(t *testing.T) {
	s, run, execute := netSim(t)
	s.probe = func(_ string, target activities.NetTarget) activities.NetPairResult {
		if target.Port == 9187 {
			return activities.NetPairResult{NetTarget: target, Outcome: activities.NetRefused, Attempts: 4, Error: "connect: connection refused"}
		}
		return activities.NetPairResult{NetTarget: target, Outcome: activities.NetOK, Attempts: 1}
	}
	execute(run)
	require.NoError(t, s.w.Env.GetWorkflowError())
	require.Contains(t, s.events, events.SegmentFinished)
	require.EqualValues(t, 1, s.network[0]["failed"])
	require.EqualValues(t, 0, s.network[0]["critical"])
	var result spec.Result
	require.NoError(t, s.w.Env.GetWorkflowResult(&result))
	require.Contains(t, result.Artifacts, "artifact/8f1c3f2a-netdiag-probe-runner-1")
	require.Contains(t, result.Artifacts, "artifact/8f1c3f2a-netdiag-probe-db-1")
	s.w.AssertNoLeaks(t)
}

func TestSimulatedNetCheckFailedReachabilitySnapshots(t *testing.T) {
	s, run, execute := netSim(t)
	s.reachErr = errors.New("database endpoint 10.130.0.10:5432 is unreachable from runner after 150 attempts: context deadline exceeded (last dial error: dial tcp 10.130.0.10:5432: connect: connection refused)")
	execute(run)
	require.ErrorContains(t, s.w.Env.GetWorkflowError(), "database connectivity")
	require.NotContains(t, s.events, events.SegmentStarted)
	require.Len(t, s.network, 2, "the deploy check, then the failed wait")
	last := s.network[1]
	require.Equal(t, netStageWorkload, last["stage"])
	require.EqualValues(t, 1, last["failed"])
	pairs := pairsOf(t, last)
	require.Equal(t, "refused", pairs[0]["outcome"])
	require.Equal(t, "runner-1", pairs[0]["from"])
	require.Len(t, last["snapshots"].([]any), 2)
	_, ok := s.w.Resource(ref.OwnerRef("artifact/8f1c3f2a-netdiag-reach-db-1"))
	require.True(t, ok)
	s.w.AssertNoLeaks(t)
}

func TestSimulatedNetCheckProbeLossOnlyWarns(t *testing.T) {
	s, run, execute := netSim(t)
	// The probe activity itself is lost: its paths are unknown, the run goes on.
	s.probeErr = errors.New("agent lost")
	execute(run)
	require.NoError(t, s.w.Env.GetWorkflowError())
	require.EqualValues(t, 2, s.network[0]["failed"])
	require.EqualValues(t, 0, s.network[0]["critical"])
}
