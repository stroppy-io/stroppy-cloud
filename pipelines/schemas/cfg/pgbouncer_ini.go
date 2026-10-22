package cfg

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// oneZero builds a PgBouncer boolean, whose config syntax is 1/0.
func oneZero(name schemapb.FieldName, def string) *schemapb.ChoiceB {
	return schemapb.Choice(name).
		Opt(schemapb.StrV("1"), "on").
		Opt(schemapb.StrV("0"), "off").
		Default(schemapb.StrV(def))
}

// PgbouncerIni1 is cfg.pgbouncer.ini@1 — the PgBouncer connection pooler
// config (PgBouncer 1.23/1.24; the settings below are common to both).
//
// Every default is from https://www.pgbouncer.org/config.html (see the `doc:`
// markers). Three product deviations, each explained in its Desc: listen_addr,
// auth_type and pool_mode.
//
// The [databases] section is cluster-filled: the server knows the primary's
// host and port only when it compiles the RunSpec.
//
//nolint:funlen // one flat key table
func PgbouncerIni1() *schemapb.Schema {
	return schemapb.NewSchema(ids.Cfg("pgbouncer.ini", 1)).
		Descr("pgbouncer.ini for PgBouncer 1.23/1.24 in front of the PostgreSQL primary.").
		Strict().Coerce().
		Fields(
			// ---------------------------------------------------- [databases]
			schemapb.Str("db_name").Title("Pool name").Group("Databases").
				// doc: config.html — [databases] section
				Desc("Name clients connect to; the left-hand side of the [databases] entry.").
				Pattern(`^[A-Za-z_][A-Za-z0-9_]{0,62}$`).Default("stroppy"),
			schemapb.Str("db_host").Title("Primary host").Group("Cluster").
				Desc("Host of the PostgreSQL primary the pool forwards to. Filled by the server from the topology.").
				MinLen(1).MaxLen(255).Nullable(),
			schemapb.Int64("db_port").Title("Primary port").Group("Cluster").
				Desc("Port of the PostgreSQL primary. Filled by the server from the topology.").
				Gte(1).Lte(65535).Default(5432),
			schemapb.Str("db_dbname").Title("Target database").Group("Databases").
				Desc("Database name on the primary; defaults to the pool name when unset.").
				Pattern(`^[A-Za-z_][A-Za-z0-9_]{0,62}$`).Default("stroppy"),
			schemapb.Computed("databases_rendered",
				`("db_host" in root) && root.db_host != ""`+
					` ? root.db_name + " = host=" + root.db_host + " port=" + string(root.db_port)`+
					` + " dbname=" + root.db_dbname : ""`).
				Title("Rendered [databases]").Group("Cluster").
				Desc("The [databases] entry, emitted only once the server has filled db_host.").
				Result(schemapb.ResultString),

			// ---------------------------------------------------- [pgbouncer]
			schemapb.Str("listen_addr").Title("Listen address").Group("Listen").
				// doc: config.html — listen_addr, upstream default unset (localhost only)
				Desc("Addresses PgBouncer binds to. Product default '*': the load generator reaches the pooler over the network, unlike the upstream localhost-only default.").
				MinLen(1).MaxLen(255).Default("*"),
			schemapb.Int64("listen_port").Title("Listen port").Group("Listen").
				// doc: config.html — listen_port, default 6432
				Desc("TCP port PgBouncer listens on.").
				Gte(1).Lte(65535).Default(6432),

			schemapb.Choice("auth_type").Title("Auth type").Group("Authentication").
				// doc: config.html — auth_type, upstream default md5
				Desc("How PgBouncer authenticates clients. Product default scram-sha-256, matching PostgreSQL's own password_encryption default; upstream still defaults to md5.").
				Opt(schemapb.StrV("scram-sha-256"), "scram-sha-256").
				Opt(schemapb.StrV("md5"), "md5").
				Opt(schemapb.StrV("plain"), "plain").
				Opt(schemapb.StrV("trust"), "trust").
				Opt(schemapb.StrV("cert"), "cert").
				Opt(schemapb.StrV("hba"), "hba").
				Opt(schemapb.StrV("any"), "any").
				Default(schemapb.StrV("scram-sha-256")),
			schemapb.Str("auth_file").Title("Auth file").Group("Authentication").
				// doc: config.html — auth_file, unset by default
				Desc("userlist.txt with the name/password pairs PgBouncer authenticates against.").
				Pattern(`^/.+$`).Default("/etc/pgbouncer/userlist.txt"),
			schemapb.Str("auth_user").Title("Auth user").Group("Authentication").
				// doc: config.html — auth_user, unset by default
				Desc("Role auth_query runs as for users missing from auth_file.").
				MinLen(1).MaxLen(63).Nullable(),
			schemapb.Str("auth_query").Title("Auth query").Group("Authentication").
				// doc: config.html — auth_query, default SELECT usename, passwd FROM pg_shadow WHERE usename=$1
				Desc("Query used to fetch a password when auth_user is set.").
				MinLen(1).MaxLen(1024).Nullable(),

			schemapb.Choice("pool_mode").Title("Pool mode").Group("Pooling").
				// doc: config.html — pool_mode, upstream default session
				Desc("When a server connection is returned to the pool. Product default transaction: it is the mode a benchmark pooler is deployed for; upstream defaults to session.").
				Opt(schemapb.StrV("session"), "session").
				Opt(schemapb.StrV("transaction"), "transaction").
				Opt(schemapb.StrV("statement"), "statement").
				Default(schemapb.StrV("transaction")),
			schemapb.Int64("max_client_conn").Title("Max client connections").Group("Pooling").
				// doc: config.html — max_client_conn, default 100
				Desc("Maximum client connections PgBouncer accepts.").
				Gte(1).Lte(1000000).Default(100),
			schemapb.Int64("default_pool_size").Title("Default pool size").Group("Pooling").
				// doc: config.html — default_pool_size, default 20
				Desc("Server connections per user/database pair.").
				Gte(1).Lte(10000).Default(20),
			schemapb.Int64("min_pool_size").Title("Min pool size").Group("Pooling").
				// doc: config.html — min_pool_size, default 0
				Desc("Server connections kept open even when idle; 0 disables.").
				Gte(0).Lte(10000).Default(0),
			schemapb.Int64("reserve_pool_size").Title("Reserve pool size").Group("Pooling").
				// doc: config.html — reserve_pool_size, default 0
				Desc("Extra connections a pool may open once clients have waited reserve_pool_timeout; 0 disables.").
				Gte(0).Lte(10000).Default(0),
			schemapb.Double("reserve_pool_timeout").Title("Reserve pool timeout").Group("Pooling").Unit("s").
				// doc: config.html — reserve_pool_timeout, default 5.0
				Desc("How long a client waits before the reserve pool is used; 0 disables.").
				Gte(0).Lte(3600).Default(5.0),
			schemapb.Int64("max_db_connections").Title("Max DB connections").Group("Pooling").
				// doc: config.html — max_db_connections, default 0 (unlimited)
				Desc("Cap on server connections per database across all pools; 0 is unlimited.").
				Gte(0).Lte(100000).Default(0),
			schemapb.Int64("max_prepared_statements").Title("Max prepared statements").Group("Pooling").
				// doc: config.html — max_prepared_statements, added in 1.21 (default 0), default 200 since 1.24
				Desc("Protocol-level prepared statements PgBouncer tracks per server connection in transaction and statement pooling; 0 turns the support off. Unset keeps the build's default: 200 on 1.24, 0 on 1.23.").
				Gte(0).Lte(100000).Nullable(),
			schemapb.Int64("max_user_connections").Title("Max user connections").Group("Pooling").
				// doc: config.html — max_user_connections, default 0 (unlimited)
				Desc("Cap on server connections per user across all pools; 0 is unlimited.").
				Gte(0).Lte(100000).Default(0),

			schemapb.Double("server_idle_timeout").Title("Server idle timeout").Group("Timeouts").Unit("s").
				// doc: config.html — server_idle_timeout, default 600.0
				Desc("Close a server connection idle longer than this; 0 disables.").
				Gte(0).Lte(86400).Default(600.0),
			schemapb.Double("server_lifetime").Title("Server lifetime").Group("Timeouts").Unit("s").
				// doc: config.html — server_lifetime, default 3600.0
				Desc("Recycle a server connection older than this; 0 disables.").
				Gte(0).Lte(604800).Default(3600.0),
			schemapb.Str("server_reset_query").Title("Server reset query").Group("Timeouts").
				// doc: config.html — server_reset_query, default DISCARD ALL
				Desc("Query run when a server connection is returned to the pool. Upstream default is `DISCARD ALL`, which is only valid in session pooling; the product default is empty because pool_mode defaults to transaction.").
				MaxLen(512).Default(""),
			schemapb.Double("query_wait_timeout").Title("Query wait timeout").Group("Timeouts").Unit("s").
				// doc: config.html — query_wait_timeout, default 120.0
				Desc("Longest a query may wait for a server connection before it is canceled; 0 disables.").
				Gte(0).Lte(86400).Default(120.0),
			schemapb.Double("client_idle_timeout").Title("Client idle timeout").Group("Timeouts").Unit("s").
				// doc: config.html — client_idle_timeout, default 0.0 (disabled)
				Desc("Disconnect a client idle outside a transaction for longer than this; 0 disables.").
				Gte(0).Lte(86400).Default(0),

			schemapb.Str("ignore_startup_parameters").Title("Ignore startup parameters").Group("Compatibility").
				// doc: config.html — ignore_startup_parameters, default empty
				Desc("Startup parameters PgBouncer accepts and ignores, e.g. extra_float_digits,options.").
				MaxLen(512).Default(""),

			oneZero("log_connections", "1").Title("Log connections").Group("Logging").
				// doc: config.html — log_connections, default 1
				Desc("Log each client connection."),
			oneZero("log_disconnections", "1").Title("Log disconnections").Group("Logging").
				// doc: config.html — log_disconnections, default 1
				Desc("Log each client disconnection with its reason."),
			schemapb.Int64("stats_period").Title("Stats period").Group("Logging").Unit("s").
				// doc: config.html — stats_period, default 60
				Desc("Interval between the aggregated statistics log lines.").
				Gte(1).Lte(86400).Default(60),
			schemapb.Str("admin_users").Title("Admin users").Group("Logging").
				// doc: config.html — admin_users, default empty
				Desc("Comma-separated roles allowed to run every command on the admin console.").
				MaxLen(512).Default("postgres"),
			schemapb.Str("stats_users").Title("Stats users").Group("Logging").
				// doc: config.html — stats_users, default empty
				Desc("Comma-separated roles allowed read-only access to the admin console; the exporter uses one of these.").
				MaxLen(512).Default("postgres"),
		).
		Rules(
			schemapb.Rule(`root.pool_mode == "session" || root.server_reset_query == ""`,
				"server_reset_query must be empty in transaction or statement pooling").
				ID("reset-query-vs-pool-mode"),
			schemapb.Rule(`!("auth_query" in root) || ("auth_user" in root)`,
				"auth_query requires auth_user").ID("auth-query-needs-user"),
			schemapb.Rule(`int(root.default_pool_size) <= int(root.max_client_conn)`,
				"default_pool_size must not exceed max_client_conn").ID("pool-vs-clients"),
			schemapb.Rule(`int(root.min_pool_size) <= int(root.default_pool_size)`,
				"min_pool_size must not exceed default_pool_size").ID("min-vs-default-pool"),
		).
		Template("conf", `; pgbouncer.ini — generated by stroppy.
[databases]
{{#values.databases_rendered}}{{{values.databases_rendered}}}
{{/values.databases_rendered}}
[pgbouncer]
listen_addr = {{{values.listen_addr}}}
listen_port = {{{values.listen_port}}}
auth_type = {{{values.auth_type}}}
auth_file = {{{values.auth_file}}}
{{#values.auth_user}}auth_user = {{{.}}}
{{/values.auth_user}}{{#values.auth_query}}auth_query = {{{.}}}
{{/values.auth_query}}pool_mode = {{{values.pool_mode}}}
max_client_conn = {{{values.max_client_conn}}}
default_pool_size = {{{values.default_pool_size}}}
min_pool_size = {{{values.min_pool_size}}}
reserve_pool_size = {{{values.reserve_pool_size}}}
reserve_pool_timeout = {{{values.reserve_pool_timeout}}}
max_db_connections = {{{values.max_db_connections}}}
max_user_connections = {{{values.max_user_connections}}}
{{#values.max_prepared_statements}}max_prepared_statements = {{{.}}}
{{/values.max_prepared_statements}}server_idle_timeout = {{{values.server_idle_timeout}}}
server_lifetime = {{{values.server_lifetime}}}
server_reset_query = {{{values.server_reset_query}}}
query_wait_timeout = {{{values.query_wait_timeout}}}
client_idle_timeout = {{{values.client_idle_timeout}}}
ignore_startup_parameters = {{{values.ignore_startup_parameters}}}
log_connections = {{{values.log_connections}}}
log_disconnections = {{{values.log_disconnections}}}
stats_period = {{{values.stats_period}}}
admin_users = {{{values.admin_users}}}
stats_users = {{{values.stats_users}}}
`).
		MustBuild()
}
