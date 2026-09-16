package spec

import (
	"strings"
	"testing"
)

func TestDiskPreparationMatchesFilesystemAndIsIdempotent(t *testing.T) {
	r := Run{Provider: Provider{Kind: ProviderYandex}, Machines: []Machine{{Name: "db-1", Role: "db", Disks: []Disk{{Name: "data", GB: 100, Type: "network-ssd", Mount: "/data"}, {Name: "wal", GB: 93, Type: "network-ssd-nonreplicated", Mount: "/wal", Filesystem: "xfs", MountOptions: []string{"noatime", "nodiratime"}}, {Name: "raw", GB: 93, Type: "network-ssd-nonreplicated"}}}}}
	EnsureDiskPreparation(&r)
	if len(r.HostPrep) != 2 {
		t.Fatal("raw disk was formatted or mounted disk omitted")
	}
	h := r.HostPrep[1]
	for _, want := range []string{"virtio-wal", "target='/wal'", "filesystem='xfs'", "options='noatime,nodiratime'", "wipefs --no-act", "refusing to hide existing files"} {
		if !strings.Contains(h.Content, want) {
			t.Fatalf("missing %q", want)
		}
	}
	if h.Machine != "db-1" {
		t.Fatal("disk step targets wrong machine")
	}
	EnsureDiskPreparation(&r)
	if len(r.HostPrep) != 2 {
		t.Fatal("replay duplicated generated preparation")
	}
}
