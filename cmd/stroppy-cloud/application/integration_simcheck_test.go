//go:build integration

package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestSimLiveRunSpecs runs every RunSpec a live YC run accepted
// (pipelines/live/tests/**/runs/*/input.json) through the real pipeline in
// the simulation: the simulated cloud must converge every topology the
// real one did. Inputs of a contract before 1.0 no longer decode and are
// counted, not failed.
func TestSimLiveRunSpecs(t *testing.T) {
	var inputs []string
	for _, pattern := range []string{"*/*/*/*/runs/*/input.json", "*/*/*/*/*/runs/*/input.json"} {
		found, err := filepath.Glob(filepath.Join("../../../pipelines/live/tests", pattern))
		if err != nil {
			t.Fatal(err)
		}
		inputs = append(inputs, found...)
	}
	if len(inputs) == 0 {
		t.Skip("no live inputs")
	}
	start := time.Date(2026, 9, 21, 10, 0, 0, 0, time.UTC)
	simulated, stale := 0, 0
	for _, path := range inputs {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		rec, err := simulate("sim-live-input", "stroppy-run", raw, start, simScenario{}, 0, nil)
		if err != nil && strings.Contains(err.Error(), "run spec:") {
			stale++
			continue
		}
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if rec.Status != "completed" {
			t.Errorf("%s: %s: %s", path, rec.Status, rec.Error)
		}
		simulated++
	}
	t.Logf("%d live RunSpecs simulated, %d of an older contract skipped", simulated, stale)
}
