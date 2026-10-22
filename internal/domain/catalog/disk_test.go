package catalog

import "testing"

func TestDiskLimits(t *testing.T) {
	yc := providers()[0]
	cases := []struct {
		disk    string
		gb, cpu int
		want    DiskLimits
	}{
		// 200 GB = 7 units of 32 GB; the VM cap (8 × 3500 IOPS, 8 × 45 MB/s) is higher.
		{"network-ssd", 200, 8, DiskLimits{ReadIOPS: 7000, WriteIOPS: 7000, ReadMBps: 105, WriteMBps: 105}},
		// 512 GB = 16 units: 240 MB/s, what CockroachDB asks for 8 vCPU.
		{"network-ssd", 512, 8, DiskLimits{ReadIOPS: 16000, WriteIOPS: 16000, ReadMBps: 240, WriteMBps: 240}},
		// A 2-vCPU VM caps the disk: 7000 IOPS from the disk, 2 × 3500 from the VM.
		{"network-ssd", 400, 2, DiskLimits{ReadIOPS: 7000, WriteIOPS: 7000, ReadMBps: 90, WriteMBps: 90}},
		{"network-ssd-nonreplicated", 186, 8, DiskLimits{ReadIOPS: 56000, WriteIOPS: 11200, ReadMBps: 220, WriteMBps: 164}},
		{"unknown", 100, 8, DiskLimits{}},
	}
	for _, tc := range cases {
		if got := yc.DiskLimitsOf(tc.disk, tc.gb, tc.cpu); got != tc.want {
			t.Errorf("%s %d GB / %d vCPU = %+v, want %+v", tc.disk, tc.gb, tc.cpu, got, tc.want)
		}
	}
}
