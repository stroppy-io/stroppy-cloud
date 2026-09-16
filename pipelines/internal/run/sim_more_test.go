package run

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/temporal"

	"github.com/graphene-ci/pipeline/pkg/ref"

	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/activities"
	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/events"
	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/provision"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// The remaining branches of stroppy-run: the aws provider, convergence
// failures, deploy failures, the nonfatal side paths (baseline, artifacts,
// expectations), activity-level segment failures, cancellation inside the
// workload and keep after a failed run.

const (
	awsVPCKind      = "k8s.ec2.aws.upbound.io.v1beta1.VPC"
	awsSubnetKind   = "k8s.ec2.aws.upbound.io.v1beta1.Subnet"
	awsIGWKind      = "k8s.ec2.aws.upbound.io.v1beta1.InternetGateway"
	awsRTKind       = "k8s.ec2.aws.upbound.io.v1beta1.RouteTable"
	awsRouteKind    = "k8s.ec2.aws.upbound.io.v1beta1.Route"
	awsRTAKind      = "k8s.ec2.aws.upbound.io.v1beta1.RouteTableAssociation"
	awsSGKind       = "k8s.ec2.aws.upbound.io.v1beta1.SecurityGroup"
	awsSGRuleKind   = "k8s.ec2.aws.upbound.io.v1beta1.SecurityGroupRule"
	awsInstanceKind = "k8s.ec2.aws.upbound.io.v1beta1.Instance"
)

// noopRun is the smallest run: one runner, no containers, the noop driver.
func noopRun() spec.Run {
	run := simRun()
	run.Containers, run.HostPrep, run.Flows = nil, nil, nil
	run.Machines = run.Machines[1:]
	run.Workload.DriverType, run.Workload.URL, run.Workload.Baseline = "noop", "noop://localhost", nil
	run.Workload.Segments = []json.RawMessage{json.RawMessage(`{"name":"smoke","workload":{"script":"simple"},"run":{"executor":"constant-vus","duration":"10s"}}`)}
	return run
}

func completed(name string, iterations float64) activities.RunSegmentResult {
	return activities.RunSegmentResult{Result: spec.SegmentResult{
		Name: name, Status: spec.SegmentCompleted,
		StartedAt: time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC), FinishedAt: time.Date(2026, 9, 9, 12, 0, 10, 0, time.UTC),
		Metrics: map[string]spec.MetricValue{"iterations_total": {Value: iterations}},
	}, LogPath: "/ws/stroppy/" + name + "/stroppy.log"}
}

func runOwner() ref.OwnerRef { return ref.OwnerRef("run/test-" + string(PipelineID)) }

// --- aws -------------------------------------------------------------------

func (s *sim) convergeAWS(t *testing.T, run spec.Run, ips map[string]string) {
	t.Helper()
	names := provision.NewNames(run.Tenant, run.RunID)
	set := func(delay time.Duration, kind, name string, at map[string]any) {
		require.NoError(t, s.objects.After(delay, ref.OwnerRef(kind+"/"+name), xpReadyStatus(at)))
	}
	set(2*time.Second, awsVPCKind, names.Network(), map[string]any{"id": "vpc-1"})
	set(3*time.Second, awsSubnetKind, names.Subnet(), map[string]any{"id": "subnet-1"})
	set(3*time.Second, awsIGWKind, names.Gateway(), map[string]any{"id": "igw-1"})
	set(4*time.Second, awsRTKind, names.RouteTable(), map[string]any{"id": "rtb-1"})
	set(5*time.Second, awsRouteKind, names.RouteTable()+"-default", map[string]any{"id": "r-1"})
	set(5*time.Second, awsRTAKind, names.RouteTable()+"-assoc", map[string]any{"id": "rtbassoc-1"})
	set(4*time.Second, awsSGKind, names.SecurityGroup(), map[string]any{"id": "sg-1"})
	set(5*time.Second, awsSGRuleKind, names.Rule("ingress", 0), map[string]any{"id": "sgr-0"})
	set(5*time.Second, awsSGRuleKind, names.Rule("egress", 1), map[string]any{"id": "sgr-1"})
	set(5*time.Second, awsSGRuleKind, names.Rule("ingress", 2), map[string]any{"id": "sgr-2"})
	for _, m := range run.Machines {
		set(40*time.Second, awsInstanceKind, names.Machine(m.Name), map[string]any{
			"id": "i-" + m.Name, "instanceState": "running", "privateIp": ips[m.Name], "publicIp": "3.0.0.1",
		})
		s.w.ConnectAfter(s.agent(run, m.Name), 90*time.Second)
	}
}

