package activities

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"go.temporal.io/sdk/activity"

	"github.com/graphene-ci/pipeline/pkg/machine"
	"github.com/graphene-ci/pipeline/pkg/obs"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
	"github.com/stroppy-io/stroppy-cloud/pipelines/stroppycfg"
)

// Wire names of the stroppy activities.
const (
	NameRunSegment  = "stroppy.segment.run"
	NameRunBaseline = "stroppy.baseline.run"
)

const (
	// stdoutCaptureLimit bounds the in-memory copy of stdout the parsers
	// read (the compliance JSON line, the baseline report); the full output
	// always goes to the log file.
	stdoutCaptureLimit = 8 << 20
	// errTailLines is how much of the log a failure message quotes.
	errTailLines = 20
)

// RunSegmentRequest runs one workload segment on the runner machine.
type RunSegmentRequest struct {
	RunID string `json:"run_id"`
	// Segment is the decoded workload.segment value.
	Segment spec.Segment `json:"segment"`
	// Index orders the segment inside the workload (directory name).
	Index int `json:"index"`
	// Workload is the run's workload: image, driver type and options, CA.
	Workload spec.Workload `json:"workload"`
	// URL is the connection URL with address placeholders expanded.
	URL string `json:"url"`
	// OTLPEndpoint receives stroppy metrics; empty disables the exporter.
	OTLPEndpoint string `json:"otlp_endpoint,omitempty"`
	// OTLPHeaders is the comma-separated key=value list stroppy sends.
	OTLPHeaders string `json:"otlp_headers,omitempty"`
	// Labels are stamped on the run's metrics as stroppy metadata.
	Labels map[string]string `json:"labels,omitempty"`
	// RegistrySecret names the registry login for a private stroppy image.
	RegistrySecret string `json:"registry_secret,omitempty"`
}

// RunSegmentResult is the segment's outcome plus where its artifacts are.
type RunSegmentResult struct {
	Result spec.SegmentResult `json:"result"`
	// ConfigPath is the rendered stroppy-config.json on the machine.
	ConfigPath string `json:"config_path,omitempty"`
	// LogPath is the full stroppy output on the machine.
	LogPath string `json:"log_path,omitempty"`
}

// RunSegment runs ON THE RUNNER MACHINE's agent container: writes the
// segment's config and files into the run workspace, starts stroppy 6 as a
// container with the workspace mounted, streams its output to obs line by
// line, waits for exit and reads the bench summary back. One-shot by
// nature (AtMostOnce at the call site): a second execution would load data
// twice.
//
// doc: stroppy Dockerfile — ENTRYPOINT stroppy, WORKDIR /workspace;
// README "Docker Usage" — --network host to reach the databases.
func RunSegment(ctx context.Context, req RunSegmentRequest) (RunSegmentResult, error) {
	seg := req.Segment
	dir, err := workspaceSubdir("stroppy", segmentSlug(req.Index, seg.Name))
	if err != nil {
		return RunSegmentResult{}, err
	}
	cfgPath, err := writeSegmentInputs(dir, req)
	if err != nil {
		return RunSegmentResult{}, err
	}
	args, cleanup, err := ydbRuntimeArgs(ctx, cfgPath, req.Workload.YDBIAMCredentialsSecret, stroppycfg.Args(seg))
	if err != nil {
		return RunSegmentResult{}, err
	}
	defer cleanup()
	started := time.Now().UTC()
	out, err := runStroppy(ctx, stroppyContainer{
		Name:           "stroppy-" + segmentSlug(req.Index, seg.Name),
		Image:          req.Workload.StroppyImage,
		RegistrySecret: req.RegistrySecret,
		Dir:            dir,
		Args:           args,
		Labels:         map[string]string{"stroppy-run": req.RunID, "stroppy-segment": seg.Name},
		LogName:        seg.Name,
	})
	if err != nil {
		return RunSegmentResult{}, err
	}
	finished := time.Now().UTC()
	res := RunSegmentResult{
		Result:     spec.SegmentResult{Name: seg.Name, StartedAt: started, FinishedAt: finished, ExitCode: out.ExitCode},
		ConfigPath: cfgPath,
		LogPath:    out.LogPath,
	}
	res.Result.Status, res.Result.Error = segmentOutcome(ctx, &seg, &out, &res.Result)
	return res, nil
}

