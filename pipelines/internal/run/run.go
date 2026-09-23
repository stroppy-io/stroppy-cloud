package run

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	k8slib "github.com/graphene-ci/library/k8s"
	"github.com/graphene-ci/pipeline/pkg/pipeline"
	"github.com/graphene-ci/pipeline/pkg/wire"

	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/activities"
	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/events"
	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/provision"
	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/topo"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
	"github.com/stroppy-io/stroppy-cloud/pipelines/stroppycfg"
)

// PipelineID is the Graphene pipeline id of stroppy-run.
const PipelineID = "stroppy-run"

const (
	ensureConfigTimeout = 5 * time.Minute
	// agentConnectTimeout bounds waiting for an agent after its VM runs.
	agentConnectTimeout = 15 * time.Minute
)

// Run is the pipeline body: RunSpec in, RunResult out.
//
// Phases: provisioning (provider config, agents, network, machines) →
// deploying (host prep, files, images, containers by dependency layer) →
// workload (stroppy segments on the runner) → collecting (artifacts,
// result) → keep (stand) or teardown (Graphene's cleanup cascade).
func Run(ctx pipeline.Context, run spec.Run) (spec.Result, error) {
	if ctx.Recording() {
		recordingWalk(ctx)
		return spec.Result{}, nil
	}
	normalized, normalizeErr := spec.NormalizeRun(run)
	if normalizeErr != nil {
		return spec.Result{}, normalizeErr
	}
	run = normalized
	for _, issue := range spec.CheckResources(run) {
		if issue.Severity == "WARNING" {
			ctx.Logger().Warn("resource preflight", "path", issue.Path, "code", issue.Code, "reason", issue.Message)
		}
	}
	if err := validate(run); err != nil {
		return spec.Result{}, err
	}
	providers := provision.Default()
	provider, err := providers.For(run.Provider.Kind)
	if err != nil {
		return spec.Result{}, err
	}
	k8s := k8slib.NewClientFromSecret(pipeline.Secret(ctx, activities.KubeconfigSecret), provider.Scheme())

	// --- provisioning ------------------------------------------------------
	type provisioned struct {
		infra provision.Infra
		addrs Addresses
	}
	prov, err := phase(ctx, PhaseProvisioning, func() (provisioned, error) {
		if err := resolveImages(ctx, &run); err != nil {
			return provisioned{}, err
		}
		if err := ensureProviderConfig(ctx, run); err != nil {
			return provisioned{}, err
		}
		agents := declareAgents(ctx, run)
		infra, err := provider.Provision(ctx, k8s, run, agents)
		if err != nil {
			return provisioned{}, err
		}
		infos, err := waitMachines(ctx, infra)
		if err != nil {
			return provisioned{}, err
		}
		endpoint, err := infra.ManagedEndpoint(ctx)
		if err != nil {
			return provisioned{}, err
		}
		if endpoint != "" {
			run.Workload.URL = endpoint
		}
		return provisioned{infra: infra, addrs: NewAddresses(infra, infos)}, nil
	})
	if err != nil {
		return spec.Result{}, err
	}
	infra, addrs := prov.infra, prov.addrs

	// --- deploying ---------------------------------------------------------
	if _, err := phase(ctx, PhaseDeploying, func() (struct{}, error) {
		if err := hostPrep(ctx, run, infra); err != nil {
			return struct{}{}, err
		}
		_, err := deployContainers(ctx, run, infra, addrs)
		return struct{}{}, err
	}); err != nil {
		return spec.Result{}, err
	}

	// --- workload ----------------------------------------------------------
	wl, wlErr := phase(ctx, PhaseWorkload, func() (workloadOutcome, error) {
		return runWorkload(ctx, run, infra, addrs)
	})

	// --- collecting: always, even after a failed workload — partial results
	// and logs are the evidence the operator needs.
	result, collectErr := phase(ctx, PhaseCollecting, func() (spec.Result, error) {
		res := spec.Result{
			Metrics:  stroppycfg.MergeMetrics(wl.Segments),
			Segments: wl.Segments,
			Baseline: wl.Baseline,
			Summary:  stroppycfg.Headline(wl.Segments),
		}
		if wl.Runner != nil {
			res.Artifacts = publishArtifacts(ctx, run, wl)
		}
		if missing := missingExpectations(run, res); len(missing) > 0 {
			ctx.Logger().Warn("result expectations not met", "missing", missing)
			events.Emit(ctx, events.ResultPublished, events.Payload{"degraded": true, "missing": missing})
		} else {
			events.Emit(ctx, events.ResultPublished, events.Payload{"segments": len(res.Segments)})
		}
		return res, nil
	})
	if collectErr != nil {
		ctx.Logger().Warn("collecting failed", "error", collectErr)
	}
	if wlErr != nil {
		return result, wlErr
	}

	// --- keep / teardown ---------------------------------------------------
	if keep := run.Keep.Std(); keep > 0 {
		_, err := phase(ctx, PhaseTeardown, func() (struct{}, error) {
			pipeline.ToStand(ctx, infra.Root, pipeline.KeepFor(keep))
			events.Emit(ctx, events.StandKept, events.Payload{"keep": keep.String(), "root": string(infra.Root.ResourceRef())})
			return struct{}{}, nil
		})
		if err != nil {
			return result, err
		}
	}
	// keep == 0: the run returns and Graphene's cleanup cascades the tree.
	return result, nil
}

