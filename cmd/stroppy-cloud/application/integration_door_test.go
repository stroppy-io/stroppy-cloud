//go:build integration

package application

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"

	managementv1 "github.com/graphene-ci/graphene/pkg/proto/management/v1"
	"github.com/graphene-ci/graphene/pkg/proto/management/v1/managementv1connect"
	"github.com/graphene-ci/pipeline/pkg/pipeline"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/provider"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/graphene"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
	"github.com/stroppy-io/stroppy-cloud/pipelines/workflows"
)

/*
DOOR: the Connect half of a Graphene installation, in memory, answering
the way graphene internal/services does. stroppy-run and stroppy-suite
are the REAL pipelines (integration_sim_test.go): StartRun simulates the
whole run at once and the door replays it on a virtual clock —

  - speed > 0: virtual time runs `speed` times faster than the wall clock
    (1 = real time, for a stand a person clicks through);
  - speed = 0: everything is visible at once; stand deadlines move only
    by advance().

A scenario may hold the replay after a milestone; release() lets it go
on, cancel re-simulates the run cancelled at that instant (the history up
to it is identical — the run is deterministic) and continues from there.

The probe pipelines answer with the canned results tests set.
*/

// fakeGraphene is the Graphene door of the harness.
type fakeGraphene struct {
	managementv1connect.UnimplementedResourcesAPIHandler
	managementv1connect.UnimplementedSecretsAPIHandler
	managementv1connect.UnimplementedRunsAPIHandler
	managementv1connect.UnimplementedRbacAPIHandler
	managementv1connect.UnimplementedObserveAPIHandler

	mu         sync.Mutex
	namespaces map[string]bool
	secrets    map[string]string // "<ns>/<name>" → value
	runs       map[string]*doorRun
	commands   []string // "<ref> <command>" of every Invoke
	deleted    []string // refs deleted
	// speed is virtual seconds per wall second; 0 = instant.
	speed float64
	// advanced moves stand deadlines forward (instant mode).
	advanced time.Duration
	// scenario picks what goes wrong in a run (nil = defaultScenario).
	scenario func(runID string, params json.RawMessage) simScenario
	// defaultScenario applies to every run scenario does not pick.
	defaultScenario simScenario
	// labelScenarios lets a run's `sim` label pick its scenario (the
	// local stand: a person chooses what goes wrong from the UI).
	labelScenarios bool
	// recorded is told about every simulation (telemetry, tests).
	recorded func(r *doorRun)
	// probes: canned answers of the probe pipelines.
	verifyOK               bool
	quotas                 []provider.Quota
	quotaUnavailableReason string
	quotaError             error
	configResultError      error
	configDeleteFailure    string
}

// doorRun is one run the door knows, with its replay state.
type doorRun struct {
	id, namespace, pipeline string
	params                  json.RawMessage
	labels                  map[string]string
	parent                  string
	rec                     *simRecord
	// wall0 is the wall time of the run's virtual start.
	wall0 time.Time
	// hold is the index of the last visible event while held (-1 = none).
	hold int
	// keepUntil overrides stand deadlines (zero time = no deadline).
	keepUntil map[string]time.Time
	// released are stand holdings torn down, at their virtual instant.
	released map[string]time.Time
	err      error
}

func newFakeGraphene() *fakeGraphene {
	return &fakeGraphene{
		namespaces: map[string]bool{}, secrets: map[string]string{}, runs: map[string]*doorRun{},
		verifyOK: true, quotas: []provider.Quota{{Name: "compute.instanceCores.count", Limit: 32, Used: 4, Unit: "cores"}},
	}
}

func nsOf(req connect.AnyRequest) string { return req.Header().Get(graphene.NamespaceHeader) }

// --- the virtual clock -------------------------------------------------------

// vnow is the virtual instant of the run now.
func (f *fakeGraphene) vnow(r *doorRun) time.Time {
	if r.rec == nil {
		return time.Now()
	}
	if f.speed <= 0 {
		return r.rec.Start.Add(r.rec.Duration + f.advanced)
	}
	return r.rec.Start.Add(time.Duration(float64(time.Since(r.wall0))*f.speed) + f.advanced)
}

