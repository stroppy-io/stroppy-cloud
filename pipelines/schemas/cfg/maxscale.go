package cfg

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// cfg.maxscale.cnf@24 — MariaDB MaxScale 24.02, the newest line with a
// free image on Docker Hub (mariadb/maxscale:24.02.x; 25.x ships only as
// "-trial" images).
//
// LICENSE: MaxScale is under the Business Source License. Its additional
// use grant allows production use with fewer than three database servers;
// a replicated or Galera topology behind it (three servers and more) needs
// a MariaDB subscription for production use. Stroppy deploys it on the
// user's own stand for benchmarking; the parameter's description says so.
//
// doc: https://mariadb.com/docs/maxscale/maxscale-management/deployment/installation-and-configuration/maxscale-configuration-guide
// doc: https://mariadb.com/docs/maxscale/reference/maxscale-monitors/common-monitor-parameters
// doc: https://mariadb.com/docs/maxscale/reference/maxscale-monitors/mariadb-monitor
// doc: https://mariadb.com/docs/maxscale/reference/maxscale-routers/maxscale-readwritesplit
// doc: https://mariadb.com/docs/maxscale/reference/maxscale-servers
// doc: https://mariadb.com/docs/maxscale/reference/maxscale-listeners
// doc: https://mariadb.com/bsl11/ (MaxScale license)
//
// What 24.02 has and is encoded here:
//
//   - the monitor's backend_connect_timeout / backend_read_timeout /
//     backend_write_timeout (25.10 later folded them into backend_timeout);
//     the schema keeps one backend_timeout field and renders all three;
//   - master_failure_mode's default became fail_on_write in 24.02, which is
//     also what transaction_replay requires;
//   - transaction_replay_timeout became a real default (30s) in 24.02 and is
//     modeled so a run does not inherit it silently.
//
// Three details this schema keeps encoding on purpose:
//
//   - a [server] section has NO `protocol` parameter — that belongs to the
//     listener (MaxScale 2.x leftovers still show it on servers);
//   - durations carry a unit suffix (unitless is deprecated since 2.4), so
//     every duration field is an integer plus a rendered suffix;
//   - slave_selection_criteria values are lowercase from 23.08.1 on; the
//     uppercase spellings still work but are deprecated.

// Maxscale24 is cfg.maxscale.cnf@24.
func Maxscale24() *schemapb.Schema { return maxscaleCnf(24) }