// validate rejects what the schema cannot express and the phases would
// trip on late.
func validate(run spec.Run) error {
	if run.RunID == "" || run.Tenant == "" {
		return fmt.Errorf("run_id and tenant are required")
	}
	if len(run.Machines) == 0 {
		return fmt.Errorf("a run needs at least one machine")
	}
	nodes := make([]containerNode, 0, len(run.Containers))
	for _, c := range run.Containers {
		nodes = append(nodes, containerNode{c: c})
	}
	if _, err := topo.Layers(nodes); err != nil {
		return fmt.Errorf("container dependencies: %w", err)
	}
	byName := run.MachineByName()
	for _, c := range run.Containers {
		if _, ok := byName[c.Machine]; !ok {
			return fmt.Errorf("container %s: unknown machine %q", c.Name, c.Machine)
		}
	}
	if len(run.MachinesByRole()[run.Workload.RunnerRole]) != 1 {
		return fmt.Errorf("exactly one machine required for runner role %q", run.Workload.RunnerRole)
	}
	if len(run.Workload.Segments) == 0 {
		return fmt.Errorf("a run needs at least one workload segment")
	}
	if run.Workload.DriverType == "" || run.Workload.URL == "" {
		return fmt.Errorf("the workload needs driver_type and url")
	}
	segments, err := spec.DecodeSegments(run.Workload.Segments)
	if err != nil {
		return err
	}
	for _, segment := range segments {
		if err := stroppycfg.ValidateSegment(segment.Raw); err != nil {
			return err
		}
		if _, err := stroppycfg.Config(stroppycfg.Input{Segment: segment, Workload: run.Workload}); err != nil {
			return err
		}
		if err := stroppycfg.ValidateFiles(segment.Files); err != nil {
			return err
		}
	}
	return nil
}

// ensureProviderConfig applies the tenant's crossplane ProviderConfig on
// the run worker (cluster side) before any managed resource is declared.
func ensureProviderConfig(ctx pipeline.Context, run spec.Run) error {
	actx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:           wire.RunQueue(ctx.RunId()),
		StartToCloseTimeout: ensureConfigTimeout,
		RetryPolicy:         &temporal.RetryPolicy{InitialInterval: 2 * time.Second, BackoffCoefficient: 2, MaximumInterval: 30 * time.Second, MaximumAttempts: 5},
	})
	req := activities.EnsureProviderConfigRequest{
		Provider: run.Provider.Kind, Name: run.Provider.ProviderConfigName,
		CredentialsSecret: run.Provider.CredentialsSecret, Settings: run.Provider.Settings,
	}
	if err := workflow.ExecuteActivity(actx, activities.NameEnsureProviderConfig, req).Get(ctx, nil); err != nil {
		return fmt.Errorf("provider config: %w", err)
	}
	return nil
}

