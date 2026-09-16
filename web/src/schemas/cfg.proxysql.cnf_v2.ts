// GENERATED from schemapb schema cfg.proxysql.cnf@2 — do not edit.
// ProxySQL 2.6/2.7 configuration rendered to /etc/proxysql.cnf (only read on a clean datadir or with --initial).

/** object  */
export interface CfgProxysqlCnf2Item {
  /** Address. Backend host or IP. */
  address: string;
  /** Port. Backend port. */
  port?: number | string;
  /** Hostgroup. Initial hostgroup id — writer_hostgroup for the primary, reader_hostgroup for replicas. */
  hostgroup: number | string;
  /** Backend TLS. Use TLS for backend connections; required for fresh caching_sha2_password authentication. */
  use_ssl?: number | string;
  /** Weight. Relative share of traffic inside its hostgroup. */
  weight?: number | string;
  /** Max connections. Backend connection-pool limit for this server. */
  max_connections?: number | string;
}

/** root */
export interface CfgProxysqlCnf2 {
  /** Data directory. Where proxysql.db lives. If that database already exists this file is ignored beyond this key. */
  datadir?: string;
  /** Error log. Path of the ProxySQL error log. */
  errorlog?: string;
  /** Admin credentials. user:password pairs for the admin interface (6032). Change it — the default admin:admin is only reachable from localhost. */
  admin_credentials?: string;
  /** Admin interfaces. host:port list the admin interface listens on, semicolon-separated. */
  admin_mysql_ifaces?: string;
  /** Metrics endpoint. Expose native Prometheus metrics through the REST API listener. */
  restapi_enabled?: "true" | "false";
  /** Metrics port. REST API and Prometheus listener port. */
  restapi_port?: number | string;
  /** Client interfaces. host:port list the SQL proxy listens on; not changeable at runtime. */
  interfaces?: string;
  /** Worker threads. Worker threads handling client traffic; not changeable at runtime. Size it to the CPU count. */
  threads?: number | string;
  /** Max frontend connections. Maximum client connections to the proxy (compiled default 10000, the shipped sample sets 2048). */
  max_connections?: number | string;
  /** Default query delay. Artificial delay before forwarding a query; a throttling knob, 0 in normal use. [ms] */
  default_query_delay?: number | string;
  /** Default query timeout. Timeout for a query with no rule-level timeout. [ms] */
  default_query_timeout?: number | string;
  /** Backend connect timeout. Timeout of a single connection attempt to a backend. [ms] */
  connect_timeout_server?: number | string;
  /** Poll timeout. Event-loop poll timeout. [ms] */
  poll_timeout?: number | string;
  /** Thread stack size. Stack size of each worker thread; not changeable at runtime. [B] */
  stacksize?: number | string;
  /** Announced server version. Version string the proxy reports to clients; set it to the real backend version so drivers pick the right protocol. */
  server_version?: string;
  /** Default schema. Schema used when a client connects without one. */
  default_schema?: string;
  /** Default charset. Character set assumed for clients that do not announce one. */
  default_charset?: string;
  /** Default collation. Client handshake collation; must match default_charset. The default matches Stroppy's utf8mb4 charset instead of the upstream utf8 collation. */
  default_collation_connection?: string;
  /** Sort sessions. Sort sessions by connection id to improve cache locality across threads. */
  sessions_sort?: "true" | "false";
  /** Command statistics. Collect per-command counters in stats_mysql_commands_counters. */
  commands_stats?: "true" | "false";
  /** Monitor user. Account the monitor module uses for its health checks; needs USAGE and REPLICATION CLIENT. */
  monitor_username?: string;
  /** Monitor password. Password of that account; filled by the server. */
  monitor_password?: string | null;
  /** Monitor history. How long monitor check results are kept. [ms] */
  monitor_history?: number | string;
  /** Connect check interval. How often the monitor opens a fresh connection to each backend. [ms] */
  monitor_connect_interval?: number | string;
  /** Ping interval. How often the monitor pings each backend. [ms] */
  monitor_ping_interval?: number | string;
  /** read_only check interval. How often read_only is polled — this is what moves a server between the writer and reader hostgroups. [ms] */
  monitor_read_only_interval?: number | string;
  /** read_only check timeout. Timeout of one read_only check. [ms] */
  monitor_read_only_timeout?: number | string;
  /** Connection-pool ping interval. How often idle pooled backend connections are pinged. [ms] */
  ping_interval_server_msec?: number | string;
  /** Connection-pool ping timeout. Timeout of one such ping. [ms] */
  ping_timeout_server?: number | string;
  /** Application user. Frontend account clients authenticate with; must exist on the backends with the same password. */
  username?: string;
  /** Application password. Password of that account; filled by the server. */
  password?: string | null;
  /** Per-user max connections. Frontend connection limit for that user. */
  user_max_connections?: number | string;
  /** Transaction persistent. 1 pins a session to one hostgroup for the whole transaction — required for correctness with read/write split. */
  transaction_persistent?: number | string;
  /** Backend topology. Which hostgroup manager block is rendered: async/semisync replication (read_only based), group replication, Galera, or none (static hostgroups only). */
  topology?: "replication" | "group_replication" | "galera" | "none";
  /** Writer hostgroup. Hostgroup id receiving writes. */
  writer_hostgroup?: number | string;
  /** Reader hostgroup. Hostgroup id receiving reads. */
  reader_hostgroup?: number | string;
  /** Backup writer hostgroup. Group Replication / Galera: members that could become primary. */
  backup_writer_hostgroup?: number | string;
  /** Offline hostgroup. Group Replication / Galera: members that left or fell too far behind. */
  offline_hostgroup?: number | string;
  /** Replication check type. Which read_only flavor decides writer vs reader. Only these three are settable from the config file — the combined forms need the admin interface. */
  check_type?: "read_only" | "innodb_read_only" | "super_read_only";
  /** Max writers. Group Replication / Galera: how many members may sit in the writer hostgroup at once. */
  max_writers?: number | string;
  /** Writer is also reader. 0 = writer stays out of the reader pool, 1 = writer also reads, 2 = only backup writers read. */
  writer_is_also_reader?: number | string;
  /** Max transactions behind. Group Replication / Galera: lag in transactions before a reader is shunned; 0 disables the check. */
  max_transactions_behind?: number | string;
  /** Read/write split. Render the two default rules: `SELECT ... FOR UPDATE` to the writer, every other `SELECT` to the reader. */
  read_write_split?: "true" | "false";
  /** Backends. The mysql_servers records; filled by the server from topology. */
  mysql_servers?: Array<CfgProxysqlCnf2Item> | null;
  /** Extra settings. Raw `key = value` settings appended verbatim at the end of the file; keys must match ^[a-z_][a-z0-9_.]*$. Entry order is not preserved. */
  custom?: Record<string, string>;
  /** mysql_servers block. The backend list in libconfig record syntax. */
  readonly servers_block?: string;
  /** mysql_users block. The single application account, routed to the writer hostgroup by default. */
  readonly users_block?: string;
  /** mysql_query_rules block. The read/write split rules; order matters — FOR UPDATE has to be matched first. */
  readonly query_rules_block?: string;
  /** Hostgroup manager block. Replication, Group Replication or Galera hostgroup manager, selected by topology. */
  readonly hostgroups_block?: string;
  /** Rendered extra mysql_variables. The `custom` map joined into extra keys inside the mysql_variables block; values must already carry libconfig quoting. */
  readonly custom_rendered?: string;
}

