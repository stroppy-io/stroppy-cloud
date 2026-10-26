package activities

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/graphene-ci/pipeline/pkg/machine"
	"github.com/graphene-ci/pipeline/pkg/obs"
)

// Wire names of the connectivity check.
const (
	NameNetProbe    = "stroppy.net.probe"
	NameNetSnapshot = "stroppy.net.snapshot"
)

// NetOutcome is how one TCP dial ended — the server's ConnectivityOutcome.
type NetOutcome string

const (
	NetOK      NetOutcome = "ok"
	NetRefused NetOutcome = "refused"
	NetTimeout NetOutcome = "timeout"
	NetOther   NetOutcome = "other"
)

// NetOutcomes is every outcome a pair can have.
var NetOutcomes = []NetOutcome{NetOK, NetRefused, NetTimeout, NetOther}

// Bounds of a probe when the request leaves them zero.
const (
	defaultProbeDial   = 3 * time.Second
	defaultProbeWindow = 10 * time.Second
	probeRetryPause    = time.Second
	// probeMaxTargets keeps one machine's request bounded whatever the spec.
	probeMaxTargets = 512
)

// NetTarget is one address a machine dials.
type NetTarget struct {
	// Machine is the machine the address belongs to (or the endpoint's host
	// when it is no machine of the run).
	Machine   string `json:"machine"`
	Address   string `json:"address"`
	Port      int    `json:"port"`
	Container string `json:"container,omitempty"`
	// Critical marks a target the workload or the database cluster needs.
	Critical bool `json:"critical,omitempty"`
	// Endpoint marks the workload's own endpoint: it must answer, not just
	// be reachable.
	Endpoint bool `json:"endpoint,omitempty"`
}

// NetProbeRequest is what one machine dials.
type NetProbeRequest struct {
	Targets []NetTarget `json:"targets"`
	// DialTimeout bounds one dial; Window bounds re-dialing a failing target.
	DialTimeout time.Duration `json:"dial_timeout,omitempty"`
	Window      time.Duration `json:"window,omitempty"`
}

// NetPairResult is the outcome of dialing one target.
type NetPairResult struct {
	NetTarget
	Outcome   NetOutcome `json:"outcome"`
	LatencyMs float64    `json:"latency_ms"`
	Attempts  int        `json:"attempts"`
	Error     string     `json:"error,omitempty"`
}

// NetProbeResult lists the targets in request order.
type NetProbeResult struct {
	Pairs []NetPairResult `json:"pairs"`
}

// NetProbe dials every target from this machine, all at once. A failing
// target is dialed again until the window closes (a path that opens a
// second late is not a broken network); the last attempt decides. The
// verdict is in the result: the activity fails only on a bad request.
func NetProbe(ctx context.Context, req NetProbeRequest) (NetProbeResult, error) {
	if len(req.Targets) > probeMaxTargets {
		return NetProbeResult{}, fmt.Errorf("net probe: %d targets, at most %d", len(req.Targets), probeMaxTargets)
	}
	dial, window := req.DialTimeout, req.Window
	if dial <= 0 {
		dial = defaultProbeDial
	}
	if window <= 0 {
		window = defaultProbeWindow
	}
	deadline := time.Now().Add(window)
	out := NetProbeResult{Pairs: make([]NetPairResult, len(req.Targets))}
	var wg sync.WaitGroup
	for i, target := range req.Targets {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out.Pairs[i] = probeTarget(ctx, target, dial, deadline)
		}()
	}
	wg.Wait()
	failed := 0
	for _, p := range out.Pairs {
		if p.Outcome != NetOK {
			failed++
		}
	}
	obs.Info(ctx, "net probe", obs.Int("targets", len(out.Pairs)), obs.Int("failed", failed))
	return out, nil
}

func probeTarget(ctx context.Context, target NetTarget, dial time.Duration, deadline time.Time) NetPairResult {
	res := NetPairResult{NetTarget: target}
	dialer := net.Dialer{Timeout: dial}
	for {
		res.Attempts++
		start := time.Now()
		conn, err := dialer.DialContext(ctx, "tcp", target.Address)
		res.LatencyMs = float64(time.Since(start).Microseconds()) / 1000
		res.Outcome = ClassifyDial(err)
		if err == nil {
			_ = conn.Close()
			res.Error = ""
			return res
		}
		res.Error = err.Error()
		if ctx.Err() != nil || time.Now().Add(probeRetryPause).After(deadline) {
			return res
		}
		timer := time.NewTimer(probeRetryPause)
		select {
		case <-ctx.Done():
			timer.Stop()
			return res
		case <-timer.C:
		}
	}
}

// ClassifyDial names how a dial ended.
func ClassifyDial(err error) NetOutcome {
	if err == nil {
		return NetOK
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		return NetRefused
	}
	var ne net.Error
	if errors.Is(err, context.DeadlineExceeded) || errors.As(err, &ne) && ne.Timeout() {
		return NetTimeout
	}
	return NetOther
}

// ClassifyDialText names a dial failure known only by its text (an
// activity error that crossed Temporal). The last dial error wins over the
// wait's own deadline.
func ClassifyDialText(s string) NetOutcome {
	switch {
	case s == "":
		return NetOK
	case strings.Contains(s, "connection refused"):
		return NetRefused
	case strings.Contains(s, "i/o timeout"), strings.Contains(s, "deadline exceeded"), strings.Contains(s, "timed out"):
		return NetTimeout
	}
	return NetOther
}

