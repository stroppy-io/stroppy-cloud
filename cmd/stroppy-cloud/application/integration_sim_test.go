//go:build integration

package application

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.temporal.io/sdk/client"
	tlog "go.temporal.io/sdk/log"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"

	"github.com/graphene-ci/library/docker/dockertest"
	"github.com/graphene-ci/library/k8s/k8stest"
	"github.com/graphene-ci/pipeline/pkg/id"
	"github.com/graphene-ci/pipeline/pkg/pipeline"
	"github.com/graphene-ci/pipeline/pkg/pipelinetest"
	"github.com/graphene-ci/pipeline/pkg/ref"
	"github.com/graphene-ci/pipeline/pkg/wire"

	managementv1 "github.com/graphene-ci/graphene/pkg/proto/management/v1"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
	"github.com/stroppy-io/stroppy-cloud/pipelines/workflows"
)

/*
SIMULATION: the REAL pipeline bodies (pipelines/workflows) executed on
Graphene's pipelinetest in virtual time. Only the cloud and the agents are
answered here: every Crossplane object turns Ready shortly after it is
declared, every agent connects after its declaration, agent activities
answer by scenario. What the run does in between — phases, milestones,
docker, artifacts, stand handover, cleanup — is the pipeline's own code.

The recording is shaped as Graphene's observe API translates a Temporal
history (graphene internal/services/observe.go): run-* and activity-*
kinds, the activity id is the scheduled event's id, the agent comes from
the "agent/<id>/run/<run>" queue, a milestone is `signal-received` with
subject `entity-note` and the note as a one-element JSON array.
*/

// simScenario is what goes wrong (or is held) in one simulated run.
type simScenario struct {
	// FailSegment ends that segment with a nonzero stroppy exit.
	FailSegment string
	// LoseSegment fails the segment activity itself (worker died).
	LoseSegment string
	// FailBaseline makes the baseline activity fail (never fatal).
	FailBaseline bool
	// FailHealthcheck is the spec name of a container that never turns healthy.
	FailHealthcheck string
	// FailDockerInstall breaks the package repository.
	FailDockerInstall bool
	// FailProviderConfig rejects the provider credentials.
	FailProviderConfig bool
	// FailMachine is a machine whose VM never converges.
	FailMachine string
	// AgentsNeverConnect leaves every agent offline.
	AgentsNeverConnect bool
	// FailArtifacts breaks every artifact upload (a warning, not a failure).
	FailArtifacts bool
	// TPS is the throughput the segments measure (0 = about 1180).
	TPS float64
	// Hold pauses the replay right after the first milestone of that name
	// until the test releases or cancels the run.
	Hold string
}

// simEvent is one recorded observe event at its virtual offset.
type simEvent struct {
	At time.Duration
	Ev *managementv1.Event
}

// simResource is a resource record with its lifecycle offsets (-1 = never).
type simResource struct {
	pipelinetest.Resource
	Declared, Ready, Deleted, Transferred time.Duration
	// Origin is the owner before a handover to the stand.
	Origin ref.OwnerRef
}

// simSegment is one workload segment as the runner lived it.
type simSegment struct {
	Name       string
	Index      int
	Agent      string
	Start, End time.Duration
	Status     spec.SegmentStatus
	Metrics    map[string]spec.MetricValue
}

// simChild is a child run a suite started, at its virtual offset.
type simChild struct {
	RunID string
	At    time.Duration
	Rec   *simRecord
}

// simRecord is everything one simulated run produced.
type simRecord struct {
	RunID     string
	Pipeline  string
	Params    json.RawMessage
	Scenario  simScenario
	Start     time.Time
	Duration  time.Duration
	Events    []simEvent
	Resources []simResource
	// Blobs are artifact bytes by blob location.
	Blobs map[string][]byte
	// Status is the terminal status Graphene reports.
	Status   string
	Result   json.RawMessage
	Error    string
	Segments []simSegment
	Children []simChild
	// CancelAt is the offset of the cancellation this record was made with.
	CancelAt time.Duration
}