func TestSimulatedRunAWS(t *testing.T) {
	s := newSim(t)
	run := noopRun()
	run.Provider = spec.Provider{Kind: spec.ProviderAWS, Settings: json.RawMessage(`{"region":"eu-central-1","availability_zone":"eu-central-1a"}`), CredentialsSecret: "aws-keys", ProviderConfigName: "t-acme"}
	run.Network.Ingress = []spec.Ingress{{Port: 22, Proto: "tcp", CIDR: "0.0.0.0/0"}}
	run.Machines[0].Image = "ami-0abc"
	run.Machines[0].InstanceType = "m6i.large"
	run.Machines[0].Disks = []spec.Disk{{Name: "data", GB: 50, Type: "gp3", Mount: "/data"}}
	wf := pipelinetestWorkflow(s)
	s.convergeAWS(t, run, map[string]string{"runner-1": "10.130.0.20"})
	runner := s.agent(run, "runner-1")
	var req activities.RunSegmentRequest
	s.w.OnAgentActivity(runner, activities.NameRunSegment, mock.Anything, mock.MatchedBy(func(r activities.RunSegmentRequest) bool { req = r; return true })).
		Return(completed("smoke", 10), nil).Once()

	s.w.Env.ExecuteWorkflow(wf, run)
	require.NoError(t, s.w.Env.GetWorkflowError())
	s.w.Env.AssertExpectations(t)
	require.Equal(t, spec.ProviderAWS, s.configs[0].Provider)
	require.Equal(t, "noop://localhost", req.URL)
	// EBS rides on the instance: no separate disk record on aws.
	names := provision.NewNames(run.Tenant, run.RunID)
	_, hasDisk := s.w.Resource(ref.OwnerRef(ycDiskKind + "/" + names.Disk("runner-1", "data")))
	require.False(t, hasDisk)
	require.Equal(t, "success", s.w.Outcome(runOwner()))
	s.w.AssertNoLeaks(t)
}

func TestSimulatedRunUnsupportedProviderFailsBeforeInfra(t *testing.T) {
	s := newSim(t)
	run := noopRun()
	run.Provider.Kind = "gcp"
	wf := pipelinetestWorkflow(s)
	s.w.Env.ExecuteWorkflow(wf, run)
	require.ErrorContains(t, s.w.Env.GetWorkflowError(), "gcp")
	require.Empty(t, s.configs, "no credentials touched for an unknown provider")
	require.Empty(t, s.w.Events(), "nothing declared")
	require.Equal(t, "failure", s.w.Outcome(runOwner()))
}

func TestSimulatedRunProviderConfigFailure(t *testing.T) {
	s := newSim(t)
	s.configErr = errors.New("secret yc-sa-key: not found")
	run := noopRun()
	wf := pipelinetestWorkflow(s)
	s.converge(t, run, map[string]string{"runner-1": "10.130.0.20"})
	s.w.Env.ExecuteWorkflow(wf, run)
	require.ErrorContains(t, s.w.Env.GetWorkflowError(), "yc-sa-key")
	names := provision.NewNames(run.Tenant, run.RunID)
	_, declared := s.w.Resource(ref.OwnerRef(ycNetworkKind + "/" + names.Network()))
	require.False(t, declared, "no cloud object before the provider config exists")
	require.Contains(t, s.events, events.PhaseFailed)
	require.Equal(t, "failure", s.w.Outcome(runOwner()))
}

// --- convergence failures --------------------------------------------------

