package run

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-units"

	dockerlib "github.com/graphene-ci/library/docker"
	"github.com/graphene-ci/pipeline/pkg/activity"
	"github.com/graphene-ci/pipeline/pkg/pipeline"

	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/activities"
	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/events"
	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/provision"
	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/topo"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// Activity bounds of the deploy phase.
const (
	dockerInstallTimeout = 15 * time.Minute
	hostPrepTimeout      = 15 * time.Minute
	writeFilesTimeout    = 2 * time.Minute
	pullImageTimeout     = 30 * time.Minute
	healthExtra          = 2 * time.Minute
	defaultHealthGap     = 5 * time.Second
	defaultRetries       = 30
	// filesDir is the workspace-relative directory of rendered configs.
	filesDir = "containers"
)

// containerNode adapts a spec container to the dependency sorter.
type containerNode struct{ c spec.Container }

func (n containerNode) NodeName() string   { return n.c.Name }
func (n containerNode) NodeDeps() []string { return n.c.DependsOn }

// deployed is what the deploy phase hands to the workload: container
// handles by name (for flows and teardown ordering) and the containers'
// addresses.
type deployed struct {
	handles map[string]pipeline.Resource[dockerlib.Info]
}

// hostPrep runs every host_prep step on the machines of its role, in spec
// order — one ActivityAll per step.
func hostPrep(ctx pipeline.Context, run spec.Run, infra provision.Infra) error {
	for i, step := range run.HostPrep {
		targets := agentsOf(infra.ByRole(step.Role))
		if len(targets) == 0 {
			return fmt.Errorf("host_prep[%d]: no machines of role %q", i, step.Role)
		}
		req := activities.HostPrepRequest{Kind: step.Kind, Content: step.Content, Index: i}
		if _, err := activity.ActivityAll(ctx, targets, activity.Fn(activities.NameHostPrep, activities.HostPrep, req),
			activity.WithTimeout(hostPrepTimeout)); err != nil {
			return fmt.Errorf("host_prep[%d] %s on %s: %w", i, step.Kind, step.Role, err)
		}
	}
	return nil
}

// deployContainers writes configs, pulls images and brings containers up
// layer by layer of their depends_on graph. Inside a layer everything runs
// in parallel across machines; a layer is complete when every container is
// running AND healthy.
func deployContainers(ctx pipeline.Context, run spec.Run, infra provision.Infra, addrs Addresses) (deployed, error) {
	out := deployed{handles: map[string]pipeline.Resource[dockerlib.Info]{}}
	nodes := make([]containerNode, 0, len(run.Containers))
	for _, c := range run.Containers {
		nodes = append(nodes, containerNode{c: c})
	}
	layers, err := topo.Layers(nodes)
	if err != nil {
		return out, err
	}
	// Per machine: write every container's files and pull every image once,
	// before any container starts — the slow parts, done in parallel.
	filePaths, err := prepareMachines(ctx, run, infra, addrs)
	if err != nil {
		return out, err
	}
	for li, layer := range layers {
		names := make([]string, 0, len(layer))
		fns := make([]func(pipeline.Context) error, 0, len(layer))
		for _, n := range layer {
			c := n.c
			m, ok := infra.Machine(c.Machine)
			if !ok {
				return out, fmt.Errorf("container %s: unknown machine %q", c.Name, c.Machine)
			}
			dspec, err := containerSpec(c, run, filePaths[c.Name], addrs)
			if err != nil {
				return out, fmt.Errorf("container %s: %w", c.Name, err)
			}
			// Declared on the main context so the handle's future is owned
			// by the run workflow, not a goroutine that may finish first.
			h := dockerlib.Container(ctx, m.Agent, dspec, flowOptions(c, run)...)
			out.handles[c.Name] = h
			names = append(names, c.Name)
			fns = append(fns, func(gctx pipeline.Context) error {
				info, err := h.TryReady(gctx)
				if err != nil {
					return err
				}
				if c.Healthcheck != nil {
					if err := waitHealthy(gctx, m.Agent, c, dspec.Name); err != nil {
						return err
					}
				}
				events.Emit(gctx, events.ContainerReady, events.Payload{
					"container": c.Name, "role": c.Role, "machine": c.Machine, "id": info.Id, "layer": li,
				})
				return nil
			})
		}
		if err := parallel(ctx, names, fns); err != nil {
			return out, fmt.Errorf("deploy layer %d: %w", li, err)
		}
	}
	return out, nil
}

