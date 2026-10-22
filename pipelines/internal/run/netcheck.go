package run

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/graphene-ci/pipeline/pkg/activity"
	"github.com/graphene-ci/pipeline/pkg/artifact"
	"github.com/graphene-ci/pipeline/pkg/pipeline"

	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/activities"
	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/events"
	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/provision"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

/*
CONNECTIVITY CHECK. Once every container is healthy, every machine of the
run dials every other machine's published ports (and the runner the
workload's endpoint), all of one machine's dials at once in one activity.
A path that fails is recorded with how it failed; the machines on both
ends of a failed path snapshot their network state (addresses, routes,
neighbors, listeners, half-open connects, firewall, sysctl, dmesg) into
artifacts. Then the policy: a failed path to a database, proxy or
coordinator port (the cluster and the workload's path) or to the endpoint
fails the run before the workload; a failed path to an exporter or a
one-shot helper only warns. The check itself never fails a run: a probe
that could not run is a warning on its paths.
*/

// Bounds of the check: a healthy run spends one round of dials.
const (
	netProbeDial    = 3 * time.Second
	netProbeWindow  = 10 * time.Second
	netProbeTimeout = time.Minute
	// netSnapshotTimeout covers ten commands of ten seconds each.
	netSnapshotTimeout = 3 * time.Minute
	// netPairsInMilestone bounds the milestone; failed pairs go first.
	netPairsInMilestone = 2000
	// netReasonPairs is how many failed paths the run's error names.
	netReasonPairs = 3
)

// Stages of the check, the milestone's `stage`.
const (
	netStageDeploy   = "deploy"
	netStageWorkload = "workload"
)

// netPair is one path: from a machine to a port of another (or to the
// workload's endpoint).
type netPair struct {
	From      string  `json:"from"`
	To        string  `json:"to"`
	Port      int     `json:"port"`
	Address   string  `json:"address,omitempty"`
	Container string  `json:"container,omitempty"`
	Critical  bool    `json:"critical"`
	Outcome   string  `json:"outcome"`
	LatencyMs float64 `json:"latency_ms"`
	Attempts  int     `json:"attempts,omitempty"`
	Error     string  `json:"error,omitempty"`
}

func (p netPair) ok() bool { return p.Outcome == string(activities.NetOK) }

func (p netPair) String() string {
	return fmt.Sprintf("%s:%d unreachable from %s (%s)", p.To, p.Port, p.From, p.Outcome)
}

// netSnapshot is a published network snapshot of one machine.
type netSnapshot struct {
	Machine  string `json:"machine"`
	Artifact string `json:"artifact"`
}

// netCheck is the check's view of the run: the machines' agents and
// addresses, and what it found so far.
type netCheck struct {
	run    spec.Run
	order  []string
	agents map[string]pipeline.Agent
	addrs  Addresses
	runner string
	pairs  []netPair
	snaps  []netSnapshot
}

func newNetCheck(run spec.Run, order []string, agents map[string]pipeline.Agent, addrs Addresses) *netCheck {
	nc := &netCheck{run: run, order: order, agents: agents, addrs: addrs}
	for _, m := range run.Machines {
		if m.Role == run.Workload.RunnerRole {
			nc.runner = m.Name
			break
		}
	}
	return nc
}

func netCheckOf(run spec.Run, infra provision.Infra, addrs Addresses) *netCheck {
	agents := make(map[string]pipeline.Agent, len(infra.Order))
	for _, name := range infra.Order {
		agents[name] = infra.Machines[name].Agent
	}
	return newNetCheck(run, infra.Order, agents, addrs)
}

// artifacts are the refs of the published snapshots.
func (nc *netCheck) artifacts() []string {
	if nc == nil {
		return nil
	}
	out := make([]string, 0, len(nc.snaps))
	for _, s := range nc.snaps {
		out = append(out, s.Artifact)
	}
	return out
}

// netSource is one machine and what it dials.
type netSource struct {
	machine string
	targets []activities.NetTarget
}

// plan lists, per machine in spec order, the ports of every other
// machine; the runner also dials the workload's endpoint.
func (nc *netCheck) plan() []netSource {
	var out []netSource
	for _, from := range nc.order {
		var targets []activities.NetTarget
		for _, to := range nc.order {
			if to == from {
				continue
			}
			ip := nc.addrs.PrivateIP(to)
			if ip == "" {
				continue
			}
			seen := map[int]bool{}
			for _, c := range nc.run.ContainersOn(to) {
				for _, p := range c.Ports {
					if p.Host <= 0 || seen[p.Host] {
						continue
					}
					seen[p.Host] = true
					targets = append(targets, activities.NetTarget{
						Machine: to, Address: net.JoinHostPort(ip, strconv.Itoa(p.Host)), Port: p.Host,
						Container: c.Name, Critical: criticalContainer(nc.run, c),
					})
				}
			}
		}
		if from == nc.runner {
			targets = nc.withEndpoint(targets)
		}
		if len(targets) > 0 {
			out = append(out, netSource{machine: from, targets: targets})
		}
	}
	return out
}