// simChildScenario picks the scenario of a suite cell's child run.
type simChildScenario func(runID string, rs spec.Run) simScenario

// simT is the TestingT of a simulation outside a test goroutine: a fatal
// harness error aborts the simulation, not the test binary.
type simT struct {
	mu   sync.Mutex
	errs []string
}

type simAbort struct{}

func (s *simT) Helper() {}
func (s *simT) Errorf(f string, a ...any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errs = append(s.errs, fmt.Sprintf(f, a...))
}
func (s *simT) Fatalf(f string, a ...any) { s.Errorf(f, a...); panic(simAbort{}) }

// simulate runs one pipeline to its end in virtual time starting at start.
// cancelAt > 0 cancels the run at that offset (a re-simulation of a run
// the operator cancelled: the history is identical up to that instant).
func simulate(runID, pipelineID string, params json.RawMessage, start time.Time, sc simScenario, cancelAt time.Duration, childScenario simChildScenario) (rec *simRecord, err error) {
	st := &simT{}
	defer func() {
		if p := recover(); p != nil {
			if _, ok := p.(simAbort); !ok {
				panic(p)
			}
			err = fmt.Errorf("simulation of %s aborted: %s", runID, strings.Join(st.errs, "; "))
		}
	}()
	rec = &simRecord{RunID: runID, Pipeline: pipelineID, Params: params, Scenario: sc, Start: start, Blobs: map[string][]byte{}, CancelAt: cancelAt}
	rr := &simRecorder{rec: rec}
	var ts testsuite.WorkflowTestSuite
	ts.SetLogger(tlog.NewStructuredLogger(slog.New(slog.DiscardHandler)))
	env := ts.NewTestWorkflowEnvironment()
	w := pipelinetest.Install(st, env)
	rr.w = w
	env.SetTestTimeout(2 * time.Minute)
	env.SetStartTime(start)
	// Milestones are read back from the emit dispatches (see history).
	w.Handle(workflows.ActivityEmit, func(workflow.Context, []any) (any, error) { return nil, nil })

	var wf any
	var input any
	switch pipelineID {
	case workflows.RunPipelineID:
		var rs spec.Run
		if err := json.Unmarshal(params, &rs); err != nil {
			return nil, fmt.Errorf("run spec: %w", err)
		}
		simCloud(w, sc)
		simAgents(w, rr, sc)
		wf, input = pipelinetest.Workflow(w, workflows.RunPipelineID, workflows.Run), rs
	case workflows.SuitePipelineID:
		var s spec.Suite
		if err := json.Unmarshal(params, &s); err != nil {
			return nil, fmt.Errorf("suite spec: %w", err)
		}
		if err := simChildren(w, rr, runID, s, start, childScenario); err != nil {
			return nil, err
		}
		wf, input = pipelinetest.Workflow(w, workflows.SuitePipelineID, workflows.Suite), s
	default:
		return nil, fmt.Errorf("pipeline %s is not simulated", pipelineID)
	}
	env.SetStartWorkflowOptions(client.StartWorkflowOptions{ID: "run/" + runID, TaskQueue: "run/" + runID})
	rawIn, _ := json.Marshal([]any{input}) //nolint:errcheck // spec types
	rr.edge(start, false, &managementv1.Event{Kind: "run-started", Input: rawIn})
	if cancelAt > 0 {
		env.RegisterDelayedCallback(env.CancelWorkflow, cancelAt)
	}
	env.ExecuteWorkflow(wf, input)
	end := env.Now()
	rec.Duration = end.Sub(start)

	werr := env.GetWorkflowError()
	var canceled *temporal.CanceledError
	switch {
	case werr == nil:
		var out json.RawMessage
		if err := env.GetWorkflowResult(&out); err != nil {
			return nil, err
		}
		rec.Status, rec.Result = "completed", out
		result, _ := json.Marshal([]json.RawMessage{out}) //nolint:errcheck // raw json
		rr.edge(end, true, &managementv1.Event{Kind: "run-completed", Status: "completed", Result: result})
	case errors.As(werr, &canceled) || (cancelAt > 0 && strings.Contains(werr.Error(), "canceled")):
		rec.Status = "canceled"
		rr.edge(end, true, &managementv1.Event{Kind: "run-canceled", Status: "canceled"})
	default:
		// A failed run closes with the partial result in the failure's
		// details (pipeline.FailureType), the way Graphene reads it back.
		rec.Status, rec.Error = "failed", rootCause(werr)
		rec.Result = partialResult(pipelineID, werr)
		rr.edge(end, true, &managementv1.Event{Kind: "run-failed", Status: "failed", Error: rec.Error})
	}
	// Children were simulated from the suite's start for their durations;
	// now that their real start offsets are known, live them again there.
	for i, ch := range rec.Children {
		sc := simScenario{}
		var cell spec.Run
		if childScenario != nil && json.Unmarshal(ch.Rec.Params, &cell) == nil {
			sc = childScenario(ch.RunID, cell)
		}
		child, err := simulate(ch.RunID, workflows.RunPipelineID, ch.Rec.Params, start.Add(ch.At), sc, 0, nil)
		if err != nil {
			return nil, err
		}
		rec.Children[i].Rec = child
	}
	rec.Resources = simResources(w, start)
	rr.history(start)
	for _, r := range rec.Resources {
		if !strings.HasPrefix(string(r.Ref), "artifact/") {
			continue
		}
		var state pipeline.ArtifactState
		if json.Unmarshal(r.State, &state) == nil {
			if data, ok := w.Blob(state.Blob); ok {
				rec.Blobs[state.Blob.Location] = data
			}
		}
	}
	return rec, nil
}