// segmentOutcome reads the summary and decides the segment status.
//
// doc: `stroppy run --help` Signals — nonfatal errors exit 0 and are
// summarized; 130/143 after a graceful cancellation; 1 for setup,
// validation, teardown, fatal or other command errors.
func segmentOutcome(ctx context.Context, seg *spec.Segment, out *stroppyOutput, r *spec.SegmentResult) (status spec.SegmentStatus, reason string) {
	summary := stroppycfg.ParseOutput(out.Log)
	if summary.Found {
		r.Metrics = summary.Metrics
		r.Errors = summary.Errors
		r.Compliance = summary.Compliance
	} else {
		obs.Warn(ctx, "no bench summary in stroppy output", obs.Str("segment", seg.Name))
	}
	switch {
	case out.StreamErr != nil && errors.Is(out.StreamErr, context.Canceled):
		return spec.SegmentCancelled, out.StreamErr.Error()
	case out.StreamErr != nil:
		return spec.SegmentFailed, out.StreamErr.Error()
	}
	if canceled, text := stroppycfg.ExitStatus(out.ExitCode); out.ExitCode != 0 {
		if canceled {
			return spec.SegmentCancelled, text
		}
		for _, line := range strings.Split(string(out.Log), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "Error:") {
				return spec.SegmentFailed, fmt.Sprintf("stroppy %s: %s", text, tail(strings.TrimSpace(line), errTailBytes))
			}
		}
		return spec.SegmentFailed, fmt.Sprintf("stroppy %s: %s", text, tail(lastLines(out.LogPath), errTailBytes))
	}
	if !summary.Found {
		return spec.SegmentFailed, "stroppy exited 0 without a bench summary: " + tail(lastLines(out.LogPath), errTailBytes)
	}
	if v := stroppycfg.ThresholdViolation(seg.Thresholds, summary); v != "" {
		return spec.SegmentFailed, "threshold: " + v
	}
	return spec.SegmentCompleted, ""
}

// RunBaselineRequest measures the runner machine with `stroppy baseline`.
type RunBaselineRequest struct {
	RunID          string        `json:"run_id"`
	Image          string        `json:"image"`
	Baseline       spec.Baseline `json:"baseline"`
	RegistrySecret string        `json:"registry_secret,omitempty"`
}

// RunBaselineResult is the parsed report plus the log location.
type RunBaselineResult struct {
	Result  spec.BaselineResult `json:"result"`
	LogPath string              `json:"log_path,omitempty"`
}

// RunBaseline runs ON THE RUNNER MACHINE before the segments: no database,
// only stroppy against itself (noop driver) and against pg-noop on
// loopback. A failed baseline is reported, not fatal — the numbers are
// context for the run, the verdicts are about stroppy's own ceiling.
//
// doc: `stroppy baseline --help`.
func RunBaseline(ctx context.Context, req RunBaselineRequest) (RunBaselineResult, error) {
	dir, err := workspaceSubdir("stroppy", "baseline")
	if err != nil {
		return RunBaselineResult{}, err
	}
	out, err := runStroppy(ctx, stroppyContainer{
		Name:           "stroppy-baseline-" + shortID(req.RunID),
		Image:          req.Image,
		RegistrySecret: req.RegistrySecret,
		Dir:            dir,
		Args:           stroppycfg.BaselineArgs(req.Baseline),
		Labels:         map[string]string{"stroppy-run": req.RunID, "stroppy-baseline": "1"},
		LogName:        "baseline",
	})
	if err != nil {
		return RunBaselineResult{}, err
	}
	res := RunBaselineResult{LogPath: out.LogPath}
	switch {
	case out.StreamErr != nil:
		return RunBaselineResult{}, out.StreamErr
	case out.ExitCode != 0:
		_, text := stroppycfg.ExitStatus(out.ExitCode)
		res.Result = spec.BaselineResult{Error: fmt.Sprintf("stroppy baseline %s: %s", text, tail(lastLines(out.LogPath), errTailBytes))}
		return res, nil
	}
	parsed, err := stroppycfg.ParseBaseline(out.Stdout)
	if err != nil {
		// An unreadable report is a baseline OUTCOME, not an activity
		// failure to retry: the run goes on without the numbers.
		res.Result = spec.BaselineResult{Error: err.Error()}
		return res, nil //nolint:nilerr // reported in the result
	}
	res.Result = parsed
	return res, nil
}

