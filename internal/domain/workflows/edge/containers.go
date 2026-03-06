package edge

import (
	"fmt"

	hatchetLib "github.com/hatchet-dev/hatchet/sdks/go"
	hatchet_ext "github.com/stroppy-io/hatchet-workflow/internal/core/hatchet-ext"
	edgeDomain "github.com/stroppy-io/hatchet-workflow/internal/domain/edge"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/edge/containers"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/deployment"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/edge"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/settings"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/workflows"
)

func SetupContainersTask(c *hatchetLib.Client, identifier *edge.Task_Identifier) *hatchetLib.StandaloneTask {
	task := c.NewStandaloneTask(
		edgeDomain.TaskIdToString(identifier),
		hatchet_ext.WTask(func(
			ctx hatchetLib.Context,
			input *workflows.Tasks_StartDockerContainers_Input,
		) (*workflows.Tasks_StartDockerContainers_Output, error) {
			if err := input.Validate(); err != nil {
				return nil, err
			}

			networkName, err := resolveContainerNetworkName(
				input.GetRunSettings().GetRunId(),
				input.GetRunSettings().GetTarget(),
				input.GetRunSettings().GetSettings(),
				input,
			)
			if err != nil {
				return nil, err
			}

			if err := containers.DeployContainersForTarget(
				ctx.GetContext(),
				ctx,
				input.GetRunSettings(),
				networkName,
				input.GetWorkerInternalIp().GetValue(),
				input.GetWorkerInternalCidr().GetValue(),
				input.GetContainers(),
			); err != nil {
				return nil, err
			}

			return &workflows.Tasks_StartDockerContainers_Output{}, nil
		}),
	)
	//task.OnFailure(func(ctx hatchetLib.Context, input workflows.Tasks_StartDockerContainers_Input) (emptypb.Empty, error) {
	//	if err := Cleanup(ctx.GetContext()); err != nil {
	//		return emptypb.Empty{}, err
	//	}
	//	return emptypb.Empty{}, fmt.Errorf("failed to start docker containers")
	//})
	return task
}

func resolveContainerNetworkName(
	runId string,
	target deployment.Target,
	settings *settings.Settings,
	input *workflows.Tasks_StartDockerContainers_Input,
) (string, error) {
	cidr := input.GetWorkerInternalCidr().GetValue()
	if cidr == "" {
		return "", fmt.Errorf("worker internal cidr is empty")
	}
	runID := containers.SanitizeDockerNamePart(runId)
	if target == deployment.Target_TARGET_DOCKER {
		base := containers.SanitizeDockerNamePart(settings.GetDocker().GetNetworkName())
		if base == "" {
			base = "stroppy"
		}
		return fmt.Sprintf("%s-%s", base, runID), nil
	}
	return fmt.Sprintf("edge-%s-%s", runID, containers.SanitizeDockerNamePart(cidr)), nil
}
