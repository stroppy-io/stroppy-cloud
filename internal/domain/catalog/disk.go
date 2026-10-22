package catalog

import "math"

// DiskPerf is how fast a provider disk type is: the cloud grows a disk's
// IOPS and bandwidth with its size in allocation units, caps it per disk,
// and caps all disks of a VM per vCPU. Zero UnitGB means a flat baseline
// (the per-unit values are the disk's own).
//
// doc: yandex.cloud/en/docs/compute/concepts/limits — "Disks", "Virtual
// machine disk performance"; docs.aws.amazon.com/ebs/latest/userguide/general-purpose.html
type DiskPerf struct {
	UnitGB                     int
	ReadIOPSPerUnit            int
	WriteIOPSPerUnit           int
	ReadMBpsPerUnit            int
	WriteMBpsPerUnit           int
	MaxReadIOPS, MaxWriteIOPS  int
	MaxReadMBps, MaxWriteMBps  int
	VMIOPSPerCPU, VMMBpsPerCPU int
}

// DiskLimits is what one disk of a machine can do.
type DiskLimits struct {
	ReadIOPS, WriteIOPS int
	ReadMBps, WriteMBps int
}

// IsZero reports an unknown disk.
func (l DiskLimits) IsZero() bool { return l == DiskLimits{} }

// Limits of a disk of gb on a machine of cpu vCPUs: the disk's own limit
// by its allocation units, capped by the disk type and by the VM.
func (p DiskPerf) Limits(gb, cpu int) DiskLimits {
	if p == (DiskPerf{}) || gb <= 0 {
		return DiskLimits{}
	}
	units := 1
	if p.UnitGB > 0 {
		units = int(math.Ceil(float64(gb) / float64(p.UnitGB)))
	}
	capped := func(perUnit, maxDisk, perCPU int) int {
		v := perUnit * units
		if maxDisk > 0 {
			v = min(v, maxDisk)
		}
		if perCPU > 0 && cpu > 0 {
			v = min(v, perCPU*cpu)
		}
		return v
	}
	return DiskLimits{
		ReadIOPS:  capped(p.ReadIOPSPerUnit, p.MaxReadIOPS, p.VMIOPSPerCPU),
		WriteIOPS: capped(p.WriteIOPSPerUnit, p.MaxWriteIOPS, p.VMIOPSPerCPU),
		ReadMBps:  capped(p.ReadMBpsPerUnit, p.MaxReadMBps, p.VMMBpsPerCPU),
		WriteMBps: capped(p.WriteMBpsPerUnit, p.MaxWriteMBps, p.VMMBpsPerCPU),
	}
}

// DiskLimitsOf is a disk's limits on this provider (zero when the type is
// unknown or carries no performance data).
func (p Provider) DiskLimitsOf(diskType string, gb, cpu int) DiskLimits {
	for _, d := range p.DiskTypes {
		if d.ID == diskType {
			return d.Perf.Limits(gb, cpu)
		}
	}
	return DiskLimits{}
}
