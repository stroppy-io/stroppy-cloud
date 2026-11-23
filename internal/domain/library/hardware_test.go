package library

import (
	"encoding/json"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

func TestHardwareDefaultsRoundFastDisksButPreserveExplicitSizes(t *testing.T) {
	base := spec.Machine{Name: "db-1", Role: "db", CPU: 16, MemoryGB: 64, Disks: []spec.Disk{{Name: "data", GB: 400, Type: "network-ssd-io-m3", Mount: "/data"}}}
	m, err := ResolveHardware(base, RoleSize{Size: "L"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if m.Disks[0].GB != 465 {
		t.Fatalf("default fast SSD must round up: %+v", m.Disks)
	}
	m, err = ResolveHardware(base, RoleSize{Size: "L", DiskGB: 400}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if m.Disks[0].GB != 400 {
		t.Fatal("explicit invalid disk silently changed")
	}
	_, err = ResolveHardware(base, RoleSize{Size: "L", Machine: json.RawMessage(`{"cpu":0}`)}, 0)
	if err == nil {
		t.Fatal("invalid explicit CPU accepted")
	}
}

func TestDataDiskGB(t *testing.T) {
	t.Parallel()
	plan := topology.Plan{DataShare: map[string]float64{topology.RoleDB: 0.25}}
	estimate := spec.DatasetEstimate{RecommendedGB: 3002}
	for _, tc := range []struct {
		role string
		want int
	}{
		{topology.RoleDB, 751},        // a quarter, rounded up
		{topology.RoleCoordinator, 0}, // no table data
		{topology.RoleRunner, 0},
	} {
		if got := DataDiskGB(plan, tc.role, estimate); got != tc.want {
			t.Errorf("%s: %d GB, want %d", tc.role, got, tc.want)
		}
	}
}