// stroppyContainer describes one throwaway stroppy process.
type stroppyContainer struct {
	Name, Image, RegistrySecret, Dir, LogName string
	Args                                      []string
	Labels                                    map[string]string
}

// stroppyOutput is what came back.
type stroppyOutput struct {
	ExitCode  int
	Log       []byte
	Stdout    []byte
	LogPath   string
	StreamErr error
}

// runStroppy pulls the image, replaces a stale container of the same name,
// runs stroppy with the directory mounted as /workspace on the host network
// and streams its output until exit.
func runStroppy(ctx context.Context, c stroppyContainer) (stroppyOutput, error) {
	cli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		return stroppyOutput{}, err
	}
	defer func() { _ = cli.Close() }()
	if _, err := PullImage(ctx, PullImageRequest{Image: c.Image, RegistrySecret: c.RegistrySecret}); err != nil {
		return stroppyOutput{}, err
	}
	// A leftover container of the same name (previous attempt that died
	// before reporting) is removed: the process is one-shot, its state
	// unknowable — the workflow decides what a re-run means, not we.
	if err := cli.ContainerRemove(ctx, c.Name, container.RemoveOptions{Force: true}); err != nil && !cerrdefs.IsNotFound(err) {
		obs.Warn(ctx, "stale stroppy container not removed", obs.Str("container", c.Name), obs.Err(err))
	}
	created, err := cli.ContainerCreate(ctx, &container.Config{
		Image:      c.Image,
		Cmd:        c.Args,
		WorkingDir: stroppycfg.ContainerWorkspace,
		Labels:     c.Labels,
	}, &container.HostConfig{
		NetworkMode: "host",
		Mounts:      []mount.Mount{{Type: mount.TypeBind, Source: c.Dir, Target: stroppycfg.ContainerWorkspace}},
		LogConfig:   container.LogConfig{Type: "json-file", Config: map[string]string{"max-size": "50m", "max-file": "3"}},
	}, nil, nil, c.Name)
	if err != nil {
		return stroppyOutput{}, fmt.Errorf("create stroppy container: %w", err)
	}
	if err := cli.ContainerStart(ctx, created.ID, container.StartOptions{}); err != nil {
		return stroppyOutput{}, fmt.Errorf("start stroppy container: %w", err)
	}
	obs.Info(ctx, "stroppy started", obs.Str("name", c.LogName), obs.Str("args", strings.Join(c.Args, " ")))
	if activity.IsActivity(ctx) {
		activity.RecordHeartbeat(ctx, c.LogName+": running workload")
	}

	out := stroppyOutput{LogPath: filepath.Join(c.Dir, "stroppy.log")}
	out.ExitCode, out.Log, out.Stdout, out.StreamErr = streamAndWait(ctx, cli, created.ID, out.LogPath, c.LogName)
	return out, nil
}

