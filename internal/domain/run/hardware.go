package run

import (
	"fmt"
	"strings"
)

// MachineGroup is machines of one shape in a stand.
type MachineGroup struct {
	Count    int
	CPU      int
	MemoryGB int
}

// ParseHardware reads the hardware key the rating query builds for a stand
// without its runner: groups of equal machines, biggest first, as
// "{count}x{cpu}c{memory}g" joined by "+" ("2x4c16g+1x2c4g"). A part that
// does not parse is skipped: the key is ours, so that only happens on a
// stand with no machines (an empty key).
func ParseHardware(key string) []MachineGroup {
	out := []MachineGroup{}
	for part := range strings.SplitSeq(key, "+") {
		var g MachineGroup
		if _, err := fmt.Sscanf(part, "%dx%dc%dg", &g.Count, &g.CPU, &g.MemoryGB); err == nil {
			out = append(out, g)
		}
	}
	return out
}
