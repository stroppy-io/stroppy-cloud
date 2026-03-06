package provision

import (
	"context"
	"fmt"
	"net"
	"os"

	"github.com/samber/lo"
	"github.com/stroppy-io/hatchet-workflow/internal/core/consts"
	"github.com/stroppy-io/hatchet-workflow/internal/core/defaults"
	"github.com/stroppy-io/hatchet-workflow/internal/core/ids"
	"github.com/stroppy-io/hatchet-workflow/internal/core/ips"
	"github.com/stroppy-io/hatchet-workflow/internal/core/logger"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/deployment/scripting"
	edgeDomain "github.com/stroppy-io/hatchet-workflow/internal/domain/edge"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/topology"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/database"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/deployment"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/edge"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/provision"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/settings"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/stroppy"
	"go.uber.org/zap/zapcore"
)

const (
	DefaultUserGroupName consts.DefaultValue = "stroppy-edge-worker"
	DefaultUserSudo      bool                = true
	DefaultUserName      consts.DefaultValue = "stroppy"
	DefaultUserSudoRules consts.DefaultValue = "ALL=(ALL) NOPASSWD:ALL"
	DefaultUserShell     consts.DefaultValue = "/bin/bash"
)

const (
	HatchetClientHostPortKey    consts.EnvKey = "HATCHET_CLIENT_HOST_PORT"
	HatchetClientTokenKey       consts.EnvKey = "HATCHET_CLIENT_TOKEN"
	HatchetClientTlsStrategyKey consts.EnvKey = "HATCHET_CLIENT_TLS_STRATEGY"

	HatchetClientTlsStrategyNone consts.Str = "none"

	metadataRoleKey          consts.ConstValue   = "METADATA_ROLE"
	metadataRunIdKey         consts.ConstValue   = "METADATA_RUN_ID"
	metadataRoleStroppyValue consts.DefaultValue = "stroppy"
	metadataDatabaseValue    consts.DefaultValue = "database"
	globalDeploymentName     consts.DefaultValue = "stroppy-test-run-deployment"
)

func stroppyMetadata(runId string) map[string]string {
	return map[string]string{
		metadataRoleKey:  metadataRoleStroppyValue,
		metadataRunIdKey: runId,
	}
}
func databaseMetadata(runId string) map[string]string {
	return map[string]string{
		metadataRoleKey:  metadataDatabaseValue,
		metadataRunIdKey: runId,
	}
}

func SearchDatabasePlacementItem(
	placement *provision.DeployedPlacement,
) []*provision.Placement_Item {
	var items []*provision.Placement_Item
	for _, item := range placement.GetItems() {
		if item.GetPlacementItem().GetMetadata()[metadataRoleKey] == metadataDatabaseValue {
			items = append(items, item.GetPlacementItem())
		}
	}
	return items
}

func SearchStroppyPlacementItem(
	placement *provision.DeployedPlacement,
) []*provision.Placement_Item {
	var items []*provision.Placement_Item
	for _, item := range placement.GetItems() {
		if item.GetPlacementItem().GetMetadata()[metadataRoleKey] == metadataRoleStroppyValue {
			items = append(items, item.GetPlacementItem())
		}
	}
	return items
}

type NetworkManager interface {
	ReserveNetwork(
		ctx context.Context,
		networkIdentifier *deployment.Identifier,
		baseCidr string,
		basePrefix int,
		ipCount int,
	) (*deployment.Network, error)
	FreeNetwork(
		ctx context.Context,
		network *deployment.Network,
	) error
}

type DeploymentService interface {
	CreateDeployment(
		ctx context.Context,
		depl *deployment.Deployment_Template,
	) (*deployment.Deployment, error)
	DestroyDeployment(
		ctx context.Context,
		depl *deployment.Deployment,
	) error
}

type ProvisionerService struct {
	networkManager    NetworkManager
	deploymentService DeploymentService
}

func NewProvisionerService(
	networkManager NetworkManager,
	deploymentService DeploymentService,
) *ProvisionerService {
	return &ProvisionerService{
		networkManager:    networkManager,
		deploymentService: deploymentService,
	}
}

