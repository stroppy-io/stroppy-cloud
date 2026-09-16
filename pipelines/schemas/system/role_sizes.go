package system

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// RoleSizes is test.sizes@1 — the hardware half of a Test: one T-shirt size
// per topology role, with an optional disk override. The roles themselves are
// not enumerated here: which roles exist follows from the database topology,
// so the server checks the keys against it and the schema only checks shape.
//
// The value of the `roles` field is exactly OpenAPI RoleSizes
// (openapi/parts/10-components-common.yaml); schemapb has no free-key map at
// the root of a form, so it is carried one level down.
//
// doc: STROPPY.MD §16.0 ("size per role"), §16.5.
func RoleSizes() *schemapb.Schema {
	return schemapb.NewSchema(ids.Test("sizes", 1)).
		Descr("Per-role machine sizing of a test: a T-shirt size and an optional disk override.").
		Strict().Coerce().DefSchema("machine", Machine()).
		Fields(
			schemapb.Map("roles",
				sizeChoice("size").Title("Size").
					Desc("T-shirt size; the platform size table turns it into a machine.").
					Required(),
				schemapb.Ref("machine", "machine").Title("Machine").Group("Sizing").Desc("Override any hardware field from the selected preset; explicit disks replace the preset disk list."),
				schemapb.Object("disk",
					schemapb.Str("type").Title("Disk type").
						Desc("Provider disk type id; empty keeps the size table's default.").
						Pattern(`^[a-z0-9][a-z0-9-]{0,31}$`),
					schemapb.Int64("gb").Title("Size").Unit("GB").
						Desc("Data disk size; empty keeps the size table's default.").
						Gte(10).Lte(262144),
				).Title("Disk").
					Desc("Overrides the disk the size table would pick; absent means take the default.").
					Strict().Nullable(),
			).Strict().Title("Roles").Group("Sizing").
				Desc("Keys are topology roles (db, db-replica, proxy, runner, coordinator); " +
					"the server validates them against the database topology.").
				MinEntries(1).MaxEntries(16).Required(),
		).
		MustBuild()
}
