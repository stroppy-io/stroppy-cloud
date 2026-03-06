package test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	hatchetLib "github.com/hatchet-dev/hatchet/sdks/go"
	"github.com/sourcegraph/conc/pool"
	hatchet_ext "github.com/stroppy-io/hatchet-workflow/internal/core/hatchet-ext"
	edgeDomain "github.com/stroppy-io/hatchet-workflow/internal/domain/edge"
	provisionSvc "github.com/stroppy-io/hatchet-workflow/internal/domain/provision"
	edgeWorkflow "github.com/stroppy-io/hatchet-workflow/internal/domain/workflows/edge"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/deployment"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/edge"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/provision"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/stroppy"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/workflows"
	valkeygo "github.com/valkey-io/valkey-go"
	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	RunWorkflowName hatchet_ext.WorkflowName = "stroppy-test-run"

	ValidateInputTaskName         hatchet_ext.TaskName = "validate-input"
	AcquireNetworkTaskName        hatchet_ext.TaskName = "acquire-network"
	PlanPlacementIntentTaskName   hatchet_ext.TaskName = "plan-placement-intent"
	BuildPlacementTaskName        hatchet_ext.TaskName = "build-placement"
	DeployPlanTaskName            hatchet_ext.TaskName = "deploy-plan"
	WaitWorkersInHatchetTaskName  hatchet_ext.TaskName = "wait-workers-in-hatchet"
	RunDatabaseContainersTaskName hatchet_ext.TaskName = "run-database-containers"
	RunStroppyContainersTaskName  hatchet_ext.TaskName = "run-stroppy-containers"
	InstallStroppyTaskName        hatchet_ext.TaskName = "install-stroppy"
	RunStroppyTestTaskName        hatchet_ext.TaskName = "run-stroppy-test"
	DestroyPlanTaskName           hatchet_ext.TaskName = "destroy-plan"
)

const (
	ValkeyUrl = "VALKEY_URL"
)

func valkeyFromEnv() (valkeygo.Client, error) {
	urlStr := os.Getenv(ValkeyUrl)
	if urlStr == "" {
		return nil, fmt.Errorf("environment variable %s is not set", ValkeyUrl)
	}
	valkeyUrl, err := valkeygo.ParseURL(urlStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Valkey URL: %w", err)
	}
	valkeyClient, err := valkeygo.NewClient(valkeyUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to create Valkey client: %w", err)
	}
	return valkeyClient, nil
}

