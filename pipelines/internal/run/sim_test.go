package run

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"

	"github.com/graphene-ci/library/docker/dockertest"
	"github.com/graphene-ci/library/k8s/k8stest"
	"github.com/graphene-ci/pipeline/pkg/id"
	"github.com/graphene-ci/pipeline/pkg/pipelinetest"
	"github.com/graphene-ci/pipeline/pkg/ref"

	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/activities"
	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/events"
	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/provision"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// Simulated run of the whole stroppy-run workflow on Graphene's pipelinetest:
// Crossplane objects are k8stest fixtures that turn Ready in virtual time,
// containers are dockertest records, machine activities are mocks per agent.
// No cloud, no docker daemon, no Temporal server.

const (
	ycInstanceKind   = "k8s.compute.yandex-cloud.jet.crossplane.io.v1alpha1.Instance"
	ycGatewayKind    = "k8s.vpc.yandex-cloud.jet.crossplane.io.v1alpha1.Gateway"
	ycRouteTableKind = "k8s.vpc.yandex-cloud.jet.crossplane.io.v1alpha1.RouteTable"
	ycNetworkKind    = "k8s.vpc.yandex-cloud.jet.crossplane.io.v1alpha1.Network"
	ycSubnetKind     = "k8s.vpc.yandex-cloud.jet.crossplane.io.v1alpha1.Subnet"
	ycSGKind         = "k8s.vpc.yandex-cloud.jet.crossplane.io.v1alpha1.SecurityGroup"
	ycDiskKind       = "k8s.compute.yandex-cloud.jet.crossplane.io.v1alpha1.Disk"
)

// simRun is the RunSpec of the simulated run: postgres single on yandex.
func simRun() spec.Run {
	return spec.Run{
		RunID: "8f1c3f2a-0000-4000-8000-000000000001", Tenant: "acme",
		Provider: spec.Provider{
			Kind: spec.ProviderYandex, Settings: json.RawMessage(`{"cloud_id":"b1glku4lgd6gabcdefgh","folder_id":"b1gia87mbaomkfvsleds","zone":"ru-central1-d","network":{"kind":"create"}}`),
			CredentialsSecret: "yc-sa-key", ProviderConfigName: "t-acme",
		},
		Network: spec.Network{CIDR: "10.130.0.0/24"},
		Machines: []spec.Machine{
			{
				Name: "db-1", Role: "db", CPU: 4, MemoryGB: 8, Image: "ubuntu-2404-lts", Location: "ru-central1-d", InstanceType: "standard-v3",
				Disks: []spec.Disk{{Name: "data", GB: 50, Type: "network-ssd", Mount: "/data"}},
			},
			{Name: "runner-1", Role: "runner", CPU: 2, MemoryGB: 4, Image: "ubuntu-2404-lts", Location: "ru-central1-d", InstanceType: "standard-v3"},
		},
		Containers: []spec.Container{{
			Name: "postgres", Role: "db", Machine: "db-1", Image: "postgres:17",
			Env:         map[string]string{"POSTGRES_PASSWORD": "x"},
			Ports:       []spec.Port{{Container: 5432, Host: 5432}},
			Files:       []spec.File{{Path: "/etc/postgresql/postgresql.conf", Content: "shared_buffers = 2GB\n"}},
			Healthcheck: &spec.Healthcheck{Cmd: []string{"pg_isready"}, Interval: spec.Duration(5 * time.Second), Retries: 10},
		}},
		HostPrep: []spec.HostPrep{{Role: "db", Kind: spec.HostPrepSysctl, Content: "vm.swappiness=1\n"}},
		Flows:    []spec.Flow{{FromRole: "runner", ToRole: "db", Protocol: "tcp", Port: 5432}},
		Workload: spec.Workload{
			RunnerRole: "runner", StroppyImage: "ghcr.io/stroppy-io/stroppy:v6.0.0.62",
			DriverType: "postgres", URL: "postgres://postgres:x@${ip:role:db}:5432/postgres?sslmode=disable",
			Segments: []json.RawMessage{
				json.RawMessage(`{"name":"load","workload":{"script":"tpcc/tx","scale_factor":2},"run":{"executor":"constant-vus","vus":4,"duration":"30s"}}`),
			},
			Baseline: &spec.Baseline{Enabled: true, Tiers: []string{"noop"}, Quick: true},
		},
		Observability: spec.Observability{OTLPEndpoint: "http://otel.stroppy.io", Labels: map[string]string{"stroppy_run_id": "x"}},
	}
}

