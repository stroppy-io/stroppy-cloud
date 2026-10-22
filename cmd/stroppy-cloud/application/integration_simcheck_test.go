//go:build integration

package application

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pipelinespec "github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// withSubnets gives a recorded Yandex RunSpec from before the profile
// network the subnets the queue allocates today (one /24 per zone, which
// fits any zone); anything else passes unchanged.
func withSubnets(raw []byte) []byte {
	var spec pipelinespec.Run
	if json.Unmarshal(raw, &spec) != nil || spec.Provider.Kind != pipelinespec.ProviderYandex || len(spec.Network.Subnets) > 0 {
		return raw
	}
	zones, err := pipelinespec.YandexZones(spec)
	if err != nil {
		return raw
	}
	var doc map[string]any
	if json.Unmarshal(raw, &doc) != nil {
		return raw
	}
	network, _ := doc["network"].(map[string]any) //nolint:errcheck // absent: a new object
	if network == nil {
		network = map[string]any{}
	}
	subnets := make([]any, 0, len(zones))
	for i, z := range zones {
		subnets = append(subnets, map[string]any{"zone": z.Zone, "cidr": fmt.Sprintf("10.128.%d.0/24", i)})
	}
	network["subnets"] = subnets
	doc["network"] = network
	out, err := json.Marshal(doc)
	if err != nil {
		return raw
	}
	return out
}

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
		rec, err := simulate("sim-live-input", "stroppy-run", withSubnets(raw), start, simScenario{}, 0, nil)
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
