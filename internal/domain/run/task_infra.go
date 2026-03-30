package run

import (
	"context"
	"fmt"
	"os"

	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"go.uber.org/zap"

	"github.com/stroppy-io/hatchet-workflow/internal/core/dag"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/agent"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/types"
)

// isHostMode returns true when the server runs on the host (not inside Docker).
// In that case we use localhost + mapped ports to reach agent containers.
func isHostMode() bool {
	// Inside a Docker container /.dockerenv exists.
	_, err := os.Stat("/.dockerenv")
	return err != nil // not in docker → host mode
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
	default:
		// Cloud providers use terraform for networking.
		nc.Log().Info("network phase: skipping (handled by terraform)", zap.String("provider", string(t.provider)))
		return nil
	}
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
}

func (t *machinesTask) Execute(nc *dag.NodeContext) error {
	switch t.runCfg.Provider {
	case types.ProviderDocker:
		return t.dockerMachines(nc)
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
					// First DB target is master — store for stroppy to connect.
					port := 5432 // postgres default
					switch t.runCfg.Database.Kind {
					case types.DatabaseMySQL:
						port = 3306
					case types.DatabasePicodata:
						port = 4327 // pgproto
					}
					t.state.SetDBEndpoint(result.ContainerName, port)
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

// teardownTask destroys containers and network.
type teardownTask struct {
	provider types.Provider
	state    *State
	deployer *agent.DockerDeployer
}

func (t *teardownTask) Execute(nc *dag.NodeContext) error {
	switch t.provider {
	case types.ProviderDocker:
		return t.dockerTeardown(nc)
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
