package activities

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
	dockerclient "github.com/docker/docker/client"
	"go.temporal.io/sdk/activity"

	"github.com/graphene-ci/pipeline/pkg/obs"
	"github.com/graphene-ci/pipeline/pkg/workerapi"
)

// Wire names of the docker-side bodies.
const (
	NamePullImage     = "stroppy.image.pull"
	NameWaitHealthy   = "stroppy.container.wait-healthy"
	NameWriteFiles    = "stroppy.files.write"
	healthPollMinimum = time.Second
)

// PullImageRequest pre-pulls an image on the machine, with registry
// credentials when the image is private.
type PullImageRequest struct {
	Image string `json:"image"`
	// RegistrySecret names a Graphene secret holding a
	// provider.registry.credentials value; empty for public images.
	RegistrySecret string `json:"registry_secret,omitempty"`
}

// PullImageResult names what was pulled.
type PullImageResult struct {
	Image string `json:"image"`
}

// PullImage pulls the image through the machine's docker daemon. Runs on
// the AGENT container. Idempotent: an image already present is a no-op for
// the daemon. Credentials resolve here, at the point of use, and never
// enter workflow history.
func PullImage(ctx context.Context, req PullImageRequest) (PullImageResult, error) {
	cli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		return PullImageResult{}, err
	}
	defer func() { _ = cli.Close() }()
	opts := image.PullOptions{}
	if req.RegistrySecret != "" {
		auth, err := registryAuth(ctx, req.RegistrySecret)
		if err != nil {
			return PullImageResult{}, err
		}
		opts.RegistryAuth = auth
	}
	rc, err := cli.ImagePull(ctx, req.Image, opts)
	if err != nil {
		return PullImageResult{}, fmt.Errorf("pull %s: %w", req.Image, err)
	}
	defer func() { _ = rc.Close() }()
	if err := readPullProgress(ctx, rc); err != nil {
		return PullImageResult{}, fmt.Errorf("pull %s: %w", req.Image, err)
	}
	obs.Info(ctx, "image pulled", obs.Str("image", req.Image))
	return PullImageResult{Image: req.Image}, nil
}

// Docker can return HTTP 200 and report a failed pull in the JSON stream.
// Decode messages instead of discarding their bytes so callers never create
// a container from an image whose download or extraction failed.
func readPullProgress(ctx context.Context, r io.Reader) error {
	dec := json.NewDecoder(r)
	for {
		var message struct {
			Error       string `json:"error"`
			ErrorDetail struct {
				Message string `json:"message"`
			} `json:"errorDetail"`
		}
		if err := dec.Decode(&message); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("decode docker progress: %w", err)
		}
		if message.ErrorDetail.Message != "" {
			return errors.New(message.ErrorDetail.Message)
		}
		if message.Error != "" {
			return errors.New(message.Error)
		}
		if activity.IsActivity(ctx) {
			activity.RecordHeartbeat(ctx, "pulling image")
		}
	}
}

