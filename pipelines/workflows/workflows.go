// Package workflows is the public door to the stroppy-run and
// stroppy-suite bodies: their ids, entry points, the wire names of the
// activities they dispatch and the milestones they emit. The implementation
// stays in internal/; this is what a simulation needs to run the REAL
// workflow on pipelinetest — the server's end-to-end tests drive it in
// virtual time and answer the agent activities themselves. (The probe
// pipelines are one activity each over cloud SDKs; nothing outside needs
// their bodies.)
package workflows

import (
	"github.com/graphene-ci/pipeline/pkg/pipeline"

	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/activities"
	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/events"
	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/run"
	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/suite"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// Pipeline ids.
const (
	RunPipelineID   = run.PipelineID
	SuitePipelineID = suite.PipelineID
)

// Run is the body of stroppy-run.
func Run(ctx pipeline.Context, r spec.Run) (spec.Result, error) { return run.Run(ctx, r) }

// Suite is the body of stroppy-suite.
func Suite(ctx pipeline.Context, s spec.Suite) (spec.SuiteResult, error) { return suite.Run(ctx, s) }

// Wire names of the activities. Worker-side ones run on the run queue,
// agent-side ones on the agent's run queue.
const (
	ActivityEmit                 = events.ActivityName
	ActivityEnsureProviderConfig = activities.NameEnsureProviderConfig
	ActivityResolveYandexImages  = activities.NameResolveYandexImages
	ActivityHostPrep             = activities.NameHostPrep
	ActivityWriteFiles           = activities.NameWriteFiles
	ActivityPullImage            = activities.NamePullImage
	ActivityWaitHealthy          = activities.NameWaitHealthy
	ActivityWaitDatabase         = activities.NameWaitDatabase
	ActivityRunSegment           = activities.NameRunSegment
	ActivityRunBaseline          = activities.NameRunBaseline
)

// Activity payloads.
type (
	EnsureProviderConfigRequest = activities.EnsureProviderConfigRequest
	EnsureProviderConfigResult  = activities.EnsureProviderConfigResult
	ResolveYandexImagesRequest  = activities.ResolveYandexImagesRequest
	ResolveYandexImagesResult   = activities.ResolveYandexImagesResult
	HostPrepRequest             = activities.HostPrepRequest
	HostPrepResult              = activities.HostPrepResult
	WriteFilesRequest           = activities.WriteFilesRequest
	WriteFilesResult            = activities.WriteFilesResult
	PullImageRequest            = activities.PullImageRequest
	PullImageResult             = activities.PullImageResult
	WaitHealthyRequest          = activities.WaitHealthyRequest
	WaitHealthyResult           = activities.WaitHealthyResult
	WaitDatabaseRequest         = activities.WaitDatabaseRequest
	WaitDatabaseResult          = activities.WaitDatabaseResult
	RunSegmentRequest           = activities.RunSegmentRequest
	RunSegmentResult            = activities.RunSegmentResult
	RunBaselineRequest          = activities.RunBaselineRequest
	RunBaselineResult           = activities.RunBaselineResult
)

// Milestones (the `name` of an entity-note).
const (
	PhaseStarted     = events.PhaseStarted
	PhaseFinished    = events.PhaseFinished
	PhaseFailed      = events.PhaseFailed
	MachineReady     = events.MachineReady
	ContainerReady   = events.ContainerReady
	SegmentStarted   = events.SegmentStarted
	SegmentFinished  = events.SegmentFinished
	SegmentFailed    = events.SegmentFailed
	BaselineStarted  = events.BaselineStarted
	BaselineFinished = events.BaselineFinished
	StandKept        = events.StandKept
	ResultPublished  = events.ResultPublished
	CellStarted      = suite.CellStarted
	CellFinished     = suite.CellFinished
	CellFailed       = suite.CellFailed
)
