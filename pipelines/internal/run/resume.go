package run

import (
	"fmt"

	"github.com/graphene-ci/pipeline/pkg/pipeline"

	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/events"
	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/provision"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// resume runs the workload again on a stand an earlier run kept. The
// stand's agents are attached — foreign to this run, never owned — so
// nothing is created, deployed or torn down: the stand stays under its
// keep timer. Provisioning is attaching and checking each agent is
// connected; deploying has nothing to do.
func resume(ctx pipeline.Context, run spec.Run) (spec.Result, error) {
	from := run.Resume
	type attached struct {
		runner pipeline.Agent
		addrs  Addresses
		agents map[string]pipeline.Agent
	}
	stand, err := phase(ctx, PhaseProvisioning, func() (attached, error) {
		infos := make(map[string]provision.MachineInfo, len(run.Machines))
		names := make([]string, 0, len(run.Machines))
		fns := make([]func(pipeline.Context) error, 0, len(run.Machines))
		var runner pipeline.Agent
		agents := make(map[string]pipeline.Agent, len(run.Machines))
		for _, m := range run.Machines {
			addr, ok := from.Machines[m.Name]
			if !ok {
				return attached{}, fmt.Errorf("resume: no address of machine %s", m.Name)
			}
			info := provision.MachineInfo{PrivateIP: addr.PrivateIP, PublicIP: addr.PublicIP}
			infos[m.Name] = info
			agent := pipeline.AttachAgent(ctx, spec.AgentName(from.FromRunID, m.Name))
			agents[m.Name] = agent
			if m.Role == run.Workload.RunnerRole && runner == nil {
				runner = agent
			}
			names = append(names, m.Name)
			fns = append(fns, func(gctx pipeline.Context) error {
				if err := waitAgent(gctx, agent); err != nil {
					return fmt.Errorf("agent of kept machine %s: %w", m.Name, err)
				}
				events.Emit(gctx, events.MachineReady, events.Payload{
					"machine": m.Name, "role": m.Role, "private_ip": info.PrivateIP, "public_ip": info.PublicIP, "resumed": true,
				})
				return nil
			})
		}
		if runner == nil {
			return attached{}, fmt.Errorf("resume: no machine of runner role %q", run.Workload.RunnerRole)
		}
		if err := parallel(ctx, names, fns); err != nil {
			return attached{}, err
		}
		return attached{runner: runner, addrs: AddressesOf(run.Machines, infos), agents: agents}, nil
	})
	if err != nil {
		return spec.Result{}, err
	}
	if _, err := phase(ctx, PhaseDeploying, func() (struct{}, error) { return struct{}{}, nil }); err != nil {
		return spec.Result{}, err
	}
	wl, wlErr := phase(ctx, PhaseWorkload, func() (workloadOutcome, error) {
		// Nothing is deployed, so nothing is probed; a failed reachability
		// wait still snapshots the stand's machines.
		order := make([]string, 0, len(run.Machines))
		for _, m := range run.Machines {
			order = append(order, m.Name)
		}
		return runWorkload(ctx, run, stand.runner, stand.addrs, newNetCheck(run, order, stand.agents, stand.addrs))
	})
	return collect(ctx, run, wl, wlErr)
}
