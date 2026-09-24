package graphene

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"connectrpc.com/connect"

	managementv1 "github.com/graphene-ci/graphene/pkg/proto/management/v1"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
)

/*
OBSERVE helpers: the run-level reads the projection and the UI need —
the event stream (resumable by event id), the status watch, the ownership
tree, keep/stand commands and artifact download. All namespace-scoped
through ctx.
*/

// Events follows the run's event stream from afterID and hands every
// event to fn until the stream ends or fn returns an error.
func (c *Client) Events(ctx context.Context, runID string, afterID int64, follow bool, fn func(run.RawEvent) error) error {
	stream, err := c.Observe.Events(ctx, connect.NewRequest(&managementv1.EventsRequest{Ref: "run/" + runID, AfterEventId: afterID, Follow: follow}))
	if err != nil {
		return fmt.Errorf("graphene: events of %s: %w", runID, err)
	}
	defer stream.Close()
	for stream.Receive() {
		e := stream.Msg()
		raw := run.RawEvent{
			ID: e.GetEventId(), At: time.Unix(0, e.GetTimeUnixNano()).UTC(), Kind: e.GetKind(), Subject: e.GetSubject(),
			Agent: e.GetAgent(), Status: e.GetStatus(), Error: e.GetError(), Attempt: int(e.GetAttempt()),
			Input: e.GetInput(), Result: e.GetResult(), ActivityID: e.GetActivityId(),
		}
		if err := fn(raw); err != nil {
			return err
		}
	}
	if err := stream.Err(); err != nil {
		return fmt.Errorf("graphene: events of %s: %w", runID, err)
	}
	return nil
}

// WatchRun streams status transitions until terminal.
func (c *Client) WatchRun(ctx context.Context, runID string, fn func(status string) error) error {
	stream, err := c.Runs.WatchRun(ctx, connect.NewRequest(&managementv1.WatchRunRequest{RunId: runID}))
	if err != nil {
		return fmt.Errorf("graphene: watch %s: %w", runID, err)
	}
	defer stream.Close()
	for stream.Receive() {
		if err := fn(stream.Msg().GetStatus()); err != nil {
			return err
		}
	}
	if err := stream.Err(); err != nil {
		return fmt.Errorf("graphene: watch %s: %w", runID, err)
	}
	return nil
}

// Tree reads the ownership tree under owner: the records it owns (live
// ones — Graphene shows a deleted record nowhere) with their subtrees.
func (c *Client) Tree(ctx context.Context, owner string) (run.TreeNode, error) {
	resp, err := c.Resources.Tree(ctx, connect.NewRequest(&managementv1.TreeRequest{Owner: owner}))
	if err != nil {
		return run.TreeNode{}, fmt.Errorf("graphene: tree of %s: %w", owner, err)
	}
	kind, _, _ := strings.Cut(owner, "/")
	out := run.TreeNode{Ref: owner, Kind: kind, Children: make([]run.TreeNode, 0, len(resp.Msg.GetRoots()))}
	for _, r := range resp.Msg.GetRoots() {
		out.Children = append(out.Children, treeOf(r))
	}
	return out, nil
}

func treeOf(n *managementv1.TreeNode) run.TreeNode {
	if n == nil {
		return run.TreeNode{Children: []run.TreeNode{}}
	}
	r := n.GetResource()
	out := run.TreeNode{Ref: r.GetRef(), Kind: r.GetKind(), Phase: r.GetPhase(), Labels: r.GetLabels(), Children: make([]run.TreeNode, 0, len(n.GetChildren()))}
	for _, f := range r.GetFlows() {
		out.Flows = append(out.Flows, run.Flow{To: f.GetTo(), Protocol: f.GetProtocol(), Port: int(f.GetPort()), Label: f.GetLabel(), Virtual: f.GetVirtual()})
	}
	for _, ch := range n.GetChildren() {
		out.Children = append(out.Children, treeOf(ch))
	}
	return out
}

