package system

import schemapb "github.com/gopherex/schemapb/go/schemapb"

// All returns every system.*, test.* and tenant.* schema, built fresh.
func All() []*schemapb.Schema {
	return []*schemapb.Schema{
		Limits(),
		Sizes(),
		StroppyCatalog(),
		RoleSizes(),
		Machine(),
		Runtime(),
		TenantWebhook(),
	}
}
