package containers

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/registry"
	dockerClient "github.com/docker/docker/client"
	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/samber/lo"
	"github.com/stroppy-io/hatchet-workflow/internal/core/logger"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/deployment"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/provision"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/stroppy"
)

const (
	bridgeDriver = "bridge"

	containerMetadataDockerIPKey      = "docker.network.ipv4"
	containerMetadataPlacementNodeKey = "docker.placement.node"
	containerMetadataLogicalNameKey   = "docker.logical_name"

	containerLabelManagedByKey = "managed_by"
	containerLabelManagedByVal = "hatchet-edge"
	containerLabelRunIDKey     = "run_id"
	containerLabelWorkerIPKey  = "worker_ip"
	containerLabelLogicalKey   = "logical_name"

	dockerConfigDirEnvName = "DOCKER_CONFIG"
	dockerConfigFileName   = "config.json"
)

type containerRun struct {
	networkName string
	networkID   string
	subnet      string
	logger      hatchetLogger
	mu          sync.Mutex
	containers  cmap.ConcurrentMap[string, string] // container name -> container ID
}

var (
	globalMapping  = cmap.New[*containerRun]() // run ID -> containerRun
	client         *dockerClient.Client
	clientInitOnce sync.Once
	clientInitErr  error
)

func cleanupContainerRun(ctx context.Context, run *containerRun) error {
	if run == nil {
		return nil
	}

	cli, err := getDockerClient()
	if err != nil {
		return err
	}

	run.mu.Lock()
	defer run.mu.Unlock()
	cleanupTrackedContainers(ctx, cli, run)
	return nil
}

func Cleanup(ctx context.Context) error {
	var errs []error
	globalMapping.IterCb(func(runID string, run *containerRun) {
		if err := cleanupContainerRun(ctx, run); err != nil {
			errs = append(errs, fmt.Errorf("run %s cleanup failed: %w", runID, err))
		}
		globalMapping.Remove(runID)
	})
	err := errors.Join(errs...)
	if err != nil {
		return err
	}
	err = client.Close()
	if err != nil {
		return err
	}
	return nil
}

func DeployContainersForTarget(
	ctx context.Context,
	taskLogger hatchetLogger,
	runSettings *stroppy.RunSettings,
	networkName string,
	workerInternalIP string,
	workerInternalCidr string,
	containers []*provision.Container,
) error {
	if runSettings == nil {
		return fmt.Errorf("run settings are nil")
	}
	runID := runSettings.GetRunId()
	if runID == "" {
		return fmt.Errorf("run id is empty")
	}

	cli, err := getDockerClient()
	if err != nil {
		return err
	}

	run, err := getOrCreateGlobalRun(runID, networkName, taskLogger)
	if err != nil {
		return err
	}

	run.mu.Lock()
	defer run.mu.Unlock()

	opts, err := runOptionsFromSettings(run, runSettings, workerInternalIP, workerInternalCidr)
	if err != nil {
		return err
	}

	if err := deployContainers(ctx, cli, run, containers, opts); err != nil {
		return err
	}

	// For Docker target, connect the otel-lgtm container to the per-run network
	// so the OTel Collector sidecar can push metrics to it.
	if opts.dockerTarget && run.networkID != "" {
		connectOtelLgtmToNetwork(ctx, cli, run)
	}

	return nil
}

type startedContainer struct {
	name string
	id   string
}

type hatchetLogger interface {
	Log(message string)
}

type runContainerOptions struct {
	dockerTarget       bool
	runID              string
	workerInternal     string
	publishPorts       bool
	primaryContainerID string // when set, share this container's network namespace
}

func runOptionsFromSettings(
	run *containerRun,
	runSettings *stroppy.RunSettings,
	workerInternalIP string,
	workerInternalCidr string,
) (runContainerOptions, error) {
	opts := runContainerOptions{
		publishPorts:   true,
		runID:          runSettings.GetRunId(),
		workerInternal: workerInternalIP,
	}
	if runSettings.GetTarget() == deployment.Target_TARGET_DOCKER {
		run.subnet = workerInternalCidr
		opts = runContainerOptions{
			dockerTarget:   true,
			runID:          runSettings.GetRunId(),
			workerInternal: workerInternalIP,
			publishPorts:   false,
		}
	}
	return opts, nil
}

func deployContainers(
	ctx context.Context,
	cli *dockerClient.Client,
	run *containerRun,
	containers []*provision.Container,
	opts runContainerOptions,
) error {
	started := make([]startedContainer, 0, len(containers))

	for i, c := range containers {
		if c == nil {
			return fmt.Errorf("container spec is nil")
		}

		runOpts := opts
		if i > 0 && opts.dockerTarget && len(started) > 0 {
			runOpts.primaryContainerID = started[0].id
		}

		sc, err := runContainer(
			ctx,
			cli,
			run,
			c,
			runOpts,
		)
		if err != nil {
			rollbackBatch(ctx, cli, run, started)
			return fmt.Errorf("failed to run container %q: %w", c.GetName(), err)
		}

		run.containers.Set(sc.name, sc.id)
		started = append(started, sc)
	}

	return nil
}