// clock is the virtual instant of a run's family (a suite and its
// children share the root's): a held root stops it; instant mode has no
// bound (ok = false).
func (f *fakeGraphene) clock(r *doorRun) (time.Time, bool) {
	root := r
	for root.parent != "" && f.runs[root.parent] != nil {
		root = f.runs[root.parent]
	}
	if root.rec == nil {
		return time.Time{}, false
	}
	if root.hold >= 0 {
		return root.rec.Start.Add(root.rec.Events[root.hold].At), true
	}
	if f.speed <= 0 {
		return time.Time{}, false
	}
	return root.rec.Start.Add(time.Duration(float64(time.Since(root.wall0)) * f.speed)), true
}

// visible is how many events of the run have happened.
func (f *fakeGraphene) visible(r *doorRun) int {
	if r.rec == nil {
		return 0
	}
	n := len(r.rec.Events)
	if at, ok := f.clock(r); ok {
		n = sort.Search(len(r.rec.Events), func(i int) bool { return r.rec.Start.Add(r.rec.Events[i].At).After(at) })
	}
	if r.hold >= 0 && n > r.hold+1 {
		n = r.hold + 1
	}
	return n
}

// finished reports the run's last event visible.
func (f *fakeGraphene) finished(r *doorRun) bool {
	return r.rec != nil && f.visible(r) == len(r.rec.Events)
}

// started reports whether a child run exists yet.
func (f *fakeGraphene) started(r *doorRun) bool {
	if r.parent == "" || r.rec == nil {
		return true
	}
	at, ok := f.clock(r)
	return !ok || !r.rec.Start.After(at)
}

// lookup finds a run of the namespace (children appear once started).
func (f *fakeGraphene) lookup(ns, id string) (*doorRun, bool) {
	r, ok := f.runs[id]
	if !ok || (ns != "" && r.namespace != ns) || !f.started(r) {
		return nil, false
	}
	return r, true
}

// --- simulation --------------------------------------------------------------

// childScenario picks a suite cell's scenario by its run spec.
func (f *fakeGraphene) childScenario(runID string, rs spec.Run) simScenario {
	raw, _ := json.Marshal(rs) //nolint:errcheck // spec type
	return f.scenarioOf(runID, raw)
}

func (f *fakeGraphene) scenarioOf(runID string, params json.RawMessage) simScenario {
	if f.scenario == nil {
		return f.defaultScenario
	}
	return f.scenario(runID, params)
}

// install registers a simulated run (and its children) at a wall start.
func (f *fakeGraphene) install(r *doorRun, rec *simRecord, wall0 time.Time) {
	r.rec, r.wall0, r.hold = rec, wall0, -1
	if r.keepUntil == nil {
		r.keepUntil, r.released = map[string]time.Time{}, map[string]time.Time{}
	}
	if hold := rec.Scenario.Hold; hold != "" {
		for i, e := range rec.Events {
			if e.Ev.GetKind() == "note" && e.Ev.GetSubject() == hold {
				r.hold = i
				break
			}
		}
	}
	f.runs[r.id] = r
	// A re-lived run (cancelled) may not have started every child.
	kept := map[string]bool{}
	for _, ch := range rec.Children {
		kept[ch.RunID] = true
	}
	for id, other := range f.runs {
		if other.parent == r.id && !kept[id] {
			delete(f.runs, id)
		}
	}
	for _, ch := range rec.Children {
		child := f.runs[ch.RunID]
		if child == nil {
			child = &doorRun{
				id: ch.RunID, namespace: r.namespace, pipeline: workflows.RunPipelineID, params: ch.Rec.Params, parent: r.id,
				labels: map[string]string{"stroppy.io/suite-run": r.id},
			}
		}
		f.install(child, ch.Rec, wall0.Add(f.wallOf(ch.At)))
	}
}

// wallOf converts a virtual span to wall time.
func (f *fakeGraphene) wallOf(d time.Duration) time.Duration {
	if f.speed <= 0 {
		return 0
	}
	return time.Duration(float64(d) / f.speed)
}

