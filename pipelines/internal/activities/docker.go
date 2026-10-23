package activities

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"

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
	stopHeartbeat := pullHeartbeat(ctx)
	defer stopHeartbeat()
	cli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		return PullImageResult{}, err
	}
	defer func() { _ = cli.Close() }()
	// A digest is immutable. Readiness and the auth-version probe share this
	// image with the workload and must not depend on the registry again.
	if strings.Contains(req.Image, "@sha256:") {
		if _, err := cli.ImageInspect(ctx, req.Image); err == nil {
			return PullImageResult{Image: req.Image}, nil
		} else if !cerrdefs.IsNotFound(err) {
			return PullImageResult{}, fmt.Errorf("inspect pinned image %s: %w", req.Image, err)
		}
	}
	opts := image.PullOptions{}
	if req.RegistrySecret != "" {
		auth, err := registryAuth(ctx, req.RegistrySecret)
		if err != nil {
			return PullImageResult{}, err
		}
		opts.RegistryAuth = auth
	}
	for attempt := 1; attempt <= pullAttempts; attempt++ {
		rc, pullErr := cli.ImagePull(ctx, req.Image, opts)
		if pullErr == nil {
			pullErr = readPullProgress(rc)
			_ = rc.Close()
		}
		if pullErr == nil {
			obs.Info(ctx, "image pulled", obs.Str("image", req.Image))
			return PullImageResult{Image: req.Image}, nil
		}
		if ctx.Err() != nil {
			return PullImageResult{}, fmt.Errorf("pull %s: %w", req.Image, pullErr)
		}
		if attempt == pullAttempts || !transientRegistryError(pullErr) {
			// A missing image or a registry that keeps failing does not heal
			// by retrying the activity forever: the run fails with the reason.
			return PullImageResult{}, permanent(fmt.Errorf("pull %s: %w", req.Image, pullErr))
		}
		obs.Warn(ctx, "transient registry error; retrying image pull", obs.Str("image", req.Image))
		timer := time.NewTimer(time.Duration(attempt) * pullBackoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return PullImageResult{}, ctx.Err()
		case <-timer.C:
		}
	}
	panic("unreachable image pull retry loop")
}

// pullAttempts bounds the retries of one image pull: a registry mirror
// under the load of a whole matrix answers late, not never.
const pullAttempts = 5

// pullBackoff grows linearly per failed attempt (5s, 10s, ...); tests shorten it.
var pullBackoff = 5 * time.Second

func transientRegistryError(err error) bool {
	message := strings.ToLower(err.Error())
	for _, marker := range []string{"i/o timeout", "tls handshake timeout", "timeout awaiting response headers", "connection reset by peer", "connection refused", "unexpected eof", "temporary failure", "service unavailable", "bad gateway", "gateway timeout", "too many requests"} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

// Docker may be silent during registry authentication, layer download or
// extraction. Keep the activity alive independently of its progress stream,
// including the initial ImagePull call. The activity's overall deadline still
// bounds the operation; stopping waits for the heartbeat goroutine to exit.
func pullHeartbeat(ctx context.Context) func() {
	return operationHeartbeat(ctx, "pulling image")
}

// How a long operation reports that it is alive: a heartbeat at the start,
// then one every heartbeatEvery until it stops. Tests replace them to watch
// the beats without Temporal's 30-second delivery batching.
var (
	heartbeatEvery = 15 * time.Second
	heartbeatsOn   = activity.IsActivity
	heartbeat      = recordHeartbeat
)

func operationHeartbeat(ctx context.Context, message string) func() {
	if !heartbeatsOn(ctx) {
		return func() {}
	}
	heartbeat(ctx, message)
	stop, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(heartbeatEvery)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-stop:
				return
			case <-ticker.C:
				heartbeat(ctx, message)
			}
		}
	}()
	return func() { close(stop); <-done }
}

// Docker can return HTTP 200 and report a failed pull in the JSON stream.
// Decode messages instead of discarding their bytes so callers never create
// a container from an image whose download or extraction failed.
func readPullProgress(r io.Reader) error {
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
		if insp.State == nil || !insp.State.Running || insp.State.Restarting {
			reason := ""
			if insp.State != nil {
				reason = fmt.Sprintf("status=%s exit=%d restarts=%d error=%q", insp.State.Status, insp.State.ExitCode, insp.RestartCount, insp.State.Error)
			}
			// A dead or crash-looping process is not cured by waiting: fail
			// the run with what the container last said.
			return WaitHealthyResult{}, permanent(fmt.Errorf("container %s is not running (%s): %s", req.Container, reason, containerLogTail(ctx, cli, insp.ID)))
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
			recordHeartbeat(ctx, fmt.Sprintf("%s: attempt %d/%d", req.Container, attempt, retries))
		}
		select {
		case <-ctx.Done():
			return WaitHealthyResult{}, ctx.Err()
		case <-time.After(interval):
		}
	}
	return WaitHealthyResult{}, permanent(fmt.Errorf("container %s not healthy after %d attempts: %s; container log: %s",
		req.Container, retries, tail(lastOut, errTailBytes), containerLogTail(ctx, cli, req.Container)))
}

// containerLogLines is how much of a failing container's own output travels
// back in the error.
const containerLogLines = "20"

// containerLogTail returns the last lines the container wrote, or a note why
// they could not be read.
func containerLogTail(ctx context.Context, cli *dockerclient.Client, id string) string {
	rc, err := cli.ContainerLogs(ctx, id, container.LogsOptions{ShowStdout: true, ShowStderr: true, Tail: containerLogLines})
	if err != nil {
		return "unavailable: " + err.Error()
	}
	defer func() { _ = rc.Close() }()
	raw, err := io.ReadAll(io.LimitReader(rc, errTailBytes*4))
	if err != nil {
		return "unavailable: " + err.Error()
	}
	// Non-TTY containers multiplex stdout and stderr; a TTY one is raw.
	var out bytes.Buffer
	if _, err := stdcopy.StdCopy(&out, &out, bytes.NewReader(raw)); err != nil {
		out.Reset()
		out.Write(raw)
	}
	return tail(strings.TrimSpace(out.String()), errTailBytes)
}

// permanent marks an error the activity must not be retried for: the
// default policy retries forever, which hides the reason from the run.
func permanent(err error) error {
	return temporal.NewNonRetryableApplicationError(err.Error(), "permanent", err)
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