// prepareMachines ensures Docker on the workload runner and container hosts,
// writes rendered files and pulls images. Returns container → (spec path → absolute machine
// path) for the bind mounts.
func prepareMachines(ctx pipeline.Context, run spec.Run, infra provision.Infra, addrs Addresses) (map[string]map[string]string, error) {
	paths := map[string]map[string]string{}
	var names []string
	var fns []func(pipeline.Context) error
	runners := infra.ByRole(run.Workload.RunnerRole)
	for _, machineName := range infra.Order {
		m := infra.Machines[machineName]
		cs := run.ContainersOn(machineName)
		isRunner := len(runners) > 0 && m == runners[0]
		if len(cs) == 0 && !isRunner {
			continue
		}
		// Files: one request per machine; the activity returns absolute paths
		// in request order, which we map back to (container, path).
		var files []activities.WorkspaceFile
		var owners []struct{ c, p string }
		for _, c := range cs {
			for _, f := range c.Files {
				content, err := addrs.Expand(f.Content)
				if err != nil {
					return nil, fmt.Errorf("container %s file %s: %w", c.Name, f.Path, err)
				}
				files = append(files, activities.WorkspaceFile{Path: c.Name + "/" + strings.TrimLeft(f.Path, "/"), Content: content, Mode: f.Mode})
				owners = append(owners, struct{ c, p string }{c.Name, f.Path})
			}
		}
		images := distinctImages(cs)
		mach := m
		names = append(names, machineName)
		fns = append(fns, func(gctx pipeline.Context) error {
			if _, err := activity.Activity(gctx, mach.Agent, dockerlib.Install(),
				activity.WithTimeout(dockerInstallTimeout), activity.WithHeartbeat(time.Minute)); err != nil {
				return fmt.Errorf("prepare docker: %w", err)
			}
			if len(files) > 0 {
				res, err := activity.Activity(gctx, mach.Agent,
					activity.Fn(activities.NameWriteFiles, activities.WriteFiles, activities.WriteFilesRequest{Dir: filesDir, Files: files}),
					activity.WithTimeout(writeFilesTimeout))
				if err != nil {
					return fmt.Errorf("write files: %w", err)
				}
				if len(res.Paths) != len(owners) {
					return fmt.Errorf("write files: %d paths for %d files", len(res.Paths), len(owners))
				}
				for i, o := range owners {
					if paths[o.c] == nil {
						paths[o.c] = map[string]string{}
					}
					paths[o.c][o.p] = res.Paths[i]
				}
			}
			for _, img := range images {
				if _, err := activity.Activity(gctx, mach.Agent,
					activity.Fn(activities.NamePullImage, activities.PullImage, activities.PullImageRequest{Image: img, RegistrySecret: run.Provider.RegistrySecret}),
					activity.WithTimeout(pullImageTimeout), activity.WithHeartbeat(time.Minute)); err != nil {
					return fmt.Errorf("pull %s: %w", img, err)
				}
			}
			return nil
		})
	}
	if err := parallel(ctx, names, fns); err != nil {
		return nil, err
	}
	return paths, nil
}

func distinctImages(cs []spec.Container) []string {
	seen := map[string]bool{}
	var out []string
	for _, c := range cs {
		if !seen[c.Image] {
			seen[c.Image] = true
			out = append(out, c.Image)
		}
	}
	sort.Strings(out)
	return out
}

// containerSpec builds the docker declaration of one container: host
// networking (a benchmark wants the wire, not a bridge), rendered files
// bind-mounted read-only from the machine workspace, restart policy,
// ulimits, and the scrape endpoint when the container exposes metrics.
func containerSpec(c spec.Container, run spec.Run, files map[string]string, addrs Addresses) (dockerlib.Spec, error) {
	env, err := addrs.ExpandMap(c.Env)
	if err != nil {
		return dockerlib.Spec{}, err
	}
	cmd, err := addrs.ExpandList(c.Cmd)
	if err != nil {
		return dockerlib.Spec{}, err
	}
	entrypoint, err := addrs.ExpandList(c.Entrypoint)
	if err != nil {
		return dockerlib.Spec{}, err
	}
	cfg := &container.Config{
		Image:      c.Image,
		Entrypoint: entrypoint,
		Cmd:        cmd,
		Env:        envList(env),
		Labels: map[string]string{
			"stroppy-run": run.RunID, "stroppy-role": c.Role, "stroppy-container": c.Name,
		},
	}
	host := &container.HostConfig{
		NetworkMode:   "host",
		RestartPolicy: container.RestartPolicy{Name: restartPolicy(c.Restart)},
		LogConfig:     container.LogConfig{Type: "json-file", Config: map[string]string{"max-size": "100m", "max-file": "5"}},
	}
	for _, m := range c.Mounts {
		host.Binds = append(host.Binds, bind(m.Source, m.Target, m.RO))
	}
	// Files go in path order for a stable spec (the entity diffs it).
	filePaths := make([]string, 0, len(files))
	for p := range files {
		filePaths = append(filePaths, p)
	}
	sort.Strings(filePaths)
	for _, p := range filePaths {
		host.Binds = append(host.Binds, bind(files[p], p, true))
	}
	if len(c.Ulimits) > 0 {
		keys := make([]string, 0, len(c.Ulimits))
		for k := range c.Ulimits {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			host.Ulimits = append(host.Ulimits, &units.Ulimit{Name: k, Soft: c.Ulimits[k], Hard: c.Ulimits[k]})
		}
	}
	return dockerlib.Spec{Name: containerName(run.RunID, c.Name), Config: cfg, Host: host, Scrape: scrapeURL(c, run)}, nil
}

