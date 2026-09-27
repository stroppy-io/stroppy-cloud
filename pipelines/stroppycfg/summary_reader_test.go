package stroppycfg

import (
	"errors"
	"io"
	"strings"
	"testing"
)

type failedSummaryReader struct{ err error }

func (r failedSummaryReader) Read([]byte) (int, error) { return 0, r.err }

func TestParseOutputReaderReportsTruncatedStream(t *testing.T) {
	want := errors.New("log read interrupted")
	r := io.MultiReader(strings.NewReader("=== bench summary ===\n  iterations_total  42\n"), failedSummaryReader{want})
	got, err := ParseOutputReader(r)
	if !errors.Is(err, want) || !got.Found || got.Metrics["iterations_total"].Value != 42 {
		t.Fatalf("summary=%+v error=%v", got, err)
	}
}