// registryAuth turns a provider.registry.credentials value into the
// base64 auth header docker expects.
func registryAuth(ctx context.Context, secret string) (string, error) {
	raw, err := workerapi.GetSecret(ctx, secret)
	if err != nil {
		return "", err
	}
	var c struct {
		Registry string `json:"registry"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return "", fmt.Errorf("registry credentials: %w", err)
	}
	b, err := json.Marshal(registry.AuthConfig{Username: c.Username, Password: c.Password, ServerAddress: c.Registry}) //nolint:gosec // this IS the docker registry auth header; it never leaves the daemon call
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// WaitHealthyRequest polls a container until its health command succeeds.
type WaitHealthyRequest struct {
	Container string        `json:"container"`
	Cmd       []string      `json:"cmd"`
	Interval  time.Duration `json:"interval"`
	Retries   int           `json:"retries"`
}

// WaitHealthyResult tells how long it took.
type WaitHealthyResult struct {
	Attempts int           `json:"attempts"`
	Elapsed  time.Duration `json:"elapsed"`
}

// WaitHealthy runs the health command inside the container (docker exec)
// until it exits 0, at most Retries times, Interval apart. A container that
// stopped running fails immediately with the daemon's reason — no point
// waiting for a dead process. Heartbeats every attempt.
func WaitHealthy(ctx context.Context, req WaitHealthyRequest) (WaitHealthyResult, error) {
	if len(req.Cmd) == 0 {
		return WaitHealthyResult{}, fmt.Errorf("wait healthy %s: empty command", req.Container)
	}
	interval := req.Interval
	if interval < healthPollMinimum {
		interval = healthPollMinimum
	}
	retries := req.Retries
	if retries <= 0 {
		retries = 30
	}
	cli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		return WaitHealthyResult{}, err
	}
	defer func() { _ = cli.Close() }()
	start := time.Now()
	var lastOut string
	for attempt := 1; attempt <= retries; attempt++ {
		insp, err := cli.ContainerInspect(ctx, req.Container)
		if err != nil {
			return WaitHealthyResult{}, fmt.Errorf("inspect %s: %w", req.Container, err)
		}
		if insp.State == nil || !insp.State.Running {
			reason := ""
			if insp.State != nil {
				reason = fmt.Sprintf("status=%s exit=%d error=%q", insp.State.Status, insp.State.ExitCode, insp.State.Error)
			}
			return WaitHealthyResult{}, fmt.Errorf("container %s is not running (%s)", req.Container, reason)
		}
		var shell []string
		if insp.Config != nil {
			shell = insp.Config.Shell
		}
		command, err := healthCommand(req.Cmd, shell)
		if err != nil {
			return WaitHealthyResult{}, fmt.Errorf("healthcheck %s: %w", req.Container, err)
		}
		code, out, err := execInContainer(ctx, cli, insp.ID, command)
		if err == nil && code == 0 {
			obs.Info(ctx, "container healthy", obs.Str("container", req.Container), obs.Int("attempts", attempt))
			return WaitHealthyResult{Attempts: attempt, Elapsed: time.Since(start)}, nil
		}
		lastOut = strings.TrimSpace(out)
		if err != nil {
			lastOut = err.Error() + ": " + lastOut
		}
		if activity.IsActivity(ctx) {
			activity.RecordHeartbeat(ctx, fmt.Sprintf("%s: attempt %d/%d", req.Container, attempt, retries))
		}
		select {
		case <-ctx.Done():
			return WaitHealthyResult{}, ctx.Err()
		case <-time.After(interval):
		}
	}
	return WaitHealthyResult{}, fmt.Errorf("container %s not healthy after %d attempts: %s", req.Container, retries, tail(lastOut, errTailBytes))
}

// execInContainer runs cmd inside the container and returns its exit code
// and combined output.
func execInContainer(ctx context.Context, cli *dockerclient.Client, id string, cmd []string) (exitCode int, output string, err error) {
	exec, err := cli.ContainerExecCreate(ctx, id, container.ExecOptions{Cmd: cmd, AttachStdout: true, AttachStderr: true})
	if err != nil {
		return -1, "", err
	}
	attach, err := cli.ContainerExecAttach(ctx, exec.ID, container.ExecStartOptions{})
	if err != nil {
		return -1, "", err
	}
	defer attach.Close()
	out, err := io.ReadAll(attach.Reader)
	if err != nil {
		return -1, string(out), err
	}
	insp, err := cli.ContainerExecInspect(ctx, exec.ID)
	if err != nil {
		return -1, string(out), err
	}
	return insp.ExitCode, string(out), nil
}

// WriteFilesRequest writes rendered configs into the run workspace of the
// machine so they can be bind-mounted into containers.
type WriteFilesRequest struct {
	// Dir is the directory to write under: absolute, or relative to the
	// run workspace of the agent.
	Dir   string          `json:"dir"`
	Files []WorkspaceFile `json:"files"`
}

// WorkspaceFile is one file: a path relative to Dir, content and mode.
type WorkspaceFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Mode    string `json:"mode,omitempty"`
}

// WriteFilesResult lists the absolute paths written.
type WriteFilesResult struct {
	Paths []string `json:"paths"`
}

func tail(s string, n int) string { //nolint:unparam // one tail size today; the knob stays
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n:]
}
