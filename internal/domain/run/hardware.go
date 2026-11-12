package run

import (
	"fmt"
	"strings"
)

// MachineGroup is machines of one shape in a stand: size, platform and data
// disk type, since a number is comparable only on the same hardware.
type MachineGroup struct {
	Count    int
	CPU      int
	MemoryGB int
	Platform string // provider platform or instance type; empty when the snapshot lacks it
	DiskType string // data disk type; empty for a machine without a data disk
}

// ParseHardware reads the hardware key the rating query builds for a stand
// without its runner: groups of equal machines, biggest first, as
// "{count}x{cpu}c{memory}g:{platform}:{disk}" joined by "+"
// ("2x4c16g:standard-v3:network-ssd+1x2c4g:standard-v3:"). A part that does
// not parse is skipped: the key is ours, so that only happens on a stand
// with no machines (an empty key).
func ParseHardware(key string) []MachineGroup {
	out := []MachineGroup{}
	for part := range strings.SplitSeq(key, "+") {
		shape, rest, _ := strings.Cut(part, ":")
		g := MachineGroup{}
		g.Platform, g.DiskType, _ = strings.Cut(rest, ":")
		if _, err := fmt.Sscanf(shape, "%dx%dc%dg", &g.Count, &g.CPU, &g.MemoryGB); err == nil {
			out = append(out, g)
		}
	}
	return out
}
