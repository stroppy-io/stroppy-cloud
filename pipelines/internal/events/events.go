// Package events puts the run's domain milestones into its Graphene
// history: phases, machines, containers, segments. The server projects
// them into the run overview; a share snapshot shows them as the timeline.
//
// A milestone is emitted from workflow code through one small activity on
// the run's own queue (the worker plane is reachable from activities only).
// Emission is best effort: a lost milestone must never fail a run.
package events

import (
	"context"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/graphene-ci/pipeline/pkg/pipeline"
	"github.com/graphene-ci/pipeline/pkg/wire"
	"github.com/graphene-ci/pipeline/pkg/workerapi"
)

// Names of the milestones a stroppy run emits.
const (
	PhaseStarted     = "phase.started"
	PhaseFinished    = "phase.finished"
	PhaseFailed      = "phase.failed"
	MachineReady     = "machine.ready"
	ContainerReady   = "container.ready"
	SegmentStarted   = "segment.started"
	SegmentFinished  = "segment.finished"
	SegmentFailed    = "segment.failed"
	BaselineStarted  = "baseline.started"
	BaselineFinished = "baseline.finished"
	StandKept        = "stand.kept"
	ResultPublished  = "result.published"
	// NetworkChecked carries the connectivity check: every path, failed
	// first, and the network snapshots of the machines on failed paths.
	NetworkChecked = "network.checked"
	// ActivityName is the wire name of the emit activity (run queue).
	ActivityName      = "stroppy.events.emit"
	activityTimeout   = 30 * time.Second
	activityMaxTries  = 3
	activityHeartbeat = 0
)

// Payload is the JSON body of a milestone.
type Payload map[string]any

// request travels workflow → activity.
type request struct {
	Ref     string  `json:"ref"`
	Name    string  `json:"name"`
	Payload Payload `json:"payload,omitempty"`
}

// Register makes the emit activity discoverable in the recording pass.
// Call it from the pipeline's recording walk.
func Register(ctx pipeline.Context) {
	ctx.RecordActivity(ActivityName, emit)
}

// Emit puts a milestone into the run's history. Never blocks the caller
// on failure: the activity is bounded and its error is logged.
func Emit(ctx pipeline.Context, name string, payload Payload) {
	if ctx.Recording() {
		return
	}
	req := request{Ref: "run/" + string(ctx.RunId()), Name: name, Payload: payload}
	actx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:           wire.RunQueue(ctx.RunId()),
		StartToCloseTimeout: activityTimeout,
		RetryPolicy:         &temporal.RetryPolicy{InitialInterval: time.Second, BackoffCoefficient: 2, MaximumAttempts: activityMaxTries},
	})
	if err := workflow.ExecuteActivity(actx, ActivityName, req).Get(ctx, nil); err != nil {
		ctx.Logger().Warn("milestone lost", "event", name, "error", err)
	}
}

func emit(ctx context.Context, req request) error {
	raw, err := marshal(req.Payload)
	if err != nil {
		return err
	}
	return workerapi.EmitEvent(ctx, req.Ref, req.Name, raw)
}