func TestSimulatedRunVMCreationFailsCleansTree(t *testing.T) {
	s := newSim(t)
	run := simRun()
	wf := pipelinetestWorkflow(s)
	s.converge(t, run, map[string]string{"db-1": "10.130.0.10", "runner-1": "10.130.0.20"})
	names := provision.NewNames(run.Tenant, run.RunID)
	vm := ref.OwnerRef(ycInstanceKind + "/" + names.Machine("db-1"))
	s.w.FailResource(vm, errors.New("quota exceeded: compute.instanceCores.count"))

	s.w.Env.ExecuteWorkflow(wf, run)
	require.ErrorContains(t, s.w.Env.GetWorkflowError(), "quota exceeded")
	require.Contains(t, s.events, events.PhaseFailed)
	require.NotContains(t, s.events, events.SegmentStarted)
	// The network and the other VM were created and are gone again.
	net, _ := s.w.Resource(ref.OwnerRef(ycNetworkKind + "/" + names.Network()))
	require.Equal(t, "deleted", net.Phase)
	other, _ := s.w.Resource(ref.OwnerRef(ycInstanceKind + "/" + names.Machine("runner-1")))
	require.Equal(t, "deleted", other.Phase)
	require.Equal(t, "failure", s.w.Outcome(runOwner()))
	s.w.AssertNoLeaks(t)
}

func TestSimulatedRunAgentNeverConnects(t *testing.T) {
	s := newSim(t)
	run := noopRun()
	wf := pipelinetestWorkflow(s)
	names := provision.NewNames(run.Tenant, run.RunID)
	// The VM runs, cloud-init never brings the agent up.
	require.NoError(t, s.objects.After(time.Second, ref.OwnerRef(ycNetworkKind+"/"+names.Network()), xpReadyStatus(nil)))
	require.NoError(t, s.objects.After(time.Second, ref.OwnerRef(ycSubnetKind+"/"+names.Subnet()), xpReadyStatus(nil)))
	require.NoError(t, s.objects.After(time.Second, ref.OwnerRef(ycSGKind+"/"+names.SecurityGroup()), xpReadyStatus(nil)))
	require.NoError(t, s.objects.After(10*time.Second, ref.OwnerRef(ycInstanceKind+"/"+names.Machine("runner-1")), xpReadyStatus(map[string]any{
		"id": "fhm", "status": "running", "networkInterface": []any{map[string]any{"ipAddress": "10.130.0.20"}},
	})))
	start := s.w.Env.Now()
	s.w.Env.ExecuteWorkflow(wf, run)
	require.ErrorContains(t, s.w.Env.GetWorkflowError(), "did not connect within")
	require.GreaterOrEqual(t, s.w.Env.Now().Sub(start), agentConnectTimeout)
	require.Equal(t, "failure", s.w.Outcome(runOwner()))
	s.w.AssertNoLeaks(t)
}

// --- deploy ----------------------------------------------------------------

func TestSimulatedRunHealthcheckFailureStopsDeploy(t *testing.T) {
	s := newSim(t)
	run := simRun()
	run.Workload.Baseline = nil
	wf := pipelinetestWorkflow(s)
	s.converge(t, run, map[string]string{"db-1": "10.130.0.10", "runner-1": "10.130.0.20"})
	db := s.agent(run, "db-1")
	s.w.OnAgentActivity(db, activities.NameHostPrep, mock.Anything, mock.Anything).Return(activities.HostPrepResult{}, nil).Once()
	s.w.OnAgentActivity(db, activities.NameWriteFiles, mock.Anything, mock.Anything).Return(activities.WriteFilesResult{Paths: []string{"/ws/containers/postgres/etc/postgresql/postgresql.conf"}}, nil).Once()
	s.w.OnAgentActivity(db, activities.NamePullImage, mock.Anything, mock.Anything).Return(activities.PullImageResult{Image: "postgres:17"}, nil).Once()
	s.w.OnAgentActivity(db, activities.NameWaitHealthy, mock.Anything, mock.Anything).
		Return(activities.WaitHealthyResult{}, temporal.NewNonRetryableApplicationError("container postgres not healthy after 10 attempts: FATAL: data directory", "Unhealthy", nil)).Once()

	s.w.Env.ExecuteWorkflow(wf, run)
	require.ErrorContains(t, s.w.Env.GetWorkflowError(), "not healthy")
	s.w.Env.AssertExpectations(t)
	require.NotContains(t, s.events, events.ContainerReady)
	require.NotContains(t, s.events, events.SegmentStarted, "no workload on a broken database")
	require.Equal(t, "failure", s.w.Outcome(runOwner()))
	s.w.AssertNoLeaks(t)
}

