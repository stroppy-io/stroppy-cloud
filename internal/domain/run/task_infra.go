package run

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"go.uber.org/zap"

	"github.com/stroppy-io/stroppy-cloud/internal/core/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/terraform"
)

// isHostMode returns true when the server runs on the host (not inside Docker).
// In that case we use localhost + mapped ports to reach agent containers.
func isHostMode() bool {
	// Inside a Docker container /.dockerenv exists.
	_, err := os.Stat("/.dockerenv")
	return err != nil // not in docker -> host mode
}

// networkTask creates a Docker network (for docker provider).
type networkTask struct {
	cfg      types.NetworkConfig
	provider types.Provider
	deployer *agent.DockerDeployer
	state    *State
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

	netName := "stroppy-run-net"
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
		return fmt.Errorf("machine provisioning not implemented for provider %q", t.runCfg.Provider)
	}
}

func (t *machinesTask) dockerMachines(nc *dag.NodeContext) error {
	if t.deployer == nil {
		return fmt.Errorf("machines: DockerDeployer is nil")
	}

	ctx := context.Context(nc)
	var dbTargets []agent.Target
	var monitorTargets []agent.Target
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

			// Use localhost + mapped port when running outside Docker (tests).
			// Use container name when running inside Docker (compose).
			host := result.ContainerName
			agentPort := port
			if result.MappedPort > 0 && isHostMode() {
				host = "localhost"
				agentPort = result.MappedPort
			}

			target := agent.Target{
				ID:           machineID,
				Host:         host,
				InternalHost: result.ContainerName, // for container-to-container
				AgentPort:    agentPort,
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
			case types.RoleMonitor:
				monitorTargets = append(monitorTargets, target)
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
	t.state.SetMonitorTargets(monitorTargets)
	t.state.SetProxyTargets(proxyTargets)

	nc.Log().Info("all machines provisioned",
		zap.Int("db", len(dbTargets)),
		zap.Int("monitor", len(monitorTargets)),
	)
	return nil
}

// yandexTfVars holds terraform variable values for Yandex Cloud VM provisioning.
type yandexTfVars struct {
	FolderID         string       `json:"folder_id"`
	Zone             string       `json:"zone"`
	SubnetID         string       `json:"subnet_id"`
	ServiceAccountID string       `json:"service_account_id"`
	SSHPublicKey     string       `json:"ssh_public_key"`
	ImageID          string       `json:"image_id"`
	Machines         []yandexTfVM `json:"machines"`
}

type yandexTfVM struct {
	Name      string `json:"name"`
	Role      string `json:"role"`
	Cores     int    `json:"cores"`
	MemoryMB  int    `json:"memory_mb"`
	DiskGB    int    `json:"disk_gb"`
	CloudInit string `json:"cloud_init"`
}

// terraformTemplatesDir is the directory containing .tf files for Yandex Cloud.
const terraformTemplatesDir = "/etc/stroppy/terraform/yandex"

func (t *machinesTask) yandexMachines(nc *dag.NodeContext) error {
	if t.settings == nil {
		return fmt.Errorf("machines: server settings not configured for Yandex Cloud provider")
	}

	cloud := t.settings.Cloud
	yc := cloud.Yandex

	// Determine binary URL for cloud-init.
	binaryURL := cloud.BinaryURL
	if binaryURL == "" && t.serverAddr != "" {
		binaryURL = t.serverAddr + "/agent/binary"
	}
	if binaryURL == "" {
		return fmt.Errorf("machines: binary_url or server_addr must be configured for cloud provider")
	}

	// Check that terraform template files exist.
	tfDir := terraformTemplatesDir
	if _, err := os.Stat(tfDir); os.IsNotExist(err) {
		return fmt.Errorf("terraform templates not configured -- set up templates via admin API (expected at %s)", tfDir)
	}

	// Read all .tf files from the templates directory.
	tfFiles, err := loadTfFiles(tfDir)
	if err != nil {
		return fmt.Errorf("machines: load terraform templates: %w", err)
	}
	if len(tfFiles) == 0 {
		return fmt.Errorf("terraform templates not configured -- no .tf files found in %s", tfDir)
	}

	// Build cloud-init and VM specs for each machine.
	var vms []yandexTfVM
	for _, spec := range t.runCfg.Machines {
		for i := range spec.Count {
			machineID := fmt.Sprintf("%s-%s-%d", t.runCfg.ID, spec.Role, i)

			cloudInit, ciErr := agent.GenerateCloudInit(agent.CloudInitParams{
				BinaryURL:  binaryURL,
				ServerAddr: t.serverAddr,
				AgentPort:  agent.DefaultAgentPort,
				MachineID:  machineID,
			})
			if ciErr != nil {
				return fmt.Errorf("machines: generate cloud-init for %s: %w", machineID, ciErr)
			}

			cores := spec.CPUs
			if cores == 0 {
				cores = 2
			}
			memMB := spec.MemoryMB
			if memMB == 0 {
				memMB = 4096
			}
			diskGB := spec.DiskGB
			if diskGB == 0 {
				diskGB = 50
			}

			nc.Log().Info("preparing VM",
				zap.String("machine_id", machineID),
				zap.String("role", string(spec.Role)),
				zap.Int("cores", cores),
				zap.Int("memory_mb", memMB),
				zap.Int("disk_gb", diskGB),
			)

			vms = append(vms, yandexTfVM{
				Name:      machineID,
				Role:      string(spec.Role),
				Cores:     cores,
				MemoryMB:  memMB,
				DiskGB:    diskGB,
				CloudInit: cloudInit,
			})
		}
	}

	// Build terraform variables.
	vars := yandexTfVars{
		FolderID:         yc.FolderID,
		Zone:             yc.Zone,
		SubnetID:         yc.SubnetID,
		ServiceAccountID: yc.ServiceAccountID,
		SSHPublicKey:     yc.SSHPublicKey,
		ImageID:          yc.ImageID,
		Machines:         vms,
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
	)

	nc.Log().Info("running terraform apply for Yandex Cloud",
		zap.String("run_id", t.runCfg.ID),
		zap.Int("vm_count", len(vms)),
	)

	ctx := context.Context(nc)
	output, err := actor.ApplyTerraform(ctx, wd)
	if err != nil {
		return fmt.Errorf("machines: terraform apply: %w", err)
	}

	// Store working directory ID for teardown.
	t.state.SetTerraformWdId(string(wdId))

	// Parse terraform output to get VM IPs.
	// Expected output key: "vm_ips" -- a map of machine name to IP address.
	vmIPs, err := terraform.GetTfOutputVal[map[string]string](output, "vm_ips")
	if err != nil {
		return fmt.Errorf("machines: parse terraform output 'vm_ips': %w", err)
	}

	// Populate state with targets from terraform output.
	var dbTargets []agent.Target
	var monitorTargets []agent.Target
	var proxyTargets []agent.Target

	for _, vm := range vms {
		ip, ok := vmIPs[vm.Name]
		if !ok {
			return fmt.Errorf("machines: terraform output missing IP for VM %q", vm.Name)
		}

		target := agent.Target{
			ID:        vm.Name,
			Host:      ip,
			AgentPort: agent.DefaultAgentPort,
		}

		role := types.MachineRole(vm.Role)
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
				t.state.SetDBEndpoint(ip, dbPort)
			}
		case types.RoleMonitor:
			monitorTargets = append(monitorTargets, target)
		case types.RoleProxy:
			proxyTargets = append(proxyTargets, target)
		case types.RoleStroppy:
			t.state.SetStroppyTarget(target)
		}
	}

	t.state.SetDBTargets(dbTargets)
	t.state.SetMonitorTargets(monitorTargets)
	t.state.SetProxyTargets(proxyTargets)

	nc.Log().Info("Yandex Cloud VMs provisioned, waiting for agents to register",
		zap.Int("db", len(dbTargets)),
		zap.Int("monitor", len(monitorTargets)),
		zap.Int("proxy", len(proxyTargets)),
	)

	// Wait for all agents to become healthy.
	if err := waitForAgents(ctx, nc.Log(), vms, vmIPs); err != nil {
		return fmt.Errorf("machines: %w", err)
	}

	nc.Log().Info("all Yandex Cloud agents healthy")
	return nil
}