// sim wires the simulators and the run-queue contracts of stroppy-run.
type sim struct {
	w       *pipelinetest.World
	objects *k8stest.Objects
	events  []string
	configs []activities.EnsureProviderConfigRequest
	// configErr makes ensure-config fail (bad credentials).
	configErr error
	// degraded is what result.published reported.
	degraded bool
}

func pipelinetestWorkflow(s *sim) func(workflow.Context, spec.Run) (spec.Result, error) {
	return pipelinetest.Workflow(s.w, PipelineID, Run)
}

func newSim(t *testing.T) *sim {
	t.Helper()
	var suite testsuite.WorkflowTestSuite
	w := pipelinetest.Install(t, suite.NewTestWorkflowEnvironment())
	s := &sim{w: w, objects: k8stest.Install(w)}
	dockertest.Install(w, "28.5.2")
	// Automatic disk preparation is infrastructure plumbing; explicit host steps
	// remain separately matched by individual deployment tests.
	// Run-queue contracts of the pipeline itself.
	pipelinetest.Handle1(w, activities.NameResolveYandexImages, func(_ workflow.Context, req activities.ResolveYandexImagesRequest) (activities.ResolveYandexImagesResult, error) {
		images := make(map[string]string, len(req.Images))
		for _, family := range req.Images {
			images[family] = "fd8d6s0blceqbto92ss8"
		}
		return activities.ResolveYandexImagesResult{Images: images}, nil
	})
	pipelinetest.Handle1(w, activities.NameEnsureProviderConfig, func(_ workflow.Context, req activities.EnsureProviderConfigRequest) (activities.EnsureProviderConfigResult, error) {
		s.configs = append(s.configs, req)
		if s.configErr != nil {
			return activities.EnsureProviderConfigResult{}, s.configErr
		}
		return activities.EnsureProviderConfigResult{SecretName: req.Name, Applied: true}, nil
	})
	w.Handle(events.ActivityName, func(_ workflow.Context, args []any) (any, error) {
		var req struct {
			Name    string         `json:"name"`
			Payload map[string]any `json:"payload"`
		}
		if err := pipelinetest.Decode(args, 0, &req); err != nil {
			return nil, err
		}
		s.events = append(s.events, req.Name)
		if req.Name == events.ResultPublished {
			s.degraded, _ = req.Payload["degraded"].(bool)
		}
		return nil, nil
	})
	return s
}

// xpReadyStatus is the observed status of a converged crossplane object.
func xpReadyStatus(extra map[string]any) map[string]any {
	at := map[string]any{}
	for k, v := range extra {
		at[k] = v
	}
	return map[string]any{"status": map[string]any{
		"conditions": []any{map[string]any{"type": "Ready", "status": "True", "reason": "Available"}},
		"atProvider": at,
	}}
}