func TestSimulatedRunMultiMachineLayers(t *testing.T) {
	s := newSim(t)
	run := simRun()
	run.Workload.Baseline = nil
	run.Machines = append(run.Machines, spec.Machine{Name: "db-2", Role: "db", CPU: 4, MemoryGB: 8, Image: "ubuntu-2404-lts", Location: "ru-central1-d", InstanceType: "standard-v3"})
	run.Containers[0].Files = nil
	run.Containers = append(run.Containers,
		spec.Container{Name: "replica", Role: "db", Machine: "db-2", Image: "postgres:17", Env: map[string]string{"PRIMARY": "${ip:db-1}"}, DependsOn: []string{"postgres"}},
		spec.Container{Name: "exporter", Role: "db", Machine: "db-1", Image: "pg-exporter", DependsOn: []string{"postgres"}, Ports: []spec.Port{{Container: 9187, Host: 9187}}, Scrape: "/metrics"},
	)
	run.Scrapes = []spec.Scrape{{Role: "db", URL: "http://127.0.0.1:9187/metrics", Job: "postgres"}}
	run.Workload.URL = "postgres://postgres:x@${ip:role:db}:5432/postgres?hosts=${ips:role:db}"
	wf := pipelinetestWorkflow(s)
	s.converge(t, run, map[string]string{"db-1": "10.130.0.10", "db-2": "10.130.0.11", "runner-1": "10.130.0.20"})
	db1, db2, runner := s.agent(run, "db-1"), s.agent(run, "db-2"), s.agent(run, "runner-1")

	// Host prep hits every db machine, images are pulled per machine, no
	// files anywhere.
	s.w.OnAgentActivity(db1, activities.NameHostPrep, mock.Anything, mock.Anything).Return(activities.HostPrepResult{}, nil).Once()
	s.w.OnAgentActivity(db2, activities.NameHostPrep, mock.Anything, mock.Anything).Return(activities.HostPrepResult{}, nil).Once()
	s.w.OnAgentActivity(db1, activities.NamePullImage, mock.Anything, activities.PullImageRequest{Image: "postgres:17"}).Return(activities.PullImageResult{}, nil).Once()
	s.w.OnAgentActivity(db1, activities.NamePullImage, mock.Anything, activities.PullImageRequest{Image: "pg-exporter"}).Return(activities.PullImageResult{}, nil).Once()
	s.w.OnAgentActivity(db2, activities.NamePullImage, mock.Anything, activities.PullImageRequest{Image: "postgres:17"}).Return(activities.PullImageResult{}, nil).Once()
	s.w.OnAgentActivity(db1, activities.NameWaitHealthy, mock.Anything, mock.Anything).Return(activities.WaitHealthyResult{}, nil).Once()
	var req activities.RunSegmentRequest
	s.w.OnAgentActivity(runner, activities.NameRunSegment, mock.Anything, mock.MatchedBy(func(r activities.RunSegmentRequest) bool { req = r; return true })).
		Return(completed("load", 100), nil).Once()

	s.w.Env.ExecuteWorkflow(wf, run)
	require.NoError(t, s.w.Env.GetWorkflowError())
	s.w.Env.AssertExpectations(t)
	require.Equal(t, "postgres://postgres:x@10.130.0.10:5432/postgres?hosts=10.130.0.10,10.130.0.11", req.URL)

	// Layer order: postgres first, then replica and exporter in one layer.
	ready := make([]string, 0, 3)
	for _, e := range s.w.Events() {
		if e.Action == "ready" && (e.Resource == ref.OwnerRef("docker/"+run.RunID+"-postgres") || e.Resource == ref.OwnerRef("docker/"+run.RunID+"-replica") || e.Resource == ref.OwnerRef("docker/"+run.RunID+"-exporter")) {
			ready = append(ready, string(e.Resource))
		}
	}
	require.Len(t, ready, 3)
	require.Equal(t, "docker/"+run.RunID+"-postgres", ready[0])
	replica, _ := s.w.Resource(ref.OwnerRef("docker/" + run.RunID + "-replica"))
	require.Equal(t, db2, replica.Agent)
	var replicaSpec struct {
		Config struct {
			Env []string `json:"Env"`
		} `json:"config"`
	}
	require.NoError(t, json.Unmarshal(replica.Spec, &replicaSpec))
	require.Contains(t, replicaSpec.Config.Env, "PRIMARY=10.130.0.10")
	require.Equal(t, 3, countOf(s.events, events.ContainerReady))
	s.w.AssertNoLeaks(t)
}

