package run

import (
	"github.com/docker/docker/api/types/container"

	dockerlib "github.com/graphene-ci/library/docker"
	k8slib "github.com/graphene-ci/library/k8s"
	"github.com/graphene-ci/pipeline/pkg/activity"
	"github.com/graphene-ci/pipeline/pkg/artifact"
	"github.com/graphene-ci/pipeline/pkg/pipeline"

	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/activities"
	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/events"
	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/provision"
)

// recordingWalk is what the registration pass sees instead of a run: a
// zero-value RunSpec has no machines, so every loop below Run is empty and
// nothing would be discovered. Here every activity body and every library
// kind the run may use is declared once, unconditionally — the invariant
// the SDK asks for ("declare on the optimistic zero path").
func recordingWalk(ctx pipeline.Context) {
	activities.Register(ctx)
	events.Register(ctx)
	for _, p := range provision.Default() {
		k8s := k8slib.NewClientFromSecret(pipeline.Secret(ctx, activities.KubeconfigSecret), p.Scheme())
		p.Record(ctx, k8s)
	}
	agent := pipeline.NewAgent(ctx, "record-agent")
	if _, err := activity.Activity(ctx, agent, dockerlib.Install()); err != nil {
		panic(err)
	}
	dockerlib.Container(ctx, agent, dockerlib.Spec{Name: "record-container", Config: &container.Config{Image: "record"}})
	pipeline.NewArtifact(ctx, "record-artifact", artifact.FromAgentFile(agent, "/record"))
	pipeline.ToStand(ctx, agent)
}