// converge makes every crossplane object of the run Ready in virtual time
// and connects the agents once their VMs run.
func (s *sim) converge(t *testing.T, run spec.Run, ips map[string]string) {
	t.Helper()
	names := provision.NewNames(run.Tenant, run.RunID)
	if spec.NeedsPrivateEgress(run) {
		require.NoError(t, s.objects.After(3*time.Second, ref.OwnerRef(ycGatewayKind+"/"+names.Gateway()), xpReadyStatus(map[string]any{"id": "nat"})))
		require.NoError(t, s.objects.After(4*time.Second, ref.OwnerRef(ycRouteTableKind+"/"+names.RouteTable()), xpReadyStatus(map[string]any{"id": "routes"})))
	}
	require.NoError(t, s.objects.After(2*time.Second, ref.OwnerRef(ycNetworkKind+"/"+names.Network()), xpReadyStatus(map[string]any{"id": "enp-net"})))
	require.NoError(t, s.objects.After(4*time.Second, ref.OwnerRef(ycSubnetKind+"/"+names.Subnet()), xpReadyStatus(map[string]any{"id": "e9b-subnet"})))
	require.NoError(t, s.objects.After(5*time.Second, ref.OwnerRef(ycSGKind+"/"+names.SecurityGroup()), xpReadyStatus(map[string]any{"id": "enp-sg"})))
	for _, m := range run.Machines {
		s.w.OnAgentActivity(s.agent(run, m.Name), activities.NameHostPrep, mock.Anything, mock.MatchedBy(func(req activities.HostPrepRequest) bool {
			return req.Kind == spec.HostPrepScript && strings.Contains(req.Content, "/dev/disk/by-id/virtio-")
		})).Return(activities.HostPrepResult{}, nil).Maybe()
		for _, d := range m.Disks {
			require.NoError(t, s.objects.After(6*time.Second, ref.OwnerRef(ycDiskKind+"/"+names.Disk(m.Name, d.Name)), xpReadyStatus(map[string]any{"id": "fhm-disk-" + m.Name})))
		}
		require.NoError(t, s.objects.After(30*time.Second, ref.OwnerRef(ycInstanceKind+"/"+names.Machine(m.Name)), xpReadyStatus(map[string]any{
			"id": "fhm-" + m.Name, "status": "running",
			"networkInterface": []any{map[string]any{"ipAddress": ips[m.Name], "natIpAddress": "84.0.0.1"}},
		})))
		s.w.ConnectAfter(id.AgentId(agentName(run, m)), 90*time.Second)
	}
}

func (s *sim) agent(run spec.Run, machine string) id.AgentId {
	for _, m := range run.Machines {
		if m.Name == machine {
			return id.AgentId(agentName(run, m))
		}
	}
	return ""
}