// --- workload side paths ---------------------------------------------------

func TestSimulatedRunBaselineFailureIsNotFatal(t *testing.T) {
	s := newSim(t)
	run := noopRun()
	run.Workload.Baseline = &spec.Baseline{Enabled: true, Tiers: []string{"wire"}}
	wf := pipelinetestWorkflow(s)
	s.converge(t, run, map[string]string{"runner-1": "10.130.0.20"})
	runner := s.agent(run, "runner-1")
	s.w.OnAgentActivity(runner, activities.NameRunBaseline, mock.Anything, mock.Anything).
		Return(activities.RunBaselineResult{}, temporal.NewNonRetryableApplicationError("pg-noop download refused", "Baseline", nil)).Once()
	s.w.OnAgentActivity(runner, activities.NameRunSegment, mock.Anything, mock.Anything).Return(completed("smoke", 10), nil).Once()

	s.w.Env.ExecuteWorkflow(wf, run)
	require.NoError(t, s.w.Env.GetWorkflowError())
	var result spec.Result
	require.NoError(t, s.w.Env.GetWorkflowResult(&result))
	require.NotNil(t, result.Baseline)
	require.False(t, result.Baseline.OK)
	require.Contains(t, result.Baseline.Error, "pg-noop download refused")
	require.Equal(t, spec.SegmentCompleted, result.Segments[0].Status)
	require.Contains(t, s.events, events.BaselineFinished)
	s.w.AssertNoLeaks(t)
}

func TestSimulatedRunSegmentActivityLostIsFailure(t *testing.T) {
	s := newSim(t)
	run := noopRun()
	wf := pipelinetestWorkflow(s)
	s.converge(t, run, map[string]string{"runner-1": "10.130.0.20"})
	runner := s.agent(run, "runner-1")
	// The worker died mid-segment: AtMostOnce turns the timeout into an
	// unknown outcome; the run must not re-run the segment.
	s.w.OnAgentActivity(runner, activities.NameRunSegment, mock.Anything, mock.Anything).
		Return(activities.RunSegmentResult{}, temporal.NewTimeoutError(1, nil)).Once()

	s.w.Env.ExecuteWorkflow(wf, run)
	require.ErrorContains(t, s.w.Env.GetWorkflowError(), "segment smoke")
	s.w.Env.AssertExpectations(t)
	require.Equal(t, 1, countOf(s.events, events.SegmentStarted), "no second attempt")
	require.Contains(t, s.events, events.SegmentFailed)
	require.Equal(t, "failure", s.w.Outcome(runOwner()))
	s.w.AssertNoLeaks(t)
}