// endpoint is the workload's endpoint as a target: the machine it lives
// on (or its host) and its port. ok is false when the workload has no TCP
// endpoint the check may dial: noop, managed YDB (its own readiness covers
// it), an unknown URL form, or an endpoint on the runner itself.
func (nc *netCheck) endpoint() (activities.NetTarget, bool) {
	if nc.run.Workload.DriverType == "noop" || nc.run.ManagedYDB != nil {
		return activities.NetTarget{}, false
	}
	url, err := nc.addrs.Expand(nc.run.Workload.URL)
	if err != nil {
		return activities.NetTarget{}, false
	}
	address := activities.EndpointAddress(url)
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return activities.NetTarget{}, false
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return activities.NetTarget{}, false
	}
	to := host
	for _, name := range nc.order {
		if nc.addrs.PrivateIP(name) == host {
			to = name
			break
		}
	}
	if to == nc.runner {
		return activities.NetTarget{}, false
	}
	return activities.NetTarget{Machine: to, Address: address, Port: port, Critical: true}, true
}

func (nc *netCheck) withEndpoint(targets []activities.NetTarget) []activities.NetTarget {
	ep, ok := nc.endpoint()
	if !ok {
		return targets
	}
	for i := range targets {
		if targets[i].Address == ep.Address {
			targets[i].Critical = true
			return targets
		}
	}
	return append(targets, ep)
}

// criticalContainer reports whether a container's ports are the cluster's
// or the workload's path: a database, a proxy, a coordinator. A spec that
// names no kinds counts its role's serving container.
func criticalContainer(run spec.Run, c spec.Container) bool {
	switch c.Kind {
	case spec.ContainerKindDatabase, spec.ContainerKindProxy, spec.ContainerKindCoordinator:
		return true
	case "":
		serving, ok := servingContainerOfRole(run, c.Role)
		return ok && serving == c.Name
	}
	return false
}

// probe runs the deploy-time check. The error is the policy's verdict; the
// paths and snapshots are kept on nc either way.
func (nc *netCheck) probe(ctx pipeline.Context) error {
	plan := nc.plan()
	if len(plan) == 0 {
		return nil
	}
	results := make([][]netPair, len(plan))
	names := make([]string, 0, len(plan))
	fns := make([]func(pipeline.Context) error, 0, len(plan))
	for i, src := range plan {
		agent := nc.agents[src.machine]
		names = append(names, src.machine)
		fns = append(fns, func(gctx pipeline.Context) error {
			req := activities.NetProbeRequest{Targets: src.targets, DialTimeout: netProbeDial, Window: netProbeWindow}
			res, err := activity.Activity(gctx, agent, activity.Fn(activities.NameNetProbe, activities.NetProbe, req),
				activity.WithGuarantee(activity.AtMostOnce), activity.WithTimeout(netProbeTimeout))
			if err != nil {
				// The check could not run here: its paths are unknown, which
				// is a warning, never the run's failure.
				gctx.Logger().Warn("net probe did not run", "machine", src.machine, "error", err)
				for _, t := range src.targets {
					results[i] = append(results[i], netPair{
						From: src.machine, To: t.Machine, Port: t.Port, Address: t.Address, Container: t.Container,
						Outcome: string(activities.NetOther), Error: "probe did not run: " + err.Error(),
					})
				}
				return nil
			}
			for _, p := range res.Pairs {
				results[i] = append(results[i], netPair{
					From: src.machine, To: p.Machine, Port: p.Port, Address: p.Address, Container: p.Container,
					Critical: p.Critical, Outcome: string(p.Outcome), LatencyMs: p.LatencyMs, Attempts: p.Attempts, Error: p.Error,
				})
			}
			return nil
		})
	}
	_ = parallel(ctx, names, fns) //nolint:errcheck // every unit records its failure as paths
	for _, r := range results {
		nc.pairs = append(nc.pairs, r...)
	}
	nc.snapshot(ctx, "probe")
	nc.emit(ctx, netStageDeploy)
	return nc.verdict()
}

