package run

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"go.uber.org/zap"

	"github.com/stroppy-io/stroppy-cloud/internal/core/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/terraform"

	yctf "github.com/stroppy-io/stroppy-cloud/deployments/terraform/yandex"
)

// networkTask creates a Docker network (for docker provider).
type networkTask struct {
	cfg      types.NetworkConfig
	provider types.Provider
	deployer *agent.DockerDeployer
	state    *State
	runID    string
}

func (t *networkTask) Execute(nc *dag.NodeContext) error {
	switch t.provider {
	case types.ProviderDocker:
		return t.dockerNetwork(nc)
	case types.ProviderYandex:
		return t.yandexNetwork(nc)
	default:
		nc.Log().Info("network phase: skipping (handled by terraform)", zap.String("provider", string(t.provider)))
		return nil
	}
}

func (t *networkTask) yandexNetwork(nc *dag.NodeContext) error {
	// VPC/subnet creation is handled as part of the machines terraform apply.
	// Log the intent for observability; no error so the pipeline proceeds.
	nc.Log().Info("network phase: Yandex Cloud VPC/subnet will be provisioned by terraform in machines phase",
		zap.String("cidr", t.cfg.CIDR),
		zap.String("zone", t.cfg.Zone),
	)
	return nil
}

func (t *networkTask) dockerNetwork(nc *dag.NodeContext) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("network: docker client: %w", err)
	}
	defer cli.Close()

	netName := fmt.Sprintf("stroppy-%s", t.runID)
	nc.Log().Info("creating docker network", zap.String("name", netName))

	// Reuse if already exists.
	nets, err := cli.NetworkList(nc, network.ListOptions{})
	if err == nil {
		for _, n := range nets {
			if n.Name == netName {
				t.state.SetNetworkID(n.ID)
				nc.Log().Info("docker network already exists, reusing", zap.String("id", n.ID))
				return nil
			}
		}
	}

	resp, err := cli.NetworkCreate(nc, netName, network.CreateOptions{
		Driver: "bridge",
		Labels: map[string]string{"stroppy": "true"},
	})
	if err != nil {
		return fmt.Errorf("network: docker create: %w", err)
	}

	t.state.SetNetworkID(resp.ID)
	nc.Log().Info("docker network created", zap.String("id", resp.ID))

	return nil
}

// machinesTask provisions containers (docker) or VMs (cloud).
// On completion it populates State with agent targets.
type machinesTask struct {
	runCfg     types.RunConfig
	state      *State
	deployer   *agent.DockerDeployer
	serverAddr string
	settings   *types.ServerSettings
}

func (t *machinesTask) Execute(nc *dag.NodeContext) error {
	switch t.runCfg.Provider {
	case types.ProviderDocker:
		return t.dockerMachines(nc)
	case types.ProviderYandex:
		return t.yandexMachines(nc)
	default:
		return fmt.Errorf("machines: unsupported provider %q (supported: %s, %s)", t.runCfg.Provider, types.ProviderDocker, types.ProviderYandex)
	}
}