// partialResult is what a failed run collected before it ended: the
// pipeline closes with the result in the failure's details. (In the test
// environment a detail is the Go value itself, so it is read into the
// pipeline's own result type and marshalled here; over the wire Graphene
// reads the same payload as JSON.)
func partialResult(pipelineID string, err error) json.RawMessage {
	var app *temporal.ApplicationError
	if !errors.As(err, &app) || !app.HasDetails() {
		return nil
	}
	var out any
	switch pipelineID {
	case workflows.RunPipelineID:
		out = &spec.Result{}
	case workflows.SuitePipelineID:
		out = &spec.SuiteResult{}
	default:
		return nil
	}
	if app.Details(out) != nil {
		return nil
	}
	raw, marshalErr := json.Marshal(out)
	if marshalErr != nil || string(raw) == "null" {
		return nil
	}
	return raw
}

// rootCause strips Temporal's wrapping down to the pipeline's message.
func rootCause(err error) string {
	var app *temporal.ApplicationError
	if errors.As(err, &app) {
		return app.Error()
	}
	return err.Error()
}

// simResources folds the world's records with their lifecycle offsets.
func simResources(w *pipelinetest.World, start time.Time) []simResource {
	byRef := map[ref.OwnerRef]*simResource{}
	var order []ref.OwnerRef
	for _, live := range w.Resources() {
		r := &simResource{Resource: live, Declared: -1, Ready: -1, Deleted: -1, Transferred: -1, Origin: live.Owner}
		if live.From != "" {
			r.Origin = live.From
		}
		byRef[live.Ref] = r
		order = append(order, live.Ref)
	}
	for _, e := range w.Events() {
		r := byRef[e.Resource]
		if r == nil {
			continue
		}
		at := e.Time.Sub(start)
		switch e.Action {
		case "declare":
			r.Declared = at
		case "ready":
			r.Ready = at
		case "delete":
			r.Deleted = at
		case "transfer":
			r.Transferred = at
		}
	}
	out := make([]simResource, 0, len(order))
	for _, name := range order {
		out = append(out, *byRef[name])
	}
	return out
}

// --- recording ---------------------------------------------------------------

/*
The history is assembled from World.Calls after the run: every activity
dispatch with its queue, payload and outcome, ordered by one counter over
dispatches and completions (virtual time stands still inside a workflow
task). A milestone is the signal the emit activity sends: it lands between
the emit's dispatch and its completion.
*/