// workspaceSubdir is a directory inside the machine's run workspace: the
// same path on the machine, in this container and for the docker daemon
// (bind-mount source).
func workspaceSubdir(parts ...string) (string, error) {
	ws := machine.Workspace()
	if ws == "" {
		return "", fmt.Errorf("stroppy: no %s — not running in an agent container", machine.EnvWorkspace)
	}
	dir := filepath.Join(append([]string{ws}, parts...)...)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func segmentSlug(index int, name string) string {
	return fmt.Sprintf("%02d-%s", index, strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' {
			return r
		}
		return '-'
	}, strings.ToLower(name)))
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// writeSegmentInputs writes stroppy-config.json, the CA certificate and the
// segment files; returns the config path.
func writeSegmentInputs(dir string, req RunSegmentRequest) (string, error) {
	cfg, err := stroppycfg.MarshalConfig(stroppycfg.Input{
		RunID: req.RunID, Segment: req.Segment, Workload: req.Workload, URL: req.URL,
		OTLPEndpoint: req.OTLPEndpoint, OTLPHeaders: req.OTLPHeaders, Labels: req.Labels,
	})
	if err != nil {
		return "", err
	}
	cfgPath := filepath.Join(dir, stroppycfg.ConfigFile)
	// The config carries the database URL with its password: readable by
	// the agent user only; the container runs as root and reads it fine.
	if err := os.WriteFile(cfgPath, cfg, 0o600); err != nil {
		return "", err
	}
	if req.Workload.CACert != "" {
		if err := os.WriteFile(filepath.Join(dir, stroppycfg.CACertFile), []byte(req.Workload.CACert), 0o600); err != nil {
			return "", err
		}
	}
	for _, f := range req.Segment.Files {
		if f.Content == "" {
			continue
		}
		name := filepath.Base(f.Name)
		if name == "" || name == "." || name == ".." {
			return "", fmt.Errorf("segment file: bad name %q", f.Name)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(f.Content), 0o644); err != nil { //nolint:gosec // workload SQL/data for the stroppy container
			return "", err
		}
	}
	return cfgPath, nil
}

// streamAndWait copies the container's output to obs and the log file
// until it exits, heartbeating on every line, and keeps a bounded copy of
// the whole log and of stdout for the parsers. Returns the exit code.
func streamAndWait(ctx context.Context, cli *dockerclient.Client, id, logPath, name string) (exitCode int, log, stdout []byte, err error) {
	logs, err := cli.ContainerLogs(ctx, id, container.LogsOptions{ShowStdout: true, ShowStderr: true, Follow: true})
	if err != nil {
		return -1, nil, nil, fmt.Errorf("container logs: %w", err)
	}
	defer func() { _ = logs.Close() }()
	f, err := os.Create(logPath)
	if err != nil {
		return -1, nil, nil, err
	}
	defer func() { _ = f.Close() }()

	var (
		logBuf    bytes.Buffer
		stdoutBuf bytes.Buffer
	)
	pr, pw := io.Pipe()
	outTee := io.MultiWriter(pw, &limitedWriter{w: &stdoutBuf, limit: stdoutCaptureLimit})
	go func() {
		_, cerr := stdcopy.StdCopy(outTee, pw, logs)
		_ = pw.CloseWithError(cerr)
	}()
	sc := bufio.NewScanner(pr)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	lines := 0
	for sc.Scan() {
		line := sc.Text()
		if _, werr := f.WriteString(line + "\n"); werr != nil {
			obs.Warn(ctx, "log file write failed", obs.Err(werr))
		}
		if logBuf.Len() < stdoutCaptureLimit {
			logBuf.WriteString(line)
			logBuf.WriteByte('\n')
		}
		obs.Info(ctx, line, obs.Str("segment", name), obs.Str("stream", "stroppy"))
		lines++
		if lines%50 == 0 && activity.IsActivity(ctx) {
			activity.RecordHeartbeat(ctx, fmt.Sprintf("%s: %d lines", name, lines))
		}
	}
	waitCh, errCh := cli.ContainerWait(ctx, id, container.WaitConditionNotRunning)
	select {
	case res := <-waitCh:
		if res.Error != nil {
			return int(res.StatusCode), logBuf.Bytes(), stdoutBuf.Bytes(), errors.New(res.Error.Message)
		}
		return int(res.StatusCode), logBuf.Bytes(), stdoutBuf.Bytes(), nil
	case werr := <-errCh:
		return -1, logBuf.Bytes(), stdoutBuf.Bytes(), werr
	case <-ctx.Done():
		return -1, logBuf.Bytes(), stdoutBuf.Bytes(), ctx.Err()
	}
}

// limitedWriter keeps the first limit bytes and silently drops the rest.
type limitedWriter struct {
	w     *bytes.Buffer
	limit int
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	if room := l.limit - l.w.Len(); room > 0 {
		keep := p
		if len(keep) > room {
			keep = keep[:room]
		}
		l.w.Write(keep)
	}
	return len(p), nil
}

func lastLines(path string) string {
	const n = errTailLines
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