func (t *machinesTask) dockerMachines(nc *dag.NodeContext) error {
	if t.deployer == nil {
		return fmt.Errorf("machines: DockerDeployer is nil")
	}

	ctx := context.Context(nc)
	var dbTargets []agent.Target
	var proxyTargets []agent.Target

	// Deploy database machines.
	for _, spec := range t.runCfg.Machines {
		for i := range spec.Count {
			machineID := fmt.Sprintf("%s-%s-%d", t.runCfg.ID, spec.Role, i)
			port := agent.DefaultAgentPort

			nc.Log().Info("deploying container",
				zap.String("machine_id", machineID),
				zap.String("role", string(spec.Role)),
			)

			result, err := t.deployer.Deploy(ctx, machineID, t.serverAddr, port)
			if err != nil {
				return fmt.Errorf("machines: deploy %s: %w", machineID, err)
			}
			t.state.AddContainerID(result.ContainerID)

			// Agent polls server for commands — no inbound port needed.
			// InternalHost (container name) is used for inter-container communication
			// (e.g., stroppy connecting to the DB container).
			target := agent.Target{
				ID:           machineID,
				InternalHost: result.ContainerName,
			}

			switch spec.Role {
			case types.RoleDatabase:
				dbTargets = append(dbTargets, target)
				if len(dbTargets) == 1 {
					// First DB target is master -- store for stroppy to connect.
					dbPort := 5432 // postgres default
					switch t.runCfg.Database.Kind {
					case types.DatabaseMySQL:
						dbPort = 3306
					case types.DatabasePicodata:
						dbPort = 4327 // pgproto
					}
					t.state.SetDBEndpoint(result.ContainerName, dbPort)
				}
			case types.RoleProxy:
				proxyTargets = append(proxyTargets, target)
			case types.RoleStroppy:
				t.state.SetStroppyTarget(target)
			}

			nc.Log().Info("container deployed",
				zap.String("machine_id", machineID),
				zap.String("container_id", result.ContainerID[:12]),
			)
		}
	}

	t.state.SetDBTargets(dbTargets)
	t.state.SetProxyTargets(proxyTargets)

	nc.Log().Info("all machines provisioned",
		zap.Int("db", len(dbTargets)),
	)
	return nil
}

// runSubnetCIDR derives a unique /16 CIDR from the run ID.
// Given a base like "10.0.0.0/8", it hashes the runID to pick the second octet (1-254),
// producing e.g. "10.42.0.0/16". This avoids collisions between concurrent runs.
func runSubnetCIDR(runID string, _ string) string {
	var h byte
	for _, b := range []byte(runID) {
		h = h*31 + b
	}
	octet := int(h%254) + 1 // 1..254
	return fmt.Sprintf("10.%d.0.0/16", octet)
}

// yandexTfVars matches the terraform variable structure from main branch.
type yandexTfVars struct {
	Networking yandexTfNetworking `json:"networking"`
	Compute    yandexTfCompute    `json:"compute"`
}

type yandexTfNetworking struct {
	Name       string `json:"name"`
	ExternalID string `json:"external_id"`
	CIDR       string `json:"cidr"`
	Zone       string `json:"zone"`
}

type yandexTfCompute struct {
	PlatformID       string                `json:"platform_id"`
	ImageID          string                `json:"image_id"`
	SerialPortEnable bool                  `json:"serial_port_enable"`
	VMs              map[string]yandexTfVM `json:"vms"`
}

type yandexTfVM struct {
	Cores       int    `json:"cores"`
	Memory      int    `json:"memory"`
	DiskSize    int    `json:"disk_size"`
	InternalIP  string `json:"internal_ip"`
	HasPublicIP bool   `json:"has_public_ip"`
	UserData    string `json:"user_data"`
}

// yandexVmIPs is the terraform output structure for vm_ips.
type yandexVmIPs map[string]struct {
	ID         string `json:"id"`
	NatIP      string `json:"nat_ip"`
	InternalIP string `json:"internal_ip"`
}