type simRecorder struct {
	mu  sync.Mutex
	w   *pipelinetest.World
	rec *simRecord
	// start/end are the run's first and last events.
	first, last *managementv1.Event
	end         time.Time
}

// edge records the run's first or last event.
func (r *simRecorder) edge(at time.Time, last bool, ev *managementv1.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if last {
		r.last, r.end = ev, at
		return
	}
	r.first = ev
}

func (r *simRecorder) segment(s simSegment) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rec.Segments = append(r.rec.Segments, s)
}

// history turns the dispatches into the observe event list.
func (r *simRecorder) history(start time.Time) {
	type item struct {
		at    time.Time
		order float64
		ev    *managementv1.Event
		call  int
	}
	items := []item{{at: start, order: -1, ev: r.first, call: -1}}
	calls := r.w.Calls()
	for i, c := range calls {
		agent := agentOfQueue(c.TaskQueue)
		seq := float64(c.Seq)
		items = append(items,
			item{at: c.Time, order: seq, ev: &managementv1.Event{Kind: "activity-scheduled", Subject: c.Name, Agent: agent, Input: c.Args}, call: i},
			item{at: c.Time, order: seq + 0.1, ev: &managementv1.Event{Kind: "activity-started", Subject: c.Name, Agent: agent, Attempt: 1}, call: i})
		if c.Name == workflows.ActivityEmit {
			// The milestone the emit carries is an event of its own: kind
			// note, the milestone's name as the subject, its payload as
			// the input.
			var args []struct {
				Name    string          `json:"name"`
				Payload json.RawMessage `json:"payload,omitempty"`
			}
			if json.Unmarshal(c.Args, &args) == nil && len(args) == 1 && args[0].Name != "" {
				items = append(items, item{at: c.Time, order: seq + 0.2, ev: &managementv1.Event{Kind: "note", Subject: args[0].Name, Input: args[0].Payload}, call: -1})
			}
		}
		if c.Done.IsZero() || c.Canceled {
			continue
		}
		ev := &managementv1.Event{Kind: "activity-completed", Subject: c.Name, Agent: agent}
		switch {
		case c.Err != "" && strings.Contains(strings.ToLower(c.Err), "timeout"):
			ev.Kind, ev.Error = "activity-timed-out", c.Err
		case c.Err != "":
			ev.Kind, ev.Error = "activity-failed", c.Err
		case len(c.Result) > 0:
			ev.Result, _ = json.Marshal([]json.RawMessage{c.Result}) //nolint:errcheck // raw json
		}
		items = append(items, item{at: c.Done, order: float64(c.DoneSeq), ev: ev, call: i})
	}
	if r.last != nil {
		items = append(items, item{at: r.end, order: math.Inf(1), ev: r.last, call: -1})
	}
	sort.SliceStable(items, func(a, b int) bool {
		if !items[a].at.Equal(items[b].at) {
			return items[a].at.Before(items[b].at)
		}
		return items[a].order < items[b].order
	})
	ids := map[int]string{}
	for n, it := range items {
		it.ev.EventId = int64(n + 1)
		it.ev.TimeUnixNano = it.at.UnixNano()
		if it.call >= 0 {
			if it.ev.Kind == "activity-scheduled" {
				ids[it.call] = strconv.FormatInt(it.ev.EventId, 10)
			}
			it.ev.ActivityId = ids[it.call]
		}
		r.rec.Events = append(r.rec.Events, simEvent{At: it.at.Sub(start), Ev: it.ev})
	}
}

// agentOfQueue extracts the agent from an "agent/<id>/run/<run>" queue.
func agentOfQueue(queue string) string {
	if rest, ok := strings.CutPrefix(queue, "agent/"); ok {
		if agent, _, ok := strings.Cut(rest, "/run/"); ok {
			return agent
		}
	}
	return ""
}

// --- the cloud ---------------------------------------------------------------