// declareAgents declares one agent record per machine, labeled by role
// and machine so activities can select them. The record exists before the
// VM (the identity goes into user-data) and is claimed by the VM as its
// child, so the machine's teardown reaches it.
func declareAgents(ctx pipeline.Context, run spec.Run) map[string]pipeline.AgentHandle {
	agents := make(map[string]pipeline.AgentHandle, len(run.Machines))
	for _, m := range run.Machines {
		opts := []pipeline.ResourceOption{pipeline.WithLabels(map[string]string{
			"role": m.Role, "machine": m.Name, "stroppy-run": run.RunID,
		})}
		agents[m.Name] = pipeline.NewAgent(ctx, agentName(run, m), append(opts, agentFlows(run, m)...)...)
	}
	return agents
}

// agentName is the agent of a machine (spec.AgentName).
func agentName(run spec.Run, m spec.Machine) string {
	return spec.AgentName(run.RunID, m.Name)
}

// waitMachines waits, in parallel, for every VM to run and its agent to
// connect, and emits machine.ready with the addresses.
func waitMachines(ctx pipeline.Context, infra provision.Infra) (map[string]provision.MachineInfo, error) {
	infos := make(map[string]provision.MachineInfo, len(infra.Order))
	names := make([]string, 0, len(infra.Order))
	fns := make([]func(pipeline.Context) error, 0, len(infra.Order))
	for _, name := range infra.Order {
		m := infra.Machines[name]
		names = append(names, name)
		fns = append(fns, func(gctx pipeline.Context) error {
			info, err := m.TryReady(gctx)
			if err != nil {
				return fmt.Errorf("vm: %w", err)
			}
			if _, err := waitAgent(gctx, m.Agent); err != nil {
				return fmt.Errorf("agent: %w", err)
			}
			infos[m.Spec.Name] = info
			events.Emit(gctx, events.MachineReady, events.Payload{
				"machine": m.Spec.Name, "role": m.Spec.Role, "id": info.ID,
				"private_ip": info.PrivateIP, "public_ip": info.PublicIP,
			})
			return nil
		})
	}
	if err := parallel(ctx, names, fns); err != nil {
		return nil, err
	}
	return infos, nil
}

// waitAgent bounds the wait for the agent's connect: a VM that runs but
// never connects (broken user-data, no route out) fails here with a clear
// reason instead of hanging on the machine's first activity.
func waitAgent(ctx pipeline.Context, agent pipeline.AgentHandle) (pipeline.AgentState, error) {
	// Temporal derives contexts by concrete type: the embedded
	// workflow.Context is handed over, never the pipeline wrapper.
	tctx, cancel := workflow.WithCancel(ctx.Context)
	defer cancel()
	var state pipeline.AgentState
	var err error
	done := false
	workflow.Go(tctx, func(gctx workflow.Context) {
		state, err = agent.TryReady(pipeline.Context{Context: gctx})
		done = true
	})
	if _, err := workflow.AwaitWithTimeout(ctx.Context, agentConnectTimeout, func() bool { return done }); err != nil {
		return state, err
	}
	if !done {
		return state, fmt.Errorf("agent %s did not connect within %s", agent.AgentId(), agentConnectTimeout)
	}
	return state, err
}

// missingExpectations lists result_expectations absent from the metrics.
func missingExpectations(run spec.Run, res spec.Result) []string {
	var missing []string
	for _, key := range run.ResultExpectations {
		found := false
		for k := range res.Metrics {
			if k == key || len(k) > len(key) && k[len(k)-len(key)-1:] == "."+key {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, key)
		}
	}
	return missing
}