func (f *fakeGraphene) StartRun(_ context.Context, req *connect.Request[managementv1.StartRunRequest]) (*connect.Response[managementv1.StartRunResponse], error) {
	id, pipelineID, params := req.Msg.GetRunId(), req.Msg.GetPipeline(), req.Msg.GetParams()
	f.mu.Lock()
	if _, exists := f.runs[id]; exists {
		f.mu.Unlock()
		return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("run/%s already exists", id))
	}
	r := &doorRun{id: id, namespace: nsOf(req), pipeline: pipelineID, params: params, labels: req.Msg.GetLabels(), hold: -1}
	f.runs[id] = r
	sc := f.scenarioOf(id, params)
	if v := req.Msg.GetLabels()["sim"]; f.labelScenarios && v != "" {
		sc = parseSimLabel(v)
	}
	childScenario := f.childScenario
	f.mu.Unlock()

	switch pipelineID {
	case workflows.RunPipelineID, workflows.SuitePipelineID:
		now := time.Now().UTC()
		rec, err := simulate(id, pipelineID, params, now, sc, 0, childScenario)
		if err == nil && f.speed <= 0 {
			// Instant: the run has already happened — it ends now, so
			// its telemetry lies in the past like a finished run's.
			// (Durations do not depend on the start: live it again there.)
			now = now.Add(-rec.Duration - time.Second)
			rec, err = simulate(id, pipelineID, params, now, sc, 0, childScenario)
		}
		f.mu.Lock()
		if err != nil {
			r.err = err
			f.mu.Unlock()
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		f.install(r, rec, now)
		f.mu.Unlock()
		f.notify(r)
	}
	return connect.NewResponse(&managementv1.StartRunResponse{WorkflowId: "run/" + id}), nil
}

// notify tells the observer about a run and its children.
func (f *fakeGraphene) notify(r *doorRun) {
	if f.recorded == nil || r.rec == nil {
		return
	}
	f.recorded(r)
	for _, ch := range r.rec.Children {
		f.mu.Lock()
		child := f.runs[ch.RunID]
		f.mu.Unlock()
		if child != nil {
			f.notify(child)
		}
	}
}

// hold makes every later run pause after the milestone ("" = run through).
func (f *fakeGraphene) hold(milestone string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.defaultScenario.Hold = milestone
}

// withTPS makes every later run measure that throughput.
func (f *fakeGraphene) withTPS(tps float64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.defaultScenario.TPS = tps
}

// release lets a held run go on.
func (f *fakeGraphene) release(runID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r := f.runs[runID]; r != nil && r.hold >= 0 {
		// The replay resumes where it stopped: shift the clock so the
		// held instant is now.
		if f.speed > 0 {
			held := r.rec.Events[r.hold].At
			r.wall0 = time.Now().Add(-f.wallOf(held))
		}
		r.hold = -1
	}
}

// replaceResult swaps a completed run's result for a fixture (contract
// tests push a complete native result through the projection).
func (f *fakeGraphene) replaceResult(runID string, raw json.RawMessage) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r := f.runs[runID]
	if r == nil || r.rec == nil || r.rec.Status != "completed" {
		panic("replaceResult needs a completed simulated run")
	}
	r.rec.Result = raw
	last := r.rec.Events[len(r.rec.Events)-1].Ev
	last.Result, _ = json.Marshal([]json.RawMessage{raw}) //nolint:errcheck // raw json
}

// advance moves every stand deadline check forward (instant mode).
func (f *fakeGraphene) advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.advanced += d
}

// dropBlobs forgets the bytes of every artifact of a run, as retention
// on the Graphene side eventually does, leaving the records behind.
func (f *fakeGraphene) dropBlobs(runID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r := f.runs[runID]; r != nil && r.rec != nil {
		r.rec.Blobs = map[string][]byte{}
	}
}

// cancel cancels the run at its current instant.
func (f *fakeGraphene) cancel(r *doorRun) error {
	if r.rec == nil || f.finished(r) {
		return nil
	}
	at := time.Duration(0)
	if shown := f.visible(r); shown > 0 {
		at = r.rec.Events[shown-1].At
	}
	return f.cancelAt(r, r.rec.Start.Add(at+time.Millisecond))
}

// cancelAt re-simulates the run cancelled at the virtual instant t; the
// history up to t is identical (the run is deterministic), so what was
// already delivered stays valid. A suite's children that were running at
// t are cancelled with it — Graphene cascades a run's cleanup to the runs
// it owns.
func (f *fakeGraphene) cancelAt(r *doorRun, t time.Time) error {
	offset := t.Sub(r.rec.Start)
	if offset >= r.rec.Duration {
		return nil
	}
	offset = max(offset, time.Millisecond)
	sc := r.rec.Scenario
	sc.Hold = ""
	rec, err := simulate(r.id, r.pipeline, r.params, r.rec.Start, sc, offset, f.childScenario)
	if err != nil {
		return err
	}
	for i := 0; i < len(r.rec.Events) && i < len(rec.Events) && r.rec.Events[i].At < offset; i++ {
		rec.Events[i] = r.rec.Events[i]
	}
	f.install(r, rec, r.wall0)
	for _, ch := range rec.Children {
		if child := f.runs[ch.RunID]; child != nil && child.rec != nil {
			if err := f.cancelAt(child, t); err != nil {
				return err
			}
		}
	}
	return nil
}