// simCloud converges every declared Crossplane object and connects every
// declared agent. Nothing here knows the names a RunSpec will produce: the
// world reports each record as it is declared. One observed object
// satisfies every kind's readiness predicate (typed decoding ignores the
// fields a kind does not have).
func simCloud(w *pipelinetest.World, sc simScenario) {
	objects := k8stest.Install(w)
	dockertest.Install(w, "28.5.2")
	if sc.FailDockerInstall {
		w.Handle("docker.install", func(workflow.Context, []any) (any, error) {
			return nil, errors.New("E: Unable to locate package docker-ce: package repository unavailable")
		})
	}
	machines := 0
	w.OnDeclare(func(r pipelinetest.Resource) {
		name := string(r.Ref)
		switch {
		case strings.HasPrefix(name, "agent/"):
			if sc.AgentsNeverConnect {
				return
			}
			agent := id.AgentId(strings.TrimPrefix(name, "agent/"))
			w.Env.RegisterDelayedCallback(func() { w.Connect(agent) }, agentConnects)
		case strings.HasPrefix(name, "k8s."):
			kind, _, _ := strings.Cut(name, "/")
			delay, ip, public := 3*time.Second, "", ""
			switch {
			case strings.HasSuffix(kind, ".Instance"):
				machines++
				delay = 40 * time.Second
				ip, public = fmt.Sprintf("10.130.0.%d", 9+machines), fmt.Sprintf("84.201.%d.%d", 140+machines/250, 10+machines%250)
				if sc.FailMachine != "" && strings.HasSuffix(name, "-"+sc.FailMachine) {
					w.FailResource(r.Ref, errors.New("Quota limit vpc.externalAddresses.count exceeded"))
					return
				}
			case strings.HasSuffix(kind, ".Disk"):
				delay = 8 * time.Second
			case strings.Contains(kind, ".ydb."):
				delay = 4 * time.Minute
			}
			live := simReadyObject(name, ip, public)
			w.Env.RegisterDelayedCallback(func() { _ = objects.Set(r.Ref, live) }, delay) //nolint:errcheck // static fixture
		}
	})
}

// agentConnects is how long a machine takes to boot and its agent to reach
// the server after the VM runs.
const agentConnects = 75 * time.Second

func simReadyObject(name, ip, public string) map[string]any {
	short := name[strings.LastIndexByte(name, '/')+1:]
	at := map[string]any{
		"id": "sim-" + short, "status": "running", "instanceState": "running",
		"ydbFullEndpoint": "grpcs://ydb.serverless.yandexcloud.net:2135/?database=/ru-central1/b1gsim/" + short,
	}
	if ip != "" {
		at["networkInterface"] = []any{map[string]any{"ipAddress": ip, "natIpAddress": public}}
		at["privateIp"], at["publicIp"] = ip, public
	}
	return map[string]any{"status": map[string]any{
		"conditions": []any{map[string]any{"type": "Ready", "status": "True", "reason": "Available"}},
		"atProvider": at,
	}}
}

// --- the agents --------------------------------------------------------------

