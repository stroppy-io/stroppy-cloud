package dbparams

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// Custom is db.custom.params@1 — a database the product has no recipe for,
// brought as a Docker image. The stand runs the image on every db machine
// and points the workload at the first one in a protocol stroppy speaks;
// everything the image needs to start (users, flags, cluster peers) comes
// from its env and command. Peers are reachable through placeholders:
// ${ip:db-1} is one machine, ${ips:role:db} all of them.
func Custom() *schemapb.Schema {
	return schemapb.NewSchema(ids.DB("custom", 1)).
		Descr("Your own database image on the stand's machines, reached in a protocol stroppy speaks.").
		Strict().Coerce().
		Fields(
			schemapb.Str("image").Title("Image").Group("Engine").
				Desc("Docker image of the database, with a tag or digest; a private registry login comes from the provider profile.").
				MinLen(1).MaxLen(256).Required().
				Examples(schemapb.StrV("ghcr.io/acme/mydb:1.4.2")),

			schemapb.Choice("protocol").Title("Protocol").Group("Connection").
				Desc("Wire protocol the workload speaks to it; decides the stroppy driver and the shape of the URL.").
				Opt(schemapb.StrV("pg"), "PostgreSQL wire protocol").
				Opt(schemapb.StrV("mysql"), "MySQL / MariaDB").
				Opt(schemapb.StrV("picodata"), "Picodata").
				Opt(schemapb.StrV("ydb_grpc"), "YDB (grpc://)").
				Opt(schemapb.StrV("ydb_grpcs"), "YDB (grpcs://, TLS)").
				Opt(schemapb.StrV("cockroach"), "CockroachDB").
				Default(schemapb.StrV("pg")).Required(),

			schemapb.Int64("port").Title("Port").Group("Connection").
				Desc("Port the database listens on; the container uses the host network, so it is the machine's port too.").
				Gte(1).Lte(65535).Default(5432),

			schemapb.Str("user").Title("User").Group("Connection").
				Desc("Login the workload connects with; the image has to create it, usually through env.").
				MaxLen(128).Default("stroppy"),

			schemapb.Str("password").Title("Password").Group("Connection").
				Desc("Password of that login. Empty connects without one. The stand is throwaway; this is not a secret store.").
				MaxLen(256).Default("stroppy"),

			schemapb.Str("database").Title("Database").Group("Connection").
				Desc("Database the workload opens: a name, or a path for YDB (/Root/db).").
				MaxLen(256).Default("stroppy"),

			schemapb.Str("url").Title("Connection URL").Group("Connection").
				Desc("Full URL instead of the one built from the fields above, for options they can't say. ${ip:role:db} is the first db machine.").
				MaxLen(2048).
				Examples(schemapb.StrV("postgresql://stroppy:stroppy@${ip:role:db}:5432/stroppy?sslmode=disable")),

			schemapb.Int64("nodes").Title("Machines").Group("Topology").
				Desc("Machines running the image; the workload connects to the first, the rest are yours to join through env or command.").
				Gte(1).Lte(16).Default(1),

			schemapb.MapOf("env", schemapb.Str("value").MaxLen(4096)).Title("Environment").Group("Container").
				Desc("Variables of the container: the login above, cluster settings. Values may use ${ip:...} placeholders.").
				MaxEntries(128),

			schemapb.List("cmd", schemapb.Str("").MaxLen(1024)).Title("Command").Group("Container").
				Desc("Arguments replacing the image command; empty keeps the image's own.").
				MaxItems(128),

			schemapb.Str("data_dir").Title("Data directory").Group("Container").
				Desc("Path inside the container to keep on the machine's data disk; empty keeps data in the container.").
				MaxLen(256).
				Examples(schemapb.StrV("/var/lib/postgresql/data")),

			schemapb.Str("healthcheck").Title("Ready check").Group("Container").
				Desc("Shell command run inside the container that exits 0 once the database accepts connections; the workload waits for it. Empty waits only for the container to start.").
				MaxLen(256).
				Examples(schemapb.StrV("pg_isready -U stroppy")),

			// The image's own Prometheus endpoint, scraped like every
			// exporter of the stand. pg and mysql images get an exporter of
			// their protocol besides, logged in with the user above.
			schemapb.Int64("metrics_port").Title("Metrics port").Group("Metrics").
				Desc("Port of the image's Prometheus endpoint; empty when the image has none.").
				Gte(1).Lte(65535).Nullable(),
			schemapb.Str("metrics_path").Title("Metrics path").Group("Metrics").
				Desc("Path of the Prometheus endpoint on the metrics port.").
				MaxLen(256).Default("/metrics"),
		).
		Rules(
			schemapb.Rule(`!(root.protocol in ["ydb_grpc", "ydb_grpcs"]) || root.database.startsWith("/")`,
				"a YDB database is a path, like /Root/stroppy").ID("ydb-database-path"),
		).
		MustBuild()
}