func TestRunWorkflow(
	c *hatchetLib.Client,
) *hatchetLib.Workflow {
	workflow := c.NewWorkflow(
		RunWorkflowName,
		hatchetLib.WithWorkflowDescription("Stroppy Test Run Workflow"),
	)
	/*
		Validate input
	*/
	validateInputTask := workflow.NewTask(
		ValidateInputTaskName,
		hatchet_ext.WTask(func(
			ctx hatchetLib.Context,
			input *workflows.Workflows_StroppyTest_Input,
		) (*workflows.Workflows_StroppyTest_Input, error) {
			err := input.Validate()
			if err != nil {
				return nil, err
			}
			return input, nil
		}),
	)

	/*
		Acquire network for test
	*/
	acquireNetworkTask := workflow.NewTask(
		AcquireNetworkTaskName,
		hatchet_ext.PTask(validateInputTask, func(
			ctx hatchetLib.Context,
			input *workflows.Workflows_StroppyTest_Input,
			parentOutput *workflows.Workflows_StroppyTest_Input,
		) (*deployment.Network, error) {
			deps, err := NewDeps(input.GetRunSettings().GetSettings())
			if err != nil {
				return nil, err
			}
			return deps.ProvisionerService.AcquireNetwork(
				ctx,
				input.GetRunSettings().GetTarget(),
				input.GetRunSettings().GetTest(),
				input.GetRunSettings().GetSettings(),
			)
		}),
		hatchetLib.WithParents(validateInputTask),
	)

	/*
		Plan placement intent for test
	*/
	planPlacementIntentTask := workflow.NewTask(
		PlanPlacementIntentTaskName,
		hatchet_ext.PTask(acquireNetworkTask, func(
			ctx hatchetLib.Context,
			input *workflows.Workflows_StroppyTest_Input,
			parentOutput *deployment.Network,
		) (*provision.PlacementIntent, error) {
			deps, err := NewDeps(input.GetRunSettings().GetSettings())
			if err != nil {
				return nil, err
			}
			return deps.ProvisionerService.PlanPlacementIntent(
				ctx,
				input.GetRunSettings().GetTest().GetDatabaseTemplate(),
				parentOutput,
			)
		}),
		hatchetLib.WithParents(acquireNetworkTask),
	)
	/*
		Build placement for test
	*/
	buildPlacementTask := workflow.NewTask(
		BuildPlacementTaskName,
		hatchet_ext.PTask(planPlacementIntentTask, func(
			ctx hatchetLib.Context,
			input *workflows.Workflows_StroppyTest_Input,
			parentOutput *provision.PlacementIntent,
		) (*provision.Placement, error) {
			deps, err := NewDeps(input.GetRunSettings().GetSettings())
			if err != nil {
				return nil, err
			}
			return deps.ProvisionerService.BuildPlacement(
				ctx,
				input.GetRunSettings().GetRunId(),
				input.GetRunSettings().GetTarget(),
				input.GetRunSettings().GetSettings(),
				input.GetRunSettings().GetTest(),
				parentOutput,
			)
		}),
		hatchetLib.WithParents(planPlacementIntentTask),
	)
	/*
		Deploy placement for test
	*/
	deployPlanTask := workflow.NewTask(
		DeployPlanTaskName,
		hatchet_ext.PTask(buildPlacementTask, func(
			ctx hatchetLib.Context,
			input *workflows.Workflows_StroppyTest_Input,
			parentOutput *provision.Placement,
		) (*provision.DeployedPlacement, error) {
			deps, err := NewDeps(input.GetRunSettings().GetSettings())
			if err != nil {
				return nil, err
			}
			return deps.ProvisionerService.DeployPlan(ctx, parentOutput)
		}),
		hatchetLib.WithExecutionTimeout(10*time.Minute),
		hatchetLib.WithParents(buildPlacementTask),
	)
	/*
		Wait for workers to be up in Hatchet
	*/
	waitWorkersInHatchetTask := workflow.NewTask(
		WaitWorkersInHatchetTaskName,
		hatchet_ext.PTask(deployPlanTask,
			func(
				ctx hatchetLib.Context,
				input *workflows.Workflows_StroppyTest_Input,
				parentOutput *provision.DeployedPlacement,
			) (*provision.DeployedPlacement, error) {
				return parentOutput, waitMultipleWorkersUp(ctx, c, parentOutput)
			},
		),
		hatchetLib.WithExecutionTimeout(2*time.Minute),
		hatchetLib.WithParents(deployPlanTask),
	)
	/*
		Run database containers
	*/
	runDatabaseContainers := workflow.NewTask(
		RunDatabaseContainersTaskName,
		hatchet_ext.PTask(waitWorkersInHatchetTask, func(
			ctx hatchetLib.Context,
			input *workflows.Workflows_StroppyTest_Input,
			parentOutput *provision.DeployedPlacement,
		) (*provision.DeployedPlacement, error) {
			dbItems := provisionSvc.SearchDatabasePlacementItem(parentOutput)
			wp := pool.New().WithErrors().WithContext(ctx.GetContext()).WithFirstError()
			for _, item := range dbItems {
				task, err := edgeDomain.FoundTaskKind(
					item.GetWorker().GetAcceptableTasks(),
					edge.Task_KIND_SETUP_CONTAINERS,
				)
				if err != nil {
					return nil, err
				}
				wp.Go(func(ctx context.Context) error {
					_, err := edgeWorkflow.SetupContainersTask(c, task).
						Run(ctx, workflows.Tasks_StartDockerContainers_Input{
							RunSettings:        input.GetRunSettings(),
							Containers:         item.GetContainers(),
							WorkerInternalIp:   item.GetVmTemplate().GetInternalIp(),
							WorkerInternalCidr: parentOutput.GetNetwork().GetCidr(),
						})
					return err
				})
			}
			return parentOutput, wp.Wait()
		}),
		hatchetLib.WithExecutionTimeout(30*time.Minute),
		hatchetLib.WithRetries(3),
		hatchetLib.WithParents(waitWorkersInHatchetTask),
	)
	/*
		Run Stroppy containers
	*/
	runStroppyContainers := workflow.NewTask(
		RunStroppyContainersTaskName,
		hatchet_ext.PTask(runDatabaseContainers, func(
			ctx hatchetLib.Context,
			input *workflows.Workflows_StroppyTest_Input,
			parentOutput *provision.DeployedPlacement,
		) (*provision.DeployedPlacement, error) {
			stroppyItems := provisionSvc.SearchStroppyPlacementItem(parentOutput)
			wp := pool.New().WithErrors().WithContext(ctx.GetContext()).WithFirstError()
			for _, item := range stroppyItems {
				task, err := edgeDomain.FoundTaskKind(
					item.GetWorker().GetAcceptableTasks(),
					edge.Task_KIND_SETUP_CONTAINERS,
				)
				if err != nil {
					return nil, err
				}
				wp.Go(func(ctx context.Context) error {
					_, err := edgeWorkflow.SetupContainersTask(c, task).
						Run(ctx, workflows.Tasks_StartDockerContainers_Input{
							RunSettings:        input.GetRunSettings(),
							Containers:         item.GetContainers(),
							WorkerInternalIp:   item.GetVmTemplate().GetInternalIp(),
							WorkerInternalCidr: parentOutput.GetNetwork().GetCidr(),
						})
					return err
				})
			}
			return parentOutput, wp.Wait()
		}),
		hatchetLib.WithExecutionTimeout(30*time.Minute),
		hatchetLib.WithRetries(3),
		hatchetLib.WithParents(runDatabaseContainers),
	)
	/*
		Install Stroppy on edge workers
	*/
	installStroppyTask := workflow.NewTask(
		InstallStroppyTaskName,
		hatchet_ext.PTask(runStroppyContainers, func(
			ctx hatchetLib.Context,
			input *workflows.Workflows_StroppyTest_Input,
			parentOutput *provision.DeployedPlacement,
		) (*provision.DeployedPlacement, error) {
			stroppyItems := provisionSvc.SearchStroppyPlacementItem(parentOutput)
			wp := pool.New().WithErrors().WithContext(ctx.GetContext()).WithFirstError()
			for _, item := range stroppyItems {
				task, err := edgeDomain.FoundTaskKind(
					item.GetWorker().GetAcceptableTasks(),
					edge.Task_KIND_INSTALL_STROPPY,
				)
				if err != nil {
					return nil, err
				}
				wp.Go(func(ctx context.Context) error {
					_, err := edgeWorkflow.InstallStroppy(c, task).
						Run(ctx, workflows.Tasks_InstallStroppy_Input{
							RunSettings: input.GetRunSettings(),
							StroppyCli:  input.GetRunSettings().GetTest().GetStroppyCli(),
						})
					return err
				})
			}
			return parentOutput, wp.Wait()
		}),
		// TODO: add if default timeout is set too low
		//hatchetLib.WithExecutionTimeout(10*time.Minute),
		hatchetLib.WithRetries(3),
		hatchetLib.WithParents(runStroppyContainers),
	)
	/*
		Run Stroppy Test
	*/
	runStroppyTestTask := workflow.NewTask(
		RunStroppyTestTaskName,
		hatchet_ext.PTask(installStroppyTask, func(
			ctx hatchetLib.Context,
			input *workflows.Workflows_StroppyTest_Input,
			parentOutput *provision.DeployedPlacement,
		) (*workflows.Tasks_RunStroppy_Output, error) {
			stroppyItems := provisionSvc.SearchStroppyPlacementItem(parentOutput)
			results := make([]*stroppy.TestResult, 0)
			wp := pool.New().WithErrors().WithContext(ctx.GetContext()).WithFirstError()
			for _, item := range stroppyItems {
				task, err := edgeDomain.FoundTaskKind(
					item.GetWorker().GetAcceptableTasks(),
					edge.Task_KIND_RUN_STROPPY,
				)
				if err != nil {
					return nil, err
				}
				wp.Go(func(ctx context.Context) error {
					taskResult, err := edgeWorkflow.RunStroppyTask(c, task).
						Run(ctx, workflows.Tasks_RunStroppy_Input{
							RunSettings:      input.GetRunSettings(),
							StroppyCliCall:   input.GetRunSettings().GetTest().GetStroppyCli(),
							ConnectionString: parentOutput.GetConnectionString(),
						})
					if err != nil {
						return err
					}
					var testOutput stroppy.TestResult
					if err := taskResult.Into(&testOutput); err != nil {
						return err
					}
					results = append(results, &testOutput)
					return nil
				})
			}
			err := wp.Wait()
			if err != nil {
				return nil, err
			}
			return &workflows.Tasks_RunStroppy_Output{
				Result:    results,
				Placement: parentOutput,
			}, nil
		}),
		hatchetLib.WithExecutionTimeout(24*time.Hour),
		hatchetLib.WithParents(installStroppyTask),
	)
	/*
		Destroy placement on end
	*/
	_ = workflow.NewTask(
		DestroyPlanTaskName,
		hatchet_ext.PTask(runStroppyTestTask, func(
			ctx hatchetLib.Context,
			input *workflows.Workflows_StroppyTest_Input,
			parentOutput *workflows.Tasks_RunStroppy_Output,
		) (*workflows.Workflows_StroppyTest_Output, error) {
			deps, err := NewDeps(input.GetRunSettings().GetSettings())
			if err != nil {
				return nil, err
			}
			err = deps.ProvisionerService.DestroyPlan(ctx, parentOutput.GetPlacement())
			if err != nil {
				return nil, err
			}
			err = deps.ProvisionerService.DestroyNetwork(ctx, parentOutput.GetPlacement().GetNetwork())
			if err != nil {
				return nil, err
			}
			// Remove the per-run Docker bridge network created by edge workers.
			_ = cleanupDockerRunNetwork(ctx.GetContext(), input.GetRunSettings().GetRunId(), input.GetRunSettings())
			return &workflows.Workflows_StroppyTest_Output{
				Result: parentOutput.GetResult(),
			}, nil
		}),
		hatchetLib.WithParents(runStroppyTestTask),
	)
	/*
		Destroy deployments on failure
	*/
	workflow.OnFailure(
		func(ctx hatchetLib.Context, input workflows.Workflows_StroppyTest_Input) (emptypb.Empty, error) {
			ctx.Log("Workflow failed, start cleanup")
			delay := input.GetRunSettings().GetSettings().GetCleanupDelay()
			if delay != nil {
				// Sleep for cleanup delay if user wants it for diagnostics purposes
				ctx.Log(fmt.Sprintf("Wait for delay %s", delay.AsDuration().String()))
				time.Sleep(delay.AsDuration())
			}
			deps, err := NewDeps(input.GetRunSettings().GetSettings())
			if err != nil {
				return emptypb.Empty{}, err
			}
			var errs []error

			var placement provision.DeployedPlacement
			if err := ctx.ParentOutput(deployPlanTask, &placement); err != nil {
				return emptypb.Empty{}, err
			}
			if err := deps.ProvisionerService.DestroyPlan(ctx.GetContext(), &placement); err != nil {
				errs = append(errs, err)
			}
			var net deployment.Network
			if err := ctx.ParentOutput(acquireNetworkTask, &net); err != nil {
				return emptypb.Empty{}, err
			}
			if err := deps.ProvisionerService.DestroyNetwork(ctx.GetContext(), &net); err != nil {
				errs = append(errs, err)
			}
			// Remove the per-run Docker bridge network created by edge workers.
			if err := cleanupDockerRunNetwork(ctx.GetContext(), input.GetRunSettings().GetRunId(), input.GetRunSettings()); err != nil {
				errs = append(errs, err)
			}
			return emptypb.Empty{}, errors.Join(errs...)
		},
	)
	return workflow
}
