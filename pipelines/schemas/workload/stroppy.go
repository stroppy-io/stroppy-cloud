package workload

import (
	"time"

	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// segmentDef is the local $defs key the segment schema is registered under.
const segmentDef = "segment"

// VersionPattern is a stroppy build id: a release (6.0.0, 6.1.0-rc.1) or a
// nightly build of one commit (nightly-24898a8) — the form `stroppy version`
// prints and the catalog resolves to an image.
// doc: stroppy CHANGELOG 6.0.0 (#144) — version reporting for images,
// nightly artifacts and release archives.
const VersionPattern = `^(\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?|nightly-[0-9a-f]{7,40})$`

// preV6Pattern matches releases before 6.0.0, which the platform does not run.
const preV6Pattern = `^[0-5][.]`

// Stroppy is workload.stroppy@1 — the root of a Workload library record: which
// stroppy build runs, over which wire protocol, with which segments, driver
// options and machine baseline.
//
// doc: OpenAPI Protocol enum (openapi/parts/40-catalog.yaml) — the JSON shape
// of `protocol` must stay identical on both doors.
func Stroppy() *schemapb.Schema {
	return schemapb.NewSchema(ids.Workload("stroppy", 1)).
		Descr("A stroppy workload: build, protocol, load segments, driver options and baseline.").
		Strict().Coerce().
		DefSchema(segmentDef, Segment()).
		Fields(
			// The set of usable builds comes from system.stroppy_catalog@1;
			// here only the shape and the v6 floor are checked.
			schemapb.Str("stroppy_version").Title("Stroppy version").Group("Workload").
				Desc("Stroppy build to run: a release (6.0.0) or a nightly of one commit (nightly-<sha>); must exist in the platform catalog. 6.0.0 is the minimum.").
				Pattern(VersionPattern).MaxLen(64).Required().
				Rules(schemapb.Rule(`!this.matches("`+preV6Pattern+`")`, "stroppy releases before 6.0.0 are not supported").ID("min-v6")).
				Examples(schemapb.StrV("6.0.0"), schemapb.StrV("nightly-24898a8")),

			// doc: stroppy `help drivers` — driverType postgres | mysql |
			// picodata | ydb | noop | csv. The product-level protocol splits ydb
			// by transport (grpc/grpcs) and adds cockroach, which speaks the
			// postgres wire protocol. csv is not a benchmark target.
			schemapb.Choice("protocol").Title("Protocol").Group("Workload").
				Desc("Wire protocol stroppy talks to the database with.").
				Opt(schemapb.StrV("pg"), "PostgreSQL").
				Opt(schemapb.StrV("mysql"), "MySQL / MariaDB").
				Opt(schemapb.StrV("picodata"), "Picodata (pgproto)").
				Opt(schemapb.StrV("ydb_grpc"), "YDB (grpc)").
				Opt(schemapb.StrV("ydb_grpcs"), "YDB (grpcs, TLS)").
				Opt(schemapb.StrV("cockroach"), "CockroachDB (pgproto)").
				Opt(schemapb.StrV("noop"), "Noop (framework ceiling)").
				Required(),

			schemapb.List("segments",
				schemapb.Ref("", segmentDef),
			).Title("Segments").Group("Segments").
				Desc("Ordered stroppy invocations; each one is a phase of the run.").
				MinItems(1).MaxItems(64).Required(),

			// doc: stroppy `help drivers` DRIVER OPTIONS — the protocol-neutral
			// keys of drivers.0 in stroppy-config.json.
			schemapb.Object("driver",
				// doc: `stroppy probe` drivers[].insert_methods; columnar is
				// postgres/ydb/noop only.
				schemapb.Choice("default_insert_method").Title("Insert method").
					Desc("Fallback for load requests that leave their method unset (defaultInsertMethod); workloads normally choose themselves.").
					Opt(schemapb.StrV("native"), "Native (COPY / BulkUpsert)").
					Opt(schemapb.StrV("columnar"), "Columnar").
					Opt(schemapb.StrV("plain_bulk"), "Plain bulk INSERT").
					Opt(schemapb.StrV("plain_query"), "Plain single-row INSERT").
					Nullable(),
				schemapb.Int64("bulk_size").Title("Bulk size").Unit("rows").
					Desc("Rows per bulk INSERT statement (bulkSize).").
					Gte(1).Lte(1000000).Default(2500),
				driverPool("pool", "Connection pool", poolFields()...),
				driverPool("postgres", "PostgreSQL driver", postgresFields()...),
				driverPool("sql", "SQL driver", sqlFields()...),

				schemapb.Object("insert_progress",
					schemapb.Bool("enabled").Title("Enabled").Desc("Explicit insertProgress.enabled override; unset uses the mode.").Nullable(),
					schemapb.Choice("mode").Title("Mode").
						Desc("insertProgress.mode: where load progress goes.").
						Opt(schemapb.StrV("off"), "Off").
						Opt(schemapb.StrV("log"), "Log").
						Opt(schemapb.StrV("metrics"), "Metrics").
						Opt(schemapb.StrV("both"), "Log and metrics").
						Default(schemapb.StrV("both")),
					schemapb.Duration("interval").Title("Interval").
						Desc("insertProgress.interval — progress cadence.").Gt(0).Lte(time.Hour).Default(10*time.Second),
					schemapb.Duration("stall_after").Title("Stall after").
						Desc("insertProgress.stallAfter — warn when no rows moved for this long.").Gt(0).Lte(24*time.Hour).Default(60*time.Second),
				).Title("Load progress").Desc("insertProgress.* — load progress reporting.").Strict(),
			).Title("Driver").Group("Driver").
				Desc("Protocol-neutral stroppy driver options.").Strict(),

			// doc: stroppy `help drivers` TLS / Authentication options + the
			// URL parameters of each wire protocol.
			schemapb.OneOf("connection", "kind").Title("Connection").Group("Driver").
				Desc("Protocol-specific connection options; the kind must match the protocol.").
				Variant("pg",
					// doc: postgresql.org/docs/current/libpq-ssl.html — URL sslmode=.
					schemapb.Choice("sslmode").Title("SSL mode").
						Desc("TLS negotiation mode of the postgres connection (URL sslmode).").
						Opt(schemapb.StrV("disable"), "Disable").
						Opt(schemapb.StrV("require"), "Require").
						Opt(schemapb.StrV("verify-full"), "Verify full").
						Default(schemapb.StrV("disable")),
					schemapb.Str("application_name").Title("Application name").
						Desc("application_name reported to postgres; shows up in pg_stat_activity.").
						MaxLen(64).Default("stroppy"),
					// doc: pkg.go.dev/github.com/jackc/pgx/v5#QueryExecMode;
					// stroppy `help config-file` — postgres.defaultQueryExecMode.
					schemapb.Choice("query_exec_mode").Title("Query exec mode").
						Desc("pgx query execution mode (postgres.defaultQueryExecMode).").
						Opt(schemapb.StrV("cache_statement"), "Cache prepared statements").
						Opt(schemapb.StrV("cache_describe"), "Cache statement descriptions").
						Opt(schemapb.StrV("describe_exec"), "Describe then exec").
						Opt(schemapb.StrV("exec"), "Exec (no cache)").
						Opt(schemapb.StrV("simple_protocol"), "Simple protocol").
						Nullable(),
				).
				Variant("mysql",
					// doc: github.com/go-sql-driver/mysql#tls
					schemapb.Choice("tls").Title("TLS").
						Desc("go-sql-driver TLS mode of the MySQL DSN.").
						Opt(schemapb.StrV("false"), "Off").
						Opt(schemapb.StrV("preferred"), "Preferred").
						Opt(schemapb.StrV("skip-verify"), "On, no verification").
						Opt(schemapb.StrV("true"), "On, verified").
						Default(schemapb.StrV("false")),
					// doc: dev.mysql.com/doc/refman/8.4/en/charset-charsets.html
					schemapb.Str("charset").Title("Charset").
						Desc("Connection character set.").
						Pattern(`^[a-z0-9_]+$`).MaxLen(32).Default("utf8mb4"),
				).
				Variant("ydb",
					// doc: stroppy `help drivers` — grpcs:// turns TLS on;
					// caCertFile / authToken / authUser+authPassword.
					schemapb.Str("ca_cert").Title("CA certificate").
						Desc("PEM of a private CA (caCertFile), when the endpoint is not signed by a public one.").
						MaxLen(1<<16).Secret().Nullable(),
					schemapb.Str("auth_token").Title("Auth token").
						Desc("IAM token passed as authToken.").MaxLen(4096).Secret().Nullable(),
					schemapb.Str("auth_user").Title("User").
						Desc("Static credentials user (authUser).").MaxLen(128).Nullable(),
					schemapb.Str("auth_password").Title("Password").
						Desc("Static credentials password (authPassword).").MaxLen(256).Secret().Nullable(),
					schemapb.Bool("tls_insecure_skip_verify").Title("Skip TLS verification").
						Desc("tlsInsecureSkipVerify — testing only.").Default(false),
				).
				Variant("picodata",
					schemapb.Choice("query_exec_mode").Title("Query execution mode").
						Desc("pgx execution mode. Exec avoids prepared-statement limits and binary parameter incompatibilities in Picodata 25.3 and 26.1.").
						Opt(schemapb.StrV("exec"), "Exec (no cache)").
						Opt(schemapb.StrV("cache_statement"), "Cache prepared statements").
						Opt(schemapb.StrV("cache_describe"), "Cache statement descriptions").
						Opt(schemapb.StrV("describe_exec"), "Describe then exec").
						Nullable(),
				).
				Variant("cockroach",
					// doc: cockroachlabs.com/docs/stable/connection-parameters
					schemapb.Choice("sslmode").Title("SSL mode").
						Desc("TLS negotiation mode of the cockroach (pgwire) connection.").
						Opt(schemapb.StrV("disable"), "Disable").
						Opt(schemapb.StrV("require"), "Require").
						Opt(schemapb.StrV("verify-full"), "Verify full").
						Default(schemapb.StrV("disable")),
					schemapb.Str("application_name").Title("Application name").
						MaxLen(64).Default("stroppy"),
				).
				Variant("noop"),

			// doc: `stroppy baseline --help`
			schemapb.Object("baseline",
				schemapb.Bool("enabled").Title("Measure the runner").
					Desc("Run `stroppy baseline` on the runner machine before the segments: the stroppy ceiling a database run can never exceed.").
					Default(false),
				schemapb.List("tiers",
					schemapb.Choice("").
						Opt(schemapb.StrV("noop"), "noop driver (framework cost)").
						Opt(schemapb.StrV("wire"), "pg-wire against pg-noop on loopback"),
				).Title("Tiers").Desc("Which tiers to run (--tiers); unset = both.").
					MinItems(1).MaxItems(2).Unique(),
				schemapb.Bool("quick").Title("Quick").
					Desc("Shorter phases and a smaller load (--quick).").Default(false),
				schemapb.Int64("vus").Title("Parallel VUs").
					Desc("VU count of the parallel tx phase (--vus); unset = 20.").Gte(1).Lte(4096).Nullable(),
				schemapb.Int64("rows").Title("Load rows").
					Desc("Rows loaded into the probe table (--rows); unset = 250000.").Gte(1000).Lte(100000000).Nullable(),
				schemapb.Duration("duration").Title("Tx phase duration").
					Desc("Duration of each tx phase (--duration); unset = 3s.").Gt(0).Lte(10*time.Minute).Nullable(),
			).Title("Baseline").Group("Baseline").
				Desc("Machine self-check with `stroppy baseline`; its JSON report lands in the run result.").Strict(),
		).
		Rules(
			schemapb.Rule(`root.protocol in ["pg", "mysql", "cockroach", "noop"] || root.segments.all(s, !(s.workload.script in ["tpcc/procs", "tpcb/procs"]))`, "stored-procedure workloads require PostgreSQL, MySQL, CockroachDB or noop").ID("stored-procedure-protocol"),
			schemapb.Rule(`!(root.protocol in ["ydb_grpc", "ydb_grpcs"]) || root.segments.all(s, s.workload.script != "tpcds" || ((! ("query_stream" in s.workload) || s.workload.query_stream == null) && (!("streams" in s.workload) || s.workload.streams == 1)))`, "YDB TPC-DS supports the baked query set only").ID("tpcds-ydb-baked"),
			schemapb.Rule(`root.protocol != "picodata" || root.segments.all(s, s.workload.script != "tpcds" || ("no_steps" in s && "workload" in s.no_steps) || ("steps" in s && size(s.steps) > 0 && !("workload" in s.steps)))`, "Picodata TPC-DS supports loading only; exclude the workload step").ID("tpcds-picodata-load-only"),
			schemapb.Rule(
				`!("connection" in root) || !("protocol" in root) || `+
					`root.connection.kind == (root.protocol.startsWith("ydb") ? "ydb" : root.protocol)`,
				"connection.kind must match the protocol",
			).ID("connection-match-protocol"),
			schemapb.Rule(
				`size(root.segments.map(s, s.name)) == size(root.segments.map(s, s.name).filter(`+
					`n, root.segments.filter(x, x.name == n).size() == 1))`,
				"segment names must be unique",
			).ID("segment-names-unique"),
		).
		MustBuild()
}