func (t *machinesTask) yandexMachines(nc *dag.NodeContext) error {
	if t.settings == nil {
		return fmt.Errorf("machines: server settings not configured for Yandex Cloud provider")
	}

	cloud := t.settings.Cloud
	if err := cloud.ValidateCloud(); err != nil {
		return fmt.Errorf("machines: %w", err)
	}
	yc := cloud.Yandex
	if err := yc.Validate(); err != nil {
		return fmt.Errorf("machines: %w", err)
	}

	// Determine binary URL for cloud-init.
	binaryURL := cloud.BinaryURL
	if binaryURL == "" && t.serverAddr != "" {
		binaryURL = t.serverAddr + "/agent/binary"
	}
	if binaryURL == "" {
		return fmt.Errorf("machines: binary_url or server_addr must be configured for cloud provider")
	}

	// Load embedded terraform templates for Yandex Cloud.
	tfFiles, err := yctf.EmbeddedTfFiles()
	if err != nil {
		return fmt.Errorf("machines: load embedded terraform templates: %w", err)
	}
	if len(tfFiles) == 0 {
		return fmt.Errorf("machines: no embedded terraform templates found")
	}

	// Build cloud-init and VM specs for each machine.
	vmSpecs := make(map[string]yandexTfVM)
	// Track role per VM name for state population after apply.
	vmRoles := make(map[string]types.MachineRole)

	for _, spec := range t.runCfg.Machines {
		for i := range spec.Count {
			machineID := fmt.Sprintf("%s-%s-%d", t.runCfg.ID, spec.Role, i)

			cloudInit, ciErr := agent.GenerateCloudInit(agent.CloudInitParams{
				BinaryURL:    binaryURL,
				ServerAddr:   t.serverAddr,
				AgentPort:    agent.DefaultAgentPort,
				MachineID:    machineID,
				SSHUser:      yc.SSHUser,
				SSHPublicKey: yc.SSHPublicKey,
			})
			if ciErr != nil {
				return fmt.Errorf("machines: generate cloud-init for %s: %w", machineID, ciErr)
			}

			cores := spec.CPUs
			if cores == 0 {
				cores = 2
			}
			memGB := spec.MemoryMB / 1024
			if memGB == 0 {
				memGB = 4
			}
			diskGB := spec.DiskGB
			if diskGB == 0 {
				diskGB = 50
			}

			nc.Log().Info("preparing VM",
				zap.String("machine_id", machineID),
				zap.String("role", string(spec.Role)),
				zap.Int("cores", cores),
				zap.Int("memory_gb", memGB),
				zap.Int("disk_gb", diskGB),
			)

			vmSpecs[machineID] = yandexTfVM{
				Cores:       cores,
				Memory:      memGB,
				DiskSize:    diskGB,
				HasPublicIP: yc.AssignPublicIP,
				UserData:    cloudInit,
			}
			vmRoles[machineID] = spec.Role
		}
	}

	platformID := yc.PlatformID
	if platformID == "" {
		platformID = "standard-v2"
	}

	// Generate unique subnet name and CIDR per run to avoid collisions.
	subnetName := fmt.Sprintf("%s-%s", yc.NetworkName, t.runCfg.ID)
	subnetCIDR := runSubnetCIDR(t.runCfg.ID, yc.SubnetCIDR)

	// Build terraform variables matching main branch format.
	vars := yandexTfVars{
		Networking: yandexTfNetworking{
			Name:       subnetName,
			ExternalID: yc.NetworkID,
			CIDR:       subnetCIDR,
			Zone:       yc.Zone,
		},
		Compute: yandexTfCompute{
			PlatformID:       platformID,
			ImageID:          yc.ImageID,
			SerialPortEnable: true,
			VMs:              vmSpecs,
		},
	}

	varFile, err := terraform.NewTfVarFile(vars)
	if err != nil {
		return fmt.Errorf("machines: marshal terraform vars: %w", err)
	}

	// Create terraform actor and apply.
	actor, err := terraform.NewActor()
	if err != nil {
		return fmt.Errorf("machines: create terraform actor: %w", err)
	}

	wdId := terraform.NewWdId(t.runCfg.ID)
	wd := terraform.NewWorkdirWithParams(wdId,
		terraform.WithTfFiles(tfFiles),
		terraform.WithVarFile(varFile),
		terraform.WithEnv(map[string]string{
			"YC_TOKEN":     yc.Token,
			"YC_CLOUD_ID":  yc.CloudID,
			"YC_FOLDER_ID": yc.FolderID,
			"YC_ZONE":      yc.Zone,
		}),
	)

	nc.Log().Info("running terraform apply for Yandex Cloud",
		zap.String("run_id", t.runCfg.ID),
		zap.Int("vm_count", len(vmSpecs)),
	)

	ctx := context.Context(nc)
	output, err := actor.ApplyTerraform(ctx, wd)
	if err != nil {
		return fmt.Errorf("machines: terraform apply: %w", err)
	}

	// Store working directory ID and actor for teardown.
	t.state.SetTerraformWdId(string(wdId))
	t.state.SetTerraformActor(actor)

	// Parse terraform output — main branch format returns nat_ip + internal_ip.
	vmIPs, err := terraform.GetTfOutputVal[yandexVmIPs](output, "vm_ips")
	if err != nil {
		return fmt.Errorf("machines: parse terraform output 'vm_ips': %w", err)
	}

	// Populate state with targets from terraform output.
	var dbTargets []agent.Target
	var proxyTargets []agent.Target

	for name, role := range vmRoles {
		vmInfo, ok := vmIPs[name]
		if !ok {
			return fmt.Errorf("machines: terraform output missing IP for VM %q", name)
		}

		ip := vmInfo.NatIP
		if ip == "" {
			ip = vmInfo.InternalIP
		}

		target := agent.Target{
			ID:           name,
			Host:         ip,
			InternalHost: vmInfo.InternalIP,
		}

		switch role {
		case types.RoleDatabase:
			dbTargets = append(dbTargets, target)
			if len(dbTargets) == 1 {
				dbPort := 5432
				switch t.runCfg.Database.Kind {
				case types.DatabaseMySQL:
					dbPort = 3306
				case types.DatabasePicodata:
					dbPort = 4327
				}
				t.state.SetDBEndpoint(vmInfo.InternalIP, dbPort)
			}
		case types.RoleProxy:
			proxyTargets = append(proxyTargets, target)
		case types.RoleStroppy:
			t.state.SetStroppyTarget(target)
		}
	}

	t.state.SetDBTargets(dbTargets)
	t.state.SetProxyTargets(proxyTargets)

	nc.Log().Info("Yandex Cloud VMs provisioned",
		zap.Int("db", len(dbTargets)),
		zap.Int("proxy", len(proxyTargets)),
	)

	return nil
}

