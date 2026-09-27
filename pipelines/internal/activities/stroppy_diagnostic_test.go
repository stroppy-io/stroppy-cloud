package activities

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

func TestSegmentOutcomeRetainsCLIErrorBeforeUsage(t *testing.T) {
	out := stroppyOutput{ExitCode: 1, Log: []byte("Error: invalid config: params must be scalar\nUsage:\n" + strings.Repeat("example command\n", 100))}
	result := spec.SegmentResult{}
	status, reason := segmentOutcome(context.Background(), &spec.Segment{Name: "tx"}, &out, &result)
	if status != spec.SegmentFailed || !strings.Contains(reason, "invalid config: params must be scalar") || strings.Contains(reason, "example command") {
		t.Fatalf("outcome=%s: %s", status, reason)
	}
}

// Reproduce a successful long run whose summary follows more than 8 MiB of logs.
func TestSegmentOutcomeReadsCompleteLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stroppy.log")
	prefix := strings.Repeat("progress line\n", stdoutCaptureLimit/14+1)
	report := `{"compliance":{"passed":true}}
=== bench summary ===
  iterations_total  1000
  iterations_per_second  123.456
  failed_iterations_total  2
=== bench completed with errors ===
  failed_iterations_total  2
`
	if err := os.WriteFile(path, []byte(prefix+report), 0600); err != nil {
		t.Fatal(err)
	}
	out := stroppyOutput{Log: []byte(prefix[:stdoutCaptureLimit]), LogPath: path}
	result := spec.SegmentResult{}
	status, reason := segmentOutcome(context.Background(), &spec.Segment{Name: "measure"}, &out, &result)
	if status != spec.SegmentCompleted {
		t.Fatalf("%s: %s", status, reason)
	}
	if result.Metrics["iterations_total"].Value != 1000 || result.Metrics["iterations_per_second"].Value != 123.456 || result.Errors == nil || result.Errors.FailedIterations != 2 || string(result.Compliance) != `{"passed":true}` {
		t.Fatalf("incomplete native result: %+v", result)
	}
	zero := float64(0)
	status, reason = segmentOutcome(context.Background(), &spec.Segment{Name: "measure", Thresholds: spec.Thresholds{ErrorRate: &zero}}, &out, &result)
	if status != spec.SegmentFailed || !strings.HasPrefix(reason, "threshold:") {
		t.Fatalf("threshold lost: %s: %s", status, reason)
	}
}

func TestSegmentOutcomeRejectsUnreadableCompleteLog(t *testing.T) {
	out := stroppyOutput{Log: []byte("=== bench summary ===\n  iterations_total  1\n"), LogPath: filepath.Join(t.TempDir(), "missing.log")}
	status, reason := segmentOutcome(context.Background(), &spec.Segment{}, &out, &spec.SegmentResult{})
	if status != spec.SegmentFailed || !strings.Contains(reason, "read stroppy summary") {
		t.Fatalf("%s: %s", status, reason)
	}
}