func cleanupTrackedContainers(ctx context.Context, cli *dockerClient.Client, run *containerRun) {
	tracked := make(map[string]string)
	run.containers.IterCb(func(name, id string) {
		tracked[name] = id
	})

	for name, id := range tracked {
		if err := stopContainer(ctx, cli, id); err != nil {
			logf(run, "failed to cleanup container %s (%s): %v", name, id, err)
			continue
		}

		run.containers.Remove(name)
	}
}

func runContainer(
	ctx context.Context,
	cli *dockerClient.Client,
	run *containerRun,
	c *provision.Container,
	opts runContainerOptions,
) (startedContainer, error) {
	if err := ensureNetwork(ctx, cli, run); err != nil {
		return startedContainer{}, err
	}

	if err := pullImage(ctx, cli, run, c.GetImage()); err != nil {
		return startedContainer{}, fmt.Errorf("failed to pull image %q: %w", c.GetImage(), err)
	}

	containerCfg := toContainerConfig(c, opts)
	hostCfg, err := toHostConfig(c, opts)
	if err != nil {
		return startedContainer{}, fmt.Errorf("failed to map host config for container %q: %w", c.GetName(), err)
	}
	containerName := containerRuntimeName(c, opts)

	// When sharing a primary container's network namespace, skip network config
	// entirely - Docker rejects EndpointsConfig for container-mode networking.
	var networkCfg *network.NetworkingConfig
	if opts.primaryContainerID == "" {
		networkCfg = toNetworkConfig(run.networkName, run.networkID, c, opts)
	}

	resp, err := cli.ContainerCreate(
		ctx,
		containerCfg,
		hostCfg,
		networkCfg,
		nil,
		containerName,
	)
	if err != nil {
		return startedContainer{}, fmt.Errorf("failed to create container %q: %w", containerName, err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		_ = cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		return startedContainer{}, fmt.Errorf("failed to start container %q: %w", containerName, err)
	}

	return startedContainer{name: containerName, id: resp.ID}, nil
}

func ensureNetwork(ctx context.Context, cli *dockerClient.Client, run *containerRun) error {
	if run.networkID != "" {
		return nil
	}
	networkName := run.networkName
	subnet := run.subnet

	inspect, err := cli.NetworkInspect(ctx, networkName, network.InspectOptions{})
	if err == nil {
		run.networkID = inspect.ID
		return nil
	}

	createOpts := network.CreateOptions{Driver: bridgeDriver}
	if subnet != "" {
		createOpts.IPAM = &network.IPAM{
			Config: []network.IPAMConfig{
				{Subnet: subnet},
			},
		}
	}

	resp, createErr := cli.NetworkCreate(ctx, networkName, createOpts)
	if createErr != nil {
		inspect, inspectErr := cli.NetworkInspect(ctx, networkName, network.InspectOptions{})
		if inspectErr == nil {
			run.networkID = inspect.ID
			return nil
		}
		return fmt.Errorf("failed to create docker network %s: %w", networkName, createErr)
	}

	run.networkID = resp.ID
	return nil
}

func pullImage(ctx context.Context, cli *dockerClient.Client, run *containerRun, imageName string) error {
	pullOptions := image.PullOptions{}
	if registryAuth, ok := registryAuthForImage(imageName); ok {
		pullOptions.RegistryAuth = registryAuth
		logf(run, "docker image pull %q: registry auth enabled", imageName)
	} else {
		logf(run, "docker image pull %q: registry auth not found, using anonymous pull", imageName)
	}

	b := backoff.NewExponentialBackOff()
	b.InitialInterval = 1 * time.Second
	b.MaxInterval = 30 * time.Second
	b.MaxElapsedTime = 5 * time.Minute

	return backoff.Retry(func() error {
		reader, err := cli.ImagePull(ctx, imageName, pullOptions)
		if err != nil {
			logf(run, "docker image pull %q failed: %v", imageName, err)
			return err
		}
		defer reader.Close()

		if err := logDockerStream(run, fmt.Sprintf("docker image pull %q", imageName), reader); err != nil {
			logf(run, "docker image pull %q stream error: %v", imageName, err)
			return err
		}
		return nil
	}, backoff.WithContext(b, ctx))
}

func getDockerClient() (*dockerClient.Client, error) {
	clientInitOnce.Do(func() {
		client, clientInitErr = dockerClient.NewClientWithOpts(
			dockerClient.FromEnv,
			dockerClient.WithAPIVersionNegotiation(),
		)
		if clientInitErr != nil {
			clientInitErr = fmt.Errorf("failed to create Docker client: %w", clientInitErr)
		}
	})
	return client, clientInitErr
}

func getOrCreateGlobalRun(runID, networkName string, taskLogger hatchetLogger) (*containerRun, error) {
	if runID == "" {
		return nil, fmt.Errorf("run id is empty")
	}

	run, ok := globalMapping.Get(runID)
	if !ok {
		run = &containerRun{
			networkName: networkName,
			logger:      taskLogger,
			containers:  cmap.New[string](),
		}
		globalMapping.Set(runID, run)
		return run, nil
	}

	run.mu.Lock()
	defer run.mu.Unlock()

	if run.logger == nil && taskLogger != nil {
		run.logger = taskLogger
	}
	if networkName == "" {
		return run, nil
	}
	if run.networkID != "" && run.networkName != networkName {
		return nil, fmt.Errorf("docker network is already initialized as %q", run.networkName)
	}
	if run.networkName == "" {
		run.networkName = networkName
	}
	return run, nil
}

type dockerConfig struct {
	Auths map[string]dockerAuthEntry `json:"auths"`
}

type dockerAuthEntry struct {
	Auth          string `json:"auth"`
	Username      string `json:"username"`
	Password      string `json:"password"`
	IdentityToken string `json:"identitytoken"`
}

func registryAuthForImage(imageName string) (string, bool) {
	configPath, ok := resolveDockerConfigPath()
	if !ok {
		return "", false
	}

	rawConfig, err := os.ReadFile(configPath)
	if err != nil {
		return "", false
	}

	var cfg dockerConfig
	if err := json.Unmarshal(rawConfig, &cfg); err != nil {
		return "", false
	}

	entry, ok := findAuthEntry(cfg.Auths, imageName)
	if !ok {
		return "", false
	}

	authCfg, ok := toRegistryAuthConfig(entry)
	if !ok {
		return "", false
	}

	encoded, err := encodeAuthConfig(authCfg)
	if err != nil {
		return "", false
	}
	return encoded, true
}

func resolveDockerConfigPath() (string, bool) {
	configDir := os.Getenv(dockerConfigDirEnvName)
	if configDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", false
		}
		configDir = filepath.Join(homeDir, ".docker")
	}

	configPath := filepath.Join(configDir, dockerConfigFileName)
	if _, err := os.Stat(configPath); err != nil {
		return "", false
	}
	return configPath, true
}

