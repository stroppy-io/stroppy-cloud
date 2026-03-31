package agent

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

const (
	// DockerBaseImage is the base image used for agent containers.
	DockerBaseImage = "ubuntu:22.04"
)

// DeployResult holds the result of deploying an agent container.
type DeployResult struct {
	ContainerID   string
	ContainerName string
	MappedPort    int // host-mapped port for the agent
}

// DockerDeployer emulates cloud VMs using Docker containers.
type DockerDeployer struct {
	cli         *client.Client
	networkName string
}

// NewDockerDeployer creates a deployer backed by the local Docker daemon.
func NewDockerDeployer(networkName string) (*DockerDeployer, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("agent: docker client: %w", err)
	}
	return &DockerDeployer{cli: cli, networkName: networkName}, nil
}

// Deploy creates and starts a container running the agent binary.
func (d *DockerDeployer) Deploy(ctx context.Context, machineID string, serverAddr string, agentPort int) (DeployResult, error) {
	binPath := os.Getenv("STROPPY_BINARY_HOST_PATH")
	if binPath == "" {
		var err error
		binPath, err = SelfBinaryPath()
		if err != nil {
			return DeployResult{}, err
		}
	}

	if agentPort == 0 {
		agentPort = DefaultAgentPort
	}

	if err := d.pullIfMissing(ctx, DockerBaseImage); err != nil {
		return DeployResult{}, err
	}

	portStr := fmt.Sprintf("%d/tcp", agentPort)
	cfg := &container.Config{
		Image: DockerBaseImage,
		Cmd:   []string{RemoteBinPath, "agent"},
		Env: []string{
			fmt.Sprintf("STROPPY_SERVER_ADDR=%s", serverAddr),
			fmt.Sprintf("STROPPY_AGENT_PORT=%d", agentPort),
			fmt.Sprintf("STROPPY_MACHINE_ID=%s", machineID),
		},
		ExposedPorts: nat.PortSet{nat.Port(portStr): struct{}{}},
	}

	hostCfg := &container.HostConfig{
		Binds: []string{
			fmt.Sprintf("%s:%s:ro", binPath, RemoteBinPath),
		},
		PortBindings: nat.PortMap{
			nat.Port(portStr): []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: "0"}},
		},
	}

	var netCfg *network.NetworkingConfig
	if d.networkName != "" {
		netCfg = &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				d.networkName: {},
			},
		}
	}

	name := fmt.Sprintf("stroppy-agent-%s", machineID)
	d.cli.ContainerRemove(ctx, name, container.RemoveOptions{Force: true})

	resp, err := d.cli.ContainerCreate(ctx, cfg, hostCfg, netCfg, nil, name)
	if err != nil {
		return DeployResult{}, fmt.Errorf("agent: docker create %s: %w", name, err)
	}

	if err := d.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return DeployResult{}, fmt.Errorf("agent: docker start %s: %w", name, err)
	}

	// Inspect to get mapped port.
	inspect, err := d.cli.ContainerInspect(ctx, resp.ID)
	if err != nil {
		return DeployResult{}, fmt.Errorf("agent: docker inspect %s: %w", name, err)
	}

	mappedPort := 0
	if bindings, ok := inspect.NetworkSettings.Ports[nat.Port(portStr)]; ok && len(bindings) > 0 {
		if p, err := strconv.Atoi(bindings[0].HostPort); err == nil {
			mappedPort = p
		}
	}

	// Get container IP on the shared network for container-to-container communication.
	containerIP := ""
	if d.networkName != "" {
		for netName, ep := range inspect.NetworkSettings.Networks {
			if strings.Contains(netName, d.networkName) || netName == d.networkName {
				containerIP = ep.IPAddress
				break
			}
		}
	}
	_ = containerIP // used by caller if needed

	return DeployResult{
		ContainerID:   resp.ID,
		ContainerName: name,
		MappedPort:    mappedPort,
	}, nil
}

// Stop removes the agent container (force).
func (d *DockerDeployer) Stop(ctx context.Context, containerID string) error {
	return d.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
}

// StopGraceful stops the container with a timeout before removing it.
func (d *DockerDeployer) StopGraceful(ctx context.Context, containerID string, timeoutSec int) error {
	_ = d.cli.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeoutSec})
	return d.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{})
}

func (d *DockerDeployer) pullIfMissing(ctx context.Context, img string) error {
	_, err := d.cli.ImageInspect(ctx, img)
	if err == nil {
		return nil
	}
	reader, err := d.cli.ImagePull(ctx, img, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("agent: docker pull %s: %w", img, err)
	}
	defer reader.Close()
	_, _ = io.Copy(os.Stderr, reader)
	return nil
}

// Close releases the Docker client.
func (d *DockerDeployer) Close() error {
	return d.cli.Close()
}
