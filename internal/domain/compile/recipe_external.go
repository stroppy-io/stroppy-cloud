package compile

import (
	"regexp"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// externalRecipe deploys nothing for the database itself; the runner
// measures it from outside with the exporter of its protocol, through the
// same DSN the workload connects with. PostgreSQL and MySQL have one;
// other protocols get the runner's own metrics only.
func externalRecipe(c *compilation) error {
	dsn := c.in.Database.ExternalDSN
	if dsn == "" {
		return nil
	}
	for _, m := range c.machinesOf(topology.RoleRunner) {
		switch c.in.Workload.Protocol { //nolint:exhaustive // only pg and mysql have an exporter
		case catalog.ProtoPg:
			c.add(spec.Container{
				Name: m + "-postgres-exporter", Role: topology.RoleRunner, Machine: m, Image: imagePostgresExport,
				Kind: spec.ContainerKindExporter, Env: map[string]string{"DATA_SOURCE_NAME": dsn}, Cmd: postgresExporterCmd(false),
				Ports:  []spec.Port{{Container: postgresExporterPort, Host: postgresExporterPort}},
				Scrape: "/metrics", Restart: "always",
			})
		case catalog.ProtoMySQL:
			addr, user, password, ok := mysqlDSNParts(dsn)
			if !ok {
				continue
			}
			c.add(spec.Container{
				Name: m + "-mysqld-exporter", Role: topology.RoleRunner, Machine: m, Image: imageMySQLDExporter,
				Kind: spec.ContainerKindExporter, Env: map[string]string{"MYSQLD_EXPORTER_PASSWORD": password},
				Cmd:    mysqldExporterCmd(addr, user, false),
				Ports:  []spec.Port{{Container: mysqldExporterPort, Host: mysqldExporterPort}},
				Scrape: "/metrics", Restart: "always",
			})
		}
	}
	return nil
}

// mysqlDSN is the go-sql-driver form stroppy takes:
// user[:password]@tcp(host:port)/database[?params].
var mysqlDSN = regexp.MustCompile(`^([^:@/]+)(?::([^@]*))?@tcp\(([^)]+)\)`)

func mysqlDSNParts(dsn string) (addr, user, password string, ok bool) {
	m := mysqlDSN.FindStringSubmatch(dsn)
	if m == nil {
		return "", "", "", false
	}
	return m[3], m[1], m[2], true
}
