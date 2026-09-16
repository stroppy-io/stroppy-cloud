// GENERATED from schemapb schema cfg.maxscale.cnf@25 — do not edit.
// MariaDB MaxScale 25.10 configuration rendered to /etc/maxscale.cnf: one monitor, one readwritesplit service, one listener.

/** object  */
export interface CfgMaxscaleCnf25Item {
  /** Section name. Name of the [server] section, e.g. db-1. */
  name: string;
  /** Address. Backend host or IP. */
  address: string;
  /** Port. Backend port. */
  port?: number | string;
  /** Priority. galeramon only: lower wins when use_priority is on. Ignored by mariadbmon. */
  priority?: number | string;
}

/** root */
export interface CfgMaxscaleCnf25 {
  /** Worker threads. Routing worker threads; `auto` means one per CPU core. */
  threads?: string;
  /** Admin host. Address the REST API / GUI binds to; the default 127.0.0.1 keeps it local. */
  admin_host?: string;
  /** Admin port. Port of the REST API and the GUI. */
  admin_port?: number | string;
  /** Secure GUI. Serve the GUI over HTTPS only; turning it off needs admin_ssl_* to be unset. */
  admin_secure_gui?: "true" | "false";
  /** Log to syslog. Also write the log to syslog. */
  syslog?: "true" | "false";
  /** Log to maxscale.log. Write MaxScale's own log file. */
  maxlog?: "true" | "false";
  /** Info logging. Log at INFO level — useful while bringing a cluster up, noisy under load. */
  log_info?: "true" | "false";
  /** Monitor module. mariadbmon for asynchronous/semisync replication, galeramon for a Galera cluster. */
  monitor_module?: "mariadbmon" | "galeramon";
  /** Monitor user. Backend account the monitor uses; needs REPLICATION CLIENT (and more for failover). */
  monitor_user?: string;
  /** Monitor password. Password of that account; filled by the server. */
  monitor_password?: string | null;
  /** Monitor interval. How often the monitor polls every backend. Rendered with an explicit ms suffix — unitless durations are deprecated. [ms] */
  monitor_interval?: number | string;
  /** Backend timeout. Connect, read and write timeout of one monitor check. Replaces the three backend_*_timeout parameters, which 25.10 deprecated. [ms] */
  backend_timeout?: number | string;
  /** Auto failover. Promote a replica when the primary is lost (mariadbmon only). */
  auto_failover?: "true" | "false";
  /** Auto rejoin. Re-attach an old primary as a replica once it comes back (mariadbmon only). */
  auto_rejoin?: "true" | "false";
  /** Enforce read_only on replicas. Set read_only=1 on any writable replica. The wider enforce_read_only_servers also exists — this is not a rename. */
  enforce_read_only_slaves?: "true" | "false";
  /** Fail count. Consecutive failed checks before the primary is declared down; 0 or 1 means immediately. */
  failcount?: number | string;
  /** Replication user. Account written into CHANGE MASTER TO during failover/rejoin. */
  replication_user?: string;
  /** Replication password. Password of that account; filled by the server. */
  replication_password?: string | null;
  /** Cooperative monitoring locks. Lets several MaxScale instances agree on who may run failover. */
  cooperative_monitoring_locks?: "none" | "majority_of_all" | "majority_of_running";
  /** Service user. Account MaxScale uses to read the backends' user tables. */
  service_user?: string;
  /** Service password. Password of that account; filled by the server. */
  service_password?: string | null;
  /** Primary accepts reads. Send read traffic to the primary as well as the replicas. */
  master_accept_reads?: "true" | "false";
  /** Causal reads. Read-your-own-writes guarantee level; anything but none costs a synchronization wait. */
  causal_reads?: "none" | "local" | "global" | "fast" | "fast_global" | "universal" | "fast_universal";
  /** Causal reads timeout. How long a causal read waits for the replica to catch up before it falls back to the primary. [ms] */
  causal_reads_timeout?: number | string;
  /** Transaction replay. Replay an interrupted transaction on another server. Enabling it forces delayed_retry, master_reconnection and master_failure_mode=fail_on_write. */
  transaction_replay?: "true" | "false";
  /** Transaction replay max size. Largest transaction MaxScale will buffer for replay. [MB] */
  transaction_replay_max_size?: number | string;
  /** Replica selection. How a read is assigned to a replica. */
  slave_selection_criteria?: "least_current_operations" | "adaptive_routing" | "least_behind_master" | "least_router_connections" | "least_global_connections";
  /** Max replica connections. How many replicas one session may hold connections to. */
  max_slave_connections?: number | string;
  /** Max replication lag. Replicas lagging more than this stop receiving reads; 0 disables the check. Whole seconds only. [s] */
  max_replication_lag?: number | string;
  /** Primary failure mode. What happens to a session when the primary disappears. 24.02 moved the default to fail_on_write, which is also what transaction_replay requires. */
  master_failure_mode?: "fail_instantly" | "fail_on_write" | "error_on_write";
  /** Transaction replay timeout. How long MaxScale keeps retrying a replayed transaction before giving up; 0 falls back to the session's own timeouts. [ms] */
  transaction_replay_timeout?: number | string;
  /** Listener port. Port clients connect to. */
  listener_port?: number | string;
  /** Listener address. Address the listener binds to; :: means every interface. */
  listener_address?: string;
  /** Listener protocol. Client protocol module. This is the only place `protocol` is valid — a [server] section has none. */
  listener_protocol?: "mariadb";
  /** Backends. The [server] sections plus the servers= lists of the monitor and the service; filled by the server from topology. */
  servers?: Array<CfgMaxscaleCnf25Item> | null;
  /** Extra settings. Raw `key = value` settings appended verbatim at the end of the file; keys must match ^[a-z_][a-z0-9_.]*$. Entry order is not preserved. */
  custom?: Record<string, string>;
  /** Server sections. One [server] section per backend — with no protocol parameter, which MaxScale does not accept there. */
  readonly server_sections?: string;
  /** Server names. Comma-separated section names for the monitor's and the service's servers= lines. */
  readonly server_names?: string;
  /** Monitor extras. The mariadbmon-only failover parameters, or galeramon's use_priority. */
  readonly monitor_extra?: string;
  /** Rendered extra service parameters. The `custom` map joined into extra key=value lines of the [rwsplit-service] section. */
  readonly custom_rendered?: string;
}
