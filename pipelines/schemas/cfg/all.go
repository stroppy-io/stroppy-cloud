// Package cfg holds cfg.* schemas: one per software config file the product
// renders into a container of a run.
package cfg

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"
)

// All returns every cfg.* schema, built fresh.
func All() []*schemapb.Schema {
	return []*schemapb.Schema{
		Cockroach24(),
		Cockroach25(),
		Cockroach26(),
		Haproxy2(),
		ExporterMysqld(),
		HostDisks(),
		HostSysctl(),
		DockerContainer(),
		Etcd3(),
		ExporterNode(),
		ExporterPostgres1(),
		OrioledbPostgresqlConf16(),
		OrioledbPostgresqlConf17(),
		OrioledbPostgresqlConf18(),
		PatroniYml3(),
		PatroniYml4(),
		PgHbaConf1(),
		PgbouncerIni1(),
		MariadbCnf1011(),
		MariadbCnf1104(),
		MariadbCnf1108(),
		Maxscale25(),
		MyCnf80(),
		MyCnf84(),
		Picodata25(),
		Picodata26(),
		Picodata261(),
		PostgresqlConf15(),
		PostgresqlConf16(),
		PostgresqlConf17(),
		PostgresqlConf18(),
		Proxysql2(),
		Ydb25(),
		Ydb26(),
	}
}