func TestSimulatedRunPostgresYandex(t *testing.T) {
	s := newSim(t)
	run := simRun()
	wf := pipelinetest.Workflow(s.w, PipelineID, Run)
	s.converge(t, run, map[string]string{"db-1": "10.130.0.10", "runner-1": "10.130.0.20"})
	db, runner := s.agent(run, "db-1"), s.agent(run, "runner-1")

	// db machine: host prep, config files, image pull, healthcheck.
	s.w.OnAgentActivity(db, activities.NameHostPrep, mock.Anything, mock.Anything).Return(activities.HostPrepResult{}, nil).Once()
	s.w.OnAgentActivity(db, activities.NameWriteFiles, mock.Anything, mock.MatchedBy(func(req activities.WriteFilesRequest) bool {
		return req.Dir == filesDir && len(req.Files) == 1 && req.Files[0].Path == "postgres/etc/postgresql/postgresql.conf"
	})).Return(activities.WriteFilesResult{Paths: []string{"/ws/containers/postgres/etc/postgresql/postgresql.conf"}}, nil).Once()
	s.w.OnAgentActivity(db, activities.NamePullImage, mock.Anything, activities.PullImageRequest{Image: "postgres:17"}).
		Run(func(mock.Arguments) { requireDockerReady(t, s.w, db) }).
		Return(activities.PullImageResult{Image: "postgres:17"}, nil).Once()
	s.w.OnAgentActivity(db, activities.NameWaitHealthy, mock.Anything, mock.MatchedBy(func(req activities.WaitHealthyRequest) bool {
		return req.Container == run.RunID+"-postgres" && req.Cmd[0] == "pg_isready"
	})).Return(activities.WaitHealthyResult{Attempts: 2}, nil).Once()

	// runner machine: baseline then the segment, both one-shot.
	s.w.OnAgentActivity(runner, activities.NameRunBaseline, mock.Anything, mock.MatchedBy(func(req activities.RunBaselineRequest) bool {
		return req.Image == run.Workload.StroppyImage && req.Baseline.Quick
	})).Return(activities.RunBaselineResult{
		Result:  spec.BaselineResult{OK: true, Verdicts: []spec.BaselineVerdict{{Check: "noop errors", Status: "ok"}}, Report: json.RawMessage(`{"schema":1}`)},
		LogPath: "/ws/stroppy/baseline/stroppy.log",
	}, nil).Once()
	var segmentReq activities.RunSegmentRequest
	s.w.OnAgentActivity(runner, activities.NameRunSegment, mock.Anything, mock.MatchedBy(func(req activities.RunSegmentRequest) bool {
		segmentReq = req
		return req.Segment.Name == "load"
	})).Run(func(mock.Arguments) { requireDockerReady(t, s.w, runner) }).Return(activities.RunSegmentResult{
		Result: spec.SegmentResult{
			Name: "load", Status: spec.SegmentCompleted,
			StartedAt: time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC), FinishedAt: time.Date(2026, 9, 9, 12, 0, 30, 0, time.UTC),
			Metrics: map[string]spec.MetricValue{"iterations_total": {Value: 3000}, "iteration_duration_p99": {Value: 12.5, Unit: "ms"}},
		},
		ConfigPath: "/ws/stroppy/00-load/stroppy-config.json",
		LogPath:    "/ws/stroppy/00-load/stroppy.log",
	}, nil).Once()
	// Artifact bytes the runner "has" on disk.
	s.w.File(runner, "/ws/stroppy/00-load/stroppy-config.json", []byte(`{"version":"1"}`))
	s.w.File(runner, "/ws/stroppy/00-load/stroppy.log", []byte("=== bench summary ===\n"))
	s.w.File(runner, "/ws/stroppy/baseline/stroppy.log", []byte("baseline\n"))

	s.w.Env.ExecuteWorkflow(wf, run)
	require.NoError(t, s.w.Env.GetWorkflowError())
	var result spec.Result
	require.NoError(t, s.w.Env.GetWorkflowResult(&result))
	s.w.Env.AssertExpectations(t)

	// The workload got the expanded URL and the run's workload.
	require.Equal(t, "postgres://postgres:x@10.130.0.10:5432/postgres?sslmode=disable", segmentReq.URL)
	require.Equal(t, "postgres", segmentReq.Workload.DriverType)
	require.Equal(t, run.RunID, segmentReq.RunID)

	// Result: summary from the segment, baseline attached, artifacts uploaded.
	require.Len(t, result.Segments, 1)
	require.Equal(t, spec.SegmentCompleted, result.Segments[0].Status)
	require.Zero(t, result.Summary.TPS, "iterations and activity time do not provide Stroppy TPS")
	require.Equal(t, 12.5, result.Summary.LatencyP99Ms)
	require.NotNil(t, result.Baseline)
	require.True(t, result.Baseline.OK)
	require.ElementsMatch(t, []string{
		"artifact/8f1c3f2a-stroppy-baseline-log", "artifact/8f1c3f2a-stroppy-load-config", "artifact/8f1c3f2a-stroppy-load-log",
	}, result.Artifacts)

	// Provider config was ensured once, for the tenant profile.
	require.Len(t, s.configs, 1)
	require.Equal(t, "t-acme", s.configs[0].Name)
	require.Equal(t, "yc-sa-key", s.configs[0].CredentialsSecret)

	// Milestones in order of the phases.
	require.Equal(t, []string{
		events.PhaseStarted, events.MachineReady, events.MachineReady, events.PhaseFinished,
		events.PhaseStarted, events.ContainerReady, events.PhaseFinished,
		events.PhaseStarted, events.BaselineStarted, events.BaselineFinished, events.SegmentStarted, events.SegmentFinished, events.PhaseFinished,
		events.PhaseStarted, events.ResultPublished, events.PhaseFinished,
	}, s.events)

	// Infrastructure is gone; artifact records survive independently until TTL.
	require.Equal(t, "success", s.w.Outcome(ref.OwnerRef("run/test-"+string(PipelineID))))
	vm, _ := s.w.Resource(ref.OwnerRef(ycInstanceKind + "/" + provision.NewNames(run.Tenant, run.RunID).Machine("runner-1")))
	require.Equal(t, "deleted", vm.Phase)
	for _, name := range result.Artifacts {
		a, ok := s.w.Resource(ref.OwnerRef(name))
		require.True(t, ok)
		require.Equal(t, "ready", a.Phase)
		require.True(t, strings.HasPrefix(string(a.Owner), "stand/"))
		if strings.HasSuffix(name, "-config") {
			require.True(t, a.KeepUntil.IsZero())
		} else {
			require.True(t, s.w.Env.Now().Add(logArtifactRetention).Equal(a.KeepUntil))
		}
	}
	s.w.Advance(logArtifactRetention - time.Hour)
	for _, name := range result.Artifacts {
		a, _ := s.w.Resource(ref.OwnerRef(name))
		require.Equal(t, "ready", a.Phase)
	}
	s.w.Advance(2 * time.Hour)
	for _, name := range result.Artifacts {
		a, _ := s.w.Resource(ref.OwnerRef(name))
		if strings.HasSuffix(name, "-config") {
			require.Equal(t, "ready", a.Phase)
		} else {
			require.Equal(t, "deleted", a.Phase)
		}
	}
	s.w.AssertNoLeaks(t)
}

