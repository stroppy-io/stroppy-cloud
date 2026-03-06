package test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hatchet-dev/hatchet/pkg/client/rest"
	hatchetLib "github.com/hatchet-dev/hatchet/sdks/go"
	"github.com/samber/lo"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/edge/containers"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/deployment"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/provision"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/stroppy"
)

var ErrWorkersNotUp = errors.New("workers not up")

// cleanupDockerRunNetwork removes the per-run Docker bridge network created by edge workers.
// This is needed because DestroyPlan only stops edge worker containers; the bridge network persists.
func cleanupDockerRunNetwork(ctx context.Context, runID string, runSettings *stroppy.RunSettings) error {
	if runSettings.GetTarget() != deployment.Target_TARGET_DOCKER {
		return nil
	}
	base := runSettings.GetSettings().GetDocker().GetNetworkName()
	if base == "" {
		base = "stroppy"
	}
	networkName := fmt.Sprintf("%s-%s", containers.SanitizeDockerNamePart(base), containers.SanitizeDockerNamePart(runID))
	return containers.RemoveNetwork(ctx, networkName)
}

func waitMultipleWorkersUp(hctx hatchetLib.Context, c *hatchetLib.Client, provision *provision.DeployedPlacement) error {
	var names []string
	for _, item := range provision.GetItems() {
		if item.GetPlacementItem().GetWorker() == nil || item.GetPlacementItem().GetWorker().GetWorkerName() == "" {
			return fmt.Errorf("worker name not found for item %+v", item)
		}
		names = append(names, item.GetPlacementItem().GetWorker().GetWorkerName())
	}
	for {
		select {
		case <-hctx.Done():
			return errors.Join(ErrWorkersNotUp, hctx.Err())
		default:
			workers, err := c.Workers().List(hctx.GetContext())
			if err != nil {
				return err
			}
			targetWorkers := lo.Filter(*workers.Rows,
				func(w rest.Worker, _ int) bool { return lo.Contains(names, w.Name) },
			)
			if len(targetWorkers) == len(names) {
				return nil
			}
			time.Sleep(time.Second)
		}
	}
}