func findAuthEntry(auths map[string]dockerAuthEntry, imageName string) (dockerAuthEntry, bool) {
	if len(auths) == 0 {
		return dockerAuthEntry{}, false
	}

	registryHost := normalizeRegistryHost(registryHostForImage(imageName))
	for key, entry := range auths {
		if normalizeRegistryHost(key) == registryHost {
			return entry, true
		}
	}

	if registryHost == "docker.io" {
		if entry, ok := auths["https://index.docker.io/v1/"]; ok {
			return entry, true
		}
	}

	return dockerAuthEntry{}, false
}

func registryHostForImage(imageName string) string {
	parts := strings.Split(imageName, "/")
	if len(parts) == 0 {
		return "docker.io"
	}

	first := parts[0]
	if !strings.Contains(first, ".") && !strings.Contains(first, ":") && first != "localhost" {
		return "docker.io"
	}
	return first
}

func normalizeRegistryHost(host string) string {
	normalized := strings.TrimSpace(strings.ToLower(host))
	normalized = strings.TrimPrefix(normalized, "https://")
	normalized = strings.TrimPrefix(normalized, "http://")
	normalized = strings.TrimSuffix(normalized, "/")
	if idx := strings.IndexByte(normalized, '/'); idx >= 0 {
		normalized = normalized[:idx]
	}
	if normalized == "index.docker.io" || normalized == "registry-1.docker.io" {
		return "docker.io"
	}
	return normalized
}

func toRegistryAuthConfig(entry dockerAuthEntry) (registry.AuthConfig, bool) {
	authCfg := registry.AuthConfig{
		Username:      entry.Username,
		Password:      entry.Password,
		IdentityToken: entry.IdentityToken,
	}
	if authCfg.Username != "" || authCfg.Password != "" || authCfg.IdentityToken != "" {
		return authCfg, true
	}

	if entry.Auth == "" {
		return registry.AuthConfig{}, false
	}

	decoded, err := base64.StdEncoding.DecodeString(entry.Auth)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(entry.Auth)
		if err != nil {
			return registry.AuthConfig{}, false
		}
	}
	credentials := strings.SplitN(string(decoded), ":", 2)
	if len(credentials) != 2 {
		return registry.AuthConfig{}, false
	}
	return registry.AuthConfig{
		Username: credentials[0],
		Password: credentials[1],
	}, true
}

