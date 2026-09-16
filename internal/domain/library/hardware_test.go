package library

import (
	"encoding/json"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

func TestHardwareDefaultsRoundFastDisksButPreserveExplicitSizes(t *testing.T) {
	base := spec.Machine{Name: "db-1", Role: "db", CPU: 16, MemoryGB: 64, Disks: []spec.Disk{{Name: "data", GB: 400, Type: "network-ssd-io-m3", Mount: "/data"}}}
	m, err := ResolveHardware(base, RoleSize{Size: "L"}, spec.DatasetEstimate{})
	if err != nil {
		t.Fatal(err)
	}
	if m.Disks[0].GB != 465 {
		t.Fatalf("default fast SSD must round up: %+v", m.Disks)
	}
	m, err = ResolveHardware(base, RoleSize{Size: "L", DiskGB: 400}, spec.DatasetEstimate{})
	if err != nil {
		t.Fatal(err)
	}
	if m.Disks[0].GB != 400 {
		t.Fatal("explicit invalid disk silently changed")
	}
	_, err = ResolveHardware(base, RoleSize{Size: "L", Machine: json.RawMessage(`{"cpu":0}`)}, spec.DatasetEstimate{})
	if err == nil {
		t.Fatal("invalid explicit CPU accepted")
	}
}