// simAgents answers the pipeline's own activities: worker-side ones on
// the run queue and the machine ones on each agent.
func simAgents(w *pipelinetest.World, rr *simRecorder, sc simScenario) {
	pipelinetest.Handle1(w, workflows.ActivityResolveYandexImages, func(_ workflow.Context, req workflows.ResolveYandexImagesRequest) (workflows.ResolveYandexImagesResult, error) {
		images := make(map[string]string, len(req.Images))
		for _, family := range req.Images {
			images[family] = "fd8sim" + fmt.Sprintf("%014x", len(family))
		}
		return workflows.ResolveYandexImagesResult{Images: images}, nil
	})
	pipelinetest.Handle1(w, workflows.ActivityEnsureProviderConfig, func(ctx workflow.Context, req workflows.EnsureProviderConfigRequest) (workflows.EnsureProviderConfigResult, error) {
		if err := workflow.Sleep(ctx, 2*time.Second); err != nil {
			return workflows.EnsureProviderConfigResult{}, err
		}
		if sc.FailProviderConfig {
			return workflows.EnsureProviderConfigResult{}, temporal.NewNonRetryableApplicationError("provider credentials rejected: PermissionDenied: iam.serviceAccounts.use", "CredentialsRejected", nil)
		}
		return workflows.EnsureProviderConfigResult{SecretName: req.Name, Applied: true}, nil
	})
	pipelinetest.Handle1(w, workflows.ActivityHostPrep, func(ctx workflow.Context, _ workflows.HostPrepRequest) (workflows.HostPrepResult, error) {
		return workflows.HostPrepResult{}, workflow.Sleep(ctx, 4*time.Second)
	})
	pipelinetest.Handle1(w, workflows.ActivityWriteFiles, func(_ workflow.Context, req workflows.WriteFilesRequest) (workflows.WriteFilesResult, error) {
		paths := make([]string, 0, len(req.Files))
		for _, f := range req.Files {
			paths = append(paths, "/ws/"+req.Dir+"/"+f.Path)
		}
		return workflows.WriteFilesResult{Paths: paths}, nil
	})
	pipelinetest.Handle1(w, workflows.ActivityPullImage, func(ctx workflow.Context, req workflows.PullImageRequest) (workflows.PullImageResult, error) {
		return workflows.PullImageResult{Image: req.Image}, workflow.Sleep(ctx, 12*time.Second)
	})
	pipelinetest.Handle1(w, workflows.ActivityWaitHealthy, func(ctx workflow.Context, req workflows.WaitHealthyRequest) (workflows.WaitHealthyResult, error) {
		if sc.FailHealthcheck != "" && strings.HasSuffix(req.Container, "-"+sc.FailHealthcheck) {
			if err := workflow.Sleep(ctx, 50*time.Second); err != nil {
				return workflows.WaitHealthyResult{}, err
			}
			return workflows.WaitHealthyResult{}, temporal.NewNonRetryableApplicationError(req.Container+" is unhealthy after 10 attempts: exit status 2", "Unhealthy", nil)
		}
		return workflows.WaitHealthyResult{Attempts: 3}, workflow.Sleep(ctx, 10*time.Second)
	})
	pipelinetest.Handle1(w, workflows.ActivityWaitDatabase, func(ctx workflow.Context, req workflows.WaitDatabaseRequest) (workflows.WaitDatabaseResult, error) {
		if err := workflow.Sleep(ctx, 20*time.Second); err != nil {
			return workflows.WaitDatabaseResult{}, err
		}
		agent := pipelinetest.Agent(ctx)
		dir := "/ws/stroppy/managed-ydb-ready"
		w.File(agent, dir+"/stroppy-config.json", []byte(`{"version":"1","driver":"ydb","segment":"managed-ydb-ready"}`))
		w.File(agent, dir+"/stroppy.log", []byte("tcp ok\ntls ok\nSELECT 1 ok\n"))
		return workflows.WaitDatabaseResult{
			Address: "ydb.serverless.yandexcloud.net:2135", Attempts: 2, QueryAttempts: 1, Ready: true,
			ConfigPath: dir + "/stroppy-config.json", LogPath: dir + "/stroppy.log",
		}, nil
	})
	pipelinetest.Handle1(w, workflows.ActivityRunBaseline, func(ctx workflow.Context, req workflows.RunBaselineRequest) (workflows.RunBaselineResult, error) {
		if err := workflow.Sleep(ctx, 25*time.Second); err != nil {
			return workflows.RunBaselineResult{}, err
		}
		if sc.FailBaseline {
			return workflows.RunBaselineResult{}, temporal.NewNonRetryableApplicationError("stroppy baseline: exit status 1: pg-noop listener failed", "Baseline", nil)
		}
		path := "/ws/stroppy/baseline/stroppy.log"
		w.File(pipelinetest.Agent(ctx), path, []byte("stroppy baseline "+strings.Join(req.Baseline.Tiers, ",")+"\nnoop errors: ok\nnoop throughput: ok\n"))
		return workflows.RunBaselineResult{
			Result: spec.BaselineResult{OK: true, Verdicts: []spec.BaselineVerdict{
				{Check: "noop errors", Status: "ok"}, {Check: "noop throughput", Status: "ok", Detail: "412k it/s"},
			}, Report: json.RawMessage(`{"schema":1,"stroppy_version":"v6.0.0","tiers":["noop"]}`)},
			LogPath: path,
		}, nil
	})
	pipelinetest.Handle1(w, workflows.ActivityRunSegment, func(ctx workflow.Context, req workflows.RunSegmentRequest) (workflows.RunSegmentResult, error) {
		agent := string(pipelinetest.Agent(ctx))
		seg := req.Segment
		d := simSegmentDuration(seg)
		start := workflow.Now(ctx)
		if sc.LoseSegment == seg.Name {
			_ = workflow.Sleep(ctx, d/2) //nolint:errcheck // lost either way
			return workflows.RunSegmentResult{}, temporal.NewTimeoutError(0, errors.New("activity heartbeat timeout: agent "+agent+" went away"))
		}
		if err := workflow.Sleep(ctx, d); err != nil {
			return workflows.RunSegmentResult{}, err
		}
		end := workflow.Now(ctx)
		res := spec.SegmentResult{Name: seg.Name, Status: spec.SegmentCompleted, StartedAt: start, FinishedAt: end, Metrics: simSegmentMetrics(seg, d, sc.TPS)}
		res.Errors = &spec.ErrorCounts{FailedQueries: 3, RetryAttempts: 3}
		if sc.FailSegment == seg.Name {
			res.Status, res.ExitCode = spec.SegmentFailed, 1
			res.Error = "stroppy exit status 1: ERROR: relation \"warehouse\" does not exist (SQLSTATE 42P01)"
			res.Errors.TerminalErrors, res.Errors.FailedIterations = 1, 17
		}
		dir := fmt.Sprintf("/ws/stroppy/%02d-%s", req.Index, seg.Name)
		cfg, _ := json.MarshalIndent(map[string]any{"version": "1", "driver": req.Workload.DriverType, "segment": seg.Name, "script": seg.Workload.Script}, "", "  ") //nolint:errcheck // plain map
		w.File(id.AgentId(agent), dir+"/stroppy-config.json", cfg)
		w.File(id.AgentId(agent), dir+"/stroppy.log", []byte(simStroppyLog(seg, res)))
		rr.segment(simSegment{Name: seg.Name, Index: req.Index, Agent: agent, Start: start.Sub(rr.rec.Start), End: end.Sub(rr.rec.Start), Status: res.Status, Metrics: res.Metrics})
		return workflows.RunSegmentResult{Result: res, ConfigPath: dir + "/stroppy-config.json", LogPath: dir + "/stroppy.log"}, nil
	})
	if sc.FailArtifacts {
		w.Handle("graphene.blob.upload-file", func(workflow.Context, []any) (any, error) {
			return nil, errors.New("blob store: 503 Service Unavailable")
		})
	}
}