func TestSimulatedRunMultipleSegmentsAndArtifactFailure(t *testing.T) {
	s := newSim(t)
	run := noopRun()
	run.Workload.Segments = []json.RawMessage{
		json.RawMessage(`{"name":"load","workload":{"script":"tpcb/tx","scale_factor":1},"run":{"executor":"shared-iterations","iterations":1},"steps":["create_schema","load_data"]}`),
		json.RawMessage(`{"name":"steady","workload":{"script":"tpcb/tx","scale_factor":1},"run":{"executor":"constant-vus","vus":8,"duration":"1m"},"no_steps":["drop_schema","create_schema","load_data"]}`),
	}
	run.ResultExpectations = []string{"iterations_total", "tpm_c"}
	wf := pipelinetestWorkflow(s)
	s.converge(t, run, map[string]string{"runner-1": "10.130.0.20"})
	runner := s.agent(run, "runner-1")
	var order []string
	s.w.OnAgentActivity(runner, activities.NameRunSegment, mock.Anything, mock.MatchedBy(func(r activities.RunSegmentRequest) bool {
		order = append(order, r.Segment.Name)
		return true
	})).Return(completed("load", 1), nil).Once()
	s.w.OnAgentActivity(runner, activities.NameRunSegment, mock.Anything, mock.MatchedBy(func(r activities.RunSegmentRequest) bool {
		order = append(order, r.Segment.Name)
		return r.Segment.Name == "steady" && r.Index == 1
	})).Return(completed("steady", 6000), nil).Once()
	// Only the second log exists on the machine: the first upload fails
	// and must not fail the run.
	s.w.File(runner, "/ws/stroppy/steady/stroppy.log", []byte("log"))

	s.w.Env.ExecuteWorkflow(wf, run)
	require.NoError(t, s.w.Env.GetWorkflowError())
	var result spec.Result
	require.NoError(t, s.w.Env.GetWorkflowResult(&result))
	require.Equal(t, []string{"load", "steady"}, order[:2])
	require.Len(t, result.Segments, 2)
	require.Equal(t, []string{"artifact/8f1c3f2a-stroppy-steady-log"}, result.Artifacts)
	require.Zero(t, result.Summary.TPS, "no throughput was reported by Stroppy")
	require.Contains(t, result.Metrics, "load.iterations_total")
	require.Contains(t, result.Metrics, "steady.iterations_total")
	// tpm_c was expected and is missing: published as degraded.
	require.Contains(t, s.events, events.ResultPublished)
	require.True(t, s.degraded)
	s.w.Advance(logArtifactRetention + time.Hour)
	s.w.AssertNoLeaks(t)
}

func TestSimulatedRunCancelDuringWorkload(t *testing.T) {
	s := newSim(t)
	run := noopRun()
	wf := pipelinetestWorkflow(s)
	s.converge(t, run, map[string]string{"runner-1": "10.130.0.20"})
	runner := s.agent(run, "runner-1")
	// The segment never returns on its own: the cancel arrives first.
	s.w.OnAgentActivity(runner, activities.NameRunSegment, mock.Anything, mock.Anything).
		After(time.Hour).Return(completed("smoke", 1), nil).Maybe()
	s.w.Env.RegisterDelayedCallback(s.w.Env.CancelWorkflow, 5*time.Minute)

	s.w.Env.ExecuteWorkflow(wf, run)
	require.Error(t, s.w.Env.GetWorkflowError())
	require.Contains(t, s.events, events.SegmentStarted)
	require.Equal(t, "canceled", s.w.Outcome(runOwner()))
	s.w.AssertNoLeaks(t)
}

func TestSimulatedRunKeepIgnoredAfterFailure(t *testing.T) {
	s := newSim(t)
	run := noopRun()
	run.Keep = spec.Duration(time.Hour)
	wf := pipelinetestWorkflow(s)
	s.converge(t, run, map[string]string{"runner-1": "10.130.0.20"})
	runner := s.agent(run, "runner-1")
	s.w.OnAgentActivity(runner, activities.NameRunSegment, mock.Anything, mock.Anything).
		Return(activities.RunSegmentResult{Result: spec.SegmentResult{Name: "smoke", Status: spec.SegmentFailed, ExitCode: 1, Error: "boom"}}, nil).Once()

	s.w.Env.ExecuteWorkflow(wf, run)
	require.Error(t, s.w.Env.GetWorkflowError())
	// A failed run is torn down even with keep — by construction, not by a
	// rule: ToStand sits at the tail of the success path, so a failure
	// returns before the transfer and finalize deletes everything the run
	// still owns. Cost-safe default: a broken benchmark has nothing to
	// inspect and burns quota; keeping infra on failure would be an
	// explicit ToStand on the error path.
	names := provision.NewNames(run.Tenant, run.RunID)
	net, _ := s.w.Resource(ref.OwnerRef(ycNetworkKind + "/" + names.Network()))
	require.Equal(t, "deleted", net.Phase)
	require.NotContains(t, s.events, events.StandKept)
	s.w.AssertNoLeaks(t)
}

func countOf(list []string, name string) int {
	n := 0
	for _, s := range list {
		if s == name {
			n++
		}
	}
	return n
}
