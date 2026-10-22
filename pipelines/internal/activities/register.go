package activities

import "github.com/graphene-ci/pipeline/pkg/pipeline"

// Register makes every body discoverable in the recording pass. The
// pipeline's recording walk calls it once; outside the pass it is a no-op.
func Register(ctx pipeline.Context) {
	ctx.RecordActivity(NameEnsureProviderConfig, EnsureProviderConfig)
	ctx.RecordActivity(NameResolveYandexImages, ResolveYandexImages)
	ctx.RecordActivity(NameHostPrep, HostPrep)
	ctx.RecordActivity(NamePullImage, PullImage)
	ctx.RecordActivity(NameWaitHealthy, WaitHealthy)
	ctx.RecordActivity(NameWaitDatabase, WaitDatabase)
	ctx.RecordActivity(NameWaitReachable, WaitReachable)
	ctx.RecordActivity(NameNetProbe, NetProbe)
	ctx.RecordActivity(NameNetSnapshot, NetSnapshot)
	ctx.RecordActivity(NameWriteFiles, WriteFiles)
	ctx.RecordActivity(NameRunSegment, RunSegment)
	ctx.RecordActivity(NameRunBaseline, RunBaseline)
}