// teardownTask destroys containers and network.
type teardownTask struct {
	provider types.Provider
	state    *State
	deployer *agent.DockerDeployer
	settings *types.ServerSettings
}

func (t *teardownTask) Execute(nc *dag.NodeContext) error {
	switch t.provider {
	case types.ProviderDocker:
		return t.dockerTeardown(nc)
	case types.ProviderYandex:
		return t.yandexTeardown(nc)
	default:
		nc.Log().Warn("teardown: provider not supported, skipping",
			zap.String("provider", string(t.provider)))
		return nil // not an error — unsupported providers just skip teardown
	}
}

func (t *teardownTask) dockerTeardown(nc *dag.NodeContext) error {
	ctx := context.Context(nc)

	// Remove containers.
	for _, cid := range t.state.ContainerIDs() {
		nc.Log().Info("removing container", zap.String("id", cid[:12]))
		if err := t.deployer.Stop(ctx, cid); err != nil {
			nc.Log().Warn("failed to remove container", zap.String("id", cid[:12]), zap.Error(err))
		}
	}

	// Remove network.
	netID := t.state.NetworkID()
	if netID != "" {
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err == nil {
			nc.Log().Info("removing docker network", zap.String("id", netID[:12]))
			cli.NetworkRemove(ctx, netID)
			cli.Close()
		}
	}

	nc.Log().Info("teardown complete")
	return nil
}

func (t *teardownTask) yandexTeardown(nc *dag.NodeContext) error {
	wdIdStr := t.state.TerraformWdId()
	if wdIdStr == "" {
		nc.Log().Info("teardown: no terraform working directory recorded, skipping")
		return nil
	}

	actor := t.state.TerraformActor()
	if actor == nil {
		nc.Log().Warn("teardown: no terraform actor in state, creating new one")
		var err error
		actor, err = terraform.NewActor()
		if err != nil {
			return fmt.Errorf("teardown: create terraform actor: %w", err)
		}
	}

	nc.Log().Info("running terraform destroy for Yandex Cloud", zap.String("wd_id", wdIdStr))

	ctx := context.Context(nc)
	if err := actor.DestroyTerraform(ctx, terraform.NewWdId(wdIdStr)); err != nil {
		return fmt.Errorf("teardown: terraform destroy: %w", err)
	}

	nc.Log().Info("teardown complete: Yandex Cloud resources destroyed")
	return nil
}
