// GENERATED from schemapb schema cfg.pgbouncer.ini@1 — do not edit.
// pgbouncer.ini for PgBouncer 1.23/1.24 in front of the PostgreSQL primary.

/** root */
export interface CfgPgbouncerIni1 {
  /** Pool name. Name clients connect to; the left-hand side of the [databases] entry. */
  db_name?: string;
  /** Primary host. Host of the PostgreSQL primary the pool forwards to. Filled by the server from the topology. */
  db_host?: string | null;
  /** Primary port. Port of the PostgreSQL primary. Filled by the server from the topology. */
  db_port?: number | string;
  /** Target database. Database name on the primary; defaults to the pool name when unset. */
  db_dbname?: string;
  /** Rendered [databases]. The [databases] entry, emitted only once the server has filled db_host. */
  readonly databases_rendered?: string;
  /** Listen address. Addresses PgBouncer binds to. Product default '*': the load generator reaches the pooler over the network, unlike the upstream localhost-only default. */
  listen_addr?: string;
  /** Listen port. TCP port PgBouncer listens on. */
  listen_port?: number | string;
  /** Auth type. How PgBouncer authenticates clients. Product default scram-sha-256, matching PostgreSQL's own password_encryption default; upstream still defaults to md5. */
  auth_type?: "scram-sha-256" | "md5" | "plain" | "trust" | "cert" | "hba" | "any";
  /** Auth file. userlist.txt with the name/password pairs PgBouncer authenticates against. */
  auth_file?: string;
  /** Auth user. Role auth_query runs as for users missing from auth_file. */
  auth_user?: string | null;
  /** Auth query. Query used to fetch a password when auth_user is set. */
  auth_query?: string | null;
  /** Pool mode. When a server connection is returned to the pool. Product default transaction: it is the mode a benchmark pooler is deployed for; upstream defaults to session. */
  pool_mode?: "session" | "transaction" | "statement";
  /** Max client connections. Maximum client connections PgBouncer accepts. */
  max_client_conn?: number | string;
  /** Default pool size. Server connections per user/database pair. */
  default_pool_size?: number | string;
  /** Min pool size. Server connections kept open even when idle; 0 disables. */
  min_pool_size?: number | string;
  /** Reserve pool size. Extra connections a pool may open once clients have waited reserve_pool_timeout; 0 disables. */
  reserve_pool_size?: number | string;
  /** Reserve pool timeout. How long a client waits before the reserve pool is used; 0 disables. [s] */
  reserve_pool_timeout?: number;
  /** Max DB connections. Cap on server connections per database across all pools; 0 is unlimited. */
  max_db_connections?: number | string;
  /** Max user connections. Cap on server connections per user across all pools; 0 is unlimited. */
  max_user_connections?: number | string;
  /** Server idle timeout. Close a server connection idle longer than this; 0 disables. [s] */
  server_idle_timeout?: number;
  /** Server lifetime. Recycle a server connection older than this; 0 disables. [s] */
  server_lifetime?: number;
  /** Server reset query. Query run when a server connection is returned to the pool. Upstream default is `DISCARD ALL`, which is only valid in session pooling; the product default is empty because pool_mode defaults to transaction. */
  server_reset_query?: string;
  /** Query wait timeout. Longest a query may wait for a server connection before it is canceled; 0 disables. [s] */
  query_wait_timeout?: number;
  /** Client idle timeout. Disconnect a client idle outside a transaction for longer than this; 0 disables. [s] */
  client_idle_timeout?: number;
  /** Ignore startup parameters. Startup parameters PgBouncer accepts and ignores, e.g. extra_float_digits,options. */
  ignore_startup_parameters?: string;
  /** Log connections. Log each client connection. */
  log_connections?: "1" | "0";
  /** Log disconnections. Log each client disconnection with its reason. */
  log_disconnections?: "1" | "0";
  /** Stats period. Interval between the aggregated statistics log lines. [s] */
  stats_period?: number | string;
  /** Admin users. Comma-separated roles allowed to run every command on the admin console. */
  admin_users?: string;
  /** Stats users. Comma-separated roles allowed read-only access to the admin console; the exporter uses one of these. */
  stats_users?: string;
}