func TestSimulatedRunKeepsStand(t *testing.T) {
	s := newSim(t)
	run := simRun()
	run.Containers, run.HostPrep, run.Flows = nil, nil, nil
	run.Machines = run.Machines[1:] // runner only
	run.Workload.DriverType, run.Workload.URL = "noop", "noop://localhost"
	run.Workload.Baseline = nil
	run.Workload.Segments = []json.RawMessage{json.RawMessage(`{"name":"smoke","workload":{"script":"simple"},"run":{"executor":"constant-vus","duration":"10s"}}`)}
	run.Keep = spec.Duration(time.Hour)
	wf := pipelinetest.Workflow(s.w, PipelineID, Run)
	s.converge(t, run, map[string]string{"runner-1": "10.130.0.20"})
	runner := s.agent(run, "runner-1")
	s.w.OnAgentActivity(runner, activities.NameRunSegment, mock.Anything, mock.Anything).Return(activities.RunSegmentResult{
		Result: spec.SegmentResult{Name: "smoke", Status: spec.SegmentCompleted, Metrics: map[string]spec.MetricValue{"iterations_total": {Value: 10}}},
	}, nil).Once()

	s.w.Env.ExecuteWorkflow(wf, run)
	require.NoError(t, s.w.Env.GetWorkflowError())
	s.w.Env.AssertExpectations(t)

	names := provision.NewNames(run.Tenant, run.RunID)
	network := ref.OwnerRef(ycNetworkKind + "/" + names.Network())
	vm := ref.OwnerRef(ycInstanceKind + "/" + names.Machine("runner-1"))
	kept, _ := s.w.Resource(network)
	t.Logf("network owner after ToStand: %q phase=%s keepUntil=%s", kept.Owner, kept.Phase, kept.KeepUntil)
	require.True(t, strings.HasPrefix(string(kept.Owner), "stand/"), "network handed to a stand: %+v", kept)
	live, _ := s.w.Resource(vm)
	require.Equal(t, "ready", live.Phase, "the VM survives under the kept network")
	require.Contains(t, s.events, events.StandKept)
	s.w.Advance(2 * time.Hour)
	gone, _ := s.w.Resource(vm)
	require.Equal(t, "deleted", gone.Phase, "the stand TTL reaps the whole tree")
	s.w.AssertNoLeaks(t)
}

