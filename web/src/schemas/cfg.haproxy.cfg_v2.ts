// GENERATED from schemapb schema cfg.haproxy.cfg@2 — do not edit.
// HAProxy 2.9 TCP balancer in front of a replicated database.

/** object check */
export interface CfgHaproxyCfg2ItemCheck {
  /** Check kind. tcp = a bare connect; httpchk = an HTTP request against the node's REST API (Patroni, and any other role-reporting endpoint). */
  kind?: "tcp" | "httpchk";
  /** Method. http-check send meth. */
  method?: "GET" | "OPTIONS" | "HEAD";
  /** URI. http-check send uri. Patroni: /primary is 200 only on the leader, /replica only on a running replica (optionally /replica?lag=1MB). */
  uri?: string;
  /** Expected status. http-check expect status. */
  expect_status?: number | string;
  /** Check port. server ... check port: the REST API port, not the database port. Patroni listens on 8008. */
  port?: number | string;
  /** Check interval. default-server inter: seconds between checks. [s] */
  inter?: number | string;
  /** Fall. default-server fall: failed checks before a server is taken out. */
  fall?: number | string;
  /** Rise. default-server rise: successful checks before a server is put back. */
  rise?: number | string;
}

/** object  */
export interface CfgHaproxyCfg2ItemItem {
  /** Name. server name in the config and in the logs. */
  name: string;
  /** Address. Backend address. Filled by the server from topology. */
  address: string;
  /** Port. Backend database port. */
  port: number | string;
  /** Weight. server weight for the balancing algorithm. */
  weight?: number | string;
  /** Backup. server backup: used only when every non-backup server is down. */
  backup?: boolean;
  /** Max connections. server maxconn: per-backend connection ceiling; 0 leaves it unlimited. */
  maxconn?: number | string;
}

/** object  */
export interface CfgHaproxyCfg2Item {
  /** Name. Proxy name; also the log tag. */
  name: string;
  /** Bind port. Port this listener binds on all interfaces. */
  bind_port: number | string;
  /** Mode. Proxy mode; tcp for every database protocol. */
  mode?: "tcp";
  /** Balance. Load-balancing algorithm. leastconn suits long-lived database connections; roundrobin a read fan-out; source pins a client to one node. */
  balance?: "roundrobin" | "leastconn" | "source" | "first";
  /** Health check. */
  check: CfgHaproxyCfg2ItemCheck;
  /** Servers. */
  servers: Array<CfgHaproxyCfg2ItemItem>;
}

/** root */
export interface CfgHaproxyCfg2 {
  /** Max connections. global maxconn: process-wide connection ceiling. Must exceed the total VU count of the workload. */
  maxconn?: number | string;
  /** Log target. global log line, "<address> <facility> [<level>]". Empty disables logging. */
  log?: string;
  /** Threads. global nbthread; 0 lets HAProxy bind one thread per available CPU. */
  nbthread?: number | string;
  /** Stats socket. global stats socket path used by the agent to read live counters. Empty disables it. */
  stats_socket?: string;
  /** Mode. defaults mode. Database wire protocols are opaque to HAProxy, so this is always tcp. */
  readonly mode?: "tcp";
  /** timeout connect. How long a connection attempt to a backend server may take. [s] */
  timeout_connect?: number | string;
  /** timeout client. Inactivity allowed on the client side. Must outlast the longest workload segment or long queries are cut. [s] */
  timeout_client?: number | string;
  /** timeout server. Inactivity allowed on the server side; mirror timeout client. [s] */
  timeout_server?: number | string;
  /** timeout check. Timeout of one health check once the connection is established. [s] */
  timeout_check?: number | string;
  /** Retries. Connection retries against a backend server before it is declared unreachable. */
  retries?: number | string;
  /** option tcplog. Log a line per TCP session (connection times, backend, termination state). */
  option_tcplog?: boolean;
  /** log-format. Custom log-format string; empty keeps the tcplog default format. */
  log_format?: string;
  /** Stats page. Serve the HTTP stats page on its own listener; the run detail links to it. */
  stats_enabled?: boolean;
  /** Stats port. bind port of the stats listener. */
  stats_port?: number | string;
  /** Stats URI. stats uri path. */
  stats_uri?: string;
  /** Stats auth. stats auth "<user>:<password>"; empty leaves the page unauthenticated inside the run network. */
  stats_auth?: string;
  /** Listeners. One frontend/backend pair per access path (rw, ro, ...). Filled by the server from topology. */
  listeners?: Array<CfgHaproxyCfg2Item>;
  /** Rendered proxy blocks. Derived: every listen block with its servers. */
  readonly listener_blocks?: string;
  /** Rendered stats block. */
  readonly stats_block?: string;
  /** Rendered optional global lines. */
  readonly global_extra?: string;
  /** Rendered optional defaults lines. */
  readonly defaults_extra?: string;
}