// scrapeURL is the container's own metrics endpoint: its scrape path on
// its explicit metrics port (legacy: first port), or the
// spec-level scrape whose job matches the container.
func scrapeURL(c spec.Container, run spec.Run) string {
	port := c.ScrapePort
	if port == 0 && len(c.Ports) > 0 {
		port = c.Ports[0].Container
	}
	if c.Scrape != "" && port > 0 {
		return fmt.Sprintf("http://127.0.0.1:%d%s", port, c.Scrape)
	}
	for _, s := range run.Scrapes {
		if s.Role == c.Role && s.Job == c.Name {
			return s.URL
		}
	}
	return ""
}

// flowOptions declares the outgoing edges of a container from the spec's
// role-level flows: every container of from_role points at the FIRST
// container of to_role (or the external target).
func flowOptions(c spec.Container, run spec.Run) []pipeline.ResourceOption {
	var opts []pipeline.ResourceOption
	for _, f := range run.Flows {
		if f.FromRole != c.Role {
			continue
		}
		proto := flowProtocol(f.Protocol)
		label := f.Label
		if label == "" {
			label = f.Protocol
		}
		switch {
		case f.External != "":
			opts = append(opts, pipeline.WithFlow(f.External, proto, label, pipeline.FlowPort(f.Port)))
		case f.ToRole != "":
			if target, ok := firstContainerOfRole(run, f.ToRole); ok && target != c.Name {
				opts = append(opts, pipeline.WithFlow(string(dockerlib.ContainerKind)+"/"+containerName(run.RunID, target), proto, label, pipeline.FlowPort(f.Port)))
			}
		}
	}
	return opts
}

func firstContainerOfRole(run spec.Run, role string) (string, bool) {
	for _, c := range run.Containers {
		if c.Role == role {
			return c.Name, true
		}
	}
	return "", false
}

func flowProtocol(p string) pipeline.Protocol {
	switch p {
	case "http":
		return pipeline.HTTP
	case "grpc":
		return pipeline.GRPC
	case "prometheus_pull":
		return pipeline.PrometheusPull
	case "otlp":
		return pipeline.OTLP
	default:
		return pipeline.TCP
	}
}

func waitHealthy(ctx pipeline.Context, agent pipeline.Agent, c spec.Container, name string) error {
	interval := c.Healthcheck.Interval.Std()
	if interval <= 0 {
		interval = defaultHealthGap
	}
	retries := c.Healthcheck.Retries
	if retries <= 0 {
		retries = defaultRetries
	}
	req := activities.WaitHealthyRequest{Container: name, Cmd: c.Healthcheck.Cmd, Interval: interval, Retries: retries}
	_, err := activity.Activity(ctx, agent, activity.Fn(activities.NameWaitHealthy, activities.WaitHealthy, req),
		activity.WithTimeout(time.Duration(retries)*interval+healthExtra), activity.WithHeartbeat(interval*3+healthExtra))
	if err != nil {
		return fmt.Errorf("healthcheck: %w", err)
	}
	return nil
}

func agentsOf(ms []*provision.Machine) []activity.Target {
	out := make([]activity.Target, 0, len(ms))
	for _, m := range ms {
		out = append(out, m.Agent)
	}
	return out
}

func envList(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, k+"="+m[k])
	}
	return out
}

func bind(src, dst string, ro bool) string {
	s := src + ":" + dst
	if ro {
		s += ":ro"
	}
	return s
}

func restartPolicy(s string) container.RestartPolicyMode {
	switch s {
	case "no":
		return container.RestartPolicyDisabled
	case "always":
		return container.RestartPolicyAlways
	case "on-failure":
		return container.RestartPolicyOnFailure
	default:
		return container.RestartPolicyUnlessStopped
	}
}

// Container records share a Graphene namespace across all runs. The full run
// UUID separates identical logical container names, including concurrent cells.
func containerName(runID, logical string) string {
	return runID + "-" + logical
}