func TestSimulatedRunSegmentFailureStopsAndCleans(t *testing.T) {
	s := newSim(t)
	run := simRun()
	run.Containers, run.HostPrep, run.Flows = nil, nil, nil
	run.Machines = run.Machines[1:]
	run.Workload.DriverType, run.Workload.URL, run.Workload.Baseline = "noop", "noop://localhost", nil
	run.Workload.Segments = []json.RawMessage{
		json.RawMessage(`{"name":"first","workload":{"script":"simple"},"run":{"executor":"constant-vus","duration":"10s"}}`),
		json.RawMessage(`{"name":"second","workload":{"script":"simple"},"run":{"executor":"constant-vus","duration":"10s"}}`),
	}
	wf := pipelinetest.Workflow(s.w, PipelineID, Run)
	s.converge(t, run, map[string]string{"runner-1": "10.130.0.20"})
	runner := s.agent(run, "runner-1")
	s.w.OnAgentActivity(runner, activities.NameRunSegment, mock.Anything, mock.MatchedBy(func(req activities.RunSegmentRequest) bool { return req.Segment.Name == "first" })).
		Return(activities.RunSegmentResult{Result: spec.SegmentResult{Name: "first", Status: spec.SegmentFailed, ExitCode: 1, Error: "stroppy exit status 1: relation missing"}}, nil).Once()

	s.w.Env.ExecuteWorkflow(wf, run)
	require.ErrorContains(t, s.w.Env.GetWorkflowError(), "segment first failed")
	s.w.Env.AssertExpectations(t) // the second segment never ran
	require.Contains(t, s.events, events.SegmentFailed)
	require.Contains(t, s.events, events.PhaseFailed)
	require.Equal(t, "failure", s.w.Outcome(ref.OwnerRef("run/test-"+string(PipelineID))))
	s.w.AssertNoLeaks(t)
}

func TestSimulatedRunCancelDuringProvisioning(t *testing.T) {
	s := newSim(t)
	run := simRun()
	wf := pipelinetest.Workflow(s.w, PipelineID, Run)
	// Nothing converges: the cancel arrives while the VMs are still creating.
	s.w.Env.RegisterDelayedCallback(s.w.Env.CancelWorkflow, 20*time.Second)
	s.w.Env.ExecuteWorkflow(wf, run)
	require.Error(t, s.w.Env.GetWorkflowError())
	require.Equal(t, "canceled", s.w.Outcome(ref.OwnerRef("run/test-"+string(PipelineID))))
	s.w.AssertNoLeaks(t)
}

func requireDockerReady(t *testing.T, w *pipelinetest.World, agent id.AgentId) {
	t.Helper()
	for _, capability := range w.AgentState(agent).Capabilities {
		if capability.Name == "docker" && capability.Ready {
			return
		}
	}
	t.Fatal("Docker must be ready before pulling images or running the workload")
}

func TestSimulatedRunDockerInstallFailureCleans(t *testing.T) {
	s := newSim(t)
	run := noopRun()
	wf := pipelinetestWorkflow(s)
	s.converge(t, run, map[string]string{"runner-1": "10.130.0.20"})
	s.w.Handle("docker.install", func(workflow.Context, []any) (any, error) {
		return nil, errors.New("package repository unavailable")
	})
	s.w.Env.ExecuteWorkflow(wf, run)
	require.ErrorContains(t, s.w.Env.GetWorkflowError(), "prepare docker")
	require.ErrorContains(t, s.w.Env.GetWorkflowError(), "package repository unavailable")
	require.NotContains(t, s.events, events.SegmentStarted)
	require.Contains(t, s.events, events.PhaseFailed)
	s.w.AssertNoLeaks(t)
}