// Node describes one record with its subtree; false when it is gone.
func (c *Client) Node(ctx context.Context, ref string) (run.TreeNode, bool, error) {
	resp, err := c.Resources.Get(ctx, connect.NewRequest(&managementv1.GetRequest{Ref: ref}))
	if IsNotFound(err) {
		return run.TreeNode{}, false, nil
	}
	if err != nil {
		return run.TreeNode{}, false, fmt.Errorf("graphene: get %s: %w", ref, err)
	}
	node := treeOf(&managementv1.TreeNode{Resource: resp.Msg.GetResource()})
	sub, err := c.Tree(ctx, ref)
	if err != nil {
		return run.TreeNode{}, false, err
	}
	node.Children = sub.Children
	return node, true, nil
}

// DeleteRef tears a subtree down (blocking on the Graphene side).
func (c *Client) DeleteRef(ctx context.Context, ref string) error {
	_, err := c.Resources.Delete(ctx, connect.NewRequest(&managementv1.DeleteRequest{Ref: ref}))
	if err != nil && !IsNotFound(err) {
		return fmt.Errorf("graphene: delete %s: %w", ref, err)
	}
	return nil
}

// Holdings reads what the run handed to the pipeline's stand. The stand
// records who handed each holding over, so one run's leftovers are told
// from the next run's.
func (c *Client) Holdings(ctx context.Context, runRef string) ([]run.Holding, error) {
	resp, err := c.Resources.Get(ctx, connect.NewRequest(&managementv1.GetRequest{Ref: "stand/" + PipelineRun}))
	if IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("graphene: stand of %s: %w", PipelineRun, err)
	}
	var state struct {
		Holdings map[string]struct {
			KeepUntil *time.Time `json:"keepUntil"`
			From      string     `json:"from"`
		} `json:"holdings"`
	}
	if err := json.Unmarshal(resp.Msg.GetResource().GetState(), &state); err != nil {
		return nil, fmt.Errorf("graphene: stand state: %w", err)
	}
	out := make([]run.Holding, 0, len(state.Holdings))
	for ref, h := range state.Holdings {
		if h.From != runRef {
			continue
		}
		out = append(out, run.Holding{Ref: ref, KeepUntil: h.KeepUntil})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Ref < out[j].Ref })
	return out, nil
}

// standCommand sends a command to the stand of stroppy-run, where the
// pipeline hands the infrastructure it keeps (standflow: extend, release).
func (c *Client) standCommand(ctx context.Context, command string, payload map[string]any) error {
	raw, _ := json.Marshal(payload) //nolint:errcheck // map
	_, err := c.Resources.Invoke(ctx, connect.NewRequest(&managementv1.InvokeRequest{
		Ref: "stand/" + PipelineRun, Command: command, Payload: raw,
	}))
	return err
}

// KeepExtend moves the deadline of a holding to keep from now.
func (c *Client) KeepExtend(ctx context.Context, held string, keep time.Duration) error {
	if err := c.standCommand(ctx, "extend", map[string]any{"ref": held, "keep": keep.Nanoseconds()}); err != nil {
		return fmt.Errorf("graphene: extend keep of %s: %w", held, err)
	}
	return nil
}

// KeepRelease tears a holding (and its subtree) down now.
func (c *Client) KeepRelease(ctx context.Context, held string) error {
	if err := c.standCommand(ctx, "release", map[string]any{"ref": held}); err != nil && !IsNotFound(err) {
		return fmt.Errorf("graphene: release keep of %s: %w", held, err)
	}
	return nil
}

// artifactBatch is GetMany's ceiling.
const artifactBatch = 100

