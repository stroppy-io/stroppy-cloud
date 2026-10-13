package dbparams

import schemapb "github.com/gopherex/schemapb/go/schemapb"

// All returns every db.<kind>.params schema of this package, built fresh.
// The order matches the catalog listing; schemas.All() sorts by public id.
func All() []*schemapb.Schema {
	return []*schemapb.Schema{
		Postgres(),
		Orioledb(),
		MySQL(),
		MariaDB(),
		Picodata(),
		Ydb(),
		YdbManaged(),
		Cockroach(),
		External(),
		Noop(),
		PgNoop(),
		Custom(),
	}
}