// --- RunsAPI / ObserveAPI ----------------------------------------------------

// phaseOf is the run's phase in Graphene's one lowercase vocabulary:
// running, completed, failed, canceled, terminated, timed-out.
func (f *fakeGraphene) phaseOf(r *doorRun) string {
	if !f.finished(r) {
		return "running"
	}
	return r.rec.Status
}

func (f *fakeGraphene) GetRun(_ context.Context, req *connect.Request[managementv1.GetRunRequest]) (*connect.Response[managementv1.GetRunResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.lookup(nsOf(req), req.Msg.GetRunId())
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("workflow not found for ID: run/%s", req.Msg.GetRunId()))
	}
	if r.rec == nil {
		return connect.NewResponse(&managementv1.GetRunResponse{Status: probeStatus}), nil
	}
	return connect.NewResponse(&managementv1.GetRunResponse{Status: f.phaseOf(r)}), nil
}

// probeStatus: the probe pipelines finish as soon as they are started.
const probeStatus = "completed"

func (f *fakeGraphene) CancelRun(_ context.Context, req *connect.Request[managementv1.CancelRunRequest]) (*connect.Response[managementv1.CancelRunResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.lookup(nsOf(req), req.Msg.GetRunId())
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("workflow not found for ID: run/%s", req.Msg.GetRunId()))
	}
	if err := f.cancel(r); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&managementv1.CancelRunResponse{}), nil
}

