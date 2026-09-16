// GENERATED from schemapb schema cfg.patroni.yml@3 — do not edit.
// patroni.yml for Patroni 3.x with an etcd3 DCS.

/** root */
export interface CfgPatroniYml3 {
  /** Cluster scope. Cluster name; every member of one Patroni cluster shares it. Filled by the server from the run's topology. */
  scope?: string;
  /** Node name. Unique member name inside the scope. Filled by the server from the node's role index. */
  name?: string | null;
  /** DCS namespace. Key prefix Patroni uses inside the DCS; rendered as the top-level `namespace` key (the field cannot be named `namespace`: it is a CEL reserved word). */
  dcs_namespace?: string;
  /** REST API listen. host:port the Patroni REST API binds to; rendered as restapi.listen. */
  restapi_listen?: string;
  /** REST API connect address. host:port other members and haproxy use to reach this node's REST API. Filled by the server from the node's address. */
  restapi_connect_address?: string | null;
  /** etcd3 hosts. host:port list of the etcd v3 cluster backing the DCS. Filled by the server from the etcd topology. */
  etcd3_hosts?: Array<string> | null;
  /** Rendered etcd3 hosts. The etcd3 host list as an indented YAML sequence (the render context does not expand lists). */
  readonly etcd3_hosts_rendered?: string;
  /** Leader TTL. Lifetime of the leader lock: how long a leaderless cluster waits before failover. [s] */
  ttl?: number | string;
  /** Loop wait. Sleep between Patroni's heartbeat cycles. [s] */
  loop_wait?: number | string;
  /** Retry timeout. Timeout for DCS and PostgreSQL operations before they are retried. [s] */
  retry_timeout?: number | string;
  /** Maximum lag on failover. Replication lag, in bytes, above which a replica may not be promoted. [B] */
  maximum_lag_on_failover?: number | string;
  /** Synchronous mode. Synchronous replication mode. `quorum` (quorum-based commit) is the Patroni 3.x addition. */
  synchronous_mode?: "off" | "on" | "quorum";
  /** Synchronous mode strict. Never drop back to asynchronous replication, even with no healthy synchronous standby: writes stall instead. */
  synchronous_mode_strict?: "true" | "false";
  /** Synchronous node count. Number of synchronous standbys Patroni maintains when synchronous_mode is on. */
  synchronous_node_count?: number | string;
  /** Failsafe mode. Keep the primary running when the DCS is unreachable but every member answers the failsafe endpoint. Patroni 3.0+. */
  failsafe_mode?: "true" | "false";
  /** Use pg_rewind. Rejoin a demoted primary with pg_rewind instead of a full basebackup. Product default true: a stroppy rerun must not spend a basebackup on every failover. */
  use_pg_rewind?: "true" | "false";
  /** Use replication slots. Have Patroni manage physical replication slots for the members. */
  use_slots?: "true" | "false";
  /** PostgreSQL listen. host:port PostgreSQL binds to; rendered as postgresql.listen. */
  postgresql_listen?: string;
  /** PostgreSQL connect address. host:port other members use to reach this PostgreSQL. Filled by the server from the node's address. */
  postgresql_connect_address?: string | null;
  /** Data directory. PGDATA directory Patroni initializes and manages. */
  data_dir?: string;
  /** Binary directory. Directory holding the PostgreSQL binaries; empty means look them up on PATH. */
  bin_dir?: string;
  /** pg_hba.conf path. Path of the pg_hba.conf rendered from cfg.pg_hba.conf@1. Patroni's inline `pg_hba` list is not used: the file is the single source. */
  pg_hba_path?: string | null;
  /** Superuser. Superuser role Patroni connects with. */
  superuser_username?: string;
  /** Superuser password. Password of the superuser role. */
  superuser_password?: string | null;
  /** Replication user. Role used for streaming replication between members. */
  replication_username?: string;
  /** Replication password. Password of the replication role. */
  replication_password?: string | null;
  /** Rewind user. Role pg_rewind runs as; only needed when use_pg_rewind is true and the superuser is not used. */
  rewind_username?: string;
  /** Rewind password. Password of the rewind role. */
  rewind_password?: string | null;
  /** nofailover. Exclude this member from leader races. */
  tag_nofailover?: "true" | "false";
  /** noloadbalance. Make the /replica REST endpoint return 503 so load balancers skip this member. */
  tag_noloadbalance?: "true" | "false";
  /** clonefrom. Prefer this member as the source of a basebackup for new replicas. */
  tag_clonefrom?: "true" | "false";
  /** nosync. Never choose this member as a synchronous standby. */
  tag_nosync?: "true" | "false";
  /** Watchdog mode. Watchdog use: `automatic` uses it when available, `required` refuses to promote without it, `off` disables it. */
  watchdog_mode?: "off" | "automatic" | "required";
  /** Watchdog device. Watchdog device node. */
  watchdog_device?: string;
  /** Watchdog safety margin. Seconds the watchdog must fire before the leader key expires; -1 keeps a fixed half-TTL margin. [s] */
  watchdog_safety_margin?: number | string;
  /** Patroni-owned PostgreSQL parameters. PostgreSQL parameters Patroni itself must manage in the DCS (the rest of postgresql.conf comes from cfg.postgresql.conf@<major>). Entry order is not preserved. */
  postgresql_parameters?: Record<string, string>;
  /** Rendered parameters. The postgresql_parameters map as an indented YAML mapping. */
  readonly postgresql_parameters_rendered?: string;
}