// EndpointAddress is the host:port of a workload URL ("" when the URL
// names none).
func EndpointAddress(endpoint string) string { return endpointAddress(endpoint) }

// netSnapshotDir is the workspace-relative directory of snapshots.
const netSnapshotDir = "netdiag"

// Bounds of a snapshot.
const (
	snapshotCommandTimeout = 10 * time.Second
	snapshotCommandBytes   = 64 << 10
)

// snapshotCommands are read-only views of the machine's network state, in
// the order an operator reads them. Tools a machine lacks say so.
var snapshotCommands = []string{
	"ip -br addr",
	"ip route",
	"ip neigh",
	"ss -ltnp",
	"ss -tn state syn-sent",
	"if command -v iptables-save >/dev/null 2>&1; then iptables-save; else echo 'iptables-save: not installed'; fi",
	"if command -v nft >/dev/null 2>&1; then nft list ruleset; else echo 'nft: not installed'; fi",
	"sysctl net.ipv4.ip_local_port_range net.ipv4.conf.all.rp_filter net.ipv4.ip_forward",
	"dmesg 2>&1 | tail -n 50",
	"cat /etc/sysctl.d/90-stroppy-*.conf",
	// Package activity during the run restarts network services: show what
	// was installed or upgraded and what systemd restarted.
	"tail -n 40 /var/log/dpkg.log",
	"journalctl --no-pager -n 40 -u unattended-upgrades -u apt-daily.service -u apt-daily-upgrade.service",
	"journalctl --no-pager -n 60 -u systemd-networkd",
}

// NetSnapshotRequest names the snapshot and what it is about.
type NetSnapshotRequest struct {
	// Label names the file (<label>.txt), e.g. probe-db-1.
	Label string `json:"label"`
	// Machine and Context head the file: which machine, which failed paths.
	Machine string   `json:"machine"`
	Context []string `json:"context,omitempty"`
}

// NetSnapshotResult is where the snapshot was written.
type NetSnapshotResult struct {
	Path     string `json:"path"`
	Commands int    `json:"commands"`
	Failed   int    `json:"failed"`
}

// NetSnapshot captures the machine's network state ON THE MACHINE into a
// workspace file. Read-only; every command is bounded in time and output,
// and a failing command is recorded, not fatal.
func NetSnapshot(ctx context.Context, req NetSnapshotRequest) (NetSnapshotResult, error) {
	if req.Label == "" || strings.ContainsAny(req.Label, "/\\") || strings.Contains(req.Label, "..") {
		return NetSnapshotResult{}, fmt.Errorf("net snapshot: bad label %q", req.Label)
	}
	dir, err := workspaceDir(netSnapshotDir)
	if err != nil {
		return NetSnapshotResult{}, fmt.Errorf("net snapshot: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return NetSnapshotResult{}, err
	}
	var b bytes.Buffer
	fmt.Fprintf(&b, "# network snapshot of %s at %s\n", req.Machine, time.Now().UTC().Format(time.RFC3339))
	for _, c := range req.Context {
		fmt.Fprintf(&b, "# %s\n", c)
	}
	res := NetSnapshotResult{Commands: len(snapshotCommands)}
	for _, command := range snapshotCommands {
		out, err := runBounded(ctx, command)
		fmt.Fprintf(&b, "\n$ %s\n%s", command, out)
		if out != "" && out[len(out)-1] != '\n' {
			b.WriteByte('\n')
		}
		if err != nil {
			res.Failed++
			fmt.Fprintf(&b, "[%v]\n", err)
		}
	}
	res.Path = filepath.Join(dir, req.Label+".txt")
	if err := os.WriteFile(res.Path, b.Bytes(), 0o644); err != nil { //nolint:gosec // published as an artifact
		return NetSnapshotResult{}, err
	}
	return res, nil
}

// runBounded runs one shell command on the machine with a deadline and
// keeps at most snapshotCommandBytes of its output.
func runBounded(ctx context.Context, command string) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, snapshotCommandTimeout)
	defer cancel()
	cmd := machine.Shell(cctx, command)
	cmd.WaitDelay = time.Second
	buf := &capped{max: snapshotCommandBytes}
	cmd.Stdout, cmd.Stderr = buf, buf
	err := cmd.Run()
	if cctx.Err() != nil {
		err = fmt.Errorf("timed out after %s", snapshotCommandTimeout)
	}
	return buf.String(), err
}

// capped keeps the head of a stream and counts what it drops.
type capped struct {
	buf     bytes.Buffer
	max     int
	dropped int
}

func (c *capped) Write(p []byte) (int, error) {
	room := c.max - c.buf.Len()
	switch {
	case room <= 0:
		c.dropped += len(p)
	case len(p) > room:
		c.buf.Write(p[:room])
		c.dropped += len(p) - room
	default:
		c.buf.Write(p)
	}
	return len(p), nil
}

func (c *capped) String() string {
	if c.dropped == 0 {
		return c.buf.String()
	}
	return fmt.Sprintf("%s\n[truncated: %d more bytes]\n", c.buf.String(), c.dropped)
}