const (
	defaultDockerNetworkCidr   = "172.28.0.0/16"
	defaultDockerNetworkPrefix = 24

	defaultCloudNetworkCidr   = "10.2.0.0/16"
	defaultCloudNetworkPrefix = 24
)

func (p ProvisionerService) AcquireNetwork(
	ctx context.Context,
	target deployment.Target,
	test *stroppy.Test,
	settings *settings.Settings,
) (*deployment.Network, error) {
	var count int
	switch test.GetDatabaseRef().(type) {
	case *stroppy.Test_DatabaseTemplate:
		count = topology.RequiredIPCount(test.GetDatabaseTemplate())
	case *stroppy.Test_ConnectionString:
		count = 1
	}
	count += 1 // for stroppy deployment

	networkName, baseCidr, basePrefix := resolveNetworkSettings(target, settings)

	return p.networkManager.ReserveNetwork(
		ctx,
		&deployment.Identifier{
			Id:     ids.NewUlid().Lower().String(),
			Name:   networkName,
			Target: target,
		},
		baseCidr,
		basePrefix,
		count,
	)
}

// resolveVmSettings returns cloud-specific VM settings based on target.
// For Docker target, returns nil (Docker doesn't create real VMs).
func resolveVmSettings(
	target deployment.Target,
	s *settings.Settings,
) (vmUser *deployment.VmUser, baseImageId string, hasPublicIp bool) {
	switch target {
	case deployment.Target_TARGET_YANDEX_CLOUD:
		vm := s.GetYandexCloud().GetVmSettings()
		return vm.GetVmUser(), vm.GetBaseImageId(), vm.GetEnablePublicIps()
	default:
		return nil, "", false
	}
}

func resolveNetworkSettings(
	target deployment.Target,
	s *settings.Settings,
) (networkName string, baseCidr string, basePrefix int) {
	switch target {
	case deployment.Target_TARGET_YANDEX_CLOUD:
		return s.GetYandexCloud().GetNetworkSettings().GetName(),
			defaultCloudNetworkCidr,
			defaultCloudNetworkPrefix
	default:
		docker := s.GetDocker()
		cidr := docker.GetNetworkCidr()
		if cidr == "" {
			cidr = defaultDockerNetworkCidr
		}
		prefix := int(docker.GetNetworkPrefix())
		if prefix == 0 {
			prefix = defaultDockerNetworkPrefix
		}
		return docker.GetNetworkName(), cidr, prefix
	}
}

func (p ProvisionerService) FreeNetwork(
	ctx context.Context,
	network *deployment.Network,
) error {
	return p.networkManager.FreeNetwork(ctx, network)
}

func (p ProvisionerService) PlanPlacementIntent(
	_ context.Context,
	template *database.Database_Template,
	network *deployment.Network,
) (*provision.PlacementIntent, error) {
	err := network.Validate()
	if err != nil {
		return nil, err
	}
	builder := topology.NewPostgresPlacementBuilder(network)
	switch t := template.GetTemplate().(type) {
	case *database.Database_Template_PostgresInstance:
		return builder.BuildForPostgresInstance(t)
	case *database.Database_Template_PostgresCluster:
		return builder.BuildForPostgresCluster(t)
	case *database.Database_Template_PicodataInstance:
		return topology.NewPicodataPlacementBuilder(network).BuildForPicodataInstance(t.PicodataInstance)
	case *database.Database_Template_PicodataCluster:
		return topology.NewPicodataPlacementBuilder(network).BuildForPicodataCluster(t.PicodataCluster)
	default:
		return nil, fmt.Errorf("unknown database template type")
	}
}

func (p ProvisionerService) newStroppyWorkerIp(
	network *deployment.Network,
	reservedIps []*provision.PlacementIntent_Item,
) (*deployment.Ip, error) {
	_, cidr, err := net.ParseCIDR(network.GetCidr().GetValue())
	if err != nil {
		return nil, err
	}
	ip, err := ips.FirstFreeIP(cidr, lo.Map(
		reservedIps,
		func(i *provision.PlacementIntent_Item, _ int) string { return i.GetInternalIp().GetValue() },
	))
	if err != nil {
		return nil, err
	}
	return &deployment.Ip{
		Value: ip.String(),
	}, nil
}

