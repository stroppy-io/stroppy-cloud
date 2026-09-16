package api

import (
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/oas"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// Both sides must expose the same fields; a new native field requires an API
// mapping and an intentional contract version change, not a silent projection.
func TestResultFieldsMatchPipeline(t *testing.T) {
	for _, pair := range [][2]any{
		{spec.Result{}, oas.RunResult{}},
		{spec.SegmentResult{}, oas.RunResultSegmentsItem{}},
		{spec.ErrorCounts{}, oas.RunSegmentErrors{}},
		{spec.BaselineResult{}, oas.RunBaselineResult{}},
		{spec.BaselineVerdict{}, oas.RunBaselineResultVerdictsItem{}},
		{spec.Summary{}, oas.PipelineSummary{}},
		{spec.MetricValue{}, oas.MetricValue{}},
	} {
		fields := func(value any) []string {
			typ := reflect.TypeOf(value)
			var out []string
			for i := range typ.NumField() {
				out = append(out, strings.Split(typ.Field(i).Tag.Get("json"), ",")[0])
			}
			slices.Sort(out)
			return out
		}
		if a, b := fields(pair[0]), fields(pair[1]); !slices.Equal(a, b) {
			t.Errorf("%T -> %T loses contract fields: %v / %v", pair[0], pair[1], a, b)
		}
	}
}

func TestCompleteNativeResultProjection(t *testing.T) {
	raw, err := os.ReadFile("../../pipelines/live/tests/platform/contracts/result-complete.json")
	if err != nil {
		t.Fatal(err)
	}
	wire, ok := resultOf(raw).Get()
	if !ok {
		t.Fatal("valid result disappeared")
	}
	encoded, err := wire.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	want := strings.ReplaceAll(string(raw), `"canceled"`, `"cancelled"`)
	sameJSON(t, json.RawMessage(want), encoded)
	empty, ok := resultOf(json.RawMessage(`{"segments":[{"name":"main","status":"completed","exit_code":0}]}`)).Get()
	if !ok || empty.Summary.IsSet() || empty.Baseline.IsSet() || !empty.Segments[0].ExitCode.IsSet() {
		t.Fatal("absent reports or explicit zero changed meaning")
	}
}

func TestSchemaValuesAreBrowserSafe(t *testing.T) {
	raw := json.RawMessage(`{"seed":18446744073709551615,"nested":{"signed":-9007199254740992,"safe":9007199254740991},"env":{},"files":[],"enabled":false,"zero":0,"file":"literal \\n 18446744073709551615"}`)
	want := json.RawMessage(`{"seed":"18446744073709551615","nested":{"signed":"-9007199254740992","safe":9007199254740991},"env":{},"files":[],"enabled":false,"zero":0,"file":"literal \\n 18446744073709551615"}`)
	sameJSON(t, want, rawOf(schemaValueOf(raw)))
	encoded, err := json.Marshal(bakedValueOf("test.runtime", "1", raw).Values)
	if err != nil {
		t.Fatal(err)
	}
	sameJSON(t, want, encoded)
}
