package compile

import "fmt"

// Exporters collect everything they can on a stand: every collector of
// the pinned version that applies there is on, the defaults and the ones
// the exporter leaves off. Left off are only the collectors that cannot
// work on the stand (no such hardware or daemon, no privileges) or that
// need a setup the stand does not have.
//
// doc: `<image> --help` of the pinned images (images.go).

// nodeCollectors are node_exporter v1.12 collectors off by default, turned
// on. Off stay drbd/runit/supervisord/wifi (nothing to read), ntp (needs an
// NTP server to query), perf (needs CAP_PERFMON), sysctl (needs an explicit
// key list).
var nodeCollectors = []string{
	"buddyinfo", "cgroups", "cpu_vulnerabilities", "drm", "ethtool", "interrupts", "ksmd",
	"lnstat", "logind", "meminfo_numa", "mountstats", "network_route", "nvmesubsystem",
	"pcidevice", "processes", "qdisc", "slabinfo", "softirqs", "swap", "systemd", "tcpstat",
	"xfrm", "zoneinfo",
}

// nodeExporterCmd reads the host through its root mounted at /host;
// systemd and logind talk to the host's D-Bus socket under it.
func nodeExporterCmd() []string {
	cmd := []string{
		"--path.rootfs=/host", "--path.procfs=/host/proc", "--path.sysfs=/host/sys",
		"--path.udev.data=/host/run/udev/data", fmt.Sprintf("--web.listen-address=:%d", nodeExporterPort),
		"--collector.cpu.info",
	}
	for _, c := range nodeCollectors {
		cmd = append(cmd, "--collector."+c)
	}
	return cmd
}

// nodeExporterEnv points the D-Bus client at the host's system bus.
var nodeExporterEnv = map[string]string{"DBUS_SYSTEM_BUS_ADDRESS": "unix:path=/host/run/dbus/system_bus_socket"}

// postgresCollectors are postgres_exporter v0.18 collectors off by default,
// turned on. Off stay xlog_location (PostgreSQL < 10 only),
// buffercache_summary (needs the pg_buffercache extension) and
// stat_statements.include_query (query text as a label).
var postgresCollectors = []string{
	"database_wraparound", "long_running_transactions", "postmaster", "process_idle",
	"stat_activity_autovacuum", "stat_checkpointer", "stat_statements", "stat_wal_receiver",
	"statio_user_indexes",
}

// postgresExporterCmd turns every postgres collector on. On OrioleDB the
// stat_user_tables collector stays off — it calls pg_table_size and
// pg_indexes_size per relation, and on an OrioleDB table under CREATE INDEX
// that backend segfaults (beta17-pg17), the postmaster restart killing the
// load — and the engine's own queries come in.
func postgresExporterCmd(oriole bool) []string {
	cmd := []string{fmt.Sprintf("--web.listen-address=:%d", postgresExporterPort)}
	for _, c := range postgresCollectors {
		cmd = append(cmd, "--collector."+c)
	}
	if oriole {
		cmd = append(cmd, "--no-collector.stat_user_tables", "--extend.query-path="+orioledbQueriesPath)
	}
	return cmd
}

// mysqldCollectors are mysqld_exporter v0.19 collectors turned on next to
// its defaults. Off stay heartbeat (needs a pt-heartbeat table),
// engine_tokudb_status and info_schema.rocksdb_perf_context (engines the
// stands do not run).
var mysqldCollectors = []string{
	"auto_increment.columns", "binlog_size", "engine_innodb_status",
	"global_status", "global_variables", "slave_status", "slave_hosts",
	"info_schema.clientstats", "info_schema.innodb_cmp", "info_schema.innodb_cmpmem",
	"info_schema.innodb_metrics", "info_schema.innodb_tablespaces", "info_schema.processlist",
	"info_schema.query_response_time", "info_schema.replica_host", "info_schema.schemastats",
	"info_schema.tables", "info_schema.tablestats", "info_schema.userstats",
	"mysql.user", "sys.user_summary",
	"perf_schema.eventsstatements", "perf_schema.eventsstatementssum", "perf_schema.eventswaits",
	"perf_schema.file_events", "perf_schema.file_instances", "perf_schema.indexiowaits",
	"perf_schema.memory_events", "perf_schema.replication_applier_status_by_worker",
	"perf_schema.tableiowaits", "perf_schema.tablelocks",
}

// mysqldExporterCmd scrapes the server at addr as user (the password comes
// in MYSQLD_EXPORTER_PASSWORD); group replication adds its member tables.
func mysqldExporterCmd(addr, user string, groupReplication bool) []string {
	cmd := []string{"--mysqld.address=" + addr, "--mysqld.username=" + user, fmt.Sprintf("--web.listen-address=:%d", mysqldExporterPort)}
	for _, c := range mysqldCollectors {
		cmd = append(cmd, "--collect."+c)
	}
	if groupReplication {
		cmd = append(cmd, "--collect.perf_schema.replication_group_members", "--collect.perf_schema.replication_group_member_stats")
	}
	return cmd
}

// orioledbQueriesPath is where the OrioleDB engine queries land in the
// postgres_exporter container.
const orioledbQueriesPath = "/etc/stroppy/orioledb-queries.yaml"

// orioledbQueries are the OrioleDB engine's own counters for
// postgres_exporter: shared page pools and undo logs, both server-wide.
// Per-relation size functions stay out — on an OrioleDB table under CREATE
// INDEX they segfault the backend (beta17-pg17). Rewind functions need
// orioledb.enable_rewind, off on the stands.
//
// doc: orioledb sql/orioledb--1.0_prod.sql orioledb_page_stats,
// sql/orioledb--1.6--1.7_prod.sql orioledb_undo_size.
const orioledbQueries = `orioledb_pages:
  query: "SELECT pool_name, busy_pages, free_pages, dirty_pages, all_pages FROM orioledb_page_stats()"
  master: true
  metrics:
    - pool_name:
        usage: "LABEL"
        description: "OrioleDB page pool"
    - busy_pages:
        usage: "GAUGE"
        description: "Pages in use"
    - free_pages:
        usage: "GAUGE"
        description: "Free pages"
    - dirty_pages:
        usage: "GAUGE"
        description: "Dirty pages not yet written"
    - all_pages:
        usage: "GAUGE"
        description: "Pages in the pool"
orioledb_undo:
  query: "SELECT undo_type, undo_size FROM orioledb_undo_size()"
  master: true
  metrics:
    - undo_type:
        usage: "LABEL"
        description: "Undo log kind"
    - undo_size:
        usage: "GAUGE"
        description: "Undo log size, bytes"
`