func (p ProvisionerService) getCloudInitForEdgeWorker(
	workerName string,
	vmUser *deployment.VmUser,
	acceptableTasks []*edge.Task_Identifier,
	settings *settings.HatchetConnection,
) (*deployment.CloudInit, error) {
	return scripting.InstallEdgeWorkerCloudInit(
		scripting.WithUser(&deployment.VmUser{
			Name:              defaults.StringOrDefault(vmUser.GetName(), DefaultUserName),
			SudoRules:         defaults.StringOrDefault(vmUser.GetSudoRules(), DefaultUserSudoRules),
			Shell:             defaults.StringOrDefault(vmUser.GetShell(), DefaultUserShell),
			Groups:            defaults.ArrayOrDefault(vmUser.GetGroups(), []string{DefaultUserGroupName}),
			SshAuthorizedKeys: vmUser.GetSshAuthorizedKeys(),
		}),
		scripting.WithEnv(map[string]string{
			edgeDomain.WorkerNameEnvKey:            workerName,
			edgeDomain.WorkerAcceptableTasksEnvKey: edgeDomain.TaskIdListToString(acceptableTasks),
			HatchetClientHostPortKey:               net.JoinHostPort(settings.GetHost(), fmt.Sprintf("%d", settings.GetPort())),
			HatchetClientTokenKey:                  settings.GetToken(),
			HatchetClientTlsStrategyKey:            HatchetClientTlsStrategyNone,
			logger.LevelEnvKey: defaults.StringOrDefault(
				os.Getenv(logger.LevelEnvKey),
				zapcore.InfoLevel.String(),
			),
			logger.LogModEnvKey: defaults.StringOrDefault(
				os.Getenv(logger.LogModEnvKey),
				logger.ProductionMod.String(),
			),
			logger.LogMappingEnvKey: os.Getenv(logger.LogMappingEnvKey),
			logger.LogSkipCallerEnvKey: defaults.StringOrDefault(
				os.Getenv(logger.LogSkipCallerEnvKey),
				"true",
			),
		}),
	)
}