// simSegmentDuration is how long the segment "runs".
func simSegmentDuration(seg spec.Segment) time.Duration {
	switch {
	case seg.Run.Duration > 0:
		return seg.Run.Duration.Std() + 3*time.Second
	case seg.Run.Iterations > 0:
		return time.Duration(max(10, seg.Run.Iterations/500)) * time.Second
	}
	return 45 * time.Second
}

// simSegmentMetrics are shaped like a real postgres tpcc segment result.
func simSegmentMetrics(seg spec.Segment, d time.Duration, want float64) map[string]spec.MetricValue {
	secs := d.Seconds()
	tps := 1180.0 + 40*math.Sin(float64(len(seg.Name)))
	if want > 0 {
		tps = want
	}
	iterations := math.Round(tps * secs)
	// Latency moves against throughput (the same VUs, Little's law).
	scale := 1180 / tps
	ms := func(v float64) spec.MetricValue {
		return spec.MetricValue{Value: math.Round(v*scale*1000) / 1000, Unit: "ms"}
	}
	n := func(v float64) spec.MetricValue { return spec.MetricValue{Value: v} }
	return map[string]spec.MetricValue{
		"tps":                    n(math.Round(tps*10) / 10),
		"iterations_total":       n(iterations),
		"iteration_duration_avg": ms(3.42), "iteration_duration_count": n(iterations),
		"iteration_duration_p50": ms(2.5), "iteration_duration_p90": ms(5), "iteration_duration_p95": ms(7.5), "iteration_duration_p99": ms(12.5),
		"run_query_duration_avg": ms(0.41), "run_query_duration_count": n(iterations * 4),
		"run_query_duration_p50": ms(0.5), "run_query_duration_p95": ms(1), "run_query_duration_p99": ms(2.5),
		"run_query_operations_total": n(iterations * 4),
		"failed_iterations_total":    n(0), "failed_queries_total": n(3), "terminal_errors_total": n(0), "retry_attempts_total": n(3),
	}
}