func encodeAuthConfig(cfg registry.AuthConfig) (string, error) {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(raw), nil
}

type dockerStreamMessage struct {
	Error       string `json:"error"`
	ErrorDetail struct {
		Message string `json:"message"`
	} `json:"errorDetail"`
}

func logDockerStream(run *containerRun, prefix string, stream io.Reader) error {
	scanner := bufio.NewScanner(stream)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		logf(run, "%s: %s", prefix, line)

		var msg dockerStreamMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		if msg.ErrorDetail.Message != "" {
			return fmt.Errorf("%s", msg.ErrorDetail.Message)
		}
		if msg.Error != "" {
			return fmt.Errorf("%s", msg.Error)
		}
	}
	return scanner.Err()
}

func stopContainer(ctx context.Context, cli *dockerClient.Client, containerID string) error {
	stopErr := cli.ContainerStop(ctx, containerID, container.StopOptions{Timeout: lo.ToPtr(30)})
	removeErr := cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})

	if stopErr != nil && removeErr != nil {
		return fmt.Errorf(
			"failed to stop container %s: %w; failed to remove container %s: %v",
			containerID,
			stopErr,
			containerID,
			removeErr,
		)
	}
	if stopErr != nil {
		return fmt.Errorf("failed to stop container %s: %w", containerID, stopErr)
	}
	if removeErr != nil {
		return fmt.Errorf("failed to remove container %s: %w", containerID, removeErr)
	}
	return nil
}

func rollbackBatch(ctx context.Context, cli *dockerClient.Client, run *containerRun, started []startedContainer) {
	for i := len(started) - 1; i >= 0; i-- {
		name := started[i].name
		containerID := started[i].id

		if err := stopContainer(ctx, cli, containerID); err != nil {
			logf(run, "failed to rollback container %s (%s): %v", name, containerID, err)
			continue
		}

		run.containers.Remove(name)
	}
}

func logf(run *containerRun, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if run != nil && run.logger != nil {
		run.logger.Log(msg)
	}
	logger.StdLog().Printf(format, args...)
}

// RemoveNetwork removes a Docker network by name, disconnecting any containers first.
// Best-effort: if the network doesn't exist, returns nil.
func RemoveNetwork(ctx context.Context, networkName string) error {
	cli, err := getDockerClient()
	if err != nil {
		return err
	}

	inspect, err := cli.NetworkInspect(ctx, networkName, network.InspectOptions{})
	if err != nil {
		// Network doesn't exist — nothing to do.
		return nil
	}

	for id := range inspect.Containers {
		_ = cli.NetworkDisconnect(ctx, inspect.ID, id, true)
	}

	return cli.NetworkRemove(ctx, inspect.ID)
}

const otelLgtmContainerName = "otel-lgtm"

func connectOtelLgtmToNetwork(ctx context.Context, cli *dockerClient.Client, run *containerRun) {
	err := cli.NetworkConnect(ctx, run.networkID, otelLgtmContainerName, nil)
	if err != nil {
		// Not fatal — otel-lgtm may not be running or already connected
		logf(run, "otel-lgtm network connect (non-fatal): %v", err)
	} else {
		logf(run, "connected %s to network %s", otelLgtmContainerName, run.networkName)
	}
}

func setNetworkName(run *containerRun, networkName string) error {
	if networkName == "" {
		return nil
	}

	if run.networkID != "" && run.networkName != networkName {
		return fmt.Errorf("docker network is already initialized as %q", run.networkName)
	}
	run.networkName = networkName
	return nil
}

func containerRuntimeName(c *provision.Container, opts runContainerOptions) string {
	base := containerLogicalName(c)
	if !opts.dockerTarget {
		return base
	}

	parts := []string{SanitizeDockerNamePart(opts.runID), SanitizeDockerNamePart(opts.workerInternal), SanitizeDockerNamePart(base)}
	return strings.Trim(strings.Join(parts, "-"), "-")
}

func containerLogicalName(c *provision.Container) string {
	if c.GetName() != "" {
		return c.GetName()
	}
	if c.GetId() != "" {
		return c.GetId()
	}
	return "container"
}

func SanitizeDockerNamePart(s string) string {
	if s == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch >= 'a' && ch <= 'z':
			b.WriteByte(ch)
		case ch >= 'A' && ch <= 'Z':
			b.WriteByte(ch + ('a' - 'A'))
		case ch >= '0' && ch <= '9':
			b.WriteByte(ch)
		case ch == '.' || ch == '-' || ch == '_':
			b.WriteByte(ch)
		default:
			b.WriteByte('-')
		}
	}

	out := strings.Trim(b.String(), "-._")
	if out == "" {
		return "c"
	}
	return out
}
