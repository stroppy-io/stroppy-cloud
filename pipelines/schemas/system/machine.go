package system

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// Machine exposes the executable VM contract as an optional overlay on a preset.
func Machine() *schemapb.Schema {
	s := patchSchema(runItem("machines"), "name", "role")
	s.Id = ids.Test("machine", 1)
	return s
}