func simStroppyLog(seg spec.Segment, res spec.SegmentResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "stroppy v6.0.0 run %s (%s)\n", seg.Workload.Script, seg.Name)
	keys := make([]string, 0, len(res.Metrics))
	for k := range res.Metrics {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	b.WriteString("=== bench summary ===\n")
	for _, k := range keys {
		fmt.Fprintf(&b, "%-28s %v %s\n", k, res.Metrics[k].Value, res.Metrics[k].Unit)
	}
	if res.Error != "" {
		b.WriteString(res.Error + "\n")
	}
	return b.String()
}

// --- suites ------------------------------------------------------------------

// simChildren answers the child-run contract with pre-simulated runs of
// every cell: start-child records the child at its virtual offset,
// await-child returns once the child's own duration has passed.
func simChildren(w *pipelinetest.World, rr *simRecorder, suiteID string, s spec.Suite, start time.Time, childScenario simChildScenario) error {
	pre := map[string]*simRecord{}
	for _, c := range s.Cells {
		childID := suiteID + "-" + c.ID
		sc := simScenario{}
		if childScenario != nil {
			sc = childScenario(childID, c.RunSpec)
		}
		params, err := json.Marshal(c.RunSpec)
		if err != nil {
			return err
		}
		// Simulated from the suite's start; shifted when it really starts.
		child, err := simulate(childID, workflows.RunPipelineID, params, start, sc, 0, nil)
		if err != nil {
			return err
		}
		pre[childID] = child
	}
	started := map[string]time.Time{}
	pipelinetest.Handle1(w, wire.StartChildRunActivity, func(ctx workflow.Context, req wire.StartChildRunRequest) (any, error) {
		child, ok := pre[string(req.RunId)]
		if !ok {
			return nil, fmt.Errorf("unknown child %s", req.RunId)
		}
		now := workflow.Now(ctx)
		started[string(req.RunId)] = now
		rr.mu.Lock()
		rr.rec.Children = append(rr.rec.Children, simChild{RunID: string(req.RunId), At: now.Sub(start), Rec: child})
		rr.mu.Unlock()
		return nil, nil
	})
	pipelinetest.Handle1(w, wire.AwaitChildRunActivity, func(ctx workflow.Context, req wire.AwaitChildRunRequest) (any, error) {
		child := pre[string(req.RunId)]
		if child == nil {
			return nil, fmt.Errorf("unknown child %s", req.RunId)
		}
		if wait := started[string(req.RunId)].Add(child.Duration).Sub(workflow.Now(ctx)); wait > 0 {
			if err := workflow.Sleep(ctx, wait); err != nil {
				return nil, err
			}
		}
		if child.Status != "completed" {
			return nil, temporal.NewNonRetryableApplicationError("child run "+string(req.RunId)+" "+child.Status+": "+child.Error, "ChildFailed", nil)
		}
		return child.Result, nil
	})
	return nil
}
