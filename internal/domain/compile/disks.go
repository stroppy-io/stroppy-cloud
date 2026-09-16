package compile

import (
	_ "embed"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

//go:embed mount-data.sh
var mountDataScript string

// prepareDataDisks precedes database-specific host preparation. Each role uses
// one size and the same data-disk layout on all of its machines.
func (c *compilation) prepareDataDisks() {
	if c.in.ProviderKind != "yandex" {
		return
	}
	seen := map[string]bool{}
	for _, machine := range c.out.Spec.Machines {
		if seen[machine.Role] || len(machine.Disks) == 0 {
			continue
		}
		seen[machine.Role] = true
		c.out.Spec.HostPrep = append(c.out.Spec.HostPrep, spec.HostPrep{Role: machine.Role, Kind: spec.HostPrepScript, Content: mountDataScript})
	}
}