// loadTfFiles reads all .tf files from a directory and returns them as terraform TfFile slices.
func loadTfFiles(dir string) ([]terraform.TfFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []terraform.TfFile
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if filepath.Ext(e.Name()) != ".tf" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", e.Name(), err)
		}
		files = append(files, terraform.NewTfFile(data, e.Name()))
	}
	return files, nil
}

// waitForAgents polls the /health endpoint on each agent until all respond or timeout.
func waitForAgents(ctx context.Context, log *zap.Logger, vms []yandexTfVM, ips map[string]string) error {
	const (
		pollInterval = 5 * time.Second
		timeout      = 5 * time.Minute
	)

	deadline := time.Now().Add(timeout)
	httpClient := &http.Client{Timeout: 3 * time.Second}

	healthy := make(map[string]bool, len(vms))
	for {
		allHealthy := true
		for _, vm := range vms {
			if healthy[vm.Name] {
				continue
			}
			ip := ips[vm.Name]
			healthURL := fmt.Sprintf("http://%s:%d/health", ip, agent.DefaultAgentPort)
			resp, err := httpClient.Get(healthURL) //nolint:gosec // URL built from trusted terraform output
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					healthy[vm.Name] = true
					log.Info("agent healthy", zap.String("vm", vm.Name), zap.String("ip", ip))
					continue
				}
			}
			allHealthy = false
		}

		if allHealthy {
			return nil
		}

		if time.Now().After(deadline) {
			var unhealthy []string
			for _, vm := range vms {
				if !healthy[vm.Name] {
					unhealthy = append(unhealthy, vm.Name)
				}
			}
			return fmt.Errorf("agents did not become healthy within %v: %v", timeout, unhealthy)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
		}
	}
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
		nc.Log().Info("teardown: not implemented for provider", zap.String("provider", string(t.provider)))
		return nil
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

	nc.Log().Info("running terraform destroy for Yandex Cloud", zap.String("wd_id", wdIdStr))

	actor, err := terraform.NewActor()
	if err != nil {
		return fmt.Errorf("teardown: create terraform actor: %w", err)
	}

	ctx := context.Context(nc)
	if err := actor.DestroyTerraform(ctx, terraform.NewWdId(wdIdStr)); err != nil {
		return fmt.Errorf("teardown: terraform destroy: %w", err)
	}

	nc.Log().Info("teardown complete: Yandex Cloud resources destroyed")
	return nil
}
