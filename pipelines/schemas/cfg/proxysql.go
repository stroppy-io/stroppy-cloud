package cfg

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// cfg.proxysql.cnf@2 — ProxySQL 2.6/2.7 configuration file.
//
// doc: https://github.com/sysown/proxysql/blob/v2.7.0/etc/proxysql.cnf
// doc: https://github.com/sysown/proxysql/blob/v2.7.0/lib/ProxySQL_Config.cpp (the cnf parser: the definitive key list)
// doc: https://proxysql.com/documentation/global-variables/mysql-variables/
// doc: https://proxysql.com/documentation/main-runtime/mysql-tables/
//
// Two things about this file drive the schema:
//
//   - the syntax is libconfig, not INI: quoted strings, `{ }` groups and
//     `( )` record lists, so every repeated block (servers, users, query
//     rules, hostgroups) is assembled in a Computed string;
//   - ProxySQL reads the file only to find `datadir`; if <datadir>/proxysql.db
//     already exists the rest is ignored. The runner must start proxysql with
//     --initial (or on a clean datadir) for this file to take effect.

// psDef renders `(("k" in s) ? string(s.k) : "<def>")` for a list element.
func psDef(key, def string) string {
	return `((` + quoteCEL(key) + ` in s) ? string(s.` + key + `) : "` + def + `")`
}