// unreachable records the workload's failed reachability wait as the
// runner → endpoint path, snapshots both ends and reports it again.
func (nc *netCheck) unreachable(ctx pipeline.Context, waitErr error) {
	ep, ok := nc.endpoint()
	if !ok || nc.runner == "" {
		return
	}
	pair := netPair{
		From: nc.runner, To: ep.Machine, Port: ep.Port, Address: ep.Address, Critical: true,
		Outcome: string(activities.ClassifyDialText(waitErr.Error())), Error: waitErr.Error(),
	}
	if pair.ok() {
		pair.Outcome = string(activities.NetOther)
	}
	replaced := false
	for i := range nc.pairs {
		if nc.pairs[i].From == pair.From && nc.pairs[i].Address == pair.Address {
			nc.pairs[i], replaced = pair, true
		}
	}
	if !replaced {
		nc.pairs = append(nc.pairs, pair)
	}
	nc.snapshot(ctx, "reach")
	nc.emit(ctx, netStageWorkload)
}

// snapshot captures the network state of every machine on a failed path
// that has no snapshot of this label yet, and publishes each as an
// artifact. A snapshot that fails is a warning.
func (nc *netCheck) snapshot(ctx pipeline.Context, label string) {
	involved := map[string][]string{}
	for _, p := range nc.pairs {
		if p.ok() {
			continue
		}
		line := fmt.Sprintf("%s:%d from %s: %s %s", p.To, p.Port, p.From, p.Outcome, p.Error)
		for _, m := range []string{p.From, p.To} {
			if _, ok := nc.agents[m]; ok {
				involved[m] = append(involved[m], line)
			}
		}
	}
	var machines []string
	for _, m := range nc.order {
		if len(involved[m]) > 0 {
			machines = append(machines, m)
		}
	}
	if len(machines) == 0 {
		return
	}
	paths := make([]string, len(machines))
	fns := make([]func(pipeline.Context) error, 0, len(machines))
	for i, m := range machines {
		agent := nc.agents[m]
		req := activities.NetSnapshotRequest{Label: label + "-" + m, Machine: m, Context: involved[m]}
		fns = append(fns, func(gctx pipeline.Context) error {
			res, err := activity.Activity(gctx, agent, activity.Fn(activities.NameNetSnapshot, activities.NetSnapshot, req),
				activity.WithGuarantee(activity.AtMostOnce), activity.WithTimeout(netSnapshotTimeout))
			if err != nil {
				gctx.Logger().Warn("network snapshot failed", "machine", m, "error", err)
				return nil
			}
			paths[i] = res.Path
			return nil
		})
	}
	_ = parallel(ctx, machines, fns) //nolint:errcheck // a failed snapshot is a warning
	for i, m := range machines {
		if paths[i] == "" {
			continue
		}
		name := nc.run.RunID[:8] + "-netdiag-" + label + "-" + m
		h := pipeline.NewArtifact(ctx, name, artifact.FromAgentFile(nc.agents[m], paths[i]))
		if _, err := h.TryReady(ctx); err != nil {
			ctx.Logger().Warn("network snapshot upload failed", "artifact", name, "error", err)
			continue
		}
		pipeline.ToStand(ctx, h, pipeline.KeepFor(logArtifactRetention))
		nc.snaps = append(nc.snaps, netSnapshot{Machine: m, Artifact: "artifact/" + name})
	}
}

// emit reports every path found so far: failed ones first.
func (nc *netCheck) emit(ctx pipeline.Context, stage string) {
	failed, critical := 0, 0
	ordered := make([]netPair, 0, len(nc.pairs))
	for _, p := range nc.pairs {
		if !p.ok() {
			failed++
			if p.Critical {
				critical++
			}
			ordered = append(ordered, p)
		}
	}
	for _, p := range nc.pairs {
		if p.ok() {
			ordered = append(ordered, p)
		}
	}
	if len(ordered) > netPairsInMilestone {
		ordered = ordered[:netPairsInMilestone]
	}
	snaps := nc.snaps
	if snaps == nil {
		snaps = []netSnapshot{}
	}
	events.Emit(ctx, events.NetworkChecked, events.Payload{
		"stage": stage, "total": len(nc.pairs), "failed": failed, "critical": critical,
		"pairs": ordered, "snapshots": snaps,
	})
}

// verdict fails the run on a failed critical path, naming the first few.
func (nc *netCheck) verdict() error {
	var bad []string
	for _, p := range nc.pairs {
		if !p.ok() && p.Critical {
			bad = append(bad, p.String())
		}
	}
	switch {
	case len(bad) == 0:
		return nil
	case len(bad) > netReasonPairs:
		return fmt.Errorf("connectivity: %s (and %d more)", strings.Join(bad[:netReasonPairs], "; "), len(bad)-netReasonPairs)
	}
	return fmt.Errorf("connectivity: %s", strings.Join(bad, "; "))
}
