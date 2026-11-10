package compile

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/library"
)

// Two machines of the product: M (8 vCPU / 32 GB, one instance per vCPU)
// and XL (32 vCPU / 128 GB, one per vCPU or 16 / 8 when set).
var (
	picoM    = picoShape{instances: 8, perMachine: 8, machines: 1, memoryMB: 32 << 10}
	picoXL   = picoShape{instances: 32, perMachine: 32, machines: 1, memoryMB: 128 << 10}
	picoXL16 = picoShape{instances: 16, perMachine: 16, machines: 1, memoryMB: 128 << 10}
)

func picoSegment(script string, sf float64, vus int) string {
	return fmt.Sprintf(`{"name":"main","workload":{"script":%q,"scale_factor":%g},"run":{"vus":%d}}`, script, sf, vus)
}

func TestPicoMotionRows(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		segments []string
		shape    picoShape
		want     int
		refusal  map[string]string
	}{
		{"no segments keep the per-instance rule", nil, picoXL, 1600000, nil},
		{"tpcc keeps the per-instance rule", []string{picoSegment("tpcc/tx", 100, 128)}, picoXL, 1600000, nil},
		{"tpcc on M", []string{picoSegment("tpcc/tx", 100, 128)}, picoM, 400000, nil},
		{"tpcds loads only, per-instance rule", []string{picoSegment("tpcds", 1, 1)}, picoXL, 1600000, nil},
		{"tpch SF 0.1 on M", []string{picoSegment("tpch/tx", 0.1, 1)}, picoM, 660134, nil},
		{"tpch SF 0.2 on M", []string{picoSegment("tpch/tx", 0.2, 1)}, picoM, 1320268, nil},
		{"tpch SF 0.5 on XL", []string{picoSegment("tpch/tx", 0.5, 1)}, picoXL, 3300669, nil},
		{"tpch SF 1 on XL with 16 instances", []string{picoSegment("tpch/tx", 1, 1)}, picoXL16, 6601337, nil},
		{"tpch next to tpcc keeps the larger", []string{picoSegment("tpcc/tx", 1, 1), picoSegment("tpch/tx", 0.1, 1)}, picoXL, 1600000, nil},
		{"tpch without a scale factor is SF 1", []string{`{"name":"main","workload":{"script":"tpch/tx"},"run":{"vus":1}}`}, picoXL16, 6601337, nil},
		{
			"tpch SF 0.5 on M does not fit", []string{picoSegment("tpch/tx", 0.5, 1)}, picoM, 0,
			map[string]string{"scale_factor": "0.5", "max_scale_factor": "0.43", "memory_gb": "32", "instances_per_machine": "8", "concurrent_queries": "1"},
		},
		{
			"tpch SF 1 on XL with an instance per vCPU does not fit", []string{picoSegment("tpch/tx", 1, 1)}, picoXL, 0,
			map[string]string{"scale_factor": "1", "max_scale_factor": "0.62", "memory_gb": "128", "instances_per_machine": "32", "concurrent_queries": "1"},
		},
		{
			"tpch SF 10 on XL does not fit", []string{picoSegment("tpch/tx", 10, 1)}, picoXL, 0,
			map[string]string{"scale_factor": "10", "max_scale_factor": "0.62", "memory_gb": "128", "instances_per_machine": "32", "concurrent_queries": "1"},
		},
		{
			"two concurrent queries halve the budget", []string{picoSegment("tpch/tx", 1, 2)}, picoXL16, 0,
			map[string]string{"scale_factor": "1", "max_scale_factor": "0.55", "memory_gb": "128", "instances_per_machine": "16", "concurrent_queries": "2"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			segments := make([]json.RawMessage, len(tc.segments))
			for i, s := range tc.segments {
				segments[i] = json.RawMessage(s)
			}
			c := &compilation{in: Input{Workload: library.WorkloadSpec{Segments: segments}}}
			got, err := c.picoMotionRows(tc.shape)
			if tc.refusal != nil {
				reason, params := errs.ReasonOf(err)
				if reason != "picodata_scale_exceeds_memory" {
					t.Fatalf("reason %q (%v), want a refusal", reason, err)
				}
				for k, v := range tc.refusal {
					if params[k] != v {
						t.Errorf("%s = %q, want %q", k, params[k], v)
					}
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("rows %d, want %d", got, tc.want)
			}
		})
	}
}

// A machine of unknown memory (an external database) is not sized: the
// analytic need stands.
func TestPicoMotionRowsWithoutMemory(t *testing.T) {
	t.Parallel()
	c := &compilation{in: Input{Workload: library.WorkloadSpec{Segments: []json.RawMessage{json.RawMessage(picoSegment("tpch/tx", 10, 1))}}}}
	got, err := c.picoMotionRows(picoShape{instances: 4, perMachine: 1, machines: 4})
	if err != nil || got != 66013365 {
		t.Errorf("rows %d, err %v, want 66013365", got, err)
	}
}

func BenchmarkPicoMotionRows(b *testing.B) {
	c := &compilation{in: Input{Workload: library.WorkloadSpec{Segments: []json.RawMessage{json.RawMessage(picoSegment("tpch/tx", 0.5, 1))}}}}
	for b.Loop() {
		if _, err := c.picoMotionRows(picoXL); err != nil {
			b.Fatal(err)
		}
	}
}