func (p ProvisionerService) BuildPlacement(
	_ context.Context,
	runId string,
	target deployment.Target,
	settings *settings.Settings,
	test *stroppy.Test,
	intent *provision.PlacementIntent,
) (*provision.Placement, error) {
	var items []*provision.Placement_Item
	runIdParsed := ids.ParseRunId(runId)
	vmTemplates := make([]*deployment.Vm_Template, 0)
	vmUser, baseImageId, hasPublicIp := resolveVmSettings(target, settings)

	for _, item := range intent.GetItems() {
		workerName := edgeDomain.NewWorkerName(runIdParsed, item.GetName())
		workerAcceptableTasks := []*edge.Task_Identifier{
			edgeDomain.NewTaskId(runIdParsed, edge.Task_KIND_SETUP_CONTAINERS),
			edgeDomain.NewTaskId(runIdParsed, edge.Task_KIND_RUN_STROPPY),
		}
		metadata := lo.Assign(
			item.GetMetadata(),
			databaseMetadata(runId),
		)
		worker := &edge.Worker{
			WorkerName:      workerName,
			AcceptableTasks: workerAcceptableTasks,
			Metadata:        metadata,
		}
		workerCloudInit, err := p.getCloudInitForEdgeWorker(
			workerName,
			vmUser,
			workerAcceptableTasks,
			settings.GetHatchetConnection(),
		)
		if err != nil {
			return nil, err
		}
		vmTemplate := &deployment.Vm_Template{
			Identifier: &deployment.Identifier{
				Id:     ids.NewUlid().Lower().String(),
				Name:   workerName,
				Target: target,
			},
			Hardware:    item.GetHardware(),
			BaseImageId: baseImageId,
			HasPublicIp: hasPublicIp,
			VmUser:      vmUser,
			InternalIp:  item.GetInternalIp(),
			CloudInit:   workerCloudInit,
			Labels:      metadata,
		}
		vmTemplates = append(vmTemplates, vmTemplate)
		items = append(items, &provision.Placement_Item{
			Name:       item.GetName(),
			Containers: item.GetContainers(),
			VmTemplate: vmTemplate,
			Worker:     worker,
			Metadata:   metadata,
		})
	}

	stroppyWorkerName := edgeDomain.NewWorkerName(runIdParsed, metadataRoleStroppyValue)
	stroppyWorkerIp, err := p.newStroppyWorkerIp(intent.GetNetwork(), intent.GetItems())
	if err != nil {
		return nil, err
	}
	stroppyWorkerAcceptableTasks := []*edge.Task_Identifier{
		edgeDomain.NewTaskId(runIdParsed, edge.Task_KIND_SETUP_CONTAINERS),
		edgeDomain.NewTaskId(runIdParsed, edge.Task_KIND_INSTALL_STROPPY),
		edgeDomain.NewTaskId(runIdParsed, edge.Task_KIND_RUN_STROPPY),
	}
	stroppyCloudInit, err := p.getCloudInitForEdgeWorker(
		stroppyWorkerName,
		vmUser,
		stroppyWorkerAcceptableTasks,
		settings.GetHatchetConnection(),
	)
	if err != nil {
		return nil, err
	}
	stroppyMd := stroppyMetadata(runId)

	stroppyPlacementItem := &provision.Placement_Item{
		Name: metadataRoleStroppyValue,
		Containers: []*provision.Container{
			topology.NewNodeExporterContainer(metadataRoleStroppyValue, true),
		},
		VmTemplate: &deployment.Vm_Template{
			Identifier: &deployment.Identifier{
				Id:     ids.NewUlid().Lower().String(),
				Name:   stroppyWorkerName,
				Target: target,
			},
			Hardware:    test.GetStroppyHardware(),
			BaseImageId: baseImageId,
			HasPublicIp: hasPublicIp,
			VmUser:      vmUser,
			InternalIp:  stroppyWorkerIp,
			CloudInit:   stroppyCloudInit,
			Labels:      stroppyMd,
		},
		Worker: &edge.Worker{
			WorkerName:      stroppyWorkerName,
			AcceptableTasks: stroppyWorkerAcceptableTasks,
			Metadata:        stroppyMd,
		},
		Metadata: stroppyMd,
	}
	return &provision.Placement{
		Network:          intent.GetNetwork(),
		ConnectionString: intent.GetConnectionString(),
		DeploymentTemplate: &deployment.Deployment_Template{
			Identifier: &deployment.Identifier{
				Id:     ids.NewUlid().Lower().String(),
				Name:   globalDeploymentName,
				Target: target,
			},
			Network:     intent.GetNetwork(),
			VmTemplates: append(vmTemplates, stroppyPlacementItem.GetVmTemplate()),
			Metadata:    stroppyMd,
		},
		Items: append(items, stroppyPlacementItem),
	}, nil
}

func (p ProvisionerService) DeployPlan(
	ctx context.Context,
	placement *provision.Placement,
) (*provision.DeployedPlacement, error) {
	depl, err := p.deploymentService.CreateDeployment(ctx, placement.GetDeploymentTemplate())
	if err != nil {
		return nil, err
	}
	var deployedItems []*provision.DeployedPlacement_Item
	for _, item := range placement.GetItems() {
		vm, ok := lo.Find(depl.GetVms(), func(i *deployment.Vm) bool {
			return i.GetTemplate().GetIdentifier().GetId() == item.GetVmTemplate().GetIdentifier().GetId()
		})
		if !ok {
			return nil, fmt.Errorf(
				"vm instance not found for vm template %s",
				item.GetVmTemplate().GetIdentifier().GetId(),
			)
		}
		deployedItems = append(deployedItems, &provision.DeployedPlacement_Item{
			PlacementItem: item,
			Vm:            vm,
		})
	}
	return &provision.DeployedPlacement{
		Items:            deployedItems,
		Deployment:       depl,
		Network:          placement.GetNetwork(),
		ConnectionString: placement.GetConnectionString(),
	}, nil
}

func (p ProvisionerService) DestroyPlan(
	ctx context.Context,
	deployedPlacement *provision.DeployedPlacement,
) error {
	return p.deploymentService.DestroyDeployment(ctx, deployedPlacement.GetDeployment())
}

func (p ProvisionerService) DestroyNetwork(
	ctx context.Context,
	network *deployment.Network,
) error {
	return p.networkManager.FreeNetwork(ctx, network)
}