//nolint:funlen // one flat schema definition
func maxscaleCnf(major uint64) *schemapb.Schema {
	return schemapb.NewSchema(ids.Cfg("maxscale.cnf", major)).
		Descr("MariaDB MaxScale 24.02 configuration rendered to /etc/maxscale.cnf: one monitor, one readwritesplit service, one listener.").
		Strict().Coerce().
		Fields(
			// ---- [maxscale] ---------------------------------------------
			// doc: maxscale-configuration-guide#threads (default "auto")
			schemapb.Str("threads").Title("Worker threads").Group("MaxScale").
				Desc("Routing worker threads; `auto` means one per CPU core.").
				Pattern(`^(auto|[1-9][0-9]*)$`).MaxLen(8).Default("auto"),
			// doc: maxscale-configuration-guide#admin_host
			schemapb.Str("admin_host").Title("Admin host").Group("MaxScale").
				Desc("Address the REST API / GUI binds to; the default 127.0.0.1 keeps it local.").
				MinLen(1).MaxLen(255).Default("127.0.0.1"),
			// doc: maxscale-configuration-guide#admin_port
			schemapb.Int64("admin_port").Title("Admin port").Group("MaxScale").
				Desc("Port of the REST API and the GUI.").Gte(1).Lte(65535).Default(8989),
			// doc: maxscale-configuration-guide#admin_secure_gui
			trueFalse("admin_secure_gui", "true").Title("Secure GUI").Group("MaxScale").
				Desc("Serve the GUI over HTTPS only; turning it off needs admin_ssl_* to be unset."),
			// doc: maxscale-configuration-guide#syslog (default flipped to false in 22.08)
			trueFalse("syslog", "false").Title("Log to syslog").Group("MaxScale").
				Desc("Also write the log to syslog."),
			// doc: maxscale-configuration-guide#maxlog
			trueFalse("maxlog", "true").Title("Log to maxscale.log").Group("MaxScale").
				Desc("Write MaxScale's own log file."),
			// doc: maxscale-configuration-guide#log_info
			trueFalse("log_info", "false").Title("Info logging").Group("MaxScale").
				Desc("Log at INFO level — useful while bringing a cluster up, noisy under load."),

			// ---- monitor -------------------------------------------------
			// doc: maxscale-configuration-guide — module
			schemapb.Choice("monitor_module").Title("Monitor module").Group("Monitor").
				Desc("mariadbmon for asynchronous/semisync replication, galeramon for a Galera cluster.").
				Opt(schemapb.StrV("mariadbmon"), "mariadbmon (replication)").
				Opt(schemapb.StrV("galeramon"), "galeramon (Galera)").
				Default(schemapb.StrV("mariadbmon")),
			// doc: common-monitor-parameters#user
			schemapb.Str("monitor_user").Title("Monitor user").Group("Monitor").
				Desc("Backend account the monitor uses; needs REPLICATION CLIENT (and more for failover).").
				MinLen(1).MaxLen(64).Default("maxscale_monitor"),
			schemapb.Str("monitor_password").Title("Monitor password").Group("Monitor").
				Desc("Password of that account; filled by the server.").
				Secret().MaxLen(255).Nullable(),
			// doc: common-monitor-parameters#monitor_interval (duration, default 2s, minimum 100ms)
			schemapb.Int64("monitor_interval").Title("Monitor interval").Group("Monitor").
				Desc("How often the monitor polls every backend. Rendered with an explicit ms suffix — unitless durations are deprecated.").
				Unit("ms").Gte(100).Lte(3600000).Default(2000),
			// doc: common-monitor-parameters#backend_connect_timeout,
			// #backend_read_timeout, #backend_write_timeout — default 3s each.
			schemapb.Int64("backend_timeout").Title("Backend timeout").Group("Monitor").
				Desc("Connect, read and write timeout of one monitor check; rendered as backend_connect_timeout, backend_read_timeout and backend_write_timeout.").
				Unit("ms").Gte(1000).Lte(600000).Default(3000),
			// doc: mariadb-monitor#auto_failover
			trueFalse("auto_failover", "false").Title("Auto failover").Group("Monitor").
				Desc("Promote a replica when the primary is lost (mariadbmon only)."),
			// doc: mariadb-monitor#auto_rejoin
			trueFalse("auto_rejoin", "false").Title("Auto rejoin").Group("Monitor").
				Desc("Re-attach an old primary as a replica once it comes back (mariadbmon only)."),
			// doc: mariadb-monitor#enforce_read_only_slaves
			trueFalse("enforce_read_only_slaves", "false").Title("Enforce read_only on replicas").Group("Monitor").
				Desc("Set read_only=1 on any writable replica. The wider enforce_read_only_servers also exists — this is not a rename."),
			// doc: mariadb-monitor#failcount
			schemapb.Int64("failcount").Title("Fail count").Group("Monitor").
				Desc("Consecutive failed checks before the primary is declared down; 0 or 1 means immediately.").
				Gte(0).Lte(100).Default(5),
			// doc: mariadb-monitor#replication_user
			schemapb.Str("replication_user").Title("Replication user").Group("Monitor").
				Desc("Account written into CHANGE MASTER TO during failover/rejoin.").
				MinLen(1).MaxLen(64).Default("maxscale_repl"),
			schemapb.Str("replication_password").Title("Replication password").Group("Monitor").
				Desc("Password of that account; filled by the server.").
				Secret().MaxLen(255).Nullable(),
			// doc: mariadb-monitor#cooperative_monitoring_locks
			schemapb.Choice("cooperative_monitoring_locks").Title("Cooperative monitoring locks").Group("Monitor").
				Desc("Lets several MaxScale instances agree on who may run failover.").
				Opt(schemapb.StrV("none"), "none").
				Opt(schemapb.StrV("majority_of_all"), "majority_of_all").
				Opt(schemapb.StrV("majority_of_running"), "majority_of_running").
				Default(schemapb.StrV("none")),

			// ---- service (readwritesplit) ----------------------------------
			// doc: maxscale-configuration-guide — service user/password
			schemapb.Str("service_user").Title("Service user").Group("Service").
				Desc("Account MaxScale uses to read the backends' user tables.").
				MinLen(1).MaxLen(64).Default("maxscale_service"),
			schemapb.Str("service_password").Title("Service password").Group("Service").
				Desc("Password of that account; filled by the server.").
				Secret().MaxLen(255).Nullable(),
			// doc: maxscale-configuration-guide#enable_root_user
			trueFalse("enable_root_user", "false").Title("Allow root").Group("Service").
				Desc("Let clients log in as root through MaxScale; off by default, the benchmark logs in as root."),
			// doc: maxscale-readwritesplit#master_accept_reads
			trueFalse("master_accept_reads", "false").Title("Primary accepts reads").Group("Service").
				Desc("Send read traffic to the primary as well as the replicas."),
			// doc: maxscale-readwritesplit#causal_reads
			// The upstream default is none; a benchmark reads what other
			// connections wrote (stroppy validates the loaded data on its own
			// connections), and an async replica behind the bulk load fails
			// those checks. global waits for the replica (MASTER_GTID_WAIT,
			// primary after causal_reads_timeout); 24.02 tracks last_gtid on
			// the backends itself.
			schemapb.Choice("causal_reads").Title("Causal reads").Group("Service").
				Desc("Read-your-own-writes guarantee level; anything but none costs a synchronization wait. global: every read sees the writes made through the service before it. Galera supports none and the fast modes only; in front of Galera the server sets none.").
				Opt(schemapb.StrV("none"), "none").
				Opt(schemapb.StrV("local"), "local").
				Opt(schemapb.StrV("global"), "global").
				Opt(schemapb.StrV("fast"), "fast").
				Opt(schemapb.StrV("fast_global"), "fast_global").
				Opt(schemapb.StrV("universal"), "universal").
				Opt(schemapb.StrV("fast_universal"), "fast_universal").
				Default(schemapb.StrV("global")),
			// doc: maxscale-readwritesplit#causal_reads_timeout (duration, default 10s)
			schemapb.Int64("causal_reads_timeout").Title("Causal reads timeout").Group("Service").
				Desc("How long a causal read waits for the replica to catch up before it falls back to the primary.").
				Unit("ms").Gte(100).Lte(600000).Default(10000),
			// doc: maxscale-readwritesplit#transaction_replay
			trueFalse("transaction_replay", "false").Title("Transaction replay").Group("Service").
				Desc("Replay an interrupted transaction on another server. Enabling it forces delayed_retry, master_reconnection and master_failure_mode=fail_on_write."),
			// doc: maxscale-readwritesplit#transaction_replay_max_size (size, default 1 MiB)
			schemapb.Int64("transaction_replay_max_size").Title("Transaction replay max size").Group("Service").
				Desc("Largest transaction MaxScale will buffer for replay.").
				Unit("MB").Gte(1).Lte(1024).Default(1),
			// doc: maxscale-readwritesplit#slave_selection_criteria (lowercase from 23.08.1; uppercase deprecated)
			schemapb.Choice("slave_selection_criteria").Title("Replica selection").Group("Service").
				Desc("How a read is assigned to a replica.").
				Opt(schemapb.StrV("least_current_operations"), "least current operations").
				Opt(schemapb.StrV("adaptive_routing"), "adaptive routing").
				Opt(schemapb.StrV("least_behind_master"), "least behind master").
				Opt(schemapb.StrV("least_router_connections"), "least router connections").
				Opt(schemapb.StrV("least_global_connections"), "least global connections").
				Default(schemapb.StrV("least_current_operations")),
			// doc: maxscale-readwritesplit#max_slave_connections (integer since 2.5; the "100%" form is deprecated)
			schemapb.Int64("max_slave_connections").Title("Max replica connections").Group("Service").
				Desc("How many replicas one session may hold connections to.").
				Gte(0).Lte(1000).Default(255),
			// doc: maxscale-readwritesplit#max_replication_lag (renamed from max_slave_replication_lag in 23.02; second granularity)
			schemapb.Int64("max_replication_lag").Title("Max replication lag").Group("Service").
				Desc("Replicas lagging more than this stop receiving reads; 0 disables the check. Whole seconds only.").
				Unit("s").Gte(0).Lte(86400).Default(0),
			// doc: maxscale-readwritesplit#master_failure_mode — the default
			// became fail_on_write in 24.02 (it was fail_instantly on 23.08).
			schemapb.Choice("master_failure_mode").Title("Primary failure mode").Group("Service").
				Desc("What happens to a session when the primary disappears. 24.02 moved the default to fail_on_write, which is also what transaction_replay requires.").
				Opt(schemapb.StrV("fail_instantly"), "fail_instantly").
				Opt(schemapb.StrV("fail_on_write"), "fail_on_write").
				Opt(schemapb.StrV("error_on_write"), "error_on_write").
				Default(schemapb.StrV("fail_on_write")),
			// doc: maxscale-readwritesplit#transaction_replay_timeout — default 30s since 24.02
			schemapb.Int64("transaction_replay_timeout").Title("Transaction replay timeout").Group("Service").
				Desc("How long MaxScale keeps retrying a replayed transaction before giving up; 0 falls back to the session's own timeouts.").
				Unit("ms").Gte(0).Lte(600000).Default(30000),

			// ---- listener ---------------------------------------------------
			// doc: maxscale-listeners — port/address/protocol
			schemapb.Int64("listener_port").Title("Listener port").Group("Listener").
				Desc("Port clients connect to.").Gte(1).Lte(65535).Default(4006),
			schemapb.Str("listener_address").Title("Listener address").Group("Listener").
				Desc("Address the listener binds to; :: means every interface.").
				MinLen(1).MaxLen(255).Default("::"),
			schemapb.Choice("listener_protocol").Title("Listener protocol").Group("Listener").
				Desc("Client protocol module. This is the only place `protocol` is valid — a [server] section has none.").
				Opt(schemapb.StrV("mariadb"), "mariadb (MariaDBClient)").
				Default(schemapb.StrV("mariadb")),

			// ---- cluster-filled ------------------------------------------------
			schemapb.List("servers",
				schemapb.Object("",
					schemapb.Str("name").Required().MinLen(1).MaxLen(64).
						Title("Section name").Desc("Name of the [server] section, e.g. db-1."),
					schemapb.Str("address").Required().MinLen(1).MaxLen(255).
						Title("Address").Desc("Backend host or IP."),
					schemapb.Int64("port").Gte(1).Lte(65535).Default(3306).
						Title("Port").Desc("Backend port."),
					schemapb.Int64("priority").Gte(0).Lte(1000).Default(0).
						Title("Priority").Desc("galeramon only: lower wins when use_priority is on. Ignored by mariadbmon."),
				).Strict(),
			).Title("Backends").Group("Cluster").
				Desc("The [server] sections plus the servers= lists of the monitor and the service; filled by the server from topology.").
				MaxItems(64).Nullable(),

			customField().MaxEntries(64),

			// ---- rendered blocks -------------------------------------------------
			schemapb.Computed("server_sections",
				`(("servers" in root) && size(root.servers) > 0) ? root.servers.map(s,
					"[" + string(s.name) + "]\ntype=server\naddress=" + string(s.address) +
					"\nport=" + `+psDef("port", "3306")+` + "\npriority=" + `+psDef("priority", "0")+` + "\n").join("\n") : ""`).
				Result(schemapb.ResultString).Group("Rendered").
				Title("Server sections").Desc("One [server] section per backend — with no protocol parameter, which MaxScale does not accept there."),

			schemapb.Computed("server_names",
				`(("servers" in root) && size(root.servers) > 0) ? root.servers.map(s, string(s.name)).join(",") : ""`).
				Result(schemapb.ResultString).Group("Rendered").
				Title("Server names").Desc("Comma-separated section names for the monitor's and the service's servers= lines."),

			schemapb.Computed("monitor_extra",
				`(root.monitor_module == "mariadbmon") ?
					("auto_failover=" + string(root.auto_failover) +
					"\nauto_rejoin=" + string(root.auto_rejoin) +
					"\nenforce_read_only_slaves=" + string(root.enforce_read_only_slaves) +
					"\nfailcount=" + string(root.failcount) +
					"\nreplication_user=" + string(root.replication_user) +
					"\nreplication_password=" + (("replication_password" in root) ? string(root.replication_password) : "") +
					"\ncooperative_monitoring_locks=" + string(root.cooperative_monitoring_locks) + "\n")
					: "use_priority=true\n"`).
				Result(schemapb.ResultString).Group("Rendered").
				Title("Monitor extras").Desc("The mariadbmon-only failover parameters, or galeramon's use_priority."),

			schemapb.Computed("custom_rendered",
				`("custom" in root) ? root.custom.map(k, k + "=" + string(root.custom[k])).join("\n") : ""`).
				Result(schemapb.ResultString).Group("Custom").
				Title("Rendered extra service parameters").
				Desc("The `custom` map joined into extra key=value lines of the [rwsplit-service] section."),
		).
		Rules(
			schemapb.Rule(`!("transaction_replay" in root) || root.transaction_replay != "true" || !("master_failure_mode" in root) || root.master_failure_mode == "fail_on_write"`,
				"transaction_replay forces master_failure_mode=fail_on_write — set it explicitly").ID("replay-failure-mode"),
			schemapb.Rule(`!("monitor_module" in root) || root.monitor_module != "galeramon" || !("auto_failover" in root) || root.auto_failover == "false"`,
				"auto_failover is a mariadbmon parameter; galeramon has no failover").ID("galera-no-failover"),
			schemapb.Rule(`!("causal_reads_timeout" in root) || !("monitor_interval" in root) || int(root.causal_reads_timeout) >= int(root.monitor_interval)`,
				"causal_reads_timeout below monitor_interval makes causal reads fail before the monitor notices anything").ID("causal-timeout"),
			// doc: maxscale-readwritesplit#causal_reads — local/global/universal
			// rely on MASTER_GTID_WAIT, which Galera does not implement; only
			// the fast modes work with Galera, and only from 24.02.5.
			schemapb.Rule(`!("monitor_module" in root) || root.monitor_module != "galeramon" || !("causal_reads" in root) || !(root.causal_reads in ["local", "global", "universal"])`,
				"causal_reads local/global/universal need MASTER_GTID_WAIT, which Galera lacks: use none or a fast mode").ID("galera-causal-reads"),
			customKeyRule(),
		).
		Template("conf", `# MaxScale 24.02 — generated by stroppy, do not edit by hand
[maxscale]
threads={{{values.threads}}}
admin_host={{{values.admin_host}}}
admin_port={{{values.admin_port}}}
admin_secure_gui={{{values.admin_secure_gui}}}
syslog={{{values.syslog}}}
maxlog={{{values.maxlog}}}
log_info={{{values.log_info}}}

{{{values.server_sections}}}
[cluster-monitor]
type=monitor
module={{{values.monitor_module}}}
servers={{{values.server_names}}}
user={{{values.monitor_user}}}
password={{{values.monitor_password}}}
monitor_interval={{{values.monitor_interval}}}ms
backend_connect_timeout={{{values.backend_timeout}}}ms
backend_read_timeout={{{values.backend_timeout}}}ms
backend_write_timeout={{{values.backend_timeout}}}ms
{{{values.monitor_extra}}}
[rwsplit-service]
type=service
router=readwritesplit
servers={{{values.server_names}}}
user={{{values.service_user}}}
password={{{values.service_password}}}
enable_root_user={{{values.enable_root_user}}}
master_accept_reads={{{values.master_accept_reads}}}
causal_reads={{{values.causal_reads}}}
causal_reads_timeout={{{values.causal_reads_timeout}}}ms
transaction_replay={{{values.transaction_replay}}}
transaction_replay_max_size={{{values.transaction_replay_max_size}}}Mi
transaction_replay_timeout={{{values.transaction_replay_timeout}}}ms
slave_selection_criteria={{{values.slave_selection_criteria}}}
max_slave_connections={{{values.max_slave_connections}}}
max_replication_lag={{{values.max_replication_lag}}}s
master_failure_mode={{{values.master_failure_mode}}}
{{{values.custom_rendered}}}

[rwsplit-listener]
type=listener
service=rwsplit-service
protocol={{{values.listener_protocol}}}
address={{{values.listener_address}}}
port={{{values.listener_port}}}
`).
		MustBuild()
}