// Proxysql2 is cfg.proxysql.cnf@2.
//
//nolint:funlen // one flat schema definition
func Proxysql2() *schemapb.Schema {
	return schemapb.NewSchema(ids.Cfg("proxysql.cnf", 2)).
		Descr("ProxySQL 2.6/2.7 configuration rendered to /etc/proxysql.cnf (only read on a clean datadir or with --initial).").
		Strict().Coerce().
		Fields(
			// ---- files ---------------------------------------------------
			// doc: etc/proxysql.cnf — datadir
			schemapb.Str("datadir").Title("Data directory").Group("Files").
				Desc("Where proxysql.db lives. If that database already exists this file is ignored beyond this key.").
				MinLen(1).MaxLen(255).Default("/var/lib/proxysql"),
			// doc: etc/proxysql.cnf — errorlog
			schemapb.Str("errorlog").Title("Error log").Group("Files").
				Desc("Path of the ProxySQL error log.").
				MinLen(1).MaxLen(255).Default("/var/lib/proxysql/proxysql.log"),

			// ---- admin_variables ------------------------------------------
			// doc: proxysql.com/documentation/global-variables/admin-variables/#admin-admin_credentials
			schemapb.Str("admin_credentials").Title("Admin credentials").Group("Admin").
				Desc("user:password pairs for the admin interface (6032). Change it — the default admin:admin is only reachable from localhost.").
				Secret().MinLen(3).MaxLen(255).Default("admin:admin"),
			// doc: .../admin-variables/#admin-mysql_ifaces
			schemapb.Str("admin_mysql_ifaces").Title("Admin interfaces").Group("Admin").
				Desc("host:port list the admin interface listens on, semicolon-separated.").
				MinLen(1).MaxLen(255).Default("0.0.0.0:6032"),

			// doc: https://proxysql.com/documentation/prometheus-exporter/
			trueFalse("restapi_enabled", "false").Title("Metrics endpoint").Group("Admin").
				Desc("Expose native Prometheus metrics through the REST API listener."),
			schemapb.Int64("restapi_port").Title("Metrics port").Group("Admin").
				Desc("REST API and Prometheus listener port.").Gte(1).Lte(65535).Default(6070),

			// ---- mysql_variables -------------------------------------------
			// doc: .../mysql-variables/#mysql-interfaces
			schemapb.Str("interfaces").Title("Client interfaces").Group("MySQL").
				Desc("host:port list the SQL proxy listens on; not changeable at runtime.").
				MinLen(1).MaxLen(255).Default("0.0.0.0:6033"),
			// doc: .../mysql-variables/#mysql-threads
			schemapb.Int64("threads").Title("Worker threads").Group("MySQL").
				Desc("Worker threads handling client traffic; not changeable at runtime. Size it to the CPU count.").
				Gte(1).Lte(1024).Default(4),
			// doc: .../mysql-variables/#mysql-max_connections
			schemapb.Int64("max_connections").Title("Max frontend connections").Group("MySQL").
				Desc("Maximum client connections to the proxy (compiled default 10000, the shipped sample sets 2048).").
				Gte(1).Lte(1000000).Default(2048),
			// doc: .../mysql-variables/#mysql-default_query_delay
			schemapb.Int64("default_query_delay").Title("Default query delay").Group("MySQL").
				Desc("Artificial delay before forwarding a query; a throttling knob, 0 in normal use.").
				Unit("ms").Gte(0).Lte(3600000).Default(0),
			// doc: .../mysql-variables/#mysql-default_query_timeout
			schemapb.Int64("default_query_timeout").Title("Default query timeout").Group("MySQL").
				Desc("Timeout for a query with no rule-level timeout.").
				Unit("ms").Gte(1000).Lte(864000000).Default(36000000),
			// doc: .../mysql-variables/#mysql-connect_timeout_server
			schemapb.Int64("connect_timeout_server").Title("Backend connect timeout").Group("MySQL").
				Desc("Timeout of a single connection attempt to a backend.").
				Unit("ms").Gte(100).Lte(600000).Default(3000),
			// doc: .../mysql-variables/#mysql-poll_timeout
			schemapb.Int64("poll_timeout").Title("Poll timeout").Group("MySQL").
				Desc("Event-loop poll timeout.").Unit("ms").Gte(10).Lte(60000).Default(2000),
			// doc: .../mysql-variables/#mysql-stacksize
			schemapb.Int64("stacksize").Title("Thread stack size").Group("MySQL").
				Desc("Stack size of each worker thread; not changeable at runtime.").
				Unit("B").Gte(262144).Lte(67108864).Default(1048576),
			// doc: .../mysql-variables/#mysql-server_version
			schemapb.Str("server_version").Title("Announced server version").Group("MySQL").
				Desc("Version string the proxy reports to clients; set it to the real backend version so drivers pick the right protocol.").
				MinLen(1).MaxLen(64).Default("8.0.36"),
			// doc: .../mysql-variables/#mysql-default_schema
			schemapb.Str("default_schema").Title("Default schema").Group("MySQL").
				Desc("Schema used when a client connects without one.").
				MinLen(1).MaxLen(64).Default("information_schema"),
			// doc: .../mysql-variables/#mysql-default_charset
			schemapb.Str("default_charset").Title("Default charset").Group("MySQL").
				Desc("Character set assumed for clients that do not announce one.").
				MinLen(1).MaxLen(32).Default("utf8mb4"),
			// doc: https://proxysql.com/documentation/global-variables/mysql-variables/#mysql-default_collation_connection
			schemapb.Str("default_collation_connection").Title("Default collation").Group("MySQL").
				Desc("Client handshake collation; must match default_charset. The default matches Stroppy's utf8mb4 charset instead of the upstream utf8 collation.").
				MinLen(1).MaxLen(64).Default("utf8mb4_general_ci"),
			// doc: .../mysql-variables/#mysql-sessions_sort
			trueFalse("sessions_sort", "true").Title("Sort sessions").Group("MySQL").
				Desc("Sort sessions by connection id to improve cache locality across threads."),
			// doc: .../mysql-variables/#mysql-commands_stats
			trueFalse("commands_stats", "true").Title("Command statistics").Group("MySQL").
				Desc("Collect per-command counters in stats_mysql_commands_counters."),

			// ---- monitor ----------------------------------------------------
			// doc: proxysql.com/documentation/global-variables/mysql-monitor-variables/
			schemapb.Str("monitor_username").Title("Monitor user").Group("Monitor").
				Desc("Account the monitor module uses for its health checks; needs USAGE and REPLICATION CLIENT.").
				MinLen(1).MaxLen(64).Default("monitor"),
			schemapb.Str("monitor_password").Title("Monitor password").Group("Monitor").
				Desc("Password of that account; filled by the server.").
				Secret().MaxLen(255).Nullable(),
			// doc: .../mysql-monitor-variables/#mysql-monitor_history
			schemapb.Int64("monitor_history").Title("Monitor history").Group("Monitor").
				Desc("How long monitor check results are kept.").
				Unit("ms").Gte(1000).Lte(86400000).Default(600000),
			// doc: .../mysql-monitor-variables/#mysql-monitor_connect_interval
			schemapb.Int64("monitor_connect_interval").Title("Connect check interval").Group("Monitor").
				Desc("How often the monitor opens a fresh connection to each backend.").
				Unit("ms").Gte(100).Lte(3600000).Default(60000),
			// doc: .../mysql-monitor-variables/#mysql-monitor_ping_interval
			schemapb.Int64("monitor_ping_interval").Title("Ping interval").Group("Monitor").
				Desc("How often the monitor pings each backend.").
				Unit("ms").Gte(100).Lte(3600000).Default(10000),
			// doc: .../mysql-monitor-variables/#mysql-monitor_read_only_interval
			schemapb.Int64("monitor_read_only_interval").Title("read_only check interval").Group("Monitor").
				Desc("How often read_only is polled — this is what moves a server between the writer and reader hostgroups.").
				Unit("ms").Gte(100).Lte(3600000).Default(1500),
			// doc: .../mysql-monitor-variables/#mysql-monitor_read_only_timeout
			schemapb.Int64("monitor_read_only_timeout").Title("read_only check timeout").Group("Monitor").
				Desc("Timeout of one read_only check.").Unit("ms").Gte(50).Lte(60000).Default(500),
			// doc: .../mysql-variables/#mysql-ping_interval_server_msec
			schemapb.Int64("ping_interval_server_msec").Title("Connection-pool ping interval").Group("Monitor").
				Desc("How often idle pooled backend connections are pinged.").
				Unit("ms").Gte(100).Lte(3600000).Default(120000),
			// doc: .../mysql-variables/#mysql-ping_timeout_server
			schemapb.Int64("ping_timeout_server").Title("Connection-pool ping timeout").Group("Monitor").
				Desc("Timeout of one such ping.").Unit("ms").Gte(50).Lte(60000).Default(500),

			// ---- users ---------------------------------------------------------
			// doc: proxysql.com/documentation/main-runtime/mysql-tables/#mysql_users
			schemapb.Str("username").Title("Application user").Group("Users").
				Desc("Frontend account clients authenticate with; must exist on the backends with the same password.").
				MinLen(1).MaxLen(64).Default("stroppy"),
			schemapb.Str("password").Title("Application password").Group("Users").
				Desc("Password of that account; filled by the server.").
				Secret().MaxLen(255).Nullable(),
			schemapb.Int64("user_max_connections").Title("Per-user max connections").Group("Users").
				Desc("Frontend connection limit for that user.").Gte(1).Lte(1000000).Default(10000),
			// doc: .../mysql-tables/#mysql_users transaction_persistent (default 1)
			schemapb.Int64("transaction_persistent").Title("Transaction persistent").Group("Users").
				Desc("1 pins a session to one hostgroup for the whole transaction — required for correctness with read/write split.").
				In(0, 1).Default(1),

			// ---- hostgroups ------------------------------------------------------
			schemapb.Choice("topology").Title("Backend topology").Group("Hostgroups").
				Desc("Which hostgroup manager block is rendered: async/semisync replication (read_only based), group replication, Galera, or none (static hostgroups only).").
				Opt(schemapb.StrV("replication"), "asynchronous / semisync replication").
				Opt(schemapb.StrV("group_replication"), "group replication").
				Opt(schemapb.StrV("galera"), "Galera").
				Opt(schemapb.StrV("none"), "static hostgroups").
				Default(schemapb.StrV("replication")),
			schemapb.Int64("writer_hostgroup").Title("Writer hostgroup").Group("Hostgroups").
				Desc("Hostgroup id receiving writes.").Gte(0).Lte(1000000).Default(10),
			schemapb.Int64("reader_hostgroup").Title("Reader hostgroup").Group("Hostgroups").
				Desc("Hostgroup id receiving reads.").Gte(0).Lte(1000000).Default(20),
			schemapb.Int64("backup_writer_hostgroup").Title("Backup writer hostgroup").Group("Hostgroups").
				Desc("Group Replication / Galera: members that could become primary.").
				Gte(0).Lte(1000000).Default(30),
			schemapb.Int64("offline_hostgroup").Title("Offline hostgroup").Group("Hostgroups").
				Desc("Group Replication / Galera: members that left or fell too far behind.").
				Gte(0).Lte(1000000).Default(40),
			// doc: .../mysql-tables/#mysql_replication_hostgroups check_type
			schemapb.Choice("check_type").Title("Replication check type").Group("Hostgroups").
				Desc("Which read_only flavor decides writer vs reader. Only these three are settable from the config file — the combined forms need the admin interface.").
				Opt(schemapb.StrV("read_only"), "read_only").
				Opt(schemapb.StrV("innodb_read_only"), "innodb_read_only").
				Opt(schemapb.StrV("super_read_only"), "super_read_only").
				Default(schemapb.StrV("read_only")),
			schemapb.Int64("max_writers").Title("Max writers").Group("Hostgroups").
				Desc("Group Replication / Galera: how many members may sit in the writer hostgroup at once.").
				Gte(1).Lte(64).Default(1),
			// doc: .../mysql-tables/#mysql_group_replication_hostgroups writer_is_also_reader
			schemapb.Int64("writer_is_also_reader").Title("Writer is also reader").Group("Hostgroups").
				Desc("0 = writer stays out of the reader pool, 1 = writer also reads, 2 = only backup writers read.").
				In(0, 1, 2).Default(0),
			schemapb.Int64("max_transactions_behind").Title("Max transactions behind").Group("Hostgroups").
				Desc("Group Replication / Galera: lag in transactions before a reader is shunned; 0 disables the check.").
				Gte(0).Lte(1000000).Default(0),

			// ---- query rules --------------------------------------------------------
			trueFalse("read_write_split", "true").Title("Read/write split").Group("Query rules").
				Desc("Render the two default rules: `SELECT ... FOR UPDATE` to the writer, every other `SELECT` to the reader."),

			// ---- cluster-filled -------------------------------------------------------
			schemapb.List("mysql_servers",
				schemapb.Object("",
					schemapb.Str("address").Required().MinLen(1).MaxLen(255).
						Title("Address").Desc("Backend host or IP."),
					schemapb.Int64("port").Gte(1).Lte(65535).Default(3306).
						Title("Port").Desc("Backend port."),
					schemapb.Int64("hostgroup").Required().Gte(0).Lte(1000000).
						Title("Hostgroup").Desc("Initial hostgroup id — writer_hostgroup for the primary, reader_hostgroup for replicas."),
					// doc: https://proxysql.com/documentation/main-runtime/#mysql_servers
					schemapb.Int64("use_ssl").In(0, 1).Default(0).
						Title("Backend TLS").Desc("Use TLS for backend connections; required for fresh caching_sha2_password authentication."),
					schemapb.Int64("weight").Gte(1).Lte(10000000).Default(1).
						Title("Weight").Desc("Relative share of traffic inside its hostgroup."),
					schemapb.Int64("max_connections").Gte(1).Lte(1000000).Default(1000).
						Title("Max connections").Desc("Backend connection-pool limit for this server."),
				).Strict(),
			).Title("Backends").Group("Cluster").
				Desc("The mysql_servers records; filled by the server from topology.").
				MaxItems(64).Nullable(),

			customField().MaxEntries(64),

			// ---- rendered blocks --------------------------------------------------------
			schemapb.Computed("servers_block",
				`(("mysql_servers" in root) && size(root.mysql_servers) > 0) ? ("mysql_servers =\n(\n" + root.mysql_servers.map(s,
					"    { address=\"" + `+psDef("address", "")+` + "\", port=" + `+psDef("port", "3306")+` +
					", hostgroup=" + `+psDef("hostgroup", "0")+` + ", weight=" + `+psDef("weight", "1")+` +
					", max_connections=" + `+psDef("max_connections", "1000")+` + ", use_ssl=" + `+psDef("use_ssl", "0")+` + " }").join(",\n") + "\n)\n") : ""`).
				Result(schemapb.ResultString).Group("Rendered").
				Title("mysql_servers block").Desc("The backend list in libconfig record syntax."),

			schemapb.Computed("users_block",
				`"mysql_users =\n(\n    { username=\"" + string(root.username) + "\", password=\"" +
					(("password" in root) ? string(root.password) : "") +
					"\", default_hostgroup=" + string(root.writer_hostgroup) +
					", transaction_persistent=" + string(root.transaction_persistent) +
					", max_connections=" + string(root.user_max_connections) + ", active=1 }\n)\n"`).
				Result(schemapb.ResultString).Group("Rendered").
				Title("mysql_users block").Desc("The single application account, routed to the writer hostgroup by default."),

			schemapb.Computed("query_rules_block",
				`(root.read_write_split == "true") ? ("mysql_query_rules =\n(\n" +
					"    { rule_id=1, active=1, match_digest=\"^SELECT .* FOR UPDATE\", destination_hostgroup=" + string(root.writer_hostgroup) + ", apply=1 },\n" +
					"    { rule_id=2, active=1, match_digest=\"^SELECT\", destination_hostgroup=" + string(root.reader_hostgroup) + ", apply=1 }\n)\n") : ""`).
				Result(schemapb.ResultString).Group("Rendered").
				Title("mysql_query_rules block").Desc("The read/write split rules; order matters — FOR UPDATE has to be matched first."),

			schemapb.Computed("hostgroups_block",
				`(root.topology == "replication") ? ("mysql_replication_hostgroups =\n(\n    { writer_hostgroup=" + string(root.writer_hostgroup) +
					", reader_hostgroup=" + string(root.reader_hostgroup) +
					", check_type=\"" + string(root.check_type) + "\" }\n)\n")
				: ((root.topology in ["group_replication", "galera"]) ? ("mysql_" + root.topology + "_hostgroups =\n(\n    { writer_hostgroup=" + string(root.writer_hostgroup) +
					", backup_writer_hostgroup=" + string(root.backup_writer_hostgroup) +
					", reader_hostgroup=" + string(root.reader_hostgroup) +
					", offline_hostgroup=" + string(root.offline_hostgroup) +
					", active=1, max_writers=" + string(root.max_writers) +
					", writer_is_also_reader=" + string(root.writer_is_also_reader) +
					", max_transactions_behind=" + string(root.max_transactions_behind) + " }\n)\n") : "")`).
				Result(schemapb.ResultString).Group("Rendered").
				Title("Hostgroup manager block").Desc("Replication, Group Replication or Galera hostgroup manager, selected by topology."),

			schemapb.Computed("custom_rendered",
				`("custom" in root) ? root.custom.map(k, "    " + k + "=" + string(root.custom[k])).join("\n") : ""`).
				Result(schemapb.ResultString).Group("Custom").
				Title("Rendered extra mysql_variables").
				Desc("The `custom` map joined into extra keys inside the mysql_variables block; values must already carry libconfig quoting."),
		).
		Rules(
			schemapb.Rule(`!("writer_hostgroup" in root) || !("reader_hostgroup" in root) || int(root.writer_hostgroup) != int(root.reader_hostgroup)`,
				"writer_hostgroup and reader_hostgroup must differ").ID("hostgroups-distinct"),
			schemapb.Rule(`!(root.topology in ["group_replication", "galera"]) || (root.backup_writer_hostgroup != root.writer_hostgroup && root.backup_writer_hostgroup != root.reader_hostgroup && root.backup_writer_hostgroup != root.offline_hostgroup && root.offline_hostgroup != root.writer_hostgroup && root.offline_hostgroup != root.reader_hostgroup && root.reader_hostgroup > 0)`,
				"Group Replication and Galera need four distinct hostgroups, with a positive reader hostgroup").ID("gr-hostgroups-distinct"),
			schemapb.Rule(`!("monitor_read_only_timeout" in root) || !("monitor_read_only_interval" in root) || int(root.monitor_read_only_timeout) < int(root.monitor_read_only_interval)`,
				"monitor_read_only_timeout must be smaller than monitor_read_only_interval").ID("read-only-timeout"),
			customKeyRule(),
		).
		Template("conf", `# ProxySQL — generated by stroppy, do not edit by hand
# Only parsed on a clean datadir or with --initial.
datadir="{{{values.datadir}}}"
errorlog="{{{values.errorlog}}}"

admin_variables=
{
    admin_credentials="{{{values.admin_credentials}}}"
    mysql_ifaces="{{{values.admin_mysql_ifaces}}}"
    restapi_enabled={{{values.restapi_enabled}}}
    restapi_port={{{values.restapi_port}}}
}

mysql_variables=
{
    interfaces="{{{values.interfaces}}}"
    threads={{{values.threads}}}
    max_connections={{{values.max_connections}}}
    stacksize={{{values.stacksize}}}
    poll_timeout={{{values.poll_timeout}}}
    default_query_delay={{{values.default_query_delay}}}
    default_query_timeout={{{values.default_query_timeout}}}
    connect_timeout_server={{{values.connect_timeout_server}}}
    server_version="{{{values.server_version}}}"
    default_schema="{{{values.default_schema}}}"
    default_charset="{{{values.default_charset}}}"
    default_collation_connection="{{{values.default_collation_connection}}}"
    sessions_sort={{{values.sessions_sort}}}
    commands_stats={{{values.commands_stats}}}
    monitor_username="{{{values.monitor_username}}}"
    monitor_password="{{{values.monitor_password}}}"
    monitor_history={{{values.monitor_history}}}
    monitor_connect_interval={{{values.monitor_connect_interval}}}
    monitor_ping_interval={{{values.monitor_ping_interval}}}
    monitor_read_only_interval={{{values.monitor_read_only_interval}}}
    monitor_read_only_timeout={{{values.monitor_read_only_timeout}}}
    ping_interval_server_msec={{{values.ping_interval_server_msec}}}
    ping_timeout_server={{{values.ping_timeout_server}}}
{{{values.custom_rendered}}}
}

{{{values.servers_block}}}
{{{values.users_block}}}
{{{values.query_rules_block}}}
{{{values.hostgroups_block}}}`).
		MustBuild()
}
