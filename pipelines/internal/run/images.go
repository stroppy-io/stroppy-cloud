package run

import (
	"fmt"
	"slices"
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/graphene-ci/pipeline/pkg/pipeline"
	"github.com/graphene-ci/pipeline/pkg/wire"

	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/activities"
	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/cloud"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

func resolveImages(ctx pipeline.Context, run *spec.Run) error {
	if run.Provider.Kind != spec.ProviderYandex {
		return nil
	}
	var families []string
	for _, machine := range run.Machines {
		if !cloud.IsYandexImageID(machine.Image) && !slices.Contains(families, machine.Image) {
			families = append(families, machine.Image)
		}
	}
	if len(families) == 0 {
		return nil
	}
	actx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{TaskQueue: wire.RunQueue(ctx.RunId()), StartToCloseTimeout: 2 * time.Minute, ScheduleToCloseTimeout: 3 * time.Minute})
	var result activities.ResolveYandexImagesResult
	if err := workflow.ExecuteActivity(actx, activities.NameResolveYandexImages, activities.ResolveYandexImagesRequest{CredentialsSecret: run.Provider.CredentialsSecret, Images: families}).Get(ctx, &result); err != nil {
		return fmt.Errorf("resolve boot images: %w", err)
	}
	run.Machines = slices.Clone(run.Machines)
	for i, machine := range run.Machines {
		if cloud.IsYandexImageID(machine.Image) {
			continue
		}
		id := result.Images[machine.Image]
		if !cloud.IsYandexImageID(id) {
			return fmt.Errorf("boot image %q was not resolved", machine.Image)
		}
		run.Machines[i].Image = id
	}
	return nil
}