func (f *fakeGraphene) RunResult(ctx context.Context, req *connect.Request[managementv1.RunResultRequest]) (*connect.Response[managementv1.RunResultResponse], error) {
	id := req.Msg.GetRunId()
	for {
		f.mu.Lock()
		r, ok := f.lookup(nsOf(req), id)
		if !ok {
			f.mu.Unlock()
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("workflow not found for ID: run/%s", id))
		}
		if r.rec == nil {
			resp, err := f.probeResult(r)
			f.mu.Unlock()
			return resp, err
		}
		if f.finished(r) {
			rec := r.rec
			f.mu.Unlock()
			// A run that did not complete answers with what it collected
			// and why it ended — the close state, not an error.
			out := &managementv1.RunResultResponse{Result: rec.Result}
			switch rec.Status {
			case "canceled":
				out.Error = "run canceled"
			case "failed":
				out.Error = rec.Error
			}
			return connect.NewResponse(out), nil
		}
		f.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, connect.NewError(connect.CodeCanceled, ctx.Err())
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func (f *fakeGraphene) probeResult(r *doorRun) (*connect.Response[managementv1.RunResultResponse], error) {
	var result any
	switch r.pipeline {
	case graphene.PipelineProviderVerify, graphene.PipelineProviderConfig:
		result = provider.VerifyResult{OK: f.verifyOK, AccountID: "acc-1", Permissions: []provider.Permission{{Name: "compute.instances.list", Granted: f.verifyOK}}, Error: map[bool]string{true: "", false: "missing rights"}[f.verifyOK]}
		if r.pipeline == graphene.PipelineProviderConfig {
			if f.configResultError != nil {
				return nil, f.configResultError
			}
			var p provider.ConfigureParams
			_ = json.Unmarshal(r.params, &p)
			if p.Action == "delete" {
				result = provider.VerifyResult{OK: f.configDeleteFailure == "", Error: f.configDeleteFailure}
			}
		}
	case graphene.PipelineQuotas:
		if f.quotaError != nil {
			return nil, f.quotaError
		}
		result = provider.QuotasResult{ObservedAt: time.Now().UTC(), Quotas: f.quotas, UnavailableReason: f.quotaUnavailableReason, Scope: "cloud:b1gcloud000000000000"}
	default:
		result = map[string]any{}
	}
	raw, _ := json.Marshal(result) //nolint:errcheck // plain types
	return connect.NewResponse(&managementv1.RunResultResponse{Result: raw}), nil
}

// Events streams the run's history after an id; follow waits for more
// until the run's last event.
func (f *fakeGraphene) Events(ctx context.Context, req *connect.Request[managementv1.EventsRequest], stream *connect.ServerStream[managementv1.Event]) error {
	id := strings.TrimPrefix(req.Msg.GetRef(), "run/")
	after := req.Msg.GetAfterEventId()
	for {
		f.mu.Lock()
		r, ok := f.lookup(nsOf(req), id)
		if !ok {
			f.mu.Unlock()
			return connect.NewError(connect.CodeNotFound, fmt.Errorf("workflow not found for ID: run/%s", id))
		}
		var batch []*managementv1.Event
		if r.rec != nil {
			kinds := req.Msg.GetKinds()
			for _, e := range r.rec.Events[:f.visible(r)] {
				if e.Ev.GetEventId() <= after {
					continue
				}
				after = e.Ev.GetEventId()
				if len(kinds) > 0 && !slices.Contains(kinds, e.Ev.GetKind()) {
					continue
				}
				batch = append(batch, e.Ev)
			}
		}
		done := r.rec == nil || f.finished(r)
		f.mu.Unlock()
		for _, ev := range batch {
			if err := stream.Send(ev); err != nil {
				return err
			}
		}
		if done || !req.Msg.GetFollow() {
			return nil
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(20 * time.Millisecond):
		}
	}
}

// --- ResourcesAPI ------------------------------------------------------------

// liveResource is one record as it stands at the run's virtual now.
type liveResource struct {
	res   simResource
	owner string
	phase string
	run   *doorRun
}

// resourcesAt folds every run of the namespace to its virtual now.
func (f *fakeGraphene) resourcesAt(ns string, withPast bool) map[string]liveResource {
	out := map[string]liveResource{}
	for _, r := range f.runs {
		if r.rec == nil || r.namespace != ns || !f.started(r) {
			continue
		}
		now := f.vnow(r).Sub(r.rec.Start)
		if !f.finished(r) {
			if shown := f.visible(r); shown > 0 {
				now = r.rec.Events[shown-1].At
			} else {
				now = 0
			}
		}
		for _, res := range r.rec.Resources {
			if res.Declared < 0 || res.Declared > now {
				continue
			}
			gone := res.Deleted >= 0 && res.Deleted <= now
			if gone && !withPast {
				continue
			}
			owner := string(res.Origin)
			if res.Transferred >= 0 && res.Transferred <= now {
				owner = string(res.Owner)
			}
			phase := "creating"
			switch {
			case gone:
				phase = "deleted"
			case res.Ready >= 0 && res.Ready <= now:
				phase = "ready"
			}
			if res.Phase == "failed" && !gone {
				phase = "failed"
			}
			out[string(res.Ref)] = liveResource{res: res, owner: owner, phase: phase, run: r}
		}
	}
	// Stand holdings end at their deadline or on release, with their subtree.
	dead := map[string]bool{}
	for name, lr := range out {
		if !strings.HasPrefix(lr.owner, "stand/") || lr.phase == "deleted" {
			continue
		}
		r := lr.run
		until := lr.res.KeepUntil
		if v, ok := r.keepUntil[name]; ok {
			until = v
		}
		_, released := r.released[name]
		if released || (!until.IsZero() && !f.vnow(r).Before(until)) {
			dead[name] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for name, lr := range out {
			if !dead[name] && dead[lr.owner] {
				dead[name], changed = true, true
			}
		}
	}
	for name := range dead {
		if !withPast {
			delete(out, name)
			continue
		}
		lr := out[name]
		lr.phase = "deleted"
		out[name] = lr
	}
	return out
}

func kindOf(ref string) string {
	kind, _, _ := strings.Cut(ref, "/")
	return kind
}

// visibility is the row a listing or a tree carries: no spec, no state,
// but the record's own edges, mirrored into visibility.
func (f *fakeGraphene) visibility(lr liveResource) *managementv1.Resource {
	res := &managementv1.Resource{Ref: string(lr.res.Ref), Kind: kindOf(string(lr.res.Ref)), Phase: lr.phase, Owner: lr.owner, Labels: lr.res.Labels}
	for _, flow := range lr.res.Flows {
		res.Flows = append(res.Flows, &managementv1.Flow{To: flow.To, Protocol: string(flow.Protocol), Port: int32(flow.Port), Label: flow.Label, Virtual: flow.Virtual})
	}
	return res
}

// runNode is a run as the tree shows it.
func (f *fakeGraphene) runNode(r *doorRun) *managementv1.Resource {
	owner := "pipeline/" + r.pipeline
	if r.parent != "" {
		owner = "run/" + r.parent
	}
	phase := "Running"
	if r.rec != nil {
		phase = f.phaseOf(r)
	}
	return &managementv1.Resource{Ref: "run/" + r.id, Kind: "run", Phase: phase, Owner: owner, Labels: r.labels}
}

// subtree lists the children of owner: live records it owns and, under a
// run, its child runs (recursed).
func (f *fakeGraphene) subtree(ns, owner string, live map[string]liveResource) []*managementv1.TreeNode {
	var out []*managementv1.TreeNode
	names := make([]string, 0)
	for name, lr := range live {
		if lr.owner == owner {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		out = append(out, &managementv1.TreeNode{Resource: f.visibility(live[name]), Children: f.subtree(ns, name, live)})
	}
	if id, ok := strings.CutPrefix(owner, "run/"); ok {
		var kids []string
		for cid, r := range f.runs {
			if r.parent == id && r.namespace == ns && f.started(r) {
				kids = append(kids, cid)
			}
		}
		sort.Strings(kids)
		for _, cid := range kids {
			out = append(out, &managementv1.TreeNode{Resource: f.runNode(f.runs[cid]), Children: f.subtree(ns, "run/"+cid, live)})
		}
	}
	return out
}

func (f *fakeGraphene) Tree(_ context.Context, req *connect.Request[managementv1.TreeRequest]) (*connect.Response[managementv1.TreeResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	ns := nsOf(req)
	// A run's tree is ALWAYS whole: a finished run is history, and its
	// topology after the teardown is what one comes to see.
	withPast := req.Msg.GetIncludeDeleted() || strings.HasPrefix(req.Msg.GetOwner(), "run/")
	return connect.NewResponse(&managementv1.TreeResponse{Roots: f.subtree(ns, req.Msg.GetOwner(), f.resourcesAt(ns, withPast))}), nil
}

func (f *fakeGraphene) List(_ context.Context, req *connect.Request[managementv1.ListRequest]) (*connect.Response[managementv1.ListResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	sel := req.Msg.GetSelector()
	ns := nsOf(req)
	var out []*managementv1.Resource
	for _, lr := range f.resourcesAt(ns, false) {
		v := f.visibility(lr)
		if (sel.GetKind() != "" && v.GetKind() != sel.GetKind()) || (sel.GetOwner() != "" && v.GetOwner() != sel.GetOwner()) {
			continue
		}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].GetRef() < out[j].GetRef() })
	return connect.NewResponse(&managementv1.ListResponse{Resources: out}), nil
}

func (f *fakeGraphene) Get(_ context.Context, req *connect.Request[managementv1.GetRequest]) (*connect.Response[managementv1.GetResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	ns, name := nsOf(req), req.Msg.GetRef()
	if id, ok := strings.CutPrefix(name, "run/"); ok {
		r, ok := f.lookup(ns, id)
		if !ok {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no record %s", name))
		}
		return connect.NewResponse(&managementv1.GetResponse{Resource: f.runNode(r)}), nil
	}
	if name == "stand/"+graphene.PipelineRun {
		return connect.NewResponse(&managementv1.GetResponse{Resource: f.standRecord(ns)}), nil
	}
	lr, ok := f.resourcesAt(ns, true)[name]
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no record %s", name))
	}
	return connect.NewResponse(&managementv1.GetResponse{Resource: f.record(lr)}), nil
}

// record is a full record: visibility plus the spec and state a listing
// does not carry.
func (f *fakeGraphene) record(lr liveResource) *managementv1.Resource {
	res := f.visibility(lr)
	res.Spec, res.State = lr.res.Spec, lr.res.State
	return res
}

// GetMany describes several records at once; refs nobody knows are named
// in `missing`.
func (f *fakeGraphene) GetMany(_ context.Context, req *connect.Request[managementv1.GetManyRequest]) (*connect.Response[managementv1.GetManyResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	live := f.resourcesAt(nsOf(req), true)
	out := &managementv1.GetManyResponse{}
	for _, name := range req.Msg.GetRefs() {
		lr, ok := live[name]
		if !ok {
			out.Missing = append(out.Missing, name)
			continue
		}
		out.Resources = append(out.Resources, f.record(lr))
	}
	return connect.NewResponse(out), nil
}

// standRecord is the stand of stroppy-run: what it holds, for whom, until
// when (standflow.State).
func (f *fakeGraphene) standRecord(ns string) *managementv1.Resource {
	holdings := map[string]any{}
	for name, lr := range f.resourcesAt(ns, false) {
		if !strings.HasPrefix(lr.owner, "stand/") {
			continue
		}
		// The stand records who handed the holding over (ToStand's From).
		from := lr.res.From
		if from == "" {
			from = lr.res.Creator
		}
		h := map[string]any{"from": string(from)}
		until := lr.res.KeepUntil
		if v, ok := lr.run.keepUntil[name]; ok {
			until = v
		}
		if !until.IsZero() {
			h["keepUntil"] = until
		}
		holdings[name] = h
	}
	state, _ := json.Marshal(map[string]any{"holdings": holdings}) //nolint:errcheck // plain map
	return &managementv1.Resource{Ref: "stand/" + graphene.PipelineRun, Kind: "stand", Phase: "ready", Owner: "pipeline/" + graphene.PipelineRun, State: state}
}

func (f *fakeGraphene) Download(_ context.Context, req *connect.Request[managementv1.DownloadRequest], stream *connect.ServerStream[managementv1.DownloadChunk]) error {
	f.mu.Lock()
	lr, ok := f.resourcesAt(nsOf(req), false)[req.Msg.GetRef()]
	f.mu.Unlock()
	if !ok {
		return connect.NewError(connect.CodeNotFound, fmt.Errorf("no record %s", req.Msg.GetRef()))
	}
	var state pipeline.ArtifactState
	if json.Unmarshal(lr.res.State, &state) != nil || state.Blob.Location == "" {
		return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("%s has no downloadable bytes", req.Msg.GetRef()))
	}
	data, ok := lr.run.rec.Blobs[state.Blob.Location]
	if !ok {
		return connect.NewError(connect.CodeNotFound, fmt.Errorf("blob %s: gone", state.Blob.Location))
	}
	for len(data) > 0 {
		n := min(len(data), 256<<10)
		if err := stream.Send(&managementv1.DownloadChunk{Data: data[:n]}); err != nil {
			return err
		}
		data = data[n:]
	}
	return nil
}

// Invoke answers the stand commands (standflow: extend, release).
func (f *fakeGraphene) Invoke(_ context.Context, req *connect.Request[managementv1.InvokeRequest]) (*connect.Response[managementv1.InvokeResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.commands = append(f.commands, req.Msg.GetRef()+" "+req.Msg.GetCommand())
	if !strings.HasPrefix(req.Msg.GetRef(), "stand/") {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("%s has no command %s", req.Msg.GetRef(), req.Msg.GetCommand()))
	}
	var cmd struct {
		Ref  string        `json:"ref"`
		Keep time.Duration `json:"keep"`
	}
	if err := json.Unmarshal(req.Msg.GetPayload(), &cmd); err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	live := f.resourcesAt(nsOf(req), false)
	var held []string
	for name, lr := range live {
		if lr.owner == req.Msg.GetRef() && (cmd.Ref == "" || cmd.Ref == name) {
			held = append(held, name)
		}
	}
	if cmd.Ref != "" && len(held) == 0 {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("stand holds no %s", cmd.Ref))
	}
	for _, name := range held {
		r := live[name].run
		switch req.Msg.GetCommand() {
		case "extend":
			until := time.Time{}
			if cmd.Keep > 0 {
				until = f.vnow(r).Add(cmd.Keep)
			}
			r.keepUntil[name] = until
		case "release":
			r.released[name] = f.vnow(r)
		default:
			return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("unknown command %s", req.Msg.GetCommand()))
		}
	}
	raw, _ := json.Marshal(map[string]int{"count": len(held)}) //nolint:errcheck // plain map
	return connect.NewResponse(&managementv1.InvokeResponse{Result: raw}), nil
}

func (f *fakeGraphene) Apply(_ context.Context, req *connect.Request[managementv1.ApplyRequest]) (*connect.Response[managementv1.ApplyResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if req.Msg.GetKind() == "namespace" {
		f.namespaces[req.Msg.GetId()] = true
	}
	return connect.NewResponse(&managementv1.ApplyResponse{Ref: req.Msg.GetKind() + "/" + req.Msg.GetId()}), nil
}

func (f *fakeGraphene) Delete(_ context.Context, req *connect.Request[managementv1.DeleteRequest]) (*connect.Response[managementv1.DeleteResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	name, ns := req.Msg.GetRef(), nsOf(req)
	f.deleted = append(f.deleted, name)
	switch {
	case strings.HasPrefix(name, "namespace/"):
		delete(f.namespaces, strings.TrimPrefix(name, "namespace/"))
	case strings.HasPrefix(name, "secret/"):
		delete(f.secrets, ns+"/"+strings.TrimPrefix(name, "secret/"))
	case strings.HasPrefix(name, "run/"):
		// Deleting a run cancels it; it tears its own resources down.
		r, ok := f.lookup(ns, strings.TrimPrefix(name, "run/"))
		if !ok {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no record %s", name))
		}
		if err := f.cancel(r); err != nil {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}
	default:
		lr, ok := f.resourcesAt(ns, false)[name]
		if !ok {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no record %s", name))
		}
		lr.run.released[name] = f.vnow(lr.run)
	}
	return connect.NewResponse(&managementv1.DeleteResponse{}), nil
}

func (f *fakeGraphene) SetSecret(_ context.Context, req *connect.Request[managementv1.SetSecretRequest]) (*connect.Response[managementv1.SetSecretResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.secrets[nsOf(req)+"/"+req.Msg.GetName()] = req.Msg.GetValue()
	return connect.NewResponse(&managementv1.SetSecretResponse{Version: 1}), nil
}

func (f *fakeGraphene) WhoAmI(context.Context, *connect.Request[managementv1.WhoAmIRequest]) (*connect.Response[managementv1.WhoAmIResponse], error) {
	return connect.NewResponse(&managementv1.WhoAmIResponse{Subject: "sa:stroppy", Namespace: "*"}), nil
}

// --- test helpers ------------------------------------------------------------

// doorView is what a test reads about a run the door knows.
type doorView struct {
	namespace string
	pipeline  string
	params    json.RawMessage
	labels    map[string]string
	status    string
	rec       *simRecord
}

// runOf reads a run the door knows.
func (f *fakeGraphene) runOf(runID string) (doorView, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.runs[runID]
	if !ok {
		return doorView{}, false
	}
	v := doorView{namespace: r.namespace, pipeline: r.pipeline, params: r.params, labels: r.labels, rec: r.rec}
	if r.rec != nil {
		v.status = f.phaseOf(r)
	}
	return v, true
}

// secret returns a stored secret value.
func (f *fakeGraphene) secret(ns, name string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.secrets[ns+"/"+name]
	return v, ok
}

func (f *fakeGraphene) hasNamespace(ns string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.namespaces[ns]
}

// parseSimLabel reads a `sim` run label: items joined by "+", each
// "what" or "what:arg" — fail-segment:<segment>, lose-segment:<segment>,
// fail-baseline, fail-healthcheck:<container>, fail-docker, fail-provider,
// fail-machine:<machine>, no-agents, fail-artifacts, hold:<milestone>,
// tps:<number>.
func parseSimLabel(v string) simScenario {
	var sc simScenario
	for _, item := range strings.Split(v, "+") {
		what, arg, _ := strings.Cut(strings.TrimSpace(item), ":")
		switch what {
		case "fail-segment":
			sc.FailSegment = arg
		case "lose-segment":
			sc.LoseSegment = arg
		case "fail-baseline":
			sc.FailBaseline = true
		case "fail-healthcheck":
			sc.FailHealthcheck = arg
		case "fail-docker":
			sc.FailDockerInstall = true
		case "fail-provider":
			sc.FailProviderConfig = true
		case "fail-machine":
			sc.FailMachine = arg
		case "no-agents":
			sc.AgentsNeverConnect = true
		case "fail-artifacts":
			sc.FailArtifacts = true
		case "hold":
			sc.Hold = arg
		case "tps":
			_, _ = fmt.Sscan(arg, &sc.TPS)
		}
	}
	return sc
}