// Artifacts describes the artifact records that can still be downloaded.
// Visibility (List) carries no state, so the records are read: the spec
// names the media type, the state the verified blob. A record already
// gone (retention, keep release) is missing from the answer, deleted or
// without a blob — none of those is offered.
func (c *Client) Artifacts(ctx context.Context, refs []string) ([]run.Artifact, error) {
	out := make([]run.Artifact, 0, len(refs))
	for start := 0; start < len(refs); start += artifactBatch {
		batch := refs[start:min(start+artifactBatch, len(refs))]
		resp, err := c.Resources.GetMany(ctx, connect.NewRequest(&managementv1.GetManyRequest{Refs: batch}))
		if err != nil {
			return nil, fmt.Errorf("graphene: artifacts: %w", err)
		}
		for _, r := range resp.Msg.GetResources() {
			if a, ok := artifactOf(r); ok {
				out = append(out, a)
			}
		}
	}
	return out, nil
}

// artifactOf reads one artifact record; ok is false when the record is
// on its way out or holds no bytes, so the API never offers a download
// that cannot answer.
func artifactOf(r *managementv1.Resource) (run.Artifact, bool) {
	ref := r.GetRef()
	name := strings.TrimPrefix(ref, "artifact/")
	a := run.Artifact{Ref: ref, Name: name, Kind: artifactKind(name)}
	var spec struct {
		MediaType string `json:"mediaType"`
	}
	if json.Unmarshal(r.GetSpec(), &spec) == nil {
		a.ContentType = spec.MediaType
	}
	var state struct {
		Blob struct {
			Size     int64  `json:"size"`
			Digest   string `json:"digest"`
			Location string `json:"location"`
		} `json:"blob"`
	}
	if json.Unmarshal(r.GetState(), &state) == nil {
		a.SizeBytes, a.Digest = state.Blob.Size, state.Blob.Digest
	}
	if at := r.GetStartedAt(); at != nil {
		t := at.AsTime()
		a.CreatedAt = &t
	}
	live := !r.GetMarkedForDeletion() && r.GetPhase() != "deleted" && state.Blob.Location != ""
	return a, live
}

// artifactKind classifies by the pipeline's names: <run8>-stroppy-<segment>-config,
// -log; managed-ydb-readiness-config/-log; stroppy-baseline-log.
func artifactKind(name string) string {
	switch {
	case strings.HasSuffix(name, "-config"):
		return "config"
	case strings.HasSuffix(name, "-log"):
		return "log_bundle"
	}
	return "other"
}

// Download streams an artifact's bytes. Connect reports a server-stream
// failure on the first Receive rather than on the call, so the first
// chunk is pulled here: a download that cannot answer fails before the
// handler has written a 200 with the bytes half sent.
func (c *Client) Download(ctx context.Context, ref string) (io.ReadCloser, error) {
	stream, err := c.Resources.Download(ctx, connect.NewRequest(&managementv1.DownloadRequest{Ref: ref}))
	if err != nil {
		return nil, downloadError(ref, err)
	}
	var first []byte
	if stream.Receive() {
		first = bytes.Clone(stream.Msg().GetData())
	} else if err := stream.Err(); err != nil {
		_ = stream.Close()
		return nil, downloadError(ref, err)
	}
	pr, pw := io.Pipe()
	go func() {
		defer func() { _ = stream.Close() }()
		if _, err := pw.Write(first); err != nil {
			pw.CloseWithError(err)
			return
		}
		for stream.Receive() {
			if _, err := pw.Write(stream.Msg().GetData()); err != nil {
				pw.CloseWithError(err)
				return
			}
		}
		pw.CloseWithError(stream.Err())
	}()
	return pr, nil
}

// downloadError states what the caller can do about it: a record or blob
// that is gone is a 404, bytes that are not there yet a 409, anything
// else an upstream failure.
func downloadError(ref string, err error) error {
	name := strings.TrimPrefix(ref, "artifact/")
	switch connect.CodeOf(err) { //nolint:exhaustive // three outcomes the caller can act on; the rest is upstream
	case connect.CodeNotFound:
		return errs.NotFound("artifact " + name)
	case connect.CodeFailedPrecondition:
		return errs.Conflict("artifact " + name + " has no downloadable bytes")
	default:
		return errs.Wrap(errs.CodeUnavailable, "download "+ref, err)
	}
}
