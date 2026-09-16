package activities

import (
	"context"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
	"strings"
	"testing"
)

func TestSegmentOutcomeRetainsCLIErrorBeforeUsage(t *testing.T) {
	out := stroppyOutput{ExitCode: 1, Log: []byte("Error: invalid config: params must be scalar\nUsage:\n" + strings.Repeat("example command\n", 100))}
	result := spec.SegmentResult{}
	status, reason := segmentOutcome(context.Background(), &spec.Segment{Name: "tx"}, &out, &result)
	if status != spec.SegmentFailed || !strings.Contains(reason, "invalid config: params must be scalar") || strings.Contains(reason, "example command") {
		t.Fatalf("outcome=%s: %s", status, reason)
	}
}
