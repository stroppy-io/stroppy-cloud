package activities

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
	"github.com/stroppy-io/stroppy-cloud/pipelines/stroppycfg"
)

const (
	inlineReportLimit = 64 << 10
	parseReportLimit  = 16 << 20
)

// reportRuntimeArgs probes without executing a workload. Legacy images retain
// their text parser; supported images must produce the requested JSON document.
func reportRuntimeArgs(ctx context.Context, probe stroppyContainer, args []string) (updated []string, reportPath string, err error) {
	probe.Name += "-report-help"
	probe.LogName += "-report-help"
	probe.Args = []string{"run", "--help"}
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	out, err := runStroppy(probeCtx, probe)
	if err != nil {
		return nil, "", fmt.Errorf("probe Stroppy report support: %w", err)
	}
	if out.StreamErr != nil {
		return nil, "", fmt.Errorf("read Stroppy report capabilities: %w", out.StreamErr)
	}
	if out.ExitCode != 0 {
		return nil, "", fmt.Errorf("probe Stroppy report support: exit %d", out.ExitCode)
	}
	if !stroppycfg.HasRunReport(out.Log) {
		return args, "", nil
	}
	f, err := os.CreateTemp(probe.Dir, ".stroppy-report-*.json")
	if err != nil {
		return nil, "", err
	}
	if err := f.Close(); err != nil {
		return nil, "", err
	}
	// A fresh absent path distinguishes no report from a stale/empty report.
	if err := os.Remove(f.Name()); err != nil {
		return nil, "", err
	}
	return managedReportArgs(args, path.Join(stroppycfg.ContainerWorkspace, filepath.Base(f.Name()))), f.Name(), nil
}

func managedReportArgs(args []string, destination string) []string {
	out := make([]string, 0, len(args)+2)
	for i := 0; i < len(args); i++ {
		name, _, assigned := strings.Cut(args[i], "=")
		switch name {
		case "--no-report":
			continue
		case "--report-file", "--report-format":
			if !assigned && i+1 < len(args) {
				i++
			}
			continue
		}
		out = append(out, args[i])
	}
	return append(out, "--report-file", destination)
}

func reportOutcome(seg *spec.Segment, out *stroppyOutput, result *spec.SegmentResult) (status spec.SegmentStatus, reason string) {
	f, err := os.Open(out.ReportPath)
	if err != nil {
		return reportFailure(out, fmt.Sprintf("read Stroppy report: %v", err))
	}
	defer func() { _ = f.Close() }()
	raw, err := io.ReadAll(io.LimitReader(f, parseReportLimit+1))
	if err != nil {
		return reportFailure(out, fmt.Sprintf("read Stroppy report: %v", err))
	}
	if len(raw) > parseReportLimit {
		result.ReportOmitted = "report exceeds parsing limit; see report artifact"
		return reportFailure(out, "Stroppy report exceeds 16 MiB parsing limit")
	}
	if json.Valid(raw) {
		var compact bytes.Buffer
		if err := json.Compact(&compact, raw); err != nil {
			return reportFailure(out, err.Error())
		}
		if compact.Len() <= inlineReportLimit {
			result.Report = compact.Bytes()
		} else {
			result.ReportOmitted = "report exceeds inline limit; see report artifact"
		}
	}
	doc, summary, err := stroppycfg.ParseReport(raw)
	if err != nil {
		return reportFailure(out, err.Error())
	}
	result.Metrics, result.Errors, result.Compliance = summary.Metrics, summary.Errors, summary.Compliance
	if out.StreamErr != nil {
		return reportFailure(out, out.StreamErr.Error())
	}
	if doc.Status == "failed" || doc.Status == "canceled" {
		reason := "Stroppy report status: " + doc.Status
		if doc.Failure != nil {
			reason += ": " + doc.Failure.Phase + ": " + doc.Failure.Reason
		}
		if doc.Status == "canceled" {
			return spec.SegmentCancelled, tail(reason, errTailBytes)
		}
		return reportFailure(out, tail(reason, errTailBytes))
	}
	if out.ExitCode != 0 {
		return reportFailure(out, fmt.Sprintf("Stroppy exited %d with report status %s", out.ExitCode, doc.Status))
	}
	if reason := stroppycfg.ThresholdViolation(seg.Thresholds, summary); reason != "" {
		return spec.SegmentFailed, "threshold: " + reason
	}
	return spec.SegmentCompleted, ""
}

func reportFailure(out *stroppyOutput, reason string) (status spec.SegmentStatus, diagnostic string) {
	canceled, _ := stroppycfg.ExitStatus(out.ExitCode)
	if canceled || errors.Is(out.StreamErr, context.Canceled) {
		return spec.SegmentCancelled, reason
	}
	return spec.SegmentFailed, reason
}
